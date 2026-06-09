package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type APIMetricsHandler struct {
	queries apiMetricsQuerier
}

type apiMetricsQuerier interface {
	GetAPIMetricsOverall(context.Context, db.GetAPIMetricsOverallParams) (db.GetAPIMetricsOverallRow, error)
	GetAPIMetricsSummary(context.Context, db.GetAPIMetricsSummaryParams) ([]db.GetAPIMetricsSummaryRow, error)
	GetAPIMetricsTrendHourly(context.Context, db.GetAPIMetricsTrendHourlyParams) ([]db.GetAPIMetricsTrendHourlyRow, error)
	GetAPIMetricsTrendDaily(context.Context, db.GetAPIMetricsTrendDailyParams) ([]db.GetAPIMetricsTrendDailyRow, error)
	GetAPIMetricsStatusCodes(context.Context, db.GetAPIMetricsStatusCodesParams) ([]db.GetAPIMetricsStatusCodesRow, error)
}

type adminAPIMetricsQuerier interface {
	GetAdminAPIMetricsOverall(context.Context, db.GetAdminAPIMetricsOverallParams) (db.GetAdminAPIMetricsOverallRow, error)
	GetAdminAPIMetricsSummary(context.Context, db.GetAdminAPIMetricsSummaryParams) ([]db.GetAdminAPIMetricsSummaryRow, error)
	GetAdminAPIMetricsTrendHourly(context.Context, db.GetAdminAPIMetricsTrendHourlyParams) ([]db.GetAdminAPIMetricsTrendHourlyRow, error)
	GetAdminAPIMetricsTrendDaily(context.Context, db.GetAdminAPIMetricsTrendDailyParams) ([]db.GetAdminAPIMetricsTrendDailyRow, error)
	GetAdminAPIMetricsStatusCodes(context.Context, db.GetAdminAPIMetricsStatusCodesParams) ([]db.GetAdminAPIMetricsStatusCodesRow, error)
	GetAdminAPIMetricsWorkspaces(context.Context, db.GetAdminAPIMetricsWorkspacesParams) ([]db.GetAdminAPIMetricsWorkspacesRow, error)
}

func NewAPIMetricsHandler(queries apiMetricsQuerier) *APIMetricsHandler {
	return &APIMetricsHandler{queries: queries}
}

type AdminAPIMetricsHandler struct {
	queries adminAPIMetricsQuerier
}

func NewAdminAPIMetricsHandler(queries adminAPIMetricsQuerier) *AdminAPIMetricsHandler {
	return &AdminAPIMetricsHandler{queries: queries}
}

// Summary returns per-endpoint metrics for a workspace within a time range.
// GET /v1/api-metrics/summary?from=...&to=...
func (h *APIMetricsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	opts, ok := h.parseOptions(w, r)
	if !ok {
		return
	}

	rows, err := h.queries.GetAPIMetricsSummary(r.Context(), db.GetAPIMetricsSummaryParams{
		WorkspaceID:  workspaceID,
		CreatedAt:    pgtype.Timestamptz{Time: opts.from, Valid: true},
		CreatedAt_2:  pgtype.Timestamptz{Time: opts.to, Valid: true},
		MethodFilter: opts.method,
		PathFilter:   opts.path,
		StatusClass:  opts.statusClass,
		SortKey:      opts.sort,
		RowLimit:     opts.limit,
	})
	if err != nil {
		slog.Error("api_metrics.Summary: query failed", "err", err, "workspace_id", workspaceID)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load metrics")
		return
	}

	writeSuccess(w, rows)
}

