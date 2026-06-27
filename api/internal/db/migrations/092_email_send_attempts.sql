-- +goose Up

CREATE TABLE email_send_attempts (
  id                      TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  event_key               TEXT NOT NULL,
  recipient_user_id       TEXT REFERENCES users(id) ON DELETE SET NULL,
  recipient_email         TEXT NOT NULL,
  workspace_id            TEXT REFERENCES workspaces(id) ON DELETE SET NULL,
  provider                TEXT NOT NULL,
  provider_template_id    TEXT,
  idempotency_key         TEXT NOT NULL DEFAULT '',
  delivery_class          TEXT NOT NULL,
  status                  TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sent','failed','skipped')),
  subject_snapshot        TEXT,
  data_variables_snapshot JSONB NOT NULL DEFAULT '{}'::JSONB,
  trigger_source          TEXT,
  trigger_reference_id    TEXT,
  attempt_count           INTEGER NOT NULL DEFAULT 1,
  last_error              TEXT,
  attempted_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  sent_at                 TIMESTAMPTZ,
  created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX email_send_attempts_provider_idempotency_idx
  ON email_send_attempts (provider, idempotency_key)
  WHERE idempotency_key <> '';

CREATE INDEX email_send_attempts_event_status_idx
  ON email_send_attempts (event_key, status, attempted_at DESC);

CREATE INDEX email_send_attempts_recipient_idx
  ON email_send_attempts (recipient_email, attempted_at DESC);

CREATE INDEX email_send_attempts_workspace_idx
  ON email_send_attempts (workspace_id, attempted_at DESC)
  WHERE workspace_id IS NOT NULL;

-- +goose Down

DROP TABLE IF EXISTS email_send_attempts;
