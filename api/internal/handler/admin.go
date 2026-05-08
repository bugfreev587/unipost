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
  AND ($2::TEXT = '' OR l.category = $2)
  AND ($3::TEXT = '' OR l.action = $3)
  AND ($4::TEXT = '' OR l.source = $4)
  AND ($5::TEXT = '' OR l.level = $5)
  AND ($6::TEXT = '' OR l.status = $6)
  AND ($7::TEXT = '' OR l.platform = $7)
  AND ($8::TEXT = '' OR l.profile_id = $8)
  AND ($9::TEXT = '' OR l.social_account_id = $9)
  AND ($10::TEXT = '' OR l.post_id = $10)
  AND ($11::TEXT = '' OR l.request_id = $11)
  AND ($12::TEXT = '' OR l.error_code = $12)
  AND (
    $13::TEXT = ''
    OR l.message ILIKE '%' || $13 || '%'
    OR l.action ILIKE '%' || $13 || '%'
    OR l.request_id ILIKE '%' || $13 || '%'
    OR l.post_id ILIKE '%' || $13 || '%'
    OR l.error_code ILIKE '%' || $13 || '%'
  )
  AND l.ts >= $14
  AND l.ts <= $15
ORDER BY l.ts DESC, l.id DESC
LIMIT $16`

	rows, err := h.pool.Query(r.Context(), sql,
		strings.TrimSpace(q.Get("workspace_id")),
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
	ID             string     `json:"id"`
	Email          string     `json:"email"`
	CreatedAt      time.Time  `json:"created_at"`
	WorkspaceCount int64      `json:"workspace_count"`
	APIKeyCount    int64      `json:"api_key_count"`
	PlatformCount  int64      `json:"platform_count"`
	Platforms      []string   `json:"platforms"`
	PostsUsed      int64      `json:"posts_used"`
	PostLimit      int64      `json:"post_limit"`
	MRRCents       int64      `json:"mrr_cents"`
	IsPaid         bool       `json:"is_paid"`
	LastPostAt     *time.Time `json:"last_post_at"`
}

// adminUserSignupTrendResponse returns raw signup timestamps so the
// dashboard can bucket them in the viewer's local timezone. Server-side
// bucketing was using the DB session timezone (UTC) which shifted late-
// evening signups in the Americas to the next calendar day.
type adminUserSignupTrendResponse struct {
	RangeDays int         `json:"range_days"`
	Events    []time.Time `json:"events"`
}

var adminUserSortOrders = map[string]string{
	"newest":      "created_at DESC",
	"mrr":         "mrr_cents DESC, created_at DESC",
	"usage":       "posts_used DESC, created_at DESC",
	"last_active": "last_post_at DESC NULLS LAST, created_at DESC",
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	search := q.Get("search")
	plan := q.Get("plan")
	if plan == "" {
		plan = "all"
	}
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

	planFilter := ""
	switch plan {
	case "paid":
		planFilter = `AND EXISTS(
			SELECT 1 FROM subscriptions s
			JOIN plans pl ON pl.id = s.plan_id
			JOIN workspaces w ON w.id = s.workspace_id
			WHERE w.user_id = u.id AND s.status='active' AND pl.price_cents > 0
		)`
	case "free":
		planFilter = `AND NOT EXISTS(
			SELECT 1 FROM subscriptions s
			JOIN plans pl ON pl.id = s.plan_id
			JOIN workspaces w ON w.id = s.workspace_id
			WHERE w.user_id = u.id AND s.status='active' AND pl.price_cents > 0
		)`
	}

	excluded, err := h.excludedUserIDs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to resolve excluded users: "+err.Error())
		return
	}

	sql := `