// Trend returns hourly call counts for a workspace.
// GET /v1/api-metrics/trend?from=...&to=...
func (h *APIMetricsHandler) Trend(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	opts, ok := h.parseOptions(w, r)
	if !ok {
		return
	}

	params := db.GetAPIMetricsTrendHourlyParams{
		WorkspaceID:  workspaceID,
		CreatedAt:    pgtype.Timestamptz{Time: opts.from, Valid: true},
		CreatedAt_2:  pgtype.Timestamptz{Time: opts.to, Valid: true},
		MethodFilter: opts.method,
		PathFilter:   opts.path,
		StatusClass:  opts.statusClass,
	}
	if opts.interval == "day" {
		rows, err := h.queries.GetAPIMetricsTrendDaily(r.Context(), db.GetAPIMetricsTrendDailyParams(params))
		if err != nil {
			slog.Error("api_metrics.Trend: query failed", "err", err, "workspace_id", workspaceID, "interval", opts.interval)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load trend")
			return
		}
		writeSuccess(w, rows)
		return
	}
	rows, err := h.queries.GetAPIMetricsTrendHourly(r.Context(), params)
	if err != nil {
		slog.Error("api_metrics.Trend: query failed", "err", err, "workspace_id", workspaceID, "interval", opts.interval)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load trend")
		return
	}

	writeSuccess(w, rows)
}

// Overall returns aggregate stats for a workspace.
// GET /v1/api-metrics/overall?from=...&to=...
func (h *APIMetricsHandler) Overall(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	opts, ok := h.parseOptions(w, r)
	if !ok {
		return
	}

	row, err := h.queries.GetAPIMetricsOverall(r.Context(), db.GetAPIMetricsOverallParams{
		WorkspaceID:  workspaceID,
		CreatedAt:    pgtype.Timestamptz{Time: opts.from, Valid: true},
		CreatedAt_2:  pgtype.Timestamptz{Time: opts.to, Valid: true},
		MethodFilter: opts.method,
		PathFilter:   opts.path,
		StatusClass:  opts.statusClass,
	})
	if err != nil {
		slog.Error("api_metrics.Overall: query failed", "err", err, "workspace_id", workspaceID)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load overall metrics")
		return
	}

	// Compute reliability percentage
	reliability := 0.0
	if row.TotalCalls > 0 {
		reliability = float64(row.TotalCalls-row.ServerErrorCount) / float64(row.TotalCalls) * 100
	}

	writeSuccess(w, map[string]any{
		"total_calls":             row.TotalCalls,
		"success_count":           row.SuccessCount,
		"client_error_count":      row.ClientErrorCount,
		"server_error_count":      row.ServerErrorCount,
		"rate_limited_count":      row.RateLimitedCount,
		"error_rate_pct":          row.ErrorRatePct,
		"server_failure_rate_pct": row.ServerFailureRatePct,
		"reliability_pct":         reliability,
		"p50_ms":                  row.P50Ms,
		"p95_ms":                  row.P95Ms,
		"p99_ms":                  row.P99Ms,
		"avg_ms":                  row.AvgMs,
	})
}

// StatusCodes returns exact status-code distribution for a workspace.
// GET /v1/api-metrics/status-codes?from=...&to=...
func (h *APIMetricsHandler) StatusCodes(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	opts, ok := h.parseOptions(w, r)
	if !ok {
		return
	}

	rows, err := h.queries.GetAPIMetricsStatusCodes(r.Context(), db.GetAPIMetricsStatusCodesParams{
		WorkspaceID:  workspaceID,
		CreatedAt:    pgtype.Timestamptz{Time: opts.from, Valid: true},
		CreatedAt_2:  pgtype.Timestamptz{Time: opts.to, Valid: true},
		MethodFilter: opts.method,
		PathFilter:   opts.path,
		StatusClass:  opts.statusClass,
		RowLimit:     opts.limit,
	})
	if err != nil {
		slog.Error("api_metrics.StatusCodes: query failed", "err", err, "workspace_id", workspaceID)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load status codes")
		return
	}

	writeSuccess(w, rows)
}

type apiMetricsOptions struct {
	from        time.Time
	to          time.Time
	workspaceID string
	method      string
	path        string
	statusClass string
	sort        string
	limit       int32
	minCalls    int32
	interval    string
}

const maxAPIMetricsRange = 90 * 24 * time.Hour

func (h *APIMetricsHandler) parseOptions(w http.ResponseWriter, r *http.Request) (apiMetricsOptions, bool) {
	opts, err := parseAPIMetricsOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return apiMetricsOptions{}, false
	}
	return opts, true
}

