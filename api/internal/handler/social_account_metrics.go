// social_account_metrics.go is GET /v1/accounts/{id}/metrics —
// follower / following / post counts for one connected social
// account. Modeled on social_account_health.go (also workspace-
// scoped, no probing during the call), but unlike health this
// endpoint DOES hit the platform's API every time so the numbers
// are fresh — the alternative is caching, which is overkill for a
// page customers open occasionally.
//
// Coverage in v1 started with X / Twitter and has expanded to platforms
// that implement platform.AccountMetricsAdapter. Unsupported platforms
// return 501 NOT_SUPPORTED so callers can branch on platform without
// parsing error strings.

package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

type accountMetricsResponse struct {
	SocialAccountID  string         `json:"social_account_id"`
	Platform         string         `json:"platform"`
	FollowerCount    int64          `json:"follower_count"`
	FollowingCount   int64          `json:"following_count"`
	PostCount        int64          `json:"post_count"`
	PlatformSpecific map[string]any `json:"platform_specific,omitempty"`
	FetchedAt        time.Time      `json:"fetched_at"`
}

// AccountMetrics handles GET /v1/accounts/{id}/metrics.
// Workspace-scoped; refuses to expose another workspace's account.
func (h *SocialAccountHandler) AccountMetrics(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	accountID := chi.URLParam(r, "id")
	if accountID == "" {
		// Profile-nested path uses {accountID} instead of {id}.
		accountID = chi.URLParam(r, "accountID")
	}
	if accountID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Missing account id")
		return
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

	if acc.DisconnectedAt.Valid {
		writeError(w, http.StatusConflict, "ACCOUNT_DISCONNECTED",
			"Account is disconnected — reconnect before fetching metrics")
		return
	}
	if acc.Platform == "tiktok" && !tiktokAnalyticsScopesEnabled(r) {
		writeError(w, http.StatusForbidden, "FEATURE_DISABLED", "TikTok analytics is not enabled in this environment.")
		return
	}

	adapter, err := platform.Get(acc.Platform)
	if err != nil {
		writeError(w, http.StatusNotImplemented, "NOT_SUPPORTED",
			"Platform "+acc.Platform+" is not registered on this server")
		return
	}

	metricsAdapter, ok := adapter.(platform.AccountMetricsAdapter)
	if !ok {
		writeError(w, http.StatusNotImplemented, "NOT_SUPPORTED",
			"Account metrics are not available for "+acc.Platform+" yet")
		return
	}

	accessToken, decErr := h.encryptor.Decrypt(acc.AccessToken)
	if decErr != nil {
		slog.Warn("account metrics: decrypt token failed",
			"account_id", acc.ID, "platform", acc.Platform, "err", decErr)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to decrypt token")
		return
	}

	metrics, err := metricsAdapter.GetAccountMetrics(r.Context(), accessToken, acc.ExternalAccountID)
	if err != nil {
		if acc.Platform == "tiktok" && (looksLikeTikTokAuthError(err) || looksLikeTikTokMissingScopeError(err)) {
			writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "Reconnect TikTok to enable analytics.")
			return
		}
		if (acc.Platform == "instagram" || acc.Platform == "threads") && looksLikeMetaAuthOrScopeError(err) {
			writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "Reconnect "+acc.Platform+" to enable analytics.")
			return
		}
		// Bubble up upstream errors as 502 — the request was valid,
		// the upstream just couldn't fulfill it. Distinguishes from
		// our own 5xx so customers know it's not a UniPost bug.
		slog.Warn("account metrics: platform fetch failed",
			"account_id", acc.ID,
			"platform", acc.Platform,
			"external_account_id", acc.ExternalAccountID,
			"err", err)
		writeError(w, http.StatusBadGateway, "UPSTREAM_ERROR",
			"Failed to fetch account metrics from "+acc.Platform)
		return
	}

	writeSuccess(w, accountMetricsResponse{
		SocialAccountID:  acc.ID,
		Platform:         acc.Platform,
		FollowerCount:    metrics.FollowerCount,
		FollowingCount:   metrics.FollowingCount,
		PostCount:        metrics.PostCount,
		PlatformSpecific: metrics.PlatformSpecific,
		FetchedAt:        time.Now().UTC(),
	})
}
