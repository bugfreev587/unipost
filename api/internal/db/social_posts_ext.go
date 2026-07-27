package db

import "context"

type MonthlyQuotaSnapshotForPeriodParams struct {
	WorkspaceID string `json:"workspace_id"`
	Period      string `json:"period"`
}

type MonthlyQuotaSnapshotForPeriodRow struct {
	PlanID    string `json:"plan_id"`
	PostLimit int32  `json:"post_limit"`
	Completed int32  `json:"completed"`
	Scheduled int32  `json:"scheduled"`
	QuotaHold int32  `json:"quota_hold"`
}

// monthlyQuotaSnapshotForPeriod intentionally returns the entire admission
// snapshot from one PostgreSQL statement. Under READ COMMITTED every command
// receives a new snapshot, so splitting usage and reservation reads can observe
// opposite sides of a terminal reservation-to-usage conversion.
const monthlyQuotaSnapshotForPeriod = `
/* monthly_quota_snapshot */
WITH selected_plan AS (
  SELECT COALESCE(NULLIF((
    SELECT plan_id
    FROM subscriptions
    WHERE workspace_id = $1
  ), ''), 'free') AS plan_id
), reset_baseline AS (
  SELECT MAX(reset_at) AS reset_at
  FROM admin_post_quota_resets
  WHERE workspace_id = $1
    AND period = $2
    AND quota_kind = 'scheduled'
), reservation_rows AS (
  SELECT sp.status,
    CASE
      WHEN sp.status = 'publishing' AND EXISTS (
        SELECT 1 FROM social_post_results existing WHERE existing.post_id = sp.id
      ) THEN (
        SELECT COUNT(*)::INTEGER
        FROM social_post_results spr
        JOIN social_accounts sa ON sa.id = spr.social_account_id
        WHERE spr.post_id = sp.id
          AND spr.status NOT IN ('published', 'failed')
          AND sa.disconnected_at IS NULL
      )
      WHEN jsonb_typeof(sp.metadata->'scheduled_quota_units') = 'number'
        AND (sp.metadata->>'scheduled_quota_units') ~ '^(0|[1-9][0-9]{0,8})$'
        AND jsonb_typeof(sp.metadata->'platform_posts') = 'array'
        AND (sp.metadata->>'scheduled_quota_units')::NUMERIC <= jsonb_array_length(sp.metadata->'platform_posts') THEN
        (sp.metadata->>'scheduled_quota_units')::INTEGER
      WHEN jsonb_typeof(sp.metadata->'platform_posts') = 'array' THEN (
        SELECT COUNT(*)::INTEGER
        FROM jsonb_array_elements(sp.metadata->'platform_posts') AS pp
        JOIN social_accounts sa ON sa.id = pp->>'account_id'
        WHERE sa.disconnected_at IS NULL
      )
      WHEN jsonb_typeof(sp.metadata->'account_ids') = 'array' THEN (
        SELECT COUNT(*)::INTEGER
        FROM jsonb_array_elements_text(sp.metadata->'account_ids') AS account_id
        JOIN social_accounts sa ON sa.id = account_id
        WHERE sa.disconnected_at IS NULL
      )
      ELSE 1
    END AS units
  FROM social_posts sp
  CROSS JOIN reset_baseline rb
  WHERE sp.workspace_id = $1
    AND sp.deleted_at IS NULL
    AND (
      (
        sp.status IN ('scheduled', 'quota_hold')
        AND sp.scheduled_at >= ($2 || '-01')::DATE
        AND sp.scheduled_at < (($2 || '-01')::DATE + INTERVAL '1 month')
        AND sp.created_at > COALESCE(rb.reset_at, '-infinity'::timestamptz)
      )
      OR (
        sp.status = 'publishing'
        AND (
          (
            jsonb_typeof(sp.metadata->'scheduled_execution_reservation_period') = 'string'
            AND (sp.metadata->>'scheduled_execution_reservation_period') ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'
            AND sp.metadata->>'scheduled_execution_reservation_period' = $2
          )
          OR (
            NOT COALESCE((
              jsonb_typeof(sp.metadata->'scheduled_execution_reservation_period') = 'string'
              AND (sp.metadata->>'scheduled_execution_reservation_period') ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'
            ), FALSE)
            AND sp.scheduled_at >= ($2 || '-01')::DATE
            AND sp.scheduled_at < (($2 || '-01')::DATE + INTERVAL '1 month')
            AND sp.created_at > COALESCE(rb.reset_at, '-infinity'::timestamptz)
          )
        )
      )
    )
), reservation_snapshot AS (
  SELECT
    COALESCE(SUM(units), 0)::INTEGER AS scheduled,
    COALESCE(SUM(units) FILTER (WHERE status = 'quota_hold'), 0)::INTEGER AS quota_hold
  FROM reservation_rows
)
SELECT selected_plan.plan_id,
       plans.post_limit,
       COALESCE(usage.post_count, 0)::INTEGER AS completed,
       reservation_snapshot.scheduled,
       reservation_snapshot.quota_hold
FROM selected_plan
JOIN plans ON plans.id = selected_plan.plan_id
CROSS JOIN reservation_snapshot
LEFT JOIN usage ON usage.workspace_id = $1 AND usage.period = $2
`

