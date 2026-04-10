-- +goose Up
-- Add 'disconnected' to the status check constraint so the
-- DisconnectSocialAccount query can set status = 'disconnected'.

ALTER TABLE social_accounts
  DROP CONSTRAINT social_accounts_status_check,
  ADD CONSTRAINT social_accounts_status_check
    CHECK (status IN ('active', 'reconnect_required', 'disconnected'));

-- +goose Down
ALTER TABLE social_accounts
  DROP CONSTRAINT social_accounts_status_check,
  ADD CONSTRAINT social_accounts_status_check
    CHECK (status IN ('active', 'reconnect_required'));
