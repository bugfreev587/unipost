-- +goose Up

CREATE TABLE IF NOT EXISTS workspace_active_scheduled_limits (
  id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  limit_count INTEGER NOT NULL CHECK (limit_count > 0),
  reason      TEXT NOT NULL DEFAULT '',
  expires_at  TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (workspace_id)
);

CREATE INDEX IF NOT EXISTS idx_workspace_active_scheduled_limits_active
  ON workspace_active_scheduled_limits (workspace_id, expires_at);

INSERT INTO workspace_active_scheduled_limits (
  workspace_id,
  limit_count,
  reason,
  expires_at
)
SELECT
  w.id,
  250,
  'temporary R2 media retention incident recovery allowance',
  TIMESTAMPTZ '2026-08-01 00:00:00+00'
FROM users u
JOIN workspaces w ON w.user_id = u.id
WHERE lower(u.email) = lower('corcodelgabrielaaa@gmail.com')
ON CONFLICT (workspace_id) DO UPDATE
SET limit_count = EXCLUDED.limit_count,
    reason = EXCLUDED.reason,
    expires_at = EXCLUDED.expires_at,
    updated_at = NOW();

-- +goose Down

DELETE FROM workspace_active_scheduled_limits
WHERE reason = 'temporary R2 media retention incident recovery allowance';

DROP INDEX IF EXISTS idx_workspace_active_scheduled_limits_active;
DROP TABLE IF EXISTS workspace_active_scheduled_limits;
