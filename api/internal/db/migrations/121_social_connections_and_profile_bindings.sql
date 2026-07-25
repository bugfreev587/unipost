-- +goose Up

-- Physical provider identity and credentials live at Workspace scope. During
-- this rollout social_accounts keeps its legacy credential columns so rows
-- whose historical ownership cannot be proven can continue on the legacy path.
CREATE TABLE social_connections (
  id                    TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  workspace_id          TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  platform              TEXT NOT NULL,
  provider_identity     TEXT,
  access_token          TEXT NOT NULL,
  refresh_token         TEXT,
  token_expires_at      TIMESTAMPTZ,
  account_name          TEXT,
  account_avatar_url    TEXT,
  metadata              JSONB NOT NULL DEFAULT '{}'::JSONB,
  scope                 TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
  status                TEXT NOT NULL CHECK (
    status IN ('active', 'reconnect_required', 'disconnected', 'migration_conflict')
  ),
  connection_type       TEXT NOT NULL CHECK (connection_type IN ('byo', 'managed')),
  external_user_id      TEXT,
  external_user_email   TEXT,
  last_refreshed_at     TIMESTAMPTZ,
  x_app_mode            TEXT,
  connected_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  disconnected_at       TIMESTAMPTZ,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (
    (connection_type = 'managed' AND external_user_id IS NOT NULL)
    OR (connection_type = 'byo' AND external_user_id IS NULL)
  ),
  CHECK (status = 'migration_conflict' OR provider_identity IS NOT NULL)
);

-- Disconnected rows remain canonical and unique so verified reconnect updates
-- the same connection_id. Only explicitly quarantined migration conflicts sit
-- outside normal identity reuse.
CREATE UNIQUE INDEX social_connections_canonical_identity_unique_idx
  ON social_connections (workspace_id, platform, provider_identity)
  WHERE provider_identity IS NOT NULL
    AND status <> 'migration_conflict';

CREATE INDEX social_connections_workspace_status_idx
  ON social_connections (workspace_id, status);

CREATE INDEX social_connections_refresh_idx
  ON social_connections (token_expires_at)
  WHERE status = 'active' AND token_expires_at IS NOT NULL;

-- Evidence deliberately has no foreign keys. It must survive source-row or
-- Workspace cleanup until an operator explicitly resolves the conflict.
CREATE TABLE social_connection_migration_conflicts (
  id                        TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  workspace_id              TEXT NOT NULL,
  platform                  TEXT NOT NULL,
  provider_identity         TEXT,
  reason                    TEXT NOT NULL CHECK (reason IN (
    'missing_provider_identity',
    'mixed_ownership',
    'cross_managed_user',
    'missing_managed_owner',
    'owner_on_byo_connection',
    'duplicate_profile_binding',
    'incompatible_app_mode'
  )),
  source_account_ids        TEXT[] NOT NULL,
  source_profile_ids        TEXT[] NOT NULL,
  source_external_user_ids  TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
  source_connection_types   TEXT[] NOT NULL,
  details                   JSONB NOT NULL DEFAULT '{}'::JSONB,
  detected_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  resolved_at               TIMESTAMPTZ,
  resolution                JSONB
);

CREATE INDEX social_connection_migration_conflicts_open_idx
  ON social_connection_migration_conflicts (workspace_id, platform, detected_at)
  WHERE resolved_at IS NULL;

ALTER TABLE social_accounts
  ADD COLUMN connection_id TEXT REFERENCES social_connections(id),
  ADD COLUMN binding_version BIGINT NOT NULL DEFAULT 1,
  ADD COLUMN binding_status TEXT NOT NULL DEFAULT 'active'
    CHECK (binding_status IN ('active', 'unbound'));

-- Missing canonical identities are recorded per source row. Instagram must use
-- its professional/webhook user ID; its application-domain external_account_id
-- is not a safe substitute. Bluesky and every other current provider use the
-- verified external_account_id (the Bluesky value is its DID).
WITH source AS (
  SELECT
    sa.id AS account_id,
    sa.profile_id,
    p.workspace_id,
    sa.platform,
    sa.connection_type,
    sa.external_user_id,
    CASE
      WHEN sa.platform = 'instagram'
        THEN NULLIF(BTRIM(sa.metadata->>'instagram_webhook_user_id'), '')
      WHEN sa.platform <> 'instagram'
        THEN NULLIF(BTRIM(sa.external_account_id), '')
    END AS provider_identity
  FROM social_accounts sa
  JOIN profiles p ON p.id = sa.profile_id
)
INSERT INTO social_connection_migration_conflicts (
  workspace_id,
  platform,
  provider_identity,
  reason,
  source_account_ids,
  source_profile_ids,
  source_external_user_ids,
  source_connection_types,
  details
)
SELECT
  workspace_id,
  platform,
  NULL,
  'missing_provider_identity',
  ARRAY[account_id],
  ARRAY[profile_id],
  CASE
    WHEN external_user_id IS NULL THEN ARRAY[]::TEXT[]
    ELSE ARRAY[external_user_id]
  END,
  ARRAY[connection_type],
  jsonb_build_object('account_id', account_id)
