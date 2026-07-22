-- +goose NO TRANSACTION
-- +goose Up

-- Preserve suspect rows as immutable incident evidence. Intentionally omit
-- foreign keys so later source-row or account cleanup cannot erase evidence.
CREATE TABLE IF NOT EXISTS inbox_item_quarantine (
  id                     TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  incident_key           TEXT NOT NULL,
  original_inbox_item_id TEXT NOT NULL,
  source                 TEXT NOT NULL,
  external_id            TEXT NOT NULL,
  social_account_id      TEXT NOT NULL,
  workspace_id           TEXT NOT NULL,
  account_external_id    TEXT NOT NULL,
  original_row           JSONB NOT NULL CHECK (jsonb_typeof(original_row) = 'object'),
  quarantined_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (incident_key, original_inbox_item_id)
);

-- CREATE INDEX CONCURRENTLY can leave an invalid same-name index after an
-- interrupted migration. Remove any partial result before every retry.
DROP INDEX CONCURRENTLY IF EXISTS social_accounts_active_instagram_webhook_user_id_idx;
DROP INDEX CONCURRENTLY IF EXISTS social_accounts_active_platform_external_account_id_idx;

CREATE INDEX CONCURRENTLY IF NOT EXISTS social_accounts_active_instagram_webhook_user_id_idx
  ON social_accounts ((metadata->>'instagram_webhook_user_id'))
  WHERE platform = 'instagram'
    AND status = 'active'
    AND disconnected_at IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS social_accounts_active_platform_external_account_id_idx
  ON social_accounts (platform, external_account_id)
  WHERE status = 'active'
    AND disconnected_at IS NULL;

-- +goose Down

-- Hold an exclusive table lock from the emptiness check through the drop so a
-- concurrent evidence insert cannot race the destructive operation. Dynamic
-- SQL keeps the block safe when a previous Down attempt already removed it.
-- +goose StatementBegin
DO $$
DECLARE
  has_rows BOOLEAN;
BEGIN
  IF to_regclass('public.inbox_item_quarantine') IS NULL THEN
    RETURN;
  END IF;

  EXECUTE 'LOCK TABLE inbox_item_quarantine IN ACCESS EXCLUSIVE MODE';
  EXECUTE 'SELECT EXISTS (SELECT 1 FROM inbox_item_quarantine)' INTO has_rows;

  IF has_rows THEN
    RAISE EXCEPTION 'refusing to drop non-empty inbox_item_quarantine evidence table';
  END IF;

  EXECUTE 'DROP TABLE IF EXISTS inbox_item_quarantine';
END;
$$;
-- +goose StatementEnd

DROP INDEX CONCURRENTLY IF EXISTS social_accounts_active_instagram_webhook_user_id_idx;
DROP INDEX CONCURRENTLY IF EXISTS social_accounts_active_platform_external_account_id_idx;
