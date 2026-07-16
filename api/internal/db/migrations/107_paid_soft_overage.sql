-- +goose Up
ALTER TABLE social_posts
  ADD COLUMN quota_hold_reason TEXT,
  ADD COLUMN quota_hold_at TIMESTAMPTZ,
  ADD COLUMN quota_hold_original_scheduled_at TIMESTAMPTZ;

CREATE TABLE paid_plan_quota_notifications (
  id TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
  email TEXT,
  plan_id TEXT NOT NULL,
  period TEXT NOT NULL,
  threshold_percent INTEGER NOT NULL CHECK (threshold_percent IN (80, 90, 100, 105, 110, 115, 120)),
  severity TEXT NOT NULL CHECK (severity IN ('warning', 'alert', 'critical_alert')),
  event_key TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
    'pending',
    'processing',
    'sent',
    'retry_wait',
    'failed',
    'skipped_superseded',
    'skipped_preference_disabled',
    'skipped_missing_recipient'
  )),
  transactional_id TEXT,
  idempotency_key TEXT NOT NULL,
  completed_usage INTEGER NOT NULL,
  scheduled_usage INTEGER NOT NULL,
  quota_hold_usage INTEGER NOT NULL,
  effective_usage INTEGER NOT NULL,
  post_limit INTEGER NOT NULL,
  attempt_count INTEGER NOT NULL DEFAULT 0,
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  lease_expires_at TIMESTAMPTZ,
  last_error TEXT,
  attempted_at TIMESTAMPTZ,
  sent_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (workspace_id, period, threshold_percent),
  UNIQUE (idempotency_key)
);

CREATE INDEX paid_plan_quota_notifications_worker_idx
  ON paid_plan_quota_notifications (status, next_attempt_at, lease_expires_at);

CREATE INDEX paid_plan_quota_notifications_admin_idx
  ON paid_plan_quota_notifications (period, threshold_percent, created_at DESC);

CREATE TABLE paid_quota_follow_ups (
  id TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  owner_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
  plan_id TEXT NOT NULL,
  period TEXT NOT NULL,
  threshold_percent INTEGER NOT NULL DEFAULT 120 CHECK (threshold_percent = 120),
  notification_id TEXT REFERENCES paid_plan_quota_notifications(id) ON DELETE SET NULL,
  status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'contacted', 'resolved', 'dismissed')),
  completed_usage INTEGER NOT NULL,
  scheduled_usage INTEGER NOT NULL,
  quota_hold_usage INTEGER NOT NULL,
  effective_usage INTEGER NOT NULL,
  post_limit INTEGER NOT NULL,
  assignee_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
  notes TEXT,
  resolved_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (workspace_id, period, threshold_percent)
);

CREATE INDEX paid_quota_follow_ups_status_idx
  ON paid_quota_follow_ups (status, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS paid_quota_follow_ups;
DROP TABLE IF EXISTS paid_plan_quota_notifications;

ALTER TABLE social_posts
  DROP COLUMN IF EXISTS quota_hold_original_scheduled_at,
  DROP COLUMN IF EXISTS quota_hold_at,
  DROP COLUMN IF EXISTS quota_hold_reason;
