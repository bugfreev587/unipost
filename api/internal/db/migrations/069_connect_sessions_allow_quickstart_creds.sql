-- +goose Up
ALTER TABLE connect_sessions
  ADD COLUMN allow_quickstart_creds BOOLEAN NOT NULL DEFAULT TRUE;

-- +goose Down
ALTER TABLE connect_sessions
  DROP COLUMN IF EXISTS allow_quickstart_creds;
