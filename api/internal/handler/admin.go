package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AdminHandler exposes read-only aggregates for the /admin dashboard.
//
// Auth + ADMIN_USERS gating live in auth.AdminMiddleware — every route
// here assumes that middleware has already cleared the request, so the
// handler does no further checks. We talk to the DB via the raw pgxpool
// instead of sqlc because most of these queries are cross-tenant
// aggregates that don't fit cleanly into the per-project query patterns
// the rest of the API uses.
type AdminHandler struct {
	pool *pgxpool.Pool
}

func NewAdminHandler(pool *pgxpool.Pool) *AdminHandler {
	return &AdminHandler{pool: pool}
}

// ── Stats ────────────────────────────────────────────────────────────

type adminStatsResponse struct {
	TotalUsers           int64 `json:"total_users"`
	NewUsersThisMonth    int64 `json:"new_users_this_month"`
	PaidUsers            int64 `json:"paid_users"`
	MRRCents             int64 `json:"mrr_cents"`
	PostsThisMonth       int64 `json:"posts_this_month"`
	PostsFailedThisMonth int64 `json:"posts_failed_this_month"`
	ActiveProjects       int64 `json:"active_projects"`
	PlatformConnections  int64 `json:"platform_connections"`
	NewSignups7d         int64 `json:"new_signups_7d"`
	PrevSignups7d        int64 `json:"prev_signups_7d"`
	Churn30d             int64 `json:"churn_30d"`
}

const adminStatsQuery = `
SELECT
  (SELECT COUNT(*) FROM users)                                                           AS total_users,
  (SELECT COUNT(*) FROM users WHERE created_at >= date_trunc('month', NOW()))            AS new_users_this_month,
  (SELECT COUNT(DISTINCT p.owner_id)
     FROM subscriptions s
     JOIN projects p ON p.id = s.project_id
     JOIN plans pl ON pl.id = s.plan_id
     WHERE s.status = 'active' AND pl.price_cents > 0)                                   AS paid_users,
  (SELECT COALESCE(SUM(pl.price_cents), 0)
     FROM subscriptions s
     JOIN plans pl ON pl.id = s.plan_id
     WHERE s.status = 'active')                                                          AS mrr_cents,
  (SELECT COUNT(*) FROM social_posts WHERE created_at >= date_trunc('month', NOW()))     AS posts_this_month,
  (SELECT COUNT(*) FROM social_posts
     WHERE created_at >= date_trunc('month', NOW()) AND status = 'failed')               AS posts_failed_this_month,
  (SELECT COUNT(*) FROM projects)                                                        AS active_projects,
  (SELECT COUNT(*) FROM social_accounts WHERE disconnected_at IS NULL)                   AS platform_connections,
  (SELECT COUNT(*) FROM users WHERE created_at >= NOW() - INTERVAL '7 days')             AS new_signups_7d,
  (SELECT COUNT(*) FROM users
     WHERE created_at >= NOW() - INTERVAL '14 days'
       AND created_at <  NOW() - INTERVAL '7 days')                                      AS prev_signups_7d,
  (SELECT COUNT(*) FROM subscriptions
     WHERE status IN ('canceled', 'past_due')
       AND updated_at >= NOW() - INTERVAL '30 days')                                     AS churn_30d
`