FROM source
WHERE provider_identity IS NULL;

-- Classify every non-null identity group before creating any connection. The
-- expressions are intentionally explicit because these are security boundaries,
-- not best-effort data cleanup heuristics.
WITH source AS (
  SELECT
    sa.id AS account_id,
    sa.profile_id,
    p.workspace_id,
    sa.platform,
    sa.connection_type,
    sa.external_user_id,
    COALESCE(sa.x_app_mode, '') AS x_app_mode,
    CASE
      WHEN sa.platform = 'instagram'
        THEN NULLIF(BTRIM(sa.metadata->>'instagram_webhook_user_id'), '')
      WHEN sa.platform <> 'instagram'
        THEN NULLIF(BTRIM(sa.external_account_id), '')
    END AS provider_identity
  FROM social_accounts sa
  JOIN profiles p ON p.id = sa.profile_id
), grouped AS (
  SELECT
    workspace_id,
    platform,
    provider_identity,
    COUNT(*) AS account_count,
    COUNT(DISTINCT profile_id) AS profile_count,
    COUNT(DISTINCT connection_type) AS connection_type_count,
    COUNT(DISTINCT external_user_id) FILTER (
      WHERE external_user_id IS NOT NULL
    ) AS managed_owner_count,
    COUNT(*) FILTER (
      WHERE connection_type = 'managed' AND external_user_id IS NULL
    ) AS missing_managed_owner_count,
    COUNT(*) FILTER (
      WHERE connection_type = 'byo' AND external_user_id IS NOT NULL
    ) AS byo_owner_count,
    COUNT(DISTINCT x_app_mode) AS app_mode_count,
    BOOL_OR(connection_type = 'managed') AS has_managed,
    BOOL_OR(connection_type = 'byo') AS has_byo,
    ARRAY_AGG(account_id ORDER BY account_id) AS account_ids,
    ARRAY_AGG(profile_id ORDER BY account_id) AS profile_ids,
    ARRAY_AGG(DISTINCT external_user_id) FILTER (
      WHERE external_user_id IS NOT NULL
    ) AS external_user_ids,
    ARRAY_AGG(DISTINCT connection_type ORDER BY connection_type) AS connection_types
  FROM source
  WHERE provider_identity IS NOT NULL
  GROUP BY workspace_id, platform, provider_identity
), unsafe AS (
  SELECT
    *,
    CASE
      WHEN connection_type_count > 1 OR (has_managed AND has_byo)
        THEN 'mixed_ownership'
      WHEN has_managed AND managed_owner_count > 1
        THEN 'cross_managed_user'
      WHEN has_managed AND missing_managed_owner_count > 0
        THEN 'missing_managed_owner'
      WHEN has_byo AND byo_owner_count > 0
        THEN 'owner_on_byo_connection'
      WHEN account_count <> profile_count
        THEN 'duplicate_profile_binding'
      WHEN app_mode_count > 1
        THEN 'incompatible_app_mode'
    END AS reason
  FROM grouped
)
INSERT INTO social_connection_migration_conflicts (
  workspace_id,
  platform,
  provider_identity,
  reason,
  source_account_ids,
  source_profile_ids,
  source_external_user_ids,
  source_connection_types,
  details
)
SELECT
  workspace_id,
  platform,
  provider_identity,
  reason,
  account_ids,
  profile_ids,
  COALESCE(external_user_ids, ARRAY[]::TEXT[]),
  connection_types,
  jsonb_build_object(
    'account_count', account_count,
    'profile_count', profile_count,
    'managed_owner_count', managed_owner_count,
    'app_mode_count', app_mode_count
  )
FROM unsafe
WHERE reason IS NOT NULL;

