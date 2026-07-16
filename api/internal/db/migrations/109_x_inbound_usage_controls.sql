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
  period_start          TIMESTAMPTZ NOT NULL,
  period_end            TIMESTAMPTZ NOT NULL,
  monthly_used_after    BIGINT NOT NULL DEFAULT 0 CHECK (monthly_used_after >= 0),
  monthly_remaining_after BIGINT NOT NULL DEFAULT 0 CHECK (monthly_remaining_after >= 0),
  inbound_daily_used_after BIGINT NOT NULL DEFAULT 0 CHECK (inbound_daily_used_after >= 0),
  inbound_daily_limit   BIGINT NOT NULL DEFAULT 0 CHECK (inbound_daily_limit >= 0),
  events_accepted_after BIGINT NOT NULL DEFAULT 0 CHECK (events_accepted_after >= 0),
  events_suppressed_after BIGINT NOT NULL DEFAULT 0 CHECK (events_suppressed_after >= 0),
  pause_paid_sources    BOOLEAN NOT NULL DEFAULT FALSE,
  pause_reason          TEXT NOT NULL DEFAULT '',
  reset_at              TIMESTAMPTZ NOT NULL,
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

-- Existing users have already passed the one-time bootstrap path. Give
-- active owner/admin members a verified email channel when they do not
-- have one, then add only missing subscriptions for these two new event
-- keys. The NOT EXISTS guard preserves any pre-created disabled row.
INSERT INTO unipost_notification_channels (
  user_id,
  workspace_id,
  kind,
  config,
  label,
  verified_at
)
SELECT DISTINCT
  wm.user_id,
  NULL,
  'email',
  jsonb_build_object('address', u.email),
  'X inbound alerts (migration 109)',
  NOW()
FROM workspace_members wm
JOIN users u ON u.id = wm.user_id
WHERE wm.status = 'active'
  AND wm.role IN ('owner', 'admin')
  AND u.email <> ''
  AND NOT EXISTS (
    SELECT 1
    FROM unipost_notification_channels c
    WHERE c.user_id = wm.user_id
      AND c.kind = 'email'
      AND c.workspace_id IS NULL
      AND c.deleted_at IS NULL
      AND c.verified_at IS NOT NULL
  );

WITH eligible_members AS (
  SELECT wm.workspace_id, wm.user_id
  FROM workspace_members wm
  WHERE wm.status = 'active'
    AND wm.role IN ('owner', 'admin')
),
target_channels AS (
  SELECT DISTINCT ON (c.user_id)
    c.user_id,
    c.id AS channel_id
  FROM unipost_notification_channels c
  JOIN eligible_members em ON em.user_id = c.user_id
  WHERE c.kind = 'email'
    AND c.workspace_id IS NULL
    AND c.deleted_at IS NULL
    AND c.verified_at IS NOT NULL
  ORDER BY c.user_id, c.created_at ASC, c.id ASC
),
event_keys(event_type) AS (
  VALUES
    ('billing.x_inbound_80pct'::TEXT),
    ('billing.x_inbound_cap_reached'::TEXT)
)
INSERT INTO unipost_notification_subscriptions (
  user_id,
  workspace_id,
  event_type,
  channel_id,
  enabled,
  filter
)
SELECT
  em.user_id,
  em.workspace_id,
  ek.event_type,
  tc.channel_id,
  TRUE,
  NULL
FROM eligible_members em
JOIN target_channels tc ON tc.user_id = em.user_id
CROSS JOIN event_keys ek
WHERE NOT EXISTS (
  SELECT 1
  FROM unipost_notification_subscriptions s
  WHERE s.user_id = em.user_id
    AND s.event_type = ek.event_type
    AND (s.workspace_id = em.workspace_id OR s.workspace_id IS NULL)
)
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM unipost_notification_subscriptions
WHERE event_type IN ('billing.x_inbound_80pct', 'billing.x_inbound_cap_reached');

DELETE FROM unipost_notification_channels c
WHERE c.label = 'X inbound alerts (migration 109)'
  AND NOT EXISTS (
    SELECT 1
    FROM unipost_notification_subscriptions s
    WHERE s.channel_id = c.id
  );

DROP TABLE IF EXISTS x_inbound_cap_notifications;
DROP TABLE IF EXISTS x_inbound_cap_settings;
DROP TABLE IF EXISTS x_inbound_event_receipts;
