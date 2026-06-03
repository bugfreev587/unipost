-- +goose Up
--
-- Short-lived, single-use setup tokens for CLI/agent onboarding.
-- Raw setup tokens are returned once to the dashboard and never stored;
-- token_hash is used for exchange lookup.

CREATE TABLE cli_setup_tokens (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL,
  token_hash TEXT NOT NULL UNIQUE,
  client TEXT NOT NULL,
  key_name TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX cli_setup_tokens_workspace_idx ON cli_setup_tokens (workspace_id, created_at DESC);
CREATE INDEX cli_setup_tokens_expires_idx ON cli_setup_tokens (expires_at) WHERE used_at IS NULL AND revoked_at IS NULL;

-- +goose Down

DROP TABLE IF EXISTS cli_setup_tokens;
