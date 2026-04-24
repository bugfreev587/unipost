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
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/connect"
	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
)

// ConnectCallbackHandler owns the OAuth dance for managed accounts.
// It depends on the connect.Registry (populated at startup with the
// Twitter + LinkedIn connectors) plus the standard db / encryptor /
// event bus trio.
type ConnectCallbackHandler struct {
	queries   *db.Queries
	encryptor *crypto.AESEncryptor
	bus       events.EventBus
	registry  *connect.Registry
	limiter   *ipLimiter // shared in-memory limiter for callback brute-force protection
}

func NewConnectCallbackHandler(queries *db.Queries, encryptor *crypto.AESEncryptor, bus events.EventBus, registry *connect.Registry) *ConnectCallbackHandler {
	if bus == nil {
		bus = events.NoopBus{}
	}
	return &ConnectCallbackHandler{
		queries:   queries,
		encryptor: encryptor,
		bus:       bus,
		registry:  registry,
		limiter:   newIPLimiter(60, time.Minute),
	}
}

// Authorize handles GET /v1/public/connect/sessions/{id}/authorize.
//
// The hosted dashboard page calls this when the user clicks
// "Authorize with Twitter / LinkedIn". We look up the session by
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

	connector, ok := h.registry.Get(session.Platform)
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
				h.redirectWithStatus(w, r, sess.ReturnUrl.String, status, reason)
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

	connector, ok := h.registry.Get(platformName)
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
		h.redirectWithStatus(w, r, session.ReturnUrl.String, "error", "token_exchange_failed")
		return
	}
	profile, err := connector.FetchProfile(r.Context(), tokens.AccessToken)
	if err != nil {
		slog.Error("connect.callback: profile fetch failed", "platform", platformName, "err", err)
		h.redirectWithStatus(w, r, session.ReturnUrl.String, "error", "profile_fetch_failed")
		return
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

	saved, err := h.queries.UpsertManagedSocialAccount(r.Context(), db.UpsertManagedSocialAccountParams{
		ProfileID:         session.ProfileID,
		Platform:          platformName,
		AccessToken:       encAccess,
		RefreshToken:      pgtype.Text{String: encRefresh, Valid: encRefresh != ""},
		TokenExpiresAt:    pgtype.Timestamptz{Time: tokens.ExpiresAt, Valid: !tokens.ExpiresAt.IsZero()},
		ExternalAccountID: profile.ExternalAccountID,
		AccountName:       pgtype.Text{String: nonEmpty(profile.Username, profile.DisplayName), Valid: profile.Username != "" || profile.DisplayName != ""},
		AccountAvatarUrl:  pgtype.Text{String: profile.AvatarURL, Valid: profile.AvatarURL != ""},
		Metadata:          metadata,
		Scope:             tokens.Scopes,
		ConnectSessionID:  pgtype.Text{String: session.ID, Valid: true},
		ExternalUserID:    pgtype.Text{String: session.ExternalUserID, Valid: true},
		ExternalUserEmail: session.ExternalUserEmail,
	})
	if err != nil {
		slog.Error("connect.callback: upsert failed", "platform", platformName, "err", err)
		renderConnectError(w, http.StatusInternalServerError, "Failed to save account.")
		return
	}

	_, _ = h.queries.MarkConnectSessionCompleted(r.Context(), db.MarkConnectSessionCompletedParams{
		ID:                       session.ID,
		CompletedSocialAccountID: pgtype.Text{String: saved.ID, Valid: true},
	})

	// Webhooks are workspace-scoped; resolve workspace_id from profile.
	wsID := session.ProfileID
	if prof, pErr := h.queries.GetProfile(r.Context(), session.ProfileID); pErr == nil {
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

	h.redirectWithStatus(w, r, session.ReturnUrl.String, "success", "")
}

// redirectWithStatus 302s to return_url with ?connect_status=... +
// optional ?reason=... query params. When return_url is empty we
// render the success / error page in place.
func (h *ConnectCallbackHandler) redirectWithStatus(w http.ResponseWriter, r *http.Request, returnURL, status, reason string) {
	if returnURL == "" {
		switch status {
		case "success":
			renderConnectSuccess(w)
		case "cancelled":
			renderConnectError(w, http.StatusOK, "Connection cancelled.")
		default:
			renderConnectError(w, http.StatusBadRequest, "Connection failed: "+reason)
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
<div class="err">{{.}}</div>
<p>If you reached this page by mistake, contact the developer who sent you the link.</p>
<p class="small">Powered by UniPost</p>
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
<p class="small">Powered by UniPost</p>
</body></html>`

var (
	connectErrorTpl   = template.Must(template.New("connect_error").Parse(connectErrorTplSrc))
	connectSuccessTpl = template.Must(template.New("connect_success").Parse(connectSuccessTplSrc))
)

func renderConnectError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := connectErrorTpl.Execute(w, msg); err != nil {
		fmt.Fprintf(w, "render error: %v", err)
	}
}

func renderConnectSuccess(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := connectSuccessTpl.Execute(w, nil); err != nil {
		fmt.Fprintf(w, "render error: %v", err)
	}
}