func parseAPIMetricsOptions(r *http.Request) (apiMetricsOptions, error) {
	now := time.Now().UTC()
	opts := apiMetricsOptions{
		from:     now.AddDate(0, 0, -7),
		to:       now,
		sort:     "total_calls_desc",
		limit:    50,
		minCalls: 1,
		interval: "",
	}
	q := r.URL.Query()

	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return opts, fmt.Errorf("invalid from timestamp")
		}
		opts.from = t
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return opts, fmt.Errorf("invalid to timestamp")
		}
		opts.to = t
	}
	if opts.from.After(opts.to) {
		return opts, fmt.Errorf("from must be before to")
	}
	if opts.to.Sub(opts.from) > maxAPIMetricsRange {
		return opts, fmt.Errorf("time range cannot exceed 90 days")
	}

	opts.method = q.Get("method")
	if opts.method != "" && !stringIn(opts.method, []string{"GET", "POST", "PUT", "PATCH", "DELETE"}) {
		return opts, fmt.Errorf("invalid method filter")
	}
	opts.path = q.Get("path")
	opts.workspaceID = q.Get("workspace_id")
	opts.statusClass = q.Get("status_class")
	if opts.statusClass != "" && !stringIn(opts.statusClass, []string{"2xx", "3xx", "4xx", "5xx"}) {
		return opts, fmt.Errorf("invalid status_class filter")
	}
	if v := q.Get("sort"); v != "" {
		if !stringIn(v, []string{"total_calls_desc", "p95_ms_desc", "p99_ms_desc", "server_errors_desc", "rate_limited_desc"}) {
			return opts, fmt.Errorf("invalid sort")
		}
		opts.sort = v
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 200 {
			return opts, fmt.Errorf("limit must be between 1 and 200")
		}
		opts.limit = int32(n)
	}
	if v := q.Get("min_calls"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return opts, fmt.Errorf("min_calls must be at least 1")
		}
		opts.minCalls = int32(n)
	}
	if v := q.Get("interval"); v != "" {
		if !stringIn(v, []string{"hour", "day"}) {
			return opts, fmt.Errorf("invalid interval")
		}
		opts.interval = v
	}
	if opts.interval == "" {
		if opts.to.Sub(opts.from) > 7*24*time.Hour {
			opts.interval = "day"
		} else {
			opts.interval = "hour"
		}
	}
	return opts, nil
}

func (h *AdminAPIMetricsHandler) parseOptions(w http.ResponseWriter, r *http.Request) (apiMetricsOptions, bool) {
	opts, err := parseAPIMetricsOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return apiMetricsOptions{}, false
	}
	return opts, true
}

func (h *AdminAPIMetricsHandler) Overall(w http.ResponseWriter, r *http.Request) {
	opts, ok := h.parseOptions(w, r)
	if !ok {
		return
	}
	row, err := h.queries.GetAdminAPIMetricsOverall(r.Context(), db.GetAdminAPIMetricsOverallParams{
		CreatedAt:       pgtype.Timestamptz{Time: opts.from, Valid: true},
		CreatedAt_2:     pgtype.Timestamptz{Time: opts.to, Valid: true},
		WorkspaceFilter: opts.workspaceID,
		MethodFilter:    opts.method,
		PathFilter:      opts.path,
		StatusClass:     opts.statusClass,
	})
	if err != nil {
		slog.Error("admin_api_metrics.Overall: query failed", "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load admin overall metrics")
		return
	}
	reliability := 0.0
	if row.TotalCalls > 0 {
		reliability = float64(row.TotalCalls-row.ServerErrorCount) / float64(row.TotalCalls) * 100
	}
	writeSuccess(w, map[string]any{
		"total_calls":             row.TotalCalls,
		"success_count":           row.SuccessCount,
		"client_error_count":      row.ClientErrorCount,
		"server_error_count":      row.ServerErrorCount,
		"rate_limited_count":      row.RateLimitedCount,
		"error_rate_pct":          row.ErrorRatePct,
		"server_failure_rate_pct": row.ServerFailureRatePct,
		"reliability_pct":         reliability,
		"p50_ms":                  row.P50Ms,
		"p95_ms":                  row.P95Ms,
		"p99_ms":                  row.P99Ms,
		"avg_ms":                  row.AvgMs,
	})
}

