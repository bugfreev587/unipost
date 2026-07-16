-- +goose Up
CREATE TABLE x_usage_periods (
  workspace_id        TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  period_start        TIMESTAMPTZ NOT NULL,
  period_end          TIMESTAMPTZ NOT NULL,
  weighted_units_used BIGINT NOT NULL DEFAULT 0 CHECK (weighted_units_used >= 0),
  weighted_units_limit BIGINT NOT NULL CHECK (weighted_units_limit >= 0),
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (workspace_id, period_start, period_end),
  CHECK (period_end > period_start)
);

CREATE TABLE x_usage_events (
  id                TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id      TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  social_account_id TEXT REFERENCES social_accounts(id) ON DELETE SET NULL,
  period_start      TIMESTAMPTZ NOT NULL,
  period_end        TIMESTAMPTZ NOT NULL,
  operation_key     TEXT NOT NULL,
  catalog_version   TEXT NOT NULL,
  source            TEXT NOT NULL,
  idempotency_key   TEXT NOT NULL,
  weighted_units    BIGINT NOT NULL CHECK (weighted_units >= 0),
  status            TEXT NOT NULL CHECK (status IN ('provisional', 'finalized', 'reversed')),
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (workspace_id, idempotency_key),
  FOREIGN KEY (workspace_id, period_start, period_end)
    REFERENCES x_usage_periods(workspace_id, period_start, period_end)
    ON DELETE CASCADE
);

CREATE INDEX x_usage_events_reconciliation_idx
  ON x_usage_events(status, created_at)
  WHERE status = 'provisional';

CREATE INDEX x_usage_events_metrics_idx
  ON x_usage_events(workspace_id, source, operation_key, created_at DESC);

CREATE TABLE x_inbound_daily_usage (
  workspace_id         TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  utc_date             DATE NOT NULL,
  weighted_units_used  BIGINT NOT NULL DEFAULT 0 CHECK (weighted_units_used >= 0),
  weighted_units_limit BIGINT NOT NULL CHECK (weighted_units_limit >= 0),
  events_accepted      BIGINT NOT NULL DEFAULT 0 CHECK (events_accepted >= 0),
  events_suppressed    BIGINT NOT NULL DEFAULT 0 CHECK (events_suppressed >= 0),
  created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (workspace_id, utc_date)
);

-- +goose Down
DROP TABLE IF EXISTS x_inbound_daily_usage;
DROP TABLE IF EXISTS x_usage_events;
DROP TABLE IF EXISTS x_usage_periods;
