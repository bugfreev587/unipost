-- name: CreateSocialConnection :one
INSERT INTO social_connections (
  workspace_id, platform, provider_identity, access_token, refresh_token,
  token_expires_at, account_name, account_avatar_url, metadata, scope, status,
  connection_type, external_user_id, external_user_email, last_refreshed_at,
  x_app_mode, connected_at, disconnected_at
)
VALUES (
  @workspace_id, @platform, @provider_identity, @access_token, @refresh_token,
  @token_expires_at, @account_name, @account_avatar_url,
  COALESCE(@metadata::jsonb, '{}'::jsonb), @scope, 'active', @connection_type,
  @external_user_id, @external_user_email, NOW(), @x_app_mode, NOW(), NULL
)
RETURNING id, workspace_id, platform, provider_identity, access_token,
  refresh_token, token_expires_at, account_name, account_avatar_url, metadata,
  scope, status, connection_type, external_user_id, external_user_email,
  last_refreshed_at, x_app_mode, connected_at, disconnected_at, created_at,
  updated_at;

-- name: GetSocialConnectionForUpdate :one
SELECT sc.id, sc.workspace_id, sc.platform, sc.provider_identity,
  sc.access_token, sc.refresh_token, sc.token_expires_at, sc.account_name,
  sc.account_avatar_url, sc.metadata, sc.scope, sc.status,
  sc.connection_type, sc.external_user_id, sc.external_user_email,
  sc.last_refreshed_at, sc.x_app_mode, sc.connected_at, sc.disconnected_at,
  sc.created_at, sc.updated_at
FROM social_connections sc
WHERE sc.id = @id
  AND sc.workspace_id = @workspace_id
FOR UPDATE OF sc;

-- name: FindCanonicalSocialConnectionForUpdate :one
SELECT sc.id, sc.workspace_id, sc.platform, sc.provider_identity,
  sc.access_token, sc.refresh_token, sc.token_expires_at, sc.account_name,
  sc.account_avatar_url, sc.metadata, sc.scope, sc.status,
  sc.connection_type, sc.external_user_id, sc.external_user_email,
  sc.last_refreshed_at, sc.x_app_mode, sc.connected_at, sc.disconnected_at,
  sc.created_at, sc.updated_at
FROM social_connections sc
WHERE sc.workspace_id = @workspace_id
  AND sc.platform = @platform
  AND sc.provider_identity = @provider_identity
  AND sc.status <> 'migration_conflict'
FOR UPDATE OF sc;

-- name: RefreshSocialConnection :one
UPDATE social_connections
SET access_token = @access_token,
    refresh_token = @refresh_token,
    token_expires_at = @token_expires_at,
    account_name = @account_name,
    account_avatar_url = @account_avatar_url,
    metadata = COALESCE(@metadata::jsonb, '{}'::jsonb),
    scope = @scope,
    external_user_email = @external_user_email,
    last_refreshed_at = NOW(),
    x_app_mode = @x_app_mode,
    status = 'active',
    disconnected_at = NULL,
    updated_at = NOW()
WHERE id = @id
  AND workspace_id = @workspace_id
RETURNING id, workspace_id, platform, provider_identity, access_token,
  refresh_token, token_expires_at, account_name, account_avatar_url, metadata,
  scope, status, connection_type, external_user_id, external_user_email,
  last_refreshed_at, x_app_mode, connected_at, disconnected_at, created_at,
  updated_at;

-- name: DisconnectSocialConnection :one
UPDATE social_connections
SET status = 'disconnected',
    disconnected_at = NOW(),
    updated_at = NOW()
WHERE id = @id
  AND workspace_id = @workspace_id
RETURNING id, workspace_id, platform, provider_identity, access_token,
  refresh_token, token_expires_at, account_name, account_avatar_url, metadata,
  scope, status, connection_type, external_user_id, external_user_email,
  last_refreshed_at, x_app_mode, connected_at, disconnected_at, created_at,
  updated_at;

