package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/jackc/pgx/v5"
	"github.com/xiaoboyu/unipost-api/internal/auth"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

// healthRecentResultLimit bounds terminal outcome history returned for account health.
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
	results, err := h.queries.ListAccountHealthResults(r.Context(), db.ListAccountHealthResultsParams{
		SocialAccountID: acc.ID,
		Limit:           healthRecentResultLimit,
	})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "HEALTH_UNAVAILABLE", "Failed to load account health")
		return
	}
	applyAccountHealthResults(&resp, results)
	writeSuccess(w, resp)
}

// Results are ordered by the actual terminal outcome time, newest first.
// A newer successful publish resolves earlier errors; historical cancellations
// are excluded by the query and never represent a provider/account failure.
func applyAccountHealthResults(resp *accountHealthResponse, results []db.ListAccountHealthResultsRow) {
	for _, res := range results {
		if res.LastSuccessfulPostAt.Valid {
			t := res.LastSuccessfulPostAt.Time
			resp.LastSuccessfulPostAt = &t
		}
		if res.Status == "published" && res.PublishedAt.Valid {
			if resp.LastSuccessfulPostAt == nil || res.PublishedAt.Time.After(*resp.LastSuccessfulPostAt) {
				t := res.PublishedAt.Time
				resp.LastSuccessfulPostAt = &t
			}
		}
	}
	if len(results) == 0 || results[0].Status != "failed" {
		return
	}
	res := results[0]
	resp.Status = "degraded"
	resp.LastError = &accountHealthLastError{
		Code:       categorizeAccountError(res.ErrorMessage.String),
		Message:    res.ErrorMessage.String,
		OccurredAt: res.OccurredAt.Time,
	}
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
