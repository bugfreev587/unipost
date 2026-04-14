-- +goose Up

ALTER TABLE inbox_items
  ADD COLUMN thread_key TEXT,
  ADD COLUMN thread_status TEXT NOT NULL DEFAULT 'open',
  ADD COLUMN assigned_to TEXT,
  ADD COLUMN linked_post_id TEXT REFERENCES social_posts(id);

UPDATE inbox_items
SET thread_key = CASE
  WHEN source = 'ig_dm' THEN COALESCE(parent_external_id, author_id, external_id)
  ELSE COALESCE(parent_external_id, external_id)
END
WHERE thread_key IS NULL;

ALTER TABLE inbox_items
  ALTER COLUMN thread_key SET NOT NULL;

ALTER TABLE inbox_items
  ADD CONSTRAINT inbox_items_thread_status_check
  CHECK (thread_status IN ('open', 'assigned', 'resolved'));

UPDATE inbox_items ii
SET linked_post_id = spr.post_id
FROM social_post_results spr
WHERE ii.linked_post_id IS NULL
  AND ii.parent_external_id IS NOT NULL
  AND spr.social_account_id = ii.social_account_id
  AND spr.external_id = ii.parent_external_id;

CREATE INDEX idx_inbox_items_thread
  ON inbox_items (workspace_id, social_account_id, source, thread_key);

-- +goose Down

DROP INDEX IF EXISTS idx_inbox_items_thread;

ALTER TABLE inbox_items
  DROP CONSTRAINT IF EXISTS inbox_items_thread_status_check;

ALTER TABLE inbox_items
  DROP COLUMN IF EXISTS linked_post_id,
  DROP COLUMN IF EXISTS assigned_to,
  DROP COLUMN IF EXISTS thread_status,
  DROP COLUMN IF EXISTS thread_key;
