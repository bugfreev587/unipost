-- +goose Up
ALTER TABLE social_accounts ADD COLUMN metadata JSONB;
ALTER TABLE social_posts ADD COLUMN metadata JSONB;

-- +goose Down
ALTER TABLE social_posts DROP COLUMN IF EXISTS metadata;
ALTER TABLE social_accounts DROP COLUMN IF EXISTS metadata;
