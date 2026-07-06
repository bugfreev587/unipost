package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/audit"
	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/billing"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

// AdminHandler exposes read-only aggregates for the /admin dashboard.
// Auth + ADMIN_USERS gating live in auth.AdminMiddleware. We talk to
// the DB via the raw pgxpool because most of these queries are
// cross-tenant aggregates that don't fit into per-workspace sqlc patterns.
type AdminHandler struct {
	pool      *pgxpool.Pool
	stripeMgr *billing.Manager
	queries   *db.Queries // for audit-log writes
}

func NewAdminHandler(pool *pgxpool.Pool, stripeMgr *billing.Manager, queries *db.Queries) *AdminHandler {
	return &AdminHandler{pool: pool, stripeMgr: stripeMgr, queries: queries}
}

func (h *AdminHandler) excludedUserIDs(ctx context.Context) ([]string, error) {
	if h.stripeMgr == nil {
		return []string{}, nil
	}
	directIDs, emails := h.stripeMgr.SuperAdminAllowlist()

	out := make([]string, 0, len(directIDs)+len(emails))
	out = append(out, directIDs...)

	if len(emails) > 0 {
		rows, err := h.pool.Query(ctx,
			`SELECT id FROM users WHERE lower(email) = ANY($1)`,
			emails,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			out = append(out, id)
		}
	}
	return out, nil
}

// ── Stats ────────────────────────────────────────────────────────────

type adminStatsResponse struct {
	TotalUsers           int64 `json:"total_users"`
	NewUsersThisMonth    int64 `json:"new_users_this_month"`
	PaidUsers            int64 `json:"paid_users"`
	MRRCents             int64 `json:"mrr_cents"`
	PostsThisMonth       int64 `json:"posts_this_month"`
	PostsFailedThisMonth int64 `json:"posts_failed_this_month"`
	ActiveWorkspaces     int64 `json:"active_workspaces"`
	PlatformConnections  int64 `json:"platform_connections"`
	NewSignups7d         int64 `json:"new_signups_7d"`
	PrevSignups7d        int64 `json:"prev_signups_7d"`
	Churn30d             int64 `json:"churn_30d"`
}

const adminStatsQuery = `
SELECT
  (SELECT COUNT(*) FROM users u WHERE u.id != ALL($1))                                   AS total_users,
  (SELECT COUNT(*) FROM users u
     WHERE u.created_at >= date_trunc('month', NOW())
       AND u.id != ALL($1))                                                              AS new_users_this_month,
  (SELECT COUNT(DISTINCT w.user_id)
     FROM subscriptions s
     JOIN workspaces w ON w.id = s.workspace_id
     JOIN plans pl ON pl.id = s.plan_id
     WHERE s.status = 'active' AND pl.price_cents > 0
       AND w.user_id != ALL($1))                                                         AS paid_users,
  (SELECT COALESCE(SUM(pl.price_cents), 0)
     FROM subscriptions s
     JOIN plans pl ON pl.id = s.plan_id
     JOIN workspaces w ON w.id = s.workspace_id
     WHERE s.status = 'active'
       AND w.user_id != ALL($1))                                                         AS mrr_cents,
  (SELECT COUNT(*)
     FROM social_posts sp
     JOIN workspaces w ON w.id = sp.workspace_id
     WHERE sp.created_at >= date_trunc('month', NOW())
       AND w.user_id != ALL($1))                                                         AS posts_this_month,
  (SELECT COUNT(*)
     FROM social_posts sp
     JOIN workspaces w ON w.id = sp.workspace_id
     WHERE sp.created_at >= date_trunc('month', NOW())
       AND sp.status = 'failed'
       AND w.user_id != ALL($1))                                                         AS posts_failed_this_month,
  (SELECT COUNT(*) FROM workspaces w WHERE w.user_id != ALL($1))                         AS active_workspaces,
  (SELECT COUNT(*)
     FROM social_accounts sa
     JOIN profiles p ON p.id = sa.profile_id
     JOIN workspaces w ON w.id = p.workspace_id
     WHERE sa.disconnected_at IS NULL
       AND w.user_id != ALL($1))                                                         AS platform_connections,
  (SELECT COUNT(*) FROM users u
     WHERE u.created_at >= NOW() - INTERVAL '7 days'
       AND u.id != ALL($1))                                                              AS new_signups_7d,
  (SELECT COUNT(*) FROM users u
     WHERE u.created_at >= NOW() - INTERVAL '14 days'
       AND u.created_at <  NOW() - INTERVAL '7 days'
       AND u.id != ALL($1))                                                              AS prev_signups_7d,
  (SELECT COUNT(*)
     FROM subscriptions s
     JOIN workspaces w ON w.id = s.workspace_id
     WHERE s.status IN ('canceled', 'past_due')
       AND s.updated_at >= NOW() - INTERVAL '30 days'
       AND w.user_id != ALL($1))                                                         AS churn_30d
`

func (h *AdminHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	excluded, err := h.excludedUserIDs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to resolve excluded users: "+err.Error())
		return
	}
	var s adminStatsResponse
	err = h.pool.QueryRow(r.Context(), adminStatsQuery, excluded).Scan(
		&s.TotalUsers,
		&s.NewUsersThisMonth,
		&s.PaidUsers,
		&s.MRRCents,
		&s.PostsThisMonth,
		&s.PostsFailedThisMonth,
		&s.ActiveWorkspaces,
		&s.PlatformConnections,
		&s.NewSignups7d,
		&s.PrevSignups7d,
		&s.Churn30d,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load stats: "+err.Error())
		return
	}
	writeSuccess(w, s)
}

