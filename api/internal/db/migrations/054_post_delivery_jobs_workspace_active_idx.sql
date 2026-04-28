-- +goose Up
-- Partial index supporting the queue-depth admission check added in
-- the rate-limit + queue-admission PRD (April 2026). The depth check
-- runs on the publish hot path:
--
--   SELECT COUNT(*) FROM post_delivery_jobs
--   WHERE workspace_id = $1
--     AND state IN ('pending','running','retrying');
--
-- The existing post_delivery_jobs_workspace_created_idx is on
-- (workspace_id, created_at DESC) and does not cover the state
-- predicate cheaply, so without this partial index the depth check
-- becomes a sequential cost on every create / publish / retry — the
-- exact pressure the PRD aims to relieve.
CREATE INDEX IF NOT EXISTS post_delivery_jobs_workspace_active_idx
  ON post_delivery_jobs(workspace_id)
  WHERE state IN ('pending', 'running', 'retrying');

-- +goose Down
DROP INDEX IF EXISTS post_delivery_jobs_workspace_active_idx;
