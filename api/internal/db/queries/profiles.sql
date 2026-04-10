-- name: ListProfilesByWorkspace :many
SELECT * FROM profiles WHERE workspace_id = $1 ORDER BY created_at DESC;

-- name: GetProfile :one
SELECT * FROM profiles WHERE id = $1;

-- name: CreateProfile :one
INSERT INTO profiles (workspace_id, name)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateProfile :one
UPDATE profiles SET name = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateProfileBranding :one
UPDATE profiles
SET branding_logo_url      = COALESCE(sqlc.narg('logo_url')::TEXT,      branding_logo_url),
    branding_display_name  = COALESCE(sqlc.narg('display_name')::TEXT,  branding_display_name),
    branding_primary_color = COALESCE(sqlc.narg('primary_color')::TEXT, branding_primary_color),
    updated_at             = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteProfile :exec
DELETE FROM profiles WHERE id = $1;

-- name: GetProfileByIDAndWorkspaceOwner :one
-- Single-query ownership check: joins profiles -> workspaces to verify
-- the requesting user owns the workspace that contains this profile.
SELECT p.* FROM profiles p
JOIN workspaces w ON w.id = p.workspace_id
WHERE p.id = $1 AND w.user_id = $2;
