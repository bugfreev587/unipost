package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

type AnalyticsHandler struct {
	queries   *db.Queries
	encryptor *crypto.AESEncryptor
}

func NewAnalyticsHandler(queries *db.Queries, encryptor *crypto.AESEncryptor) *AnalyticsHandler {
	return &AnalyticsHandler{queries: queries, encryptor: encryptor}
}

type analyticsResponse struct {
	PostID           string         `json:"post_id"`
	SocialAccountID  string         `json:"social_account_id"`
	Platform         string         `json:"platform"`
	ExternalID       string         `json:"external_id"`
	Impressions      int64          `json:"impressions"`
	Reach            int64          `json:"reach"`
	Likes            int64          `json:"likes"`
	Comments         int64          `json:"comments"`
	Shares           int64          `json:"shares"`
	Saves            int64          `json:"saves"`
	Clicks           int64          `json:"clicks"`
	VideoViews       int64          `json:"video_views"`
	Views            int64          `json:"views"` // legacy alias for video_views; phased out in PR 5
	EngagementRate   float64        `json:"engagement_rate"`
	PlatformSpecific map[string]any `json:"platform_specific,omitempty"`
	FetchedAt        string         `json:"fetched_at"`
}

// computeEngagementRate implements the unified PRD §9.1 formula:
// (likes + comments + shares + saves + clicks) / impressions.
// Returns 0 when impressions is 0 (e.g. Bluesky, YouTube, TikTok which don't
// expose impressions). Rounded to 4 decimal places to match the column type.
func computeEngagementRate(m *platform.PostMetrics) float64 {
	if m.Impressions <= 0 {
		return 0
	}
	total := m.Likes + m.Comments + m.Shares + m.Saves + m.Clicks
	rate := float64(total) / float64(m.Impressions)
	// Round to 4 decimals.
	return float64(int64(rate*10000+0.5)) / 10000
}

