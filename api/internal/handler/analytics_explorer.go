package handler

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/auth"
)

const (
	analyticsPostsDefaultLimit  = 50
	analyticsPostsMaxLimit      = 100
	analyticsPostsExportMaxRows = 1000
	analyticsRefreshMaxRows     = 500
)

type AnalyticsExplorerHandler struct {
	pool *pgxpool.Pool
}

func NewAnalyticsExplorerHandler(pool *pgxpool.Pool) *AnalyticsExplorerHandler {
	return &AnalyticsExplorerHandler{pool: pool}
}

type analyticsPlatformCapability struct {
	Metrics          []string `json:"metrics"`
	RefreshSupported bool     `json:"refresh_supported"`
	Notes            []string `json:"notes,omitempty"`
}

func analyticsMetricNames() []string {
	return []string{
		"impressions",
		"reach",
		"likes",
		"comments",
		"shares",
		"saves",
		"clicks",
		"video_views",
		"engagement_rate",
	}
}

func analyticsPlatformCapabilities() map[string]analyticsPlatformCapability {
	engagement := analyticsMetricNames()
	engagementOnly := []string{"likes", "comments", "shares", "engagement_rate"}
	videoBasic := []string{"views", "likes", "comments", "shares", "video_views", "engagement_rate"}
	return map[string]analyticsPlatformCapability{
		"instagram": {
			Metrics:          engagement,
			RefreshSupported: true,
			Notes:            []string{"Requires Instagram Business insights permissions."},
		},
		"threads": {
			Metrics:          []string{"views", "likes", "comments", "shares", "engagement_rate"},
			RefreshSupported: true,
			Notes:            []string{"Threads exposes post engagement and view-style metrics where available."},
		},
		"pinterest": {
			Metrics:          engagement,
			RefreshSupported: true,
			Notes:            []string{"Pinterest organic analytics may require business account access."},
		},
		"tiktok": {
			Metrics:          videoBasic,
			RefreshSupported: true,
			Notes:            []string{"TikTok analytics requires approved analytics scopes on connected accounts."},
		},
		"facebook": {
			Metrics:          engagement,
			RefreshSupported: true,
		},
		"youtube": {
			Metrics:          videoBasic,
			RefreshSupported: true,
		},
		"twitter": {
			Metrics:          engagementOnly,
			RefreshSupported: true,
		},
		"linkedin": {
			Metrics:          engagementOnly,
			RefreshSupported: true,
		},
		"bluesky": {
			Metrics:          []string{"likes", "comments", "shares", "engagement_rate"},
			RefreshSupported: true,
			Notes:            []string{"Bluesky does not expose impressions or reach."},
		},
	}
}

type analyticsPostSort struct {
	APIName    string
	Expression string
	Direction  string
}

func analyticsPostSortSpec(raw string) (analyticsPostSort, error) {
	key := strings.ToLower(strings.TrimSpace(raw))
	direction := "DESC"
	if key == "" {
		key = "published_at"
	}
	if strings.HasPrefix(key, "-") {
		key = strings.TrimPrefix(key, "-")
		direction = "DESC"
	}
	if strings.HasSuffix(key, "_asc") {
		key = strings.TrimSuffix(key, "_asc")
		direction = "ASC"
	}
	if strings.HasSuffix(key, "_desc") {
		key = strings.TrimSuffix(key, "_desc")
		direction = "DESC"
	}

	expressions := map[string]string{
		"published_at":    "COALESCE(spr.published_at, sp.published_at, sp.created_at)",
		"created_at":      "sp.created_at",
		"impressions":     "COALESCE(pa.impressions, 0)",
		"reach":           "COALESCE(pa.reach, 0)",
		"likes":           "COALESCE(pa.likes, 0)",
		"comments":        "COALESCE(pa.comments, 0)",
		"shares":          "COALESCE(pa.shares, 0)",
		"saves":           "COALESCE(pa.saves, 0)",
		"clicks":          "COALESCE(pa.clicks, 0)",
		"video_views":     "COALESCE(pa.video_views, 0)",
		"engagement_rate": "COALESCE(pa.engagement_rate, 0)",
	}
	expr, ok := expressions[key]
	if !ok {
		return analyticsPostSort{}, fmt.Errorf("unsupported analytics posts sort %q", raw)
	}
	return analyticsPostSort{APIName: key, Expression: expr, Direction: direction}, nil
}

func normalizeAnalyticsPostsLimit(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return analyticsPostsDefaultLimit, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, errors.New("limit must be a positive integer")
	}
	if n <= 0 {
		return 0, errors.New("limit must be greater than 0")
	}
	if n > analyticsPostsMaxLimit {
		return analyticsPostsMaxLimit, nil
	}
	return n, nil
}

