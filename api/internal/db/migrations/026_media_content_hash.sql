-- +goose Up
-- Add content_hash column to media table for dedup. SHA-256 hex string
-- computed client-side before upload. The unique index per workspace
-- ensures the same file is only stored once per workspace.

ALTER TABLE media ADD COLUMN content_hash TEXT;

CREATE UNIQUE INDEX idx_media_workspace_hash
  ON media (workspace_id, content_hash)
  WHERE content_hash IS NOT NULL AND status != 'deleted';

-- +goose Down
DROP INDEX IF EXISTS idx_media_workspace_hash;
ALTER TABLE media DROP COLUMN IF EXISTS content_hash;
