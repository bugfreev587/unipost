-- +goose Up

CREATE TABLE platform_publishing_restrictions (
  id                     TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  platform               TEXT NOT NULL UNIQUE,
  enabled                BOOLEAN NOT NULL DEFAULT FALSE,
  restricted_plan_ids    TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
  reason_code            TEXT NOT NULL,
  user_message           TEXT NOT NULL,
  cycle_id               TEXT,
  version                BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  enabled_at             TIMESTAMPTZ,
  disabled_at            TIMESTAMPTZ,
  updated_by_user_id     TEXT REFERENCES users(id) ON DELETE SET NULL,
  created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE platform_publishing_restriction_events (
  id                     TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  restriction_id         TEXT NOT NULL REFERENCES platform_publishing_restrictions(id) ON DELETE RESTRICT,
  platform               TEXT NOT NULL,
  cycle_id               TEXT NOT NULL,
  event_type             TEXT NOT NULL CHECK (event_type IN ('enabled', 'disabled')),
  actor_user_id          TEXT REFERENCES users(id) ON DELETE SET NULL,
  expected_version       BIGINT NOT NULL,
  resulting_version      BIGINT NOT NULL,
  before_state           JSONB NOT NULL,
  after_state            JSONB NOT NULL,
  request_id             TEXT,
  actor_ip               TEXT,
  actor_user_agent       TEXT,
  created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX platform_publishing_restriction_events_lookup_idx
  ON platform_publishing_restriction_events (restriction_id, created_at DESC);

CREATE TABLE platform_publishing_restriction_email_campaigns (
  id                     TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  restriction_id         TEXT NOT NULL REFERENCES platform_publishing_restrictions(id) ON DELETE RESTRICT,
  cycle_id               TEXT NOT NULL,
  campaign_type          TEXT NOT NULL CHECK (campaign_type IN ('restriction_notice', 'recovery_notice')),
  status                 TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'completed', 'completed_with_failures', 'failed')),
  subject_snapshot       TEXT NOT NULL,
  body_snapshot          TEXT NOT NULL,
  restriction_version    BIGINT NOT NULL,
  previewed_count        INTEGER NOT NULL DEFAULT 0 CHECK (previewed_count >= 0),
  snapshotted_count      INTEGER NOT NULL DEFAULT 0 CHECK (snapshotted_count >= 0),
  pending_count          INTEGER NOT NULL DEFAULT 0 CHECK (pending_count >= 0),
  sent_count             INTEGER NOT NULL DEFAULT 0 CHECK (sent_count >= 0),
  failed_count           INTEGER NOT NULL DEFAULT 0 CHECK (failed_count >= 0),
  skipped_count          INTEGER NOT NULL DEFAULT 0 CHECK (skipped_count >= 0),
  created_by_user_id     TEXT REFERENCES users(id) ON DELETE SET NULL,
  confirmed_by_user_id   TEXT REFERENCES users(id) ON DELETE SET NULL,
  snapshotted_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  started_at             TIMESTAMPTZ,
  completed_at           TIMESTAMPTZ,
  created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (cycle_id, campaign_type)
);

CREATE INDEX platform_publishing_restriction_campaigns_status_idx
  ON platform_publishing_restriction_email_campaigns (status, created_at);

CREATE TABLE platform_publishing_restriction_email_recipients (
  id                     TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  campaign_id            TEXT NOT NULL REFERENCES platform_publishing_restriction_email_campaigns(id) ON DELETE CASCADE,
  canonical_user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  recipient_email        TEXT NOT NULL,
  normalized_email       TEXT NOT NULL,
  first_name_snapshot    TEXT,
  represented_workspace_ids TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
  status                 TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sending', 'sent', 'failed', 'skipped_ineligible')),
  attempt_count          INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
  next_attempt_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  claimed_at             TIMESTAMPTZ,
  last_error             TEXT,
  idempotency_key        TEXT NOT NULL,
  email_send_attempt_id  TEXT REFERENCES email_send_attempts(id) ON DELETE SET NULL,
  sent_at                TIMESTAMPTZ,
  created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (campaign_id, canonical_user_id),
  UNIQUE (campaign_id, normalized_email),
  UNIQUE (idempotency_key)
);

CREATE INDEX platform_publishing_restriction_recipients_claim_idx
  ON platform_publishing_restriction_email_recipients (status, next_attempt_at, created_at)
  WHERE status IN ('pending', 'failed');

ALTER TABLE media_post_usages
  ADD COLUMN retention_reason TEXT NOT NULL DEFAULT 'plan_status'
  CHECK (retention_reason IN ('active_post', 'plan_status', 'publishing_restriction'));

INSERT INTO platform_publishing_restrictions (
  platform,
  enabled,
  restricted_plan_ids,
  reason_code,
  user_message
) VALUES (
  'tiktok', FALSE, ARRAY['free']::TEXT[], 'platform_capacity_limit',
  'TikTok publishing is temporarily unavailable on the Free plan due to platform capacity limits. We’re working with TikTok to increase capacity. Upgrade your plan or try again after the restriction is lifted.'
)
ON CONFLICT (platform) DO NOTHING;

-- +goose Down

ALTER TABLE media_post_usages
  DROP COLUMN IF EXISTS retention_reason;

DROP TABLE IF EXISTS platform_publishing_restriction_email_recipients;
DROP TABLE IF EXISTS platform_publishing_restriction_email_campaigns;
DROP TABLE IF EXISTS platform_publishing_restriction_events;
DROP TABLE IF EXISTS platform_publishing_restrictions;
