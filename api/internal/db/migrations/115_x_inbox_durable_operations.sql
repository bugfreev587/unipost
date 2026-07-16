-- +goose Up

ALTER TABLE x_inbox_outbound_requests
  DROP CONSTRAINT IF EXISTS x_inbox_outbound_requests_status_check;

ALTER TABLE x_inbox_outbound_requests
  ADD COLUMN IF NOT EXISTS encrypted_payload TEXT,
  ADD COLUMN IF NOT EXISTS body_hash TEXT,
  ADD COLUMN IF NOT EXISTS usage_event_id TEXT REFERENCES x_usage_events(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS operation_key TEXT,
  ADD COLUMN IF NOT EXISTS reserved_units BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS remote_external_id TEXT,
  ADD COLUMN IF NOT EXISTS remote_conversation_id TEXT,
  ADD COLUMN IF NOT EXISTS remote_url TEXT,
  ADD COLUMN IF NOT EXISTS send_started_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS remote_outcome_known_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS reconciliation_deadline TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS completion_attempts INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ADD COLUMN IF NOT EXISTS last_error TEXT;

ALTER TABLE x_inbox_outbound_requests
  ADD CONSTRAINT x_inbox_outbound_requests_status_check
  CHECK (status IN (
    'pending',
    'sending',
    'outcome_unknown',
    'remote_succeeded',
    'usage_reversal_pending',
    'completed',
    'needs_reconciliation',
    'succeeded'
  )) NOT VALID;

ALTER TABLE x_inbox_outbound_requests
  VALIDATE CONSTRAINT x_inbox_outbound_requests_status_check;

-- Migration 114 claims did not persist enough state to determine whether X
-- accepted the write. Preserve the no-resend guarantee by routing them to
-- manual reconciliation instead of leaving them permanently pending.
UPDATE x_inbox_outbound_requests
SET status = 'needs_reconciliation',
    last_error = 'Legacy X Inbox write claim requires manual reconciliation',
    updated_at = NOW()
WHERE status = 'pending'
  AND encrypted_payload IS NULL;

CREATE INDEX IF NOT EXISTS x_inbox_outbound_requests_reconcile_idx
  ON x_inbox_outbound_requests (next_attempt_at, created_at)
  WHERE status IN ('sending', 'outcome_unknown', 'remote_succeeded', 'usage_reversal_pending')
     OR (status = 'pending' AND encrypted_payload IS NULL);

CREATE TABLE x_inbox_backfill_confirmation_operations (
  id                    TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  workspace_id          TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  account_ids           JSONB NOT NULL,
  account_fingerprint   TEXT NOT NULL,
  request_snapshot      JSONB NOT NULL,
  estimated_x_credits   BIGINT NOT NULL CHECK (estimated_x_credits >= 0),
  nonce                 TEXT NOT NULL UNIQUE,
  status                TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'running', 'completed', 'failed', 'expired')),
  result                 JSONB,
  last_error             TEXT,
  expires_at             TIMESTAMPTZ NOT NULL,
  started_at             TIMESTAMPTZ,
  execution_owner        TEXT,
  execution_lease_expires_at TIMESTAMPTZ,
  completed_at           TIMESTAMPTZ,
  created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX x_inbox_backfill_confirmation_active_idx
  ON x_inbox_backfill_confirmation_operations (workspace_id, expires_at)
  WHERE status IN ('pending', 'running');

CREATE TABLE x_inbox_backfill_exposure_reservations (
  id                  TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  workspace_id        TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  social_account_id   TEXT NOT NULL REFERENCES social_accounts(id) ON DELETE CASCADE,
  operation_key       TEXT NOT NULL,
  idempotency_key     TEXT NOT NULL,
  requested_resources INTEGER NOT NULL CHECK (requested_resources > 0),
  reserved_units      BIGINT NOT NULL CHECK (reserved_units > 0),
  actual_units        BIGINT,
  period_start        TIMESTAMPTZ NOT NULL,
  period_end          TIMESTAMPTZ NOT NULL,
  utc_date            DATE NOT NULL,
  status              TEXT NOT NULL DEFAULT 'reserved'
    CHECK (status IN (
      'reserved', 'read_started', 'finalize_pending', 'finalized', 'released',
      'release_pending', 'needs_reconciliation'
    )),
  reconciliation_deadline TIMESTAMPTZ,
  reconciliation_attempts INTEGER NOT NULL DEFAULT 0,
  next_attempt_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_error          TEXT,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (workspace_id, idempotency_key)
);

CREATE INDEX x_inbox_backfill_exposure_pending_idx
  ON x_inbox_backfill_exposure_reservations (next_attempt_at, created_at)
  WHERE status IN ('reserved', 'read_started', 'finalize_pending', 'release_pending');

-- +goose Down

DROP TABLE IF EXISTS x_inbox_backfill_exposure_reservations;
DROP TABLE IF EXISTS x_inbox_backfill_confirmation_operations;

DROP INDEX IF EXISTS x_inbox_outbound_requests_reconcile_idx;

ALTER TABLE x_inbox_outbound_requests
  DROP CONSTRAINT IF EXISTS x_inbox_outbound_requests_status_check;

UPDATE x_inbox_outbound_requests
SET status = CASE
  WHEN status IN ('completed', 'succeeded') THEN 'succeeded'
  ELSE 'pending'
END;

ALTER TABLE x_inbox_outbound_requests
  ADD CONSTRAINT x_inbox_outbound_requests_status_check
  CHECK (status IN ('pending', 'succeeded')) NOT VALID;

ALTER TABLE x_inbox_outbound_requests
  VALIDATE CONSTRAINT x_inbox_outbound_requests_status_check;

ALTER TABLE x_inbox_outbound_requests
  DROP COLUMN IF EXISTS encrypted_payload,
  DROP COLUMN IF EXISTS body_hash,
  DROP COLUMN IF EXISTS usage_event_id,
  DROP COLUMN IF EXISTS operation_key,
  DROP COLUMN IF EXISTS reserved_units,
  DROP COLUMN IF EXISTS remote_external_id,
  DROP COLUMN IF EXISTS remote_conversation_id,
  DROP COLUMN IF EXISTS remote_url,
  DROP COLUMN IF EXISTS send_started_at,
  DROP COLUMN IF EXISTS remote_outcome_known_at,
  DROP COLUMN IF EXISTS reconciliation_deadline,
  DROP COLUMN IF EXISTS completion_attempts,
  DROP COLUMN IF EXISTS next_attempt_at,
  DROP COLUMN IF EXISTS last_error;
