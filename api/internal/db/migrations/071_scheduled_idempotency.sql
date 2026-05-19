-- +goose Up
DROP INDEX IF EXISTS social_posts_workspace_idempotency_uniq;

CREATE UNIQUE INDEX social_posts_workspace_scheduled_idempotency_uniq
  ON social_posts (workspace_id, idempotency_key)
  WHERE idempotency_key IS NOT NULL
    AND status = 'scheduled';

-- +goose Down
DROP INDEX IF EXISTS social_posts_workspace_scheduled_idempotency_uniq;

CREATE UNIQUE INDEX social_posts_workspace_idempotency_uniq
  ON social_posts (workspace_id, idempotency_key)
  WHERE idempotency_key IS NOT NULL;
