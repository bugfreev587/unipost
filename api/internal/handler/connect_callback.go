// connect_callback.go is the Sprint 3 PR3 OAuth callback dispatcher.
//
// Two endpoints land here, both unauthenticated (the OAuth provider
// is calling us; there's no API key in the request):
//
//	GET /v1/public/connect/sessions/{id}/authorize?state=<oauth_state>
//	    → called by the hosted dashboard page when the user clicks
//	      "Authorize". 302s to the platform's authorize URL after
//	      computing the PKCE challenge for Twitter.
//
//	GET /v1/connect/callback/{platform}?code=...&state=...
//	    → the redirect target after the user consents on the platform.
//	      Verifies state, runs Connector.ExchangeCode + FetchProfile,
//	      upserts the social_accounts row, fires the account.connected
//	      webhook, and 302s back to the customer's return_url.
//
// Both endpoints are HTML-shaped (HTTP 302 + HTML error pages); they
// never return JSON. Cancellations / failures redirect to return_url
// with ?connect_status=cancelled|error&reason=... so the customer's
// app can render its own follow-up.

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/connect"
	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
	"github.com/xiaoboyu/unipost-api/internal/integrationlogs"
)

// ConnectCallbackHandler owns the OAuth dance for managed accounts.
// It depends on the connect.Registry (populated at startup with the
// OAuth Connect connectors) plus the standard db / encryptor / event
// bus trio.
type ConnectCallbackHandler struct {
	queries           *db.Queries
	encryptor         *crypto.AESEncryptor
	bus               events.EventBus
	registry          *connect.Registry
	callbackBaseURL   string
	limiter           *ipLimiter // shared in-memory limiter for callback brute-force protection
	ilog              *integrationlogs.Logger
	superAdminChecker *auth.SuperAdminChecker
}

func NewConnectCallbackHandler(queries *db.Queries, encryptor *crypto.AESEncryptor, bus events.EventBus, registry *connect.Registry, callbackBaseURL string, superAdminChecker *auth.SuperAdminChecker) *ConnectCallbackHandler {
	if bus == nil {
		bus = events.NoopBus{}
	}
	if callbackBaseURL == "" {
		callbackBaseURL = "https://api.unipost.dev"
	}
	return &ConnectCallbackHandler{
		queries:           queries,
		encryptor:         encryptor,
		bus:               bus,
		registry:          registry,
		callbackBaseURL:   callbackBaseURL,
		limiter:           newIPLimiter(60, time.Minute),
		superAdminChecker: superAdminChecker,
	}
}

func (h *ConnectCallbackHandler) SetIntegrationLogger(logger *integrationlogs.Logger) *ConnectCallbackHandler {
	h.ilog = logger
	return h
}

func (h *ConnectCallbackHandler) workspaceIDForProfile(ctx context.Context, profileID string) string {
	if profileID == "" {
		return ""
	}
	profile, err := h.queries.GetProfile(ctx, profileID)
	if err != nil {
		return ""
	}
	return profile.WorkspaceID
}

func (h *ConnectCallbackHandler) resolveConnector(ctx context.Context, workspaceID, platform string, allowQuickstartCreds bool) (connect.Connector, bool, error) {
	if workspaceID != "" {
		cred, err := h.queries.GetPlatformCredential(ctx, db.GetPlatformCredentialParams{
			WorkspaceID: workspaceID,
			Platform:    platform,
		})
		switch err {
		case nil:
			clientSecret, decErr := h.encryptor.Decrypt(cred.ClientSecret)
			if decErr != nil {
				return nil, false, decErr
			}
			if connector := connect.NewManagedConnector(platform, cred.ClientID, clientSecret, h.callbackBaseURL); connector != nil {
				return connector, true, nil
			}
		case pgx.ErrNoRows:
			// Fall through to the default registry-backed connector.
		default:
			return nil, false, err
		}
	}
	if !allowQuickstartCreds {
		return nil, false, nil
	}
	connector, ok := h.registry.Get(platform)
	return connector, ok, nil
}

