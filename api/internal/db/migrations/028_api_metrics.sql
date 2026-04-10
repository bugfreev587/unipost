-- +goose Up
-- Per-request API metrics for the developer-facing API (API key auth only).
-- Kept lean: no request/response bodies, just path + status + duration.
-- The dashboard aggregates on read via GROUP BY + percentile functions.

CREATE TABLE api_metrics (
  id            BIGSERIAL PRIMARY KEY,
  workspace_id  TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  method        TEXT NOT NULL,            -- GET, POST, DELETE, etc.
  path          TEXT NOT NULL,            -- normalized: /v1/social-posts, not /v1/social-posts/abc123
  status_code   INTEGER NOT NULL,
  duration_ms   INTEGER NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for dashboard queries: per-workspace time-range aggregation.
CREATE INDEX idx_api_metrics_workspace_time
  ON api_metrics (workspace_id, created_at DESC);

-- Index for per-endpoint breakdown within a workspace.
CREATE INDEX idx_api_metrics_workspace_path_time
  ON api_metrics (workspace_id, path, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS api_metrics;
