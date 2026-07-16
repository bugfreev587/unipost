-- +goose Up
CREATE TABLE x_inbound_event_receipts (
  workspace_id          TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  social_account_id     TEXT NOT NULL REFERENCES social_accounts(id) ON DELETE CASCADE,
  upstream_resource_type TEXT NOT NULL,
  upstream_resource_id  TEXT NOT NULL,
  utc_date              DATE NOT NULL,
  decision              TEXT NOT NULL
    CHECK (decision IN ('accepted', 'suppressed_daily_cap', 'suppressed_monthly_allowance')),
  weighted_units        BIGINT NOT NULL CHECK (weighted_units >= 0),
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (
    workspace_id,
    social_account_id,
    upstream_resource_type,
    upstream_resource_id,
    utc_date
  )
);

CREATE INDEX x_inbound_event_receipts_workspace_date_idx
  ON x_inbound_event_receipts(workspace_id, utc_date, created_at DESC);

CREATE TABLE x_inbound_cap_settings (
  workspace_id          TEXT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
  inbound_daily_limit   BIGINT NOT NULL CHECK (inbound_daily_limit >= 0),
  updated_by            TEXT NOT NULL,
  acknowledged_exposure BOOLEAN NOT NULL DEFAULT FALSE,
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE x_inbound_cap_notifications (
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  utc_date     DATE NOT NULL,
  threshold    SMALLINT NOT NULL CHECK (threshold IN (80, 100)),
  claimed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (workspace_id, utc_date, threshold)
);

-- +goose Down
DROP TABLE IF EXISTS x_inbound_cap_notifications;
DROP TABLE IF EXISTS x_inbound_cap_settings;
DROP TABLE IF EXISTS x_inbound_event_receipts;
