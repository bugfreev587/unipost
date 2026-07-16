-- name: CreateSocialAccount :one
INSERT INTO social_accounts (profile_id, platform, access_token, refresh_token, token_expires_at, external_account_id, account_name, account_avatar_url, metadata, scope, x_app_mode)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListSocialAccountsByProfile :many
SELECT * FROM social_accounts
WHERE profile_id = $1
  AND disconnected_at IS NULL
  AND COALESCE(metadata->>'dismissed_at', '') = ''
ORDER BY connected_at DESC;

-- name: ListSocialAccountsByProfileFiltered :many
SELECT * FROM social_accounts
WHERE profile_id = $1
  AND disconnected_at IS NULL
  AND COALESCE(metadata->>'dismissed_at', '') = ''
  AND (sqlc.narg('external_user_id')::TEXT IS NULL OR external_user_id = sqlc.narg('external_user_id')::TEXT)
  AND (sqlc.narg('platform')::TEXT IS NULL OR platform = sqlc.narg('platform')::TEXT)
ORDER BY connected_at DESC;

-- name: ListAllSocialAccountsByProfile :many
SELECT * FROM social_accounts
WHERE profile_id = $1
  AND COALESCE(metadata->>'dismissed_at', '') = ''
ORDER BY connected_at DESC;

-- name: GetSocialAccount :one
SELECT * FROM social_accounts WHERE id = $1;

-- name: GetSocialAccountByIDAndProfile :one
SELECT * FROM social_accounts WHERE id = $1 AND profile_id = $2;

-- name: DisconnectSocialAccount :one
UPDATE social_accounts
SET disconnected_at = NOW(),
    status = 'disconnected',
    access_token = CASE WHEN platform = 'youtube' THEN '' ELSE access_token END,
    refresh_token = CASE WHEN platform = 'youtube' THEN NULL ELSE refresh_token END,
    token_expires_at = CASE WHEN platform = 'youtube' THEN NULL ELSE token_expires_at END,
    external_account_id = CASE WHEN platform = 'youtube' THEN 'disconnected:' || id ELSE external_account_id END,
    account_name = CASE WHEN platform = 'youtube' THEN NULL ELSE account_name END,
    account_avatar_url = CASE WHEN platform = 'youtube' THEN NULL ELSE account_avatar_url END,
    metadata = CASE WHEN platform = 'youtube' THEN '{}'::jsonb ELSE metadata END,
    scope = CASE WHEN platform = 'youtube' THEN ARRAY[]::TEXT[] ELSE scope END,
    last_refreshed_at = CASE WHEN platform = 'youtube' THEN NOW() ELSE last_refreshed_at END
WHERE id = $1 AND profile_id = $2
RETURNING *;

-- name: DismissSocialAccount :execrows
UPDATE social_accounts
SET metadata = COALESCE(metadata, '{}'::jsonb) || jsonb_build_object('dismissed_at', NOW()::TEXT)
WHERE id = $1
  AND profile_id = $2
  AND (disconnected_at IS NOT NULL OR status = 'disconnected')
  AND COALESCE(metadata->>'dismissed_at', '') = '';

-- name: GetExpiringTokens :many
SELECT * FROM social_accounts
WHERE disconnected_at IS NULL
  AND status = 'active'
  AND connection_type <> 'managed'
  AND token_expires_at IS NOT NULL
  AND token_expires_at < NOW() + INTERVAL '24 hours';

-- name: UpdateSocialAccountTokens :exec
UPDATE social_accounts
SET access_token = $2,
    refresh_token = $3,
    token_expires_at = $4,
    metadata = COALESCE(metadata, '{}'::jsonb) - 'dismissed_at' - 'disconnect_notified_at' - 'reconnect_required_at',
    status = 'active',
    disconnected_at = NULL,
    last_refreshed_at = NOW()
WHERE id = $1;

-- name: FindActiveManagedSocialAccountByExternalAccount :one
SELECT *
FROM social_accounts
WHERE profile_id = $1
  AND platform = $2
  AND external_account_id = $3
  AND connection_type = 'managed'
  AND disconnected_at IS NULL
LIMIT 1;

-- name: CountActiveManagedAccountsByWorkspace :one
SELECT COUNT(*)::INTEGER AS total
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE p.workspace_id = $1
  AND sa.connection_type = 'managed'
  AND sa.disconnected_at IS NULL
  AND COALESCE(sa.metadata->>'dismissed_at', '') = '';

-- name: CountManagedUsersByWorkspace :one
SELECT COUNT(DISTINCT sa.external_user_id)::INTEGER AS total
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE p.workspace_id = $1
  AND sa.connection_type = 'managed'
  AND sa.external_user_id IS NOT NULL
  AND COALESCE(sa.metadata->>'dismissed_at', '') = '';

-- name: CountManagedAccountsByWorkspaceAndExternalUser :one
SELECT COUNT(*)::INTEGER AS total
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE p.workspace_id = $1
  AND sa.connection_type = 'managed'
  AND sa.external_user_id = $2
  AND COALESCE(sa.metadata->>'dismissed_at', '') = '';

-- name: CountManagedAccountsByWorkspaceExternalUserAndPlatform :one
SELECT COUNT(*)::INTEGER AS total
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE p.workspace_id = $1
  AND sa.connection_type = 'managed'
  AND sa.external_user_id = $2
  AND sa.platform = $3
  AND sa.disconnected_at IS NULL
  AND COALESCE(sa.metadata->>'dismissed_at', '') = '';

