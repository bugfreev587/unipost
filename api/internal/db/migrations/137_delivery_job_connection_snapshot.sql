-- +goose Up
-- unipost:safety reversible
ALTER TABLE post_delivery_jobs
  ADD COLUMN connection_id TEXT REFERENCES social_connections(id) ON DELETE SET NULL,
  ADD COLUMN binding_version BIGINT;

CREATE INDEX post_delivery_jobs_physical_connection_active_idx
  ON post_delivery_jobs ((COALESCE(connection_id, social_account_id)), created_at, id)
  WHERE state IN ('pending', 'running', 'retrying');

-- +goose Down
DROP INDEX IF EXISTS post_delivery_jobs_physical_connection_active_idx;

ALTER TABLE post_delivery_jobs
  DROP COLUMN IF EXISTS binding_version,
  DROP COLUMN IF EXISTS connection_id;
