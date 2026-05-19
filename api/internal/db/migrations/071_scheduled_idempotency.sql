-- +goose Up
-- This transactional DROP/CREATE is acceptable at the current table
-- size, but it takes a blocking lock on social_posts. At higher
-- volume, use a no-transaction Goose migration with DROP/CREATE INDEX
-- CONCURRENTLY instead.
DROP INDEX IF EXISTS social_posts_workspace_idempotency_uniq;

CREATE UNIQUE INDEX social_posts_workspace_scheduled_idempotency_uniq
  ON social_posts (workspace_id, idempotency_key)
  WHERE idempotency_key IS NOT NULL
    AND status = 'scheduled';

-- +goose Down
DROP INDEX IF EXISTS social_posts_workspace_scheduled_idempotency_uniq;

-- Best-effort rollback only: if duplicate non-scheduled keys were
-- inserted while the scheduled-only index was active, recreating this
-- broader unique index will fail and the duplicates must be cleaned up
-- manually first.
CREATE UNIQUE INDEX social_posts_workspace_idempotency_uniq
  ON social_posts (workspace_id, idempotency_key)
  WHERE idempotency_key IS NOT NULL;
