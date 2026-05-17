-- +goose Up

ALTER TABLE landing_visits
  ADD COLUMN IF NOT EXISTS attribution JSONB NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS raw_query TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS landing_session_users (
  session_id     TEXT NOT NULL,
  user_id        TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  first_bound_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (session_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_landing_session_users_session_id
  ON landing_session_users (session_id);

CREATE INDEX IF NOT EXISTS idx_landing_session_users_user_id
  ON landing_session_users (user_id);

-- +goose Down

DROP TABLE IF EXISTS landing_session_users;

ALTER TABLE landing_visits
  DROP COLUMN IF EXISTS raw_query,
  DROP COLUMN IF EXISTS attribution;
