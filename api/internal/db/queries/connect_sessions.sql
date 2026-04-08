-- name: CreateConnectSession :one
INSERT INTO connect_sessions (
  project_id, platform, external_user_id, external_user_email,
  return_url, oauth_state, pkce_verifier, expires_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetConnectSessionByID :one
SELECT * FROM connect_sessions
WHERE id = $1 AND project_id = $2;

-- name: GetConnectSessionByOAuthState :one
-- Public lookup used by both the hosted dashboard page and the OAuth
-- callback. The oauth_state is the bearer — no project_id check here.
SELECT * FROM connect_sessions
WHERE oauth_state = $1;

-- name: MarkConnectSessionCompleted :one
UPDATE connect_sessions
SET status = 'completed',
    completed_social_account_id = $2,
    completed_at = NOW()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: MarkConnectSessionCancelled :one
UPDATE connect_sessions
SET status = 'cancelled',
    completed_at = NOW()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: ExpireConnectSession :exec
-- Lazy expiry: called from the read path when expires_at < NOW().
-- No background sweeper required for Sprint 3.
UPDATE connect_sessions
SET status = 'expired'
WHERE id = $1 AND status = 'pending' AND expires_at < NOW();
