// tiktok_creator_info.go surfaces TikTok's creator_info query endpoint to the
// compose UI, which is required for the Content Posting API audit. The UI
// uses the returned metadata to:
//
//   - show the creator's nickname above the compose form
//   - populate the privacy dropdown from privacy_level_options (no default
//     selection — the user must pick)
//   - grey out Comment / Duet / Stitch toggles when the creator disabled
//     them in TikTok's own settings
//   - reject videos longer than max_video_post_duration_sec before upload
//
// Two routes hit this handler:
//
//	GET /v1/profiles/{profileID}/social-accounts/{accountID}/tiktok/creator-info
//	GET /v1/social-accounts/{id}/tiktok/creator-info
//
// The first is dashboard (Clerk session); the second is API-key. Both resolve
// ownership via the same helpers other TikTok/account routes use.

package handler

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

type tiktokCreatorInfoResponse struct {
	CreatorAvatarURL        string   `json:"creator_avatar_url"`
	CreatorUsername         string   `json:"creator_username"`
	CreatorNickname         string   `json:"creator_nickname"`
	PrivacyLevelOptions     []string `json:"privacy_level_options"`
	CommentDisabled         bool     `json:"comment_disabled"`
	DuetDisabled            bool     `json:"duet_disabled"`
	StitchDisabled          bool     `json:"stitch_disabled"`
	MaxVideoPostDurationSec int      `json:"max_video_post_duration_sec"`
}

// TikTokCreatorInfo handles GET .../social-accounts/{id}/tiktok/creator-info.
// Returns 409 when the account isn't on TikTok so the dashboard can surface
// a clear "wrong platform" error instead of an opaque 500.
func (h *SocialAccountHandler) TikTokCreatorInfo(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "id")
	if accountID == "" {
		accountID = chi.URLParam(r, "accountID")
	}

	acc, ok := h.loadAccountForRequest(r, accountID)
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Account not found")
		return
	}
	if acc.Platform != "tiktok" {
		writeError(w, http.StatusConflict, "WRONG_PLATFORM", "Account is not a TikTok account")
		return
	}

	adapter, err := platform.Get("tiktok")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "TikTok adapter unavailable")
		return
	}
	tiktokAdapter, ok := adapter.(*platform.TikTokAdapter)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "TikTok adapter unavailable")
		return
	}

	accessToken, err := h.encryptor.Decrypt(acc.AccessToken)
	if err != nil {
		slog.Error("tiktok creator_info: decrypt access token failed", "account_id", acc.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to decrypt access token")
		return
	}

	// If the stored access token is already empty — typically the
	// fallout from an old bug that wrote empty tokens to the DB after
	// a failed refresh — treat it as a reconnect case immediately so
	// the user sees a clear message instead of a TikTok 401.
	if accessToken == "" {
		slog.Warn("tiktok creator_info: stored access token is empty; user must reconnect", "account_id", acc.ID)
		writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "Your TikTok connection has expired. Please reconnect the account.")
		return
	}

	// Refresh expired tokens inline so the caller doesn't get a 401
	// from TikTok and have to retry. Mirrors the pattern in
	// social_posts.go dispatchOne — every step must succeed before we
	// overwrite the stored tokens, and the user is steered toward
	// reconnect whenever TikTok won't cooperate.
	if acc.TokenExpiresAt.Valid && acc.TokenExpiresAt.Time.Before(time.Now()) && acc.RefreshToken.Valid {
		refreshTok, decErr := h.encryptor.Decrypt(acc.RefreshToken.String)
		if decErr != nil {
			slog.Error("tiktok creator_info: decrypt refresh token failed", "account_id", acc.ID, "error", decErr)
			writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "Your TikTok connection has expired. Please reconnect the account.")
			return
		}
		newAccess, newRefresh, expiresAt, refErr := tiktokAdapter.RefreshToken(r.Context(), refreshTok)
		if refErr != nil || newAccess == "" {
			slog.Warn("tiktok creator_info: refresh failed; steering to reconnect", "account_id", acc.ID, "error", refErr)
			writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "Your TikTok connection has expired. Please reconnect the account.")
			return
		}
		encAccess, encErr := h.encryptor.Encrypt(newAccess)
		encRefresh, encErr2 := h.encryptor.Encrypt(newRefresh)
		if encErr != nil || encErr2 != nil {
			slog.Error("tiktok creator_info: encrypt refreshed tokens failed", "account_id", acc.ID, "access_err", encErr, "refresh_err", encErr2)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to persist refreshed tokens")
			return
		}
		accessToken = newAccess
		if updateErr := h.queries.UpdateSocialAccountTokens(r.Context(), db.UpdateSocialAccountTokensParams{
			ID:             acc.ID,
			AccessToken:    encAccess,
			RefreshToken:   pgtype.Text{String: encRefresh, Valid: true},
			TokenExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
		}); updateErr != nil {
			slog.Error("tiktok creator_info: update tokens failed", "account_id", acc.ID, "error", updateErr)
		}
	}

	info, err := tiktokAdapter.FetchCreatorInfo(r.Context(), accessToken)
	if err != nil {
		// Auth-shaped errors → reconnect message. Anything else is an
		// upstream TikTok problem the user can't fix themselves.
		slog.Warn("tiktok creator_info: upstream fetch failed", "account_id", acc.ID, "error", err)
		if looksLikeTikTokAuthError(err) {
			writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "TikTok rejected your credentials. Please reconnect the account.")
			return
		}
		writeError(w, http.StatusBadGateway, "TIKTOK_ERROR", err.Error())
		return
	}

	writeSuccess(w, tiktokCreatorInfoResponse{
		CreatorAvatarURL:        info.CreatorAvatarURL,
		CreatorUsername:         info.CreatorUsername,
		CreatorNickname:         info.CreatorNickname,
		PrivacyLevelOptions:     info.PrivacyLevelOptions,
		CommentDisabled:         info.CommentDisabled,
		DuetDisabled:            info.DuetDisabled,
		StitchDisabled:          info.StitchDisabled,
		MaxVideoPostDurationSec: info.MaxVideoPostDurationSec,
	})
}

