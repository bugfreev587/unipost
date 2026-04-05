-- +goose Up
ALTER TABLE subscriptions ADD COLUMN trial_used BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE subscriptions DROP COLUMN trial_used;
