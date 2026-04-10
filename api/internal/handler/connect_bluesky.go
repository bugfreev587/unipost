// connect_bluesky.go is the Sprint 3 PR5 Bluesky Connect surface.
//
// One endpoint:
//
//	POST /v1/public/connect/sessions/{id}/bluesky?state=<oauth_state>
//
// The hosted dashboard page (PR6) renders an HTML form whose action
// targets this endpoint. The form is submitted as native
// application/x-www-form-urlencoded — never as JSON via fetch — so
// the app password never lives in dashboard JS where DevTools could
// inspect it.
//
// On valid credentials we run BlueskyAdapter.Connect (the existing
// BYO path), encrypt the resulting accessJwt + refreshJwt, upsert
// into social_accounts as a managed row, mark the connect_session
// completed, fire the account.connected webhook, and 302 the user
// to the customer's return_url.
//
// On invalid credentials or session errors we render a server-side
// HTML page that includes the form again (handle pre-filled, password
// blank) plus an inline error. No JSON, no fetch, no client-side
// state.
//
// Per Sprint 3 decision #1 the upsert is keyed on
// (profile_id, external_account_id) for Bluesky — the DID/handle —
// because the same external_user_id may legitimately map to multiple
// Bluesky handles (different aliases for one human).

package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

// ConnectBlueskyHandler owns the public Bluesky form endpoint. It
// shares the database + encryption + event bus dependencies with
// the rest of the handler package.
type ConnectBlueskyHandler struct {
	queries   *db.Queries
	encryptor *crypto.AESEncryptor
	bus       events.EventBus
	limiter   *ipLimiter
}

func NewConnectBlueskyHandler(queries *db.Queries, encryptor *crypto.AESEncryptor, bus events.EventBus) *ConnectBlueskyHandler {
	if bus == nil {
		bus = events.NoopBus{}
	}
	return &ConnectBlueskyHandler{
		queries:   queries,
		encryptor: encryptor,
		bus:       bus,
		limiter:   newIPLimiter(10, time.Minute),
	}
}

