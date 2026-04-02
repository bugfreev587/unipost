-- name: ListAPIKeysByProject :many
SELECT * FROM api_keys
WHERE project_id = $1 AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: CreateAPIKey :one
INSERT INTO api_keys (id, project_id, name, prefix, key_hash, environment, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: RevokeAPIKey :one
UPDATE api_keys SET revoked_at = NOW()
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: GetAPIKey :one
SELECT * FROM api_keys WHERE id = $1;

-- name: GetAPIKeyByHash :one
SELECT * FROM api_keys WHERE key_hash = $1;

-- name: UpdateAPIKeyLastUsedAt :exec
UPDATE api_keys SET last_used_at = NOW() WHERE id = $1;
