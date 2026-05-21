// connect_sessions.go is the Sprint 3 PR2 Connect sessions API.
//
// Three endpoints:
//
//	POST /v1/connect/sessions                 (API key)  — create a session, get a hosted-page URL
//	GET  /v1/connect/sessions/{id}            (API key)  — poll session status
//	GET  /v1/public/connect/sessions/{id}     (no auth)  — read minimal session view, oauth_state-protected
//
// The customer creates a session, sends the URL to their end user via
// email / in-app, and the user lands on the hosted dashboard page
// at app.unipost.dev/connect/<platform>?session=<id>&state=<oauth_state>.
// The page completes the OAuth handshake (PR3/PR4) or app-password
// form (PR5) and the resulting social_accounts row is referenced from
// connect_sessions.completed_social_account_id.
//
// Sessions expire 30 minutes after creation. Expired sessions are
// flipped to status='expired' lazily on read — no background sweeper
// is required for Sprint 3.

package handler

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/integrationlogs"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

// ConnectSessionHandler owns the Connect session lifecycle.
type ConnectSessionHandler struct {
	queries *db.Queries
	// dashboardURL is the public origin of the hosted page (e.g.
	// "https://app.unipost.dev"). Read once at construction time
	// from NEXT_PUBLIC_APP_URL — same env var the preview link uses.
	dashboardURL string
	// quota is the plan-aware checker used to enforce the X-paid-only
	// gate (migration 057). Optional — nil means the connect handler
	// runs without plan checks (legacy + test path).
	quota *quota.Checker
	ilog  *integrationlogs.Logger
}

func NewConnectSessionHandler(queries *db.Queries, dashboardURL string, quotaChecker *quota.Checker) *ConnectSessionHandler {
	if dashboardURL == "" {
		dashboardURL = "https://app.unipost.dev"
	}
	return &ConnectSessionHandler{queries: queries, dashboardURL: dashboardURL, quota: quotaChecker}
}

func (h *ConnectSessionHandler) SetIntegrationLogger(logger *integrationlogs.Logger) *ConnectSessionHandler {
	h.ilog = logger
	return h
}

// connectablePlatformNames is the allowlist for POST /v1/connect/sessions.
// Keep this in sync with the connectors actually registered in main.go.
var connectablePlatformNames = []string{
	"twitter",
	"linkedin",
	"bluesky",
	"youtube",
	"tiktok",
	"instagram",
	"threads",
	"facebook",
	"pinterest",
}

var connectablePlatforms = func() map[string]bool {
	out := make(map[string]bool, len(connectablePlatformNames))
	for _, platform := range connectablePlatformNames {
		out[platform] = true
	}
	return out
}()

var connectablePlatformList = strings.Join(connectablePlatformNames, ", ")

// connectSessionTTL is the wall-clock window during which a hosted
// page link is honored. Stripe uses 24h for Connect — we go shorter
// because the link is meant to be acted on immediately, not stashed.
const connectSessionTTL = 30 * time.Minute

// connectSessionResponse is the API-key-authenticated view of a session.
// The public view (publicConnectSessionResponse) is a strict subset.
type connectSessionResponse struct {
	ID                       string     `json:"id"`
	Platform                 string     `json:"platform"`
	ExternalUserID           string     `json:"external_user_id"`
	ExternalUserEmail        string     `json:"external_user_email,omitempty"`
	ReturnURL                string     `json:"return_url,omitempty"`
	AllowQuickstartCreds     bool       `json:"allow_quickstart_creds"`
	Status                   string     `json:"status"`
	URL                      string     `json:"url,omitempty"`
	ExpiresAt                time.Time  `json:"expires_at"`
	CreatedAt                time.Time  `json:"created_at"`
	CompletedAt              *time.Time `json:"completed_at,omitempty"`
	CompletedSocialAccountID string     `json:"completed_social_account_id,omitempty"`
	ManagedAccountID         string     `json:"managed_account_id,omitempty"`
}

