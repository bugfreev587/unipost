-- +goose Up

CREATE TABLE free_plan_quota_email_reminders (
  id                 TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  workspace_id       TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id            TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  email              TEXT NOT NULL,
  period             TEXT NOT NULL,
  threshold_percent  INTEGER NOT NULL CHECK (threshold_percent IN (80, 85, 90, 95, 100)),
  status             TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
  transactional_id   TEXT NOT NULL,
  idempotency_key    TEXT NOT NULL UNIQUE,
  effective_usage    INTEGER NOT NULL,
  completed_usage    INTEGER NOT NULL,
  reserved_usage     INTEGER NOT NULL DEFAULT 0,
  post_limit         INTEGER NOT NULL,
  failure_reason     TEXT,
  attempted_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  sent_at            TIMESTAMPTZ,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (workspace_id, period, threshold_percent)
);

CREATE INDEX free_plan_quota_email_reminders_workspace_period_idx
  ON free_plan_quota_email_reminders (workspace_id, period, threshold_percent);

CREATE INDEX free_plan_quota_email_reminders_status_idx
  ON free_plan_quota_email_reminders (status, attempted_at DESC);

-- +goose Down

DROP TABLE IF EXISTS free_plan_quota_email_reminders;
