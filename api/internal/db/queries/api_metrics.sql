-- name: InsertAPIMetric :exec
INSERT INTO api_metrics (workspace_id, api_key_id, method, path, status_code, duration_ms)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetAPIMetricsSummary :many
-- Per-endpoint summary for a workspace within a time range.
-- Returns total calls, success (2xx/3xx), client errors (4xx),
-- server errors (5xx), and latency percentiles.
WITH summary AS (
SELECT
  path,
  method,
  COUNT(*)::INTEGER AS total_calls,
  COUNT(*) FILTER (WHERE status_code < 400)::INTEGER AS success_count,
  COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500)::INTEGER AS client_error_count,
  COUNT(*) FILTER (WHERE status_code >= 500)::INTEGER AS server_error_count,
  COUNT(*) FILTER (WHERE status_code = 429)::INTEGER AS rate_limited_count,
  CAST(CASE WHEN COUNT(*) = 0 THEN 0 ELSE (COUNT(*) FILTER (WHERE status_code >= 400)::DOUBLE PRECISION / COUNT(*)::DOUBLE PRECISION) * 100 END AS DOUBLE PRECISION) AS error_rate_pct,
  CAST(CASE WHEN COUNT(*) = 0 THEN 0 ELSE (COUNT(*) FILTER (WHERE status_code >= 500)::DOUBLE PRECISION / COUNT(*)::DOUBLE PRECISION) * 100 END AS DOUBLE PRECISION) AS server_failure_rate_pct,
  COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p50_ms,
  COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p95_ms,
  COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p99_ms,
  COALESCE(AVG(duration_ms), 0)::INTEGER AS avg_ms
FROM api_metrics
WHERE workspace_id = $1
  AND created_at >= $2
  AND created_at <= $3
  AND (sqlc.arg(method_filter)::TEXT = '' OR method = sqlc.arg(method_filter)::TEXT)
  AND (sqlc.arg(path_filter)::TEXT = '' OR path = sqlc.arg(path_filter)::TEXT)
  AND (
    sqlc.arg(status_class)::TEXT = ''
    OR (sqlc.arg(status_class)::TEXT = '2xx' AND status_code >= 200 AND status_code < 300)
    OR (sqlc.arg(status_class)::TEXT = '3xx' AND status_code >= 300 AND status_code < 400)
    OR (sqlc.arg(status_class)::TEXT = '4xx' AND status_code >= 400 AND status_code < 500)
    OR (sqlc.arg(status_class)::TEXT = '5xx' AND status_code >= 500 AND status_code < 600)
  )
GROUP BY path, method
)
SELECT * FROM summary
ORDER BY
  CASE WHEN sqlc.arg(sort_key)::TEXT = 'p95_ms_desc' THEN p95_ms END DESC,
  CASE WHEN sqlc.arg(sort_key)::TEXT = 'p99_ms_desc' THEN p99_ms END DESC,
  CASE WHEN sqlc.arg(sort_key)::TEXT = 'server_errors_desc' THEN server_error_count END DESC,
  CASE WHEN sqlc.arg(sort_key)::TEXT = 'rate_limited_desc' THEN rate_limited_count END DESC,
  total_calls DESC
LIMIT sqlc.arg(row_limit)::INTEGER;

-- name: GetAPIMetricsTrendHourly :many
-- Hourly call counts for a workspace within a time range.
-- Used for the usage-over-time chart.
SELECT
  date_trunc('hour', created_at)::timestamptz AS bucket,
  COUNT(*)::INTEGER AS total_calls,
  COUNT(*) FILTER (WHERE status_code < 400)::INTEGER AS success_count,
  COUNT(*) FILTER (WHERE status_code >= 400)::INTEGER AS error_count,
  COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500)::INTEGER AS client_error_count,
  COUNT(*) FILTER (WHERE status_code >= 500)::INTEGER AS server_error_count,
  COUNT(*) FILTER (WHERE status_code = 429)::INTEGER AS rate_limited_count,
  COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p50_ms,
  COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p95_ms,
  COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p99_ms,
  COALESCE(AVG(duration_ms), 0)::INTEGER AS avg_ms
