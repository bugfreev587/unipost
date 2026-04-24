-- +goose Up
ALTER TABLE webhooks ADD COLUMN name TEXT;

UPDATE webhooks
SET name = COALESCE(
  NULLIF(split_part(regexp_replace(url, '^https?://', '', 'i'), '/', 1), ''),
  'Webhook'
)
WHERE name IS NULL;

ALTER TABLE webhooks ALTER COLUMN name SET NOT NULL;
ALTER TABLE webhooks ALTER COLUMN name SET DEFAULT 'Webhook';

-- +goose Down
ALTER TABLE webhooks DROP COLUMN IF EXISTS name;
