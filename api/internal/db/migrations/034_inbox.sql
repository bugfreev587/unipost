-- +goose Up

CREATE TABLE inbox_items (
  id                 TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  social_account_id  TEXT NOT NULL REFERENCES social_accounts(id),
  workspace_id       TEXT NOT NULL,
  source             TEXT NOT NULL,          -- 'ig_comment', 'ig_dm', 'threads_reply'
  external_id        TEXT NOT NULL,          -- platform's message/comment ID
  parent_external_id TEXT,                   -- media ID (comments), conversation ID (DMs), parent post ID (Threads)
  author_name        TEXT,
  author_id          TEXT,
  author_avatar_url  TEXT,
  body               TEXT,
  is_read            BOOLEAN NOT NULL DEFAULT false,
  is_own             BOOLEAN NOT NULL DEFAULT false,  -- true if sent by the connected account (our reply)
  received_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  metadata           JSONB DEFAULT '{}',
  UNIQUE(social_account_id, external_id)
);

CREATE INDEX idx_inbox_items_workspace_received
  ON inbox_items (workspace_id, received_at DESC);

CREATE INDEX idx_inbox_items_unread
  ON inbox_items (workspace_id, is_read, received_at DESC)
  WHERE is_read = false;

CREATE INDEX idx_inbox_items_parent
  ON inbox_items (social_account_id, parent_external_id, received_at ASC);

-- +goose Down
DROP TABLE IF EXISTS inbox_items;
