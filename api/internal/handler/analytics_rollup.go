// analytics_rollup.go is the Sprint 5 PR1 analytics rollup endpoint.
//
//	GET /v1/analytics/rollup?from=...&to=...&granularity=day&group_by=platform
//
// Returns one row per (time bucket, group dimension) with published /
// failed / partial counts. Designed to power dashboard charts and
// LLM-summary tools without forcing the client to walk individual
// post_results rows.
//
// Implementation note: sqlc cannot parameterize the GROUP BY clause,
// so this handler builds the SQL dynamically (still parameterized
// for the WHERE clause filters) and executes via the raw pgxpool.
// The GROUP BY columns are restricted to a fixed allowlist so this
// can never become a SQL injection vector.

package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/auth"
)

// AnalyticsRollupHandler owns GET /v1/analytics/rollup. Holds a raw
// pgx pool because sqlc can't model the dynamic GROUP BY.
type AnalyticsRollupHandler struct {
	pool *pgxpool.Pool
}

func NewAnalyticsRollupHandler(pool *pgxpool.Pool) *AnalyticsRollupHandler {
	return &AnalyticsRollupHandler{pool: pool}
}

// Allowlists for the dynamic SQL pieces. NEVER read user input
// directly into the SQL string — always look up via these maps so
// the only way a value lands in the query is if we put it there.
var allowedGranularity = map[string]string{
	"day":   "day",
	"week":  "week",
	"month": "month",
}

// Map of allowed group_by dimension → SQL column expression. The
// expression is plain SQL the handler interpolates into the query;
// the keys are what the API client sends.
var allowedGroupBy = map[string]string{
	"platform":          "sa.platform",
	"social_account_id": "sa.id",
	"external_user_id":  "sa.external_user_id",
	"status":            "spr.status",
}

// MaxRollupRangeDays is the upper bound on (to - from). One year is
// long enough for "show me last year's posts" without letting a
// client bomb the database with a multi-year query.
const MaxRollupRangeDays = 366

// rollupGroup is one row in the response — the dimension columns
// are populated based on what the client asked for.
type rollupGroup struct {
	Platform        string `json:"platform,omitempty"`
	SocialAccountID string `json:"social_account_id,omitempty"`
	ExternalUserID  string `json:"external_user_id,omitempty"`
	Status          string `json:"status,omitempty"`
	PublishedCount  int    `json:"published_count"`
	FailedCount     int    `json:"failed_count"`
	PartialCount    int    `json:"partial_count"`
}

type rollupBucket struct {
	Bucket time.Time     `json:"bucket"`
	Groups []rollupGroup `json:"groups"`
}

type rollupResponse struct {
	Granularity string         `json:"granularity"`
	GroupBy     []string       `json:"group_by"`
	Series      []rollupBucket `json:"series"`
}