FROM api_metrics
WHERE workspace_id = $1
  AND created_at >= $2
  AND created_at <= $3
  AND (sqlc.arg(method_filter)::TEXT = '' OR method = sqlc.arg(method_filter)::TEXT)
  AND (sqlc.arg(path_filter)::TEXT = '' OR path = sqlc.arg(path_filter)::TEXT)
  AND (
    sqlc.arg(status_class)::TEXT = ''
    OR (sqlc.arg(status_class)::TEXT = '2xx' AND status_code >= 200 AND status_code < 300)
    OR (sqlc.arg(status_class)::TEXT = '3xx' AND status_code >= 300 AND status_code < 400)
    OR (sqlc.arg(status_class)::TEXT = '4xx' AND status_code >= 400 AND status_code < 500)
    OR (sqlc.arg(status_class)::TEXT = '5xx' AND status_code >= 500 AND status_code < 600)
  )
GROUP BY bucket
ORDER BY bucket;

-- name: GetAPIMetricsTrendDaily :many
SELECT
  date_trunc('day', created_at)::timestamptz AS bucket,
  COUNT(*)::INTEGER AS total_calls,
  COUNT(*) FILTER (WHERE status_code < 400)::INTEGER AS success_count,
  COUNT(*) FILTER (WHERE status_code >= 400)::INTEGER AS error_count,
  COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500)::INTEGER AS client_error_count,
  COUNT(*) FILTER (WHERE status_code >= 500)::INTEGER AS server_error_count,
  COUNT(*) FILTER (WHERE status_code = 429)::INTEGER AS rate_limited_count,
  COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p50_ms,
  COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p95_ms,
  COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p99_ms,
  COALESCE(AVG(duration_ms), 0)::INTEGER AS avg_ms
FROM api_metrics
WHERE workspace_id = $1
  AND created_at >= $2
  AND created_at <= $3
  AND (sqlc.arg(method_filter)::TEXT = '' OR method = sqlc.arg(method_filter)::TEXT)
  AND (sqlc.arg(path_filter)::TEXT = '' OR path = sqlc.arg(path_filter)::TEXT)
  AND (
    sqlc.arg(status_class)::TEXT = ''
    OR (sqlc.arg(status_class)::TEXT = '2xx' AND status_code >= 200 AND status_code < 300)
    OR (sqlc.arg(status_class)::TEXT = '3xx' AND status_code >= 300 AND status_code < 400)
    OR (sqlc.arg(status_class)::TEXT = '4xx' AND status_code >= 400 AND status_code < 500)
    OR (sqlc.arg(status_class)::TEXT = '5xx' AND status_code >= 500 AND status_code < 600)
  )
GROUP BY bucket
ORDER BY bucket;

-- name: GetAPIMetricsOverall :one
-- Overall stats for a workspace within a time range.
SELECT
  COUNT(*)::INTEGER AS total_calls,
  COUNT(*) FILTER (WHERE status_code < 400)::INTEGER AS success_count,
  COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500)::INTEGER AS client_error_count,
  COUNT(*) FILTER (WHERE status_code >= 500)::INTEGER AS server_error_count,
  COUNT(*) FILTER (WHERE status_code = 429)::INTEGER AS rate_limited_count,
  CAST(CASE WHEN COUNT(*) = 0 THEN 0 ELSE (COUNT(*) FILTER (WHERE status_code >= 400)::DOUBLE PRECISION / COUNT(*)::DOUBLE PRECISION) * 100 END AS DOUBLE PRECISION) AS error_rate_pct,
  CAST(CASE WHEN COUNT(*) = 0 THEN 0 ELSE (COUNT(*) FILTER (WHERE status_code >= 500)::DOUBLE PRECISION / COUNT(*)::DOUBLE PRECISION) * 100 END AS DOUBLE PRECISION) AS server_failure_rate_pct,
  COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p50_ms,
  COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p95_ms,
  COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p99_ms,
  COALESCE(AVG(duration_ms), 0)::INTEGER AS avg_ms
FROM api_metrics
WHERE workspace_id = $1
  AND created_at >= $2
  AND created_at <= $3
  AND (sqlc.arg(method_filter)::TEXT = '' OR method = sqlc.arg(method_filter)::TEXT)
  AND (sqlc.arg(path_filter)::TEXT = '' OR path = sqlc.arg(path_filter)::TEXT)
  AND (
    sqlc.arg(status_class)::TEXT = ''
    OR (sqlc.arg(status_class)::TEXT = '2xx' AND status_code >= 200 AND status_code < 300)
    OR (sqlc.arg(status_class)::TEXT = '3xx' AND status_code >= 300 AND status_code < 400)
    OR (sqlc.arg(status_class)::TEXT = '4xx' AND status_code >= 400 AND status_code < 500)
    OR (sqlc.arg(status_class)::TEXT = '5xx' AND status_code >= 500 AND status_code < 600)
  );

