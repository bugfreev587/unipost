-- +goose Up
ALTER TABLE platform_credentials
  ADD COLUMN webhook_route_key TEXT;

CREATE INDEX platform_credentials_webhook_route_key_idx
  ON platform_credentials(webhook_route_key)
  WHERE platform = 'twitter' AND webhook_route_key IS NOT NULL;

-- Route keys depend on plaintext consumer secrets, while this table stores
-- ciphertext. Application startup lazily backfills existing workspace X
-- credentials after decryption; new writes persist the key atomically.

-- Collapse any historical cross-date duplicates before changing the identity
-- constraint. Daily metrics remain in their original UTC-date aggregate rows.
WITH ranked AS (
  SELECT
    ctid,
    ROW_NUMBER() OVER (
      PARTITION BY workspace_id, social_account_id, upstream_resource_type, upstream_resource_id
      ORDER BY created_at, utc_date, ctid
    ) AS duplicate_rank
  FROM x_inbound_event_receipts
)
DELETE FROM x_inbound_event_receipts
WHERE ctid IN (SELECT ctid FROM ranked WHERE duplicate_rank > 1);

ALTER TABLE x_inbound_event_receipts
  DROP CONSTRAINT x_inbound_event_receipts_pkey,
  ADD CONSTRAINT x_inbound_event_receipts_pkey
    PRIMARY KEY (workspace_id, social_account_id, upstream_resource_type, upstream_resource_id);

-- +goose Down
ALTER TABLE x_inbound_event_receipts
  DROP CONSTRAINT x_inbound_event_receipts_pkey,
  ADD CONSTRAINT x_inbound_event_receipts_pkey
    PRIMARY KEY (workspace_id, social_account_id, upstream_resource_type, upstream_resource_id, utc_date);

DROP INDEX IF EXISTS platform_credentials_webhook_route_key_idx;

ALTER TABLE platform_credentials
  DROP COLUMN IF EXISTS webhook_route_key;
