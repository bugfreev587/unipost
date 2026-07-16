-- +goose Up
CREATE TABLE x_workspace_allowances (
  workspace_id         TEXT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
  monthly_allowance    BIGINT NOT NULL CHECK (monthly_allowance >= 0),
  inbound_daily_limit  BIGINT NOT NULL CHECK (inbound_daily_limit >= 0),
  created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE x_usage_events
  ADD COLUMN connection_mode TEXT NOT NULL DEFAULT 'managed'
    CHECK (connection_mode IN ('managed', 'byo'));

ALTER TABLE social_post_results
  ADD COLUMN x_credits_counted BIGINT NOT NULL DEFAULT 0 CHECK (x_credits_counted >= 0),
  ADD COLUMN x_credit_operation TEXT,
  ADD COLUMN x_credit_catalog_version TEXT,
  ADD COLUMN x_credit_billing_mode TEXT
    CHECK (x_credit_billing_mode IS NULL OR x_credit_billing_mode IN ('unipost_managed_app', 'customer_x_app'));

-- +goose Down
ALTER TABLE social_post_results
  DROP COLUMN IF EXISTS x_credit_billing_mode,
  DROP COLUMN IF EXISTS x_credit_catalog_version,
  DROP COLUMN IF EXISTS x_credit_operation,
  DROP COLUMN IF EXISTS x_credits_counted;

ALTER TABLE x_usage_events
  DROP COLUMN IF EXISTS connection_mode;

DROP TABLE IF EXISTS x_workspace_allowances;
