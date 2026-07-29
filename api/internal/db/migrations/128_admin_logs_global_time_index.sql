-- +goose NO TRANSACTION
-- +goose Up
-- unipost:safety reversible

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_integration_logs_admin_ts_id
    ON integration_logs (ts DESC, id DESC);

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS idx_integration_logs_admin_ts_id;
