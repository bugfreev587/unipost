-- +goose Up

ALTER TABLE x_inbox_backfill_exposure_reservations
  ADD COLUMN accounting_enabled BOOLEAN NOT NULL DEFAULT TRUE;

CREATE TABLE feature_flags (
  key         TEXT PRIMARY KEY CHECK (key IN ('x_dms_v1', 'x_credits_billing_v1')),
  enabled     BOOLEAN NOT NULL DEFAULT FALSE,
  description TEXT NOT NULL,
  updated_by  TEXT NOT NULL DEFAULT 'system',
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE feature_flag_changes (
  id               TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  flag_key         TEXT NOT NULL REFERENCES feature_flags(key) ON DELETE RESTRICT,
  previous_enabled BOOLEAN NOT NULL,
  enabled          BOOLEAN NOT NULL,
  changed_by       TEXT NOT NULL,
  changed_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX feature_flag_changes_flag_time_idx
  ON feature_flag_changes (flag_key, changed_at DESC);

INSERT INTO feature_flags (key, enabled, description)
VALUES
  ('x_dms_v1', FALSE, 'Makes X direct messages available to regular users.'),
  ('x_credits_billing_v1', FALSE, 'Counts managed X API operations against customer X Credits.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DROP TABLE IF EXISTS feature_flag_changes;
DROP TABLE IF EXISTS feature_flags;
ALTER TABLE x_inbox_backfill_exposure_reservations
  DROP COLUMN IF EXISTS accounting_enabled;
