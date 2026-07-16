-- name: CreateOAuthState :one
INSERT INTO oauth_states (state, profile_id, platform, redirect_url, pkce_verifier)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ConsumeOAuthState :one
DELETE FROM oauth_states
WHERE state = $1 AND expires_at > NOW()
RETURNING *;

-- name: DeleteExpiredOAuthStates :exec
DELETE FROM oauth_states WHERE expires_at <= NOW();
