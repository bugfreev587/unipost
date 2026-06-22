-- name: ListFreePlanQuotaReminderAttemptedThresholds :many
SELECT threshold_percent
FROM free_plan_quota_email_reminders
WHERE workspace_id = $1
  AND period = $2
  AND status IN ('pending', 'sent')
ORDER BY threshold_percent ASC;

-- name: CreatePendingFreePlanQuotaReminder :one
INSERT INTO free_plan_quota_email_reminders (
  workspace_id,
  user_id,
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
  sent_at
)
VALUES ($1, $2, $3, $4, $5, 'pending', $6, $7, $8, $9, $10, $11, NULL, NOW(), NULL)
ON CONFLICT (workspace_id, period, threshold_percent)
DO UPDATE SET
  user_id = EXCLUDED.user_id,
  email = EXCLUDED.email,
  status = 'pending',
  transactional_id = EXCLUDED.transactional_id,
  idempotency_key = EXCLUDED.idempotency_key,
  effective_usage = EXCLUDED.effective_usage,
  completed_usage = EXCLUDED.completed_usage,
  reserved_usage = EXCLUDED.reserved_usage,
  post_limit = EXCLUDED.post_limit,
  failure_reason = NULL,
  attempted_at = NOW(),
  sent_at = NULL,
  updated_at = NOW()
WHERE free_plan_quota_email_reminders.status = 'failed'
RETURNING *;

-- name: MarkFreePlanQuotaReminderSent :exec
UPDATE free_plan_quota_email_reminders
SET status = 'sent',
    sent_at = NOW(),
    failure_reason = NULL,
    updated_at = NOW()
WHERE id = $1;

-- name: MarkFreePlanQuotaReminderFailed :exec
UPDATE free_plan_quota_email_reminders
SET status = 'failed',
    failure_reason = $2,
    updated_at = NOW()
WHERE id = $1;
