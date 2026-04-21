-- +goose Up
CREATE TABLE post_delivery_jobs (
  id                    TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  post_id               TEXT NOT NULL REFERENCES social_posts(id) ON DELETE CASCADE,
  social_post_result_id TEXT NOT NULL REFERENCES social_post_results(id) ON DELETE CASCADE,
  workspace_id          TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  social_account_id     TEXT NOT NULL REFERENCES social_accounts(id) ON DELETE CASCADE,
  platform              TEXT NOT NULL,
  post_input_index      INTEGER NOT NULL,
  kind                  TEXT NOT NULL,
  state                 TEXT NOT NULL,
  attempts              INTEGER NOT NULL DEFAULT 0,
  max_attempts          INTEGER NOT NULL DEFAULT 5,
  failure_stage         TEXT,
  error_code            TEXT,
  platform_error_code   TEXT,
  last_error            TEXT,
  next_run_at           TIMESTAMPTZ,
  last_attempt_at       TIMESTAMPTZ,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  finished_at           TIMESTAMPTZ,
  CHECK (kind IN ('dispatch', 'retry')),
  CHECK (state IN ('pending', 'running', 'retrying', 'succeeded', 'failed', 'dead', 'cancelled'))
);

CREATE INDEX post_delivery_jobs_workspace_created_idx
  ON post_delivery_jobs(workspace_id, created_at DESC);

CREATE INDEX post_delivery_jobs_post_idx
  ON post_delivery_jobs(post_id, created_at DESC);

CREATE INDEX post_delivery_jobs_result_idx
  ON post_delivery_jobs(social_post_result_id, created_at DESC);

CREATE INDEX post_delivery_jobs_claim_dispatch_idx
  ON post_delivery_jobs(kind, state, created_at)
  WHERE kind = 'dispatch' AND state = 'pending';

CREATE INDEX post_delivery_jobs_claim_retry_idx
  ON post_delivery_jobs(kind, state, next_run_at, created_at)
  WHERE kind = 'retry' AND state = 'pending';

CREATE UNIQUE INDEX post_delivery_jobs_one_active_per_result_idx
  ON post_delivery_jobs(social_post_result_id)
  WHERE state IN ('pending', 'running', 'retrying');

-- +goose Down
DROP TABLE IF EXISTS post_delivery_jobs;
