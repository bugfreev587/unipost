-- name: CreateSocialAccount :one
INSERT INTO social_accounts (project_id, platform, access_token, refresh_token, token_expires_at, external_account_id, account_name, account_avatar_url, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListSocialAccountsByProject :many
SELECT * FROM social_accounts
WHERE project_id = $1 AND disconnected_at IS NULL
ORDER BY connected_at DESC;

-- name: GetSocialAccount :one
SELECT * FROM social_accounts WHERE id = $1;

-- name: GetSocialAccountByIDAndProject :one
SELECT * FROM social_accounts WHERE id = $1 AND project_id = $2;

-- name: DisconnectSocialAccount :one
UPDATE social_accounts SET disconnected_at = NOW()
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: GetExpiringTokens :many
SELECT * FROM social_accounts
WHERE disconnected_at IS NULL
  AND token_expires_at IS NOT NULL
  AND token_expires_at < NOW() + INTERVAL '24 hours';

-- name: UpdateSocialAccountTokens :exec
UPDATE social_accounts
SET access_token = $2, refresh_token = $3, token_expires_at = $4
WHERE id = $1;
