-- name: InsertPaidQuotaFollowUp :one
INSERT INTO paid_quota_follow_ups (
  workspace_id,
  owner_user_id,
  plan_id,
  period,
  threshold_percent,
  notification_id,
  completed_usage,
  scheduled_usage,
  quota_hold_usage,
  effective_usage,
  post_limit
)
VALUES (
  sqlc.arg(workspace_id),
  sqlc.narg(owner_user_id),
  sqlc.arg(plan_id),
  sqlc.arg(period),
  120,
  sqlc.narg(notification_id),
  sqlc.arg(completed_usage),
  sqlc.arg(scheduled_usage),
  sqlc.arg(quota_hold_usage),
  sqlc.arg(effective_usage),
  sqlc.arg(post_limit)
)
ON CONFLICT (workspace_id, period, threshold_percent)
DO UPDATE SET
  owner_user_id = EXCLUDED.owner_user_id,
  plan_id = EXCLUDED.plan_id,
  notification_id = COALESCE(paid_quota_follow_ups.notification_id, EXCLUDED.notification_id),
  completed_usage = EXCLUDED.completed_usage,
  scheduled_usage = EXCLUDED.scheduled_usage,
  quota_hold_usage = EXCLUDED.quota_hold_usage,
  effective_usage = EXCLUDED.effective_usage,
  post_limit = EXCLUDED.post_limit,
  updated_at = NOW()
RETURNING *;

-- name: ListPaidQuotaFollowUps :many
SELECT *
FROM paid_quota_follow_ups
WHERE (sqlc.narg(status)::TEXT IS NULL OR status = sqlc.narg(status))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit)
OFFSET sqlc.arg(row_offset);

-- name: UpdatePaidQuotaFollowUp :one
UPDATE paid_quota_follow_ups
SET status = sqlc.arg(status),
    assignee_user_id = sqlc.narg(assignee_user_id),
    notes = sqlc.narg(notes),
    resolved_at = CASE
      WHEN sqlc.arg(status) IN ('resolved', 'dismissed') THEN COALESCE(resolved_at, NOW())
      ELSE NULL
    END,
    updated_at = NOW()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ResolvePaidQuotaFollowUpsBelowLimit :exec
UPDATE paid_quota_follow_ups
SET status = 'resolved',
    resolved_at = COALESCE(resolved_at, NOW()),
    updated_at = NOW()
WHERE workspace_id = $1
  AND period = $2
  AND status = 'open';
