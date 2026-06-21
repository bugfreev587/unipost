-- name: ListWorkspacesByUser :many
SELECT * FROM workspaces WHERE user_id = $1 ORDER BY created_at DESC;

-- name: GetDefaultWorkspaceForUser :one
SELECT * FROM workspaces WHERE user_id = $1 ORDER BY created_at ASC LIMIT 1;

-- name: GetWorkspace :one
SELECT * FROM workspaces WHERE id = $1;

-- name: GetWorkspaceByIDAndOwner :one
SELECT * FROM workspaces WHERE id = $1 AND user_id = $2;

-- name: CreateWorkspace :one
INSERT INTO workspaces (user_id, name)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateWorkspace :one
UPDATE workspaces SET name = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateWorkspacePerAccountQuota :one
UPDATE workspaces
SET per_account_monthly_limit = sqlc.narg('per_account_monthly_limit')::INTEGER,
    updated_at                = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateWorkspaceCustomPlatformSlot :one
UPDATE workspaces
SET custom_platform_slot = NULLIF(sqlc.arg('custom_platform_slot')::TEXT, ''),
    updated_at           = NOW()
WHERE id = $1
RETURNING *;

-- name: ClaimWorkspaceCustomPlatformSlot :one
UPDATE workspaces
SET custom_platform_slot = $2,
    updated_at           = NOW()
WHERE id = $1
  AND (custom_platform_slot IS NULL OR custom_platform_slot = $2)
RETURNING *;

-- name: UpdateWorkspaceUsageModes :one
UPDATE workspaces SET usage_modes = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteWorkspace :exec
DELETE FROM workspaces WHERE id = $1;
