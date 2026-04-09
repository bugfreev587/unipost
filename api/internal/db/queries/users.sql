-- name: UpsertUser :one
INSERT INTO users (id, email, name)
VALUES ($1, $2, $3)
ON CONFLICT (id)
DO UPDATE SET email = $2, name = $3, updated_at = NOW()
RETURNING *;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: SetUserDefaultProject :exec
-- Stamps the user's auto-created "Default" project. Called once
-- from /me/bootstrap when default_project_id IS NULL — either at
-- first signup (create + stamp) or as a lazy backfill for legacy
-- users with existing projects (oldest project gets stamped).
UPDATE users SET default_project_id = $2, updated_at = NOW()
WHERE id = $1;

-- name: SetUserLastProject :exec
-- Side-effect of GET /v1/projects/{id}: tracks the last project the
-- user actually rendered, so the dashboard root can redirect them
-- back to it on the next visit. No updated_at touch — this fires
-- on every project page navigation and we don't want it to look
-- like the user row was edited every time.
UPDATE users SET last_project_id = $2 WHERE id = $1;
