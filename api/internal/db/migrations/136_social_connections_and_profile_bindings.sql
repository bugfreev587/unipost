-- +goose Up
-- unipost:safety reversible

-- Expand-only schema. Historical classification and authority promotion are
-- performed later by the explicit, backup-gated social-connections-cutover
-- command after every legacy process has drained.
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

CREATE UNIQUE INDEX social_connections_canonical_identity_unique_idx
  ON social_connections (workspace_id, platform, provider_identity)
  WHERE provider_identity IS NOT NULL
    AND status <> 'migration_conflict';

CREATE INDEX social_connections_workspace_status_idx
  ON social_connections (workspace_id, status);

CREATE INDEX social_connections_refresh_idx
  ON social_connections (token_expires_at)
  WHERE status = 'active' AND token_expires_at IS NOT NULL;

-- Evidence deliberately has no foreign keys. It survives source-row cleanup
-- until targeted reconnect or an operator explicitly resolves the conflict.
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

CREATE TABLE social_connection_rollout_state (
  id                         TEXT PRIMARY KEY CHECK (id = 'global'),
  phase                      TEXT NOT NULL CHECK (
    phase IN ('expand', 'draining', 'cutting_over', 'cutover')
  ),
  cutover_application_sha    TEXT,
  cutover_environment_id     TEXT,
  cutover_completed_at       TIMESTAMPTZ,
  cutover_backend_pid        INTEGER,
  last_legacy_write_at       TIMESTAMPTZ,
  updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO social_connection_rollout_state (id, phase)
VALUES ('global', 'expand');

CREATE TABLE social_connection_cutover_runs (
  id               TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  application_sha  TEXT NOT NULL,
  environment_id   TEXT NOT NULL,
  phase_before     TEXT NOT NULL,
  status           TEXT NOT NULL CHECK (status IN ('started', 'succeeded', 'failed')),
  report           JSONB NOT NULL DEFAULT '{}'::JSONB,
  started_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  completed_at     TIMESTAMPTZ
);

ALTER TABLE social_accounts
  ADD COLUMN connection_id TEXT REFERENCES social_connections(id),
  ADD COLUMN binding_version BIGINT NOT NULL DEFAULT 1,
  ADD COLUMN binding_status TEXT NOT NULL DEFAULT 'active'
    CHECK (binding_status IN ('active', 'unbound'));

-- Persist the user-selected recovery target across every credential flow.
-- These rows are short-lived; the FK prevents a deleted binding ID from being
-- reused or accidentally redirected to a different public account.
ALTER TABLE oauth_states
  ADD COLUMN reconnect_account_id TEXT REFERENCES social_accounts(id) ON DELETE SET NULL;
ALTER TABLE connect_sessions
  ADD COLUMN reconnect_account_id TEXT REFERENCES social_accounts(id) ON DELETE SET NULL;
ALTER TABLE pending_connections
  ADD COLUMN reconnect_account_id TEXT REFERENCES social_accounts(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX social_accounts_profile_connection_unique_idx
  ON social_accounts (profile_id, connection_id)
  WHERE connection_id IS NOT NULL;

CREATE INDEX social_accounts_connection_status_idx
  ON social_accounts (connection_id, binding_status)
  WHERE connection_id IS NOT NULL;

-- During Expand, old binaries may continue writing binding-local credentials.
-- This trigger mirrors such writes to an existing connection without rejecting
-- legacy rows. Cutover changes the rollout phase and makes null authority fail
-- closed. Workspace and platform equality are enforced in every phase.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION enforce_social_connection_authority_write_gate()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  rollout_phase TEXT;
  profile_workspace_id TEXT;
  authority social_connections%ROWTYPE;
  cutover_backend_pid INTEGER;
  cutover_internal BOOLEAN;
BEGIN
  SELECT state.phase, profile.workspace_id, state.cutover_backend_pid
  INTO STRICT rollout_phase, profile_workspace_id, cutover_backend_pid
  FROM social_connection_rollout_state state
  JOIN profiles profile ON profile.id = NEW.profile_id
  WHERE state.id = 'global';

  cutover_internal := rollout_phase = 'cutting_over'
    AND cutover_backend_pid = pg_backend_pid();

  IF NEW.connection_id IS NULL THEN
    IF rollout_phase IN ('cutting_over', 'cutover') AND NOT cutover_internal THEN
      RAISE EXCEPTION 'cutover social account bindings require a connection'
        USING ERRCODE = '23514';
    END IF;

    UPDATE social_connection_rollout_state
    SET last_legacy_write_at = NOW(), updated_at = NOW()
    WHERE id = 'global';
    RETURN NEW;
  END IF;

  SELECT * INTO authority
  FROM social_connections
  WHERE id = NEW.connection_id;

  IF authority.id IS NULL
     OR authority.workspace_id IS DISTINCT FROM profile_workspace_id
     OR authority.platform IS DISTINCT FROM NEW.platform THEN
    RAISE EXCEPTION 'social account connection authority scope mismatch'
      USING ERRCODE = '23514';
  END IF;

  IF TG_OP = 'UPDATE'
     AND OLD.connection_id IS NOT NULL
     AND OLD.connection_id IS DISTINCT FROM NEW.connection_id THEN
    RAISE EXCEPTION 'classified social account bindings cannot change their social connection'
      USING ERRCODE = '23514';
  END IF;

  IF rollout_phase = 'expand' AND TG_OP = 'UPDATE' THEN
    UPDATE social_connections
    SET access_token = CASE
          WHEN NEW.access_token IS DISTINCT FROM OLD.access_token THEN NEW.access_token
          ELSE access_token
        END,
        refresh_token = CASE
          WHEN NEW.refresh_token IS DISTINCT FROM OLD.refresh_token THEN NEW.refresh_token
          ELSE refresh_token
        END,
        token_expires_at = CASE
          WHEN NEW.token_expires_at IS DISTINCT FROM OLD.token_expires_at THEN NEW.token_expires_at
          ELSE token_expires_at
        END,
        account_name = CASE
          WHEN NEW.account_name IS DISTINCT FROM OLD.account_name THEN NEW.account_name
          ELSE account_name
        END,
        account_avatar_url = CASE
          WHEN NEW.account_avatar_url IS DISTINCT FROM OLD.account_avatar_url THEN NEW.account_avatar_url
          ELSE account_avatar_url
        END,
        metadata = CASE
          WHEN NEW.metadata IS DISTINCT FROM OLD.metadata THEN NEW.metadata
          ELSE metadata
        END,
        scope = CASE
          WHEN NEW.scope IS DISTINCT FROM OLD.scope THEN NEW.scope
          ELSE scope
        END,
        external_user_email = CASE
          WHEN NEW.external_user_email IS DISTINCT FROM OLD.external_user_email THEN NEW.external_user_email
          ELSE external_user_email
        END,
        last_refreshed_at = CASE
          WHEN NEW.last_refreshed_at IS DISTINCT FROM OLD.last_refreshed_at THEN NEW.last_refreshed_at
          ELSE last_refreshed_at
        END,
        x_app_mode = CASE
          WHEN NEW.x_app_mode IS DISTINCT FROM OLD.x_app_mode THEN NEW.x_app_mode
          ELSE x_app_mode
        END,
        status = CASE
          WHEN COALESCE(NEW.binding_status, 'active') <> 'unbound'
               AND NEW.status IS DISTINCT FROM OLD.status THEN NEW.status
          ELSE status
        END,
        disconnected_at = CASE
          WHEN COALESCE(NEW.binding_status, 'active') <> 'unbound'
               AND NEW.disconnected_at IS DISTINCT FROM OLD.disconnected_at
            THEN NEW.disconnected_at
          ELSE disconnected_at
        END,
        updated_at = NOW()
    WHERE id = NEW.connection_id;

    UPDATE social_connection_rollout_state
    SET last_legacy_write_at = NOW(), updated_at = NOW()
    WHERE id = 'global';
    RETURN NEW;
  END IF;

  -- After cutover the connection is authoritative. Projection writes are only
  -- accepted when they already agree with the authority row.
  IF rollout_phase IN ('cutting_over', 'cutover') AND NOT cutover_internal AND (
       NEW.access_token IS DISTINCT FROM authority.access_token
       OR NEW.refresh_token IS DISTINCT FROM authority.refresh_token
       OR NEW.token_expires_at IS DISTINCT FROM authority.token_expires_at
       OR NEW.connection_type IS DISTINCT FROM authority.connection_type
       OR NEW.external_user_id IS DISTINCT FROM authority.external_user_id
       OR (
         COALESCE(NEW.binding_status, 'active') <> 'unbound'
         AND (
           NEW.status IS DISTINCT FROM authority.status
           OR NEW.disconnected_at IS DISTINCT FROM authority.disconnected_at
         )
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

ALTER TABLE pending_connections DROP COLUMN IF EXISTS reconnect_account_id;
ALTER TABLE connect_sessions DROP COLUMN IF EXISTS reconnect_account_id;
ALTER TABLE oauth_states DROP COLUMN IF EXISTS reconnect_account_id;

DROP TRIGGER IF EXISTS social_accounts_connection_authority_write_gate ON social_accounts;
DROP FUNCTION IF EXISTS enforce_social_connection_authority_write_gate();
DROP INDEX IF EXISTS social_accounts_connection_status_idx;
DROP INDEX IF EXISTS social_accounts_profile_connection_unique_idx;

ALTER TABLE social_accounts
  DROP COLUMN IF EXISTS binding_status,
  DROP COLUMN IF EXISTS binding_version,
  DROP COLUMN IF EXISTS connection_id;

DROP TABLE IF EXISTS social_connection_rollout_state;
DROP TABLE IF EXISTS social_connection_cutover_runs;
DROP INDEX IF EXISTS social_connection_migration_audit_identity_idx;
DROP TABLE IF EXISTS social_connection_migration_audit;
DROP TABLE IF EXISTS social_connection_migration_conflicts;
DROP TABLE IF EXISTS social_connections;
