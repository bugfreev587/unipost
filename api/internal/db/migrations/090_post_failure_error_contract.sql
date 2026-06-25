-- +goose Up
ALTER TABLE post_failures
  ADD COLUMN IF NOT EXISTS error_source TEXT,
  ADD COLUMN IF NOT EXISTS error_temporality TEXT,
  ADD COLUMN IF NOT EXISTS provider_error JSONB;

ALTER TABLE social_post_results
  ADD COLUMN IF NOT EXISTS error_source TEXT,
  ADD COLUMN IF NOT EXISTS error_temporality TEXT,
  ADD COLUMN IF NOT EXISTS provider_error JSONB;

-- +goose Down
ALTER TABLE social_post_results
  DROP COLUMN IF EXISTS provider_error,
  DROP COLUMN IF EXISTS error_temporality,
  DROP COLUMN IF EXISTS error_source;

ALTER TABLE post_failures
  DROP COLUMN IF EXISTS provider_error,
  DROP COLUMN IF EXISTS error_temporality,
  DROP COLUMN IF EXISTS error_source;
