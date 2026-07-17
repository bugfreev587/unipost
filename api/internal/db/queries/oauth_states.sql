-- name: CreateOAuthState :one
INSERT INTO oauth_states (state, profile_id, platform, redirect_url, pkce_verifier, x_app_mode)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING state, profile_id, platform, redirect_url, expires_at, created_at,
  pkce_verifier, x_app_mode;

-- name: ConsumeOAuthState :one
DELETE FROM oauth_states
WHERE state = $1 AND expires_at > NOW()
RETURNING state, profile_id, platform, redirect_url, expires_at, created_at,
  pkce_verifier, x_app_mode;

-- name: DeleteExpiredOAuthStates :exec
DELETE FROM oauth_states WHERE expires_at <= NOW();
