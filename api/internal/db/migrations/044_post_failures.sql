-- +goose Up

CREATE TABLE post_failures (
  id                    TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  post_id               TEXT NOT NULL REFERENCES social_posts(id) ON DELETE CASCADE,
  social_post_result_id TEXT REFERENCES social_post_results(id) ON DELETE SET NULL,
  workspace_id          TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  social_account_id     TEXT REFERENCES social_accounts(id) ON DELETE SET NULL,
  platform              TEXT NOT NULL,
  failure_stage         TEXT NOT NULL,
  error_code            TEXT NOT NULL,
  platform_error_code   TEXT,
  message               TEXT NOT NULL,
  raw_error             TEXT,
  is_retriable          BOOLEAN NOT NULL DEFAULT false,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX post_failures_workspace_created_idx
  ON post_failures (workspace_id, created_at DESC);

CREATE INDEX post_failures_post_created_idx
  ON post_failures (post_id, created_at DESC);

CREATE INDEX post_failures_platform_code_idx
  ON post_failures (platform, error_code, created_at DESC);

CREATE INDEX post_failures_account_created_idx
  ON post_failures (social_account_id, created_at DESC);

-- +goose Down

DROP INDEX IF EXISTS post_failures_account_created_idx;
DROP INDEX IF EXISTS post_failures_platform_code_idx;
DROP INDEX IF EXISTS post_failures_post_created_idx;
DROP INDEX IF EXISTS post_failures_workspace_created_idx;
DROP TABLE IF EXISTS post_failures;
