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
  created_at
FROM x_inbound_event_receipts
WHERE workspace_id = $1
  AND social_account_id = $2
  AND upstream_resource_type = $3
  AND upstream_resource_id = $4
  AND utc_date = $5;
