-- +goose Up
-- App Review Autopilot MVP: customer-domain TikTok review kits and
-- recording jobs. The feature ships behind app_review.autopilot_v1.

CREATE TABLE review_domains (
  id                 TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id       TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  domain             TEXT NOT NULL,
  provider           TEXT,
  status             TEXT NOT NULL DEFAULT 'pending',
  verification_token TEXT NOT NULL,
  cname_target       TEXT NOT NULL,
  dns_verified_at    TIMESTAMPTZ,
  tls_status         TEXT NOT NULL DEFAULT 'pending',
  tls_issued_at      TIMESTAMPTZ,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT review_domains_status_check
    CHECK (status IN ('pending', 'dns_pending', 'dns_verified', 'tls_pending', 'ready', 'failed')),
  CONSTRAINT review_domains_tls_status_check
    CHECK (tls_status IN ('pending', 'issued', 'failed'))
);

CREATE UNIQUE INDEX review_domains_domain_unique_idx
  ON review_domains (lower(domain));

CREATE INDEX review_domains_workspace_created_idx
  ON review_domains (workspace_id, created_at DESC);

CREATE TABLE review_kits (
  id               TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id     TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  platform         TEXT NOT NULL,
  use_case         TEXT NOT NULL,
  review_domain_id TEXT NOT NULL REFERENCES review_domains(id) ON DELETE RESTRICT,
  brand_snapshot   JSONB NOT NULL DEFAULT '{}'::jsonb,
  required_scopes  TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
  status           TEXT NOT NULL DEFAULT 'draft',
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT review_kits_platform_check
    CHECK (platform IN ('tiktok')),
  CONSTRAINT review_kits_status_check
    CHECK (status IN ('draft', 'blocked', 'ready', 'archived'))
);

CREATE INDEX review_kits_workspace_created_idx
  ON review_kits (workspace_id, created_at DESC);

CREATE INDEX review_kits_domain_idx
  ON review_kits (review_domain_id);

CREATE TABLE review_jobs (
  id                      TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  review_kit_id           TEXT NOT NULL REFERENCES review_kits(id) ON DELETE CASCADE,
  workspace_id             TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  platform                 TEXT NOT NULL,
  status                   TEXT NOT NULL DEFAULT 'queued',
  started_at               TIMESTAMPTZ,
  completed_at             TIMESTAMPTZ,
  failed_at                TIMESTAMPTZ,
  failure_reason           TEXT,
  agent_version            TEXT,
  review_session_token_id  TEXT,
  video_file_id            TEXT,
  artifacts_json           JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT review_jobs_platform_check
    CHECK (platform IN ('tiktok')),
  CONSTRAINT review_jobs_status_check
    CHECK (status IN ('queued', 'running', 'waiting_for_user', 'completed', 'failed'))
);

CREATE INDEX review_jobs_workspace_created_idx
  ON review_jobs (workspace_id, created_at DESC);

CREATE INDEX review_jobs_kit_created_idx
  ON review_jobs (review_kit_id, created_at DESC);

CREATE TABLE review_job_events (
  id            BIGSERIAL PRIMARY KEY,
  review_job_id TEXT NOT NULL REFERENCES review_jobs(id) ON DELETE CASCADE,
  event_type    TEXT NOT NULL,
  message       TEXT NOT NULL DEFAULT '',
  metadata      JSONB NOT NULL DEFAULT '{}'::jsonb,
  elapsed_ms    BIGINT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX review_job_events_job_created_idx
  ON review_job_events (review_job_id, created_at ASC);

CREATE TABLE review_sessions (
  id             TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  review_job_id  TEXT NOT NULL REFERENCES review_jobs(id) ON DELETE CASCADE,
  review_kit_id  TEXT NOT NULL REFERENCES review_kits(id) ON DELETE CASCADE,
  workspace_id    TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  platform        TEXT NOT NULL,
  review_domain   TEXT NOT NULL,
  token_hash      TEXT NOT NULL UNIQUE,
  expires_at      TIMESTAMPTZ NOT NULL,
  claimed_at      TIMESTAMPTZ,
  revoked_at      TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT review_sessions_platform_check
    CHECK (platform IN ('tiktok'))
);

CREATE INDEX review_sessions_job_idx
  ON review_sessions (review_job_id);

CREATE INDEX review_sessions_workspace_created_idx
  ON review_sessions (workspace_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS review_sessions;
DROP TABLE IF EXISTS review_job_events;
DROP TABLE IF EXISTS review_jobs;
DROP TABLE IF EXISTS review_kits;
DROP TABLE IF EXISTS review_domains;
