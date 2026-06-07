-- +goose Up

CREATE TABLE error_triage_runs (
  id                   TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  run_type             TEXT NOT NULL CHECK (run_type IN ('scheduled','manual')),
  status               TEXT NOT NULL DEFAULT 'running' CHECK (status IN ('running','completed','failed')),
  window_start         TIMESTAMPTZ NOT NULL,
  window_end           TIMESTAMPTZ NOT NULL,
  timezone             TEXT NOT NULL DEFAULT 'America/Los_Angeles',
  supersedes_run_id    TEXT REFERENCES error_triage_runs(id) ON DELETE SET NULL,
  started_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  completed_at         TIMESTAMPTZ,
  model                TEXT,
  prompt_version       TEXT,
  failures_analyzed    INTEGER NOT NULL DEFAULT 0,
  affected_users       INTEGER NOT NULL DEFAULT 0,
  affected_workspaces  INTEGER NOT NULL DEFAULT 0,
  summary              TEXT,
  error_message        TEXT,
  created_by_admin_id  TEXT,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX error_triage_runs_scheduled_window_uniq
  ON error_triage_runs (window_start)
  WHERE run_type = 'scheduled';

CREATE INDEX error_triage_runs_created_idx
  ON error_triage_runs (created_at DESC);

CREATE TABLE error_triage_items (
  id                         TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id                     TEXT NOT NULL REFERENCES error_triage_runs(id) ON DELETE CASCADE,
  dedupe_key                 TEXT NOT NULL,
  classification             TEXT NOT NULL CHECK (classification IN ('unipost_bug','user_action_needed','upstream_platform_issue','transient_no_action','needs_human_review')),
  action_kind                TEXT NOT NULL CHECK (action_kind IN ('none','email','bug_plan','monitor','review')),
  workflow_status            TEXT NOT NULL CHECK (workflow_status IN ('pending_review','ready','partially_completed','completed','dismissed','failed')),
  confidence                 NUMERIC NOT NULL DEFAULT 0,
  platform                   TEXT,
  source                     TEXT,
  error_code                 TEXT,
  platform_error_code        TEXT,
  failure_stage              TEXT,
  affected_user_count        INTEGER NOT NULL DEFAULT 0,
  affected_workspace_count   INTEGER NOT NULL DEFAULT 0,
  affected_post_count        INTEGER NOT NULL DEFAULT 0,
  latest_failure_at          TIMESTAMPTZ,
  evidence_json              JSONB NOT NULL DEFAULT '{}'::jsonb,
  ai_summary                 TEXT,
  admin_notes                TEXT,
  bug_plan_json              JSONB,
  email_draft_json           JSONB,
  cta_url                    TEXT,
  duplicate_of_item_id       TEXT REFERENCES error_triage_items(id) ON DELETE SET NULL,
  created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX error_triage_items_run_idx
  ON error_triage_items (run_id, created_at DESC);

CREATE INDEX error_triage_items_dedupe_idx
  ON error_triage_items (dedupe_key, created_at DESC);

CREATE TABLE error_triage_item_failures (
  id                    TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  item_id               TEXT NOT NULL REFERENCES error_triage_items(id) ON DELETE CASCADE,
  post_id               TEXT NOT NULL REFERENCES social_posts(id) ON DELETE CASCADE,
  social_post_result_id TEXT REFERENCES social_post_results(id) ON DELETE SET NULL,
  post_failure_id       TEXT REFERENCES post_failures(id) ON DELETE SET NULL,
  workspace_id          TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id               TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  user_email            TEXT NOT NULL,
  platform              TEXT,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX error_triage_item_failures_item_idx
  ON error_triage_item_failures (item_id);

CREATE TABLE error_triage_item_recipients (
  id                      TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  item_id                 TEXT NOT NULL REFERENCES error_triage_items(id) ON DELETE CASCADE,
  recipient_scope_key     TEXT NOT NULL,
  workspace_id            TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  recipient_user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  email_snapshot          TEXT NOT NULL,
  status                  TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sent','dismissed','send_failed')),
  latest_send_attempt_id  TEXT,
  dismissed_by_admin_id   TEXT,
  dismissed_at            TIMESTAMPTZ,
  dismiss_reason          TEXT,
  created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (item_id, recipient_scope_key)
);

CREATE INDEX error_triage_item_recipients_item_idx
  ON error_triage_item_recipients (item_id);

CREATE TABLE error_triage_email_sends (
  id                      TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  item_id                 TEXT NOT NULL REFERENCES error_triage_items(id) ON DELETE CASCADE,
  recipient_id            TEXT NOT NULL REFERENCES error_triage_item_recipients(id) ON DELETE CASCADE,
  recipient_scope_key     TEXT NOT NULL,
  recipient_user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  recipient_email         TEXT NOT NULL,
  attempt_number          INTEGER NOT NULL,
  loops_event_name        TEXT NOT NULL DEFAULT 'error_triage_user_action',
  loops_transactional_id  TEXT,
  idempotency_key         TEXT NOT NULL,
  subject_snapshot        TEXT NOT NULL,
  body_snapshot           TEXT NOT NULL,
  sent_by_admin_id        TEXT NOT NULL,
  sent_at                 TIMESTAMPTZ,
  provider_status         TEXT NOT NULL DEFAULT 'pending' CHECK (provider_status IN ('pending','succeeded','failed')),
  provider_error          TEXT,
  created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX error_triage_email_sends_recipient_idx
  ON error_triage_email_sends (recipient_id, created_at DESC);

CREATE UNIQUE INDEX error_triage_email_sends_success_idempotency_uniq
  ON error_triage_email_sends (idempotency_key)
  WHERE provider_status = 'succeeded';

-- +goose Down

DROP TABLE IF EXISTS error_triage_email_sends;
DROP TABLE IF EXISTS error_triage_item_recipients;
DROP TABLE IF EXISTS error_triage_item_failures;
DROP TABLE IF EXISTS error_triage_items;
DROP TABLE IF EXISTS error_triage_runs;
