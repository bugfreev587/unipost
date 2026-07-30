-- +goose Up
-- unipost:safety reversible

-- The Inbox backfill reservation table already contains the authoritative X
-- paid-read financial state machine. Keep its physical name during rolling
-- deploys so old replicas remain compatible, and expose the generalized name
-- through a simple automatically-updatable view for new replicas.
-- The existing reconciliation_deadline remains the single authoritative
-- deadline for both legacy Inbox and new account-read exposures.
ALTER TABLE x_inbox_backfill_exposure_reservations
  ADD COLUMN purpose TEXT NOT NULL DEFAULT 'inbox_backfill'
    CHECK (purpose IN ('inbox_backfill', 'account_profile', 'account_post_history')),
  ADD COLUMN external_user_id TEXT,
  ADD COLUMN safety_policy TEXT NOT NULL DEFAULT 'inbound_daily_cap'
    CHECK (safety_policy IN ('inbound_daily_cap', 'request_limits')),
  ADD COLUMN units_per_resource BIGINT NOT NULL DEFAULT 1 CHECK (units_per_resource > 0),
  ADD COLUMN catalog_version TEXT NOT NULL DEFAULT 'x-credits-2026-07-16-v1',
  ADD COLUMN app_mode TEXT NOT NULL DEFAULT 'unipost_managed_app',
  ADD COLUMN bypass_reason TEXT,
  ADD COLUMN finalized_at TIMESTAMPTZ;

ALTER TABLE x_inbox_backfill_exposure_reservations
  DROP CONSTRAINT IF EXISTS x_inbox_backfill_exposure_reservations_reserved_units_check;
ALTER TABLE x_inbox_backfill_exposure_reservations
  ADD CONSTRAINT x_read_exposures_reserved_units_check CHECK (reserved_units >= 0);

CREATE VIEW x_read_exposures AS
  SELECT * FROM x_inbox_backfill_exposure_reservations;

CREATE INDEX x_read_exposures_workspace_created_idx
  ON x_inbox_backfill_exposure_reservations (workspace_id, created_at DESC, id DESC);

CREATE TABLE x_read_receipts (
  operation_id               TEXT PRIMARY KEY DEFAULT ('xro_' || replace(gen_random_uuid()::TEXT, '-', '')),
  workspace_id               TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  social_account_id          TEXT NOT NULL REFERENCES social_accounts(id) ON DELETE CASCADE,
  external_user_id           TEXT NOT NULL,
  exposure_id                TEXT NOT NULL UNIQUE REFERENCES x_inbox_backfill_exposure_reservations(id) ON DELETE CASCADE,
  endpoint                   TEXT NOT NULL CHECK (endpoint IN ('profile', 'posts')),
  idempotency_key_hash       TEXT NOT NULL,
  request_fingerprint        TEXT NOT NULL,
  encrypted_request          BYTEA NOT NULL,
  requested_resources        INTEGER NOT NULL CHECK (requested_resources > 0),
  operation_key              TEXT NOT NULL,
  status                     TEXT NOT NULL DEFAULT 'executing'
    CHECK (status IN ('executing', 'outcome_unknown', 'settlement_pending', 'succeeded', 'failed')),
  response_json              JSONB,
  failure_class              TEXT,
  execution_owner            TEXT,
  execution_lease_expires_at TIMESTAMPTZ,
  attempt_count              INTEGER NOT NULL DEFAULT 0,
  next_attempt_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  completed_at               TIMESTAMPTZ,
  expires_at                 TIMESTAMPTZ NOT NULL,
  created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (workspace_id, idempotency_key_hash)
);

CREATE INDEX x_read_receipts_recovery_idx
  ON x_read_receipts (next_attempt_at, created_at)
  WHERE status = 'outcome_unknown';

CREATE INDEX x_read_receipts_expiry_idx
  ON x_read_receipts (expires_at);

CREATE INDEX x_read_receipts_workspace_rate_idx
  ON x_read_receipts (workspace_id, created_at DESC);

CREATE INDEX x_read_receipts_account_rate_idx
  ON x_read_receipts (social_account_id, created_at DESC);

-- +goose Down

DROP TABLE IF EXISTS x_read_receipts;
DROP INDEX IF EXISTS x_read_exposures_workspace_created_idx;
DROP VIEW IF EXISTS x_read_exposures;

ALTER TABLE x_inbox_backfill_exposure_reservations
  DROP CONSTRAINT IF EXISTS x_read_exposures_reserved_units_check;
ALTER TABLE x_inbox_backfill_exposure_reservations
  ADD CONSTRAINT x_inbox_backfill_exposure_reservations_reserved_units_check CHECK (reserved_units > 0);

ALTER TABLE x_inbox_backfill_exposure_reservations
  DROP COLUMN IF EXISTS finalized_at,
  DROP COLUMN IF EXISTS bypass_reason,
  DROP COLUMN IF EXISTS catalog_version,
  DROP COLUMN IF EXISTS app_mode,
  DROP COLUMN IF EXISTS safety_policy,
  DROP COLUMN IF EXISTS units_per_resource,
  DROP COLUMN IF EXISTS external_user_id,
  DROP COLUMN IF EXISTS purpose;
