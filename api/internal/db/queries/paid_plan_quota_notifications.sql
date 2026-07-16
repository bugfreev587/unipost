-- name: ListPaidPlanQuotaNotificationDecisions :many
SELECT threshold_percent, status
FROM paid_plan_quota_notifications
WHERE workspace_id = $1
  AND period = $2
ORDER BY threshold_percent;

-- name: InsertPaidPlanQuotaNotificationDecision :one
INSERT INTO paid_plan_quota_notifications (
  workspace_id,
  user_id,
  email,
  plan_id,
  period,
  threshold_percent,
  severity,
  event_key,
  status,
  transactional_id,
  idempotency_key,
  completed_usage,
  scheduled_usage,
  quota_hold_usage,
  effective_usage,
  post_limit
)
VALUES (
  sqlc.arg(workspace_id),
  sqlc.narg(user_id),
  sqlc.narg(email),
  sqlc.arg(plan_id),
  sqlc.arg(period),
  sqlc.arg(threshold_percent),
  sqlc.arg(severity),
  sqlc.arg(event_key),
  sqlc.arg(status),
  sqlc.narg(transactional_id),
  sqlc.arg(idempotency_key),
  sqlc.arg(completed_usage),
  sqlc.arg(scheduled_usage),
  sqlc.arg(quota_hold_usage),
  sqlc.arg(effective_usage),
  sqlc.arg(post_limit)
)
ON CONFLICT (workspace_id, period, threshold_percent) DO NOTHING
RETURNING *;

-- name: ClaimPaidPlanQuotaNotifications :many
WITH candidates AS (
  SELECT id
  FROM paid_plan_quota_notifications
  WHERE (
      status IN ('pending', 'retry_wait')
      OR (status = 'processing' AND lease_expires_at < NOW())
    )
    AND next_attempt_at <= NOW()
  ORDER BY next_attempt_at, created_at, id
  FOR UPDATE SKIP LOCKED
  LIMIT $1
)
UPDATE paid_plan_quota_notifications n
SET status = 'processing',
    attempt_count = attempt_count + 1,
    attempted_at = NOW(),
    lease_expires_at = NOW() + INTERVAL '5 minutes',
    updated_at = NOW()
FROM candidates
WHERE n.id = candidates.id
RETURNING n.*;

-- name: MarkPaidPlanQuotaNotificationSent :exec
UPDATE paid_plan_quota_notifications
SET status = 'sent',
    sent_at = NOW(),
    lease_expires_at = NULL,
    last_error = NULL,
    updated_at = NOW()
WHERE id = $1;

-- name: MarkPaidPlanQuotaNotificationRetryWait :exec
UPDATE paid_plan_quota_notifications
SET status = 'retry_wait',
    next_attempt_at = sqlc.arg(next_attempt_at),
    lease_expires_at = NULL,
    last_error = sqlc.narg(last_error),
    updated_at = NOW()
WHERE id = sqlc.arg(id);

-- name: MarkPaidPlanQuotaNotificationFailed :exec
UPDATE paid_plan_quota_notifications
SET status = 'failed',
    lease_expires_at = NULL,
    last_error = sqlc.narg(last_error),
    updated_at = NOW()
WHERE id = sqlc.arg(id);

-- name: MarkPaidPlanQuotaNotificationPreferenceDisabled :exec
UPDATE paid_plan_quota_notifications
SET status = 'skipped_preference_disabled',
    lease_expires_at = NULL,
    last_error = 'preference_disabled',
    updated_at = NOW()
WHERE id = $1;

-- name: MarkLowerPaidPlanQuotaNotificationsSuperseded :exec
UPDATE paid_plan_quota_notifications
SET status = 'skipped_superseded',
    lease_expires_at = NULL,
    last_error = NULL,
    updated_at = NOW()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND period = sqlc.arg(period)
  AND threshold_percent < sqlc.arg(threshold_percent)
  AND status IN ('pending', 'retry_wait');

-- name: RetryFailedPaidPlanQuotaNotification :one
UPDATE paid_plan_quota_notifications
SET status = 'pending',
    attempt_count = 0,
    next_attempt_at = NOW(),
    lease_expires_at = NULL,
    last_error = NULL,
    updated_at = NOW()
WHERE id = $1
  AND status = 'failed'
RETURNING *;

-- name: ListPaidQuotaReconciliationWorkspaces :many
SELECT s.workspace_id
FROM subscriptions s
JOIN plans p ON p.id = s.plan_id
WHERE s.plan_id IN ('api', 'basic', 'growth')
  AND p.post_limit > 0
ORDER BY s.workspace_id;
