-- +goose Up
-- Track consecutive analytics fetch failures so the dashboard can
-- show "Analytics temporarily unavailable" instead of stale data
-- when a platform keeps returning errors (deleted post, lost perms).
-- The worker increments on each failure and resets on success.

ALTER TABLE post_analytics
  ADD COLUMN consecutive_failures INT NOT NULL DEFAULT 0,
  ADD COLUMN last_failure_reason  TEXT;

-- +goose Down
ALTER TABLE post_analytics
  DROP COLUMN IF EXISTS last_failure_reason,
  DROP COLUMN IF EXISTS consecutive_failures;
