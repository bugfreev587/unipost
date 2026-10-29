-- +goose Up
-- unipost:safety reversible

CREATE TABLE platform_operational_controls (
  id                  TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  platform            TEXT NOT NULL UNIQUE,
  state               TEXT NOT NULL DEFAULT 'active'
                      CHECK (state IN ('active', 'publishing_disabled', 'fully_disabled')),
  reason_code         TEXT NOT NULL DEFAULT 'operational',
  user_message        TEXT NOT NULL DEFAULT 'This platform is temporarily unavailable.',
  version             BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  state_changed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by_user_id  TEXT REFERENCES users(id) ON DELETE SET NULL,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE platform_operational_control_events (
  id                  TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  control_id          TEXT NOT NULL REFERENCES platform_operational_controls(id) ON DELETE RESTRICT,
  platform            TEXT NOT NULL,
  event_type          TEXT NOT NULL CHECK (event_type IN ('state_changed')),
  actor_user_id       TEXT REFERENCES users(id) ON DELETE SET NULL,
  expected_version    BIGINT NOT NULL,
  resulting_version   BIGINT NOT NULL,
  before_state        JSONB NOT NULL,
  after_state         JSONB NOT NULL,
  request_id          TEXT,
  actor_ip            TEXT,
  actor_user_agent    TEXT,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX platform_operational_control_events_lookup_idx
  ON platform_operational_control_events (control_id, created_at DESC);

INSERT INTO platform_operational_controls (platform, state, reason_code, user_message)
VALUES
  ('twitter', 'active', 'operational', 'X is temporarily unavailable.'),
  ('linkedin', 'active', 'operational', 'LinkedIn is temporarily unavailable.'),
  ('bluesky', 'active', 'operational', 'Bluesky is temporarily unavailable.'),
  ('youtube', 'active', 'operational', 'YouTube is temporarily unavailable.'),
  ('tiktok', 'active', 'operational', 'TikTok is temporarily unavailable.'),
  ('instagram', 'active', 'operational', 'Instagram is temporarily unavailable.'),
  ('threads', 'active', 'operational', 'Threads is temporarily unavailable.'),
  ('facebook', 'active', 'operational', 'Facebook is temporarily unavailable.'),
  ('pinterest', 'active', 'operational', 'Pinterest is temporarily unavailable.')
ON CONFLICT (platform) DO NOTHING;

-- +goose Down

DROP TABLE IF EXISTS platform_operational_control_events;
DROP TABLE IF EXISTS platform_operational_controls;
