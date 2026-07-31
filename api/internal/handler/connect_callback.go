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
//	      persists through workspace-level managed ownership checks,
//	      fires the account.connected webhook, and 302s back to the
//	      customer's return_url.
//
// Both endpoints are HTML-shaped (HTTP 302 + HTML error pages); they
// never return JSON. Cancellations / failures redirect to return_url
// with ?connect_status=cancelled|error&reason=... so the customer's
// app can render its own follow-up.

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/connect"
	"github.com/xiaoboyu/unipost-api/internal/connectownership"
	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
	"github.com/xiaoboyu/unipost-api/internal/instagramwebhooks"
	appmw "github.com/xiaoboyu/unipost-api/internal/middleware"
	"github.com/xiaoboyu/unipost-api/internal/quota"
	"github.com/xiaoboyu/unipost-api/internal/socialconnections"
	"github.com/xiaoboyu/unipost-api/internal/xinbox"
)

type instagramWebhookSubscriber interface {
	Subscribe(context.Context, string, string) error
}

type managedAccountOwnershipStore interface {
	Check(context.Context, connectownership.OwnershipKey) (connectownership.Decision, error)
	Save(context.Context, connectownership.SaveRequest) (db.SocialAccount, error)
}

// ConnectCallbackHandler owns the OAuth dance for managed accounts.
// It depends on the connect.Registry (populated at startup with the
// OAuth Connect connectors) plus the standard db / encryptor / event
// bus trio.
type ConnectCallbackHandler struct {
	queries                    *db.Queries
	encryptor                  *crypto.AESEncryptor
	bus                        events.EventBus
	registry                   *connect.Registry
	callbackBaseURL            string
	limiter                    *ipLimiter // shared in-memory limiter for callback brute-force protection
	ilog                       hostedConnectOutcomeWriter
	superAdminChecker          *auth.SuperAdminChecker
	quota                      *quota.Checker
	instagramWebhookSubscriber instagramWebhookSubscriber
	ownershipStore             managedAccountOwnershipStore
	connections                socialconnections.Store
}

func (h *ConnectCallbackHandler) SetConnectionStore(store socialconnections.Store) *ConnectCallbackHandler {
	h.connections = store
	return h
}

func NewConnectCallbackHandler(queries *db.Queries, encryptor *crypto.AESEncryptor, bus events.EventBus, registry *connect.Registry, callbackBaseURL string, superAdminChecker *auth.SuperAdminChecker, ownershipStore managedAccountOwnershipStore) *ConnectCallbackHandler {
	if bus == nil {
		bus = events.NoopBus{}
	}
	if callbackBaseURL == "" {
		callbackBaseURL = "https://api.unipost.dev"
	}
	return &ConnectCallbackHandler{
		queries:                    queries,
		encryptor:                  encryptor,
		bus:                        bus,
		registry:                   registry,
		callbackBaseURL:            callbackBaseURL,
		limiter:                    newIPLimiter(60, time.Minute),
		superAdminChecker:          superAdminChecker,
		quota:                      quota.NewChecker(queries),
		instagramWebhookSubscriber: instagramwebhooks.NewSubscriber(nil, ""),
		ownershipStore:             ownershipStore,
	}
}

func (h *ConnectCallbackHandler) SetIntegrationLogger(logger hostedConnectOutcomeWriter) *ConnectCallbackHandler {
	h.ilog = logger
	return h
}

func (h *ConnectCallbackHandler) freePlanManagedConnectBlocked(ctx context.Context, workspaceID, externalUserID, platform string, wouldCreateManagedAccount bool) (bool, string, error) {
	if h == nil {
		return false, "", nil
	}
	return freePlanManagedConnectBlocked(ctx, h.queries, h.quota, workspaceID, externalUserID, platform, wouldCreateManagedAccount)
}

