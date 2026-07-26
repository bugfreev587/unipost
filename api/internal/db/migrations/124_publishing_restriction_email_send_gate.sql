-- +goose Up

ALTER TABLE platform_publishing_restriction_email_recipients
  ADD COLUMN retryable BOOLEAN NOT NULL DEFAULT TRUE,
  ADD COLUMN attempt_generation INTEGER NOT NULL DEFAULT 1 CHECK (attempt_generation > 0);

-- Migration 122 initially used plan_status for every existing usage row.
-- Active rows have no cleanup deadline and must preserve the established
-- active_post invariant without rewriting any retention deadline.
UPDATE media_post_usages
SET retention_reason = 'active_post'
WHERE cleanup_after_at IS NULL
  AND retention_reason = 'plan_status';

-- +goose Down

ALTER TABLE platform_publishing_restriction_email_recipients
  DROP COLUMN IF EXISTS attempt_generation,
  DROP COLUMN IF EXISTS retryable;