-- name: CreateOrReactivateSocialAccountBinding :one
INSERT INTO social_accounts (
  profile_id, platform, access_token, refresh_token, token_expires_at,
  external_account_id, account_name, account_avatar_url, metadata, scope,
  status, connection_type, connect_session_id, external_user_id,
  external_user_email, last_refreshed_at, x_app_mode, connection_id,
  binding_version, binding_status
)
VALUES (
  @profile_id, @platform, @legacy_access_token, @legacy_refresh_token,
  @token_expires_at, @external_account_id, @account_name,
  @account_avatar_url, COALESCE(@metadata::jsonb, '{}'::jsonb), @scope,
  'active', @connection_type, @connect_session_id, @external_user_id,
  @external_user_email, NOW(), @x_app_mode, @connection_id, 1, 'active'
)
ON CONFLICT (profile_id, connection_id) WHERE connection_id IS NOT NULL
DO UPDATE SET
  access_token = EXCLUDED.access_token,
  refresh_token = EXCLUDED.refresh_token,
  token_expires_at = EXCLUDED.token_expires_at,
  external_account_id = EXCLUDED.external_account_id,
  account_name = EXCLUDED.account_name,
  account_avatar_url = EXCLUDED.account_avatar_url,
  metadata = EXCLUDED.metadata,
  scope = EXCLUDED.scope,
  connection_type = EXCLUDED.connection_type,
  connect_session_id = EXCLUDED.connect_session_id,
  external_user_id = EXCLUDED.external_user_id,
  external_user_email = EXCLUDED.external_user_email,
  last_refreshed_at = NOW(),
  x_app_mode = EXCLUDED.x_app_mode,
  binding_status = 'active',
  binding_version = social_accounts.binding_version + 1,
  disconnected_at = NULL,
  status = 'active'
RETURNING id, profile_id, platform, access_token, refresh_token,
  token_expires_at, external_account_id, account_name, account_avatar_url,
  connected_at, disconnected_at, metadata, scope, status, connection_type,
  connect_session_id, external_user_id, external_user_email, last_refreshed_at,
  x_app_mode, connection_id, binding_version, binding_status;

-- name: UpdateSocialConnectionTokens :exec
UPDATE social_connections
SET access_token = @access_token,
    refresh_token = @refresh_token,
    token_expires_at = @token_expires_at,
    last_refreshed_at = NOW(),
    updated_at = NOW()
WHERE id = @id;

-- name: ListBoundProfileIDsForAccount :many
SELECT DISTINCT target.profile_id
FROM social_accounts source
JOIN profiles source_profile ON source_profile.id = source.profile_id
JOIN social_accounts target
  ON target.connection_id = source.connection_id
 AND target.binding_status = 'active'
JOIN profiles target_profile ON target_profile.id = target.profile_id
LEFT JOIN social_connections sc ON sc.id = source.connection_id
WHERE source.id = @account_id
  AND source_profile.workspace_id = @workspace_id
  AND target_profile.workspace_id = @workspace_id
  AND (
    sqlc.narg('external_user_id')::TEXT IS NULL
    OR COALESCE(sc.external_user_id, target.external_user_id) = sqlc.narg('external_user_id')::TEXT
  )
ORDER BY target.profile_id;

-- name: UnbindSocialAccountBinding :one
UPDATE social_accounts sa
SET binding_status = 'unbound',
    binding_version = binding_version + 1,
    status = 'disconnected',
    disconnected_at = NOW()
FROM profiles p
WHERE sa.id = @id
  AND sa.profile_id = p.id
  AND p.workspace_id = @workspace_id
  AND sa.connection_id = @connection_id
  AND sa.binding_status = 'active'
RETURNING sa.id, sa.profile_id, sa.platform, sa.access_token, sa.refresh_token,
  sa.token_expires_at, sa.external_account_id, sa.account_name,
  sa.account_avatar_url, sa.connected_at, sa.disconnected_at, sa.metadata,
  sa.scope, sa.status, sa.connection_type, sa.connect_session_id,
  sa.external_user_id, sa.external_user_email, sa.last_refreshed_at,
  sa.x_app_mode, sa.connection_id, sa.binding_version, sa.binding_status;

-- name: DisconnectAllSocialAccountBindings :many
UPDATE social_accounts
SET status = 'disconnected',
    disconnected_at = NOW(),
    binding_version = binding_version + 1
WHERE connection_id = @connection_id
  AND (status <> 'disconnected' OR disconnected_at IS NULL)
RETURNING id, profile_id, platform, access_token, refresh_token,
  token_expires_at, external_account_id, account_name, account_avatar_url,
  connected_at, disconnected_at, metadata, scope, status, connection_type,
  connect_session_id, external_user_id, external_user_email, last_refreshed_at,
  x_app_mode, connection_id, binding_version, binding_status;
