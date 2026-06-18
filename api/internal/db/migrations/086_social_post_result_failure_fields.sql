-- +goose Up
ALTER TABLE social_post_results
  ADD COLUMN IF NOT EXISTS error_code TEXT,
  ADD COLUMN IF NOT EXISTS failure_stage TEXT,
  ADD COLUMN IF NOT EXISTS platform_error_code TEXT,
  ADD COLUMN IF NOT EXISTS is_retriable BOOLEAN,
  ADD COLUMN IF NOT EXISTS next_action TEXT;

-- +goose Down
ALTER TABLE social_post_results
  DROP COLUMN IF EXISTS next_action,
  DROP COLUMN IF EXISTS is_retriable,
  DROP COLUMN IF EXISTS platform_error_code,
  DROP COLUMN IF EXISTS failure_stage,
  DROP COLUMN IF EXISTS error_code;