// looksLikeTikTokAuthError substring-matches the error message for signs
// that TikTok rejected our credentials, as opposed to a network blip or
// a transient 5xx. Kept deliberately broad so "access_token_invalid",
// "invalid_token", generic "401", etc. all route to the reconnect path.
func looksLikeTikTokAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "access_token_invalid") ||
		strings.Contains(msg, "invalid_token") ||
		strings.Contains(msg, "invalid_access_token") ||
		strings.Contains(msg, "token_expired") ||
		strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "(401)") ||
		strings.Contains(msg, "(403)")
}

// loadAccountForRequest fetches the account row for the caller, enforcing
// ownership via workspace (API key) or profile (dashboard Clerk) context.
// Returns (nil, false) when the account doesn't exist OR the caller can't
// reach it — we don't distinguish, to avoid leaking account existence across
// workspaces.
func (h *SocialAccountHandler) loadAccountForRequest(r *http.Request, accountID string) (*db.SocialAccount, bool) {
	if workspaceID := auth.GetWorkspaceID(r.Context()); workspaceID != "" {
		acc, err := h.queries.GetSocialAccountByIDAndWorkspace(r.Context(), db.GetSocialAccountByIDAndWorkspaceParams{
			ID:          accountID,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return nil, false
		}
		return &acc, true
	}
	profileID := h.getProfileID(r)
	if profileID == "" {
		return nil, false
	}
	acc, err := h.queries.GetSocialAccountByIDAndProfile(r.Context(), db.GetSocialAccountByIDAndProfileParams{
		ID:        accountID,
		ProfileID: profileID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, false
		}
		return nil, false
	}
	return &acc, true
}
