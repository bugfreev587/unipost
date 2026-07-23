-- +goose Up
ALTER TABLE x_inbox_delivery_resources
  ADD COLUMN dm_subscription_forbidden_fingerprint TEXT;

-- +goose Down
ALTER TABLE x_inbox_delivery_resources
  DROP COLUMN IF EXISTS dm_subscription_forbidden_fingerprint;
