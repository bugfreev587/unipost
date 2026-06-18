-- name: CreateSupportBundle :one
INSERT INTO support_bundles (
  id,
  workspace_id,
  actor_user_id,
  actor_api_key_id,
  run_id,
  schema_version,
  cli_version,
  summary,
  report_markdown,
  payload,
  finding_count,
  recent_error_count
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
ON CONFLICT (workspace_id, run_id) DO UPDATE SET
  actor_user_id = EXCLUDED.actor_user_id,
  actor_api_key_id = EXCLUDED.actor_api_key_id,
  schema_version = EXCLUDED.schema_version,
  cli_version = EXCLUDED.cli_version,
  summary = EXCLUDED.summary,
  report_markdown = EXCLUDED.report_markdown,
  payload = EXCLUDED.payload,
  finding_count = EXCLUDED.finding_count,
  recent_error_count = EXCLUDED.recent_error_count,
  created_at = NOW()
RETURNING *;

-- name: ListAdminSupportBundles :many
SELECT
  sb.id,
  sb.workspace_id,
  COALESCE(w.name, '') AS workspace_name,
  COALESCE(u.email, '') AS owner_email,
  COALESCE(s.plan_id, 'free') AS plan_id,
  sb.run_id,
  sb.schema_version,
  sb.cli_version,
  sb.summary,
  sb.finding_count,
  sb.recent_error_count,
  sb.created_at
FROM support_bundles sb
LEFT JOIN workspaces w ON w.id = sb.workspace_id
LEFT JOIN users u ON u.id = w.user_id
LEFT JOIN subscriptions s ON s.workspace_id = w.id
WHERE (sqlc.arg('workspace_id')::TEXT = '' OR sb.workspace_id = sqlc.arg('workspace_id'))
  AND (sqlc.arg('owner_email')::TEXT = '' OR u.email ILIKE '%' || sqlc.arg('owner_email') || '%')
  AND (
    sqlc.arg('query')::TEXT = ''
    OR sb.summary ILIKE '%' || sqlc.arg('query') || '%'
    OR sb.run_id ILIKE '%' || sqlc.arg('query') || '%'
    OR sb.id ILIKE '%' || sqlc.arg('query') || '%'
    OR u.email ILIKE '%' || sqlc.arg('query') || '%'
    OR w.name ILIKE '%' || sqlc.arg('query') || '%'
  )
ORDER BY sb.created_at DESC
LIMIT sqlc.arg('limit')::int;

-- name: GetAdminSupportBundle :one
SELECT
  sb.id,
  sb.workspace_id,
  COALESCE(w.name, '') AS workspace_name,
  COALESCE(u.email, '') AS owner_email,
  COALESCE(s.plan_id, 'free') AS plan_id,
  sb.run_id,
  sb.schema_version,
  sb.cli_version,
  sb.summary,
  sb.report_markdown,
  sb.finding_count,
  sb.recent_error_count,
  sb.created_at
FROM support_bundles sb
LEFT JOIN workspaces w ON w.id = sb.workspace_id
LEFT JOIN users u ON u.id = w.user_id
LEFT JOIN subscriptions s ON s.workspace_id = w.id
WHERE sb.id = $1
LIMIT 1;