func normalizeAnalyticsCursorOffset(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, errors.New("cursor must be a non-negative integer offset")
	}
	if n < 0 {
		return 0, errors.New("cursor must be a non-negative integer offset")
	}
	return n, nil
}

func rollupEngagementRate(impressions, likes, comments, shares, saves, clicks int64) float64 {
	if impressions <= 0 {
		return 0
	}
	total := likes + comments + shares + saves + clicks
	rate := float64(total) / float64(impressions)
	return float64(int64(rate*10000+0.5)) / 10000
}

type analyticsPostFilters struct {
	WorkspaceID string
	From        time.Time
	To          time.Time
	Platform    string
	ProfileID   string
	Status      string
	AccountID   string
	PostID      string
	Limit       int
	Offset      int
	Sort        analyticsPostSort
}

type analyticsPostRow struct {
	PostID              string         `json:"post_id"`
	SocialPostResultID  string         `json:"social_post_result_id"`
	SocialAccountID     string         `json:"social_account_id"`
	ProfileID           string         `json:"profile_id"`
	Platform            string         `json:"platform"`
	ExternalID          string         `json:"external_id,omitempty"`
	ExternalUserID      string         `json:"external_user_id,omitempty"`
	ResultStatus        string         `json:"result_status"`
	PostStatus          string         `json:"post_status"`
	Caption             string         `json:"caption,omitempty"`
	URL                 string         `json:"url,omitempty"`
	CreatedAt           string         `json:"created_at"`
	PublishedAt         string         `json:"published_at,omitempty"`
	Impressions         int64          `json:"impressions"`
	Reach               int64          `json:"reach"`
	Likes               int64          `json:"likes"`
	Comments            int64          `json:"comments"`
	Shares              int64          `json:"shares"`
	Saves               int64          `json:"saves"`
	Clicks              int64          `json:"clicks"`
	VideoViews          int64          `json:"video_views"`
	EngagementRate      float64        `json:"engagement_rate"`
	PlatformSpecific    map[string]any `json:"platform_specific,omitempty"`
	FetchedAt           string         `json:"fetched_at,omitempty"`
	ConsecutiveFailures int32          `json:"consecutive_failures"`
	LastFailureReason   string         `json:"last_failure_reason,omitempty"`
}

type analyticsPlatformAvailability struct {
	Platform              string   `json:"platform"`
	SupportedMetrics      []string `json:"supported_metrics"`
	RefreshSupported      bool     `json:"refresh_supported"`
	AccountCount          int64    `json:"account_count"`
	ActiveAccountCount    int64    `json:"active_account_count"`
	NeedsReconnectCount   int64    `json:"needs_reconnect_count"`
	AnalyticsRowCount     int64    `json:"analytics_row_count"`
	LastSuccessfulFetchAt string   `json:"last_successful_fetch_at,omitempty"`
	LastFailureReason     string   `json:"last_failure_reason,omitempty"`
	Health                string   `json:"health"`
	Notes                 []string `json:"notes,omitempty"`
}

type analyticsPlatformSummary struct {
	Posts          int64   `json:"posts"`
	Accounts       int64   `json:"accounts"`
	Impressions    int64   `json:"impressions"`
	Reach          int64   `json:"reach"`
	Likes          int64   `json:"likes"`
	Comments       int64   `json:"comments"`
	Shares         int64   `json:"shares"`
	Saves          int64   `json:"saves"`
	Clicks         int64   `json:"clicks"`
	VideoViews     int64   `json:"video_views"`
	EngagementRate float64 `json:"engagement_rate"`
}

type analyticsPlatformTrendRow struct {
	Date        string `json:"date"`
	Posts       int64  `json:"posts"`
	Impressions int64  `json:"impressions"`
	Reach       int64  `json:"reach"`
	Likes       int64  `json:"likes"`
	Comments    int64  `json:"comments"`
	Shares      int64  `json:"shares"`
	Saves       int64  `json:"saves"`
	Clicks      int64  `json:"clicks"`
	VideoViews  int64  `json:"video_views"`
}

type analyticsAccountAvailability struct {
	SocialAccountID       string `json:"social_account_id"`
	ProfileID             string `json:"profile_id"`
	AccountName           string `json:"account_name,omitempty"`
	ExternalUserID        string `json:"external_user_id,omitempty"`
	Status                string `json:"status"`
	PostCount             int64  `json:"post_count"`
	LastSuccessfulFetchAt string `json:"last_successful_fetch_at,omitempty"`
	LastFailureReason     string `json:"last_failure_reason,omitempty"`
}

