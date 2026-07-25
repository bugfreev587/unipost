-- +goose Up
CREATE TABLE workspace_trial_grants (
  id                         TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  workspace_id               TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  kind                       TEXT NOT NULL CHECK (kind IN ('free_to_paid', 'paid_same_plan')),
  plan_id                    TEXT NOT NULL REFERENCES plans(id),
  duration_days              INTEGER NOT NULL CHECK (duration_days BETWEEN 1 AND 730),
  status                     TEXT NOT NULL CHECK (status IN ('provisioning', 'pending_activation', 'checkout_pending', 'scheduled', 'active', 'completed', 'canceled', 'revoked', 'superseded', 'failed')),
  granted_by_user_id         TEXT NOT NULL REFERENCES users(id),
  stripe_mode                TEXT,
  stripe_customer_id         TEXT,
  stripe_subscription_id     TEXT,
  stripe_schedule_id         TEXT,
  stripe_checkout_session_id TEXT,
  granted_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  scheduled_start_at         TIMESTAMPTZ,
  started_at                 TIMESTAMPTZ,
  ends_at                    TIMESTAMPTZ,
  activated_at               TIMESTAMPTZ,
  canceled_at                TIMESTAMPTZ,
  revoked_at                 TIMESTAMPTZ,
  superseded_at              TIMESTAMPTZ,
  completed_at               TIMESTAMPTZ,
  superseded_by_plan_id      TEXT REFERENCES plans(id),
  failure_code               TEXT,
  failure_message            TEXT,
  created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX workspace_trial_grants_open_workspace_idx
  ON workspace_trial_grants (workspace_id)
  WHERE status IN ('provisioning', 'pending_activation', 'checkout_pending', 'scheduled', 'active');

CREATE UNIQUE INDEX workspace_trial_grants_checkout_session_idx
  ON workspace_trial_grants (stripe_checkout_session_id)
  WHERE stripe_checkout_session_id IS NOT NULL;

CREATE INDEX workspace_trial_grants_subscription_idx
  ON workspace_trial_grants (stripe_subscription_id)
  WHERE stripe_subscription_id IS NOT NULL;

CREATE INDEX workspace_trial_grants_schedule_idx
  ON workspace_trial_grants (stripe_schedule_id)
  WHERE stripe_schedule_id IS NOT NULL;

CREATE INDEX workspace_trial_grants_workspace_history_idx
  ON workspace_trial_grants (workspace_id, granted_at DESC);

CREATE INDEX workspace_trial_grants_status_ends_at_idx
  ON workspace_trial_grants (status, ends_at);

-- +goose Down
DROP TABLE workspace_trial_grants;
