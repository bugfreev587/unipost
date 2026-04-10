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

-- name: SetUserDefaultProfile :exec
UPDATE users SET default_profile_id = $2, updated_at = NOW()
WHERE id = $1;

-- name: SetUserLastProfile :exec
UPDATE users SET last_profile_id = $2 WHERE id = $1;
