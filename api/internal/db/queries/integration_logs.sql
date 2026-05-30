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
WHERE workspace_id = sqlc.arg('workspace_id')::text
  AND (sqlc.arg('category')::TEXT = '' OR category = sqlc.arg('category'))
  AND (sqlc.arg('action')::TEXT = '' OR action = sqlc.arg('action'))
  AND (sqlc.arg('source')::TEXT = '' OR source = sqlc.arg('source'))
  AND (sqlc.arg('level')::TEXT = '' OR level = sqlc.arg('level'))
  AND (sqlc.arg('status')::TEXT = '' OR status = sqlc.arg('status'))
  AND (sqlc.arg('platform')::TEXT = '' OR platform = sqlc.arg('platform'))
  AND (sqlc.arg('profile_id')::TEXT = '' OR profile_id = sqlc.arg('profile_id'))
  AND (sqlc.arg('social_account_id')::TEXT = '' OR social_account_id = sqlc.arg('social_account_id'))
  AND (sqlc.arg('post_id')::TEXT = '' OR post_id = sqlc.arg('post_id'))
  AND (sqlc.arg('request_id')::TEXT = '' OR request_id = sqlc.arg('request_id'))
  AND (sqlc.arg('error_code')::TEXT = '' OR error_code = sqlc.arg('error_code'))
  AND (
    sqlc.arg('query')::TEXT = ''
    OR message ILIKE '%' || sqlc.arg('query') || '%'
    OR action ILIKE '%' || sqlc.arg('query') || '%'
    OR request_id ILIKE '%' || sqlc.arg('query') || '%'
    OR post_id ILIKE '%' || sqlc.arg('query') || '%'
    OR error_code ILIKE '%' || sqlc.arg('query') || '%'
  )
  AND ts >= sqlc.arg('from_ts')::timestamptz
  AND ts <= sqlc.arg('to_ts')::timestamptz
ORDER BY ts DESC, id DESC
LIMIT sqlc.arg('limit')::int;

-- name: DeleteExpiredIntegrationLogsForWorkspace :exec
DELETE FROM integration_logs
WHERE workspace_id = $1
  AND ts < $2;
