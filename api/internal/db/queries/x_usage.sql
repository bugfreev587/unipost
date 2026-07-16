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
