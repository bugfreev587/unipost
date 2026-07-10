-- +goose Up
ALTER TABLE post_delivery_jobs
  ADD COLUMN first_claimed_at TIMESTAMPTZ,
  ADD COLUMN platform_started_at TIMESTAMPTZ;

CREATE INDEX post_delivery_jobs_reserved_idx
  ON post_delivery_jobs (last_attempt_at)
  WHERE state IN ('running', 'retrying') AND platform_started_at IS NULL;

CREATE INDEX post_delivery_jobs_platform_duration_idx
  ON post_delivery_jobs (platform_started_at)
  WHERE state IN ('running', 'retrying', 'succeeded', 'failed', 'dead');

-- +goose Down
DROP INDEX IF EXISTS post_delivery_jobs_platform_duration_idx;
DROP INDEX IF EXISTS post_delivery_jobs_reserved_idx;
ALTER TABLE post_delivery_jobs DROP COLUMN IF EXISTS platform_started_at;
ALTER TABLE post_delivery_jobs DROP COLUMN IF EXISTS first_claimed_at;