-- name: GetAPIMetricsStatusCodes :many
SELECT
  status_code,
  method,
  path,
  COUNT(*)::INTEGER AS total_calls
FROM api_metrics
WHERE workspace_id = $1
  AND created_at >= $2
  AND created_at <= $3
  AND (sqlc.arg(method_filter)::TEXT = '' OR method = sqlc.arg(method_filter)::TEXT)
  AND (sqlc.arg(path_filter)::TEXT = '' OR path = sqlc.arg(path_filter)::TEXT)
  AND (
    sqlc.arg(status_class)::TEXT = ''
    OR (sqlc.arg(status_class)::TEXT = '2xx' AND status_code >= 200 AND status_code < 300)
    OR (sqlc.arg(status_class)::TEXT = '3xx' AND status_code >= 300 AND status_code < 400)
    OR (sqlc.arg(status_class)::TEXT = '4xx' AND status_code >= 400 AND status_code < 500)
    OR (sqlc.arg(status_class)::TEXT = '5xx' AND status_code >= 500 AND status_code < 600)
  )
GROUP BY status_code, method, path
ORDER BY total_calls DESC, status_code ASC
LIMIT sqlc.arg(row_limit)::INTEGER;

-- name: GetAdminAPIMetricsOverall :one
SELECT
  COUNT(*)::INTEGER AS total_calls,
  COUNT(*) FILTER (WHERE status_code < 400)::INTEGER AS success_count,
  COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500)::INTEGER AS client_error_count,
  COUNT(*) FILTER (WHERE status_code >= 500)::INTEGER AS server_error_count,
  COUNT(*) FILTER (WHERE status_code = 429)::INTEGER AS rate_limited_count,
  CAST(CASE WHEN COUNT(*) = 0 THEN 0 ELSE (COUNT(*) FILTER (WHERE status_code >= 400)::DOUBLE PRECISION / COUNT(*)::DOUBLE PRECISION) * 100 END AS DOUBLE PRECISION) AS error_rate_pct,
  CAST(CASE WHEN COUNT(*) = 0 THEN 0 ELSE (COUNT(*) FILTER (WHERE status_code >= 500)::DOUBLE PRECISION / COUNT(*)::DOUBLE PRECISION) * 100 END AS DOUBLE PRECISION) AS server_failure_rate_pct,
  COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p50_ms,
  COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p95_ms,
  COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p99_ms,
  COALESCE(AVG(duration_ms), 0)::INTEGER AS avg_ms
FROM api_metrics
WHERE created_at >= $1
  AND created_at <= $2
  AND (sqlc.arg(workspace_filter)::TEXT = '' OR workspace_id = sqlc.arg(workspace_filter)::TEXT)
  AND (sqlc.arg(method_filter)::TEXT = '' OR method = sqlc.arg(method_filter)::TEXT)
  AND (sqlc.arg(path_filter)::TEXT = '' OR path = sqlc.arg(path_filter)::TEXT)
  AND (
    sqlc.arg(status_class)::TEXT = ''
    OR (sqlc.arg(status_class)::TEXT = '2xx' AND status_code >= 200 AND status_code < 300)
    OR (sqlc.arg(status_class)::TEXT = '3xx' AND status_code >= 300 AND status_code < 400)
    OR (sqlc.arg(status_class)::TEXT = '4xx' AND status_code >= 400 AND status_code < 500)
    OR (sqlc.arg(status_class)::TEXT = '5xx' AND status_code >= 500 AND status_code < 600)
  );

