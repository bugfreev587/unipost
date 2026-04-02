-- +goose Up
ALTER TABLE social_accounts ADD COLUMN scope TEXT[];

-- +goose Down
ALTER TABLE social_accounts DROP COLUMN IF EXISTS scope;
