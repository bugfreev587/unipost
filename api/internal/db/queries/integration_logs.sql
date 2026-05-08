-- name: InsertIntegrationLog :one
INSERT INTO integration_logs (
    workspace_id, ts, level, status, category, action, source, message,
    request_id, trace_id,
    actor_user_id, actor_api_key_id,
    profile_id, social_account_id, post_id, platform_post_id, platform,
    endpoint, method, http_status_code, remote_status_code, duration_ms,
    error_code, metadata, request_payload, response_payload
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8,
    $9, $10,
    $11, $12,
    $13, $14, $15, $16, $17,
    $18, $19, $20, $21, $22,
    $23, $24, $25, $26
) RETURNING id, workspace_id, ts, level, status, category, action, source, message,
            request_id, trace_id,
            actor_user_id, actor_api_key_id,
            profile_id, social_account_id, post_id, platform_post_id, platform,
            endpoint, method, http_status_code, remote_status_code, duration_ms,
            error_code, metadata, request_payload, response_payload, created_at;

-- name: GetIntegrationLog :one
SELECT id, workspace_id, ts, level, status, category, action, source, message,
       request_id, trace_id,
       actor_user_id, actor_api_key_id,
       profile_id, social_account_id, post_id, platform_post_id, platform,
       endpoint, method, http_status_code, remote_status_code, duration_ms,
       error_code, metadata, request_payload, response_payload, created_at
FROM integration_logs
WHERE id = $1 AND workspace_id = $2;

-- name: ListIntegrationLogs :many
SELECT id, workspace_id, ts, level, status, category, action, source, message,
       request_id, trace_id,
       actor_user_id, actor_api_key_id,
       profile_id, social_account_id, post_id, platform_post_id, platform,
       endpoint, method, http_status_code, remote_status_code, duration_ms,
       error_code, metadata, request_payload, response_payload, created_at
FROM integration_logs
WHERE workspace_id = $1
  AND ($2::TEXT = '' OR category = $2)
  AND ($3::TEXT = '' OR action = $3)
  AND ($4::TEXT = '' OR source = $4)
  AND ($5::TEXT = '' OR level = $5)
  AND ($6::TEXT = '' OR status = $6)
  AND ($7::TEXT = '' OR platform = $7)
  AND ($8::TEXT = '' OR profile_id = $8)
  AND ($9::TEXT = '' OR social_account_id = $9)
  AND ($10::TEXT = '' OR post_id = $10)
  AND ($11::TEXT = '' OR request_id = $11)
  AND ($12::TEXT = '' OR error_code = $12)
  AND (
    $13::TEXT = ''
    OR message ILIKE '%' || $13 || '%'
    OR action ILIKE '%' || $13 || '%'
    OR request_id ILIKE '%' || $13 || '%'
    OR post_id ILIKE '%' || $13 || '%'
    OR error_code ILIKE '%' || $13 || '%'
  )
  AND ts >= $14
  AND ts <= $15
ORDER BY ts DESC, id DESC
LIMIT $16;

-- name: DeleteExpiredIntegrationLogsForWorkspace :exec
DELETE FROM integration_logs
WHERE workspace_id = $1
  AND ts < $2;