func toConnectSessionResponse(s db.ConnectSession, hostedURL string) connectSessionResponse {
	resp := connectSessionResponse{
		ID:                   s.ID,
		Platform:             s.Platform,
		ExternalUserID:       s.ExternalUserID,
		AllowQuickstartCreds: s.AllowQuickstartCreds,
		Status:               s.Status,
		URL:                  hostedURL,
		ExpiresAt:            s.ExpiresAt.Time,
		CreatedAt:            s.CreatedAt.Time,
	}
	if s.ExternalUserEmail.Valid {
		resp.ExternalUserEmail = s.ExternalUserEmail.String
	}
	if s.ReturnUrl.Valid {
		resp.ReturnURL = s.ReturnUrl.String
	}
	if s.CompletedAt.Valid {
		t := s.CompletedAt.Time
		resp.CompletedAt = &t
	}
	if s.CompletedSocialAccountID.Valid {
		resp.CompletedSocialAccountID = s.CompletedSocialAccountID.String
		resp.ManagedAccountID = s.CompletedSocialAccountID.String
	}
	return resp
}

// publicConnectSessionResponse is the no-auth projection used by the
// hosted page. Carefully strips anything sensitive (oauth_state,
// pkce_verifier, internal IDs the page doesn't need).
//
// Branding (Sprint 4 PR4) is read from the profile's branding_*
// columns and rendered by the dashboard /connect/[platform] page.
// Any of the three may be empty — the page falls back to UniPost
// defaults when a field is missing.
type publicConnectSessionResponse struct {
	Platform    string                 `json:"platform"`
	ProfileName string                 `json:"profile_name"`
	Status      string                 `json:"status"`
	ReturnURL   string                 `json:"return_url,omitempty"`
	ExpiresAt   time.Time              `json:"expires_at"`
	Branding    *publicBrandingPayload `json:"branding,omitempty"`
}

type publicBrandingPayload struct {
	LogoURL       string `json:"logo_url,omitempty"`
	DisplayName   string `json:"display_name,omitempty"`
	PrimaryColor  string `json:"primary_color,omitempty"`
	HidePoweredBy bool   `json:"hide_powered_by,omitempty"`
}

func connectSessionPlatformUsesOAuthApp(platform string) bool {
	switch platform {
	case "twitter", "linkedin", "youtube", "instagram", "tiktok", "threads", "facebook", "pinterest":
		return true
	default:
		return false
	}
}

