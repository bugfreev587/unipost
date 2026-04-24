package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

// ── Shared range parsing ───────────────────────────────────────────────────

// parseDateRange reads analytics date range params.
//
// Preferred query params:
// - from=YYYY-MM-DD
// - to=YYYY-MM-DD
//
// Legacy aliases retained for compatibility:
// - start_date=YYYY-MM-DD
// - end_date=YYYY-MM-DD
//
// Defaults: end = today (UTC, end-of-day), start = end - 30 days.
// end is exclusive in the SQL queries (`<`), so the caller passes end+1d.
//
// Returns (start, endExclusive, ok). On invalid input writes 422 to w and
// returns ok=false.
func parseDateRange(w http.ResponseWriter, r *http.Request) (time.Time, time.Time, bool) {
	now := time.Now().UTC()
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)
	start := end.Add(-30 * 24 * time.Hour)

	if v := firstQueryValue(r, "to", "end_date"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid to, expected YYYY-MM-DD")
			return time.Time{}, time.Time{}, false
		}
		end = t.UTC().Add(24 * time.Hour)
	}
	if v := firstQueryValue(r, "from", "start_date"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid from, expected YYYY-MM-DD")
			return time.Time{}, time.Time{}, false
		}
		start = t.UTC()
	}
	if !start.Before(end) {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "from must be before to")
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}

func firstQueryValue(r *http.Request, keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(r.URL.Query().Get(key)); v != "" {
			return v
		}
	}
	return ""
}

