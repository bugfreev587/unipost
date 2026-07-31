-- +goose Up
-- unipost:safety reversible

ALTER TABLE inbox_items
  ADD COLUMN connection_id TEXT REFERENCES social_connections(id) ON DELETE SET NULL;

CREATE TABLE inbox_item_supersessions (
  inbox_item_id TEXT PRIMARY KEY REFERENCES inbox_items(id) ON DELETE CASCADE,
  canonical_inbox_item_id TEXT NOT NULL REFERENCES inbox_items(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (inbox_item_id <> canonical_inbox_item_id)
);

CREATE UNIQUE INDEX inbox_items_connection_source_external_unique_idx
  ON inbox_items (connection_id, source, external_id)
  WHERE connection_id IS NOT NULL;

CREATE INDEX inbox_items_connection_received_idx
  ON inbox_items (connection_id, received_at DESC)
  WHERE connection_id IS NOT NULL;

-- inbox_items keeps a representative public binding ID for compatibility with
-- existing API models. If that binding is deleted while another Profile remains
-- bound to the same physical connection, move connection-owned Inbox rows to a
-- stable sibling before the legacy ON DELETE CASCADE foreign key runs.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION rehome_connection_inbox_items_before_binding_delete()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  sibling_account_id TEXT;
BEGIN
  IF OLD.connection_id IS NULL THEN
    RETURN OLD;
  END IF;

  SELECT id INTO sibling_account_id
  FROM social_accounts
  WHERE connection_id = OLD.connection_id
    AND id <> OLD.id
    AND binding_status = 'active'
  ORDER BY profile_id, id
  LIMIT 1;

  IF sibling_account_id IS NOT NULL THEN
    UPDATE x_inbox_outbound_requests
    SET social_account_id = sibling_account_id,
        updated_at = NOW()
    WHERE social_account_id = OLD.id;

    UPDATE x_inbox_backfill_exposure_reservations
    SET social_account_id = sibling_account_id,
        updated_at = NOW()
    WHERE social_account_id = OLD.id;

    DELETE FROM x_inbound_event_receipts old_receipt
    USING x_inbound_event_receipts sibling_receipt
    WHERE old_receipt.social_account_id = OLD.id
      AND sibling_receipt.workspace_id = old_receipt.workspace_id
      AND sibling_receipt.social_account_id = sibling_account_id
      AND sibling_receipt.upstream_resource_type = old_receipt.upstream_resource_type
      AND sibling_receipt.upstream_resource_id = old_receipt.upstream_resource_id
      AND sibling_receipt.utc_date = old_receipt.utc_date;

    UPDATE x_inbound_event_receipts
    SET social_account_id = sibling_account_id
    WHERE social_account_id = OLD.id;

    INSERT INTO x_inbox_delivery_resources (
      social_account_id,
      filtered_stream_rule_id,
      activity_dm_subscription_id,
      delivery_status,
      last_error,
      last_synced_at,
      created_at,
      updated_at
    )
    SELECT
      sibling_account_id,
      filtered_stream_rule_id,
      activity_dm_subscription_id,
      delivery_status,
      last_error,
      last_synced_at,
      created_at,
      NOW()
    FROM x_inbox_delivery_resources
    WHERE social_account_id = OLD.id
    ON CONFLICT (social_account_id) DO UPDATE SET
      filtered_stream_rule_id = COALESCE(
        x_inbox_delivery_resources.filtered_stream_rule_id,
        EXCLUDED.filtered_stream_rule_id
      ),
      activity_dm_subscription_id = COALESCE(
        x_inbox_delivery_resources.activity_dm_subscription_id,
        EXCLUDED.activity_dm_subscription_id
      ),
      last_synced_at = GREATEST(
        x_inbox_delivery_resources.last_synced_at,
        EXCLUDED.last_synced_at
      ),
      updated_at = NOW();

    DELETE FROM x_inbox_delivery_resources
    WHERE social_account_id = OLD.id;

    -- Published result history feeds the physical daily safety ledger. Preserve
    -- it on a surviving sibling so deleting one Profile cannot erase usage.
    UPDATE social_post_results
    SET social_account_id = sibling_account_id
    WHERE social_account_id = OLD.id;

    UPDATE inbox_items
    SET social_account_id = sibling_account_id
    WHERE social_account_id = OLD.id
      AND connection_id = OLD.connection_id;
  END IF;

  RETURN OLD;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER aa_social_accounts_rehome_connection_inbox_items
BEFORE DELETE ON social_accounts
FOR EACH ROW
EXECUTE FUNCTION rehome_connection_inbox_items_before_binding_delete();

-- +goose Down

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM inbox_item_supersessions) THEN
    RAISE EXCEPTION 'refusing destructive rollback with preserved Inbox supersession evidence';
  END IF;
END;
$$;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS aa_social_accounts_rehome_connection_inbox_items ON social_accounts;
DROP FUNCTION IF EXISTS rehome_connection_inbox_items_before_binding_delete();
DROP INDEX IF EXISTS inbox_items_connection_received_idx;
DROP INDEX IF EXISTS inbox_items_connection_source_external_unique_idx;
DROP TABLE IF EXISTS inbox_item_supersessions;
ALTER TABLE inbox_items DROP COLUMN IF EXISTS connection_id;
