-- +goose Up

-- pending_connections holds the intermediate state between the OAuth
-- callback and the user finalizing their selection. It exists for
-- multi-row OAuth flows — specifically Facebook, where a single
-- consent returns a list of Pages the user can pick from — so the
-- callback can stash the LL User Token + page list, redirect the
-- browser to the Dashboard picker, and let the finalize endpoint
-- create N social_accounts rows from the user's selection.
--
-- 10-minute TTL matches oauth_states so a user who closes the tab
-- doesn't leak tokens indefinitely. Cleanup is lazy (callers check
-- expires_at before reading).
CREATE TABLE pending_connections (
  id                        TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id              TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  profile_id                TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
  platform                  TEXT NOT NULL,
  meta_user_id              TEXT NOT NULL,
  user_token_encrypted      TEXT NOT NULL,
  user_token_expires_at     TIMESTAMPTZ NOT NULL,
  -- pages_json mirrors what /me/accounts returned: array of
  -- { id, name, category, picture_url, access_token_encrypted,
  --   tasks }. Each Page's access_token lives ENCRYPTED inside
  -- this blob so the finalize endpoint can pick only the selected
  -- ones, decrypt them, and write social_accounts rows.
  pages_json                JSONB NOT NULL,
  expires_at                TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '10 minutes',
  created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pending_connections_workspace_id ON pending_connections (workspace_id);
CREATE INDEX idx_pending_connections_expires_at ON pending_connections (expires_at);

-- meta_user_tokens stores the long-lived Meta User Access Token per
-- (workspace, Meta user) so we can call /me/accounts again later to
-- add more Pages without forcing the user through another full
-- OAuth consent. Page Tokens (stored on social_accounts rows) are
-- permanent; this User Token has a ~60 day lifetime and is what the
-- "Add another Page" button needs.
--
-- Primary key is (workspace_id, meta_user_id) so a workspace with
-- two different Meta users each get their own row, but a single
-- user re-authorizing just updates the existing row via upsert.
CREATE TABLE meta_user_tokens (
  workspace_id              TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  meta_user_id              TEXT NOT NULL,
  long_lived_token_encrypted TEXT NOT NULL,
  expires_at                TIMESTAMPTZ NOT NULL,
  created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (workspace_id, meta_user_id)
);

CREATE INDEX idx_meta_user_tokens_expires_at ON meta_user_tokens (expires_at);

-- +goose Down

DROP TABLE IF EXISTS meta_user_tokens;
DROP TABLE IF EXISTS pending_connections;