func freePlanManagedConnectBlocked(ctx context.Context, queries *db.Queries, checker *quota.Checker, workspaceID, externalUserID, platform string, wouldCreateManagedAccount bool) (bool, string, error) {
	if queries == nil || checker == nil || workspaceID == "" || externalUserID == "" {
		return false, "", nil
	}
	externalUser := pgtype.Text{String: externalUserID, Valid: true}
	if cap, hasCap := checker.MaxManagedUsersForPlan(ctx, workspaceID); hasCap {
		existingForUser, err := queries.CountManagedAccountsByWorkspaceAndExternalUser(ctx, db.CountManagedAccountsByWorkspaceAndExternalUserParams{
			WorkspaceID:    workspaceID,
			ExternalUserID: externalUser,
		})
		if err != nil {
			return false, "", err
		}
		if existingForUser == 0 {
			currentUsers, err := queries.CountManagedUsersByWorkspace(ctx, workspaceID)
			if err != nil {
				return false, "", err
			}
			if int(currentUsers) >= cap {
				return true, "Free plan workspaces can connect up to 3 managed users. Upgrade to onboard more users through Hosted Connect.", nil
			}
		}
	}
	if !wouldCreateManagedAccount {
		return false, "", nil
	}
	if cap, hasCap := checker.MaxManagedAccountsForPlan(ctx, workspaceID); hasCap {
		existingForUserPlatform, err := queries.CountManagedAccountsByWorkspaceExternalUserAndPlatform(ctx, db.CountManagedAccountsByWorkspaceExternalUserAndPlatformParams{
			WorkspaceID:    workspaceID,
			ExternalUserID: externalUser,
			Platform:       platform,
		})
		if err != nil {
			return false, "", err
		}
		if existingForUserPlatform > 0 {
			return false, "", nil
		}
		currentAccounts, err := queries.CountActiveManagedAccountsByWorkspace(ctx, workspaceID)
		if err != nil {
			return false, "", err
		}
		if int(currentAccounts) >= cap {
			return true, "Free plan workspaces can connect up to 2 managed social accounts. Upgrade to connect more accounts through Hosted Connect.", nil
		}
	}
	return false, "", nil
}

func logManagedOwnershipConflict(workspaceID, platformName string, decision connectownership.Decision, err error) {
	conflictClass := decision.ConflictClass
	matchCount := decision.MatchCount
	var conflict *connectownership.OwnershipConflictError
	if errors.As(err, &conflict) {
		if conflict.ConflictClass != "" {
			conflictClass = conflict.ConflictClass
		}
		matchCount = conflict.MatchCount
	}
	if conflictClass == "" {
		conflictClass = connectownership.ConflictAuthoritative
	}
	slog.Warn("managed account ownership conflict",
		"workspace_id", workspaceID,
		"platform", platformName,
		"conflict_class", conflictClass,
		"match_count", matchCount)
}

func verifiedOAuthProviderIdentity(platformName string, profile *connect.Profile) (string, bool) {
	if profile == nil {
		return "", false
	}
	if platformName == "instagram" {
		identity := strings.TrimSpace(profile.WebhookAccountID)
		return identity, identity != ""
	}
	identity := strings.TrimSpace(profile.ExternalAccountID)
	return identity, identity != ""
}

var errConnectSessionCompletionMismatch = errors.New("CONNECT_SESSION_COMPLETION_MISMATCH")

