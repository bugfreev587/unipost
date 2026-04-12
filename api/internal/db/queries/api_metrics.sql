-- name: InsertAPIMetric :exec
INSERT INTO api_metrics (workspace_id, method, path, status_code, duration_ms)
VALUES ($1, $2, $3, $4, $5);

-- name: GetAPIMetricsSummary :many
-- Per-endpoint summary for a workspace within a time range.
-- Returns total calls, success (2xx/3xx), client errors (4xx),
-- server errors (5xx), and latency percentiles.
SELECT
  path,
  method,
  COUNT(*)::INTEGER AS total_calls,
  COUNT(*) FILTER (WHERE status_code < 400)::INTEGER AS success_count,
  COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500)::INTEGER AS client_error_count,
  COUNT(*) FILTER (WHERE status_code >= 500)::INTEGER AS server_error_count,
  COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p50_ms,
  COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p95_ms,
  COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p99_ms,
  COALESCE(AVG(duration_ms), 0)::INTEGER AS avg_ms
FROM api_metrics
WHERE workspace_id = $1
  AND created_at >= $2
  AND created_at <= $3
GROUP BY path, method
ORDER BY total_calls DESC;

-- name: GetAPIMetricsTrend :many
-- Hourly call counts for a workspace within a time range.
-- Used for the usage-over-time chart.
SELECT
  date_trunc('hour', created_at)::timestamptz AS bucket,
  COUNT(*)::INTEGER AS total_calls,
  COUNT(*) FILTER (WHERE status_code < 400)::INTEGER AS success_count,
  COUNT(*) FILTER (WHERE status_code >= 400)::INTEGER AS error_count
FROM api_metrics
WHERE workspace_id = $1
  AND created_at >= $2
  AND created_at <= $3
GROUP BY bucket
ORDER BY bucket;

-- name: GetAPIMetricsOverall :one
-- Overall stats for a workspace within a time range.
SELECT
  COUNT(*)::INTEGER AS total_calls,
  COUNT(*) FILTER (WHERE status_code < 400)::INTEGER AS success_count,
  COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500)::INTEGER AS client_error_count,
  COUNT(*) FILTER (WHERE status_code >= 500)::INTEGER AS server_error_count,
  COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p50_ms,
  COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p95_ms,
  COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p99_ms
FROM api_metrics
WHERE workspace_id = $1
  AND created_at >= $2
  AND created_at <= $3;
