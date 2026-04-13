-- +goose Up

CREATE TABLE IF NOT EXISTS landing_visits (
  id          BIGSERIAL PRIMARY KEY,
  path        TEXT NOT NULL DEFAULT '/',
  source_code TEXT NOT NULL,
  referer     TEXT NOT NULL DEFAULT '',
  session_id  TEXT NOT NULL,
  user_agent  TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_landing_visits_created_at
  ON landing_visits (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_landing_visits_source_created
  ON landing_visits (source_code, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_landing_visits_session_created
  ON landing_visits (session_id, created_at DESC);

-- +goose Down

DROP TABLE IF EXISTS landing_visits;
