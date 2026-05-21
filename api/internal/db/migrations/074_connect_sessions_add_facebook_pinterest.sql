-- +goose Up
ALTER TABLE connect_sessions
  DROP CONSTRAINT IF EXISTS connect_sessions_platform_check;

ALTER TABLE connect_sessions
  ADD CONSTRAINT connect_sessions_platform_check
    CHECK (platform IN ('twitter', 'linkedin', 'bluesky', 'youtube', 'tiktok', 'instagram', 'threads', 'facebook', 'pinterest'));

-- +goose Down
ALTER TABLE connect_sessions
  DROP CONSTRAINT IF EXISTS connect_sessions_platform_check;

ALTER TABLE connect_sessions
  ADD CONSTRAINT connect_sessions_platform_check
    CHECK (platform IN ('twitter', 'linkedin', 'bluesky', 'youtube', 'tiktok', 'instagram', 'threads'));
