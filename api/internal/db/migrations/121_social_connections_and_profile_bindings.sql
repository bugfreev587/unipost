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
    'incompatible_app_mode',
    'incompatible_credential_state',
    'incompatible_scope',
    'incompatible_routing_metadata'
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

-- Successful promotions also retain an immutable, non-secret audit record.
-- Credential values are represented by SHA-256 digests so operators can prove
-- which duplicate values agreed without copying usable provider credentials.
CREATE TABLE social_connection_migration_audit (
  id                          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  workspace_id                TEXT NOT NULL,
  platform                    TEXT NOT NULL,
  provider_identity           TEXT NOT NULL,
  connection_id               TEXT,
  outcome                     TEXT NOT NULL CHECK (outcome IN ('promoted')),
  selected_source_account_id  TEXT NOT NULL,
  source_account_ids          TEXT[] NOT NULL,
  authority_evidence          JSONB NOT NULL,
  recorded_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX social_connection_migration_audit_identity_idx
  ON social_connection_migration_audit (workspace_id, platform, provider_identity);

ALTER TABLE social_accounts
  ADD COLUMN connection_id TEXT REFERENCES social_connections(id),
  ADD COLUMN binding_version BIGINT NOT NULL DEFAULT 1,
  ADD COLUMN binding_status TEXT NOT NULL DEFAULT 'active'
    CHECK (binding_status IN ('active', 'unbound'));

-- Goose applies this migration transactionally. This lock is stronger than the
-- per-identity advisory lock used by connect flows: it permits reads but blocks
-- every legacy writer until classification, promotion, audit, and attachment
-- finish in the same transaction. No row can appear between inventory and
-- authority cutover without being classified.
LOCK TABLE social_accounts IN SHARE ROW EXCLUSIVE MODE;

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
    sa.access_token,
    COALESCE(sa.refresh_token, '') AS refresh_token,
    sa.token_expires_at,
    TO_JSONB(ARRAY(
      SELECT DISTINCT item
      FROM UNNEST(COALESCE(sa.scope, ARRAY[]::TEXT[])) AS item
      ORDER BY item
    )) AS authority_scope,
    COALESCE(sa.metadata, '{}'::JSONB)
      - 'dismissed_at'
      - 'disconnect_notified_at'
      - 'reconnect_required_at' AS authority_metadata,
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
    COUNT(DISTINCT access_token) AS access_token_count,
    COUNT(DISTINCT refresh_token) AS refresh_token_count,
    COUNT(DISTINCT COALESCE(token_expires_at::TEXT, '')) AS token_expiry_count,
    COUNT(DISTINCT authority_scope) AS scope_count,
    COUNT(DISTINCT authority_metadata) AS routing_metadata_count,
    BOOL_OR(connection_type = 'managed') AS has_managed,
    BOOL_OR(connection_type = 'byo') AS has_byo,
    ARRAY_AGG(account_id ORDER BY account_id) AS account_ids,
    ARRAY_AGG(profile_id ORDER BY account_id) AS profile_ids,
    ARRAY_AGG(DISTINCT external_user_id) FILTER (
      WHERE external_user_id IS NOT NULL
    ) AS external_user_ids,
    ARRAY_AGG(DISTINCT connection_type ORDER BY connection_type) AS connection_types,
    JSONB_AGG(
      JSONB_BUILD_OBJECT(
        'account_id', account_id,
        'access_token_sha256', ENCODE(SHA256(CONVERT_TO(access_token, 'UTF8')), 'hex'),
        'refresh_token_sha256', ENCODE(SHA256(CONVERT_TO(refresh_token, 'UTF8')), 'hex'),
        'token_expires_at', token_expires_at,
        'scope', authority_scope,
        'routing_metadata_sha256', ENCODE(
          SHA256(CONVERT_TO(authority_metadata::TEXT, 'UTF8')),
          'hex'
        )
      ) ORDER BY account_id
    ) AS authority_evidence
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
      WHEN access_token_count > 1 OR refresh_token_count > 1 OR token_expiry_count > 1
        THEN 'incompatible_credential_state'
      WHEN scope_count > 1
        THEN 'incompatible_scope'
      WHEN routing_metadata_count > 1
        THEN 'incompatible_routing_metadata'
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
    'app_mode_count', app_mode_count,
    'access_token_count', access_token_count,
    'refresh_token_count', refresh_token_count,
    'token_expiry_count', token_expiry_count,
    'scope_count', scope_count,
    'routing_metadata_count', routing_metadata_count,
    'authority_evidence', authority_evidence,
    'promotion_blocked', TRUE
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
    COUNT(DISTINCT access_token) AS access_token_count,
    COUNT(DISTINCT COALESCE(refresh_token, '')) AS refresh_token_count,
    COUNT(DISTINCT COALESCE(token_expires_at::TEXT, '')) AS token_expiry_count,
    COUNT(DISTINCT TO_JSONB(ARRAY(
      SELECT DISTINCT item
      FROM UNNEST(COALESCE(scope, ARRAY[]::TEXT[])) AS item
      ORDER BY item
    ))) AS scope_count,
    COUNT(DISTINCT (
      COALESCE(metadata, '{}'::JSONB)
        - 'dismissed_at'
        - 'disconnect_notified_at'
        - 'reconnect_required_at'
    )) AS routing_metadata_count,
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
    AND access_token_count <= 1
    AND refresh_token_count <= 1
    AND token_expiry_count <= 1
    AND scope_count <= 1
    AND routing_metadata_count <= 1
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

-- Record the exact winning public binding and non-secret evidence for every
-- promoted group before attaching bindings to the new authority.
WITH source AS (
  SELECT
    sa.*,
    p.workspace_id,
    CASE
      WHEN sa.platform = 'instagram'
        THEN NULLIF(BTRIM(sa.metadata->>'instagram_webhook_user_id'), '')
      WHEN sa.platform <> 'instagram'
        THEN NULLIF(BTRIM(sa.external_account_id), '')
    END AS provider_identity,
    TO_JSONB(ARRAY(
      SELECT DISTINCT item
      FROM UNNEST(COALESCE(sa.scope, ARRAY[]::TEXT[])) AS item
      ORDER BY item
    )) AS authority_scope,
    COALESCE(sa.metadata, '{}'::JSONB)
      - 'dismissed_at'
      - 'disconnect_notified_at'
      - 'reconnect_required_at' AS authority_metadata
  FROM social_accounts sa
  JOIN profiles p ON p.id = sa.profile_id
), ranked AS (
  SELECT
    source.*,
    sc.id AS promoted_connection_id,
    ROW_NUMBER() OVER (
      PARTITION BY source.workspace_id, source.platform, source.provider_identity
      ORDER BY
        (source.status = 'active' AND source.disconnected_at IS NULL) DESC,
        source.last_refreshed_at DESC NULLS LAST,
        source.connected_at DESC,
        source.id
    ) AS credential_rank
  FROM source
  JOIN social_connections sc
    ON sc.workspace_id = source.workspace_id
   AND sc.platform = source.platform
   AND sc.provider_identity = source.provider_identity
   AND sc.status <> 'migration_conflict'
)
INSERT INTO social_connection_migration_audit (
  workspace_id,
  platform,
  provider_identity,
  connection_id,
  outcome,
  selected_source_account_id,
  source_account_ids,
  authority_evidence
)
SELECT
  workspace_id,
  platform,
  provider_identity,
  promoted_connection_id,
  'promoted',
  MAX(id) FILTER (WHERE credential_rank = 1),
  ARRAY_AGG(id ORDER BY id),
  JSONB_BUILD_OBJECT(
    'authority_rows', JSONB_AGG(
      JSONB_BUILD_OBJECT(
        'account_id', id,
        'access_token_sha256', ENCODE(SHA256(CONVERT_TO(access_token, 'UTF8')), 'hex'),
        'refresh_token_sha256', ENCODE(SHA256(CONVERT_TO(COALESCE(refresh_token, ''), 'UTF8')), 'hex'),
        'token_expires_at', token_expires_at,
        'scope', authority_scope,
        'routing_metadata_sha256', ENCODE(
          SHA256(CONVERT_TO(authority_metadata::TEXT, 'UTF8')),
          'hex'
        ),
        'selected', credential_rank = 1
      ) ORDER BY id
    )
  )
FROM ranked
GROUP BY workspace_id, platform, provider_identity, promoted_connection_id;

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

-- Install the rolling-deploy write gate before this transaction releases its
-- social_accounts lock. Old application versions may still issue legacy
-- binding-local credential updates, but classified bindings must never diverge
-- from the connection authority. Projection status may be disconnected only
-- when the Profile binding itself is explicitly unbound.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION enforce_social_connection_authority_write_gate()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  authority social_connections%ROWTYPE;
BEGIN
  IF TG_OP = 'INSERT' AND NEW.connection_id IS NULL THEN
    RAISE EXCEPTION 'new social account bindings require an authoritative social connection'
      USING ERRCODE = '23514';
  END IF;

  -- Quarantined historical rows deliberately remain on the legacy path.
  IF NEW.connection_id IS NULL THEN
    RETURN NEW;
  END IF;

  IF TG_OP = 'UPDATE' AND OLD.connection_id IS DISTINCT FROM NEW.connection_id THEN
    RAISE EXCEPTION 'classified social account bindings cannot change their social connection'
      USING ERRCODE = '23514';
  END IF;

  SELECT * INTO STRICT authority
  FROM social_connections
  WHERE id = NEW.connection_id;

  IF TG_OP = 'INSERT' AND (
       NEW.platform IS DISTINCT FROM authority.platform
       OR NEW.access_token IS DISTINCT FROM authority.access_token
       OR NEW.refresh_token IS DISTINCT FROM authority.refresh_token
       OR NEW.token_expires_at IS DISTINCT FROM authority.token_expires_at
       OR NEW.account_name IS DISTINCT FROM authority.account_name
       OR NEW.account_avatar_url IS DISTINCT FROM authority.account_avatar_url
       OR (
         COALESCE(NEW.metadata, '{}'::JSONB)
           - 'dismissed_at'
           - 'disconnect_notified_at'
           - 'reconnect_required_at'
         IS DISTINCT FROM
         authority.metadata
           - 'dismissed_at'
           - 'disconnect_notified_at'
           - 'reconnect_required_at'
       )
       OR COALESCE(NEW.scope, ARRAY[]::TEXT[]) IS DISTINCT FROM authority.scope
       OR NEW.connection_type IS DISTINCT FROM authority.connection_type
       OR NEW.external_user_id IS DISTINCT FROM authority.external_user_id
       OR NEW.external_user_email IS DISTINCT FROM authority.external_user_email
       OR NEW.x_app_mode IS DISTINCT FROM authority.x_app_mode
       OR NEW.status IS DISTINCT FROM authority.status
       OR NEW.disconnected_at IS DISTINCT FROM authority.disconnected_at
     ) THEN
    RAISE EXCEPTION 'social account binding authority must match its social connection'
      USING ERRCODE = '23514';
  END IF;

  IF TG_OP = 'UPDATE' AND (
       (NEW.platform IS DISTINCT FROM OLD.platform
        AND NEW.platform IS DISTINCT FROM authority.platform)
       OR (NEW.access_token IS DISTINCT FROM OLD.access_token
        AND NEW.access_token IS DISTINCT FROM authority.access_token)
       OR (NEW.refresh_token IS DISTINCT FROM OLD.refresh_token
        AND NEW.refresh_token IS DISTINCT FROM authority.refresh_token)
       OR (NEW.token_expires_at IS DISTINCT FROM OLD.token_expires_at
        AND NEW.token_expires_at IS DISTINCT FROM authority.token_expires_at)
       OR (NEW.account_name IS DISTINCT FROM OLD.account_name
        AND NEW.account_name IS DISTINCT FROM authority.account_name)
       OR (NEW.account_avatar_url IS DISTINCT FROM OLD.account_avatar_url
        AND NEW.account_avatar_url IS DISTINCT FROM authority.account_avatar_url)
       OR (
         NEW.metadata IS DISTINCT FROM OLD.metadata
         AND COALESCE(NEW.metadata, '{}'::JSONB)
           - 'dismissed_at'
           - 'disconnect_notified_at'
           - 'reconnect_required_at'
         IS DISTINCT FROM
         authority.metadata
           - 'dismissed_at'
           - 'disconnect_notified_at'
           - 'reconnect_required_at'
       )
       OR (NEW.scope IS DISTINCT FROM OLD.scope
        AND COALESCE(NEW.scope, ARRAY[]::TEXT[]) IS DISTINCT FROM authority.scope)
       OR (NEW.connection_type IS DISTINCT FROM OLD.connection_type
        AND NEW.connection_type IS DISTINCT FROM authority.connection_type)
       OR (NEW.external_user_id IS DISTINCT FROM OLD.external_user_id
        AND NEW.external_user_id IS DISTINCT FROM authority.external_user_id)
       OR (NEW.external_user_email IS DISTINCT FROM OLD.external_user_email
        AND NEW.external_user_email IS DISTINCT FROM authority.external_user_email)
       OR (NEW.x_app_mode IS DISTINCT FROM OLD.x_app_mode
        AND NEW.x_app_mode IS DISTINCT FROM authority.x_app_mode)
       OR (
         COALESCE(NEW.binding_status, 'active') <> 'unbound'
         AND NEW.status IS DISTINCT FROM OLD.status
         AND NEW.status IS DISTINCT FROM authority.status
       )
       OR (
         COALESCE(NEW.binding_status, 'active') <> 'unbound'
         AND NEW.disconnected_at IS DISTINCT FROM OLD.disconnected_at
         AND NEW.disconnected_at IS DISTINCT FROM authority.disconnected_at
       )
     ) THEN
    RAISE EXCEPTION 'social account binding authority update conflicts with its social connection'
      USING ERRCODE = '23514';
  END IF;

  RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER social_accounts_connection_authority_write_gate
BEFORE INSERT OR UPDATE ON social_accounts
FOR EACH ROW
EXECUTE FUNCTION enforce_social_connection_authority_write_gate();

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
DROP TRIGGER IF EXISTS social_accounts_connection_authority_write_gate ON social_accounts;
DROP FUNCTION IF EXISTS enforce_social_connection_authority_write_gate();
DROP INDEX IF EXISTS social_connection_migration_audit_identity_idx;
DROP TABLE IF EXISTS social_connection_migration_audit;

ALTER TABLE social_accounts
  DROP COLUMN IF EXISTS binding_status,
  DROP COLUMN IF EXISTS binding_version,
  DROP COLUMN IF EXISTS connection_id;

DROP TABLE social_connection_migration_conflicts;
DROP TABLE social_connections;