type analyticsPlatformDetail struct {
	Platform     string                         `json:"platform"`
	Period       summaryPeriod                  `json:"period"`
	Availability analyticsPlatformAvailability  `json:"availability"`
	Summary      analyticsPlatformSummary       `json:"summary"`
	Trend        []analyticsPlatformTrendRow    `json:"trend"`
	Accounts     []analyticsAccountAvailability `json:"accounts"`
	TopPosts     []analyticsPostRow             `json:"top_posts"`
}

type analyticsRefreshRequest struct {
	Platform  string `json:"platform"`
	ProfileID string `json:"profile_id"`
	AccountID string `json:"account_id"`
	PostID    string `json:"post_id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Limit     int    `json:"limit"`
}

type analyticsRefreshResponse struct {
	Status         string                  `json:"status"`
	MatchedCount   int64                   `json:"matched_count"`
	RequestedCount int64                   `json:"requested_count"`
	Limit          int                     `json:"limit"`
	ProcessedBy    string                  `json:"processed_by"`
	Filters        analyticsRefreshRequest `json:"filters"`
}

func (h *AnalyticsExplorerHandler) ListPosts(w http.ResponseWriter, r *http.Request) {
	params, ok := h.parsePostFilters(w, r, analyticsPostsMaxLimit)
	if !ok {
		return
	}

	rows, err := h.queryAnalyticsPosts(r.Context(), params)
	if err != nil {
		slog.Error("analytics posts query failed", "workspace_id", params.WorkspaceID, "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load analytics posts")
		return
	}

	hasMore := len(rows) > params.Limit
	if hasMore {
		rows = rows[:params.Limit]
	}
	nextCursor := ""
	if hasMore {
		nextCursor = strconv.Itoa(params.Offset + params.Limit)
	}
	writeSuccessWithCursor(w, rows, nextCursor, hasMore, params.Limit)
}

func (h *AnalyticsExplorerHandler) ExportPostsCSV(w http.ResponseWriter, r *http.Request) {
	params, ok := h.parsePostFilters(w, r, analyticsPostsExportMaxRows)
	if !ok {
		return
	}
	params.Limit = analyticsPostsExportMaxRows
	params.Offset = 0

	rows, err := h.queryAnalyticsPosts(r.Context(), params)
	if err != nil {
		slog.Error("analytics posts export query failed", "workspace_id", params.WorkspaceID, "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to export analytics posts")
		return
	}
	if len(rows) > params.Limit {
		rows = rows[:params.Limit]
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="unipost-analytics-posts.csv"`)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{
		"post_id", "social_post_result_id", "platform", "social_account_id", "profile_id",
		"result_status", "post_status", "external_id", "url", "created_at", "published_at",
		"impressions", "reach", "likes", "comments", "shares", "saves", "clicks",
		"video_views", "engagement_rate", "fetched_at", "last_failure_reason",
	})
	for _, row := range rows {
		_ = writer.Write([]string{
			row.PostID,
			row.SocialPostResultID,
			row.Platform,
			row.SocialAccountID,
			row.ProfileID,
			row.ResultStatus,
			row.PostStatus,
			row.ExternalID,
			row.URL,
			row.CreatedAt,
			row.PublishedAt,
			strconv.FormatInt(row.Impressions, 10),
			strconv.FormatInt(row.Reach, 10),
			strconv.FormatInt(row.Likes, 10),
			strconv.FormatInt(row.Comments, 10),
			strconv.FormatInt(row.Shares, 10),
			strconv.FormatInt(row.Saves, 10),
			strconv.FormatInt(row.Clicks, 10),
			strconv.FormatInt(row.VideoViews, 10),
			strconv.FormatFloat(row.EngagementRate, 'f', 4, 64),
			row.FetchedAt,
			row.LastFailureReason,
		})
	}
	writer.Flush()
}

func (h *AnalyticsExplorerHandler) ListPlatforms(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	start, end, ok := parseDateRange(w, r)
	if !ok {
		return
	}
	rows, err := h.queryPlatformAvailability(r.Context(), workspaceID, profileFilter(r), start, end)
	if err != nil {
		slog.Error("analytics platforms query failed", "workspace_id", workspaceID, "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load analytics platforms")
		return
	}
	writeSuccess(w, rows)
}

