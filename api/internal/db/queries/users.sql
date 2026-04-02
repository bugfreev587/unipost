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