type adminIntegrationLogResponse struct {
	ID               int64           `json:"id"`
	WorkspaceID      string          `json:"workspace_id"`
	WorkspaceName    string          `json:"workspace_name,omitempty"`
	OwnerEmail       string          `json:"owner_email,omitempty"`
	PlanID           string          `json:"plan_id,omitempty"`
	TS               time.Time       `json:"ts"`
	Level            string          `json:"level"`
	Status           string          `json:"status"`
	Category         string          `json:"category"`
	Action           string          `json:"action"`
	Source           string          `json:"source"`
	Message          string          `json:"message"`
	RequestID        string          `json:"request_id,omitempty"`
	TraceID          string          `json:"trace_id,omitempty"`
	ActorUserID      string          `json:"actor_user_id,omitempty"`
	ActorAPIKeyID    string          `json:"actor_api_key_id,omitempty"`
	ProfileID        string          `json:"profile_id,omitempty"`
	SocialAccountID  string          `json:"social_account_id,omitempty"`
	PostID           string          `json:"post_id,omitempty"`
	PlatformPostID   string          `json:"platform_post_id,omitempty"`
	Platform         string          `json:"platform,omitempty"`
	Endpoint         string          `json:"endpoint,omitempty"`
	Method           string          `json:"method,omitempty"`
	HTTPStatusCode   *int32          `json:"http_status_code,omitempty"`
	RemoteStatusCode *int32          `json:"remote_status_code,omitempty"`
	DurationMs       *int32          `json:"duration_ms,omitempty"`
	ErrorCode        string          `json:"error_code,omitempty"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
	RequestPayload   json.RawMessage `json:"request_payload,omitempty"`
	ResponsePayload  json.RawMessage `json:"response_payload,omitempty"`
}

const adminLogsBaseSelect = `
SELECT
  l.id,
  l.workspace_id,
  COALESCE(w.name, '') AS workspace_name,
  COALESCE(u.email, '') AS owner_email,
  COALESCE(s.plan_id, 'free') AS plan_id,
  l.ts,
  l.level,
  l.status,
  l.category,
  l.action,
  l.source,
  l.message,
  l.request_id,
  l.trace_id,
  l.actor_user_id,
  l.actor_api_key_id,
  l.profile_id,
  l.social_account_id,
  l.post_id,
  l.platform_post_id,
  l.platform,
  l.endpoint,
  l.method,
  l.http_status_code,
  l.remote_status_code,
  l.duration_ms,
  l.error_code,
  l.metadata,
  l.request_payload,
  l.response_payload
FROM integration_logs l
LEFT JOIN workspaces w ON w.id = l.workspace_id
LEFT JOIN users u ON u.id = w.user_id
LEFT JOIN subscriptions s ON s.workspace_id = w.id
`

func parseAdminLogTime(raw string, fallback time.Time) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback.UTC()
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return fallback.UTC()
	}
	return parsed.UTC()
}

func scanAdminIntegrationLogRow(row pgx.Row, includePayloads bool) (adminIntegrationLogResponse, error) {
	var out adminIntegrationLogResponse
	var requestID, traceID, actorUserID, actorAPIKeyID, profileID, socialAccountID, postID, platformPostID, platform, endpoint, method, errorCode *string
	var httpStatusCode, remoteStatusCode, durationMs *int32
	var requestPayload, responsePayload []byte
	err := row.Scan(
		&out.ID,
		&out.WorkspaceID,
		&out.WorkspaceName,
		&out.OwnerEmail,
		&out.PlanID,
		&out.TS,
		&out.Level,
		&out.Status,
		&out.Category,
		&out.Action,
		&out.Source,
		&out.Message,
		&requestID,
		&traceID,
		&actorUserID,
		&actorAPIKeyID,
		&profileID,
		&socialAccountID,
		&postID,
		&platformPostID,
		&platform,
		&endpoint,
		&method,
		&httpStatusCode,
		&remoteStatusCode,
		&durationMs,
		&errorCode,
		&out.Metadata,
		&requestPayload,
		&responsePayload,
	)
	if err != nil {
		return adminIntegrationLogResponse{}, err
	}
	if requestID != nil {
		out.RequestID = *requestID
	}
	if traceID != nil {
		out.TraceID = *traceID
	}
	if actorUserID != nil {
		out.ActorUserID = *actorUserID
	}
	if actorAPIKeyID != nil {
		out.ActorAPIKeyID = *actorAPIKeyID
	}
	if profileID != nil {
		out.ProfileID = *profileID
	}
	if socialAccountID != nil {
		out.SocialAccountID = *socialAccountID
	}
	if postID != nil {
		out.PostID = *postID
	}
	if platformPostID != nil {
		out.PlatformPostID = *platformPostID
	}
	if platform != nil {
		out.Platform = *platform
	}
	if endpoint != nil {
		out.Endpoint = *endpoint
	}
	if method != nil {
		out.Method = *method
	}
	if httpStatusCode != nil {
		out.HTTPStatusCode = httpStatusCode
	}
	if remoteStatusCode != nil {
		out.RemoteStatusCode = remoteStatusCode
	}
	if durationMs != nil {
		out.DurationMs = durationMs
	}
	if errorCode != nil {
		out.ErrorCode = *errorCode
	}
	if includePayloads {
		out.RequestPayload = requestPayload
		out.ResponsePayload = responsePayload
	}
	return out, nil
}

func (h *AdminHandler) ListLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	from := parseAdminLogTime(q.Get("from"), time.Now().AddDate(0, 0, -7))
	to := parseAdminLogTime(q.Get("to"), time.Now())

	sql := adminLogsBaseSelect + `
WHERE ($1::TEXT = '' OR l.workspace_id = $1)
  AND ($2::TEXT = '' OR u.email ILIKE '%' || $2 || '%')
  AND ($3::TEXT = '' OR l.category = $3)
  AND ($4::TEXT = '' OR l.action = $4)
  AND ($5::TEXT = '' OR l.source = $5)
  AND ($6::TEXT = '' OR l.level = $6)
  AND ($7::TEXT = '' OR l.status = $7)
  AND ($8::TEXT = '' OR l.platform = $8)
  AND ($9::TEXT = '' OR l.profile_id = $9)
  AND ($10::TEXT = '' OR l.social_account_id = $10)
  AND ($11::TEXT = '' OR l.post_id = $11)
  AND ($12::TEXT = '' OR l.request_id = $12)
  AND ($13::TEXT = '' OR l.error_code = $13)
  AND (
    $14::TEXT = ''
    OR l.message ILIKE '%' || $14 || '%'
    OR l.action ILIKE '%' || $14 || '%'
    OR l.request_id ILIKE '%' || $14 || '%'
    OR l.post_id ILIKE '%' || $14 || '%'
    OR l.error_code ILIKE '%' || $14 || '%'
    OR u.email ILIKE '%' || $14 || '%'
  )
  AND l.ts >= $15
  AND l.ts <= $16
ORDER BY l.ts DESC, l.id DESC
LIMIT $17`

	rows, err := h.pool.Query(r.Context(), sql,
		strings.TrimSpace(q.Get("workspace_id")),
		strings.TrimSpace(q.Get("owner_email")),
		strings.TrimSpace(q.Get("category")),
		strings.TrimSpace(q.Get("action")),
		strings.TrimSpace(q.Get("source")),
		strings.TrimSpace(q.Get("level")),
		strings.TrimSpace(q.Get("status")),
		strings.TrimSpace(q.Get("platform")),
		strings.TrimSpace(q.Get("profile_id")),
		strings.TrimSpace(q.Get("social_account_id")),
		strings.TrimSpace(q.Get("post_id")),
		strings.TrimSpace(q.Get("request_id")),
		strings.TrimSpace(q.Get("error_code")),
		strings.TrimSpace(q.Get("q")),
		from,
		to,
		limit,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load admin logs: "+err.Error())
		return
	}
	defer rows.Close()

	out := make([]adminIntegrationLogResponse, 0, limit)
	for rows.Next() {
		item, scanErr := scanAdminIntegrationLogRow(rows, false)
		if scanErr != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to scan admin logs: "+scanErr.Error())
			return
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to read admin logs: "+err.Error())
		return
	}
	writeSuccessWithListMeta(w, out, len(out), limit)
}

func (h *AdminHandler) GetLog(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid log id")
		return
	}

	row := h.pool.QueryRow(r.Context(), adminLogsBaseSelect+`
WHERE l.id = $1
LIMIT 1`, id)

	item, err := scanAdminIntegrationLogRow(row, true)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Log not found")
		return
	}
	writeSuccess(w, item)
}

// ── User list ────────────────────────────────────────────────────────

type adminUserRow struct {
	ID                   string     `json:"id"`
	Email                string     `json:"email"`
	CreatedAt            time.Time  `json:"created_at"`
	SignupCountry        string     `json:"signup_country_code"`
	WorkspaceCount       int64      `json:"workspace_count"`
	APIKeyCount          int64      `json:"api_key_count"`
	PlatformCount        int64      `json:"platform_count"`
	Platforms            []string   `json:"platforms"`
	PostsUsed            int64      `json:"posts_used"`
	ScheduledPosts       int64      `json:"scheduled_posts"`
	FailedPostsThisMonth int64      `json:"failed_posts_this_month"`
	PostLimit            int64      `json:"post_limit"`
	MRRCents             int64      `json:"mrr_cents"`
	IsPaid               bool       `json:"is_paid"`
	LastPostAt           *time.Time `json:"last_post_at"`
}

type adminCountryBreakdownRow struct {
	CountryCode string `json:"country_code"`
	Count       int64  `json:"count"`
}

// adminUserSignupTrendResponse returns raw signup timestamps so the
// dashboard can bucket them in the viewer's local timezone. Server-side
// bucketing was using the DB session timezone (UTC) which shifted late-
// evening signups in the Americas to the next calendar day.
type adminUserSignupTrendResponse struct {
	RangeDays int                        `json:"range_days"`
	Events    []time.Time                `json:"events"`
	Countries []adminCountryBreakdownRow `json:"countries"`
}

var adminUserSortOrders = map[string]string{
	"newest":      "created_at DESC",
	"mrr":         "mrr_cents DESC, created_at DESC",
	"usage":       "posts_used DESC, created_at DESC",
	"last_active": "last_post_at DESC NULLS LAST, created_at DESC",
}

func adminUserPlanFilterSQL(plan string) string {
	switch plan {
	case "paid":
		return `AND EXISTS(
			SELECT 1 FROM subscriptions s
			JOIN plans pl ON pl.id = s.plan_id
			JOIN workspaces w ON w.id = s.workspace_id
			WHERE w.user_id = u.id AND s.status='active' AND pl.price_cents > 0
		)`
	case "free":
		return `AND NOT EXISTS(
			SELECT 1 FROM subscriptions s
			JOIN plans pl ON pl.id = s.plan_id
			JOIN workspaces w ON w.id = s.workspace_id
			WHERE w.user_id = u.id AND s.status='active' AND pl.price_cents > 0
		)`
	default:
		return ""
	}
}

func adminUserActivityFilterSQL(activity string) string {
	switch activity {
	case "active":
		return `AND EXISTS(
			SELECT 1 FROM social_posts sp
			JOIN workspaces w ON w.id = sp.workspace_id
			WHERE w.user_id = u.id
			  AND sp.deleted_at IS NULL
			  AND sp.published_at >= NOW() - INTERVAL '30 days'
		)`
	default:
		return ""
	}
}

func adminUserFiltersSQL(plan, activity string) string {
	return adminUserPlanFilterSQL(plan) + adminUserActivityFilterSQL(activity)
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	search := q.Get("search")
	plan := q.Get("plan")
	if plan == "" {
		plan = "all"
	}
	activity := q.Get("activity")
	sortKey := q.Get("sort")
	if sortKey == "" {
		sortKey = "newest"
	}
	orderBy, ok := adminUserSortOrders[sortKey]
	if !ok {
		orderBy = adminUserSortOrders["newest"]
	}

	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset, _ := strconv.Atoi(q.Get("offset"))
	if offset < 0 {
		offset = 0
	}

	filtersSQL := adminUserFiltersSQL(plan, activity)

	excluded, err := h.excludedUserIDs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to resolve excluded users: "+err.Error())
		return
	}

	sql := `
WITH base AS (
  SELECT
    u.id, u.email, u.created_at,
    COALESCE((
      SELECT NULLIF(lv.country_code, '')
      FROM landing_session_users lsu
      JOIN landing_visits lv ON lv.session_id = lsu.session_id
      WHERE lsu.user_id = u.id
        AND NULLIF(lv.country_code, '') IS NOT NULL
      ORDER BY lsu.first_bound_at ASC, lv.created_at ASC
      LIMIT 1
    ), '') AS signup_country_code,
    (SELECT COUNT(*) FROM workspaces w WHERE w.user_id = u.id) AS workspace_count,
    (SELECT COUNT(*)
       FROM api_keys ak
       JOIN workspaces w ON w.id = ak.workspace_id
       WHERE w.user_id = u.id AND ak.revoked_at IS NULL) AS api_key_count,
    (SELECT COUNT(*)
       FROM social_accounts sa
       JOIN profiles p ON p.id = sa.profile_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE w.user_id = u.id AND sa.disconnected_at IS NULL) AS platform_count,
    COALESCE((SELECT array_agg(DISTINCT sa.platform)
       FROM social_accounts sa
       JOIN profiles p ON p.id = sa.profile_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE w.user_id = u.id AND sa.disconnected_at IS NULL), '{}') AS platforms,
    COALESCE((SELECT SUM(usg.post_count)::bigint
       FROM usage usg
       JOIN workspaces w ON w.id = usg.workspace_id
       WHERE w.user_id = u.id AND usg.period = to_char(NOW(), 'YYYY-MM')), 0) AS posts_used,
    COALESCE((SELECT COUNT(*)::bigint
       FROM social_posts sp
       JOIN workspaces w ON w.id = sp.workspace_id
       WHERE w.user_id = u.id
         AND sp.status = 'scheduled'
         AND sp.deleted_at IS NULL), 0) AS scheduled_posts,
    COALESCE((SELECT COUNT(DISTINCT sp.id)::bigint
       FROM social_posts sp
       JOIN workspaces w ON w.id = sp.workspace_id
       WHERE w.user_id = u.id
         AND sp.deleted_at IS NULL
         AND sp.created_at >= date_trunc('month', NOW())
         AND (
           sp.status = 'failed'
           OR EXISTS (
             SELECT 1
             FROM social_post_results spr
             WHERE spr.post_id = sp.id
               AND spr.status = 'failed'
           )
         )), 0) AS failed_posts_this_month,
    CASE WHEN EXISTS(
       SELECT 1
       FROM subscriptions s
       JOIN plans pl ON pl.id = s.plan_id
       JOIN workspaces w ON w.id = s.workspace_id
       WHERE w.user_id = u.id AND s.status = 'active' AND pl.post_limit < 0
    ) THEN -1 ELSE COALESCE((
       SELECT SUM(pl.post_limit)::bigint
       FROM subscriptions s
       JOIN plans pl ON pl.id = s.plan_id
       JOIN workspaces w ON w.id = s.workspace_id
       WHERE w.user_id = u.id AND s.status = 'active'
    ), 0) END AS post_limit,
    COALESCE((SELECT SUM(pl.price_cents)::bigint
       FROM subscriptions s
       JOIN plans pl ON pl.id = s.plan_id
       JOIN workspaces w ON w.id = s.workspace_id
       WHERE w.user_id = u.id AND s.status = 'active'), 0) AS mrr_cents,
    EXISTS(SELECT 1
       FROM subscriptions s
       JOIN plans pl ON pl.id = s.plan_id
       JOIN workspaces w ON w.id = s.workspace_id
       WHERE w.user_id = u.id AND s.status = 'active' AND pl.price_cents > 0) AS is_paid,
    (SELECT MAX(sp.published_at)
       FROM social_posts sp
       JOIN workspaces w ON w.id = sp.workspace_id
       WHERE w.user_id = u.id) AS last_post_at
  FROM users u
  WHERE ($1 = '' OR u.email ILIKE '%' || $1 || '%' OR u.id ILIKE '%' || $1 || '%')
    AND u.id != ALL($4)
  ` + filtersSQL + `
)
SELECT * FROM base ORDER BY ` + orderBy + ` LIMIT $2 OFFSET $3`

	rows, err := h.pool.Query(r.Context(), sql, search, limit, offset, excluded)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list users: "+err.Error())
		return
	}
	defer rows.Close()

	out := make([]adminUserRow, 0)
	for rows.Next() {
		var u adminUserRow
		var lastPostAt *time.Time
		if err := rows.Scan(
			&u.ID, &u.Email, &u.CreatedAt,
			&u.SignupCountry,
			&u.WorkspaceCount, &u.APIKeyCount, &u.PlatformCount,
			&u.Platforms,
			&u.PostsUsed, &u.ScheduledPosts, &u.FailedPostsThisMonth, &u.PostLimit,
			&u.MRRCents, &u.IsPaid,
			&lastPostAt,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to scan user: "+err.Error())
			return
		}
		u.LastPostAt = lastPostAt
		out = append(out, u)
	}

	var total int64
	totalSQL := `SELECT COUNT(*) FROM users u
		 WHERE ($1 = '' OR u.email ILIKE '%' || $1 || '%' OR u.id ILIKE '%' || $1 || '%')
		   AND u.id != ALL($2)
		   ` + filtersSQL + `
		   `
	_ = h.pool.QueryRow(r.Context(),
		totalSQL,
		search, excluded,
	).Scan(&total)

	writeSuccessWithListMeta(w, out, int(total), limit)
}

func (h *AdminHandler) GetUserSignups(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}

	excluded, err := h.excludedUserIDs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to resolve excluded users: "+err.Error())
		return
	}

	// Pull signups from a slightly wider server-side window than the
	// caller asked for: viewers in far timezones may bucket events into
	// a "today" that extends past UTC NOW(), and into a "30 days ago"
	// that starts before UTC NOW() - 30d. +2 days covers any IANA TZ.
	const sql = `
SELECT u.created_at
FROM users u
WHERE u.created_at >= NOW() - (($1::int + 2) * INTERVAL '1 day')
  AND u.id != ALL($2)
ORDER BY u.created_at ASC`

	rows, err := h.pool.Query(r.Context(), sql, days, excluded)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load signup trend: "+err.Error())
		return
	}
	defer rows.Close()

	resp := adminUserSignupTrendResponse{
		RangeDays: days,
		Events:    []time.Time{},
		Countries: []adminCountryBreakdownRow{},
	}
	for rows.Next() {
		var t time.Time
		if err := rows.Scan(&t); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to scan signup event: "+err.Error())
			return
		}
		resp.Events = append(resp.Events, t)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to iterate signup events: "+err.Error())
		return
	}

	countryRows, err := h.pool.Query(r.Context(), `
WITH signup_countries AS (
  SELECT COALESCE((
    SELECT NULLIF(lv.country_code, '')
    FROM landing_session_users lsu
    JOIN landing_visits lv ON lv.session_id = lsu.session_id
    WHERE lsu.user_id = u.id
      AND NULLIF(lv.country_code, '') IS NOT NULL
    ORDER BY lsu.first_bound_at ASC, lv.created_at ASC
    LIMIT 1
  ), '') AS country_code
  FROM users u
  WHERE u.created_at >= NOW() - ($1::int * INTERVAL '1 day')
    AND u.id != ALL($2)
)
SELECT country_code, COUNT(*)::BIGINT
FROM signup_countries
GROUP BY country_code
ORDER BY COUNT(*) DESC, country_code ASC`, days, excluded)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load signup countries: "+err.Error())
		return
	}
	defer countryRows.Close()
	for countryRows.Next() {
		var row adminCountryBreakdownRow
		if err := countryRows.Scan(&row.CountryCode, &row.Count); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to scan signup countries: "+err.Error())
			return
		}
		resp.Countries = append(resp.Countries, row)
	}
	if err := countryRows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to iterate signup countries: "+err.Error())
		return
	}

	writeSuccess(w, resp)
}

// ── User detail ──────────────────────────────────────────────────────

type adminUserWorkspace struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	CreatedAt     time.Time `json:"created_at"`
	PlanID        string    `json:"plan_id"`
	PlanName      string    `json:"plan_name"`
	PriceCents    int32     `json:"price_cents"`
	PostsUsed     int64     `json:"posts_used"`
	PostLimit     int32     `json:"post_limit"`
	Status        string    `json:"status"`
	PlatformCount int64     `json:"platform_count"`
}

type adminUserDetailResponse struct {
	ID                 string               `json:"id"`
	Email              string               `json:"email"`
	Name               string               `json:"name"`
	CreatedAt          time.Time            `json:"created_at"`
	SignupCountry      string               `json:"signup_country_code"`
	WorkspaceCount     int64                `json:"workspace_count"`
	APIKeyCount        int64                `json:"api_key_count"`
	PlatformCount      int64                `json:"platform_count"`
	Platforms          []string             `json:"platforms"`
	PostsUsedThisMonth int64                `json:"posts_used_this_month"`
	PostLimit          int64                `json:"post_limit"`
	MRRCents           int64                `json:"mrr_cents"`
	TotalPosts         int64                `json:"total_posts"`
	FailedPosts30d     int64                `json:"failed_posts_30d"`
	LastPostAt         *time.Time           `json:"last_post_at"`
	Workspaces         []adminUserWorkspace `json:"workspaces"`
}

type adminUserScheduledPost struct {
	PostID      string     `json:"post_id"`
	Title       string     `json:"title"`
	CreatedAt   time.Time  `json:"created_at"`
	ScheduledAt *time.Time `json:"scheduled_at"`
	Platforms   []string   `json:"platforms"`
}

type adminPostFailure struct {
	PostID             string    `json:"post_id"`
	PostFailureID      *string   `json:"post_failure_id,omitempty"`
	SocialPostResultID *string   `json:"social_post_result_id,omitempty"`
	UserID             string    `json:"user_id"`
	UserEmail          string    `json:"user_email"`
	WorkspaceID        string    `json:"workspace_id"`
	WorkspaceName      string    `json:"workspace_name"`
	CreatedAt          time.Time `json:"created_at"`
	PostStatus         string    `json:"post_status"`
	Source             string    `json:"source"`
	Platform           *string   `json:"platform,omitempty"`
	AccountName        *string   `json:"account_name,omitempty"`
	Caption            *string   `json:"caption,omitempty"`
	ErrorMessage       *string   `json:"error_message,omitempty"`
	ErrorSummary       *string   `json:"error_summary,omitempty"`
	ErrorCode          *string   `json:"error_code,omitempty"`
	FailureStage       *string   `json:"failure_stage,omitempty"`
	PlatformErrorCode  *string   `json:"platform_error_code,omitempty"`
	IsRetriable        *bool     `json:"is_retriable,omitempty"`
	NextAction         *string   `json:"next_action,omitempty"`
	// DebugCurl is the redacted curl dump captured by debugrt when the
	// adapter's HTTP call failed. Always included for admins — this
	// is the primary diagnostic surface for platform failures.
	DebugCurl *string `json:"debug_curl,omitempty"`
}

type adminPostFailureQuery struct {
	UserID   string
	Search   string
	Platform string
	Source   string
	Period   string
	Days     int
	Limit    int
	Excluded []string
}

type adminPostRow struct {
	PostID               string     `json:"post_id"`
	UserID               string     `json:"user_id"`
	UserEmail            string     `json:"user_email"`
	WorkspaceID          string     `json:"workspace_id"`
	WorkspaceName        string     `json:"workspace_name"`
	Status               string     `json:"status"`
	Source               string     `json:"source"`
	Caption              *string    `json:"caption,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	ScheduledAt          *time.Time `json:"scheduled_at,omitempty"`
	PublishedAt          *time.Time `json:"published_at,omitempty"`
	Platforms            []string   `json:"platforms"`
	ResultCount          int64      `json:"result_count"`
	PublishedResultCount int64      `json:"published_result_count"`
	FailedResultCount    int64      `json:"failed_result_count"`
}

