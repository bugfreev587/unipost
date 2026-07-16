-- +goose Up

CREATE TABLE x_inbox_outbound_requests (
  id                     TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  workspace_id           TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  social_account_id      TEXT NOT NULL REFERENCES social_accounts(id) ON DELETE CASCADE,
  inbox_item_id          TEXT NOT NULL REFERENCES inbox_items(id) ON DELETE CASCADE,
  idempotency_key        TEXT NOT NULL,
  payload_hash           TEXT NOT NULL,
  status                 TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'succeeded')),
  response_inbox_item_id TEXT REFERENCES inbox_items(id) ON DELETE SET NULL,
  created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (workspace_id, inbox_item_id, idempotency_key)
);

CREATE INDEX x_inbox_outbound_requests_pending_idx
  ON x_inbox_outbound_requests (created_at)
  WHERE status = 'pending';

-- +goose Down

DROP TABLE IF EXISTS x_inbox_outbound_requests;
