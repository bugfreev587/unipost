-- name: CreateOAuthState :one
INSERT INTO oauth_states (state, profile_id, platform, redirect_url)
VALUES ($1, $2, $3, $4)
RETURNING state, profile_id, platform, redirect_url, expires_at, created_at;

-- name: GetOAuthState :one
SELECT state, profile_id, platform, redirect_url, expires_at, created_at
FROM oauth_states
WHERE state = $1 AND expires_at > NOW();

-- name: DeleteOAuthState :exec
DELETE FROM oauth_states WHERE state = $1;

-- name: DeleteExpiredOAuthStates :exec
DELETE FROM oauth_states WHERE expires_at <= NOW();
