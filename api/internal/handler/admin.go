package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/billing"
)

// AdminHandler exposes read-only aggregates for the /admin dashboard.
// Auth + ADMIN_USERS gating live in auth.AdminMiddleware. We talk to
// the DB via the raw pgxpool because most of these queries are
// cross-tenant aggregates that don't fit into per-workspace sqlc patterns.
type AdminHandler struct {
	pool      *pgxpool.Pool
	stripeMgr *billing.Manager
}

func NewAdminHandler(pool *pgxpool.Pool, stripeMgr *billing.Manager) *AdminHandler {
	return &AdminHandler{pool: pool, stripeMgr: stripeMgr}
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

// ── User list ────────────────────────────────────────────────────────

type adminUserRow struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	CreatedAt       time.Time  `json:"created_at"`
	WorkspaceCount  int64      `json:"workspace_count"`
	APIKeyCount     int64      `json:"api_key_count"`
	PlatformCount   int64      `json:"platform_count"`
	Platforms       []string   `json:"platforms"`
	PostsUsed       int64      `json:"posts_used"`
	PostLimit       int64      `json:"post_limit"`
	MRRCents        int64      `json:"mrr_cents"`
	IsPaid          bool       `json:"is_paid"`
	LastPostAt      *time.Time `json:"last_post_at"`
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

	writeSuccessWithMeta(w, out, int(total))
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

type adminUserPostFailure struct {
	PostID       string     `json:"post_id"`
	WorkspaceID  string     `json:"workspace_id"`
	WorkspaceName string    `json:"workspace_name"`
	CreatedAt    time.Time  `json:"created_at"`
	PostStatus   string     `json:"post_status"`
	Source       string     `json:"source"`
	Platform     *string    `json:"platform,omitempty"`
	AccountName  *string    `json:"account_name,omitempty"`
	Caption      *string    `json:"caption,omitempty"`
	ErrorMessage *string    `json:"error_message,omitempty"`
	ErrorSummary *string    `json:"error_summary,omitempty"`
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
	if days <= 0 || days > 365 {
		days = 30
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	rows, err := h.pool.Query(r.Context(), `
WITH failed_results AS (
  SELECT
    sp.id AS post_id,
    sp.workspace_id,
    w.name AS workspace_name,
    sp.created_at,
    sp.status AS post_status,
    sp.source,
    sa.platform,
    sa.account_name,
    NULLIF(COALESCE(spr.caption, sp.caption), '') AS caption,
    NULLIF(spr.error_message, '') AS error_message,
    NULL::TEXT AS error_summary
  FROM social_posts sp
  JOIN workspaces w ON w.id = sp.workspace_id
  JOIN social_post_results spr ON spr.post_id = sp.id
  LEFT JOIN social_accounts sa ON sa.id = spr.social_account_id
  WHERE w.user_id = $1
    AND sp.deleted_at IS NULL
    AND sp.created_at >= NOW() - ($2::INT * INTERVAL '1 day')
    AND spr.status = 'failed'
),
parent_failures AS (
  SELECT
    sp.id AS post_id,
    sp.workspace_id,
    w.name AS workspace_name,
    sp.created_at,
    sp.status AS post_status,
    sp.source,
    NULL::TEXT AS platform,
    NULL::TEXT AS account_name,
    NULLIF(sp.caption, '') AS caption,
    NULL::TEXT AS error_message,
    NULLIF(sp.metadata->>'error_summary', '') AS error_summary
  FROM social_posts sp
  JOIN workspaces w ON w.id = sp.workspace_id
  WHERE w.user_id = $1
    AND sp.deleted_at IS NULL
    AND sp.created_at >= NOW() - ($2::INT * INTERVAL '1 day')
    AND sp.status = 'failed'
    AND NOT EXISTS (
      SELECT 1
      FROM social_post_results spr
      WHERE spr.post_id = sp.id
    )
    AND COALESCE(sp.metadata->>'error_summary', '') <> ''
)
SELECT
  post_id,
  workspace_id,
  workspace_name,
  created_at,
  post_status,
  source,
  platform,
  account_name,
  caption,
  error_message,
  error_summary
FROM (
  SELECT * FROM failed_results
  UNION ALL
  SELECT * FROM parent_failures
) failures
ORDER BY created_at DESC
LIMIT $3
`, userID, days, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load post failures: "+err.Error())
		return
	}
	defer rows.Close()

	out := make([]adminUserPostFailure, 0)
	for rows.Next() {
		var item adminUserPostFailure
		var platform, accountName, caption, errorMessage, errorSummary *string
		if err := rows.Scan(
			&item.PostID,
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
		); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to scan post failure: "+err.Error())
			return
		}
		item.Platform = platform
		item.AccountName = accountName
		item.Caption = caption
		item.ErrorMessage = errorMessage
		item.ErrorSummary = errorSummary
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to read post failures: "+err.Error())
		return
	}

	writeSuccess(w, out)
}