-- Create one connection only for groups that passed every ownership, routing,
-- and stable-binding check. Credential precedence is deterministic: active,
-- latest verified refresh, latest connection, then public account ID.
WITH source AS (
  SELECT
    sa.*,
    p.workspace_id,
    CASE
      WHEN sa.platform = 'instagram'
        THEN NULLIF(BTRIM(sa.metadata->>'instagram_webhook_user_id'), '')
      WHEN sa.platform <> 'instagram'
        THEN NULLIF(BTRIM(sa.external_account_id), '')
    END AS provider_identity
  FROM social_accounts sa
  JOIN profiles p ON p.id = sa.profile_id
), grouped AS (
  SELECT
    workspace_id,
    platform,
    provider_identity,
    COUNT(*) AS account_count,
    COUNT(DISTINCT profile_id) AS profile_count,
    COUNT(DISTINCT connection_type) AS connection_type_count,
    COUNT(DISTINCT external_user_id) FILTER (
      WHERE external_user_id IS NOT NULL
    ) AS managed_owner_count,
    COUNT(*) FILTER (
      WHERE connection_type = 'managed' AND external_user_id IS NULL
    ) AS missing_managed_owner_count,
    COUNT(*) FILTER (
      WHERE connection_type = 'byo' AND external_user_id IS NOT NULL
    ) AS byo_owner_count,
    COUNT(DISTINCT COALESCE(x_app_mode, '')) AS app_mode_count,
    BOOL_OR(connection_type = 'managed') AS has_managed,
    BOOL_OR(connection_type = 'byo') AS has_byo
  FROM source
  WHERE provider_identity IS NOT NULL
  GROUP BY workspace_id, platform, provider_identity
), eligible AS (
  SELECT *
  FROM grouped
  WHERE connection_type_count = 1
    AND NOT (has_managed AND has_byo)
    AND (NOT has_managed OR (managed_owner_count = 1 AND missing_managed_owner_count = 0))
    AND (NOT has_byo OR byo_owner_count = 0)
    AND account_count = profile_count
    AND app_mode_count <= 1
), ranked AS (
  SELECT
    source.*,
    ROW_NUMBER() OVER (
      PARTITION BY source.workspace_id, source.platform, source.provider_identity
      ORDER BY
        (source.status = 'active' AND source.disconnected_at IS NULL) DESC,
        source.last_refreshed_at DESC NULLS LAST,
        source.connected_at DESC,
        source.id
    ) AS credential_rank
  FROM source
  JOIN eligible
    ON eligible.workspace_id = source.workspace_id
   AND eligible.platform = source.platform
   AND eligible.provider_identity = source.provider_identity
)
INSERT INTO social_connections (
  workspace_id,
  platform,
  provider_identity,
  access_token,
  refresh_token,
  token_expires_at,
  account_name,
  account_avatar_url,
  metadata,
  scope,
  status,
  connection_type,
  external_user_id,
  external_user_email,
  last_refreshed_at,
  x_app_mode,
  connected_at,
  disconnected_at
)
SELECT
  workspace_id,
  platform,
  provider_identity,
  access_token,
  refresh_token,
  token_expires_at,
  account_name,
  account_avatar_url,
  COALESCE(metadata, '{}'::JSONB),
  COALESCE(scope, ARRAY[]::TEXT[]),
  CASE
    WHEN status IN ('active', 'reconnect_required', 'disconnected') THEN status
    WHEN disconnected_at IS NULL THEN 'active'
    ELSE 'disconnected'
  END,
  connection_type,
  external_user_id,
  external_user_email,
  last_refreshed_at,
  x_app_mode,
  connected_at,
  disconnected_at
FROM ranked
WHERE credential_rank = 1;

-- Only safe groups have a matching connection row, so conflict rows retain a
-- null connection_id and continue to use the old credential/owner columns.
UPDATE social_accounts sa
SET connection_id = sc.id
FROM profiles p, social_connections sc
WHERE p.id = sa.profile_id
  AND sc.workspace_id = p.workspace_id
  AND sc.platform = sa.platform
  AND sc.provider_identity = CASE
    WHEN sa.platform = 'instagram'
      THEN NULLIF(BTRIM(sa.metadata->>'instagram_webhook_user_id'), '')
    WHEN sa.platform <> 'instagram'
      THEN NULLIF(BTRIM(sa.external_account_id), '')
  END;

CREATE UNIQUE INDEX social_accounts_profile_connection_unique_idx
  ON social_accounts (profile_id, connection_id)
  WHERE connection_id IS NOT NULL;

CREATE INDEX social_accounts_connection_status_idx
  ON social_accounts (connection_id, binding_status)
  WHERE connection_id IS NOT NULL;

-- +goose Down

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM social_connection_migration_conflicts) THEN
    RAISE EXCEPTION 'refusing to drop unresolved social connection migration evidence';
  END IF;
END;
$$;
-- +goose StatementEnd

DROP INDEX IF EXISTS social_accounts_connection_status_idx;
DROP INDEX IF EXISTS social_accounts_profile_connection_unique_idx;

ALTER TABLE social_accounts
  DROP COLUMN IF EXISTS binding_status,
  DROP COLUMN IF EXISTS binding_version,
  DROP COLUMN IF EXISTS connection_id;

DROP TABLE social_connection_migration_conflicts;
DROP TABLE social_connections;
