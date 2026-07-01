package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

type tiktokProfileResponse struct {
	SocialAccountID string    `json:"social_account_id"`
	Platform        string    `json:"platform"`
	OpenID          string    `json:"open_id"`
	DisplayName     string    `json:"display_name"`
	AvatarURL       string    `json:"avatar_url"`
	Username        string    `json:"username"`
	ProfileWebLink  string    `json:"profile_web_link"`
	ProfileDeepLink string    `json:"profile_deep_link"`
	BioDescription  string    `json:"bio_description"`
	IsVerified      bool      `json:"is_verified"`
	FetchedAt       time.Time `json:"fetched_at"`
}

type tiktokVideosResponse struct {
	Videos    []platform.TikTokVideo `json:"videos"`
	Cursor    int64                  `json:"cursor"`
	HasMore   bool                   `json:"has_more"`
	FetchedAt time.Time              `json:"fetched_at"`
}

func (h *SocialAccountHandler) TikTokProfile(w http.ResponseWriter, r *http.Request) {
	acc, tiktokAdapter, accessToken, ok := h.loadTikTokForAnalytics(w, r)
	if !ok {
		return
	}

	profile, err := tiktokAdapter.FetchProfile(r.Context(), accessToken)
	if err != nil {
		slog.Warn("tiktok profile: upstream fetch failed", "account_id", acc.ID, "error", err)
		writeTikTokAnalyticsError(w, err)
		return
	}

	writeSuccess(w, tiktokProfileResponse{
		SocialAccountID: acc.ID,
		Platform:        acc.Platform,
		OpenID:          profile.OpenID,
		DisplayName:     profile.DisplayName,
		AvatarURL:       profile.AvatarURL,
		Username:        profile.Username,
		ProfileWebLink:  profile.ProfileWebLink,
		ProfileDeepLink: profile.ProfileDeepLink,
		BioDescription:  profile.BioDescription,
		IsVerified:      profile.IsVerified,
		FetchedAt:       time.Now().UTC(),
	})
}

func (h *SocialAccountHandler) TikTokVideos(w http.ResponseWriter, r *http.Request) {
	_, tiktokAdapter, accessToken, ok := h.loadTikTokForAnalytics(w, r)
	if !ok {
		return
	}

	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	videos, err := tiktokAdapter.ListVideos(r.Context(), accessToken, cursor, limit)
	if err != nil {
		slog.Warn("tiktok videos: upstream fetch failed", "error", err)
		writeTikTokAnalyticsError(w, err)
		return
	}

	writeSuccess(w, tiktokVideosResponse{
		Videos:    videos.Videos,
		Cursor:    videos.Cursor,
		HasMore:   videos.HasMore,
		FetchedAt: time.Now().UTC(),
	})
}

func (h *SocialAccountHandler) loadTikTokForAnalytics(w http.ResponseWriter, r *http.Request) (*db.SocialAccount, *platform.TikTokAdapter, string, bool) {
	accountID := accountIDFromRequest(r)
	acc, ok := h.loadAccountForRequest(r, accountID)
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Account not found")
		return nil, nil, "", false
	}
	if acc.Platform != "tiktok" {
		writeError(w, http.StatusConflict, "WRONG_PLATFORM", "Account is not a TikTok account")
		return nil, nil, "", false
	}

	adapter, err := platform.Get("tiktok")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "TikTok adapter unavailable")
		return nil, nil, "", false
	}
	tiktokAdapter, ok := adapter.(*platform.TikTokAdapter)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "TikTok adapter unavailable")
		return nil, nil, "", false
	}

	accessToken, err := h.encryptor.Decrypt(acc.AccessToken)
	if err != nil || strings.TrimSpace(accessToken) == "" {
		slog.Warn("tiktok analytics: decrypt access token failed", "account_id", acc.ID, "error", err)
		writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "Your TikTok connection has expired. Please reconnect the account.")
		return nil, nil, "", false
	}
	return acc, tiktokAdapter, accessToken, true
}

func accountIDFromRequest(r *http.Request) string {
	accountID := chiURLParam(r, "id")
	if accountID == "" {
		accountID = chiURLParam(r, "accountID")
	}
	return accountID
}

func chiURLParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}

func writeTikTokAnalyticsError(w http.ResponseWriter, err error) {
	if looksLikeTikTokAuthError(err) || looksLikeTikTokMissingScopeError(err) {
		writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "Reconnect TikTok to enable analytics.")
		return
	}
	writeError(w, http.StatusBadGateway, "TIKTOK_ERROR", err.Error())
}

func looksLikeTikTokMissingScopeError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "missing") && strings.Contains(msg, "scope")
}
