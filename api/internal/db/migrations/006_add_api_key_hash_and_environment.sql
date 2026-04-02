-- +goose Up
ALTER TABLE api_keys ADD COLUMN key_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE api_keys ADD COLUMN environment TEXT NOT NULL DEFAULT 'production';
ALTER TABLE api_keys ALTER COLUMN key_hash DROP DEFAULT;

CREATE UNIQUE INDEX idx_api_keys_key_hash ON api_keys(key_hash);

-- +goose Down
DROP INDEX IF EXISTS idx_api_keys_key_hash;
ALTER TABLE api_keys DROP COLUMN IF EXISTS environment;
ALTER TABLE api_keys DROP COLUMN IF EXISTS key_hash;
