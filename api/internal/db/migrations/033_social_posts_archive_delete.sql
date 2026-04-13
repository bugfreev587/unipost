-- +goose Up
ALTER TABLE social_posts
  ADD COLUMN archived_at TIMESTAMPTZ,
  ADD COLUMN deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_social_posts_workspace_active_created
  ON social_posts (workspace_id, created_at DESC, id DESC)
  WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_social_posts_workspace_active_created;

ALTER TABLE social_posts
  DROP COLUMN IF EXISTS deleted_at,
  DROP COLUMN IF EXISTS archived_at;