func (h *AdminHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	var s adminStatsResponse
	err := h.pool.QueryRow(r.Context(), adminStatsQuery).Scan(
		&s.TotalUsers,
		&s.NewUsersThisMonth,
		&s.PaidUsers,
		&s.MRRCents,
		&s.PostsThisMonth,
		&s.PostsFailedThisMonth,
		&s.ActiveProjects,
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
	ProjectCount    int64      `json:"project_count"`
	APIKeyCount     int64      `json:"api_key_count"`
	PlatformCount   int64      `json:"platform_count"`
	Platforms       []string   `json:"platforms"`
	PostsUsed       int64      `json:"posts_used"`
	PostLimit       int64      `json:"post_limit"`
	MRRCents        int64      `json:"mrr_cents"`
	IsPaid          bool       `json:"is_paid"`
	LastPostAt      *time.Time `json:"last_post_at"`
}

// Sort enums — whitelisted before being interpolated into ORDER BY.
// These run against the outer `SELECT * FROM base`, where columns are
// unqualified (the CTE projects u.created_at as plain created_at), so
// no `u.` prefix here.
var adminUserSortOrders = map[string]string{
	"newest":      "created_at DESC",
	"mrr":         "mrr_cents DESC, created_at DESC",
	"usage":       "posts_used DESC, created_at DESC",
	"last_active": "last_post_at DESC NULLS LAST, created_at DESC",
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	search := q.Get("search")
	plan := q.Get("plan") // all | free | paid
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
			JOIN projects p ON p.id = s.project_id
			WHERE p.owner_id = u.id AND s.status='active' AND pl.price_cents > 0
		)`
	case "free":
		planFilter = `AND NOT EXISTS(
			SELECT 1 FROM subscriptions s
			JOIN plans pl ON pl.id = s.plan_id
			JOIN projects p ON p.id = s.project_id
			WHERE p.owner_id = u.id AND s.status='active' AND pl.price_cents > 0
		)`
	}

	sql := `
WITH base AS (
  SELECT
    u.id, u.email, u.created_at,
    (SELECT COUNT(*) FROM projects p WHERE p.owner_id = u.id) AS project_count,
    (SELECT COUNT(*)
       FROM api_keys ak
       JOIN projects p ON p.id = ak.project_id
       WHERE p.owner_id = u.id AND ak.revoked_at IS NULL) AS api_key_count,
    (SELECT COUNT(*)
       FROM social_accounts sa
       JOIN projects p ON p.id = sa.project_id
       WHERE p.owner_id = u.id AND sa.disconnected_at IS NULL) AS platform_count,
    COALESCE((SELECT array_agg(DISTINCT sa.platform)
       FROM social_accounts sa
       JOIN projects p ON p.id = sa.project_id
       WHERE p.owner_id = u.id AND sa.disconnected_at IS NULL), '{}') AS platforms,
    COALESCE((SELECT SUM(usg.post_count)::bigint
       FROM usage usg
       JOIN projects p ON p.id = usg.project_id
       WHERE p.owner_id = u.id AND usg.period = to_char(NOW(), 'YYYY-MM')), 0) AS posts_used,
    COALESCE((SELECT SUM(pl.post_limit)::bigint
       FROM subscriptions s
       JOIN plans pl ON pl.id = s.plan_id
       JOIN projects p ON p.id = s.project_id
       WHERE p.owner_id = u.id AND s.status = 'active'), 0) AS post_limit,
    COALESCE((SELECT SUM(pl.price_cents)::bigint
       FROM subscriptions s
       JOIN plans pl ON pl.id = s.plan_id
       JOIN projects p ON p.id = s.project_id
       WHERE p.owner_id = u.id AND s.status = 'active'), 0) AS mrr_cents,
    EXISTS(SELECT 1
       FROM subscriptions s
       JOIN plans pl ON pl.id = s.plan_id
       JOIN projects p ON p.id = s.project_id
       WHERE p.owner_id = u.id AND s.status = 'active' AND pl.price_cents > 0) AS is_paid,
    (SELECT MAX(sp.published_at)
       FROM social_posts sp
       JOIN projects p ON p.id = sp.project_id
       WHERE p.owner_id = u.id) AS last_post_at
  FROM users u
  WHERE ($1 = '' OR u.email ILIKE '%' || $1 || '%' OR u.id ILIKE '%' || $1 || '%')
  ` + planFilter + `
)
SELECT * FROM base ORDER BY ` + orderBy + ` LIMIT $2 OFFSET $3`

	rows, err := h.pool.Query(r.Context(), sql, search, limit, offset)
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
			&u.ProjectCount, &u.APIKeyCount, &u.PlatformCount,
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

	// Total (without limit/offset) for pagination — separate cheap query.
	var total int64
	_ = h.pool.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM users u WHERE ($1 = '' OR u.email ILIKE '%' || $1 || '%' OR u.id ILIKE '%' || $1 || '%')`,
		search,
	).Scan(&total)

	writeSuccessWithMeta(w, out, int(total))
}

// ── User detail ──────────────────────────────────────────────────────