type adminPostsQuery struct {
	Search       string
	Status       string
	ResultStatus string
	Platform     string
	Source       string
	UserID       string
	WorkspaceID  string
	Days         int
	// StartAt/EndAt are absolute [start, end) bounds sent by the
	// dashboard for calendar periods (today, this month, last month) —
	// computed client-side so the boundaries follow the admin's local
	// timezone, not the server's. They take precedence over Days.
	StartAt *time.Time
	EndAt   *time.Time
	// All disables the time filter entirely ("All" period option).
	All      bool
	Limit    int
	Excluded []string
}

// adminPostsWindow resolves the query's time filter to absolute
// [start, end) bounds. nil means unbounded on that side: All returns
// (nil, nil), explicit StartAt/EndAt pass through, otherwise Days
// (default 30, capped at 365) anchors to the current instant.
func adminPostsWindow(opts adminPostsQuery) (start, end *time.Time) {
	if opts.All {
		return nil, nil
	}
	if opts.StartAt != nil {
		return opts.StartAt, opts.EndAt
	}
	days := opts.Days
	if days <= 0 || days > 365 {
		days = 30
	}
	s := time.Now().AddDate(0, 0, -days)
	return &s, nil
}

// parseAdminTimeParam parses an RFC3339 query param, returning nil for
// empty or malformed values so bad input degrades to the default window.
func parseAdminTimeParam(value string) *time.Time {
	if value == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	return &t
}

// adminPostsAggregatesResponse drives the four headline cards, the
// per-platform breakdown row, and the published-vs-failed time series
// on /admin/posts. Numbers are SQL-side aggregates so the cards stay
// accurate even when the row list is LIMITed; the previous client-side
// counts undercounted whenever filters matched more than 100 posts.
//
// by_platform is RESULT-level (one social_post_results row = one
// platform attempt) so a multi-platform post that succeeds on Twitter
// and fails on LinkedIn shows up as +1 success / +1 failure split
// across the two cards rather than getting hidden under "partial".
type adminPostsAggregatesResponse struct {
	TotalPosts  int64                         `json:"total_posts"`
	UniqueUsers int64                         `json:"unique_users"`
	ByStatus    map[string]int64              `json:"by_status"`
	ByPlatform  []adminPostsPlatformAggregate `json:"by_platform"`
	// Events: raw per-post timestamps for the chart, filtered to
	// status IN ('published', 'failed') so the dashboard can bucket by
	// the viewer's local day. Server bucketing here used UTC and was
	// off by up to 24h for viewers outside UTC.
	Events []adminPostsEvent `json:"events"`
}

type adminPostsPlatformAggregate struct {
	Platform  string `json:"platform"`
	Published int64  `json:"published"`
	Failed    int64  `json:"failed"`
	Total     int64  `json:"total"`
}

type adminPostsEvent struct {
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"`
}

