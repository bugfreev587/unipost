-- +goose Up
-- One-time local CLI tokens for App Review Autopilot recording jobs.

CREATE TABLE review_agent_tokens (
  id            TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  review_job_id TEXT NOT NULL REFERENCES review_jobs(id) ON DELETE CASCADE,
  workspace_id  TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  platform      TEXT NOT NULL,
  token_hash    TEXT NOT NULL UNIQUE,
  expires_at    TIMESTAMPTZ NOT NULL,
  used_at       TIMESTAMPTZ,
  revoked_at    TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT review_agent_tokens_platform_check
    CHECK (platform IN ('tiktok'))
);

CREATE INDEX review_agent_tokens_job_idx
  ON review_agent_tokens (review_job_id);

CREATE INDEX review_agent_tokens_workspace_created_idx
  ON review_agent_tokens (workspace_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS review_agent_tokens;