-- name: GetAdminAPIMetricsSummary :many
WITH summary AS (
SELECT
  path,
  method,
  COUNT(*)::INTEGER AS total_calls,
  COUNT(*) FILTER (WHERE status_code < 400)::INTEGER AS success_count,
  COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500)::INTEGER AS client_error_count,
  COUNT(*) FILTER (WHERE status_code >= 500)::INTEGER AS server_error_count,
  COUNT(*) FILTER (WHERE status_code = 429)::INTEGER AS rate_limited_count,
  CAST(CASE WHEN COUNT(*) = 0 THEN 0 ELSE (COUNT(*) FILTER (WHERE status_code >= 400)::DOUBLE PRECISION / COUNT(*)::DOUBLE PRECISION) * 100 END AS DOUBLE PRECISION) AS error_rate_pct,
  CAST(CASE WHEN COUNT(*) = 0 THEN 0 ELSE (COUNT(*) FILTER (WHERE status_code >= 500)::DOUBLE PRECISION / COUNT(*)::DOUBLE PRECISION) * 100 END AS DOUBLE PRECISION) AS server_failure_rate_pct,
  COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p50_ms,
  COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p95_ms,
  COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p99_ms,
  COALESCE(AVG(duration_ms), 0)::INTEGER AS avg_ms
FROM api_metrics
WHERE created_at >= $1
  AND created_at <= $2
  AND (sqlc.arg(workspace_filter)::TEXT = '' OR workspace_id = sqlc.arg(workspace_filter)::TEXT)
  AND (sqlc.arg(method_filter)::TEXT = '' OR method = sqlc.arg(method_filter)::TEXT)
  AND (sqlc.arg(path_filter)::TEXT = '' OR path = sqlc.arg(path_filter)::TEXT)
  AND (
    sqlc.arg(status_class)::TEXT = ''
    OR (sqlc.arg(status_class)::TEXT = '2xx' AND status_code >= 200 AND status_code < 300)
    OR (sqlc.arg(status_class)::TEXT = '3xx' AND status_code >= 300 AND status_code < 400)
    OR (sqlc.arg(status_class)::TEXT = '4xx' AND status_code >= 400 AND status_code < 500)
    OR (sqlc.arg(status_class)::TEXT = '5xx' AND status_code >= 500 AND status_code < 600)
  )
GROUP BY path, method
HAVING COUNT(*) >= sqlc.arg(min_calls)::INTEGER
)
SELECT * FROM summary
ORDER BY
  CASE WHEN sqlc.arg(sort_key)::TEXT = 'p95_ms_desc' THEN p95_ms END DESC,
  CASE WHEN sqlc.arg(sort_key)::TEXT = 'p99_ms_desc' THEN p99_ms END DESC,
  CASE WHEN sqlc.arg(sort_key)::TEXT = 'server_errors_desc' THEN server_error_count END DESC,
  CASE WHEN sqlc.arg(sort_key)::TEXT = 'rate_limited_desc' THEN rate_limited_count END DESC,
  total_calls DESC
LIMIT sqlc.arg(row_limit)::INTEGER;

-- name: GetAdminAPIMetricsTrendHourly :many
SELECT
  date_trunc('hour', created_at)::timestamptz AS bucket,
  COUNT(*)::INTEGER AS total_calls,
  COUNT(*) FILTER (WHERE status_code < 400)::INTEGER AS success_count,
  COUNT(*) FILTER (WHERE status_code >= 400)::INTEGER AS error_count,
  COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500)::INTEGER AS client_error_count,
  COUNT(*) FILTER (WHERE status_code >= 500)::INTEGER AS server_error_count,
  COUNT(*) FILTER (WHERE status_code = 429)::INTEGER AS rate_limited_count,
  COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p50_ms,
  COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p95_ms,
  COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p99_ms,
  COALESCE(AVG(duration_ms), 0)::INTEGER AS avg_ms
FROM api_metrics
WHERE created_at >= $1
  AND created_at <= $2
  AND (sqlc.arg(workspace_filter)::TEXT = '' OR workspace_id = sqlc.arg(workspace_filter)::TEXT)
  AND (sqlc.arg(method_filter)::TEXT = '' OR method = sqlc.arg(method_filter)::TEXT)
  AND (sqlc.arg(path_filter)::TEXT = '' OR path = sqlc.arg(path_filter)::TEXT)
  AND (
    sqlc.arg(status_class)::TEXT = ''
    OR (sqlc.arg(status_class)::TEXT = '2xx' AND status_code >= 200 AND status_code < 300)
    OR (sqlc.arg(status_class)::TEXT = '3xx' AND status_code >= 300 AND status_code < 400)
    OR (sqlc.arg(status_class)::TEXT = '4xx' AND status_code >= 400 AND status_code < 500)
    OR (sqlc.arg(status_class)::TEXT = '5xx' AND status_code >= 500 AND status_code < 600)
  )