type adminBillingRow struct {
	WorkspaceID          string     `json:"workspace_id"`
	WorkspaceName        string     `json:"workspace_name"`
	UserID               string     `json:"user_id"`
	UserEmail            string     `json:"user_email"`
	PlanID               string     `json:"plan_id"`
	PlanName             string     `json:"plan_name"`
	PriceCents           int64      `json:"price_cents"`
	Status               string     `json:"status"`
	StripeCustomerID     *string    `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID *string    `json:"stripe_subscription_id,omitempty"`
	CurrentPeriodEnd     *time.Time `json:"current_period_end,omitempty"`
	CancelAtPeriodEnd    bool       `json:"cancel_at_period_end"`
	TrialUsed            bool       `json:"trial_used"`
	PostsUsed            int64      `json:"posts_used"`
	PostLimit            int64      `json:"post_limit"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type adminBillingQuery struct {
	Search   string
	Status   string
	PlanID   string
	Days     int
	Limit    int
	Excluded []string
}

type adminEmailNotificationRow struct {
	ID                 string     `json:"id"`
	EventKey           string     `json:"event_key"`
	EventType          string     `json:"event_type"`
	TriggerEvent       string     `json:"trigger_event"`
	WorkspaceID        string     `json:"workspace_id"`
	WorkspaceName      string     `json:"workspace_name"`
	UserID             string     `json:"user_id"`
	OwnerEmail         string     `json:"owner_email"`
	Email              string     `json:"email"`
	Period             string     `json:"period"`
	ThresholdPercent   int32      `json:"threshold_percent"`
	Status             string     `json:"status"`
	TransactionalID    string     `json:"transactional_id"`
	IdempotencyKey     string     `json:"idempotency_key"`
	EffectiveUsage     int32      `json:"effective_usage"`
	CompletedUsage     int32      `json:"completed_usage"`
	ReservedUsage      int32      `json:"reserved_usage"`
	PostLimit          int32      `json:"post_limit"`
	FailureReason      *string    `json:"failure_reason,omitempty"`
	AttemptedAt        time.Time  `json:"attempted_at"`
	SentAt             *time.Time `json:"sent_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	Provider           string     `json:"provider"`
	DeliveryClass      string     `json:"delivery_class"`
	PreferenceCategory string     `json:"preference_category"`
	FooterPolicy       string     `json:"footer_policy"`
	PreferenceDecision string     `json:"preference_decision"`
	TriggerSource      string     `json:"trigger_source"`
	TriggerReference   string     `json:"trigger_reference_id"`
	SubjectSnapshot    string     `json:"subject_snapshot"`
}

type adminEmailNotificationsQuery struct {
	Search    string
	Status    string
	Provider  string
	EventKey  string
	Threshold int
	Period    string
	Limit     int
	Offset    int
}

const adminEmailNotificationsCTESQL = `
WITH email_notifications AS (
  SELECT
    e.id,
    e.event_key,
    e.event_key AS event_type,
    COALESCE(NULLIF(e.trigger_reference_id, ''), NULLIF(e.idempotency_key, ''), e.id) AS trigger_event,
    COALESCE(e.workspace_id, '') AS workspace_id,
    COALESCE(w.name, '') AS workspace_name,
    COALESCE(e.recipient_user_id, '') AS user_id,
    COALESCE(u.email, '') AS owner_email,
    e.recipient_email AS email,
    ''::TEXT AS period,
    0::INTEGER AS threshold_percent,
    e.status,
    COALESCE(e.provider_template_id, '') AS transactional_id,
    e.idempotency_key,
    0::INTEGER AS effective_usage,
    0::INTEGER AS completed_usage,
    0::INTEGER AS reserved_usage,
    0::INTEGER AS post_limit,
    e.last_error AS failure_reason,
    e.attempted_at,
    e.sent_at,
    e.created_at,
    e.updated_at,
    e.provider,
    e.delivery_class,
    CASE e.event_key
      WHEN 'email.post.failed.v1' THEN 'publishing_failures'
      WHEN 'email.account.disconnected.v1' THEN 'account_connection_alerts'
      WHEN 'email.quota.free_plan_reminder.v1' THEN 'usage_quota_alerts'
      WHEN 'email.support.error_triage_follow_up.v1' THEN 'support_follow_ups'
      WHEN 'email.user.welcome.v1' THEN 'onboarding_tips'
      WHEN 'email.notification.test.v1' THEN 'test_emails'
      ELSE 'essential_account_billing'
    END AS preference_category,
    CASE e.delivery_class
      WHEN 'critical_transactional' THEN 'required_notice'
      WHEN 'lifecycle' THEN 'unsubscribe'
      WHEN 'test' THEN 'test_notice'
      ELSE 'manage_preferences'
    END AS footer_policy,
    CASE
      WHEN e.status = 'skipped' AND e.last_error = 'preference_disabled' THEN 'skipped_preference_disabled'
      WHEN e.delivery_class = 'critical_transactional' THEN 'required'
      WHEN e.delivery_class = 'lifecycle' THEN 'unsubscribe_eligible'
      WHEN e.delivery_class = 'test' THEN 'user_initiated'
      ELSE 'preference_checked'
    END AS preference_decision,
    COALESCE(e.trigger_source, '') AS trigger_source,
    COALESCE(e.trigger_reference_id, '') AS trigger_reference_id,
    COALESCE(e.subject_snapshot, '') AS subject_snapshot
  FROM email_send_attempts e
  LEFT JOIN workspaces w ON w.id = e.workspace_id
  LEFT JOIN users u ON u.id = e.recipient_user_id

  UNION ALL

  SELECT
    r.id,
    'email.quota.free_plan_reminder.v1' AS event_key,
    'free_plan_quota_reminder' AS event_type,
    'usage_' || r.threshold_percent::TEXT || '_percent' AS trigger_event,
    r.workspace_id,
    COALESCE(w.name, '') AS workspace_name,
    r.user_id,
    COALESCE(u.email, '') AS owner_email,
    r.email,
    r.period,
    r.threshold_percent,
    r.status,
    r.transactional_id,
    r.idempotency_key,
    r.effective_usage,
    r.completed_usage,
    r.reserved_usage,
    r.post_limit,
    r.failure_reason,
    r.attempted_at,
    r.sent_at,
    r.created_at,
    r.updated_at,
    'loops' AS provider,
    'service_alert' AS delivery_class,
    'usage_quota_alerts' AS preference_category,
    'manage_preferences' AS footer_policy,
    'preference_documented' AS preference_decision,
    'quota threshold evaluator' AS trigger_source,
    r.workspace_id || ':' || r.period || ':' || r.threshold_percent::TEXT AS trigger_reference_id,
    '' AS subject_snapshot
  FROM free_plan_quota_email_reminders r
  LEFT JOIN workspaces w ON w.id = r.workspace_id
  LEFT JOIN users u ON u.id = r.user_id

  UNION ALL

  SELECT
    s.id,
    'email.support.error_triage_follow_up.v1' AS event_key,
    'error_triage_user_action' AS event_type,
    s.item_id || ':' || s.recipient_scope_key AS trigger_event,
    COALESCE(rec.workspace_id, '') AS workspace_id,
    COALESCE(w.name, '') AS workspace_name,
    s.recipient_user_id AS user_id,
    COALESCE(u.email, '') AS owner_email,
    s.recipient_email AS email,
    ''::TEXT AS period,
    0::INTEGER AS threshold_percent,
    CASE s.provider_status WHEN 'succeeded' THEN 'sent' ELSE s.provider_status END AS status,
    COALESCE(s.loops_transactional_id, '') AS transactional_id,
    s.idempotency_key,
    0::INTEGER AS effective_usage,
    0::INTEGER AS completed_usage,
    0::INTEGER AS reserved_usage,
    0::INTEGER AS post_limit,
    s.provider_error AS failure_reason,
    s.created_at AS attempted_at,
    s.sent_at,
    s.created_at,
    s.created_at AS updated_at,
    'loops' AS provider,
    'service_alert' AS delivery_class,
    'support_follow_ups' AS preference_category,
    'manage_preferences' AS footer_policy,
    'admin_reviewed' AS preference_decision,
    'admin error triage send' AS trigger_source,
    s.item_id AS trigger_reference_id,
    s.subject_snapshot
  FROM error_triage_email_sends s
  LEFT JOIN error_triage_item_recipients rec ON rec.id = s.recipient_id
  LEFT JOIN workspaces w ON w.id = rec.workspace_id
  LEFT JOIN users u ON u.id = s.recipient_user_id

  UNION ALL

  SELECT
    d.id,
    CASE d.event_type
      WHEN 'post.failed' THEN 'email.post.failed.v1'
      WHEN 'account.disconnected' THEN 'email.account.disconnected.v1'
      WHEN 'billing.payment_failed' THEN 'email.billing.payment_failed.v1'
      WHEN 'billing.usage_80pct' THEN 'email.quota.free_plan_reminder.v1'
      ELSE d.event_type
    END AS event_key,
    d.event_type AS event_type,
    d.event_id AS trigger_event,
    COALESCE(c.workspace_id, '') AS workspace_id,
    COALESCE(w.name, '') AS workspace_name,
    c.user_id AS user_id,
    COALESCE(u.email, '') AS owner_email,
    COALESCE(c.config->>'address', '') AS email,
    ''::TEXT AS period,
    0::INTEGER AS threshold_percent,
    CASE d.status WHEN 'dead' THEN 'failed' ELSE d.status END AS status,
    '' AS transactional_id,
    d.event_id || ':' || d.channel_id AS idempotency_key,
    0::INTEGER AS effective_usage,
    0::INTEGER AS completed_usage,
    0::INTEGER AS reserved_usage,
    0::INTEGER AS post_limit,
    d.last_error AS failure_reason,
    COALESCE(d.delivered_at, d.next_retry_at, d.created_at) AS attempted_at,
    d.delivered_at AS sent_at,
    d.created_at,
    COALESCE(d.delivered_at, d.next_retry_at, d.created_at) AS updated_at,
    CASE d.status WHEN 'skipped' THEN 'notification_system' ELSE 'resend_legacy' END AS provider,
    CASE d.event_type WHEN 'billing.payment_failed' THEN 'critical_transactional' ELSE 'service_alert' END AS delivery_class,
    CASE d.event_type
      WHEN 'post.failed' THEN 'publishing_failures'
      WHEN 'account.disconnected' THEN 'account_connection_alerts'
      WHEN 'billing.usage_80pct' THEN 'usage_quota_alerts'
      ELSE 'essential_account_billing'
    END AS preference_category,
    CASE d.event_type WHEN 'billing.payment_failed' THEN 'required_notice' ELSE 'manage_preferences' END AS footer_policy,
    CASE
      WHEN d.status = 'skipped' THEN 'migrated_to_loops'
      WHEN d.event_type = 'billing.payment_failed' THEN 'required'
      ELSE 'legacy_notification_path'
    END AS preference_decision,
    'notification dispatcher' AS trigger_source,
    d.event_id AS trigger_reference_id,
    '' AS subject_snapshot
  FROM unipost_notification_deliveries d
  JOIN unipost_notification_channels c ON c.id = d.channel_id
  LEFT JOIN workspaces w ON w.id = c.workspace_id
  LEFT JOIN users u ON u.id = c.user_id
  WHERE c.kind = 'email'
)
`

const adminEmailNotificationsSelectSQL = `
SELECT
  id,
  event_key,
  event_type,
  trigger_event,
  workspace_id,
  workspace_name,
  user_id,
  owner_email,
  email,
  period,
  threshold_percent,
  status,
  transactional_id,
  idempotency_key,
  effective_usage,
  completed_usage,
  reserved_usage,
  post_limit,
  failure_reason,
  attempted_at,
  sent_at,
  created_at,
  updated_at,
  provider,
  delivery_class,
  preference_category,
  footer_policy,
  preference_decision,
  trigger_source,
  trigger_reference_id,
  subject_snapshot
FROM email_notifications
`

const adminEmailNotificationsWhereSQL = `
WHERE ($1::TEXT = '' OR status = $1)
  AND ($2::INT = 0 OR threshold_percent = $2)
  AND ($3::TEXT = '' OR period = $3)
  AND ($5::TEXT = '' OR provider = $5)
  AND ($6::TEXT = '' OR event_key = $6)
  AND (
    $4::TEXT = ''
    OR email ILIKE '%' || $4 || '%'
    OR owner_email ILIKE '%' || $4 || '%'
    OR workspace_name ILIKE '%' || $4 || '%'
    OR workspace_id ILIKE '%' || $4 || '%'
    OR user_id ILIKE '%' || $4 || '%'
    OR id ILIKE '%' || $4 || '%'
    OR event_key ILIKE '%' || $4 || '%'
    OR event_type ILIKE '%' || $4 || '%'
    OR trigger_event ILIKE '%' || $4 || '%'
    OR idempotency_key ILIKE '%' || $4 || '%'
    OR trigger_reference_id ILIKE '%' || $4 || '%'
  )
`

func adminEmailNotificationsBaseSelect() string {
	return adminEmailNotificationsCTESQL + adminEmailNotificationsSelectSQL + adminEmailNotificationsWhereSQL + `
ORDER BY attempted_at DESC, created_at DESC`
}

func normalizeAdminEmailNotificationStatus(raw string) (string, bool) {
	status := strings.TrimSpace(strings.ToLower(raw))
	switch status {
	case "", "all":
		return "", true
	case "pending", "sent", "failed", "skipped":
		return status, true
	default:
		return "", false
	}
}

func parseAdminEmailNotificationThreshold(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "all" {
		return 0, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	switch value {
	case 80, 85, 90, 95, 100:
		return value, true
	default:
		return 0, false
	}
}

func normalizeAdminPostFailurePeriod(raw string) string {
	period := strings.TrimSpace(strings.ToLower(raw))
	if period == "this_month" {
		return period
	}
	return ""
}

func (h *AdminHandler) queryPostFailures(ctx context.Context, opts adminPostFailureQuery) ([]adminPostFailure, error) {
	days := opts.Days
	if days <= 0 || days > 365 {
		days = 30
	}
	limit := opts.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	postDateFilterSQL := `(
      ($8::TEXT = 'this_month' AND sp.created_at >= date_trunc('month', NOW()))
      OR ($8::TEXT <> 'this_month' AND sp.created_at >= NOW() - ($2::INT * INTERVAL '1 day'))
    )`
	failureEventDateFilterSQL := `(
      ($8::TEXT = 'this_month' AND pf.created_at >= date_trunc('month', NOW()))
      OR ($8::TEXT <> 'this_month' AND pf.created_at >= NOW() - ($2::INT * INTERVAL '1 day'))
    )`

	rows, err := h.pool.Query(ctx, `
WITH failed_results AS (
  SELECT
    pf.id AS post_failure_id,
    spr.id AS social_post_result_id,
    sp.id AS post_id,
    u.id AS user_id,
    u.email AS user_email,
    sp.workspace_id,
    w.name AS workspace_name,
    sp.created_at,
    sp.status AS post_status,
    sp.source,
    COALESCE(NULLIF(pf.platform, ''), sa.platform) AS platform,
    sa.account_name,
    NULLIF(COALESCE(spr.caption, sp.caption), '') AS caption,
    NULLIF(COALESCE(pf.message, spr.error_message), '') AS error_message,
    NULL::TEXT AS error_summary,
    NULLIF(spr.debug_curl, '') AS debug_curl,
    NULLIF(COALESCE(pf.error_code, spr.error_code), '') AS error_code,
    NULLIF(COALESCE(pf.failure_stage, spr.failure_stage), '') AS failure_stage,
    NULLIF(COALESCE(pf.platform_error_code, spr.platform_error_code), '') AS platform_error_code,
    COALESCE(CASE WHEN pf.id IS NOT NULL THEN pf.is_retriable END, spr.is_retriable) AS is_retriable,
    NULLIF(spr.next_action, '') AS next_action
  FROM social_posts sp
  JOIN workspaces w ON w.id = sp.workspace_id
  JOIN users u ON u.id = w.user_id
  JOIN social_post_results spr ON spr.post_id = sp.id
  LEFT JOIN social_accounts sa ON sa.id = spr.social_account_id
  LEFT JOIN LATERAL (
    SELECT pf.*
    FROM post_failures pf
    WHERE pf.social_post_result_id = spr.id
    ORDER BY CASE WHEN pf.id = $5 THEN 0 ELSE 1 END, pf.created_at DESC
    LIMIT 1
  ) pf ON TRUE
  WHERE ($1::TEXT = '' OR u.id = $1)
    AND u.id != ALL($7)
    AND sp.deleted_at IS NULL
    AND `+postDateFilterSQL+`
    AND spr.status = 'failed'
    AND ($3::TEXT = '' OR sp.source = $3)
    AND ($4::TEXT = '' OR COALESCE(NULLIF(pf.platform, ''), sa.platform) = $4)
),
parent_failures AS (
  SELECT
    pf.id AS post_failure_id,
    NULL::TEXT AS social_post_result_id,
    sp.id AS post_id,
    u.id AS user_id,
    u.email AS user_email,
    sp.workspace_id,
    w.name AS workspace_name,
    sp.created_at,
    sp.status AS post_status,
    sp.source,
    NULLIF(pf.platform, '') AS platform,
    NULL::TEXT AS account_name,
    NULLIF(sp.caption, '') AS caption,
    NULLIF(pf.message, '') AS error_message,
    NULLIF(sp.metadata->>'error_summary', '') AS error_summary,
    NULL::TEXT AS debug_curl,
    NULLIF(pf.error_code, '') AS error_code,
    NULLIF(pf.failure_stage, '') AS failure_stage,
    NULLIF(pf.platform_error_code, '') AS platform_error_code,
    CASE WHEN pf.id IS NOT NULL THEN pf.is_retriable END AS is_retriable,
    NULL::TEXT AS next_action
  FROM social_posts sp
  JOIN workspaces w ON w.id = sp.workspace_id
  JOIN users u ON u.id = w.user_id
  LEFT JOIN LATERAL (
    SELECT pf.*
    FROM post_failures pf
    WHERE pf.post_id = sp.id
      AND pf.social_post_result_id IS NULL
    ORDER BY CASE WHEN pf.id = $5 THEN 0 ELSE 1 END, pf.created_at DESC
    LIMIT 1
  ) pf ON TRUE
  WHERE ($1::TEXT = '' OR u.id = $1)
    AND u.id != ALL($7)
    AND sp.deleted_at IS NULL
    AND `+postDateFilterSQL+`
    AND sp.status = 'failed'
    AND ($3::TEXT = '' OR sp.source = $3)
    AND ($4::TEXT = '' OR pf.platform = $4)
    AND NOT EXISTS (
      SELECT 1
      FROM social_post_results spr
      WHERE spr.post_id = sp.id
    )
    AND (COALESCE(sp.metadata->>'error_summary', '') <> '' OR pf.id IS NOT NULL)
),
linked_failure_events AS (
  SELECT
    pf.id AS post_failure_id,
    spr.id AS social_post_result_id,
    sp.id AS post_id,
    u.id AS user_id,
    u.email AS user_email,
    sp.workspace_id,
    w.name AS workspace_name,
    pf.created_at,
    sp.status AS post_status,
    sp.source,
    COALESCE(NULLIF(pf.platform, ''), sa.platform) AS platform,
    sa.account_name,
    NULLIF(COALESCE(spr.caption, sp.caption), '') AS caption,
    NULLIF(COALESCE(pf.message, spr.error_message), '') AS error_message,
    NULLIF(sp.metadata->>'error_summary', '') AS error_summary,
    NULLIF(spr.debug_curl, '') AS debug_curl,
    NULLIF(COALESCE(pf.error_code, spr.error_code), '') AS error_code,
    NULLIF(COALESCE(pf.failure_stage, spr.failure_stage), '') AS failure_stage,
    NULLIF(COALESCE(pf.platform_error_code, spr.platform_error_code), '') AS platform_error_code,
    pf.is_retriable AS is_retriable,
    NULLIF(spr.next_action, '') AS next_action
  FROM post_failures pf
  JOIN social_posts sp ON sp.id = pf.post_id
  JOIN workspaces w ON w.id = sp.workspace_id
  JOIN users u ON u.id = w.user_id
  LEFT JOIN social_post_results spr ON spr.id = pf.social_post_result_id
  LEFT JOIN social_accounts sa ON sa.id = spr.social_account_id
  WHERE ($1::TEXT = '' OR u.id = $1)
    AND u.id != ALL($7)
    AND sp.deleted_at IS NULL
    AND `+failureEventDateFilterSQL+`
    AND ($3::TEXT = '' OR sp.source = $3)
    AND ($4::TEXT = '' OR COALESCE(NULLIF(pf.platform, ''), sa.platform) = $4)
    AND $5::TEXT <> ''
    AND (pf.id = $5 OR COALESCE(spr.id, '') = $5 OR sp.id = $5)
    AND NOT (
      (pf.social_post_result_id IS NOT NULL AND spr.status = 'failed')
      OR (
        pf.social_post_result_id IS NULL
        AND sp.status = 'failed'
        AND NOT EXISTS (
          SELECT 1
          FROM social_post_results child_spr
          WHERE child_spr.post_id = sp.id
        )
      )
    )
)
SELECT
  post_failure_id,
  social_post_result_id,
  post_id,
  user_id,
  user_email,
  workspace_id,
  workspace_name,
  created_at,
  post_status,
  source,
  platform,
  account_name,
  caption,
  error_message,
  error_summary,
  debug_curl,
  error_code,
  failure_stage,
  platform_error_code,
  is_retriable,
  next_action
FROM (
  SELECT * FROM failed_results
  UNION ALL
  SELECT * FROM parent_failures
  UNION ALL
  SELECT * FROM linked_failure_events
) failures
WHERE (
  $5::TEXT = ''
  OR COALESCE(post_failure_id, '') = $5
  OR COALESCE(social_post_result_id, '') = $5
  OR post_id = $5
  OR workspace_id = $5
  OR user_id = $5
  OR user_email ILIKE '%' || $5 || '%'
  OR workspace_name ILIKE '%' || $5 || '%'
  OR COALESCE(account_name, '') ILIKE '%' || $5 || '%'
  OR COALESCE(caption, '') ILIKE '%' || $5 || '%'
  OR COALESCE(error_message, error_summary, '') ILIKE '%' || $5 || '%'
  OR COALESCE(error_code, '') ILIKE '%' || $5 || '%'
  OR COALESCE(failure_stage, '') ILIKE '%' || $5 || '%'
  OR COALESCE(platform_error_code, '') ILIKE '%' || $5 || '%'
)
ORDER BY created_at DESC
LIMIT $6
`, opts.UserID, days, opts.Source, opts.Platform, opts.Search, limit, opts.Excluded, strings.TrimSpace(opts.Period))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]adminPostFailure, 0)
	for rows.Next() {
		var item adminPostFailure
		var platform, accountName, caption, errorMessage, errorSummary, debugCurl *string
		var postFailureID, socialPostResultID, errorCode, failureStage, platformErrorCode, nextAction *string
		var isRetriable *bool
		if err := rows.Scan(
			&postFailureID,
			&socialPostResultID,
			&item.PostID,
			&item.UserID,
			&item.UserEmail,
			&item.WorkspaceID,
			&item.WorkspaceName,
			&item.CreatedAt,
			&item.PostStatus,
			&item.Source,
			&platform,
			&accountName,
			&caption,
			&errorMessage,
			&errorSummary,
			&debugCurl,
			&errorCode,
			&failureStage,
			&platformErrorCode,
			&isRetriable,
			&nextAction,
		); err != nil {
			return nil, err
		}
		item.PostFailureID = postFailureID
		item.SocialPostResultID = socialPostResultID
		item.Platform = platform
		item.AccountName = accountName
		item.Caption = caption
		item.ErrorMessage = errorMessage
		item.ErrorSummary = errorSummary
		item.DebugCurl = debugCurl
		item.ErrorCode = errorCode
		item.FailureStage = failureStage
		item.PlatformErrorCode = platformErrorCode
		item.IsRetriable = isRetriable
		item.NextAction = nextAction
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func adminPostPlatformsSQL(postAlias string) string {
	return strings.ReplaceAll(`
COALESCE((
  SELECT array_agg(DISTINCT post_platforms.platform ORDER BY post_platforms.platform)
  FROM (
    SELECT sa_result.platform
    FROM social_post_results spr_platforms
    JOIN social_accounts sa_result ON sa_result.id = spr_platforms.social_account_id
    WHERE spr_platforms.post_id = __POST_ALIAS__.id

    UNION

    SELECT sa_target.platform
    FROM jsonb_array_elements(
      CASE
        WHEN jsonb_typeof(__POST_ALIAS__.metadata->'platform_posts') = 'array' THEN __POST_ALIAS__.metadata->'platform_posts'
        ELSE '[]'::jsonb
      END
    ) AS target_post(value)
    JOIN social_accounts sa_target ON sa_target.id = target_post.value->>'account_id'

    UNION

    SELECT sa_legacy.platform
    FROM jsonb_array_elements_text(
      CASE
        WHEN jsonb_typeof(__POST_ALIAS__.metadata->'account_ids') = 'array' THEN __POST_ALIAS__.metadata->'account_ids'
        ELSE '[]'::jsonb
      END
    ) AS target_account(account_id)
    JOIN social_accounts sa_legacy ON sa_legacy.id = target_account.account_id
  ) post_platforms
), '{}')`, "__POST_ALIAS__", postAlias)
}

func adminPostPlatformFilterSQL(postAlias, param string) string {
	return param + "::TEXT = '' OR " + param + " = ANY(" + adminPostPlatformsSQL(postAlias) + ")"
}

func (h *AdminHandler) queryPosts(ctx context.Context, opts adminPostsQuery) ([]adminPostRow, error) {
	startAt, endAt := adminPostsWindow(opts)
	limit := opts.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	platformsSQL := adminPostPlatformsSQL("sp")

	rows, err := h.pool.Query(ctx, `
WITH post_rollup AS (
  SELECT
    sp.id AS post_id,
    u.id AS user_id,
    u.email AS user_email,
    sp.workspace_id,
    w.name AS workspace_name,
    sp.status,
    sp.source,
    NULLIF(sp.caption, '') AS caption,
    sp.created_at,
    sp.scheduled_at,
    sp.published_at,
    `+platformsSQL+` AS platforms,
    COUNT(spr.id)::BIGINT AS result_count,
    COUNT(*) FILTER (WHERE spr.status = 'published')::BIGINT AS published_result_count,
    COUNT(*) FILTER (WHERE spr.status = 'failed')::BIGINT AS failed_result_count
  FROM social_posts sp
  JOIN workspaces w ON w.id = sp.workspace_id
  JOIN users u ON u.id = w.user_id
  LEFT JOIN social_post_results spr ON spr.post_id = sp.id
  WHERE u.id != ALL($1)
    AND sp.deleted_at IS NULL
    AND ($2::TIMESTAMPTZ IS NULL OR sp.created_at >= $2)
    AND ($11::TIMESTAMPTZ IS NULL OR sp.created_at < $11)
    AND ($3::TEXT = '' OR sp.status = $3)
    AND ($4::TEXT = '' OR sp.source = $4)
    AND ($8::TEXT = '' OR u.id = $8)
    AND ($9::TEXT = '' OR sp.workspace_id = $9)
  GROUP BY
    sp.id, u.id, u.email, sp.workspace_id, w.name, sp.status, sp.source,
    sp.caption, sp.metadata, sp.created_at, sp.scheduled_at, sp.published_at
)
SELECT
  post_id,
  user_id,
  user_email,
  workspace_id,
  workspace_name,
  status,
  source,
  caption,
  created_at,
  scheduled_at,
  published_at,
  platforms,
  result_count,
  published_result_count,
  failed_result_count
FROM post_rollup
WHERE ($5::TEXT = '' OR $5 = ANY(platforms))
  AND ($10::TEXT = '' OR ($10 = 'failed' AND failed_result_count > 0))
  AND (
    $6::TEXT = ''
    OR user_email ILIKE '%' || $6 || '%'
    OR workspace_name ILIKE '%' || $6 || '%'
    OR COALESCE(caption, '') ILIKE '%' || $6 || '%'
    OR post_id ILIKE '%' || $6 || '%'
  )
ORDER BY created_at DESC
LIMIT $7
`, opts.Excluded, startAt, opts.Status, opts.Source, opts.Platform, opts.Search, limit, opts.UserID, opts.WorkspaceID, opts.ResultStatus, endAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]adminPostRow, 0)
	for rows.Next() {
		var item adminPostRow
		var caption *string
		var scheduledAt, publishedAt *time.Time
		if err := rows.Scan(
			&item.PostID,
			&item.UserID,
			&item.UserEmail,
			&item.WorkspaceID,
			&item.WorkspaceName,
			&item.Status,
			&item.Source,
			&caption,
			&item.CreatedAt,
			&scheduledAt,
			&publishedAt,
			&item.Platforms,
			&item.ResultCount,
			&item.PublishedResultCount,
			&item.FailedResultCount,
		); err != nil {
			return nil, err
		}
		item.Caption = caption
		item.ScheduledAt = scheduledAt
		item.PublishedAt = publishedAt
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

// queryPostsAggregates produces the headline / per-platform / daily
// numbers for the admin posts page. Same WHERE clause as queryPosts so
// the cards and the table always describe the same set; LIMIT only
// applies to the row list, never to the aggregates.
//
// Three independent queries instead of one mega-CTE because each
// rolls up at a different grain (post / result / day) and PostgreSQL
// can't share the scan across them anyway. Total round-trip stays
// under ~50 ms on the indexes we already have on social_posts.
func (h *AdminHandler) queryPostsAggregates(ctx context.Context, opts adminPostsQuery) (*adminPostsAggregatesResponse, error) {
	startAt, endAt := adminPostsWindow(opts)

	// Common WHERE used by all three queries. Parameter ordering must
	// match every Query() call below.
	platformFilterSQL := adminPostPlatformFilterSQL("sp", "$7")
	commonWhere := `
  WHERE u.id != ALL($1)
    AND sp.deleted_at IS NULL
    AND ($2::TIMESTAMPTZ IS NULL OR sp.created_at >= $2)
    AND ($10::TIMESTAMPTZ IS NULL OR sp.created_at < $10)
    AND ($3::TEXT = '' OR sp.status = $3)
    AND ($4::TEXT = '' OR sp.source = $4)
    AND ($5::TEXT = '' OR u.id = $5)
    AND ($6::TEXT = '' OR sp.workspace_id = $6)
    AND (` + platformFilterSQL + `)
    AND ($9::TEXT = '' OR ($9 = 'failed' AND EXISTS (
      SELECT 1 FROM social_post_results spr_filter
      WHERE spr_filter.post_id = sp.id AND spr_filter.status = 'failed'
    )))
    AND (
      $8::TEXT = ''
      OR u.email ILIKE '%' || $8 || '%'
      OR w.name ILIKE '%' || $8 || '%'
      OR COALESCE(sp.caption, '') ILIKE '%' || $8 || '%'
      OR sp.id ILIKE '%' || $8 || '%'
    )`

	args := []any{
		opts.Excluded, startAt,
		opts.Status, opts.Source,
		opts.UserID, opts.WorkspaceID,
		opts.Platform, opts.Search,
		opts.ResultStatus, endAt,
	}

	out := &adminPostsAggregatesResponse{
		ByStatus:   map[string]int64{},
		ByPlatform: []adminPostsPlatformAggregate{},
		Events:     []adminPostsEvent{},
	}

	// 1) Headline: total posts + status breakdown + unique users.
	statusRows, err := h.pool.Query(ctx, `
SELECT sp.status, COUNT(*)::BIGINT AS cnt, COUNT(DISTINCT u.id)::BIGINT AS users
FROM social_posts sp
JOIN workspaces w ON w.id = sp.workspace_id
JOIN users u ON u.id = w.user_id
`+commonWhere+`
GROUP BY sp.status`, args...)
	if err != nil {
		return nil, err
	}
	defer statusRows.Close()
	uniqueUserSet := map[string]struct{}{}
	for statusRows.Next() {
		var status string
		var cnt, users int64
		if err := statusRows.Scan(&status, &cnt, &users); err != nil {
			return nil, err
		}
		out.ByStatus[status] = cnt
		out.TotalPosts += cnt
		// Per-status user counts overlap; recompute total uniques in
		// one query below rather than trying to dedupe across groups.
		_ = users
		_ = uniqueUserSet
	}
	if err := statusRows.Err(); err != nil {
		return nil, err
	}

	// Unique users — one extra small query is cheaper than carrying
	// per-row user_ids back to Go and deduping.
	if err := h.pool.QueryRow(ctx, `
SELECT COUNT(DISTINCT u.id)::BIGINT
FROM social_posts sp
JOIN workspaces w ON w.id = sp.workspace_id
JOIN users u ON u.id = w.user_id
`+commonWhere, args...).Scan(&out.UniqueUsers); err != nil {
		return nil, err
	}

	// 2) Per-platform aggregates — RESULT level. One social_post_results
	// row equals one platform attempt; this lets a Twitter-published /
	// LinkedIn-failed post split across the two cards correctly.
	platformRows, err := h.pool.Query(ctx, `
SELECT
  sa.platform,
  COUNT(*) FILTER (WHERE spr.status = 'published')::BIGINT AS published,
  COUNT(*) FILTER (WHERE spr.status = 'failed')::BIGINT    AS failed,
  COUNT(*)::BIGINT                                          AS total
FROM social_posts sp
JOIN workspaces w ON w.id = sp.workspace_id
JOIN users u ON u.id = w.user_id
JOIN social_post_results spr ON spr.post_id = sp.id
JOIN social_accounts sa ON sa.id = spr.social_account_id
`+commonWhere+`
GROUP BY sa.platform
ORDER BY sa.platform`, args...)
	if err != nil {
		return nil, err
	}
	defer platformRows.Close()
	for platformRows.Next() {
		var item adminPostsPlatformAggregate
		if err := platformRows.Scan(&item.Platform, &item.Published, &item.Failed, &item.Total); err != nil {
			return nil, err
		}
		out.ByPlatform = append(out.ByPlatform, item)
	}
	if err := platformRows.Err(); err != nil {
		return nil, err
	}

	// 3) Raw events for the chart — published/failed parent posts only,
	// since those are the two lines rendered. Caller buckets these into
	// local-time days so late-evening posts don't shift to the next UTC
	// calendar day.
	eventRows, err := h.pool.Query(ctx, `
SELECT sp.created_at, sp.status
FROM social_posts sp
JOIN workspaces w ON w.id = sp.workspace_id
JOIN users u ON u.id = w.user_id
`+commonWhere+`
  AND sp.status IN ('published', 'failed')
ORDER BY sp.created_at ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer eventRows.Close()
	for eventRows.Next() {
		var item adminPostsEvent
		if err := eventRows.Scan(&item.CreatedAt, &item.Status); err != nil {
			return nil, err
		}
		out.Events = append(out.Events, item)
	}
	if err := eventRows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (h *AdminHandler) queryBilling(ctx context.Context, opts adminBillingQuery) ([]adminBillingRow, error) {
	days := opts.Days
	if days <= 0 || days > 365 {
		days = 90
	}
	limit := opts.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}

	rows, err := h.pool.Query(ctx, `
SELECT
  w.id AS workspace_id,
  w.name AS workspace_name,
  u.id AS user_id,
  u.email AS user_email,
  s.plan_id,
  pl.name AS plan_name,
  pl.price_cents::BIGINT,
  s.status,
  NULLIF(s.stripe_customer_id, '') AS stripe_customer_id,
  NULLIF(s.stripe_subscription_id, '') AS stripe_subscription_id,
  s.current_period_end,
  COALESCE(s.cancel_at_period_end, false) AS cancel_at_period_end,
  s.trial_used,
  COALESCE(usg.post_count, 0)::BIGINT AS posts_used,
  COALESCE(pl.post_limit, 0)::BIGINT AS post_limit,
  s.updated_at
FROM subscriptions s
JOIN workspaces w ON w.id = s.workspace_id
JOIN users u ON u.id = w.user_id
JOIN plans pl ON pl.id = s.plan_id
LEFT JOIN usage usg ON usg.workspace_id = w.id AND usg.period = to_char(NOW(), 'YYYY-MM')
WHERE u.id != ALL($1)
  AND s.updated_at >= NOW() - ($2::INT * INTERVAL '1 day')
  AND ($3::TEXT = '' OR s.status = $3)
  AND ($4::TEXT = '' OR s.plan_id = $4)
  AND (
    $5::TEXT = ''
    OR u.email ILIKE '%' || $5 || '%'
    OR w.name ILIKE '%' || $5 || '%'
    OR s.plan_id ILIKE '%' || $5 || '%'
  )
ORDER BY pl.price_cents DESC, s.updated_at DESC
LIMIT $6
`, opts.Excluded, days, opts.Status, opts.PlanID, opts.Search, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]adminBillingRow, 0)
	for rows.Next() {
		var item adminBillingRow
		var stripeCustomerID, stripeSubscriptionID *string
		var currentPeriodEnd *time.Time
		if err := rows.Scan(
			&item.WorkspaceID,
			&item.WorkspaceName,
			&item.UserID,
			&item.UserEmail,
			&item.PlanID,
			&item.PlanName,
			&item.PriceCents,
			&item.Status,
			&stripeCustomerID,
			&stripeSubscriptionID,
			&currentPeriodEnd,
			&item.CancelAtPeriodEnd,
			&item.TrialUsed,
			&item.PostsUsed,
			&item.PostLimit,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.StripeCustomerID = stripeCustomerID
		item.StripeSubscriptionID = stripeSubscriptionID
		item.CurrentPeriodEnd = currentPeriodEnd
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")

	var d adminUserDetailResponse
	var name *string
	var lastPostAt *time.Time
	err := h.pool.QueryRow(r.Context(), `
SELECT
  u.id, u.email, u.name, u.created_at,
  COALESCE((
    SELECT NULLIF(lv.country_code, '')
    FROM landing_session_users lsu
    JOIN landing_visits lv ON lv.session_id = lsu.session_id
    WHERE lsu.user_id = u.id
      AND NULLIF(lv.country_code, '') IS NOT NULL
    ORDER BY lsu.first_bound_at ASC, lv.created_at ASC
    LIMIT 1
  ), ''),
  (SELECT COUNT(*) FROM workspaces w WHERE w.user_id = u.id),
  (SELECT COUNT(*) FROM api_keys ak JOIN workspaces w ON w.id = ak.workspace_id WHERE w.user_id = u.id AND ak.revoked_at IS NULL),
  (SELECT COUNT(*) FROM social_accounts sa JOIN profiles p ON p.id = sa.profile_id JOIN workspaces w ON w.id = p.workspace_id WHERE w.user_id = u.id AND sa.disconnected_at IS NULL),
  COALESCE((SELECT array_agg(DISTINCT sa.platform) FROM social_accounts sa JOIN profiles p ON p.id = sa.profile_id JOIN workspaces w ON w.id = p.workspace_id WHERE w.user_id = u.id AND sa.disconnected_at IS NULL), '{}'),
  COALESCE((SELECT SUM(usg.post_count)::bigint FROM usage usg JOIN workspaces w ON w.id = usg.workspace_id WHERE w.user_id = u.id AND usg.period = to_char(NOW(), 'YYYY-MM')), 0),
  CASE WHEN EXISTS(SELECT 1 FROM subscriptions s JOIN plans pl ON pl.id = s.plan_id JOIN workspaces w ON w.id = s.workspace_id WHERE w.user_id = u.id AND s.status='active' AND pl.post_limit < 0)
    THEN -1
    ELSE COALESCE((SELECT SUM(pl.post_limit)::bigint FROM subscriptions s JOIN plans pl ON pl.id = s.plan_id JOIN workspaces w ON w.id = s.workspace_id WHERE w.user_id = u.id AND s.status='active'), 0)
  END,
  COALESCE((SELECT SUM(pl.price_cents)::bigint FROM subscriptions s JOIN plans pl ON pl.id = s.plan_id JOIN workspaces w ON w.id = s.workspace_id WHERE w.user_id = u.id AND s.status='active'), 0),
  (SELECT COUNT(*) FROM social_posts sp JOIN workspaces w ON w.id = sp.workspace_id WHERE w.user_id = u.id),
  (SELECT COUNT(*) FROM social_posts sp JOIN workspaces w ON w.id = sp.workspace_id WHERE w.user_id = u.id AND sp.status='failed' AND sp.created_at >= NOW() - INTERVAL '30 days'),
  (SELECT MAX(sp.published_at) FROM social_posts sp JOIN workspaces w ON w.id = sp.workspace_id WHERE w.user_id = u.id)
FROM users u
WHERE u.id = $1
`, userID).Scan(
		&d.ID, &d.Email, &name, &d.CreatedAt,
		&d.SignupCountry,
		&d.WorkspaceCount, &d.APIKeyCount, &d.PlatformCount,
		&d.Platforms,
		&d.PostsUsedThisMonth, &d.PostLimit, &d.MRRCents,
		&d.TotalPosts, &d.FailedPosts30d, &lastPostAt,
	)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "User not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load user: "+err.Error())
		return
	}
	if name != nil {
		d.Name = *name
	}
	d.LastPostAt = lastPostAt

	// Per-workspace breakdown
	rows, err := h.pool.Query(r.Context(), `
SELECT
  w.id, w.name, w.created_at,
  COALESCE(s.plan_id, 'free'),
  COALESCE(pl.name, 'Free'),
  COALESCE(pl.price_cents, 0),
  COALESCE((SELECT post_count FROM usage WHERE workspace_id = w.id AND period = to_char(NOW(),'YYYY-MM')), 0),
  COALESCE(pl.post_limit, 100),
  COALESCE(s.status, 'active'),
  (SELECT COUNT(*) FROM social_accounts sa JOIN profiles p ON p.id = sa.profile_id WHERE p.workspace_id = w.id AND sa.disconnected_at IS NULL)
FROM workspaces w
LEFT JOIN subscriptions s ON s.workspace_id = w.id
LEFT JOIN plans pl ON pl.id = s.plan_id
WHERE w.user_id = $1
ORDER BY w.created_at DESC
`, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load workspaces: "+err.Error())
		return
	}
	defer rows.Close()

	d.Workspaces = make([]adminUserWorkspace, 0)
	for rows.Next() {
		var ws adminUserWorkspace
		var posts int64
		if err := rows.Scan(
			&ws.ID, &ws.Name, &ws.CreatedAt,
			&ws.PlanID, &ws.PlanName, &ws.PriceCents,
			&posts, &ws.PostLimit, &ws.Status, &ws.PlatformCount,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to scan workspace: "+err.Error())
			return
		}
		ws.PostsUsed = posts
		d.Workspaces = append(d.Workspaces, ws)
	}

	writeSuccess(w, d)
}

