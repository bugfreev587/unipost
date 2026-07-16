-- name: GetXUsagePeriod :one
SELECT *
FROM x_usage_periods
WHERE workspace_id = $1
  AND period_start = $2
  AND period_end = $3;

-- name: GetXInboundDailyUsage :one
SELECT *
FROM x_inbound_daily_usage
WHERE workspace_id = $1
  AND utc_date = $2;

-- name: ListProvisionalXUsageEvents :many
SELECT *
FROM x_usage_events
WHERE status = 'provisional'
  AND created_at <= $1
ORDER BY created_at ASC
LIMIT $2;

-- name: GetXInboundCapSetting :one
SELECT
  workspace_id,
  inbound_daily_limit,
  updated_by,
  acknowledged_exposure,
  updated_at
FROM x_inbound_cap_settings
WHERE workspace_id = $1;

-- name: UpsertXInboundCapSetting :one
INSERT INTO x_inbound_cap_settings (
  workspace_id,
  inbound_daily_limit,
  updated_by,
  acknowledged_exposure,
  updated_at
) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (workspace_id) DO UPDATE SET
  inbound_daily_limit = EXCLUDED.inbound_daily_limit,
  updated_by = EXCLUDED.updated_by,
  acknowledged_exposure = EXCLUDED.acknowledged_exposure,
  updated_at = EXCLUDED.updated_at
RETURNING
  workspace_id,
  inbound_daily_limit,
  updated_by,
  acknowledged_exposure,
  updated_at;

-- name: GetXInboundEventReceipt :one
SELECT
  workspace_id,
  social_account_id,
  upstream_resource_type,
  upstream_resource_id,
  utc_date,
  decision,
  weighted_units,
  period_start,
  period_end,
  monthly_used_after,
  monthly_remaining_after,
  inbound_daily_used_after,
  inbound_daily_limit,
  events_accepted_after,
  events_suppressed_after,
  pause_paid_sources,
  pause_reason,
  reset_at,
  created_at
FROM x_inbound_event_receipts
WHERE workspace_id = $1
  AND social_account_id = $2
  AND upstream_resource_type = $3
  AND upstream_resource_id = $4
  AND utc_date = $5;

-- name: ClaimPendingXInboundNotifications :many
WITH candidates AS (
  SELECT id
  FROM x_inbound_cap_notifications
  WHERE (
      status = 'pending'
      OR (status = 'processing' AND lease_expires_at <= NOW())
    )
    AND next_attempt_at <= NOW()
  ORDER BY next_attempt_at ASC, claimed_at ASC
  FOR UPDATE SKIP LOCKED
  LIMIT $1
)
UPDATE x_inbound_cap_notifications n
SET status = 'processing',
    attempts = n.attempts + 1,
    lease_expires_at = NOW() + INTERVAL '5 minutes',
    last_error = NULL
FROM candidates c
WHERE n.id = c.id
RETURNING
  n.id,
  n.workspace_id,
  n.utc_date,
  n.threshold,
  n.event_type,
  n.payload,
  n.status,
  n.attempts,
  n.next_attempt_at,
  n.lease_expires_at,
  n.last_error,
  n.enqueued_at,
  n.claimed_at;

-- name: MarkXInboundNotificationEnqueued :exec
UPDATE x_inbound_cap_notifications
SET status = 'enqueued',
    enqueued_at = NOW(),
    lease_expires_at = NULL,
    last_error = NULL
WHERE id = $1;

-- name: RetryXInboundNotification :exec
UPDATE x_inbound_cap_notifications
SET status = 'pending',
    next_attempt_at = $2,
    lease_expires_at = NULL,
    last_error = $3
WHERE id = $1;
