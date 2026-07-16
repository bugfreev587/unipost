-- +goose Up
ALTER TABLE oauth_states
  ADD COLUMN pkce_verifier TEXT;

-- +goose Down
ALTER TABLE oauth_states
  DROP COLUMN IF EXISTS pkce_verifier;
