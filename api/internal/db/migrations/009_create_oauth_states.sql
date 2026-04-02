-- +goose Up
CREATE TABLE oauth_states (
  state        TEXT PRIMARY KEY,
  project_id   TEXT NOT NULL REFERENCES projects(id),
  platform     TEXT NOT NULL,
  redirect_url TEXT,
  expires_at   TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '10 minutes',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_oauth_states_expires_at ON oauth_states(expires_at);

-- +goose Down
DROP TABLE IF EXISTS oauth_states;
