// social_account_health.go is the Sprint 2 W7 endpoint:
//
//   GET /v1/social-accounts/{id}/health
//
// Cheap derivation from existing tables — no active probing, no new
// background workers. Status is computed from the most recent N=10
// social_post_results rows for the account:
//
//   - all 10 successful → ok
//   - any of the 10 failed → degraded (with last_error pointing at
//     the most recent failure)
//   - account row marked disconnected → disconnected (overrides)
//
// Sprint 3 will add active token-expiry probing and a richer
// "warning" tier — for now we have ok / degraded / disconnected.

package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/jackc/pgx/v5"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

// healthRecentResultLimit is how many recent results we look at to
// derive "degraded" status. Small enough to be cheap, large enough
// that one stale failure rolls off after a few successful posts.
const healthRecentResultLimit = 10

type accountHealthLastError struct {
	Code       string    `json:"code"`
	Message    string    `json:"message"`
	OccurredAt time.Time `json:"occurred_at"`
}

type accountHealthResponse struct {
	SocialAccountID      string                  `json:"social_account_id"`
	Platform             string                  `json:"platform"`
	Status               string                  `json:"status"`
	LastSuccessfulPostAt *time.Time              `json:"last_successful_post_at,omitempty"`
	LastError            *accountHealthLastError `json:"last_error,omitempty"`
	TokenExpiresAt       *time.Time              `json:"token_expires_at,omitempty"`
}

// AccountHealth handles GET /v1/social-accounts/{id}/health.
// Workspace-scoped — refuses to expose another workspace's account.
func (h *SocialAccountHandler) AccountHealth(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	accountID := chi.URLParam(r, "id")
	if accountID == "" {
		accountID = chi.URLParam(r, "accountID")
	}

	acc, err := h.queries.GetSocialAccountByIDAndWorkspace(r.Context(), db.GetSocialAccountByIDAndWorkspaceParams{
		ID:          accountID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Account not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load account")
		return
	}

	resp := accountHealthResponse{
		SocialAccountID: acc.ID,
		Platform:        acc.Platform,
		Status:          "ok",
	}
	if acc.TokenExpiresAt.Valid {
		t := acc.TokenExpiresAt.Time
		resp.TokenExpiresAt = &t
	}

	// Disconnected wins over everything else — even if the last 10
	// posts succeeded, if the account is currently disconnected the
	// caller can't post to it right now.
	if acc.DisconnectedAt.Valid {
		resp.Status = "disconnected"
		writeSuccess(w, resp)
		return
	}

	// Walk the last N results to derive ok / degraded.
	results, err := h.queries.ListRecentResultsByAccount(r.Context(), db.ListRecentResultsByAccountParams{
		SocialAccountID: acc.ID,
		Limit:           healthRecentResultLimit,
	})
	if err != nil {
		// Don't fail the whole endpoint just because the results
		// query hiccupped — return the static fields with a generic
		// "ok" so the dashboard always has something to render.
		writeSuccess(w, resp)
		return
	}

	for _, res := range results {
		switch res.Status {
		case "published":
			if res.PublishedAt.Valid {
				if resp.LastSuccessfulPostAt == nil || res.PublishedAt.Time.After(*resp.LastSuccessfulPostAt) {
					t := res.PublishedAt.Time
					resp.LastSuccessfulPostAt = &t
				}
			}
		case "failed":
			if resp.LastError == nil ||
				(res.PublishedAt.Valid && res.PublishedAt.Time.After(resp.LastError.OccurredAt)) {
				msg := ""
				if res.ErrorMessage.Valid {
					msg = res.ErrorMessage.String
				}
				occurred := time.Time{}
				if res.PublishedAt.Valid {
					occurred = res.PublishedAt.Time
				}
				resp.LastError = &accountHealthLastError{
					Code:       categorizeAccountError(msg),
					Message:    msg,
					OccurredAt: occurred,
				}
			}
			resp.Status = "degraded"
		}
	}

	writeSuccess(w, resp)
}

// categorizeAccountError walks the platform's free-form error message
// and tags it with a coarse category code so the dashboard can match
// against a fixed enum instead of substring searching at render time.
// Patterns lifted from the Sprint 1 dashboard categorizeError logic.
func categorizeAccountError(msg string) string {
	lower := lowerASCII(msg)
	switch {
	case contains(lower, "disconnect"), contains(lower, "not found"):
		return "account_disconnected"
	case contains(lower, "token") && (contains(lower, "expired") || contains(lower, "invalid") || contains(lower, "revoked")):
		return "token_expired"
	case contains(lower, "rate limit"), contains(lower, "too many requests"), contains(lower, "429"):
		return "rate_limited"
	case contains(lower, "media") && contains(lower, "size"):
		return "media_too_large"
	case contains(lower, "url_ownership_unverified"):
		return "url_unverified"
	}
	return "unknown"
}

// lowerASCII / contains avoid bringing in the strings package solely
// for two calls. Tiny ASCII helpers.
func lowerASCII(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		out[i] = c
	}
	return string(out)
}

func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
