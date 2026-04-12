-- +goose Up
-- Migration 028_api_metrics.sql was skipped on production due to a
-- version-number collision: the onboarding migration was originally
-- 028, then renamed to 029, but goose had already marked version 028
-- as applied (with the old onboarding content). This migration
-- re-creates the api_metrics table idempotently so it self-heals
-- on the next deploy without manual intervention.

CREATE TABLE IF NOT EXISTS api_metrics (
  id            BIGSERIAL PRIMARY KEY,
  workspace_id  TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  method        TEXT NOT NULL,
  path          TEXT NOT NULL,
  status_code   INTEGER NOT NULL,
  duration_ms   INTEGER NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_metrics_workspace_time
  ON api_metrics (workspace_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_api_metrics_workspace_path_time
  ON api_metrics (workspace_id, path, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS api_metrics;