// SubmitForm handles POST /v1/public/connect/sessions/{id}/bluesky.
//
// Auth model: no API key. The session id in the URL plus the
// `state` query param (which must equal connect_sessions.oauth_state)
// is the bearer. Wrong/expired state → render an error page, never
// reveal whether the session existed.
func (h *ConnectBlueskyHandler) SubmitForm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")

	// Per-IP rate limit. Bluesky's session endpoint is documented at
	// ~30 req/5min for unauthenticated calls; capping us well below
	// keeps a brute-forcer from blowing through the upstream limit.
	if !h.limiter.Allow(clientIP(r)) {
		w.WriteHeader(http.StatusTooManyRequests)
		renderBlueskyResult(w, blueskyTplData{
			Error: "Too many attempts — please wait a minute and try again.",
		})
		return
	}

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		renderBlueskyResult(w, blueskyTplData{Error: "Invalid form submission."})
		return
	}

	sessionID := chi.URLParam(r, "id")
	state := r.URL.Query().Get("state")
	handle := strings.TrimSpace(r.FormValue("handle"))
	appPassword := r.FormValue("app_password") // never trim — Bluesky tokens may not have leading/trailing spaces but we don't normalise

	if sessionID == "" || state == "" || handle == "" || appPassword == "" {
		w.WriteHeader(http.StatusBadRequest)
		renderBlueskyResult(w, blueskyTplData{
			Handle:    handle,
			SessionID: sessionID,
			State:     state,
			Error:     "Both handle and app password are required.",
		})
		return
	}

	// Look up the session by oauth_state. The id-vs-row check below
	// guards against state-id mismatch. Wrong state → 404 framing
	// to avoid leaking session existence.
	session, err := h.queries.GetConnectSessionByOAuthState(r.Context(), state)
	if err != nil || session.ID != sessionID {
		w.WriteHeader(http.StatusNotFound)
		renderBlueskyResult(w, blueskyTplData{Error: "This Connect link is invalid or has expired."})
		return
	}
	if session.Platform != "bluesky" {
		w.WriteHeader(http.StatusBadRequest)
		renderBlueskyResult(w, blueskyTplData{Error: "This Connect link is for a different platform."})
		return
	}
	if session.Status != "pending" {
		w.WriteHeader(http.StatusConflict)
		renderBlueskyResult(w, blueskyTplData{Error: "This Connect link has already been used or has expired."})
		return
	}
	if session.ExpiresAt.Time.Before(time.Now()) {
		_ = h.queries.ExpireConnectSession(r.Context(), session.ID)
		w.WriteHeader(http.StatusGone)
		renderBlueskyResult(w, blueskyTplData{Error: "This Connect link has expired."})
		return
	}

	// Validate credentials with Bluesky. We reuse the existing
	// BlueskyAdapter.Connect path which does the createSession call
	// and returns JWT + refresh JWT. Storing JWTs (rather than the
	// raw app password) keeps Bluesky managed accounts on the same
	// refresh-on-expiry rails as BYO Bluesky — no special-case in
	// the post pipeline. The trade-off is that if the Bluesky JWT
	// chain ever breaks the user has to reconnect; we accept that
	// for Sprint 3 because the chain typically lasts months.
	adapter, _ := platform.Get("bluesky")
	connectResult, err := adapter.Connect(r.Context(), map[string]string{
		"handle":       handle,
		"app_password": appPassword,
	})
	if err != nil {
		// Don't echo the platform error verbatim — it can leak
		// "user not found" vs "wrong password" which we don't want
		// to expose. Generic message keeps the brute-force surface
		// flat.
		w.WriteHeader(http.StatusUnauthorized)
		renderBlueskyResult(w, blueskyTplData{
			Handle:    handle,
			SessionID: sessionID,
			State:     state,
			Error:     "Bluesky rejected those credentials. Double-check your handle and app password.",
		})
		return
	}

	encAccess, err := h.encryptor.Encrypt(connectResult.AccessToken)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		renderBlueskyResult(w, blueskyTplData{Error: "Internal error encrypting credentials."})
		return
	}
	encRefresh, err := h.encryptor.Encrypt(connectResult.RefreshToken)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		renderBlueskyResult(w, blueskyTplData{Error: "Internal error encrypting credentials."})
		return
	}

	metadataJSON, _ := json.Marshal(connectResult.Metadata)

	// Bluesky upsert path: look up by (profile_id, external_account_id).
	// If a row exists for this DID we update in place so historical
	// post_results FKs stay intact (Sprint 3 decision #1). Otherwise
	// insert a fresh managed row.
	existing, lookupErr := h.queries.GetManagedBlueskyAccount(r.Context(), db.GetManagedBlueskyAccountParams{
		ProfileID:         session.ProfileID,
		ExternalAccountID: connectResult.ExternalAccountID,
	})

	var savedID string
	if lookupErr == nil {
		// Refresh in place. Note we DO update the JWT (access_token)
		// here even though UpdateManagedBlueskyAccount's signature is
		// "access_token" — for Bluesky that's the JWT.
		updated, err := h.queries.UpdateManagedBlueskyAccount(r.Context(), db.UpdateManagedBlueskyAccountParams{
			ID:                existing.ID,
			AccessToken:       encAccess,
			AccountName:       pgtype.Text{String: connectResult.AccountName, Valid: connectResult.AccountName != ""},
			AccountAvatarUrl:  pgtype.Text{String: connectResult.AvatarURL, Valid: connectResult.AvatarURL != ""},
			ExternalUserID:    pgtype.Text{String: session.ExternalUserID, Valid: true},
			ExternalUserEmail: session.ExternalUserEmail,
			ConnectSessionID:  pgtype.Text{String: session.ID, Valid: true},
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			renderBlueskyResult(w, blueskyTplData{Error: "Internal error saving account."})
			return
		}
		// Re-stash the refresh JWT alongside the access JWT. Reuse
		// the existing token-update query so we don't need a new
		// sqlc method just for Bluesky.
		_ = h.queries.UpdateSocialAccountTokens(r.Context(), db.UpdateSocialAccountTokensParams{
			ID:             updated.ID,
			AccessToken:    encAccess,
			RefreshToken:   pgtype.Text{String: encRefresh, Valid: encRefresh != ""},
			TokenExpiresAt: pgtype.Timestamptz{Time: connectResult.TokenExpiresAt, Valid: !connectResult.TokenExpiresAt.IsZero()},
		})
		savedID = updated.ID
	} else {
		// Fresh insert. Mirrors the BYO Connect handler with the
		// managed-flow extras populated.
		created, err := h.queries.UpsertManagedSocialAccount(r.Context(), db.UpsertManagedSocialAccountParams{
			ProfileID:         session.ProfileID,
			Platform:          "bluesky",
			AccessToken:       encAccess,
			RefreshToken:      pgtype.Text{String: encRefresh, Valid: encRefresh != ""},
			TokenExpiresAt:    pgtype.Timestamptz{Time: connectResult.TokenExpiresAt, Valid: !connectResult.TokenExpiresAt.IsZero()},
			ExternalAccountID: connectResult.ExternalAccountID,
			AccountName:       pgtype.Text{String: connectResult.AccountName, Valid: connectResult.AccountName != ""},
			AccountAvatarUrl:  pgtype.Text{String: connectResult.AvatarURL, Valid: connectResult.AvatarURL != ""},
			Metadata:          metadataJSON,
			Scope:             nil,
			ConnectSessionID:  pgtype.Text{String: session.ID, Valid: true},
			ExternalUserID:    pgtype.Text{String: session.ExternalUserID, Valid: true},
			ExternalUserEmail: session.ExternalUserEmail,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			renderBlueskyResult(w, blueskyTplData{Error: "Internal error saving account."})
			return
		}
		savedID = created.ID
	}

	// Mark the session completed and link to the saved account.
	_, _ = h.queries.MarkConnectSessionCompleted(r.Context(), db.MarkConnectSessionCompletedParams{
		ID: session.ID,
		CompletedSocialAccountID: pgtype.Text{String: savedID, Valid: true},
	})

	// Fire account.connected webhook. Best-effort — never blocks.
	// Webhooks are workspace-scoped; resolve workspace_id from profile.
	wsID := session.ProfileID
	if prof, pErr := h.queries.GetProfile(r.Context(), session.ProfileID); pErr == nil {
		wsID = prof.WorkspaceID
	}
	h.bus.Publish(r.Context(), wsID, events.EventAccountConnected, map[string]any{
		"social_account_id": savedID,
		"platform":          "bluesky",
		"account_name":      connectResult.AccountName,
		"external_user_id":  session.ExternalUserID,
		"connection_type":   "managed",
	})

	// Redirect to return_url with success marker.
	returnURL := ""
	if session.ReturnUrl.Valid {
		returnURL = session.ReturnUrl.String
	}
	if returnURL == "" {
		// No return URL — render the success page in-place.
		renderBlueskyResult(w, blueskyTplData{Success: true, AccountName: connectResult.AccountName})
		return
	}
	sep := "?"
	if strings.Contains(returnURL, "?") {
		sep = "&"
	}
	http.Redirect(w, r, returnURL+sep+"connect_status=success", http.StatusFound)
}

// blueskyTplData is the payload for the result page template.
type blueskyTplData struct {
	Success     bool
	AccountName string
	Error       string
	Handle      string
	SessionID   string
	State       string
}

// blueskyResultTplSrc is the server-rendered HTML returned when we
// can't / don't want to redirect. The form is repeated so users can
// retry without bouncing back to the dashboard. Password field is
// always blank — never echo a credential.
const blueskyResultTplSrc = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Connect Bluesky · UniPost</title>
<style>
  body{font-family:system-ui,-apple-system,sans-serif;max-width:480px;margin:48px auto;padding:0 24px;color:#111;line-height:1.5}
  h1{font-size:22px;margin-bottom:8px}
  p{color:#444}
  .err{background:#fef2f2;border:1px solid #fecaca;color:#991b1b;padding:12px 16px;border-radius:8px;margin:16px 0}
  .ok{background:#f0fdf4;border:1px solid #bbf7d0;color:#166534;padding:12px 16px;border-radius:8px;margin:16px 0}
  label{display:block;margin-top:16px;font-size:14px;font-weight:500}
  input{width:100%;padding:10px 12px;border:1px solid #d1d5db;border-radius:6px;font-size:15px;margin-top:4px;box-sizing:border-box}
  button{margin-top:20px;width:100%;background:#111;color:#fff;border:0;padding:12px;border-radius:6px;font-size:15px;cursor:pointer}
  .small{font-size:13px;color:#666;margin-top:8px}
  .small a{color:#2563eb}
</style>
</head><body>
{{if .Success}}
  <h1>✓ Connected</h1>
  <div class="ok">Your Bluesky account <strong>{{.AccountName}}</strong> is now connected. You can close this window.</div>
{{else}}
  <h1>Connect Bluesky</h1>
  <p>Enter your Bluesky handle and an <strong>app password</strong> created at
  <a href="https://bsky.app/settings/app-passwords" target="_blank" rel="noopener">bsky.app/settings/app-passwords</a>.
  Do <strong>not</strong> use your main account password.</p>
  {{if .Error}}<div class="err">{{.Error}}</div>{{end}}
  <form method="POST" action="/v1/public/connect/sessions/{{.SessionID}}/bluesky?state={{.State}}">
    <label>Handle
      <input name="handle" type="text" placeholder="you.bsky.social" value="{{.Handle}}" autocapitalize="off" autocorrect="off" required>
    </label>
    <label>App password
      <input name="app_password" type="password" placeholder="xxxx-xxxx-xxxx-xxxx" required>
    </label>
    <button type="submit">Connect</button>
  </form>
  <p class="small">Powered by UniPost</p>
{{end}}
</body></html>`

var blueskyResultTpl = template.Must(template.New("bluesky_result").Parse(blueskyResultTplSrc))

func renderBlueskyResult(w http.ResponseWriter, data blueskyTplData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := blueskyResultTpl.Execute(w, data); err != nil {
		// Last-resort plaintext fallback — should never trigger.
		fmt.Fprintf(w, "render error: %v", err)
	}
}

// clientIP extracts a best-effort client IP for rate-limit keying.
// Trusts X-Forwarded-For only if it looks present; otherwise falls
// back to RemoteAddr (which on Railway is the proxy, so all requests
// would share a bucket — XFF is the right header behind their LB).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first hop (the original client).
		if comma := strings.Index(xff, ","); comma > 0 {
			return strings.TrimSpace(xff[:comma])
		}
		return strings.TrimSpace(xff)
	}
	return r.RemoteAddr
}

// ── Tiny in-memory IP rate limiter ──────────────────────────────
//
// Sliding 1-minute bucket per IP. Not distributed-safe — fine for
// a single Railway instance, and for Sprint 3 there's no separate
// rate-limit need elsewhere. If we ever scale to multiple API
// instances we'll swap this for a Redis token bucket.

type ipLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time
}

func newIPLimiter(limit int, window time.Duration) *ipLimiter {
	return &ipLimiter{
		limit:  limit,
		window: window,
		hits:   make(map[string][]time.Time),
	}
}

func (l *ipLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-l.window)

	hits := l.hits[ip]
	// Drop expired entries.
	keep := hits[:0]
	for _, t := range hits {
		if t.After(cutoff) {
			keep = append(keep, t)
		}
	}
	if len(keep) >= l.limit {
		l.hits[ip] = keep
		return false
	}
	keep = append(keep, now)
	l.hits[ip] = keep

	// Opportunistic GC: every ~1000 inserts, prune empty IPs to
	// keep the map from leaking memory in long-running processes.
	if len(l.hits) > 1000 {
		for k, v := range l.hits {
			if len(v) == 0 {
				delete(l.hits, k)
			}
		}
	}
	return true
}
