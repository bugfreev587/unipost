-- name: ListAPIKeysByWorkspace :many
SELECT * FROM api_keys
WHERE workspace_id = $1 AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: CreateAPIKey :one
-- created_by_user_id (RBAC migration 063) attributes the key to the
-- member who created it. The auth path uses this to derive the key's
-- role from the creator's current membership.
INSERT INTO api_keys (id, workspace_id, name, prefix, key_hash, environment, expires_at, created_by_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: RevokeAPIKey :one
UPDATE api_keys SET revoked_at = NOW()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: GetAPIKey :one
SELECT * FROM api_keys WHERE id = $1;

-- name: GetAPIKeyByHash :one
SELECT * FROM api_keys WHERE key_hash = $1;

-- name: UpdateAPIKeyLastUsedAt :exec
UPDATE api_keys SET last_used_at = NOW() WHERE id = $1;