-- name: CreateManagedSocialAccount :one
INSERT INTO social_accounts (
  profile_id, platform, access_token, refresh_token, token_expires_at,
  external_account_id, account_name, account_avatar_url, metadata, scope,
  connection_type, connect_session_id, external_user_id, external_user_email,
  status, last_refreshed_at, x_app_mode
)
VALUES (
  $1, $2, $3, $4, $5,
  $6, $7, $8, $9, $10,
  'managed', $11, $12, $13,
  'active', NOW(), $14
)
RETURNING *;

-- name: RefreshConnectedSocialAccount :one
UPDATE social_accounts
SET access_token        = @access_token,
    refresh_token       = @refresh_token,
    token_expires_at    = @token_expires_at,
    external_account_id = @external_account_id,
    account_name        = @account_name,
    account_avatar_url  = @account_avatar_url,
    metadata            = COALESCE(@metadata::jsonb, '{}'::jsonb) - 'dismissed_at' - 'disconnect_notified_at' - 'reconnect_required_at',
    scope               = @scope,
    connection_type     = @connection_type,
    connect_session_id  = @connect_session_id,
    external_user_id    = @external_user_id,
    external_user_email = @external_user_email,
    x_app_mode          = @x_app_mode,
    status              = 'active',
    disconnected_at     = NULL,
    last_refreshed_at   = NOW()
WHERE id = @id
RETURNING *;

-- name: UpsertManagedSocialAccount :one
INSERT INTO social_accounts (
  profile_id, platform, access_token, refresh_token, token_expires_at,
  external_account_id, account_name, account_avatar_url, metadata, scope,
  connection_type, connect_session_id, external_user_id, external_user_email,
  status, last_refreshed_at, x_app_mode
)
VALUES (
  $1, $2, $3, $4, $5,
  $6, $7, $8, $9, $10,
  'managed', $11, $12, $13,
  'active', NOW(), $14
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
  metadata           = COALESCE(EXCLUDED.metadata, '{}'::jsonb) - 'dismissed_at' - 'disconnect_notified_at' - 'reconnect_required_at',
  scope              = EXCLUDED.scope,
  connect_session_id = EXCLUDED.connect_session_id,
  external_user_email= EXCLUDED.external_user_email,
  x_app_mode         = EXCLUDED.x_app_mode,
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
    metadata           = COALESCE(metadata, '{}'::jsonb) - 'dismissed_at' - 'disconnect_notified_at' - 'reconnect_required_at',
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
    account_name      = COALESCE($5, account_name),
    account_avatar_url= COALESCE($6, account_avatar_url),
    metadata          = COALESCE($7, metadata, '{}'::jsonb) - 'dismissed_at' - 'disconnect_notified_at' - 'reconnect_required_at',
    scope             = COALESCE($8, scope),
    x_app_mode        = COALESCE($9, x_app_mode),
    status            = 'active',
    disconnected_at   = NULL,
    last_refreshed_at = NOW()
WHERE id = $1
RETURNING *;

-- name: MarkSocialAccountReconnectRequired :execrows
UPDATE social_accounts
SET status = 'reconnect_required',
    metadata = COALESCE(metadata, '{}'::jsonb) || jsonb_build_object('reconnect_required_at', NOW()::TEXT)
WHERE id = $1
  AND status = 'active';

-- name: ArmSocialAccountDisconnectNotification :execrows
UPDATE social_accounts
SET metadata = COALESCE(metadata, '{}'::jsonb) || jsonb_build_object('disconnect_notified_at', NOW()::TEXT)
WHERE id = $1
  AND COALESCE(metadata->>'disconnect_notified_at', '') = '';

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
WHERE p.workspace_id = $1
  AND sa.disconnected_at IS NULL
  AND COALESCE(sa.metadata->>'dismissed_at', '') = ''
ORDER BY sa.connected_at DESC;

-- name: ListAllSocialAccountsByWorkspaceIncludingDisconnected :many
-- Same shape as ListSocialAccountsByWorkspace but returns disconnected
-- accounts too. The Posts list renders historical results by joining
-- their social_account_id through this map — filtering disconnected
-- accounts out would strip the platform badges from any post whose
-- account was later removed (the user's "platform column went empty"
-- bug).
SELECT sa.* FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE p.workspace_id = $1
ORDER BY sa.connected_at DESC;

-- name: ListSocialAccountsByWorkspaceFiltered :many
-- Workspace-level list with optional profile_id, external_user_id, and platform filters.
SELECT sa.* FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE p.workspace_id = $1
  AND sa.disconnected_at IS NULL
  AND COALESCE(sa.metadata->>'dismissed_at', '') = ''
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

-- name: GetDistinctProfileIDsForAccounts :many
-- Look up the distinct profile_ids across a set of social_account IDs.
-- Used at post-create/claim time to populate social_posts.profile_ids
-- so per-profile views can filter posts via `profile_id = ANY(profile_ids)`.
-- Scoped to a workspace so callers can't pull profile ids from accounts
-- they don't own — defense-in-depth on top of the handler-side ownership
-- check that already gates the parsed account_ids.
SELECT DISTINCT sa.profile_id
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE sa.id = ANY($1::text[])
  AND p.workspace_id = $2;
