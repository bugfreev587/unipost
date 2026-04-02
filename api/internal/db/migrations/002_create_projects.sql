-- +goose Up
CREATE TABLE projects (
  id          TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id    TEXT NOT NULL REFERENCES users(id),
  name        TEXT NOT NULL,
  mode        TEXT NOT NULL DEFAULT 'quickstart',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_projects_owner_id ON projects(owner_id);

-- +goose Down
DROP TABLE IF EXISTS projects;
