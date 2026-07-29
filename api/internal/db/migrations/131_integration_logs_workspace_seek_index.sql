-- +goose NO TRANSACTION
-- +goose Up
-- unipost:safety reversible

-- A failed CREATE INDEX CONCURRENTLY can leave an INVALID index with the
-- target name. IF NOT EXISTS would then incorrectly treat that artifact as
-- success, so remove any same-name partial attempt before rebuilding it.
-- Operational constraint: this migration is a once-per-deployment rebuild.
-- Do not blindly rerun it after an interrupted deployment: first inspect
-- pg_index.indisvalid for the target. Re-running after a successful build
-- intentionally drops the valid index before rebuilding it concurrently.
DROP INDEX CONCURRENTLY IF EXISTS idx_integration_logs_workspace_ts_id;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_integration_logs_workspace_ts_id
    ON integration_logs (workspace_id, ts DESC, id DESC);

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS idx_integration_logs_workspace_ts_id;
