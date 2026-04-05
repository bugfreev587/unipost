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
	PostID          string  `json:"post_id"`
	SocialAccountID string  `json:"social_account_id"`
	Platform        string  `json:"platform"`
	ExternalID      string  `json:"external_id"`
	Views           int64   `json:"views"`
	Likes           int64   `json:"likes"`
	Comments        int64   `json:"comments"`
	Shares          int64   `json:"shares"`
	Reach           int64   `json:"reach"`
	Impressions     int64   `json:"impressions"`
	EngagementRate  float64 `json:"engagement_rate"`
	FetchedAt       string  `json:"fetched_at"`
}

// GetAnalytics handles GET /v1/social-posts/{id}/analytics
func (h *AnalyticsHandler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	projectID := h.getProjectID(r)
	postID := chi.URLParam(r, "id")
	if postID == "" {
		postID = chi.URLParam(r, "postID")
	}

	post, err := h.queries.GetSocialPostByIDAndProject(r.Context(), db.GetSocialPostByIDAndProjectParams{
		ID: postID, ProjectID: projectID,
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

		// Check cache (1 hour)
		cached, err := h.queries.GetPostAnalytics(r.Context(), res.ID)
		if err == nil && cached.FetchedAt.Time.After(time.Now().Add(-1*time.Hour)) {
			analytics = append(analytics, analyticsResponse{
				PostID:          post.ID,
				SocialAccountID: res.SocialAccountID,
				ExternalID:      res.ExternalID.String,
				Views:           cached.Views.Int64,
				Likes:           cached.Likes.Int64,
				Comments:        cached.Comments.Int64,
				Shares:          cached.Shares.Int64,
				Reach:           cached.Reach.Int64,
				Impressions:     cached.Impressions.Int64,
				EngagementRate:  float64FromNumeric(cached.EngagementRate),
				FetchedAt:       cached.FetchedAt.Time.Format(time.RFC3339),
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

		// Cache result
		rawData, _ := json.Marshal(metrics)
		h.queries.UpsertPostAnalytics(r.Context(), db.UpsertPostAnalyticsParams{
			SocialPostResultID: res.ID,
			Views:              pgtype.Int8{Int64: metrics.Views, Valid: true},
			Likes:              pgtype.Int8{Int64: metrics.Likes, Valid: true},
			Comments:           pgtype.Int8{Int64: metrics.Comments, Valid: true},
			Shares:             pgtype.Int8{Int64: metrics.Shares, Valid: true},
			Reach:              pgtype.Int8{Int64: metrics.Reach, Valid: true},
			Impressions:        pgtype.Int8{Int64: metrics.Impressions, Valid: true},
			EngagementRate:     numericFromFloat(metrics.EngagementRate),
			RawData:            rawData,
		})

		analytics = append(analytics, analyticsResponse{
			PostID:          post.ID,
			SocialAccountID: res.SocialAccountID,
			Platform:        acc.Platform,
			ExternalID:      res.ExternalID.String,
			Views:           metrics.Views,
			Likes:           metrics.Likes,
			Comments:        metrics.Comments,
			Shares:          metrics.Shares,
			Reach:           metrics.Reach,
			Impressions:     metrics.Impressions,
			EngagementRate:  metrics.EngagementRate,
			FetchedAt:       time.Now().Format(time.RFC3339),
		})
	}

	writeSuccess(w, analytics)
}

func (h *AnalyticsHandler) getProjectID(r *http.Request) string {
	if pid := auth.GetProjectID(r.Context()); pid != "" {
		return pid
	}
	projectID := chi.URLParam(r, "projectID")
	if projectID == "" {
		return ""
	}
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		return ""
	}
	_, err := h.queries.GetProjectByIDAndOwner(r.Context(), db.GetProjectByIDAndOwnerParams{
		ID: projectID, OwnerID: userID,
	})
	if err != nil {
		return ""
	}
	return projectID
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
