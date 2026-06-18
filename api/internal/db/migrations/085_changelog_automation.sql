-- +goose Up

CREATE TABLE changelog_candidates (
  id                 TEXT PRIMARY KEY,
  source_hash        TEXT NOT NULL UNIQUE,
  status             TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','saved','discarded','publishing','published','failed')),
  payload_json       JSONB NOT NULL,
  window_start       TIMESTAMPTZ NOT NULL,
  window_end         TIMESTAMPTZ NOT NULL,
  discord_message_id TEXT,
  action_request_id  TEXT,
  workflow_run_url   TEXT,
  acted_by_admin_id  TEXT,
  acted_at           TIMESTAMPTZ,
  error_message      TEXT,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX changelog_candidates_status_created_idx
  ON changelog_candidates (status, created_at DESC);

CREATE INDEX changelog_candidates_window_idx
  ON changelog_candidates (window_start, window_end);

-- +goose Down

DROP TABLE IF EXISTS changelog_candidates;
