-- name: ListProjectsByOwner :many
SELECT * FROM projects WHERE owner_id = $1 ORDER BY created_at DESC;

-- name: GetProject :one
SELECT * FROM projects WHERE id = $1;

-- name: CreateProject :one
INSERT INTO projects (owner_id, name)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateProject :one
UPDATE projects SET name = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateProjectBranding :one
-- Sprint 4 PR4: white-label Connect page branding. nullable params
-- via sqlc.narg let the caller patch a subset (e.g. just the logo)
-- without forcing them to re-supply name + color. NULL passed in
-- via sqlc.narg means "leave the column unchanged"; pass an empty
-- string to clear a value.
UPDATE projects
SET branding_logo_url      = COALESCE(sqlc.narg('logo_url')::TEXT,      branding_logo_url),
    branding_display_name  = COALESCE(sqlc.narg('display_name')::TEXT,  branding_display_name),
    branding_primary_color = COALESCE(sqlc.narg('primary_color')::TEXT, branding_primary_color),
    updated_at             = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteProject :exec
DELETE FROM projects WHERE id = $1;

-- name: GetProjectByIDAndOwner :one
SELECT * FROM projects WHERE id = $1 AND owner_id = $2;

-- name: UpdateProjectPerAccountQuota :one
-- Sprint 5 PR2: set / clear the per-social-account monthly publish
-- cap. Pass NULL to disable the cap (the default — unlimited).
-- Pass a positive integer to enforce. Zero is allowed and means
-- "this account cannot publish at all this month" — handy for
-- emergency lockouts. The publish path counts published_at rows in
-- the current calendar month and refuses dispatch when count >= cap.
UPDATE projects
SET per_account_monthly_limit = sqlc.narg('per_account_monthly_limit')::INTEGER,
    updated_at                = NOW()
WHERE id = $1
RETURNING *;
