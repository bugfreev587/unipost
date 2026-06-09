-- +goose Up
ALTER TABLE api_metrics
  ADD COLUMN IF NOT EXISTS api_key_id TEXT REFERENCES api_keys(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_api_metrics_workspace_time
  ON api_metrics (workspace_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_api_metrics_workspace_path_time
  ON api_metrics (workspace_id, path, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_api_metrics_time_path
  ON api_metrics (created_at DESC, path, method);

-- +goose Down
DROP INDEX IF EXISTS idx_api_metrics_time_path;
ALTER TABLE api_metrics DROP COLUMN IF EXISTS api_key_id;

