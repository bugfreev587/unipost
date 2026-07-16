package db

import "context"

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
    sp.status IN ('scheduled', 'quota_hold')
    OR (sp.status = 'publishing' AND sp.scheduled_at IS NOT NULL)
  )
  AND sp.deleted_at IS NULL
  AND sp.scheduled_at >= ($2 || '-01')::DATE
  AND sp.scheduled_at < (($2 || '-01')::DATE + INTERVAL '1 month')
  AND sp.created_at > COALESCE(rb.reset_at, '-infinity'::timestamptz)
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
