-- +goose Up
-- unipost:safety reversible
-- Reversible schema-only migration with no UPDATE, DELETE, or backfill. It
-- does not change or bypass the existing backup gate for migrations 124/125.
CREATE TABLE terminal_post_first_publish_elections (
  workspace_id TEXT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
  post_id      TEXT NOT NULL,
  elected_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE terminal_post_event_outbox (
  id              TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  sequence_number BIGSERIAL NOT NULL UNIQUE,
  post_id         TEXT NOT NULL REFERENCES social_posts(id) ON DELETE CASCADE,
  workspace_id    TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  parent_status   TEXT NOT NULL,
  parent_version  TEXT NOT NULL,
  event_type      TEXT NOT NULL DEFAULT '',
  payload         JSONB NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (post_id, parent_version)
);

CREATE INDEX terminal_post_event_outbox_post_sequence_idx
  ON terminal_post_event_outbox (post_id, sequence_number);

CREATE TABLE terminal_post_event_deliveries (
  event_id        TEXT NOT NULL REFERENCES terminal_post_event_outbox(id) ON DELETE CASCADE,
  sink            TEXT NOT NULL CHECK (sink IN ('webhook', 'notification', 'loops')),
  status          TEXT NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending', 'processing', 'enqueued')),
  attempts        INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  lease_expires_at TIMESTAMPTZ,
  last_error      TEXT,
  enqueued_at     TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (event_id, sink)
);

CREATE INDEX terminal_post_event_deliveries_pending_idx
  ON terminal_post_event_deliveries (next_attempt_at, event_id, sink)
  WHERE status IN ('pending', 'processing');

CREATE TABLE terminal_post_event_webhook_deliveries (
  event_id            TEXT NOT NULL REFERENCES terminal_post_event_outbox(id) ON DELETE CASCADE,
  webhook_id          TEXT NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
  webhook_delivery_id TEXT REFERENCES webhook_deliveries(id) ON DELETE SET NULL,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (event_id, webhook_id),
  UNIQUE (webhook_delivery_id)
);

-- +goose Down
DROP TABLE IF EXISTS terminal_post_event_webhook_deliveries;
DROP TABLE IF EXISTS terminal_post_event_deliveries;
DROP TABLE IF EXISTS terminal_post_event_outbox;
DROP TABLE IF EXISTS terminal_post_first_publish_elections;