WITH base AS (
  SELECT
    u.id, u.email, u.created_at,
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
    COALESCE((SELECT SUM(pl.post_limit)::bigint
       FROM subscriptions s
       JOIN plans pl ON pl.id = s.plan_id
       JOIN workspaces w ON w.id = s.workspace_id
       WHERE w.user_id = u.id AND s.status = 'active'), 0) AS post_limit,
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
  ` + planFilter + `
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
			&u.WorkspaceCount, &u.APIKeyCount, &u.PlatformCount,
			&u.Platforms,
			&u.PostsUsed, &u.PostLimit,
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
	_ = h.pool.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM users u
		 WHERE ($1 = '' OR u.email ILIKE '%' || $1 || '%' OR u.id ILIKE '%' || $1 || '%')
		   AND u.id != ALL($2)`,
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

type adminPostFailure struct {
	PostID        string    `json:"post_id"`
	UserID        string    `json:"user_id"`
	UserEmail     string    `json:"user_email"`
	WorkspaceID   string    `json:"workspace_id"`
	WorkspaceName string    `json:"workspace_name"`
	CreatedAt     time.Time `json:"created_at"`
	PostStatus    string    `json:"post_status"`
	Source        string    `json:"source"`
	Platform      *string   `json:"platform,omitempty"`
	AccountName   *string   `json:"account_name,omitempty"`
	Caption       *string   `json:"caption,omitempty"`
	ErrorMessage  *string   `json:"error_message,omitempty"`
	ErrorSummary  *string   `json:"error_summary,omitempty"`
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
	Search      string
	Status      string
	Platform    string
	Source      string
	UserID      string
	WorkspaceID string
	Days        int
	Limit       int
	Excluded    []string
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

func (h *AdminHandler) queryPostFailures(ctx context.Context, opts adminPostFailureQuery) ([]adminPostFailure, error) {
	days := opts.Days
	if days <= 0 || days > 365 {
		days = 30
	}
	limit := opts.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	rows, err := h.pool.Query(ctx, `
WITH failed_results AS (
  SELECT
    sp.id AS post_id,
    u.id AS user_id,
    u.email AS user_email,
    sp.workspace_id,
    w.name AS workspace_name,
    sp.created_at,
    sp.status AS post_status,
    sp.source,
    sa.platform,
    sa.account_name,
    NULLIF(COALESCE(spr.caption, sp.caption), '') AS caption,
    NULLIF(spr.error_message, '') AS error_message,
    NULL::TEXT AS error_summary,
    NULLIF(spr.debug_curl, '') AS debug_curl
  FROM social_posts sp
  JOIN workspaces w ON w.id = sp.workspace_id
  JOIN users u ON u.id = w.user_id
  JOIN social_post_results spr ON spr.post_id = sp.id
  LEFT JOIN social_accounts sa ON sa.id = spr.social_account_id
  WHERE ($1::TEXT = '' OR u.id = $1)
    AND u.id != ALL($7)
    AND sp.deleted_at IS NULL
    AND sp.created_at >= NOW() - ($2::INT * INTERVAL '1 day')
    AND spr.status = 'failed'
    AND ($3::TEXT = '' OR sp.source = $3)
    AND ($4::TEXT = '' OR sa.platform = $4)
),
parent_failures AS (
  SELECT
    sp.id AS post_id,
    u.id AS user_id,
    u.email AS user_email,
    sp.workspace_id,
    w.name AS workspace_name,
    sp.created_at,
    sp.status AS post_status,
    sp.source,
    NULL::TEXT AS platform,
    NULL::TEXT AS account_name,
    NULLIF(sp.caption, '') AS caption,
    NULL::TEXT AS error_message,
    NULLIF(sp.metadata->>'error_summary', '') AS error_summary,
    NULL::TEXT AS debug_curl
  FROM social_posts sp
  JOIN workspaces w ON w.id = sp.workspace_id
  JOIN users u ON u.id = w.user_id
  WHERE ($1::TEXT = '' OR u.id = $1)
    AND u.id != ALL($7)
    AND sp.deleted_at IS NULL
    AND sp.created_at >= NOW() - ($2::INT * INTERVAL '1 day')
    AND sp.status = 'failed'
    AND ($3::TEXT = '' OR sp.source = $3)
    AND ($4::TEXT = '')
    AND NOT EXISTS (
      SELECT 1
      FROM social_post_results spr
      WHERE spr.post_id = sp.id
    )
    AND COALESCE(sp.metadata->>'error_summary', '') <> ''
)
SELECT
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
  debug_curl
FROM (
  SELECT * FROM failed_results
  UNION ALL
  SELECT * FROM parent_failures
) failures
WHERE (
  $5::TEXT = ''
  OR user_email ILIKE '%' || $5 || '%'
  OR workspace_name ILIKE '%' || $5 || '%'
  OR COALESCE(account_name, '') ILIKE '%' || $5 || '%'
  OR COALESCE(caption, '') ILIKE '%' || $5 || '%'
  OR COALESCE(error_message, error_summary, '') ILIKE '%' || $5 || '%'
)
ORDER BY created_at DESC
LIMIT $6
`, opts.UserID, days, opts.Source, opts.Platform, opts.Search, limit, opts.Excluded)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]adminPostFailure, 0)
	for rows.Next() {
		var item adminPostFailure
		var platform, accountName, caption, errorMessage, errorSummary, debugCurl *string
		if err := rows.Scan(
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
		); err != nil {
			return nil, err
		}
		item.Platform = platform
		item.AccountName = accountName
		item.Caption = caption
		item.ErrorMessage = errorMessage
		item.ErrorSummary = errorSummary
		item.DebugCurl = debugCurl
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (h *AdminHandler) queryPosts(ctx context.Context, opts adminPostsQuery) ([]adminPostRow, error) {
	days := opts.Days
	if days <= 0 || days > 365 {
		days = 30
	}
	limit := opts.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}

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
    COALESCE(array_remove(array_agg(DISTINCT sa.platform), NULL), '{}') AS platforms,
    COUNT(spr.id)::BIGINT AS result_count,
    COUNT(*) FILTER (WHERE spr.status = 'published')::BIGINT AS published_result_count,
    COUNT(*) FILTER (WHERE spr.status = 'failed')::BIGINT AS failed_result_count
  FROM social_posts sp
  JOIN workspaces w ON w.id = sp.workspace_id
  JOIN users u ON u.id = w.user_id
  LEFT JOIN social_post_results spr ON spr.post_id = sp.id
  LEFT JOIN social_accounts sa ON sa.id = spr.social_account_id
  WHERE u.id != ALL($1)
    AND sp.deleted_at IS NULL
    AND sp.created_at >= NOW() - ($2::INT * INTERVAL '1 day')
    AND ($3::TEXT = '' OR sp.status = $3)
    AND ($4::TEXT = '' OR sp.source = $4)
    AND ($8::TEXT = '' OR u.id = $8)
    AND ($9::TEXT = '' OR sp.workspace_id = $9)
  GROUP BY
    sp.id, u.id, u.email, sp.workspace_id, w.name, sp.status, sp.source,
    sp.caption, sp.created_at, sp.scheduled_at, sp.published_at
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
  AND (
    $6::TEXT = ''
    OR user_email ILIKE '%' || $6 || '%'
    OR workspace_name ILIKE '%' || $6 || '%'
    OR COALESCE(caption, '') ILIKE '%' || $6 || '%'
    OR post_id ILIKE '%' || $6 || '%'
  )
ORDER BY created_at DESC
LIMIT $7
`, opts.Excluded, days, opts.Status, opts.Source, opts.Platform, opts.Search, limit, opts.UserID, opts.WorkspaceID)
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
	days := opts.Days
	if days <= 0 || days > 365 {
		days = 30
	}

	// Common WHERE used by all three queries. Parameter ordering must
	// match every Query() call below.
	commonWhere := `
  WHERE u.id != ALL($1)
    AND sp.deleted_at IS NULL
    AND sp.created_at >= NOW() - ($2::INT * INTERVAL '1 day')
    AND ($3::TEXT = '' OR sp.status = $3)
    AND ($4::TEXT = '' OR sp.source = $4)
    AND ($5::TEXT = '' OR u.id = $5)
    AND ($6::TEXT = '' OR sp.workspace_id = $6)
    AND ($7::TEXT = '' OR EXISTS (
      SELECT 1 FROM social_post_results spr2
      JOIN social_accounts sa2 ON sa2.id = spr2.social_account_id
      WHERE spr2.post_id = sp.id AND sa2.platform = $7
    ))
    AND (
      $8::TEXT = ''
      OR u.email ILIKE '%' || $8 || '%'
      OR w.name ILIKE '%' || $8 || '%'
      OR COALESCE(sp.caption, '') ILIKE '%' || $8 || '%'
      OR sp.id ILIKE '%' || $8 || '%'
    )`

	args := []any{
		opts.Excluded, days,
		opts.Status, opts.Source,
		opts.UserID, opts.WorkspaceID,
		opts.Platform, opts.Search,
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
  (SELECT COUNT(*) FROM workspaces w WHERE w.user_id = u.id),
  (SELECT COUNT(*) FROM api_keys ak JOIN workspaces w ON w.id = ak.workspace_id WHERE w.user_id = u.id AND ak.revoked_at IS NULL),
  (SELECT COUNT(*) FROM social_accounts sa JOIN profiles p ON p.id = sa.profile_id JOIN workspaces w ON w.id = p.workspace_id WHERE w.user_id = u.id AND sa.disconnected_at IS NULL),
  COALESCE((SELECT array_agg(DISTINCT sa.platform) FROM social_accounts sa JOIN profiles p ON p.id = sa.profile_id JOIN workspaces w ON w.id = p.workspace_id WHERE w.user_id = u.id AND sa.disconnected_at IS NULL), '{}'),
  COALESCE((SELECT SUM(usg.post_count)::bigint FROM usage usg JOIN workspaces w ON w.id = usg.workspace_id WHERE w.user_id = u.id AND usg.period = to_char(NOW(), 'YYYY-MM')), 0),
  COALESCE((SELECT SUM(pl.post_limit)::bigint FROM subscriptions s JOIN plans pl ON pl.id = s.plan_id JOIN workspaces w ON w.id = s.workspace_id WHERE w.user_id = u.id AND s.status='active'), 0),
  COALESCE((SELECT SUM(pl.price_cents)::bigint FROM subscriptions s JOIN plans pl ON pl.id = s.plan_id JOIN workspaces w ON w.id = s.workspace_id WHERE w.user_id = u.id AND s.status='active'), 0),
  (SELECT COUNT(*) FROM social_posts sp JOIN workspaces w ON w.id = sp.workspace_id WHERE w.user_id = u.id),
  (SELECT COUNT(*) FROM social_posts sp JOIN workspaces w ON w.id = sp.workspace_id WHERE w.user_id = u.id AND sp.status='failed' AND sp.created_at >= NOW() - INTERVAL '30 days'),
  (SELECT MAX(sp.published_at) FROM social_posts sp JOIN workspaces w ON w.id = sp.workspace_id WHERE w.user_id = u.id)
FROM users u
WHERE u.id = $1
`, userID).Scan(
		&d.ID, &d.Email, &name, &d.CreatedAt,
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

func (h *AdminHandler) ListUserPostFailures(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	q := r.URL.Query()

	days, _ := strconv.Atoi(q.Get("days"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	out, err := h.queryPostFailures(r.Context(), adminPostFailureQuery{
		UserID:   userID,
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
		Search:   q.Get("search"),
		Platform: q.Get("platform"),
		Source:   q.Get("source"),
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

	excluded, err := h.excludedUserIDs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to resolve excluded users: "+err.Error())
		return
	}

	out, err := h.queryPosts(r.Context(), adminPostsQuery{
		Search:      q.Get("search"),
		Status:      q.Get("status"),
		Platform:    q.Get("platform"),
		Source:      q.Get("source"),
		UserID:      q.Get("user_id"),
		WorkspaceID: q.Get("workspace_id"),
		Days:        days,
		Limit:       limit,
		Excluded:    excluded,
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

	excluded, err := h.excludedUserIDs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to resolve excluded users: "+err.Error())
		return
	}

	out, err := h.queryPostsAggregates(r.Context(), adminPostsQuery{
		Search:      q.Get("search"),
		Status:      q.Get("status"),
		Platform:    q.Get("platform"),
		Source:      q.Get("source"),
		UserID:      q.Get("user_id"),
		WorkspaceID: q.Get("workspace_id"),
		Days:        days,
		Excluded:    excluded,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load aggregates: "+err.Error())
		return
	}

	writeSuccess(w, out)
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