// GetAnalytics handles GET /v1/social-posts/{id}/analytics
// (and the project-scoped /v1/projects/{projectID}/social-posts/{id}/analytics).
//
// Pass ?refresh=1 to bypass the 1-hour cache and force a live fetch from each
// platform. Without it, cached rows are served whenever fresh — the
// AnalyticsRefreshWorker keeps them up to date in the background.
func (h *AnalyticsHandler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	projectID := h.getWorkspaceID(r)
	postID := chi.URLParam(r, "id")
	if postID == "" {
		postID = chi.URLParam(r, "postID")
	}
	forceRefresh := r.URL.Query().Get("refresh") == "1"

	post, err := h.queries.GetSocialPostByIDAndWorkspace(r.Context(), db.GetSocialPostByIDAndWorkspaceParams{
		ID: postID, WorkspaceID: projectID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Post not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get post")
		return
	}

	results, _ := h.queries.ListSocialPostResultsByPost(r.Context(), post.ID)

	var analytics []analyticsResponse
	for _, res := range results {
		if !res.ExternalID.Valid {
			continue
		}

		// Check cache (1 hour) unless forceRefresh is set.
		cached, err := h.queries.GetPostAnalytics(r.Context(), res.ID)
		if !forceRefresh && err == nil && cached.FetchedAt.Time.After(time.Now().Add(-1*time.Hour)) {
			var ps map[string]any
			if len(cached.PlatformSpecific) > 0 {
				_ = json.Unmarshal(cached.PlatformSpecific, &ps)
			}
			analytics = append(analytics, analyticsResponse{
				PostID:           post.ID,
				SocialAccountID:  res.SocialAccountID,
				ExternalID:       res.ExternalID.String,
				Impressions:      cached.Impressions.Int64,
				Reach:            cached.Reach.Int64,
				Likes:            cached.Likes.Int64,
				Comments:         cached.Comments.Int64,
				Shares:           cached.Shares.Int64,
				Saves:            cached.Saves.Int64,
				Clicks:           cached.Clicks.Int64,
				VideoViews:       cached.VideoViews.Int64,
				Views:            cached.Views.Int64,
				EngagementRate:   float64FromNumeric(cached.EngagementRate),
				PlatformSpecific: ps,
				FetchedAt:        cached.FetchedAt.Time.Format(time.RFC3339),
			})
			continue
		}

		// Fetch from platform
		acc, accErr := h.queries.GetSocialAccount(r.Context(), res.SocialAccountID)
		if accErr != nil {
			continue
		}

		adapter, adErr := platform.Get(acc.Platform)
		if adErr != nil {
			continue
		}

		analyticsAdapter, ok := adapter.(platform.AnalyticsAdapter)
		if !ok {
			continue
		}

		accessToken, decErr := h.encryptor.Decrypt(acc.AccessToken)
		if decErr != nil {
			continue
		}

		metrics, metErr := analyticsAdapter.GetAnalytics(r.Context(), accessToken, res.ExternalID.String)
		if metErr != nil {
			continue
		}

		// Compute the unified engagement rate at the call site so all platforms
		// share the same denominator (PRD §9.1). Adapters return 0.
		metrics.EngagementRate = computeEngagementRate(metrics)

		// Cache result
		rawData, _ := json.Marshal(metrics)
		var psBytes []byte
		if metrics.PlatformSpecific != nil {
			psBytes, _ = json.Marshal(metrics.PlatformSpecific)
		}
		h.queries.UpsertPostAnalytics(r.Context(), db.UpsertPostAnalyticsParams{
			SocialPostResultID: res.ID,
			Views:              pgtype.Int8{Int64: metrics.Views, Valid: true},
			Likes:              pgtype.Int8{Int64: metrics.Likes, Valid: true},
			Comments:           pgtype.Int8{Int64: metrics.Comments, Valid: true},
			Shares:             pgtype.Int8{Int64: metrics.Shares, Valid: true},
			Reach:              pgtype.Int8{Int64: metrics.Reach, Valid: true},
			Impressions:        pgtype.Int8{Int64: metrics.Impressions, Valid: true},
			Saves:              pgtype.Int8{Int64: metrics.Saves, Valid: true},
			Clicks:             pgtype.Int8{Int64: metrics.Clicks, Valid: true},
			VideoViews:         pgtype.Int8{Int64: metrics.VideoViews, Valid: true},
			PlatformSpecific:   psBytes,
			EngagementRate:     numericFromFloat(metrics.EngagementRate),
			RawData:            rawData,
		})

		analytics = append(analytics, analyticsResponse{
			PostID:           post.ID,
			SocialAccountID:  res.SocialAccountID,
			Platform:         acc.Platform,
			ExternalID:       res.ExternalID.String,
			Impressions:      metrics.Impressions,
			Reach:            metrics.Reach,
			Likes:            metrics.Likes,
			Comments:         metrics.Comments,
			Shares:           metrics.Shares,
			Saves:            metrics.Saves,
			Clicks:           metrics.Clicks,
			VideoViews:       metrics.VideoViews,
			Views:            metrics.Views,
			EngagementRate:   metrics.EngagementRate,
			PlatformSpecific: metrics.PlatformSpecific,
			FetchedAt:        time.Now().Format(time.RFC3339),
		})
	}

	writeSuccess(w, analytics)
}

func (h *AnalyticsHandler) getWorkspaceID(r *http.Request) string {
	if pid := auth.GetWorkspaceID(r.Context()); pid != "" {
		return pid
	}
	workspaceID := chi.URLParam(r, "workspaceID")
	if workspaceID == "" {
		return ""
	}
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		return ""
	}
	_, err := h.queries.GetWorkspaceByIDAndOwner(r.Context(), db.GetWorkspaceByIDAndOwnerParams{
		ID: workspaceID, UserID: userID,
	})
	if err != nil {
		return ""
	}
	return workspaceID
}

func float64FromNumeric(n pgtype.Numeric) float64 {
	f, _ := n.Float64Value()
	return f.Float64
}

func numericFromFloat(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	n.Scan(f)
	return n
}