func parseBoolQueryParam(r *http.Request, key string) bool {
	v := strings.ToLower(strings.TrimSpace(r.URL.Query().Get(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func tsParam(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// platformFilter normalizes the ?platform= query param. "all" or empty maps
// to the empty-string sentinel that disables the filter in the SQL query.
// Unknown values are passed through verbatim — they'll just match nothing.
func platformFilter(r *http.Request) string {
	v := strings.TrimSpace(r.URL.Query().Get("platform"))
	if v == "" || v == "all" {
		return ""
	}
	return v
}

// statusFilter normalizes the ?status= query param the same way.
func statusFilter(r *http.Request) string {
	v := strings.TrimSpace(r.URL.Query().Get("status"))
	if v == "" || v == "all" {
		return ""
	}
	return v
}

func profileFilter(r *http.Request) string {
	v := strings.TrimSpace(r.URL.Query().Get("profile_id"))
	if v == "" || v == "all" {
		return ""
	}
	return v
}

// percentChange returns (curr - prev) / prev. When prev is 0 it returns 0
// (the dashboard renders "--" for the first period to avoid divide-by-zero
// or misleading "+∞%" labels).
func percentChange(curr, prev int64) float64 {
	if prev == 0 {
		return 0
	}
	return float64(curr-prev) / float64(prev)
}

// engagementFromRow recomputes the unified engagement rate from a summary
// row using the PRD §9.1 formula. Kept here so the SQL doesn't have to know
// the formula.
func engagementFromRow(impressions, likes, comments, shares, saves, clicks int64) float64 {
	if impressions <= 0 {
		return 0
	}
	total := likes + comments + shares + saves + clicks
	rate := float64(total) / float64(impressions)
	return float64(int64(rate*10000+0.5)) / 10000
}

// ── GET /v1/analytics/summary ──────────────────────────────────────────────

type summaryPosts struct {
	Total      int64   `json:"total"`
	Published  int64   `json:"published"`
	Scheduled  int64   `json:"scheduled"`
	Failed     int64   `json:"failed"`
	FailedRate float64 `json:"failed_rate"`
}

type summaryEngagement struct {
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

type summaryPeriod struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type summaryDelta struct {
	ImpressionsChange float64 `json:"impressions_change"`
	LikesChange       float64 `json:"likes_change"`
	EngagementChange  float64 `json:"engagement_change"`
}

type summaryResponse struct {
	Period           summaryPeriod     `json:"period"`
	Posts            summaryPosts      `json:"posts"`
	Engagement       summaryEngagement `json:"engagement"`
	VsPreviousPeriod summaryDelta      `json:"vs_previous_period"`
}

// GetSummary handles GET /v1/analytics/summary
// and  GET /v1/workspaces/{workspaceID}/analytics/summary
func (h *AnalyticsHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	start, end, ok := parseDateRange(w, r)
	if !ok {
		return
	}
	platform := platformFilter(r)
	status := statusFilter(r)

	// Previous period: same length, immediately preceding.
	windowLen := end.Sub(start)
	prevStart := start.Add(-windowLen)
	prevEnd := start

	curr, err := h.queries.GetAnalyticsSummaryByWorkspace(r.Context(), db.GetAnalyticsSummaryByWorkspaceParams{
		WorkspaceID: workspaceID,
		CreatedAt:   tsParam(start),
		CreatedAt_2: tsParam(end),
		Column4:     platform,
		Column5:     status,
		Column6:     profileFilter(r),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load summary")
		return
	}
	prev, err := h.queries.GetAnalyticsSummaryByWorkspace(r.Context(), db.GetAnalyticsSummaryByWorkspaceParams{
		WorkspaceID: workspaceID,
		CreatedAt:   tsParam(prevStart),
		CreatedAt_2: tsParam(prevEnd),
		Column4:     platform,
		Column5:     status,
		Column6:     profileFilter(r),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load previous-period summary")
		return
	}

	currEngRate := engagementFromRow(curr.Impressions, curr.Likes, curr.Comments, curr.Shares, curr.Saves, curr.Clicks)
	prevEngRate := engagementFromRow(prev.Impressions, prev.Likes, prev.Comments, prev.Shares, prev.Saves, prev.Clicks)

	failedDenom := curr.PublishedPosts + curr.FailedPosts
	var failedRate float64
	if failedDenom > 0 {
		failedRate = float64(curr.FailedPosts) / float64(failedDenom)
		failedRate = float64(int64(failedRate*10000+0.5)) / 10000
	}

	// Engagement-rate change is computed as a relative delta on the rate
	// itself, not via percentChange (which is integer-based).
	var engChange float64
	if prevEngRate > 0 {
		engChange = (currEngRate - prevEngRate) / prevEngRate
	}

	resp := summaryResponse{
		Period: summaryPeriod{
			Start: start.Format("2006-01-02"),
			// SQL end is exclusive for date-only input, so report the
			// inclusive day-oriented range in the summary response.
			End: end.Add(-24 * time.Hour).Format("2006-01-02"),
		},
		Posts: summaryPosts{
			Total:      curr.TotalPosts,
			Published:  curr.PublishedPosts,
			Scheduled:  curr.ScheduledPosts,
			Failed:     curr.FailedPosts,
			FailedRate: failedRate,
		},
		Engagement: summaryEngagement{
			Impressions:    curr.Impressions,
			Reach:          curr.Reach,
			Likes:          curr.Likes,
			Comments:       curr.Comments,
			Shares:         curr.Shares,
			Saves:          curr.Saves,
			Clicks:         curr.Clicks,
			VideoViews:     curr.VideoViews,
			EngagementRate: currEngRate,
		},
		VsPreviousPeriod: summaryDelta{
			ImpressionsChange: percentChange(curr.Impressions, prev.Impressions),
			LikesChange:       percentChange(curr.Likes, prev.Likes),
			EngagementChange:  engChange,
		},
	}

	writeSuccess(w, resp)
}

// ── GET /v1/analytics/trend ────────────────────────────────────────────────

type trendResponse struct {
	Dates  []string           `json:"dates"`
	Series map[string][]int64 `json:"series"`
}

// GetTrend handles GET /v1/analytics/trend
// and  GET /v1/workspaces/{workspaceID}/analytics/trend
//
// metric query param is a CSV of: posts, impressions, likes, comments, shares.
// Defaults to "posts,impressions,likes" if absent.
func (h *AnalyticsHandler) GetTrend(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	start, end, ok := parseDateRange(w, r)
	if !ok {
		return
	}

	metricParam := r.URL.Query().Get("metric")
	if metricParam == "" {
		metricParam = "posts,impressions,likes"
	}
	requested := map[string]bool{}
	for _, m := range strings.Split(metricParam, ",") {
		m = strings.TrimSpace(m)
		switch m {
		case "posts", "impressions", "likes", "comments", "shares":
			requested[m] = true
		}
	}
	if len(requested) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "metric must contain at least one of: posts, impressions, likes, comments, shares")
		return
	}

	rows, err := h.queries.GetAnalyticsTrendByWorkspace(r.Context(), db.GetAnalyticsTrendByWorkspaceParams{
		WorkspaceID: workspaceID,
		CreatedAt:   tsParam(start),
		CreatedAt_2: tsParam(end),
		Column4:     platformFilter(r),
		Column5:     statusFilter(r),
		Column6:     profileFilter(r),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load trend")
		return
	}

	// Build a date → row map for zero-fill.
	rowByDay := make(map[string]db.GetAnalyticsTrendByWorkspaceRow, len(rows))
	for _, row := range rows {
		key := row.Day.Time.UTC().Format("2006-01-02")
		rowByDay[key] = row
	}

	// Walk every day in [start, end) inclusive of start, exclusive of end.
	var dates []string
	series := map[string][]int64{}
	for k := range requested {
		series[k] = []int64{}
	}
	for d := start; d.Before(end); d = d.Add(24 * time.Hour) {
		key := d.Format("2006-01-02")
		dates = append(dates, key)
		row := rowByDay[key] // zero value if missing
		if requested["posts"] {
			series["posts"] = append(series["posts"], row.Posts)
		}
		if requested["impressions"] {
			series["impressions"] = append(series["impressions"], row.Impressions)
		}
		if requested["likes"] {
			series["likes"] = append(series["likes"], row.Likes)
		}
		if requested["comments"] {
			series["comments"] = append(series["comments"], row.Comments)
		}
		if requested["shares"] {
			series["shares"] = append(series["shares"], row.Shares)
		}
	}

	writeSuccess(w, trendResponse{Dates: dates, Series: series})
}

// ── GET /v1/analytics/by-platform ──────────────────────────────────────────

type byPlatformRow struct {
	Platform       string  `json:"platform"`
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

// GetByPlatform handles GET /v1/analytics/by-platform
// and  GET /v1/workspaces/{workspaceID}/analytics/by-platform
func (h *AnalyticsHandler) GetByPlatform(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	start, end, ok := parseDateRange(w, r)
	if !ok {
		return
	}

	rows, err := h.queries.GetAnalyticsByPlatformByWorkspace(r.Context(), db.GetAnalyticsByPlatformByWorkspaceParams{
		WorkspaceID: workspaceID,
		CreatedAt:   tsParam(start),
		CreatedAt_2: tsParam(end),
		Column4:     platformFilter(r),
		Column5:     statusFilter(r),
		Column6:     profileFilter(r),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load by-platform breakdown")
		return
	}

	out := make([]byPlatformRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, byPlatformRow{
			Platform:       r.Platform,
			Posts:          r.Posts,
			Accounts:       r.Accounts,
			Impressions:    r.Impressions,
			Reach:          r.Reach,
			Likes:          r.Likes,
			Comments:       r.Comments,
			Shares:         r.Shares,
			Saves:          r.Saves,
			Clicks:         r.Clicks,
			VideoViews:     r.VideoViews,
			EngagementRate: engagementFromRow(r.Impressions, r.Likes, r.Comments, r.Shares, r.Saves, r.Clicks),
		})
	}

	writeSuccess(w, out)
}
