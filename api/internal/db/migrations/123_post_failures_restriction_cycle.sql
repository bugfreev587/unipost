-- +goose Up

ALTER TABLE post_failures
  ADD COLUMN restriction_cycle_id TEXT;

CREATE INDEX post_failures_restriction_cycle_idx
  ON post_failures (restriction_cycle_id, created_at DESC)
  WHERE restriction_cycle_id IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS post_failures_restriction_cycle_idx;

ALTER TABLE post_failures
  DROP COLUMN IF EXISTS restriction_cycle_id;