func (h *ConnectCallbackHandler) logOAuthEvent(ctx context.Context, workspaceID string, event integrationlogs.Event) {
	if h == nil || h.ilog == nil {
		return
	}
	if workspaceID == "" {
		slog.Warn("connect callback log skipped: missing workspace",
			"action", event.Action,
			"profile_id", event.ProfileID,
			"platform", event.Platform,
			"error_code", event.ErrorCode,
		)
		return
	}
	event.WorkspaceID = workspaceID
	if event.Category == "" {
		event.Category = integrationlogs.CategoryOAuth
	}
	if event.Source == "" {
		event.Source = integrationlogs.SourceOAuth
	}
	h.ilog.Write(ctx, event)
}

// Authorize handles GET /v1/public/connect/sessions/{id}/authorize.
//
// The hosted dashboard page calls this when the user clicks
// "Authorize" on an OAuth platform. We look up the session by
// oauth_state, build the platform's authorize URL via the connector,
// and 302 the browser there. The PKCE verifier is already on the
// session row from POST /v1/connect/sessions — we don't generate it
// here, just consume it.
func (h *ConnectCallbackHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")

	sessionID := chi.URLParam(r, "id")
	state := r.URL.Query().Get("state")
	if sessionID == "" || state == "" {
		renderConnectError(w, http.StatusNotFound, "Connect link is invalid or has expired.")
		return
	}

	session, err := h.queries.GetConnectSessionByOAuthState(r.Context(), state)
	if err != nil || session.ID != sessionID {
		renderConnectError(w, http.StatusNotFound, "Connect link is invalid or has expired.")
		return
	}
	if session.Status != "pending" {
		renderConnectError(w, http.StatusConflict, "This Connect link has already been used.")
		return
	}
	if session.ExpiresAt.Time.Before(time.Now()) {
		_ = h.queries.ExpireConnectSession(r.Context(), session.ID)
		renderConnectError(w, http.StatusGone, "This Connect link has expired.")
		return
	}

	workspaceID := h.workspaceIDForProfile(r.Context(), session.ProfileID)
	if !connectSessionPlatformFeatureEnabled(r.Context(), workspaceID, session.Platform) {
		renderConnectError(w, http.StatusBadRequest, "Platform "+session.Platform+" is not enabled for hosted Connect.")
		return
	}
	connector, ok, err := h.resolveConnector(r.Context(), workspaceID, session.Platform, session.AllowQuickstartCreds)
	if err != nil {
		slog.Error("connect.authorize: resolve connector", "platform", session.Platform, "workspace_id", workspaceID, "err", err)
		renderConnectError(w, http.StatusInternalServerError, "Failed to load platform credentials.")
		return
	}
	if !ok {
		renderConnectError(w, http.StatusBadRequest, "Platform "+session.Platform+" is not supported.")
		return
	}

	authURL, err := connector.AuthorizeURL(connect.SessionView{
		ID:           session.ID,
		OAuthState:   session.OauthState,
		PKCEVerifier: session.PkceVerifier.String,
	})
	if err != nil {
		slog.Error("connect.authorize: build url", "platform", session.Platform, "err", err)
		renderConnectError(w, http.StatusInternalServerError, "Failed to build authorize URL.")
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

// Callback handles GET /v1/connect/callback/{platform}?code=...&state=...
//
// The platform redirects the user here after they accept (or deny).
// We verify state, exchange the code, fetch the profile, encrypt the
// tokens, upsert the social_accounts row, mark the session done,
// fire account.connected, and bounce back to the customer's app.
func (h *ConnectCallbackHandler) Callback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")

	// Cheap brute-force defense — prevents floods of bogus ?code=...
	// from costing us real Twitter/LinkedIn API calls.
	if !h.limiter.Allow(clientIP(r)) {
		renderConnectError(w, http.StatusTooManyRequests, "Too many requests.")
		return
	}

	platformName := chi.URLParam(r, "platform")
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	errParam := r.URL.Query().Get("error")
	errDesc := r.URL.Query().Get("error_description")

	// OAuth provider returned an error. The most common is
	// access_denied (user clicked "Cancel" on the consent screen),
	// but other errors include unauthorized_scope, invalid_request,
	// etc. Distinguish access_denied (truly cancelled) from real
	// errors so the customer's app sees the right connect_status.
	if errParam != "" {
		status := "error"
		if errParam == "access_denied" {
			status = "cancelled"
		}
		// Best-effort: find the session via state so we can flip
		// it to the right terminal state and redirect to return_url.
		if state != "" {
			if sess, err := h.queries.GetConnectSessionByOAuthState(r.Context(), state); err == nil {
				workspaceID := h.workspaceIDForProfile(r.Context(), sess.ProfileID)
				logStatus := integrationlogs.StatusError
				if status == "cancelled" {
					logStatus = integrationlogs.StatusWarning
				}
				h.logOAuthEvent(r.Context(), workspaceID, integrationlogs.Event{
					Level:     integrationlogs.LevelWarn,
					Status:    logStatus,
					Action:    integrationlogs.ActionAccountConnectCallbackFailed,
					Message:   "OAuth callback returned an error.",
					ProfileID: sess.ProfileID,
					Platform:  sess.Platform,
					ErrorCode: strings.TrimSpace(errParam),
					Metadata: map[string]any{
						"connect_session_id": sess.ID,
						"external_user_id":   sess.ExternalUserID,
						"callback_status":    status,
						"reason":             firstNonEmptyString(errDesc, errParam),
					},
				})
				if status == "cancelled" {
					_, _ = h.queries.MarkConnectSessionCancelled(r.Context(), sess.ID)
				}
				// For non-access_denied errors we leave the session
				// in 'pending' so the user can retry with the same
				// link if the issue is fixable (e.g. transient).
				reason := errDesc
				if reason == "" {
					reason = errParam
				}
				h.redirectWithStatus(w, r, sess.ReturnUrl.String, status, reason, false)
				return
			}
		}
		renderConnectError(w, http.StatusOK, "Connection failed: "+errParam)
		return
	}

	if state == "" || code == "" {
		renderConnectError(w, http.StatusBadRequest, "Missing code or state.")
		return
	}

	session, err := h.queries.GetConnectSessionByOAuthState(r.Context(), state)
	if err != nil {
		// State mismatch — never reveal whether the row existed.
		// 400 (not 404) because this is the OAuth callback path
		// and 4xx is the right shape for a bad request.
		renderConnectError(w, http.StatusBadRequest, "Invalid state.")
		return
	}
	if session.Platform != platformName {
		renderConnectError(w, http.StatusBadRequest, "Platform mismatch.")
		return
	}
	if session.Status != "pending" {
		renderConnectError(w, http.StatusConflict, "This Connect link has already been used.")
		return
	}
	if session.ExpiresAt.Time.Before(time.Now()) {
		_ = h.queries.ExpireConnectSession(r.Context(), session.ID)
		renderConnectError(w, http.StatusGone, "This Connect link has expired.")
		return
	}
	workspaceID := h.workspaceIDForProfile(r.Context(), session.ProfileID)
	if !connectSessionPlatformFeatureEnabled(r.Context(), workspaceID, platformName) {
		h.redirectWithStatus(w, r, session.ReturnUrl.String, "error", "platform_not_enabled", false)
		return
	}
	connector, ok, err := h.resolveConnector(r.Context(), workspaceID, platformName, session.AllowQuickstartCreds)
	if err != nil {
		slog.Error("connect.callback: resolve connector failed", "platform", platformName, "workspace_id", workspaceID, "err", err)
		h.redirectWithStatus(w, r, session.ReturnUrl.String, "error", "connector_resolution_failed", false)
		return
	}
	if !ok {
		renderConnectError(w, http.StatusBadRequest, "Platform not supported.")
		return
	}

	view := connect.SessionView{
		ID:           session.ID,
		OAuthState:   session.OauthState,
		PKCEVerifier: session.PkceVerifier.String,
	}

	tokens, err := connector.ExchangeCode(r.Context(), view, code)
	if err != nil {
		slog.Error("connect.callback: token exchange failed", "platform", platformName, "err", err)
		h.logOAuthEvent(r.Context(), workspaceID, integrationlogs.Event{
			Level:     integrationlogs.LevelError,
			Status:    integrationlogs.StatusError,
			Action:    integrationlogs.ActionAccountConnectCallbackFailed,
			Message:   "OAuth token exchange failed.",
			ProfileID: session.ProfileID,
			Platform:  platformName,
			ErrorCode: "token_exchange_failed",
			Metadata: map[string]any{
				"connect_session_id": session.ID,
				"external_user_id":   session.ExternalUserID,
			},
			ResponsePayload: map[string]any{"error": err.Error()},
		})
		h.redirectWithStatus(w, r, session.ReturnUrl.String, "error", "token_exchange_failed", false)
		return
	}
	profile, err := connector.FetchProfile(r.Context(), tokens.AccessToken)
	if err != nil {
		slog.Error("connect.callback: profile fetch failed", "platform", platformName, "err", err)
		h.logOAuthEvent(r.Context(), workspaceID, integrationlogs.Event{
			Level:     integrationlogs.LevelError,
			Status:    integrationlogs.StatusError,
			Action:    integrationlogs.ActionAccountConnectCallbackFailed,
			Message:   "OAuth profile fetch failed.",
			ProfileID: session.ProfileID,
			Platform:  platformName,
			ErrorCode: "profile_fetch_failed",
			Metadata: map[string]any{
				"connect_session_id": session.ID,
				"external_user_id":   session.ExternalUserID,
			},
			ResponsePayload: map[string]any{"error": err.Error()},
		})
		h.redirectWithStatus(w, r, session.ReturnUrl.String, "error", "profile_fetch_failed", false)
		return
	}

	prof, profErr := h.queries.GetProfile(r.Context(), session.ProfileID)
	hidePoweredBy := false
	if profErr == nil {
		if sub, subErr := h.queries.GetSubscriptionByWorkspace(r.Context(), prof.WorkspaceID); subErr == nil {
			hidePoweredBy = planAllowsHidePoweredBy(sub.PlanID) && prof.BrandingHidePoweredBy
		}
	}
	if profErr == nil {
		if blocked, shareErr := freePlanSharingBlocked(r.Context(), h.queries, h.superAdminChecker, prof.WorkspaceID, platformName, profile.ExternalAccountID); shareErr != nil {
			slog.Warn("connect.callback: free-plan sharing check failed", "platform", platformName, "external_id", profile.ExternalAccountID, "workspace_id", prof.WorkspaceID, "err", shareErr)
		} else if blocked {
			renderConnectError(w, http.StatusConflict, accountNotAvailableOnFreePlanMessage)
			return
		}
	}

	encAccess, err := h.encryptor.Encrypt(tokens.AccessToken)
	if err != nil {
		renderConnectError(w, http.StatusInternalServerError, "Internal error encrypting token.")
		return
	}
	encRefresh := ""
	if tokens.RefreshToken != "" {
		encRefresh, err = h.encryptor.Encrypt(tokens.RefreshToken)
		if err != nil {
			renderConnectError(w, http.StatusInternalServerError, "Internal error encrypting token.")
			return
		}
	}

	metadata, _ := json.Marshal(map[string]any{
		"username":     profile.Username,
		"display_name": profile.DisplayName,
	})

	activeAccount, lookupErr := h.queries.FindActiveManagedSocialAccountByExternalAccount(r.Context(), db.FindActiveManagedSocialAccountByExternalAccountParams{
		ProfileID:         session.ProfileID,
		Platform:          platformName,
		ExternalAccountID: profile.ExternalAccountID,
	})

	accountName := pgtype.Text{String: nonEmpty(profile.Username, profile.DisplayName), Valid: profile.Username != "" || profile.DisplayName != ""}
	accountAvatarURL := pgtype.Text{String: profile.AvatarURL, Valid: profile.AvatarURL != ""}
	refreshToken := pgtype.Text{String: encRefresh, Valid: encRefresh != ""}
	tokenExpiresAt := pgtype.Timestamptz{Time: tokens.ExpiresAt, Valid: !tokens.ExpiresAt.IsZero()}
	connectSessionID := pgtype.Text{String: session.ID, Valid: true}
	externalUserID := pgtype.Text{String: session.ExternalUserID, Valid: true}

	var saved db.SocialAccount
	switch {
	case lookupErr == nil:
		saved, err = h.queries.RefreshConnectedSocialAccount(r.Context(), db.RefreshConnectedSocialAccountParams{
			ID:                activeAccount.ID,
			AccessToken:       encAccess,
			RefreshToken:      refreshToken,
			TokenExpiresAt:    tokenExpiresAt,
			ExternalAccountID: profile.ExternalAccountID,
			AccountName:       accountName,
			AccountAvatarUrl:  accountAvatarURL,
			Metadata:          metadata,
			Scope:             tokens.Scopes,
			ConnectionType:    "managed",
			ConnectSessionID:  connectSessionID,
			ExternalUserID:    externalUserID,
			ExternalUserEmail: session.ExternalUserEmail,
		})
	case lookupErr == pgx.ErrNoRows:
		saved, err = h.queries.CreateManagedSocialAccount(r.Context(), db.CreateManagedSocialAccountParams{
			ProfileID:         session.ProfileID,
			Platform:          platformName,
			AccessToken:       encAccess,
			RefreshToken:      refreshToken,
			TokenExpiresAt:    tokenExpiresAt,
			ExternalAccountID: profile.ExternalAccountID,
			AccountName:       accountName,
			AccountAvatarUrl:  accountAvatarURL,
			Metadata:          metadata,
			Scope:             tokens.Scopes,
			ConnectSessionID:  connectSessionID,
			ExternalUserID:    externalUserID,
			ExternalUserEmail: session.ExternalUserEmail,
		})
	default:
		slog.Error("connect.callback: lookup failed", "platform", platformName, "err", lookupErr)
		renderConnectError(w, http.StatusInternalServerError, "Failed to save account.")
		return
	}
	if err != nil {
		slog.Error("connect.callback: save failed", "platform", platformName, "err", err)
		h.logOAuthEvent(r.Context(), workspaceID, integrationlogs.Event{
			Level:     integrationlogs.LevelError,
			Status:    integrationlogs.StatusError,
			Action:    integrationlogs.ActionAccountConnectCallbackFailed,
			Message:   "Failed to persist connected account.",
			ProfileID: session.ProfileID,
			Platform:  platformName,
			ErrorCode: "account_save_failed",
			Metadata: map[string]any{
				"connect_session_id": session.ID,
				"external_user_id":   session.ExternalUserID,
			},
			ResponsePayload: map[string]any{"error": err.Error()},
		})
		renderConnectError(w, http.StatusInternalServerError, "Failed to save account.")
		return
	}

	_, _ = h.queries.MarkConnectSessionCompleted(r.Context(), db.MarkConnectSessionCompletedParams{
		ID:                       session.ID,
		CompletedSocialAccountID: pgtype.Text{String: saved.ID, Valid: true},
	})

	// Webhooks are workspace-scoped; resolve workspace_id from profile.
	wsID := session.ProfileID
	if profErr == nil {
		wsID = prof.WorkspaceID
	}
	h.bus.Publish(r.Context(), wsID, events.EventAccountConnected, map[string]any{
		"social_account_id": saved.ID,
		"profile_id":        session.ProfileID,
		"platform":          platformName,
		"account_name":      profile.Username,
		"external_user_id":  session.ExternalUserID,
		"connection_type":   "managed",
	})

	slog.Info("connect.callback: account connected",
		"platform", platformName,
		"profile_id", session.ProfileID,
		"external_user_id", session.ExternalUserID,
		"account_id", saved.ID,
	)
	h.logOAuthEvent(r.Context(), wsID, integrationlogs.Event{
		Level:           integrationlogs.LevelInfo,
		Status:          integrationlogs.StatusSuccess,
		Action:          integrationlogs.ActionAccountConnectCallbackOK,
		Message:         "OAuth callback completed and account connected.",
		ProfileID:       session.ProfileID,
		SocialAccountID: saved.ID,
		Platform:        platformName,
		Metadata: map[string]any{
			"connect_session_id": session.ID,
			"external_user_id":   session.ExternalUserID,
			"account_name":       profile.Username,
			"connection_type":    "managed",
		},
	})

	h.redirectWithStatus(w, r, session.ReturnUrl.String, "success", "", hidePoweredBy)
}

// redirectWithStatus 302s to return_url with ?connect_status=... +
// optional ?reason=... query params. When return_url is empty we
// render the success / error page in place.
func (h *ConnectCallbackHandler) redirectWithStatus(w http.ResponseWriter, r *http.Request, returnURL, status, reason string, hidePoweredBy bool) {
	if returnURL == "" {
		switch status {
		case "success":
			renderConnectSuccess(w, hidePoweredBy)
		case "cancelled":
			renderConnectError(w, http.StatusOK, "Connection cancelled.", hidePoweredBy)
		default:
			renderConnectError(w, http.StatusBadRequest, "Connection failed: "+reason, hidePoweredBy)
		}
		return
	}
	q := url.Values{}
	q.Set("connect_status", status)
	if reason != "" {
		q.Set("reason", reason)
	}
	sep := "?"
	if strings.Contains(returnURL, "?") {
		sep = "&"
	}
	http.Redirect(w, r, returnURL+sep+q.Encode(), http.StatusFound)
}

func nonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

// Server-rendered HTML for the no-return_url cases.

const connectErrorTplSrc = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Connect · UniPost</title>
<style>
body{font-family:system-ui,-apple-system,sans-serif;max-width:480px;margin:48px auto;padding:0 24px;color:#111;line-height:1.5}
h1{font-size:22px;margin-bottom:8px}
.err{background:#fef2f2;border:1px solid #fecaca;color:#991b1b;padding:12px 16px;border-radius:8px;margin:16px 0}
.small{font-size:13px;color:#666;margin-top:24px}
</style>
</head><body>
<h1>Connect</h1>
<div class="err">{{.Message}}</div>
<p>If you reached this page by mistake, contact the developer who sent you the link.</p>
{{if .ShowPoweredBy}}<p class="small">Powered by UniPost</p>{{end}}
</body></html>`

const connectSuccessTplSrc = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Connected · UniPost</title>
<style>
body{font-family:system-ui,-apple-system,sans-serif;max-width:480px;margin:48px auto;padding:0 24px;color:#111;line-height:1.5;text-align:center}
h1{font-size:24px;margin-bottom:8px;color:#166534}
.ok{font-size:48px;margin-bottom:8px}
.small{font-size:13px;color:#666;margin-top:24px}
</style>
</head><body>
<div class="ok">✓</div>
<h1>Connected!</h1>
<p>You can close this window now.</p>
{{if .ShowPoweredBy}}<p class="small">Powered by UniPost</p>{{end}}
</body></html>`

var (
	connectErrorTpl   = template.Must(template.New("connect_error").Parse(connectErrorTplSrc))
	connectSuccessTpl = template.Must(template.New("connect_success").Parse(connectSuccessTplSrc))
)

func renderConnectError(w http.ResponseWriter, status int, msg string, hidePoweredBy ...bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	showPoweredBy := true
	if len(hidePoweredBy) > 0 && hidePoweredBy[0] {
		showPoweredBy = false
	}
	if err := connectErrorTpl.Execute(w, map[string]any{"Message": msg, "ShowPoweredBy": showPoweredBy}); err != nil {
		fmt.Fprintf(w, "render error: %v", err)
	}
}

func renderConnectSuccess(w http.ResponseWriter, hidePoweredBy bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := connectSuccessTpl.Execute(w, map[string]any{"ShowPoweredBy": !hidePoweredBy}); err != nil {
		fmt.Fprintf(w, "render error: %v", err)
	}
}
