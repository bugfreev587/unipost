-- +goose NO TRANSACTION
-- +goose Up
-- unipost:safety reversible

-- A failed CREATE INDEX CONCURRENTLY can leave an INVALID index with the
-- target name. IF NOT EXISTS would then incorrectly treat that artifact as
-- success, so remove any same-name partial attempt before rebuilding it.
DROP INDEX CONCURRENTLY IF EXISTS idx_integration_logs_admin_ts_id;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_integration_logs_admin_ts_id
    ON integration_logs (ts DESC, id DESC);

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS idx_integration_logs_admin_ts_id;