// GetRollup handles GET /v1/analytics/rollup.
func (h *AnalyticsRollupHandler) GetRollup(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	// Parse + validate query params.
	from, to, ok := parseRollupRange(w, r)
	if !ok {
		return
	}

	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "day"
	}
	if _, valid := allowedGranularity[granularity]; !valid {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"granularity must be day, week, or month")
		return
	}

	groupBy := parseGroupBy(r.URL.Query().Get("group_by"))
	if len(groupBy) == 0 {
		// Default group_by is platform — same as the existing
		// /v1/analytics/by-platform endpoint.
		groupBy = []string{"platform"}
	}
	for _, g := range groupBy {
		if _, valid := allowedGroupBy[g]; !valid {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
				"unknown group_by dimension: "+g)
			return
		}
	}

	// Build the SQL with the validated group-by columns interpolated.
	// Every interpolated string came from the allowedGroupBy map, so
	// there's no injection vector — but we still bind workspace_id +
	// from + to as parameters.
	groupCols := make([]string, len(groupBy))
	for i, g := range groupBy {
		groupCols[i] = allowedGroupBy[g]
	}
	groupColsJoined := strings.Join(groupCols, ", ")

	query := fmt.Sprintf(`
		SELECT
			date_trunc('%s', sp.created_at) AS bucket,
			%s,
			COUNT(*) FILTER (WHERE spr.status = 'published')::INTEGER AS published_count,
			COUNT(*) FILTER (WHERE spr.status = 'failed')::INTEGER    AS failed_count,
			COUNT(*) FILTER (WHERE spr.status = 'partial')::INTEGER   AS partial_count
		FROM social_post_results spr
		JOIN social_posts sp ON spr.post_id = sp.id
		JOIN social_accounts sa ON spr.social_account_id = sa.id
		WHERE sp.workspace_id = $1
		  AND sp.deleted_at IS NULL
		  AND sp.created_at >= $2
		  AND sp.created_at < $3
		GROUP BY bucket, %s
		ORDER BY bucket DESC, %s
	`, allowedGranularity[granularity], groupColsJoined, groupColsJoined, groupColsJoined)

	rows, err := h.pool.Query(r.Context(), query, workspaceID, from, to)
	if err != nil {
		slog.Error("analytics rollup query failed", "workspace_id", workspaceID, "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to query rollup")
		return
	}
	defer rows.Close()

	// Read the column count first so we know how many group-by
	// values to scan per row. Layout: bucket, [group cols...],
	// published, failed, partial.
	resp := rollupResponse{
		Granularity: granularity,
		GroupBy:     groupBy,
	}
	bucketsByTime := map[time.Time]*rollupBucket{}
	bucketOrder := []time.Time{} // preserve query ORDER BY

	for rows.Next() {
		var bucket time.Time
		// Pre-allocate per-dimension destinations.
		groupVals := make([]any, len(groupBy))
		groupValPtrs := make([]any, len(groupBy))
		for i := range groupVals {
			var s *string
			groupValPtrs[i] = &s
		}
		var published, failed, partial int

		// Build the scan target list dynamically.
		dest := make([]any, 0, 2+len(groupBy)+3)
		dest = append(dest, &bucket)
		dest = append(dest, groupValPtrs...)
		dest = append(dest, &published, &failed, &partial)

		if err := rows.Scan(dest...); err != nil {
			slog.Error("analytics rollup scan failed", "err", err)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to scan rollup row")
			return
		}

		// Hydrate the rollupGroup based on which dimensions were
		// requested. Pointer-to-pointer-to-string handles nullable
		// columns (external_user_id is the only one that can be null).
		grp := rollupGroup{
			PublishedCount: published,
			FailedCount:    failed,
			PartialCount:   partial,
		}
		for i, g := range groupBy {
			pp := groupValPtrs[i].(**string)
			val := ""
			if pp != nil && *pp != nil {
				val = **pp
			}
			switch g {
			case "platform":
				grp.Platform = val
			case "social_account_id":
				grp.SocialAccountID = val
			case "external_user_id":
				grp.ExternalUserID = val
			case "status":
				grp.Status = val
			}
		}

		b, ok := bucketsByTime[bucket]
		if !ok {
			b = &rollupBucket{Bucket: bucket}
			bucketsByTime[bucket] = b
			bucketOrder = append(bucketOrder, bucket)
		}
		b.Groups = append(b.Groups, grp)
	}
	if err := rows.Err(); err != nil {
		slog.Error("analytics rollup rows.Err", "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to read rollup rows")
		return
	}

	resp.Series = make([]rollupBucket, 0, len(bucketOrder))
	for _, t := range bucketOrder {
		resp.Series = append(resp.Series, *bucketsByTime[t])
	}

	writeSuccess(w, resp)
}

// parseRollupRange reads from + to from the query string. Both are
// required. Range capped at MaxRollupRangeDays to prevent runaway
// queries.
func parseRollupRange(w http.ResponseWriter, r *http.Request) (time.Time, time.Time, bool) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr == "" || toStr == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"from and to are required (RFC3339 timestamps)")
		return time.Time{}, time.Time{}, false
	}
	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"from must be RFC3339")
		return time.Time{}, time.Time{}, false
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"to must be RFC3339")
		return time.Time{}, time.Time{}, false
	}
	if !from.Before(to) {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"from must be before to")
		return time.Time{}, time.Time{}, false
	}
	if to.Sub(from) > time.Duration(MaxRollupRangeDays)*24*time.Hour {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"date range too large; max 366 days")
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}

// parseGroupBy splits a comma-separated group_by query param into
// a deduplicated string slice, preserving order.
func parseGroupBy(raw string) []string {
	if raw == "" {
		return nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// getWorkspaceID resolves the workspace context for either auth mode
// (API key or Clerk session via /v1/workspaces/{workspaceID}/...).
func (h *AnalyticsRollupHandler) getWorkspaceID(r *http.Request) string {
	if pid := auth.GetWorkspaceID(r.Context()); pid != "" {
		return pid
	}
	// Dashboard route fallback would go here if/when we wire one.
	// For Sprint 5 PR1 the rollup is API-key-only.
	return ""
}

// Avoid unused-import errors when context is referenced only inside
// strings.
var _ = context.Background
