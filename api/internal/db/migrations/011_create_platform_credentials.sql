-- +goose Up
CREATE TABLE platform_credentials (
  id            TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id    TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  platform      TEXT NOT NULL,
  client_id     TEXT NOT NULL,
  client_secret TEXT NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(project_id, platform)
);

-- +goose Down
DROP TABLE IF EXISTS platform_credentials;