GROUP BY bucket
ORDER BY bucket;

-- name: GetAdminAPIMetricsTrendDaily :many
SELECT
  date_trunc('day', created_at)::timestamptz AS bucket,
  COUNT(*)::INTEGER AS total_calls,
  COUNT(*) FILTER (WHERE status_code < 400)::INTEGER AS success_count,
  COUNT(*) FILTER (WHERE status_code >= 400)::INTEGER AS error_count,
  COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500)::INTEGER AS client_error_count,
  COUNT(*) FILTER (WHERE status_code >= 500)::INTEGER AS server_error_count,
  COUNT(*) FILTER (WHERE status_code = 429)::INTEGER AS rate_limited_count,
  COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p50_ms,
  COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p95_ms,
  COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY duration_ms), 0)::INTEGER AS p99_ms,
  COALESCE(AVG(duration_ms), 0)::INTEGER AS avg_ms
FROM api_metrics
WHERE created_at >= $1
  AND created_at <= $2
  AND (sqlc.arg(workspace_filter)::TEXT = '' OR workspace_id = sqlc.arg(workspace_filter)::TEXT)
  AND (sqlc.arg(method_filter)::TEXT = '' OR method = sqlc.arg(method_filter)::TEXT)
  AND (sqlc.arg(path_filter)::TEXT = '' OR path = sqlc.arg(path_filter)::TEXT)
  AND (
    sqlc.arg(status_class)::TEXT = ''
    OR (sqlc.arg(status_class)::TEXT = '2xx' AND status_code >= 200 AND status_code < 300)
    OR (sqlc.arg(status_class)::TEXT = '3xx' AND status_code >= 300 AND status_code < 400)
    OR (sqlc.arg(status_class)::TEXT = '4xx' AND status_code >= 400 AND status_code < 500)
    OR (sqlc.arg(status_class)::TEXT = '5xx' AND status_code >= 500 AND status_code < 600)
  )
GROUP BY bucket
ORDER BY bucket;

-- name: GetAdminAPIMetricsStatusCodes :many
SELECT
  status_code,
  method,
  path,
  COUNT(*)::INTEGER AS total_calls
FROM api_metrics
WHERE created_at >= $1
  AND created_at <= $2
  AND (sqlc.arg(workspace_filter)::TEXT = '' OR workspace_id = sqlc.arg(workspace_filter)::TEXT)
  AND (sqlc.arg(method_filter)::TEXT = '' OR method = sqlc.arg(method_filter)::TEXT)
  AND (sqlc.arg(path_filter)::TEXT = '' OR path = sqlc.arg(path_filter)::TEXT)
  AND (
    sqlc.arg(status_class)::TEXT = ''
    OR (sqlc.arg(status_class)::TEXT = '2xx' AND status_code >= 200 AND status_code < 300)
    OR (sqlc.arg(status_class)::TEXT = '3xx' AND status_code >= 300 AND status_code < 400)
    OR (sqlc.arg(status_class)::TEXT = '4xx' AND status_code >= 400 AND status_code < 500)
    OR (sqlc.arg(status_class)::TEXT = '5xx' AND status_code >= 500 AND status_code < 600)
  )
GROUP BY status_code, method, path
ORDER BY total_calls DESC, status_code ASC
LIMIT sqlc.arg(row_limit)::INTEGER;

