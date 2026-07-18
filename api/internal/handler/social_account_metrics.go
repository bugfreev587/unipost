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
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/connect"
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

	if acc.Platform == "tiktok" {
		if status, code, message, reason, blocked := tiktokAnalyticsAccountStateError(&acc); blocked {
			writeErrorWithDetails(w, status, code, message, ErrorDetails{
				Details: map[string]any{"reason": reason},
			})
			return
		}
	} else if acc.DisconnectedAt.Valid {
		writeError(w, http.StatusConflict, "ACCOUNT_DISCONNECTED",
			"Account is disconnected — reconnect before fetching metrics")
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
	if decErr != nil || strings.TrimSpace(accessToken) == "" {
		slog.Warn("account metrics: decrypt token failed",
			"account_id", acc.ID, "platform", acc.Platform, "err", decErr)
		if acc.Platform == "tiktok" {
			writeErrorWithDetails(w, http.StatusConflict, "NEEDS_RECONNECT", "Your TikTok connection has expired. Reconnect the account.", ErrorDetails{
				Details: map[string]any{"reason": platform.TikTokAccountTokenInvalid},
			})
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to decrypt token")
		return
	}

	// Close the race with background token refresh workers: if the token expired
	// between ticks, refresh inline before hitting the provider metrics endpoint.
	if acc.TokenExpiresAt.Valid && acc.TokenExpiresAt.Time.Before(time.Now()) && acc.RefreshToken.Valid && acc.RefreshToken.String != "" {
		if refreshTok, decErr := h.encryptor.Decrypt(acc.RefreshToken.String); decErr == nil {
			var newAccess, newRefresh string
			var expiresAt time.Time
			var refErr error
			if acc.Platform == "twitter" {
				if h.xTokenRefresher == nil {
					refErr = errors.New("X token refresher is not configured")
				} else {
					var tokens *connect.TokenSet
					tokens, refErr = h.xTokenRefresher.Refresh(r.Context(), acc, refreshTok)
					if refErr == nil {
						newAccess, newRefresh, expiresAt = tokens.AccessToken, tokens.RefreshToken, tokens.ExpiresAt
					}
				}
			} else {
				newAccess, newRefresh, expiresAt, refErr = adapter.RefreshToken(r.Context(), refreshTok)
			}
			if refErr != nil {
				slog.Warn("account metrics: token refresh failed",
					"account_id", acc.ID, "platform", acc.Platform, "err", refErr)
			} else if newAccess == "" {
				slog.Warn("account metrics: token refresh returned empty access token",
					"account_id", acc.ID, "platform", acc.Platform)
			} else {
				encAccess, encErr := h.encryptor.Encrypt(newAccess)
				var encRefresh string
				var encRefreshErr error
				if newRefresh != "" {
					encRefresh, encRefreshErr = h.encryptor.Encrypt(newRefresh)
				}
				if encErr != nil || encRefreshErr != nil {
					slog.Error("account metrics: encrypt refreshed tokens failed",
						"account_id", acc.ID, "platform", acc.Platform,
						"access_err", encErr, "refresh_err", encRefreshErr)
				} else {
					accessToken = newAccess
					if updateErr := h.queries.UpdateSocialAccountTokens(r.Context(), db.UpdateSocialAccountTokensParams{
						ID:             acc.ID,
						AccessToken:    encAccess,
						RefreshToken:   accountMetricsRefreshTokenForUpdate(acc.RefreshToken, encRefresh),
						TokenExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: !expiresAt.IsZero()},
					}); updateErr != nil {
						slog.Error("account metrics: update refreshed tokens failed",
							"account_id", acc.ID, "platform", acc.Platform, "err", updateErr)
					}
				}
			}
		} else {
			slog.Warn("account metrics: decrypt refresh token failed",
				"account_id", acc.ID, "platform", acc.Platform, "err", decErr)
		}
	}

	metrics, err := metricsAdapter.GetAccountMetrics(r.Context(), accessToken, acc.ExternalAccountID)
	if err != nil {
		if acc.Platform == "tiktok" {
			writeTikTokAnalyticsError(w, err)
			return
		}
		if status, code, message, ok := accountMetricsPlatformErrorResponse(acc.Platform, err); ok {
			writeError(w, status, code, message)
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

func accountMetricsPlatformErrorResponse(platformName string, err error) (int, string, string, bool) {
	if err == nil {
		return 0, "", "", false
	}
	if errors.Is(err, platform.ErrNeedsReconnect) || errors.Is(err, platform.ErrYouTubeNoChannel) {
		return accountMetricsNeedsReconnectResponse(platformName)
	}
	if platformName == "tiktok" && (looksLikeTikTokAuthError(err) || looksLikeTikTokMissingScopeError(err)) {
		return accountMetricsNeedsReconnectResponse(platformName)
	}
	if (platformName == "instagram" || platformName == "threads") && looksLikeMetaAuthOrScopeError(err) {
		return accountMetricsNeedsReconnectResponse(platformName)
	}
	return 0, "", "", false
}

func accountMetricsNeedsReconnectResponse(platformName string) (int, string, string, bool) {
	return http.StatusConflict, "NEEDS_RECONNECT", "Reconnect " + accountMetricsPlatformDisplayName(platformName) + " to enable analytics.", true
}

func accountMetricsPlatformDisplayName(platformName string) string {
	switch platformName {
	case "youtube":
		return "YouTube"
	case "tiktok":
		return "TikTok"
	case "instagram":
		return "Instagram"
	case "threads":
		return "Threads"
	default:
		return platformName
	}
}

func accountMetricsRefreshTokenForUpdate(existing pgtype.Text, encryptedNewRefresh string) pgtype.Text {
	if encryptedNewRefresh == "" {
		return existing
	}
	return pgtype.Text{String: encryptedNewRefresh, Valid: true}
}
