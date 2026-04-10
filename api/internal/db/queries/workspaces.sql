-- name: ListWorkspacesByUser :many
SELECT * FROM workspaces WHERE user_id = $1 ORDER BY created_at DESC;

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

-- name: DeleteWorkspace :exec
DELETE FROM workspaces WHERE id = $1;