func (q *Queries) MonthlyQuotaSnapshotForPeriod(ctx context.Context, arg MonthlyQuotaSnapshotForPeriodParams) (MonthlyQuotaSnapshotForPeriodRow, error) {
	row := q.db.QueryRow(ctx, monthlyQuotaSnapshotForPeriod, arg.WorkspaceID, arg.Period)
	var snapshot MonthlyQuotaSnapshotForPeriodRow
	err := row.Scan(&snapshot.PlanID, &snapshot.PostLimit, &snapshot.Completed, &snapshot.Scheduled, &snapshot.QuotaHold)
	return snapshot, err
}

func (q *Queries) LockScheduledQuotaPeriod(ctx context.Context, workspaceID, period string) error {
	_, err := q.db.Exec(ctx,
		"SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))",
		workspaceID,
		"paid_schedule_quota:"+period,
	)
	return err
}

const setScheduledExecutionReservationPeriod = `
UPDATE social_posts
SET metadata = jsonb_set(
  COALESCE(metadata, '{}'::jsonb),
  '{scheduled_execution_reservation_period}',
  to_jsonb($2::text),
  true
)
WHERE id = $1
  AND status = 'publishing'
RETURNING id
`

// SetScheduledExecutionReservationPeriod persists the server-selected month
// after the execution quota gate and before delivery rows become visible.
// Request metadata cannot choose this value: every claim overwrites it inside
// the same transaction that creates the publishing work.
func (q *Queries) SetScheduledExecutionReservationPeriod(ctx context.Context, postID, period string) error {
	var id string
	return q.db.QueryRow(ctx, setScheduledExecutionReservationPeriod, postID, period).Scan(&id)
}

type CountScheduledQuotaUnitsByWorkspaceAndPeriodParams struct {
	WorkspaceID string `json:"workspace_id"`
	Period      string `json:"period"`
}

const countScheduledQuotaUnitsByWorkspaceAndPeriod = `
WITH reset_baseline AS (
  SELECT MAX(reset_at) AS reset_at
  FROM admin_post_quota_resets
  WHERE workspace_id = $1
    AND period = $2
    AND quota_kind = 'scheduled'
)
SELECT COALESCE(SUM(
  CASE
    WHEN sp.status = 'publishing' AND EXISTS (
      SELECT 1 FROM social_post_results existing WHERE existing.post_id = sp.id
    ) THEN (
      SELECT COUNT(*)::INTEGER
      FROM social_post_results spr
      JOIN social_accounts sa ON sa.id = spr.social_account_id
      WHERE spr.post_id = sp.id
        AND spr.status NOT IN ('published', 'failed')
        AND sa.disconnected_at IS NULL
    )
    WHEN jsonb_typeof(sp.metadata->'scheduled_quota_units') = 'number'
      AND (sp.metadata->>'scheduled_quota_units') ~ '^(0|[1-9][0-9]{0,8})$'
      AND jsonb_typeof(sp.metadata->'platform_posts') = 'array'
      AND (sp.metadata->>'scheduled_quota_units')::NUMERIC <= jsonb_array_length(sp.metadata->'platform_posts') THEN
      (sp.metadata->>'scheduled_quota_units')::INTEGER
    WHEN jsonb_typeof(sp.metadata->'platform_posts') = 'array' THEN (
      SELECT COUNT(*)::INTEGER
      FROM jsonb_array_elements(sp.metadata->'platform_posts') AS pp
      JOIN social_accounts sa ON sa.id = pp->>'account_id'
      WHERE sa.disconnected_at IS NULL
    )
    WHEN jsonb_typeof(sp.metadata->'account_ids') = 'array' THEN (
      SELECT COUNT(*)::INTEGER
      FROM jsonb_array_elements_text(sp.metadata->'account_ids') AS account_id
      JOIN social_accounts sa ON sa.id = account_id
      WHERE sa.disconnected_at IS NULL
    )
    ELSE 1
  END
), 0)::INTEGER
FROM social_posts sp
CROSS JOIN reset_baseline rb
WHERE sp.workspace_id = $1
  AND (
    (
      sp.status IN ('scheduled', 'quota_hold')
      AND sp.scheduled_at >= ($2 || '-01')::DATE
      AND sp.scheduled_at < (($2 || '-01')::DATE + INTERVAL '1 month')
      AND sp.created_at > COALESCE(rb.reset_at, '-infinity'::timestamptz)
    )
    OR (
      sp.status = 'publishing'
      AND (
        (
          jsonb_typeof(sp.metadata->'scheduled_execution_reservation_period') = 'string'
          AND (sp.metadata->>'scheduled_execution_reservation_period') ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'
          AND sp.metadata->>'scheduled_execution_reservation_period' = $2
        )
        OR (
          NOT COALESCE((
            jsonb_typeof(sp.metadata->'scheduled_execution_reservation_period') = 'string'
            AND (sp.metadata->>'scheduled_execution_reservation_period') ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'
          ), FALSE)
          AND sp.scheduled_at >= ($2 || '-01')::DATE
          AND sp.scheduled_at < (($2 || '-01')::DATE + INTERVAL '1 month')
          AND sp.created_at > COALESCE(rb.reset_at, '-infinity'::timestamptz)
        )
      )
    )
  )
  AND sp.deleted_at IS NULL
`

