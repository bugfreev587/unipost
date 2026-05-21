package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/featureflags"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/runtimeenv"
)

type facebookPageAnalyticsResponse struct {
	SocialAccountID   string                              `json:"social_account_id"`
	Platform          string                              `json:"platform"`
	Page              *platform.FacebookPageProfile       `json:"page"`
	Insights          *facebookPageInsightsResponse       `json:"insights,omitempty"`
	InsightsError     string                              `json:"insights_error,omitempty"`
	Posts             []facebookPageAnalyticsPostResponse `json:"posts"`
	FetchedAt         time.Time                           `json:"fetched_at"`
	PostLimit         int                                 `json:"post_limit"`
	GrantedScopes     []string                            `json:"granted_scopes,omitempty"`
	RequiredScopes    []string                            `json:"required_scopes"`
	RecommendedScopes []string                            `json:"recommended_scopes"`
}

type facebookPageAnalyticsPostResponse struct {
	ID                       string `json:"id"`
	Message                  string `json:"message"`
	CreatedTime              string `json:"created_time"`
	PermalinkURL             string `json:"permalink_url"`
	FullPicture              string `json:"full_picture"`
	MediaURL                 string `json:"media_url"`
	MediaType                string `json:"media_type"`
	Likes                    int64  `json:"likes"`
	Comments                 int64  `json:"comments"`
	Shares                   int64  `json:"shares"`
	Clicks                   int64  `json:"clicks"`
	VideoViews               int64  `json:"video_views"`
	EngagementTotal          int64  `json:"engagement_total"`
	MetricsUnavailableReason string `json:"metrics_unavailable_reason,omitempty"`
}

var (
	facebookPageAnalyticsRequiredScopes    = []string{"pages_read_engagement"}
	facebookPageAnalyticsRecommendedScopes = []string{"read_insights"}
)

// FacebookPageAnalytics handles
//
//	GET .../social-accounts/{id}/facebook/page-analytics?days=28&limit=12
//
// It powers the dashboard's Analytics -> Platforms -> Facebook Page page. The
// endpoint intentionally aggregates Page identity, Page-level insights, recent
// published Page posts, and per-post engagement into one token-scoped response
// so the browser never talks to Meta directly.
func (h *SocialAccountHandler) FacebookPageAnalytics(w http.ResponseWriter, r *http.Request) {
	acc, fb, accessToken, ok := h.loadFacebookForAnalytics(w, r)
	if !ok {
		return
	}

	days := parseClampedInt(r.URL.Query().Get("days"), 28, 1, 92)
	limit := parseClampedInt(r.URL.Query().Get("limit"), 12, 1, 50)
	until := time.Now().UTC()
	since := until.Add(-time.Duration(days) * 24 * time.Hour)

	page, err := fb.FetchPageProfile(r.Context(), accessToken, acc.ExternalAccountID)
	if err != nil {
		slog.Warn("facebook page analytics: profile fetch failed", "account_id", acc.ID, "error", err)
		writeFacebookAnalyticsError(w, err)
		return
	}

	var insightsResp *facebookPageInsightsResponse
	insightsErr := ""
	if stats, err := fb.GetPageInsights(r.Context(), accessToken, acc.ExternalAccountID, since, until); err != nil {
		slog.Warn("facebook page analytics: page insights fetch failed", "account_id", acc.ID, "error", err)
		insightsErr = err.Error()
	} else if stats != nil {
		insightsResp = &facebookPageInsightsResponse{
			Follows:             stats.Follows,
			Impressions:         stats.Impressions,
			PostEngagements:     stats.PostEngagements,
			Below100LikesNotice: stats.Below100LikesNotice,
			Since:               since.Format(time.RFC3339),
			Until:               until.Format(time.RFC3339),
		}
	}

	posts, err := fb.FetchPagePosts(r.Context(), accessToken, acc.ExternalAccountID, limit)
	if err != nil {
		slog.Warn("facebook page analytics: page posts fetch failed", "account_id", acc.ID, "error", err)
		writeFacebookAnalyticsError(w, err)
		return
	}

	postRows := make([]facebookPageAnalyticsPostResponse, 0, len(posts))
	for _, post := range posts {
		row := facebookPageAnalyticsPostResponse{
			ID:              post.ID,
			Message:         post.Message,
			CreatedTime:     post.CreatedTime,
			PermalinkURL:    post.PermalinkURL,
			FullPicture:     post.FullPicture,
			MediaURL:        post.MediaURL,
			MediaType:       post.MediaType,
			Likes:           post.Likes,
			Comments:        post.Comments,
			Shares:          post.Shares,
			EngagementTotal: post.Likes + post.Comments + post.Shares,
		}
		if metrics, err := fb.GetAnalytics(r.Context(), accessToken, post.ID); err != nil {
			row.MetricsUnavailableReason = err.Error()
		} else if metrics != nil {
			row.Likes = metrics.Likes
			row.Comments = metrics.Comments
			row.Shares = metrics.Shares
			row.Clicks = metrics.Clicks
			row.VideoViews = metrics.VideoViews
			row.EngagementTotal = metrics.Likes + metrics.Comments + metrics.Shares + metrics.Clicks
		}
		postRows = append(postRows, row)
	}

	writeSuccess(w, facebookPageAnalyticsResponse{
		SocialAccountID:   acc.ID,
		Platform:          acc.Platform,
		Page:              page,
		Insights:          insightsResp,
		InsightsError:     insightsErr,
		Posts:             postRows,
		FetchedAt:         time.Now().UTC(),
		PostLimit:         limit,
		GrantedScopes:     acc.Scope,
		RequiredScopes:    facebookPageAnalyticsRequiredScopes,
		RecommendedScopes: facebookPageAnalyticsRecommendedScopes,
	})
}