func (h *AnalyticsExplorerHandler) GetPlatform(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	platformName := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "platform")))
	if _, ok := analyticsPlatformCapabilities()[platformName]; !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Unknown analytics platform")
		return
	}
	start, end, ok := parseDateRange(w, r)
	if !ok {
		return
	}

	availabilityRows, err := h.queryPlatformAvailability(r.Context(), workspaceID, profileFilter(r), start, end)
	if err != nil {
		slog.Error("analytics platform availability query failed", "workspace_id", workspaceID, "platform", platformName, "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load platform availability")
		return
	}
	var availability analyticsPlatformAvailability
	for _, row := range availabilityRows {
		if row.Platform == platformName {
			availability = row
			break
		}
	}

	summary, err := h.queryPlatformSummary(r.Context(), workspaceID, platformName, profileFilter(r), start, end)
	if err != nil {
		slog.Error("analytics platform summary query failed", "workspace_id", workspaceID, "platform", platformName, "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load platform summary")
		return
	}
	trend, err := h.queryPlatformTrend(r.Context(), workspaceID, platformName, profileFilter(r), start, end)
	if err != nil {
		slog.Error("analytics platform trend query failed", "workspace_id", workspaceID, "platform", platformName, "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load platform trend")
		return
	}
	accounts, err := h.queryPlatformAccounts(r.Context(), workspaceID, platformName, profileFilter(r), start, end)
	if err != nil {
		slog.Error("analytics platform accounts query failed", "workspace_id", workspaceID, "platform", platformName, "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load platform accounts")
		return
	}
	sortSpec, _ := analyticsPostSortSpec("engagement_rate")
	topPosts, err := h.queryAnalyticsPosts(r.Context(), analyticsPostFilters{
		WorkspaceID: workspaceID,
		From:        start,
		To:          end,
		Platform:    platformName,
		ProfileID:   profileFilter(r),
		Limit:       6,
		Sort:        sortSpec,
	})
	if err != nil {
		slog.Error("analytics platform top posts query failed", "workspace_id", workspaceID, "platform", platformName, "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load platform top posts")
		return
	}
	if len(topPosts) > 6 {
		topPosts = topPosts[:6]
	}

	writeSuccess(w, analyticsPlatformDetail{
		Platform: platformName,
		Period: summaryPeriod{
			Start: start.Format("2006-01-02"),
			End:   end.Add(-24 * time.Hour).Format("2006-01-02"),
		},
		Availability: availability,
		Summary:      summary,
		Trend:        trend,
		Accounts:     accounts,
		TopPosts:     topPosts,
	})
}

func (h *AnalyticsExplorerHandler) RequestRefresh(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	var body analyticsRefreshRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body")
			return
		}
	}
	body.Platform = strings.ToLower(strings.TrimSpace(body.Platform))
	body.ProfileID = strings.TrimSpace(body.ProfileID)
	body.AccountID = strings.TrimSpace(body.AccountID)
	body.PostID = strings.TrimSpace(body.PostID)
	if body.Limit <= 0 {
		body.Limit = analyticsRefreshMaxRows
	}
	if body.Limit > analyticsRefreshMaxRows {
		body.Limit = analyticsRefreshMaxRows
	}
	start, end, err := analyticsRefreshRange(body.From, body.To)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return
	}
	if body.Platform != "" {
		if _, ok := analyticsPlatformCapabilities()[body.Platform]; !ok {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "unknown analytics platform")
			return
		}
	}

	matched, requested, err := h.requestAnalyticsRefresh(r.Context(), workspaceID, body, start, end)
	if err != nil {
		slog.Error("analytics refresh request failed", "workspace_id", workspaceID, "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to request analytics refresh")
		return
	}

	writeAccepted(w, analyticsRefreshResponse{
		Status:         "queued",
		MatchedCount:   matched,
		RequestedCount: requested,
		Limit:          body.Limit,
		ProcessedBy:    "analytics_refresh_worker",
		Filters:        body,
	})
}

func (h *AnalyticsExplorerHandler) parsePostFilters(w http.ResponseWriter, r *http.Request, maxLimit int) (analyticsPostFilters, bool) {
	workspaceID := h.getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return analyticsPostFilters{}, false
	}
	start, end, ok := parseDateRange(w, r)
	if !ok {
		return analyticsPostFilters{}, false
	}
	limit, err := normalizeAnalyticsPostsLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return analyticsPostFilters{}, false
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	offset, err := normalizeAnalyticsCursorOffset(r.URL.Query().Get("cursor"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return analyticsPostFilters{}, false
	}
	sortSpec, err := analyticsPostSortSpec(r.URL.Query().Get("sort"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return analyticsPostFilters{}, false
	}

	return analyticsPostFilters{
		WorkspaceID: workspaceID,
		From:        start,
		To:          end,
		Platform:    platformFilter(r),
		ProfileID:   profileFilter(r),
		Status:      statusFilter(r),
		AccountID:   strings.TrimSpace(firstQueryValue(r, "account_id", "social_account_id")),
		PostID:      strings.TrimSpace(r.URL.Query().Get("post_id")),
		Limit:       limit,
		Offset:      offset,
		Sort:        sortSpec,
	}, true
}

func (h *AnalyticsExplorerHandler) getWorkspaceID(r *http.Request) string {
	return auth.GetWorkspaceID(r.Context())
}
