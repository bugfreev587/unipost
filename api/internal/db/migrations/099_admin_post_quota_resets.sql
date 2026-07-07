-- +goose Up

CREATE TABLE IF NOT EXISTS admin_post_quota_resets (
  id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  period       TEXT NOT NULL,
  quota_kind TEXT NOT NULL CHECK (quota_kind IN ('post', 'scheduled')),
  reset_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, workspace_id, period, quota_kind)
);

CREATE INDEX IF NOT EXISTS idx_admin_post_quota_resets_workspace_period
  ON admin_post_quota_resets (workspace_id, period, quota_kind, reset_at DESC);

-- +goose Down

DROP TABLE IF EXISTS admin_post_quota_resets;
