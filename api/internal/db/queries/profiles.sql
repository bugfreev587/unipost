-- name: ListProfilesByWorkspace :many
SELECT id, name, created_at, updated_at, branding_logo_url, branding_display_name, branding_primary_color, workspace_id, branding_hide_powered_by, branding_logo_storage_key FROM profiles WHERE workspace_id = $1 ORDER BY created_at DESC;

-- name: CountProfilesByWorkspace :one
-- Used by the profile-create gate (migration 059) to enforce
-- max_profiles per plan. Cheap — workspaces have at most a few dozen
-- profiles even on Growth, so a sequential count is fine.
SELECT COUNT(*)::INTEGER AS count FROM profiles WHERE workspace_id = $1;

-- name: GetProfile :one
SELECT id, name, created_at, updated_at, branding_logo_url, branding_display_name, branding_primary_color, workspace_id, branding_hide_powered_by, branding_logo_storage_key FROM profiles WHERE id = $1;

-- name: CreateProfile :one
INSERT INTO profiles (workspace_id, name)
VALUES ($1, $2)
RETURNING id, name, created_at, updated_at, branding_logo_url, branding_display_name, branding_primary_color, workspace_id, branding_hide_powered_by, branding_logo_storage_key;

-- name: UpdateProfile :one
UPDATE profiles SET name = $2, updated_at = NOW()
WHERE id = $1
RETURNING id, name, created_at, updated_at, branding_logo_url, branding_display_name, branding_primary_color, workspace_id, branding_hide_powered_by, branding_logo_storage_key;

-- name: UpdateProfileBranding :one
UPDATE profiles
SET branding_logo_url      = COALESCE(sqlc.narg('logo_url')::TEXT,      branding_logo_url),
    branding_display_name  = COALESCE(sqlc.narg('display_name')::TEXT,  branding_display_name),
    branding_primary_color = COALESCE(sqlc.narg('primary_color')::TEXT, branding_primary_color),
    branding_hide_powered_by = COALESCE(sqlc.narg('hide_powered_by')::BOOLEAN, branding_hide_powered_by),
    branding_logo_storage_key = CASE
        WHEN sqlc.narg('logo_url')::TEXT IS NULL THEN branding_logo_storage_key
        ELSE NULL
    END,
    updated_at             = NOW()
WHERE id = $1
RETURNING id, name, created_at, updated_at, branding_logo_url, branding_display_name, branding_primary_color, workspace_id, branding_hide_powered_by, branding_logo_storage_key;

-- name: UpdateProfileBrandingLogo :one
UPDATE profiles
SET branding_logo_url = $2,
    branding_logo_storage_key = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING id, name, created_at, updated_at, branding_logo_url, branding_display_name, branding_primary_color, workspace_id, branding_hide_powered_by, branding_logo_storage_key;

-- name: ClearProfileBrandingLogo :one
UPDATE profiles
SET branding_logo_url = NULL,
    branding_logo_storage_key = NULL,
    updated_at = NOW()
WHERE id = $1
RETURNING id, name, created_at, updated_at, branding_logo_url, branding_display_name, branding_primary_color, workspace_id, branding_hide_powered_by, branding_logo_storage_key;

-- name: DeleteProfile :exec
DELETE FROM profiles WHERE id = $1;

-- name: GetProfileByIDAndWorkspaceOwner :one
-- Single-query ownership check: joins profiles -> workspaces to verify
-- the requesting user owns the workspace that contains this profile.
SELECT p.id, p.name, p.created_at, p.updated_at, p.branding_logo_url, p.branding_display_name, p.branding_primary_color, p.workspace_id, p.branding_hide_powered_by, p.branding_logo_storage_key FROM profiles p
JOIN workspaces w ON w.id = p.workspace_id
WHERE p.id = $1 AND w.user_id = $2;