-- name: GetAdminAPIMetricsWorkspaces :many
WITH workspace_stats AS (
  SELECT
    am.workspace_id,
    COALESCE(w.name, '') AS workspace_name,
    COUNT(*)::INTEGER AS total_calls,
    COUNT(*) FILTER (WHERE am.status_code = 429)::INTEGER AS rate_limited_count,
    CAST(CASE WHEN COUNT(*) = 0 THEN 0 ELSE (COUNT(*) FILTER (WHERE am.status_code >= 400)::DOUBLE PRECISION / COUNT(*)::DOUBLE PRECISION) * 100 END AS DOUBLE PRECISION) AS error_rate_pct,
    CAST(CASE WHEN COUNT(*) = 0 THEN 0 ELSE (COUNT(*) FILTER (WHERE am.status_code >= 500)::DOUBLE PRECISION / COUNT(*)::DOUBLE PRECISION) * 100 END AS DOUBLE PRECISION) AS server_failure_rate_pct,
    COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY am.duration_ms), 0)::INTEGER AS p95_ms,
    COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY am.duration_ms), 0)::INTEGER AS p99_ms
  FROM api_metrics am
  LEFT JOIN workspaces w ON w.id = am.workspace_id
  WHERE am.created_at >= $1
    AND am.created_at <= $2
    AND (sqlc.arg(workspace_filter)::TEXT = '' OR am.workspace_id = sqlc.arg(workspace_filter)::TEXT)
    AND (sqlc.arg(method_filter)::TEXT = '' OR am.method = sqlc.arg(method_filter)::TEXT)
    AND (sqlc.arg(path_filter)::TEXT = '' OR am.path = sqlc.arg(path_filter)::TEXT)
    AND (
      sqlc.arg(status_class)::TEXT = ''
      OR (sqlc.arg(status_class)::TEXT = '2xx' AND am.status_code >= 200 AND am.status_code < 300)
      OR (sqlc.arg(status_class)::TEXT = '3xx' AND am.status_code >= 300 AND am.status_code < 400)
      OR (sqlc.arg(status_class)::TEXT = '4xx' AND am.status_code >= 400 AND am.status_code < 500)
      OR (sqlc.arg(status_class)::TEXT = '5xx' AND am.status_code >= 500 AND am.status_code < 600)
    )
  GROUP BY am.workspace_id, w.name
  HAVING COUNT(*) >= sqlc.arg(min_calls)::INTEGER
),
endpoint_p95 AS (
  SELECT
    am.workspace_id,
    am.path AS slowest_endpoint,
    COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY am.duration_ms), 0)::INTEGER AS slowest_endpoint_p95_ms
  FROM api_metrics am
  WHERE am.created_at >= $1
    AND am.created_at <= $2
    AND (sqlc.arg(workspace_filter)::TEXT = '' OR am.workspace_id = sqlc.arg(workspace_filter)::TEXT)
    AND (sqlc.arg(method_filter)::TEXT = '' OR am.method = sqlc.arg(method_filter)::TEXT)
    AND (sqlc.arg(path_filter)::TEXT = '' OR am.path = sqlc.arg(path_filter)::TEXT)
    AND (
      sqlc.arg(status_class)::TEXT = ''
      OR (sqlc.arg(status_class)::TEXT = '2xx' AND am.status_code >= 200 AND am.status_code < 300)
      OR (sqlc.arg(status_class)::TEXT = '3xx' AND am.status_code >= 300 AND am.status_code < 400)
      OR (sqlc.arg(status_class)::TEXT = '4xx' AND am.status_code >= 400 AND am.status_code < 500)
      OR (sqlc.arg(status_class)::TEXT = '5xx' AND am.status_code >= 500 AND am.status_code < 600)
    )
  GROUP BY am.workspace_id, am.path
),
endpoint_stats AS (
  SELECT
    workspace_id,
    slowest_endpoint,
    slowest_endpoint_p95_ms,
    ROW_NUMBER() OVER (
      PARTITION BY workspace_id
      ORDER BY slowest_endpoint_p95_ms DESC, slowest_endpoint ASC
    ) AS rn
  FROM endpoint_p95
)
SELECT
  ws.workspace_id,
  ws.workspace_name,
  ws.total_calls,
  ws.rate_limited_count,
  ws.error_rate_pct,
  ws.server_failure_rate_pct,
  ws.p95_ms,
  ws.p99_ms,
  COALESCE(es.slowest_endpoint, '') AS slowest_endpoint,
  COALESCE(es.slowest_endpoint_p95_ms, 0)::INTEGER AS slowest_endpoint_p95_ms
FROM workspace_stats ws
LEFT JOIN endpoint_stats es ON es.workspace_id = ws.workspace_id AND es.rn = 1
ORDER BY
  CASE WHEN sqlc.arg(sort_key)::TEXT = 'p95_ms_desc' THEN ws.p95_ms END DESC,
  CASE WHEN sqlc.arg(sort_key)::TEXT = 'p99_ms_desc' THEN ws.p99_ms END DESC,
  CASE WHEN sqlc.arg(sort_key)::TEXT = 'rate_limited_desc' THEN ws.rate_limited_count END DESC,
  ws.total_calls DESC
LIMIT sqlc.arg(row_limit)::INTEGER;
