package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type APIMetricsHandler struct {
	queries *db.Queries
}

func NewAPIMetricsHandler(queries *db.Queries) *APIMetricsHandler {
	return &APIMetricsHandler{queries: queries}
}

// Summary returns per-endpoint metrics for a workspace within a time range.
// GET /v1/workspaces/{workspaceID}/api-metrics/summary?from=...&to=...
func (h *APIMetricsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")
	from, to := parseTimeRange(r)

	rows, err := h.queries.GetAPIMetricsSummary(r.Context(), db.GetAPIMetricsSummaryParams{
		WorkspaceID: workspaceID,
		CreatedAt:   pgtype.Timestamptz{Time: from, Valid: true},
		CreatedAt_2: pgtype.Timestamptz{Time: to, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load metrics")
		return
	}

	writeSuccess(w, rows)
}

// Trend returns hourly call counts for a workspace.
// GET /v1/workspaces/{workspaceID}/api-metrics/trend?from=...&to=...
func (h *APIMetricsHandler) Trend(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")
	from, to := parseTimeRange(r)

	rows, err := h.queries.GetAPIMetricsTrend(r.Context(), db.GetAPIMetricsTrendParams{
		WorkspaceID: workspaceID,
		CreatedAt:   pgtype.Timestamptz{Time: from, Valid: true},
		CreatedAt_2: pgtype.Timestamptz{Time: to, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load trend")
		return
	}

	writeSuccess(w, rows)
}

// Overall returns aggregate stats for a workspace.
// GET /v1/workspaces/{workspaceID}/api-metrics/overall?from=...&to=...
func (h *APIMetricsHandler) Overall(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")
	from, to := parseTimeRange(r)

	row, err := h.queries.GetAPIMetricsOverall(r.Context(), db.GetAPIMetricsOverallParams{
		WorkspaceID: workspaceID,
		CreatedAt:   pgtype.Timestamptz{Time: from, Valid: true},
		CreatedAt_2: pgtype.Timestamptz{Time: to, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load overall metrics")
		return
	}

	// Compute reliability percentage
	reliability := 0.0
	if row.TotalCalls > 0 {
		reliability = float64(row.TotalCalls-row.ServerErrorCount) / float64(row.TotalCalls) * 100
	}

	writeSuccess(w, map[string]any{
		"total_calls":        row.TotalCalls,
		"success_count":      row.SuccessCount,
		"client_error_count": row.ClientErrorCount,
		"server_error_count": row.ServerErrorCount,
		"reliability_pct":    reliability,
		"p50_ms":             row.P50Ms,
		"p95_ms":             row.P95Ms,
		"p99_ms":             row.P99Ms,
	})
}

func parseTimeRange(r *http.Request) (time.Time, time.Time) {
	from := time.Now().AddDate(0, 0, -7) // default: last 7 days
	to := time.Now()

	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = t
		}
	}
	return from, to
}