func claimConnectSessionCompletion(ctx context.Context, queries *db.Queries, sessionID, socialAccountID string) error {
	completed, err := queries.MarkConnectSessionCompleted(ctx, db.MarkConnectSessionCompletedParams{
		ID:                       sessionID,
		CompletedSocialAccountID: pgtype.Text{String: socialAccountID, Valid: true},
	})
	if err != nil {
		return err
	}
	if completed.ID != sessionID || completed.Status != "completed" ||
		!completed.CompletedSocialAccountID.Valid || completed.CompletedSocialAccountID.String != socialAccountID {
		return errConnectSessionCompletionMismatch
	}
	return nil
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

type connectorResolution struct {
	connector connect.Connector
	xAppMode  pgtype.Text
	ok        bool
}

func (h *ConnectCallbackHandler) resolveConnector(ctx context.Context, workspaceID, platform string, allowQuickstartCreds bool, appModes ...pgtype.Text) (connect.Connector, bool, error) {
	appMode := pgtype.Text{}
	if len(appModes) > 0 {
		appMode = appModes[0]
	}
	resolved, err := h.resolveConnectorForStoredMode(ctx, workspaceID, platform, allowQuickstartCreds, appMode)
	return resolved.connector, resolved.ok, err
}

// resolveConnectorForStoredMode preserves explicit app identity for new
// sessions. NULL is a transient rolling-deploy compatibility case for sessions
// written by pre-108 instances, so it replays the old plan/credential/
// allow_quickstart selection policy and returns the resulting explicit mode.
func (h *ConnectCallbackHandler) resolveConnectorForStoredMode(ctx context.Context, workspaceID, platform string, allowQuickstartCreds bool, appMode pgtype.Text) (connectorResolution, error) {
	if platform == "twitter" {
		if !appMode.Valid {
			return h.resolveConnectorUsingLegacyPolicy(ctx, workspaceID, platform, allowQuickstartCreds)
		}
		switch xinbox.AppMode(appMode.String) {
		case xinbox.AppModeWorkspace:
			cred, err := h.queries.GetPlatformCredential(ctx, db.GetPlatformCredentialParams{
				WorkspaceID: workspaceID,
				Platform:    platform,
			})
			if err == pgx.ErrNoRows {
				return connectorResolution{}, nil
			}
			if err != nil {
				return connectorResolution{}, err
			}
			clientSecret, err := h.encryptor.Decrypt(cred.ClientSecret)
			if err != nil {
				return connectorResolution{}, err
			}
			connector := connect.NewManagedConnector(platform, cred.ClientID, clientSecret, h.callbackBaseURL)
			return connectorResolution{connector: connector, xAppMode: appMode, ok: connector != nil}, nil
		case xinbox.AppModeUniPostManaged:
			connector, ok := h.registry.Get(platform)
			return connectorResolution{connector: connector, xAppMode: appMode, ok: ok}, nil
		default:
			return connectorResolution{}, fmt.Errorf("invalid stored X app mode %q", appMode.String)
		}
	}
	return h.resolveConnectorUsingLegacyPolicy(ctx, workspaceID, platform, allowQuickstartCreds)
}

func (h *ConnectCallbackHandler) resolveConnectorUsingLegacyPolicy(ctx context.Context, workspaceID, platform string, allowQuickstartCreds bool) (connectorResolution, error) {
	if workspaceAllowsPlatformCredentialsForPlatform(ctx, h.queries, workspaceID, platform) {
		cred, err := h.queries.GetPlatformCredential(ctx, db.GetPlatformCredentialParams{
			WorkspaceID: workspaceID,
			Platform:    platform,
		})
		switch err {
		case nil:
			clientSecret, decErr := h.encryptor.Decrypt(cred.ClientSecret)
			if decErr != nil {
				return connectorResolution{}, decErr
			}
			if connector := connect.NewManagedConnector(platform, cred.ClientID, clientSecret, h.callbackBaseURL); connector != nil {
				mode := pgtype.Text{}
				if platform == "twitter" {
					mode = pgtype.Text{String: string(xinbox.AppModeWorkspace), Valid: true}
				}
				return connectorResolution{connector: connector, xAppMode: mode, ok: true}, nil
			}
		case pgx.ErrNoRows:
			// Fall through to the default registry-backed connector.
		default:
			return connectorResolution{}, err
		}
	}
	if !allowQuickstartCreds {
		return connectorResolution{}, nil
	}
	connector, ok := h.registry.Get(platform)
	mode := pgtype.Text{}
	if platform == "twitter" && ok {
		mode = pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true}
	}
	return connectorResolution{connector: connector, xAppMode: mode, ok: ok}, nil
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
	resolved, err := h.resolveConnectorForStoredMode(r.Context(), workspaceID, session.Platform, session.AllowQuickstartCreds, session.XAppMode)
	if err != nil {
		slog.Error("connect.authorize: resolve connector", "platform", session.Platform, "workspace_id", workspaceID, "err", err)
		renderConnectError(w, http.StatusInternalServerError, "Failed to load platform credentials.")
		return
	}
	if !resolved.ok {
		renderConnectError(w, http.StatusBadRequest, "Platform "+session.Platform+" is not supported.")
		return
	}
	if session.Platform == "twitter" && !session.XAppMode.Valid {
		persisted, persistErr := h.queries.SetConnectSessionXAppModeIfNull(r.Context(), db.SetConnectSessionXAppModeIfNullParams{
			ID:       session.ID,
			XAppMode: resolved.xAppMode,
		})
		if persistErr == pgx.ErrNoRows {
			persisted, persistErr = h.queries.GetConnectSessionByOAuthState(r.Context(), state)
		}
		if persistErr != nil || persisted.ID != session.ID || !persisted.XAppMode.Valid {
			slog.Error("connect.authorize: persist resolved X app mode", "session_id", session.ID, "workspace_id", workspaceID, "err", persistErr)
			renderConnectError(w, http.StatusInternalServerError, "Failed to preserve platform credentials.")
			return
		}
		session = persisted
		resolved, err = h.resolveConnectorForStoredMode(r.Context(), workspaceID, session.Platform, session.AllowQuickstartCreds, session.XAppMode)
		if err != nil {
			slog.Error("connect.authorize: resolve persisted connector", "platform", session.Platform, "workspace_id", workspaceID, "err", err)
			renderConnectError(w, http.StatusInternalServerError, "Failed to load platform credentials.")
			return
		}
		if !resolved.ok {
			renderConnectError(w, http.StatusBadRequest, "Platform "+session.Platform+" is not supported.")
			return
		}
	}

	authURL, err := resolved.connector.AuthorizeURL(connect.SessionView{
		ID:             session.ID,
		OAuthState:     session.OauthState,
		PKCEVerifier:   session.PkceVerifier.String,
		ExternalUserID: session.ExternalUserID,
	})
	if err != nil {
		slog.Error("connect.authorize: build url", "platform", session.Platform, "err", err)
		renderConnectError(w, http.StatusInternalServerError, "Failed to build authorize URL.")
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

var connectErrorCodePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,64}$`)

func boundedConnectErrorCode(raw, fallback string) string {
	raw = strings.TrimSpace(raw)
	if !connectErrorCodePattern.MatchString(raw) {
		return fallback
	}
	return strings.ToLower(raw)
}

func classifyFacebookConnectFailure(err error) (string, string, map[string]any) {
	code := string(connect.FacebookAuthorizationFailed)
	message := "Hosted Connect failed during authorization."
	metadata := map[string]any{}

	var failure *connect.FacebookConnectFailure
	if !errors.As(err, &failure) {
		return code, message, metadata
	}
	actionablePageFailure := false
	switch failure.Code {
	case connect.FacebookPageNotAvailable:
		actionablePageFailure = true
		code = string(connect.FacebookPageNotAvailable)
		message = "Hosted Connect failed: no manageable Facebook Page was found."
		metadata["page_count"] = failure.PageCount
		metadata["publishable_page_count"] = failure.PublishablePageCount
	case connect.FacebookPagePermissionRequired:
		actionablePageFailure = true
		code = string(connect.FacebookPagePermissionRequired)
		message = "Hosted Connect failed: Facebook Page publishing permission is required."
		metadata["page_count"] = failure.PageCount
		metadata["publishable_page_count"] = failure.PublishablePageCount
	case connect.FacebookAuthorizationFailed:
		// Keep the safe default.
	}
	if !actionablePageFailure && failure.Stage != "" {
		metadata["facebook_stage"] = failure.Stage
	}
	if !actionablePageFailure && failure.RemoteStatusCode != 0 {
		metadata["remote_status_code"] = failure.RemoteStatusCode
	}
	if !actionablePageFailure && failure.MetaCode != 0 {
		metadata["meta_error_code"] = failure.MetaCode
	}
	if !actionablePageFailure && failure.MetaSubcode != 0 {
		metadata["meta_error_subcode"] = failure.MetaSubcode
	}
	return code, message, metadata
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
		if state != "" {
			if sess, err := h.queries.GetConnectSessionByOAuthState(r.Context(), state); err == nil {
				managedProfile, profileErr := h.queries.GetProfile(r.Context(), sess.ProfileID)
				if profileErr != nil || strings.TrimSpace(managedProfile.WorkspaceID) == "" {
					slog.Error("connect.callback: workspace resolution failed", "platform", sess.Platform, "error_class", "workspace_resolution_failed")
					renderConnectError(w, http.StatusInternalServerError, "Failed to verify workspace.")
					return
				}
				outcome := newHostedConnectOutcome(r.Context(), h.ilog, hostedConnectAttempt{
					WorkspaceID:    managedProfile.WorkspaceID,
					ProfileID:      sess.ProfileID,
					Platform:       sess.Platform,
					SessionID:      sess.ID,
					ExternalUserID: sess.ExternalUserID,
					RequestID:      appmw.GetRequestID(r.Context()),
				})
				w = wrapHostedConnectResponse(w, outcome)
				if errParam == "access_denied" {
					outcome.Cancel("access_denied")
					_, _ = h.queries.MarkConnectSessionCancelled(r.Context(), sess.ID)
					publicReason := "access_denied"
					if sess.Platform != "facebook" && strings.TrimSpace(errDesc) != "" {
						publicReason = errDesc
					}
					h.redirectWithStatus(w, r, sess.ReturnUrl.String, "cancelled", publicReason, false)
					return
				}
				reason := boundedConnectErrorCode(errParam, "authorization_failed")
				if sess.Platform == "facebook" {
					reason = string(connect.FacebookAuthorizationFailed)
				}
				outcome.Fail(reason, "Hosted Connect failed during authorization.", nil)
				publicReason := reason
				if sess.Platform != "facebook" && strings.TrimSpace(errDesc) != "" {
					publicReason = errDesc
				}
				h.redirectWithStatus(w, r, sess.ReturnUrl.String, "error", publicReason, false)
				return
			}
		}
		renderConnectError(w, http.StatusOK, "Connection failed.")
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
	managedProfile, err := h.queries.GetProfile(r.Context(), session.ProfileID)
	if err != nil || strings.TrimSpace(managedProfile.WorkspaceID) == "" {
		slog.Error("connect.callback: workspace resolution failed", "platform", platformName, "error_class", "workspace_resolution_failed")
		renderConnectError(w, http.StatusInternalServerError, "Failed to verify workspace.")
		return
	}
	workspaceID := managedProfile.WorkspaceID
	outcome := newHostedConnectOutcome(r.Context(), h.ilog, hostedConnectAttempt{
		WorkspaceID:    workspaceID,
		ProfileID:      session.ProfileID,
		Platform:       session.Platform,
		SessionID:      session.ID,
		ExternalUserID: session.ExternalUserID,
		RequestID:      appmw.GetRequestID(r.Context()),
	})
	w = wrapHostedConnectResponse(w, outcome)
	if session.Platform != platformName {
		outcome.Fail("platform_mismatch", "Hosted Connect failed during authorization.", nil)
		renderConnectError(w, http.StatusBadRequest, "Platform mismatch.")
		return
	}
	if session.Status != "pending" {
		outcome.Fail("connect_session_not_pending", "Hosted Connect failed because the Session is no longer pending.", nil)
		renderConnectError(w, http.StatusConflict, "This Connect link has already been used.")
		return
	}
	if session.ExpiresAt.Time.Before(time.Now()) {
		_ = h.queries.ExpireConnectSession(r.Context(), session.ID)
		outcome.Fail("connect_session_expired", "Hosted Connect failed because the Session expired.", nil)
		renderConnectError(w, http.StatusGone, "This Connect link has expired.")
		return
	}
	resolved, err := h.resolveConnectorForStoredMode(r.Context(), workspaceID, platformName, session.AllowQuickstartCreds, session.XAppMode)
	if err != nil {
		slog.Error("connect.callback: resolve connector failed", "platform", platformName, "workspace_id", workspaceID, "error_class", "connector_resolution_failed")
		outcome.Fail("connector_resolution_failed", "Hosted Connect failed during authorization.", nil)
		h.redirectWithStatus(w, r, session.ReturnUrl.String, "error", "connector_resolution_failed", false)
		return
	}
	if !resolved.ok {
		outcome.Fail("connector_resolution_failed", "Hosted Connect failed during authorization.", nil)
		renderConnectError(w, http.StatusBadRequest, "Platform not supported.")
		return
	}

	view := connect.SessionView{
		ID:             session.ID,
		OAuthState:     session.OauthState,
		PKCEVerifier:   session.PkceVerifier.String,
		ExternalUserID: session.ExternalUserID,
	}

	tokens, err := resolved.connector.ExchangeCode(r.Context(), view, code)
	if err != nil {
		slog.Error("connect.callback: token exchange failed", "platform", platformName, "error_class", "token_exchange_failed")
		reason := "token_exchange_failed"
		message := "Hosted Connect failed during authorization."
		var metadata map[string]any
		if platformName == "facebook" {
			reason, message, metadata = classifyFacebookConnectFailure(err)
		}
		outcome.Fail(reason, message, metadata)
		h.redirectWithStatus(w, r, session.ReturnUrl.String, "error", reason, false)
		return
	}
	profile, err := resolved.connector.FetchProfile(r.Context(), tokens.AccessToken)
	if err != nil {
		slog.Error("connect.callback: profile fetch failed", "platform", platformName, "error_class", "profile_fetch_failed")
		reason := "profile_fetch_failed"
		message := "Hosted Connect failed during authorization."
		var metadata map[string]any
		if platformName == "facebook" {
			reason, message, metadata = classifyFacebookConnectFailure(err)
		}
		outcome.Fail(reason, message, metadata)
		h.redirectWithStatus(w, r, session.ReturnUrl.String, "error", reason, false)
		return
	}
	providerIdentity, hasProviderIdentity := verifiedOAuthProviderIdentity(platformName, profile)
	if !hasProviderIdentity {
		slog.Error("connect.callback: verified provider identity missing", "platform", platformName, "workspace_id", workspaceID)
		outcome.Fail("provider_identity_missing", "Hosted Connect failed during authorization.", nil)
		renderConnectError(w, http.StatusBadGateway, "Provider account identity is missing.")
		return
	}
	if workspaceID == "" || h.ownershipStore == nil {
		slog.Error("connect.callback: ownership store unavailable", "platform", platformName, "workspace_id", workspaceID)
		outcome.Fail("account_ownership_failed", "Hosted Connect failed while verifying account ownership.", nil)
		renderConnectError(w, http.StatusInternalServerError, "Failed to verify account ownership.")
		return
	}
	ownershipKey := connectownership.OwnershipKey{
		WorkspaceID:      workspaceID,
		ProfileID:        session.ProfileID,
		Platform:         platformName,
		ProviderIdentity: providerIdentity,
		ExternalUserID:   session.ExternalUserID,
	}
	ownershipDecision, ownershipErr := h.ownershipStore.Check(r.Context(), ownershipKey)
	if ownershipErr != nil {
		slog.Error("connect.callback: ownership check failed", "platform", platformName, "workspace_id", workspaceID, "error_class", "ownership_check_failed")
		outcome.Fail("account_ownership_failed", "Hosted Connect failed while verifying account ownership.", nil)
		renderConnectError(w, http.StatusInternalServerError, "Failed to verify account ownership.")
		return
	}
	if ownershipDecision.Kind == connectownership.Conflict {
		logManagedOwnershipConflict(workspaceID, platformName, ownershipDecision, nil)
		outcome.Fail("account_ownership_conflict", "Hosted Connect failed while verifying account ownership.", nil)
		renderConnectError(w, http.StatusConflict, "This social account is already connected and cannot be reassigned.")
		return
	}

	hidePoweredBy := false
	if sub, subErr := h.queries.GetSubscriptionByWorkspace(r.Context(), workspaceID); subErr == nil {
		hidePoweredBy = planAllowsHidePoweredBy(sub.PlanID) && managedProfile.BrandingHidePoweredBy
	}
	if blocked, msg, limitErr := h.freePlanManagedConnectBlocked(r.Context(), workspaceID, session.ExternalUserID, platformName, ownershipDecision.Kind == connectownership.Create); limitErr != nil {
		slog.Error("connect.callback: managed connect limit check failed", "workspace_id", workspaceID, "profile_id", session.ProfileID, "platform", platformName, "error_class", "managed_account_limit_check_failed")
		outcome.Fail("managed_account_limit_check_failed", "Hosted Connect failed while checking workspace limits.", nil)
		renderConnectError(w, http.StatusInternalServerError, "Failed to check plan limits.")
		return
	} else if blocked {
		outcome.Fail("managed_account_limit_reached", "Hosted Connect failed because the workspace account limit was reached.", nil)
		renderConnectError(w, http.StatusPaymentRequired, msg)
		return
	}
	if blocked, shareErr := freePlanManagedSharingBlocked(r.Context(), h.queries, h.superAdminChecker, workspaceID, platformName, providerIdentity); shareErr != nil {
		slog.Error("connect.callback: managed sharing check failed", "workspace_id", workspaceID, "platform", platformName, "error_class", "managed_sharing_lookup_failed")
		outcome.Fail("account_availability_check_failed", "Hosted Connect failed because the account is unavailable to this workspace.", nil)
		renderConnectError(w, http.StatusInternalServerError, "Failed to verify account availability.")
		return
	} else if blocked {
		outcome.Fail("account_unavailable", "Hosted Connect failed because the account is unavailable to this workspace.", nil)
		renderConnectError(w, http.StatusConflict, accountNotAvailableOnFreePlanMessage)
		return
	}

	encAccess, err := h.encryptor.Encrypt(tokens.AccessToken)
	if err != nil {
		outcome.Fail("credential_encryption_failed", "Hosted Connect failed while securing credentials.", nil)
		renderConnectError(w, http.StatusInternalServerError, "Internal error encrypting token.")
		return
	}
	encRefresh := ""
	if tokens.RefreshToken != "" {
		encRefresh, err = h.encryptor.Encrypt(tokens.RefreshToken)
		if err != nil {
			outcome.Fail("credential_encryption_failed", "Hosted Connect failed while securing credentials.", nil)
			renderConnectError(w, http.StatusInternalServerError, "Internal error encrypting token.")
			return
		}
	}

	accountMetadata := map[string]any{
		"username":     profile.Username,
		"display_name": profile.DisplayName,
	}
	if platformName == "instagram" {
		accountMetadata["instagram_webhook_user_id"] = providerIdentity
	}
	metadata, _ := json.Marshal(accountMetadata)

	accountName := pgtype.Text{String: nonEmpty(profile.Username, profile.DisplayName), Valid: profile.Username != "" || profile.DisplayName != ""}
	accountAvatarURL := pgtype.Text{String: profile.AvatarURL, Valid: profile.AvatarURL != ""}
	refreshToken := pgtype.Text{String: encRefresh, Valid: encRefresh != ""}
	tokenExpiresAt := pgtype.Timestamptz{Time: tokens.ExpiresAt, Valid: !tokens.ExpiresAt.IsZero()}
	connectSessionID := pgtype.Text{String: session.ID, Valid: true}
	externalUserID := pgtype.Text{String: session.ExternalUserID, Valid: true}

	refreshParams := db.RefreshConnectedSocialAccountParams{
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
		XAppMode:          resolved.xAppMode,
	}
	upsertParams := db.UpsertManagedSocialAccountParams{
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
		XAppMode:          resolved.xAppMode,
	}
	var saved db.SocialAccount
	if h.connections != nil {
		saved, err = h.connections.SaveVerified(r.Context(), socialconnections.SaveManagedReuse, socialconnections.CredentialInput{
			WorkspaceID: workspaceID, ProfileID: session.ProfileID, Platform: platformName,
			ProviderIdentity: providerIdentity, ExternalAccountID: profile.ExternalAccountID,
			AccessToken: encAccess, RefreshToken: encRefresh, TokenExpiresAt: tokens.ExpiresAt,
			AccountName: accountName.String, AvatarURL: profile.AvatarURL, Metadata: metadata,
			Scopes: tokens.Scopes, XAppMode: resolved.xAppMode.String, ConnectSessionID: session.ID,
			ReconnectAccountID: session.ReconnectAccountID.String,
			Ownership: socialconnections.Ownership{
				ConnectionType: "managed", ExternalUserID: session.ExternalUserID,
				ExternalUserEmail: session.ExternalUserEmail.String,
			},
		})
	} else {
		saved, err = h.ownershipStore.Save(r.Context(), connectownership.SaveRequest{
			WorkspaceID: workspaceID, ProfileID: session.ProfileID, Platform: platformName,
			ProviderIdentity: providerIdentity, ExternalUserID: session.ExternalUserID,
			Refresh: refreshParams, Upsert: upsertParams,
		})
	}
	if err != nil {
		if errors.Is(err, connectownership.ErrOwnershipConflict) || errors.Is(err, socialconnections.ErrOwnershipConflict) {
			logManagedOwnershipConflict(workspaceID, platformName, ownershipDecision, err)
			outcome.Fail("account_ownership_conflict", "Hosted Connect failed while verifying account ownership.", nil)
			renderConnectError(w, http.StatusConflict, "This social account is already connected and cannot be reassigned.")
			return
		}
		if errors.Is(err, connectownership.ErrInvalidOwnershipRequest) {
			slog.Error("connect.callback: invalid ownership save request", "platform", platformName, "workspace_id", workspaceID, "error_class", "invalid_ownership_request")
			outcome.Fail("account_save_failed", "Hosted Connect failed while saving the account.", nil)
			renderConnectError(w, http.StatusInternalServerError, "Failed to save account.")
			return
		}
		slog.Error("connect.callback: save failed", "platform", platformName, "error_class", "account_save_failed")
		outcome.Fail("account_save_failed", "Hosted Connect failed while saving the account.", nil)
		renderConnectError(w, http.StatusInternalServerError, "Failed to save account.")
		return
	}

	if platformName == "instagram" {
		if err := h.instagramWebhookSubscriber.Subscribe(r.Context(), providerIdentity, tokens.AccessToken); err != nil {
			reconnectRows, reconnectErr := h.queries.MarkSocialAccountReconnectRequired(r.Context(), saved.ID)
			failureReason := "webhook_subscription_failed"
			failureMessage := "Instagram webhook subscription failed."
			containmentConfirmed := reconnectErr == nil && reconnectRows == 1
			if !containmentConfirmed {
				failureReason = "webhook_subscription_containment_failed"
				failureMessage = "Instagram webhook subscription failed and reconnect containment could not be confirmed."
				slog.Error("connect.callback: instagram webhook subscription containment failed",
					"account_id", saved.ID,
					"rows_affected", reconnectRows,
					"transition_error", reconnectErr != nil)
			}
			slog.Error("connect.callback: instagram webhook subscription failed",
				"account_id", saved.ID,
				"containment_confirmed", containmentConfirmed)
			outcome.Fail(failureReason, failureMessage, map[string]any{"social_account_id": saved.ID})
			h.redirectWithStatus(w, r, session.ReturnUrl.String, "error", failureReason, hidePoweredBy)
			return
		}
	}

	// Save has already committed. If the completion claim fails, a retry is a
	// same-owner reconnect through the ownership store; only the successful
	// pending -> completed claimant may publish or return success.
	if completionErr := claimConnectSessionCompletion(r.Context(), h.queries, session.ID, saved.ID); completionErr != nil {
		if errors.Is(completionErr, pgx.ErrNoRows) {
			slog.Warn("connect.callback: completion claim lost", "workspace_id", workspaceID, "platform", platformName, "error_class", "completion_claim_lost")
			outcome.Fail("connect_session_not_pending", "Hosted Connect failed because the Session is no longer pending.", nil)
			renderConnectError(w, http.StatusConflict, "This Connect link has already been used.")
			return
		}
		slog.Error("connect.callback: completion claim failed", "workspace_id", workspaceID, "platform", platformName, "error_class", "completion_claim_failed")
		outcome.Fail("session_completion_failed", "Hosted Connect failed while completing the session.", nil)
		renderConnectError(w, http.StatusInternalServerError, "Failed to complete connection.")
		return
	}

	h.bus.Publish(r.Context(), workspaceID, events.EventAccountConnected, map[string]any{
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
	outcome.Success(saved.ID, profile.Username)
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
			if presentation, ok := presentationForConnectReason(reason); ok {
				renderConnectErrorPresentation(w, http.StatusBadRequest, presentation, hidePoweredBy)
			} else {
				renderConnectError(w, http.StatusBadRequest, "Connection failed: "+reason, hidePoweredBy)
			}
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

const (
	facebookPageNotAvailableMessage       = "We couldn’t find a Facebook Page this account can manage or has allowed UniPost to access. Make sure this Facebook account manages a Page and that UniPost is allowed to access it, or ask a Page admin to grant you access. Then open the original connection link and try again."
	facebookPagePermissionRequiredMessage = "Your Facebook account can access a Page, but it doesn’t have permission to publish content. Ask a Page admin to grant you Facebook content-management access, then open the original connection link and try again."
	facebookAuthorizationFailedMessage    = "Facebook authorization couldn’t be completed. Please try again later or contact the developer who sent you the link."
)

type connectErrorPresentation struct {
	Title string
	Body  string
}

func presentationForConnectReason(reason string) (connectErrorPresentation, bool) {
	switch reason {
	case string(connect.FacebookPageNotAvailable):
		return connectErrorPresentation{Title: "Facebook Page unavailable", Body: facebookPageNotAvailableMessage}, true
	case string(connect.FacebookPagePermissionRequired):
		return connectErrorPresentation{Title: "Facebook Page permission required", Body: facebookPagePermissionRequiredMessage}, true
	case string(connect.FacebookAuthorizationFailed):
		return connectErrorPresentation{Title: "Connection failed", Body: facebookAuthorizationFailedMessage}, true
	default:
		return connectErrorPresentation{}, false
	}
}

const connectErrorTplSrc = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}} · UniPost</title>
<style>
body{font-family:system-ui,-apple-system,sans-serif;max-width:480px;margin:48px auto;padding:0 24px;color:#111;line-height:1.5}
h1{font-size:22px;margin-bottom:8px}
.err{background:#fef2f2;border:1px solid #fecaca;color:#991b1b;padding:12px 16px;border-radius:8px;margin:16px 0}
.small{font-size:13px;color:#666;margin-top:24px}
</style>
</head><body>
<h1>{{.Title}}</h1>
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
	hideBranding := len(hidePoweredBy) > 0 && hidePoweredBy[0]
	renderConnectErrorPresentation(w, status, connectErrorPresentation{Title: "Connect", Body: msg}, hideBranding)
}

func renderConnectErrorPresentation(w http.ResponseWriter, status int, presentation connectErrorPresentation, hidePoweredBy bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := connectErrorTpl.Execute(w, map[string]any{
		"Title":         presentation.Title,
		"Message":       presentation.Body,
		"ShowPoweredBy": !hidePoweredBy,
	}); err != nil {
		fmt.Fprintf(w, "render error: %v", err)
	}
}

func renderConnectSuccess(w http.ResponseWriter, hidePoweredBy bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := connectSuccessTpl.Execute(w, map[string]any{"ShowPoweredBy": !hidePoweredBy}); err != nil {
		fmt.Fprintf(w, "render error: %v", err)
	}
}