// Create handles POST /v1/connect/sessions.
//
// Body: {platform, external_user_id, external_user_email?, return_url?}
// Returns: {id, platform, status, url, expires_at, ...}
//
// PKCE verifier is generated up front for Twitter so the hosted page's
// /authorize endpoint (PR3) can derive the challenge from the row
// without re-rolling the verifier on every click.
func (h *ConnectSessionHandler) Create(w http.ResponseWriter, r *http.Request) {
	// API keys are workspace-scoped; connect_sessions rows reference
	// profiles. Resolve the right profile below from the request body
	// (explicit) or the workspace's single profile (implicit).
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	var body struct {
		Platform             string `json:"platform"`
		ProfileID            string `json:"profile_id"`
		ExternalUserID       string `json:"external_user_id"`
		ExternalUserEmail    string `json:"external_user_email"`
		ReturnURL            string `json:"return_url"`
		AllowQuickstartCreds bool   `json:"allow_quickstart_creds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	profileID, perr := h.resolveProfileForWorkspace(r.Context(), workspaceID, body.ProfileID)
	if perr != nil {
		writeError(w, perr.status, perr.code, perr.msg)
		return
	}

	body.Platform = strings.ToLower(strings.TrimSpace(body.Platform))
	body.ExternalUserID = strings.TrimSpace(body.ExternalUserID)

	if !connectablePlatforms[body.Platform] {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"platform must be one of "+connectablePlatformList)
		return
	}
	// Plan gate (migration 057): block new X / Twitter connections on
	// plans that disallow it. Already-connected accounts on free
	// workspaces stay reachable; only the connect path is gated, so
	// downgraded customers don't lose visibility into existing tokens.
	// Falls open if the quota checker isn't wired (test path).
	if h.quota != nil && !h.quota.PlanAllowsPlatform(r.Context(), workspaceID, body.Platform) {
		writeError(w, http.StatusPaymentRequired, "PLAN_PLATFORM_NOT_ALLOWED",
			"connecting "+body.Platform+" accounts requires a paid plan — upgrade at unipost.dev/pricing")
		return
	}
	if body.ExternalUserID == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"external_user_id is required")
		return
	}
	if len(body.ExternalUserID) > 256 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"external_user_id must be ≤ 256 chars")
		return
	}
	if body.ReturnURL != "" {
		if err := validateReturnURL(body.ReturnURL); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
				"invalid return_url: "+err.Error())
			return
		}
	}
	if connectSessionPlatformUsesOAuthApp(body.Platform) && !body.AllowQuickstartCreds {
		_, credErr := h.queries.GetPlatformCredential(r.Context(), db.GetPlatformCredentialParams{
			WorkspaceID: workspaceID,
			Platform:    body.Platform,
		})
		if credErr == pgx.ErrNoRows {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
				"workspace is missing "+body.Platform+" platform credentials; upload white-label credentials first or pass allow_quickstart_creds=true")
			return
		}
		if credErr != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load platform credentials")
			return
		}
	}

	oauthState, err := randomBase64URL(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate state")
		return
	}

	// PKCE verifier is required by Twitter's OAuth 2.0 PKCE flow.
	// LinkedIn doesn't use PKCE; Bluesky doesn't use OAuth at all.
	// Generate one only when it'll actually be consumed so the column
	// is NULL for non-Twitter rows and the worker can spot at a
	// glance which sessions belong to which flow.
	pkceVerifier := pgtype.Text{}
	if body.Platform == "twitter" {
		v, err := randomBase64URL(64) // 64 bytes → 86 base64url chars (PKCE max is 128, min 43)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate verifier")
			return
		}
		pkceVerifier = pgtype.Text{String: v, Valid: true}
	}

	expiresAt := time.Now().Add(connectSessionTTL)

	session, err := h.queries.CreateConnectSession(r.Context(), db.CreateConnectSessionParams{
		ProfileID:            profileID,
		Platform:             body.Platform,
		ExternalUserID:       body.ExternalUserID,
		ExternalUserEmail:    pgtype.Text{String: body.ExternalUserEmail, Valid: body.ExternalUserEmail != ""},
		ReturnUrl:            pgtype.Text{String: body.ReturnURL, Valid: body.ReturnURL != ""},
		OauthState:           oauthState,
		PkceVerifier:         pkceVerifier,
		ExpiresAt:            pgtype.Timestamptz{Time: expiresAt, Valid: true},
		AllowQuickstartCreds: body.AllowQuickstartCreds,
	})
	if err != nil {
		slog.Error("connect session create failed",
			"workspace_id", workspaceID,
			"profile_id", profileID,
			"platform", body.Platform,
			"external_user_id", body.ExternalUserID,
			"error", err,
		)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create connect session")
		return
	}

	hostedURL := h.buildHostedURL(session.Platform, session.ID, session.OauthState)
	if h.ilog != nil {
		h.ilog.Write(r.Context(), integrationlogs.Event{
			WorkspaceID:   workspaceID,
			Level:         integrationlogs.LevelInfo,
			Status:        integrationlogs.StatusSuccess,
			Category:      integrationlogs.CategoryOAuth,
			Action:        integrationlogs.ActionAccountConnectSessionCreated,
			Source:        integrationlogs.SourceAPI,
			Message:       "Created connect session.",
			ActorUserID:   auth.GetUserID(r.Context()),
			ActorAPIKeyID: auth.GetAPIKeyID(r.Context()),
			ProfileID:     profileID,
			Platform:      body.Platform,
			Metadata: map[string]any{
				"connect_session_id":     session.ID,
				"external_user_id":       body.ExternalUserID,
				"has_return_url":         body.ReturnURL != "",
				"allow_quickstart_creds": body.AllowQuickstartCreds,
				"expires_at":             expiresAt,
			},
		})
	}
	writeCreated(w, toConnectSessionResponse(session, hostedURL))
}

// Get handles GET /v1/connect/sessions/{id} (API key, profile-scoped).
// Used by customers polling for completion when they don't want to
// run a webhook receiver. Webhooks remain the recommended path —
// this is here for the curl-loop dev case.
func (h *ConnectSessionHandler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	sessionID := chi.URLParam(r, "id")

	// Fetch the session by id alone, then verify it belongs to a
	// profile inside the caller's workspace. Avoids the project_id↔
	// workspace_id confusion from before migration 025.
	session, err := h.queries.GetConnectSessionByIDOnly(r.Context(), sessionID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Connect session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load session")
		return
	}
	profile, err := h.queries.GetProfile(r.Context(), session.ProfileID)
	if err != nil || profile.WorkspaceID != workspaceID {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Connect session not found")
		return
	}

	// Lazy expiry: if the session is past its TTL but still pending,
	// flip the column before responding so the customer sees the
	// terminal state immediately.
	if session.Status == "pending" && session.ExpiresAt.Time.Before(time.Now()) {
		_ = h.queries.ExpireConnectSession(r.Context(), session.ID)
		session.Status = "expired"
	}

	hostedURL := h.buildHostedURL(session.Platform, session.ID, session.OauthState)
	writeSuccess(w, toConnectSessionResponse(session, hostedURL))
}

// PublicGet handles GET /v1/public/connect/sessions/{id}?state=...
//
// No API key. The oauth_state query param is the bearer — it's the
// same 32-byte token embedded in the hosted-page URL we return from
// Create(). Mismatch returns 404 (NOT 403) so an attacker probing
// random session ids can't tell which exist.
//
// Returns a minimal projection: platform, project_name, status,
// return_url, expires_at. Notably absent: oauth_state, pkce_verifier,
// internal IDs, the customer's full session payload.
func (h *ConnectSessionHandler) PublicGet(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	state := r.URL.Query().Get("state")
	if sessionID == "" || state == "" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Connect session not found")
		return
	}

	session, err := h.queries.GetConnectSessionByOAuthState(r.Context(), state)
	if err != nil {
		// Don't leak whether the row exists or the state was wrong.
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Connect session not found")
		return
	}
	// Defense in depth: also check the URL id matches the row id, and
	// use a constant-time compare on the state to defeat timing probes.
	if session.ID != sessionID {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Connect session not found")
		return
	}
	if subtle.ConstantTimeCompare([]byte(session.OauthState), []byte(state)) != 1 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Connect session not found")
		return
	}

	// Lazy expiry on the public path too — the hosted page should
	// see the expired state without a separate sweeper running.
	if session.Status == "pending" && session.ExpiresAt.Time.Before(time.Now()) {
		_ = h.queries.ExpireConnectSession(r.Context(), session.ID)
		session.Status = "expired"
	}

	// Look up the profile name for display in the hosted page.
	profile, err := h.queries.GetProfile(r.Context(), session.ProfileID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Connect session not found")
		return
	}

	resp := publicConnectSessionResponse{
		Platform:    session.Platform,
		ProfileName: profile.Name,
		Status:      session.Status,
		ExpiresAt:   session.ExpiresAt.Time,
	}
	if session.ReturnUrl.Valid {
		resp.ReturnURL = session.ReturnUrl.String
	}

	planID := "free"
	if sub, subErr := h.queries.GetSubscriptionByWorkspace(r.Context(), profile.WorkspaceID); subErr == nil && sub.PlanID != "" {
		planID = sub.PlanID
	}

	// Hosted Connect branding is Basic+; the attribution-removal toggle
	// is Growth+. Older branding values may remain in the DB after a
	// downgrade, so we gate what the public page sees here instead of
	// trying to retroactively scrub stored values.
	if planAllowsHostedConnectBranding(planID) && (profile.BrandingLogoUrl.Valid || profile.BrandingDisplayName.Valid || profile.BrandingPrimaryColor.Valid || profile.BrandingHidePoweredBy) {
		resp.Branding = &publicBrandingPayload{}
		if profile.BrandingLogoUrl.Valid {
			resp.Branding.LogoURL = profile.BrandingLogoUrl.String
		}
		if profile.BrandingDisplayName.Valid {
			resp.Branding.DisplayName = profile.BrandingDisplayName.String
		}
		if profile.BrandingPrimaryColor.Valid {
			resp.Branding.PrimaryColor = profile.BrandingPrimaryColor.String
		}
		if planAllowsHidePoweredBy(planID) && profile.BrandingHidePoweredBy {
			resp.Branding.HidePoweredBy = true
		}
	}

	// Headers tuned for the hosted page: never cache, never leak
	// referrer to the OAuth provider redirect target.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	writeSuccess(w, resp)
}

// buildHostedURL renders the public dashboard URL the customer
// emails to their end user. The end user clicks → lands on the
// hosted page → page calls PublicGet using ?state=... as the bearer.
func (h *ConnectSessionHandler) buildHostedURL(platform, sessionID, oauthState string) string {
	base := strings.TrimRight(h.dashboardURL, "/")
	return base + "/connect/" + platform + "?session=" + sessionID + "&state=" + oauthState
}

// validateReturnURL rejects javascript:/data:/file: schemes and
// any non-http(s) URL. We don't enforce same-origin because the
// customer's app is by definition on a different origin.
func validateReturnURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return errReturnURLScheme
	}
	if u.Host == "" {
		return errReturnURLHost
	}
	return nil
}

var (
	errReturnURLScheme = &validationError{"return_url must use http or https"}
	errReturnURLHost   = &validationError{"return_url must have a host"}
)

type validationError struct{ msg string }

func (e *validationError) Error() string { return e.msg }

// randomBase64URL returns n bytes of crypto/rand encoded as base64url
// (no padding). Used for oauth_state and PKCE verifier generation.
func randomBase64URL(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// httpError carries enough structured info for the caller to emit a
// proper writeError without this helper needing to know about the
// response writer.
type httpError struct {
	status int
	code   string
	msg    string
}

func planAllowsHostedConnectBranding(planID string) bool {
	switch planID {
	case "basic", "growth", "team":
		return true
	default:
		return false
	}
}

func planAllowsHidePoweredBy(planID string) bool {
	switch planID {
	case "growth", "team":
		return true
	default:
		return false
	}
}

// resolveProfileForWorkspace figures out which profile row a
// connect_session should be anchored to, given an API key that only
// names a workspace.
//
//	requestedID != ""  — accept only if it belongs to this workspace
//	requestedID == ""  — if the workspace has exactly one profile, use it;
//	                     otherwise fail with a helpful error.
//
// Returning (profileID, nil) tells the caller to proceed.
func (h *ConnectSessionHandler) resolveProfileForWorkspace(ctx context.Context, workspaceID, requestedID string) (string, *httpError) {
	return resolveProfileForWorkspace(ctx, h.queries, workspaceID, requestedID)
}

func resolveProfileForWorkspace(ctx context.Context, queries *db.Queries, workspaceID, requestedID string) (string, *httpError) {
	requestedID = strings.TrimSpace(requestedID)
	if requestedID != "" {
		profile, err := queries.GetProfile(ctx, requestedID)
		if err != nil || profile.WorkspaceID != workspaceID {
			return "", &httpError{
				status: http.StatusUnprocessableEntity,
				code:   "VALIDATION_ERROR",
				msg:    "profile_id does not belong to this workspace",
			}
		}
		return profile.ID, nil
	}
	profiles, err := queries.ListProfilesByWorkspace(ctx, workspaceID)
	if err != nil {
		return "", &httpError{
			status: http.StatusInternalServerError,
			code:   "INTERNAL_ERROR",
			msg:    "Failed to list profiles",
		}
	}
	if len(profiles) == 0 {
		return "", &httpError{
			status: http.StatusUnprocessableEntity,
			code:   "VALIDATION_ERROR",
			msg:    "Workspace has no profile; create one first",
		}
	}
	if len(profiles) > 1 {
		return "", &httpError{
			status: http.StatusUnprocessableEntity,
			code:   "VALIDATION_ERROR",
			msg:    "Workspace has multiple profiles; include profile_id in the request body",
		}
	}
	return profiles[0].ID, nil
}