func (h *SocialAccountHandler) loadFacebookForAnalytics(w http.ResponseWriter, r *http.Request) (*db.SocialAccount, *platform.FacebookAdapter, string, bool) {
	accountID := accountIDFromRequest(r)
	acc, ok := h.loadAccountForRequest(r, accountID)
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Account not found")
		return nil, nil, "", false
	}
	if acc.Platform != "facebook" {
		writeError(w, http.StatusConflict, "WRONG_PLATFORM", "Account is not a Facebook Page")
		return nil, nil, "", false
	}
	if !facebookPageAnalyticsEnabled(r) {
		writeError(w, http.StatusForbidden, "FEATURE_DISABLED", "Facebook Page analytics is not enabled in this environment.")
		return nil, nil, "", false
	}

	adapter, err := platform.Get("facebook")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Facebook adapter unavailable")
		return nil, nil, "", false
	}
	fb, ok := adapter.(*platform.FacebookAdapter)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Facebook adapter unavailable")
		return nil, nil, "", false
	}

	accessToken, err := h.encryptor.Decrypt(acc.AccessToken)
	if err != nil || accessToken == "" {
		slog.Warn("facebook page analytics: decrypt access token failed", "account_id", acc.ID, "error", err)
		writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "Your Facebook Page connection has expired. Please reconnect the Page.")
		return nil, nil, "", false
	}
	return acc, fb, accessToken, true
}

func facebookPageAnalyticsEnabled(r *http.Request) bool {
	return featureflags.Enabled(r.Context(), featureflags.FacebookPageAnalytics, featureflags.Target{
		UserID:      auth.GetUserID(r.Context()),
		WorkspaceID: auth.GetWorkspaceID(r.Context()),
		Env:         runtimeenv.Current(),
	})
}

func parseClampedInt(raw string, fallback, min, max int) int {
	value := fallback
	if raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			value = parsed
		}
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func writeFacebookAnalyticsError(w http.ResponseWriter, err error) {
	if looksLikeFacebookAuthError(err) {
		writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "Reconnect Facebook Page to enable analytics.")
		return
	}
	writeError(w, http.StatusBadGateway, "FACEBOOK_ERROR", err.Error())
}

func looksLikeFacebookAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return containsAnyFold(msg,
		"access token",
		"invalid oauth",
		"session has expired",
		"permission",
		"requires pages_read_engagement",
		"requires read_insights",
		"code 190",
	)
}

func containsAnyFold(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}
