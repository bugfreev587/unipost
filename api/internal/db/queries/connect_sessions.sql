-- name: CreateConnectSession :one
INSERT INTO connect_sessions (
  profile_id, platform, external_user_id, external_user_email,
  return_url, oauth_state, pkce_verifier, expires_at, allow_quickstart_creds,
  x_app_mode, reconnect_account_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id, profile_id, platform, external_user_id, external_user_email,
  return_url, status, completed_social_account_id, oauth_state, pkce_verifier,
  expires_at, created_at, completed_at, allow_quickstart_creds, x_app_mode, reconnect_account_id;

-- name: GetConnectSessionByID :one
SELECT id, profile_id, platform, external_user_id, external_user_email,
  return_url, status, completed_social_account_id, oauth_state, pkce_verifier,
  expires_at, created_at, completed_at, allow_quickstart_creds, x_app_mode, reconnect_account_id
FROM connect_sessions
WHERE id = $1 AND profile_id = $2;

-- name: GetConnectSessionByIDOnly :one
-- API-key callers authenticate at the workspace level, not the profile
-- level. Handlers fetch the session by id here and then verify the
-- session's profile belongs to the caller's workspace.
SELECT id, profile_id, platform, external_user_id, external_user_email,
  return_url, status, completed_social_account_id, oauth_state, pkce_verifier,
  expires_at, created_at, completed_at, allow_quickstart_creds, x_app_mode, reconnect_account_id
FROM connect_sessions
WHERE id = $1;

-- name: GetConnectSessionByOAuthState :one
SELECT id, profile_id, platform, external_user_id, external_user_email,
  return_url, status, completed_social_account_id, oauth_state, pkce_verifier,
  expires_at, created_at, completed_at, allow_quickstart_creds, x_app_mode, reconnect_account_id
FROM connect_sessions
WHERE oauth_state = $1;

-- name: SetConnectSessionXAppModeIfNull :one
UPDATE connect_sessions
SET x_app_mode = $2
WHERE id = $1 AND x_app_mode IS NULL
RETURNING id, profile_id, platform, external_user_id, external_user_email,
  return_url, status, completed_social_account_id, oauth_state, pkce_verifier,
  expires_at, created_at, completed_at, allow_quickstart_creds, x_app_mode, reconnect_account_id;

-- name: MarkConnectSessionCompleted :one
UPDATE connect_sessions
SET status = 'completed',
    completed_social_account_id = $2,
    completed_at = NOW()
WHERE id = $1 AND status = 'pending'
RETURNING id, profile_id, platform, external_user_id, external_user_email,
  return_url, status, completed_social_account_id, oauth_state, pkce_verifier,
  expires_at, created_at, completed_at, allow_quickstart_creds, x_app_mode, reconnect_account_id;

-- name: MarkConnectSessionCancelled :one
UPDATE connect_sessions
SET status = 'cancelled',
    completed_at = NOW()
WHERE id = $1 AND status = 'pending'
RETURNING id, profile_id, platform, external_user_id, external_user_email,
  return_url, status, completed_social_account_id, oauth_state, pkce_verifier,
  expires_at, created_at, completed_at, allow_quickstart_creds, x_app_mode, reconnect_account_id;

-- name: ExpireConnectSession :exec
UPDATE connect_sessions
SET status = 'expired'
WHERE id = $1 AND status = 'pending' AND expires_at < NOW();
