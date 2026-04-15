-- name: CreateSocialAccount :one
INSERT INTO social_accounts (profile_id, platform, access_token, refresh_token, token_expires_at, external_account_id, account_name, account_avatar_url, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListSocialAccountsByProfile :many
SELECT * FROM social_accounts
WHERE profile_id = $1 AND disconnected_at IS NULL
ORDER BY connected_at DESC;

-- name: ListSocialAccountsByProfileFiltered :many
SELECT * FROM social_accounts
WHERE profile_id = $1
  AND disconnected_at IS NULL
  AND (sqlc.narg('external_user_id')::TEXT IS NULL OR external_user_id = sqlc.narg('external_user_id')::TEXT)
  AND (sqlc.narg('platform')::TEXT IS NULL OR platform = sqlc.narg('platform')::TEXT)
ORDER BY connected_at DESC;

-- name: ListAllSocialAccountsByProfile :many
SELECT * FROM social_accounts
WHERE profile_id = $1
ORDER BY connected_at DESC;

-- name: GetSocialAccount :one
SELECT * FROM social_accounts WHERE id = $1;

-- name: GetSocialAccountByIDAndProfile :one
SELECT * FROM social_accounts WHERE id = $1 AND profile_id = $2;

-- name: DisconnectSocialAccount :one
UPDATE social_accounts SET disconnected_at = NOW(), status = 'disconnected'
WHERE id = $1 AND profile_id = $2
RETURNING *;

-- name: GetExpiringTokens :many
SELECT * FROM social_accounts
WHERE disconnected_at IS NULL
  AND token_expires_at IS NOT NULL
  AND token_expires_at < NOW() + INTERVAL '24 hours';

-- name: UpdateSocialAccountTokens :exec
UPDATE social_accounts
SET access_token = $2,
    refresh_token = $3,
    token_expires_at = $4,
    status = 'active',
    disconnected_at = NULL
WHERE id = $1;

-- name: UpsertManagedSocialAccount :one
INSERT INTO social_accounts (
  profile_id, platform, access_token, refresh_token, token_expires_at,
  external_account_id, account_name, account_avatar_url, metadata, scope,
  connection_type, connect_session_id, external_user_id, external_user_email,
  status, last_refreshed_at
)
VALUES (
  $1, $2, $3, $4, $5,
  $6, $7, $8, $9, $10,
  'managed', $11, $12, $13,
  'active', NOW()
)
ON CONFLICT (profile_id, platform, external_user_id)
  WHERE external_user_id IS NOT NULL AND platform <> 'bluesky'
DO UPDATE SET
  access_token       = EXCLUDED.access_token,
  refresh_token      = EXCLUDED.refresh_token,
  token_expires_at   = EXCLUDED.token_expires_at,
  external_account_id= EXCLUDED.external_account_id,
  account_name       = EXCLUDED.account_name,
  account_avatar_url = EXCLUDED.account_avatar_url,
  metadata           = EXCLUDED.metadata,
  scope              = EXCLUDED.scope,
  connect_session_id = EXCLUDED.connect_session_id,
  external_user_email= EXCLUDED.external_user_email,
  status             = 'active',
  disconnected_at    = NULL,
  last_refreshed_at  = NOW()
RETURNING *;

-- name: GetManagedBlueskyAccount :one
SELECT * FROM social_accounts
WHERE profile_id = $1
  AND platform = 'bluesky'
  AND external_account_id = $2
  AND connection_type = 'managed'
  AND disconnected_at IS NULL
LIMIT 1;

-- name: UpdateManagedBlueskyAccount :one
UPDATE social_accounts
SET access_token       = $2,
    account_name       = $3,
    account_avatar_url = $4,
    external_user_id   = $5,
    external_user_email= $6,
    connect_session_id = $7,
    connection_type    = 'managed',
    status             = 'active',
    disconnected_at    = NULL,
    last_refreshed_at  = NOW()
WHERE id = $1 AND connection_type = 'managed'
RETURNING *;

-- name: ReactivateSocialAccount :one
-- Reactivate a disconnected account with fresh tokens. Preserves
-- the original row ID so all FK references (post results, analytics,
-- inbox items) remain intact.
UPDATE social_accounts
SET access_token      = $2,
    refresh_token     = $3,
    token_expires_at  = $4,
    status            = 'active',
    disconnected_at   = NULL,
    last_refreshed_at = NOW()
WHERE id = $1
RETURNING *;

-- name: MarkSocialAccountReconnectRequired :exec
UPDATE social_accounts
SET status = 'reconnect_required'
WHERE id = $1;

-- name: UpdateManagedTokenRefresh :exec
UPDATE social_accounts
SET access_token      = $2,
    refresh_token     = $3,
    token_expires_at  = $4,
    last_refreshed_at = NOW()
WHERE id = $1;

-- name: ListManagedAccountsDueForRefresh :many
SELECT *
FROM social_accounts
WHERE connection_type = 'managed'
  AND status = 'active'
  AND token_expires_at IS NOT NULL
  AND token_expires_at < NOW() + INTERVAL '30 minutes'
  AND platform <> 'bluesky'
ORDER BY token_expires_at ASC
LIMIT 50
FOR UPDATE SKIP LOCKED;

-- name: ListSocialAccountsByWorkspace :many
-- Returns all active accounts across all profiles in a workspace.
-- Used by the API key auth path where the caller has workspace-level access.
SELECT sa.* FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE p.workspace_id = $1 AND sa.disconnected_at IS NULL
ORDER BY sa.connected_at DESC;

-- name: ListSocialAccountsByWorkspaceFiltered :many
-- Workspace-level list with optional profile_id, external_user_id, and platform filters.
SELECT sa.* FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE p.workspace_id = $1
  AND sa.disconnected_at IS NULL
  AND (sqlc.narg('profile_id')::TEXT IS NULL OR sa.profile_id = sqlc.narg('profile_id')::TEXT)
  AND (sqlc.narg('external_user_id')::TEXT IS NULL OR sa.external_user_id = sqlc.narg('external_user_id')::TEXT)
  AND (sqlc.narg('platform')::TEXT IS NULL OR sa.platform = sqlc.narg('platform')::TEXT)
ORDER BY sa.connected_at DESC;

-- name: GetSocialAccountByIDAndWorkspace :one
-- Workspace-level ownership check for a single account.
-- Used by the cross-profile posting path to verify account belongs to workspace.
SELECT sa.* FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE sa.id = $1 AND p.workspace_id = $2;

-- name: FindSocialAccountByExternalID :one
-- Dedup check: find an existing account (active OR disconnected) with the
-- same platform + external_account_id anywhere in the workspace. Used during
-- connect to reactivate disconnected accounts instead of creating duplicates.
SELECT sa.* FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE sa.platform = $1
  AND sa.external_account_id = $2
  AND p.workspace_id = $3
LIMIT 1;
