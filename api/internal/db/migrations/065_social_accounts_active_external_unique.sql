-- +goose Up
-- Enforce one active managed social account per real external account
-- within a profile. BYO / white-label rows remain independent.
DROP INDEX IF EXISTS social_accounts_active_external_unique_idx;
CREATE UNIQUE INDEX IF NOT EXISTS social_accounts_active_external_unique_idx
  ON social_accounts (profile_id, platform, external_account_id)
  WHERE disconnected_at IS NULL AND connection_type = 'managed';

-- +goose Down
DROP INDEX IF EXISTS social_accounts_active_external_unique_idx;
