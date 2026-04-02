-- +goose Up
CREATE TABLE social_accounts (
  id                    TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id            TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  platform              TEXT NOT NULL,
  access_token          TEXT NOT NULL,
  refresh_token         TEXT,
  token_expires_at      TIMESTAMPTZ,
  external_account_id   TEXT NOT NULL,
  account_name          TEXT,
  account_avatar_url    TEXT,
  connected_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  disconnected_at       TIMESTAMPTZ
);

CREATE INDEX idx_social_accounts_project_id ON social_accounts(project_id);

-- +goose Down
DROP TABLE IF EXISTS social_accounts;