type adminUserProject struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Mode          string    `json:"mode"`
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
	ID                string             `json:"id"`
	Email             string             `json:"email"`
	Name              string             `json:"name"`
	CreatedAt         time.Time          `json:"created_at"`
	ProjectCount      int64              `json:"project_count"`
	APIKeyCount       int64              `json:"api_key_count"`
	PlatformCount     int64              `json:"platform_count"`
	Platforms         []string           `json:"platforms"`
	PostsUsedThisMonth int64             `json:"posts_used_this_month"`
	PostLimit         int64              `json:"post_limit"`
	MRRCents          int64              `json:"mrr_cents"`
	TotalPosts        int64              `json:"total_posts"`
	FailedPosts30d    int64              `json:"failed_posts_30d"`
	LastPostAt        *time.Time         `json:"last_post_at"`
	Projects          []adminUserProject `json:"projects"`
}

func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")

	var d adminUserDetailResponse
	var name *string
	var lastPostAt *time.Time
	err := h.pool.QueryRow(r.Context(), `
SELECT
  u.id, u.email, u.name, u.created_at,
  (SELECT COUNT(*) FROM projects p WHERE p.owner_id = u.id),
  (SELECT COUNT(*) FROM api_keys ak JOIN projects p ON p.id = ak.project_id WHERE p.owner_id = u.id AND ak.revoked_at IS NULL),
  (SELECT COUNT(*) FROM social_accounts sa JOIN projects p ON p.id = sa.project_id WHERE p.owner_id = u.id AND sa.disconnected_at IS NULL),
  COALESCE((SELECT array_agg(DISTINCT sa.platform) FROM social_accounts sa JOIN projects p ON p.id = sa.project_id WHERE p.owner_id = u.id AND sa.disconnected_at IS NULL), '{}'),
  COALESCE((SELECT SUM(usg.post_count)::bigint FROM usage usg JOIN projects p ON p.id = usg.project_id WHERE p.owner_id = u.id AND usg.period = to_char(NOW(), 'YYYY-MM')), 0),
  COALESCE((SELECT SUM(pl.post_limit)::bigint FROM subscriptions s JOIN plans pl ON pl.id = s.plan_id JOIN projects p ON p.id = s.project_id WHERE p.owner_id = u.id AND s.status='active'), 0),
  COALESCE((SELECT SUM(pl.price_cents)::bigint FROM subscriptions s JOIN plans pl ON pl.id = s.plan_id JOIN projects p ON p.id = s.project_id WHERE p.owner_id = u.id AND s.status='active'), 0),
  (SELECT COUNT(*) FROM social_posts sp JOIN projects p ON p.id = sp.project_id WHERE p.owner_id = u.id),
  (SELECT COUNT(*) FROM social_posts sp JOIN projects p ON p.id = sp.project_id WHERE p.owner_id = u.id AND sp.status='failed' AND sp.created_at >= NOW() - INTERVAL '30 days'),
  (SELECT MAX(sp.published_at) FROM social_posts sp JOIN projects p ON p.id = sp.project_id WHERE p.owner_id = u.id)
FROM users u
WHERE u.id = $1
`, userID).Scan(
		&d.ID, &d.Email, &name, &d.CreatedAt,
		&d.ProjectCount, &d.APIKeyCount, &d.PlatformCount,
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

	// Per-project breakdown
	rows, err := h.pool.Query(r.Context(), `
SELECT
  p.id, p.name, p.mode, p.created_at,
  COALESCE(s.plan_id, 'free'),
  COALESCE(pl.name, 'Free'),
  COALESCE(pl.price_cents, 0),
  COALESCE((SELECT post_count FROM usage WHERE project_id = p.id AND period = to_char(NOW(),'YYYY-MM')), 0),
  COALESCE(pl.post_limit, 100),
  COALESCE(s.status, 'active'),
  (SELECT COUNT(*) FROM social_accounts sa WHERE sa.project_id = p.id AND sa.disconnected_at IS NULL)
FROM projects p
LEFT JOIN subscriptions s ON s.project_id = p.id
LEFT JOIN plans pl ON pl.id = s.plan_id
WHERE p.owner_id = $1
ORDER BY p.created_at DESC
`, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load projects: "+err.Error())
		return
	}
	defer rows.Close()

	d.Projects = make([]adminUserProject, 0)
	for rows.Next() {
		var p adminUserProject
		var posts int64
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Mode, &p.CreatedAt,
			&p.PlanID, &p.PlanName, &p.PriceCents,
			&posts, &p.PostLimit, &p.Status, &p.PlatformCount,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to scan project: "+err.Error())
			return
		}
		p.PostsUsed = posts
		d.Projects = append(d.Projects, p)
	}

	writeSuccess(w, d)
}
