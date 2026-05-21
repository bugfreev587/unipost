package handler

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

type instagramProfileResponse struct {
	SocialAccountID   string    `json:"social_account_id"`
	Platform          string    `json:"platform"`
	ID                string    `json:"id"`
	Username          string    `json:"username"`
	ProfilePictureURL string    `json:"profile_picture_url"`
	FollowersCount    int64     `json:"followers_count"`
	FollowsCount      int64     `json:"follows_count"`
	MediaCount        int64     `json:"media_count"`
	FetchedAt         time.Time `json:"fetched_at"`
}

type instagramMediaListResponse struct {
	Media     []instagramMediaResponse `json:"media"`
	FetchedAt time.Time                `json:"fetched_at"`
	Limit     int                      `json:"limit"`
}

type instagramMediaResponse struct {
	ID                       string `json:"id"`
	Caption                  string `json:"caption"`
	MediaType                string `json:"media_type"`
	MediaURL                 string `json:"media_url"`
	ThumbnailURL             string `json:"thumbnail_url"`
	Permalink                string `json:"permalink"`
	Timestamp                string `json:"timestamp"`
	LikeCount                int64  `json:"like_count"`
	CommentsCount            int64  `json:"comments_count"`
	Reach                    int64  `json:"reach"`
	Shares                   int64  `json:"shares"`
	Saves                    int64  `json:"saves"`
	MetricsUnavailableReason string `json:"metrics_unavailable_reason,omitempty"`
}

type threadsProfileResponse struct {
	SocialAccountID   string    `json:"social_account_id"`
	Platform          string    `json:"platform"`
	ID                string    `json:"id"`
	Username          string    `json:"username"`
	ProfilePictureURL string    `json:"threads_profile_picture_url"`
	FetchedAt         time.Time `json:"fetched_at"`
}

type threadsPostsResponse struct {
	Posts     []threadsPostResponse `json:"posts"`
	FetchedAt time.Time             `json:"fetched_at"`
	Limit     int                   `json:"limit"`
}

type threadsPostResponse struct {
	ID                       string `json:"id"`
	Text                     string `json:"text"`
	MediaType                string `json:"media_type"`
	MediaURL                 string `json:"media_url"`
	Permalink                string `json:"permalink"`
	Timestamp                string `json:"timestamp"`
	Views                    int64  `json:"views"`
	Likes                    int64  `json:"likes"`
	Replies                  int64  `json:"replies"`
	Reposts                  int64  `json:"reposts"`
	Quotes                   int64  `json:"quotes"`
	Shares                   int64  `json:"shares"`
	MetricsUnavailableReason string `json:"metrics_unavailable_reason,omitempty"`
}

func (h *SocialAccountHandler) InstagramProfile(w http.ResponseWriter, r *http.Request) {
	acc, ig, accessToken, ok := h.loadInstagramForAnalytics(w, r)
	if !ok {
		return
	}

	profile, err := ig.FetchProfile(r.Context(), accessToken)
	if err != nil {
		slog.Warn("instagram profile: upstream fetch failed", "account_id", acc.ID, "error", err)
		writeMetaAnalyticsError(w, "Instagram", err)
		return
	}

	writeSuccess(w, instagramProfileResponse{
		SocialAccountID:   acc.ID,
		Platform:          acc.Platform,
		ID:                profile.ID,
		Username:          profile.Username,
		ProfilePictureURL: profile.ProfilePictureURL,
		FollowersCount:    profile.FollowersCount,
		FollowsCount:      profile.FollowsCount,
		MediaCount:        profile.MediaCount,
		FetchedAt:         time.Now().UTC(),
	})
}

func (h *SocialAccountHandler) InstagramMedia(w http.ResponseWriter, r *http.Request) {
	_, ig, accessToken, ok := h.loadInstagramForAnalytics(w, r)
	if !ok {
		return
	}

	limit := parseClampedInt(r.URL.Query().Get("limit"), 20, 1, 50)
	mediaList, err := ig.ListMedia(r.Context(), accessToken, limit)
	if err != nil {
		slog.Warn("instagram media: upstream list failed", "error", err)
		writeMetaAnalyticsError(w, "Instagram", err)
		return
	}

	rows := make([]instagramMediaResponse, 0, len(mediaList.Media))
	for _, media := range mediaList.Media {
		row := instagramMediaResponse{
			ID:            media.ID,
			Caption:       media.Caption,
			MediaType:     media.MediaType,
			MediaURL:      media.MediaURL,
			ThumbnailURL:  media.ThumbnailURL,
			Permalink:     media.Permalink,
			Timestamp:     media.Timestamp,
			LikeCount:     media.LikeCount,
			CommentsCount: media.CommentsCount,
		}
		if metrics, err := ig.GetAnalytics(r.Context(), accessToken, media.ID); err != nil {
			row.MetricsUnavailableReason = err.Error()
		} else if metrics != nil {
			row.LikeCount = metrics.Likes
			row.CommentsCount = metrics.Comments
			row.Reach = metrics.Reach
			row.Shares = metrics.Shares
			row.Saves = metrics.Saves
		}
		rows = append(rows, row)
	}

	writeSuccess(w, instagramMediaListResponse{
		Media:     rows,
		FetchedAt: time.Now().UTC(),
		Limit:     limit,
	})
}