func adminScheduledPostTitle(caption *string) string {
	if caption == nil {
		return "Untitled scheduled post"
	}
	for _, line := range strings.Split(*caption, "\n") {
		title := strings.TrimSpace(line)
		if title == "" {
			continue
		}
		const maxRunes = 80
		runes := []rune(title)
		if len(runes) > maxRunes {
			return string(runes[:maxRunes])
		}
		return title
	}
	return "Untitled scheduled post"
}

func (h *AdminHandler) ListUserScheduledPosts(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	platformsSQL := adminPostPlatformsSQL("sp")

	rows, err := h.pool.Query(r.Context(), `
SELECT
  sp.id,
  NULLIF(sp.caption, '') AS caption,
  sp.created_at,
  sp.scheduled_at,
  `+platformsSQL+` AS platforms
FROM social_posts sp
JOIN workspaces w ON w.id = sp.workspace_id
WHERE w.user_id = $1
  AND sp.status = 'scheduled'
  AND sp.deleted_at IS NULL
ORDER BY sp.scheduled_at ASC NULLS LAST, sp.created_at DESC
`, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load scheduled posts: "+err.Error())
		return
	}
	defer rows.Close()

	out := make([]adminUserScheduledPost, 0)
	for rows.Next() {
		var item adminUserScheduledPost
		var caption *string
		var scheduledAt *time.Time
		if err := rows.Scan(
			&item.PostID,
			&caption,
			&item.CreatedAt,
			&scheduledAt,
			&item.Platforms,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to scan scheduled post: "+err.Error())
			return
		}
		item.Title = adminScheduledPostTitle(caption)
		item.ScheduledAt = scheduledAt
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to iterate scheduled posts: "+err.Error())
		return
	}

	writeSuccess(w, out)
}