func (h *AdminAPIMetricsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	opts, ok := h.parseOptions(w, r)
	if !ok {
		return
	}
	rows, err := h.queries.GetAdminAPIMetricsSummary(r.Context(), db.GetAdminAPIMetricsSummaryParams{
		CreatedAt:       pgtype.Timestamptz{Time: opts.from, Valid: true},
		CreatedAt_2:     pgtype.Timestamptz{Time: opts.to, Valid: true},
		WorkspaceFilter: opts.workspaceID,
		MethodFilter:    opts.method,
		PathFilter:      opts.path,
		StatusClass:     opts.statusClass,
		SortKey:         opts.sort,
		RowLimit:        opts.limit,
		MinCalls:        opts.minCalls,
	})
	if err != nil {
		slog.Error("admin_api_metrics.Summary: query failed", "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load admin metrics")
		return
	}
	writeSuccess(w, rows)
}

func (h *AdminAPIMetricsHandler) Trend(w http.ResponseWriter, r *http.Request) {
	opts, ok := h.parseOptions(w, r)
	if !ok {
		return
	}
	params := db.GetAdminAPIMetricsTrendHourlyParams{
		CreatedAt:       pgtype.Timestamptz{Time: opts.from, Valid: true},
		CreatedAt_2:     pgtype.Timestamptz{Time: opts.to, Valid: true},
		WorkspaceFilter: opts.workspaceID,
		MethodFilter:    opts.method,
		PathFilter:      opts.path,
		StatusClass:     opts.statusClass,
	}
	if opts.interval == "day" {
		rows, err := h.queries.GetAdminAPIMetricsTrendDaily(r.Context(), db.GetAdminAPIMetricsTrendDailyParams(params))
		if err != nil {
			slog.Error("admin_api_metrics.Trend: query failed", "err", err, "interval", opts.interval)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load admin trend")
			return
		}
		writeSuccess(w, rows)
		return
	}
	rows, err := h.queries.GetAdminAPIMetricsTrendHourly(r.Context(), params)
	if err != nil {
		slog.Error("admin_api_metrics.Trend: query failed", "err", err, "interval", opts.interval)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load admin trend")
		return
	}
	writeSuccess(w, rows)
}

func (h *AdminAPIMetricsHandler) StatusCodes(w http.ResponseWriter, r *http.Request) {
	opts, ok := h.parseOptions(w, r)
	if !ok {
		return
	}
	rows, err := h.queries.GetAdminAPIMetricsStatusCodes(r.Context(), db.GetAdminAPIMetricsStatusCodesParams{
		CreatedAt:       pgtype.Timestamptz{Time: opts.from, Valid: true},
		CreatedAt_2:     pgtype.Timestamptz{Time: opts.to, Valid: true},
		WorkspaceFilter: opts.workspaceID,
		MethodFilter:    opts.method,
		PathFilter:      opts.path,
		StatusClass:     opts.statusClass,
		RowLimit:        opts.limit,
	})
	if err != nil {
		slog.Error("admin_api_metrics.StatusCodes: query failed", "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load admin status codes")
		return
	}
	writeSuccess(w, rows)
}

func (h *AdminAPIMetricsHandler) Workspaces(w http.ResponseWriter, r *http.Request) {
	opts, ok := h.parseOptions(w, r)
	if !ok {
		return
	}
	rows, err := h.queries.GetAdminAPIMetricsWorkspaces(r.Context(), db.GetAdminAPIMetricsWorkspacesParams{
		CreatedAt:       pgtype.Timestamptz{Time: opts.from, Valid: true},
		CreatedAt_2:     pgtype.Timestamptz{Time: opts.to, Valid: true},
		WorkspaceFilter: opts.workspaceID,
		MethodFilter:    opts.method,
		PathFilter:      opts.path,
		StatusClass:     opts.statusClass,
		SortKey:         opts.sort,
		RowLimit:        opts.limit,
		MinCalls:        opts.minCalls,
	})
	if err != nil {
		slog.Error("admin_api_metrics.Workspaces: query failed", "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load admin workspace metrics")
		return
	}
	writeSuccess(w, rows)
}

func stringIn(v string, allowed []string) bool {
	for _, candidate := range allowed {
		if v == candidate {
			return true
		}
	}
	return false
}