func (h *SocialAccountHandler) ThreadsProfile(w http.ResponseWriter, r *http.Request) {
	acc, th, accessToken, ok := h.loadThreadsForAnalytics(w, r)
	if !ok {
		return
	}

	profile, err := th.FetchProfile(r.Context(), accessToken)
	if err != nil {
		slog.Warn("threads profile: upstream fetch failed", "account_id", acc.ID, "error", err)
		writeMetaAnalyticsError(w, "Threads", err)
		return
	}

	writeSuccess(w, threadsProfileResponse{
		SocialAccountID:   acc.ID,
		Platform:          acc.Platform,
		ID:                profile.ID,
		Username:          profile.Username,
		ProfilePictureURL: profile.ProfilePictureURL,
		FetchedAt:         time.Now().UTC(),
	})
}

func (h *SocialAccountHandler) ThreadsPosts(w http.ResponseWriter, r *http.Request) {
	_, th, accessToken, ok := h.loadThreadsForAnalytics(w, r)
	if !ok {
		return
	}

	limit := parseClampedInt(r.URL.Query().Get("limit"), 20, 1, 50)
	postList, err := th.ListPosts(r.Context(), accessToken, limit)
	if err != nil {
		slog.Warn("threads posts: upstream list failed", "error", err)
		writeMetaAnalyticsError(w, "Threads", err)
		return
	}

	rows := make([]threadsPostResponse, 0, len(postList.Posts))
	for _, post := range postList.Posts {
		row := threadsPostResponse{
			ID:        post.ID,
			Text:      post.Text,
			MediaType: post.MediaType,
			MediaURL:  post.MediaURL,
			Permalink: post.Permalink,
			Timestamp: post.Timestamp,
		}
		if metrics, err := th.GetAnalytics(r.Context(), accessToken, post.ID); err != nil {
			row.MetricsUnavailableReason = err.Error()
		} else if metrics != nil {
			row.Views = metrics.Impressions
			row.Likes = metrics.Likes
			row.Replies = metrics.Comments
			row.Shares = metrics.Shares
			row.Reposts = int64FromPlatformSpecific(metrics.PlatformSpecific, "reposts")
			row.Quotes = int64FromPlatformSpecific(metrics.PlatformSpecific, "quotes")
		}
		rows = append(rows, row)
	}

	writeSuccess(w, threadsPostsResponse{
		Posts:     rows,
		FetchedAt: time.Now().UTC(),
		Limit:     limit,
	})
}

func (h *SocialAccountHandler) loadInstagramForAnalytics(w http.ResponseWriter, r *http.Request) (*db.SocialAccount, *platform.InstagramAdapter, string, bool) {
	acc, accessToken, ok := h.loadMetaAccountForAnalytics(w, r, "instagram", "Instagram account")
	if !ok {
		return nil, nil, "", false
	}
	adapter, err := platform.Get("instagram")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Instagram adapter unavailable")
		return nil, nil, "", false
	}
	ig, ok := adapter.(*platform.InstagramAdapter)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Instagram adapter unavailable")
		return nil, nil, "", false
	}
	return acc, ig, accessToken, true
}

func (h *SocialAccountHandler) loadThreadsForAnalytics(w http.ResponseWriter, r *http.Request) (*db.SocialAccount, *platform.ThreadsAdapter, string, bool) {
	acc, accessToken, ok := h.loadMetaAccountForAnalytics(w, r, "threads", "Threads profile")
	if !ok {
		return nil, nil, "", false
	}
	adapter, err := platform.Get("threads")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Threads adapter unavailable")
		return nil, nil, "", false
	}
	th, ok := adapter.(*platform.ThreadsAdapter)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Threads adapter unavailable")
		return nil, nil, "", false
	}
	return acc, th, accessToken, true
}

func (h *SocialAccountHandler) loadMetaAccountForAnalytics(w http.ResponseWriter, r *http.Request, platformName string, label string) (*db.SocialAccount, string, bool) {
	accountID := accountIDFromRequest(r)
	acc, ok := h.loadAccountForRequest(r, accountID)
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Account not found")
		return nil, "", false
	}
	if acc.Platform != platformName {
		writeError(w, http.StatusConflict, "WRONG_PLATFORM", "Account is not a "+label)
		return nil, "", false
	}

	accessToken, err := h.encryptor.Decrypt(acc.AccessToken)
	if err != nil || strings.TrimSpace(accessToken) == "" {
		slog.Warn("meta analytics: decrypt access token failed", "account_id", acc.ID, "platform", acc.Platform, "error", err)
		writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "Your "+label+" connection has expired. Please reconnect.")
		return nil, "", false
	}
	return acc, accessToken, true
}

func writeMetaAnalyticsError(w http.ResponseWriter, platformName string, err error) {
	if looksLikeMetaAuthOrScopeError(err) {
		writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "Reconnect "+platformName+" to enable analytics.")
		return
	}
	writeError(w, http.StatusBadGateway, "UPSTREAM_ERROR", err.Error())
}

func looksLikeMetaAuthOrScopeError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return containsAnyFold(msg,
		"access token",
		"invalid oauth",
		"session has expired",
		"permission",
		"scope",
		"code 190",
		"code 10",
	)
}

func int64FromPlatformSpecific(values map[string]any, key string) int64 {
	if values == nil {
		return 0
	}
	switch v := values[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}