func (h *AdminHandler) ListUserPostFailures(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	q := r.URL.Query()

	days, _ := strconv.Atoi(q.Get("days"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	out, err := h.queryPostFailures(r.Context(), adminPostFailureQuery{
		UserID:   userID,
		Period:   normalizeAdminPostFailurePeriod(r.URL.Query().Get("period")),
		Days:     days,
		Limit:    limit,
		Excluded: []string{},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load post failures: "+err.Error())
		return
	}

	writeSuccess(w, out)
}

func (h *AdminHandler) ListPostFailures(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	days, _ := strconv.Atoi(q.Get("days"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	excluded, err := h.excludedUserIDs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to resolve excluded users: "+err.Error())
		return
	}

	out, err := h.queryPostFailures(r.Context(), adminPostFailureQuery{
		UserID:   strings.TrimSpace(q.Get("user_id")),
		Search:   q.Get("search"),
		Platform: q.Get("platform"),
		Source:   q.Get("source"),
		Period:   normalizeAdminPostFailurePeriod(q.Get("period")),
		Days:     days,
		Limit:    limit,
		Excluded: excluded,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load post failures: "+err.Error())
		return
	}

	writeSuccess(w, out)
}

func (h *AdminHandler) ListPosts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	days, _ := strconv.Atoi(q.Get("days"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	resultStatus := normalizeAdminPostResultStatus(q.Get("result_status"))
	startAt := parseAdminTimeParam(q.Get("start_at"))
	endAt := parseAdminTimeParam(q.Get("end_at"))
	all := q.Get("all") == "true"

	excluded, err := h.excludedUserIDs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to resolve excluded users: "+err.Error())
		return
	}

	out, err := h.queryPosts(r.Context(), adminPostsQuery{
		Search:       q.Get("search"),
		Status:       q.Get("status"),
		ResultStatus: resultStatus,
		Platform:     q.Get("platform"),
		Source:       q.Get("source"),
		UserID:       q.Get("user_id"),
		WorkspaceID:  q.Get("workspace_id"),
		Days:         days,
		StartAt:      startAt,
		EndAt:        endAt,
		All:          all,
		Limit:        limit,
		Excluded:     excluded,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load posts: "+err.Error())
		return
	}

	writeSuccess(w, out)
}

// ListPostsAggregates serves GET /v1/admin/posts/aggregates. Same
// filter params as ListPosts; returns headline / per-platform /
// per-day numbers without the row list.
func (h *AdminHandler) ListPostsAggregates(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	days, _ := strconv.Atoi(q.Get("days"))
	resultStatus := normalizeAdminPostResultStatus(q.Get("result_status"))
	startAt := parseAdminTimeParam(q.Get("start_at"))
	endAt := parseAdminTimeParam(q.Get("end_at"))
	all := q.Get("all") == "true"

	excluded, err := h.excludedUserIDs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to resolve excluded users: "+err.Error())
		return
	}

	out, err := h.queryPostsAggregates(r.Context(), adminPostsQuery{
		Search:       q.Get("search"),
		Status:       q.Get("status"),
		ResultStatus: resultStatus,
		Platform:     q.Get("platform"),
		Source:       q.Get("source"),
		UserID:       q.Get("user_id"),
		WorkspaceID:  q.Get("workspace_id"),
		Days:         days,
		StartAt:      startAt,
		EndAt:        endAt,
		All:          all,
		Excluded:     excluded,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load aggregates: "+err.Error())
		return
	}

	writeSuccess(w, out)
}

func normalizeAdminPostResultStatus(value string) string {
	if value == "failed" {
		return value
	}
	return ""
}

func (h *AdminHandler) queryEmailNotifications(ctx context.Context, opts adminEmailNotificationsQuery) ([]adminEmailNotificationRow, int64, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	args := []any{
		strings.TrimSpace(opts.Status),
		opts.Threshold,
		strings.TrimSpace(opts.Period),
		strings.TrimSpace(opts.Search),
		strings.TrimSpace(opts.Provider),
		strings.TrimSpace(opts.EventKey),
	}

	var total int64
	if err := h.pool.QueryRow(ctx, adminEmailNotificationsCTESQL+`
SELECT COUNT(*)::BIGINT
FROM email_notifications
`+adminEmailNotificationsWhereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := h.pool.Query(ctx, adminEmailNotificationsBaseSelect()+`
LIMIT $7 OFFSET $8`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]adminEmailNotificationRow, 0, limit)
	for rows.Next() {
		var item adminEmailNotificationRow
		var failureReason *string
		var sentAt *time.Time
		if err := rows.Scan(
			&item.ID,
			&item.EventKey,
			&item.EventType,
			&item.TriggerEvent,
			&item.WorkspaceID,
			&item.WorkspaceName,
			&item.UserID,
			&item.OwnerEmail,
			&item.Email,
			&item.Period,
			&item.ThresholdPercent,
			&item.Status,
			&item.TransactionalID,
			&item.IdempotencyKey,
			&item.EffectiveUsage,
			&item.CompletedUsage,
			&item.ReservedUsage,
			&item.PostLimit,
			&failureReason,
			&item.AttemptedAt,
			&sentAt,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.Provider,
			&item.DeliveryClass,
			&item.PreferenceCategory,
			&item.FooterPolicy,
			&item.PreferenceDecision,
			&item.TriggerSource,
			&item.TriggerReference,
			&item.SubjectSnapshot,
		); err != nil {
			return nil, 0, err
		}
		item.FailureReason = failureReason
		item.SentAt = sentAt
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return out, total, nil
}

// ListEmailNotifications serves GET /v1/admin/email-notifications.
// It is the read-only operational view for user-facing email sends and
// migration audit rows across Loops, quota, support, and legacy
// notification-email paths.
func (h *AdminHandler) ListEmailNotifications(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	status, ok := normalizeAdminEmailNotificationStatus(q.Get("status"))
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "status must be one of: all, pending, sent, failed, skipped")
		return
	}
	threshold, ok := parseAdminEmailNotificationThreshold(q.Get("threshold"))
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "threshold must be one of: 80, 85, 90, 95, 100")
		return
	}

	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	offset, _ := strconv.Atoi(q.Get("offset"))
	out, total, err := h.queryEmailNotifications(r.Context(), adminEmailNotificationsQuery{
		Search:    q.Get("search"),
		Status:    status,
		Provider:  q.Get("provider"),
		EventKey:  q.Get("event_key"),
		Threshold: threshold,
		Period:    q.Get("period"),
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load email notifications: "+err.Error())
		return
	}

	writeSuccessWithListMeta(w, out, int(total), limit)
}

// SetPlan flips a workspace's subscription.plan_id directly, bypassing
// Stripe entirely. This is a developer/QA tool — there is no payment
// flow, no proration, no Stripe webhook side effect. Used for:
//
//   - end-to-end testing of the plan-feature gates (Inbox, Analytics,
//     profile-cap, daily-cap) without going through Stripe Checkout
//   - rescuing a customer whose Stripe webhook missed
//   - manually granting comp / Enterprise access ahead of contract sign
//
// The admin middleware (RequireAdminMiddleware in cmd/api/main.go)
// protects this endpoint — only ADMIN_USERS / SUPER_ADMINS can reach
// it. The route is also intentionally unmentioned in any docs page.
//
// Body: {plan_id: "free|api|basic|growth|team|enterprise"}
// Response: 204 No Content on success.
func (h *AdminHandler) SetPlan(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "workspace ID required")
		return
	}

	var body struct {
		PlanID string `json:"plan_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid request body: "+err.Error())
		return
	}

	// Allowlist mirrors plans.id rows from migration 058 + 057's free.
	// Hardcoded so a typo in the plans table doesn't open up the gate;
	// when adding a new tier, update this list AND the migration.
	allowed := map[string]bool{
		"free":       true,
		"api":        true,
		"basic":      true,
		"growth":     true,
		"team":       true,
		"enterprise": true,
	}
	if !allowed[body.PlanID] {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"plan_id must be one of: free, api, basic, growth, team, enterprise")
		return
	}

	// Workspace existence check — surface 404 instead of silently
	// upserting against a non-existent workspace.
	var exists bool
	if err := h.pool.QueryRow(r.Context(),
		`SELECT EXISTS (SELECT 1 FROM workspaces WHERE id = $1)`,
		workspaceID).Scan(&exists); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to check workspace: "+err.Error())
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "workspace not found")
		return
	}

	// Capture the previous plan_id for the audit trail before we
	// overwrite it. Best-effort — if it fails, the audit row just has
	// a nil "before" snapshot.
	var previousPlan string
	_ = h.pool.QueryRow(r.Context(),
		`SELECT plan_id FROM subscriptions WHERE workspace_id = $1`,
		workspaceID).Scan(&previousPlan)

	// Upsert: create the subscription row if missing, otherwise flip
	// plan_id + reset status to active. status='active' matches the
	// shape Stripe webhooks would produce on a successful change.
	if _, err := h.pool.Exec(r.Context(),
		`INSERT INTO subscriptions (workspace_id, plan_id, status)
		 VALUES ($1, $2, 'active')
		 ON CONFLICT (workspace_id) DO UPDATE
		 SET plan_id = EXCLUDED.plan_id, status = 'active', updated_at = NOW()`,
		workspaceID, body.PlanID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update subscription: "+err.Error())
		return
	}

	audit.Log(r.Context(), h.queries, audit.Event{
		WorkspaceID:  workspaceID,
		ActorUserID:  auth.GetUserID(r.Context()),
		Action:       audit.ActionPlanChanged,
		ResourceType: "subscription",
		ResourceID:   workspaceID,
		Category:     audit.CategoryBilling,
		IPAddress:    r.RemoteAddr,
		UserAgent:    r.UserAgent(),
		Before:       map[string]any{"plan_id": previousPlan},
		After:        map[string]any{"plan_id": body.PlanID},
		Metadata:     map[string]any{"source": "admin_flip", "stripe": false},
	})

	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandler) ListBilling(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	days, _ := strconv.Atoi(q.Get("days"))
	limit, _ := strconv.Atoi(q.Get("limit"))

	excluded, err := h.excludedUserIDs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to resolve excluded users: "+err.Error())
		return
	}

	out, err := h.queryBilling(r.Context(), adminBillingQuery{
		Search:   q.Get("search"),
		Status:   q.Get("status"),
		PlanID:   q.Get("plan"),
		Days:     days,
		Limit:    limit,
		Excluded: excluded,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load billing rows: "+err.Error())
		return
	}

	writeSuccess(w, out)
}
