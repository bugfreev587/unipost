-- name: WriteAuditLog :exec
-- Single insert path for the audit subsystem. All mutations route
-- through internal/audit.Log() which calls this. before_json /
-- after_json / metadata are JSONB — pass nil pgtype.JSON when not
-- applicable.
INSERT INTO audit_log (
    workspace_id, actor_user_id, actor_api_key_id, action, resource_type,
    resource_id, category, ip_address, user_agent, before_json, after_json, metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: ListAuditLogByWorkspace :many
-- Paginated scan of audit events for the workspace. Filters are
-- optional (empty string = no filter). Ordered newest-first to match
-- the typical "what just changed?" use case.
SELECT * FROM audit_log
WHERE workspace_id = $1
  AND ($2::TEXT = '' OR action = $2)
  AND ($3::TEXT = '' OR category = $3)
  AND created_at >= $4
ORDER BY created_at DESC
LIMIT $5;