func (q *Queries) CountScheduledQuotaUnitsByWorkspaceAndPeriod(ctx context.Context, arg CountScheduledQuotaUnitsByWorkspaceAndPeriodParams) (int32, error) {
	row := q.db.QueryRow(ctx, countScheduledQuotaUnitsByWorkspaceAndPeriod, arg.WorkspaceID, arg.Period)
	var count int32
	err := row.Scan(&count)
	return count, err
}

type CountQuotaHoldUnitsByWorkspaceAndPeriodParams struct {
	WorkspaceID string `json:"workspace_id"`
	Period      string `json:"period"`
}

const countQuotaHoldUnitsByWorkspaceAndPeriod = `
WITH reset_baseline AS (
  SELECT MAX(reset_at) AS reset_at
  FROM admin_post_quota_resets
  WHERE workspace_id = $1
    AND period = $2
    AND quota_kind = 'scheduled'
)
SELECT COALESCE(SUM(
  CASE
    WHEN jsonb_typeof(sp.metadata->'scheduled_quota_units') = 'number'
      AND (sp.metadata->>'scheduled_quota_units') ~ '^(0|[1-9][0-9]{0,8})$'
      AND jsonb_typeof(sp.metadata->'platform_posts') = 'array'
      AND (sp.metadata->>'scheduled_quota_units')::NUMERIC <= jsonb_array_length(sp.metadata->'platform_posts') THEN
      (sp.metadata->>'scheduled_quota_units')::INTEGER
    WHEN jsonb_typeof(sp.metadata->'platform_posts') = 'array' THEN (
      SELECT COUNT(*)::INTEGER
      FROM jsonb_array_elements(sp.metadata->'platform_posts') AS pp
      JOIN social_accounts sa ON sa.id = pp->>'account_id'
      WHERE sa.disconnected_at IS NULL
    )
    WHEN jsonb_typeof(sp.metadata->'account_ids') = 'array' THEN (
      SELECT COUNT(*)::INTEGER
      FROM jsonb_array_elements_text(sp.metadata->'account_ids') AS account_id
      JOIN social_accounts sa ON sa.id = account_id
      WHERE sa.disconnected_at IS NULL
    )
    ELSE 1
  END
), 0)::INTEGER
FROM social_posts sp
CROSS JOIN reset_baseline rb
WHERE sp.workspace_id = $1
  AND sp.status = 'quota_hold'
  AND sp.deleted_at IS NULL
  AND sp.scheduled_at >= ($2 || '-01')::DATE
  AND sp.scheduled_at < (($2 || '-01')::DATE + INTERVAL '1 month')
  AND sp.created_at > COALESCE(rb.reset_at, '-infinity'::timestamptz)
`

func (q *Queries) CountQuotaHoldUnitsByWorkspaceAndPeriod(ctx context.Context, arg CountQuotaHoldUnitsByWorkspaceAndPeriodParams) (int32, error) {
	row := q.db.QueryRow(ctx, countQuotaHoldUnitsByWorkspaceAndPeriod, arg.WorkspaceID, arg.Period)
	var count int32
	err := row.Scan(&count)
	return count, err
}
