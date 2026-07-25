-- +goose Up
ALTER TABLE post_delivery_jobs
  ADD COLUMN connection_id TEXT REFERENCES social_connections(id) ON DELETE SET NULL,
  ADD COLUMN binding_version BIGINT;

-- Preserve the physical destination and binding generation selected at
-- admission. Quarantined legacy bindings intentionally keep both values NULL
-- and continue through the legacy social_account_id fallback.
UPDATE post_delivery_jobs j
SET connection_id = sa.connection_id,
    binding_version = sa.binding_version
FROM social_accounts sa
WHERE sa.id = j.social_account_id
  AND sa.connection_id IS NOT NULL;

CREATE INDEX post_delivery_jobs_physical_connection_active_idx
  ON post_delivery_jobs ((COALESCE(connection_id, social_account_id)), created_at, id)
  WHERE state IN ('pending', 'running', 'retrying');

-- +goose Down
DROP INDEX IF EXISTS post_delivery_jobs_physical_connection_active_idx;

ALTER TABLE post_delivery_jobs
  DROP COLUMN IF EXISTS binding_version,
  DROP COLUMN IF EXISTS connection_id;
