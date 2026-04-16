-- +goose Up

-- User-facing notification system. Separate from the developer webhooks
-- system in migration 008 — that one is a firehose keyed on workspace,
-- this one is a curated set of events rendered to a human and delivered
-- on a channel they picked (email / Slack / SMS / in-app). v0 only
-- wires the 'email' channel; the other kinds are modeled but unused.

-- A single destination the user has configured. A user may have several
-- (home email + work email + ops Slack). workspace_id NULL means the
-- channel is account-level; otherwise scoped to that workspace.
CREATE TABLE notification_channels (
  id            TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  workspace_id  TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
  kind          TEXT NOT NULL CHECK (kind IN ('email','slack_webhook','sms','in_app')),
  -- Destination-specific config. Shape per kind:
  --   email:         {"address": "user@example.com"}
  --   slack_webhook: {"url": "https://hooks.slack.com/services/..."}
  --   sms:           {"e164": "+14155551212"}
  --   in_app:        {}  (the user_id already identifies the inbox)
  config        JSONB NOT NULL DEFAULT '{}'::jsonb,
  label         TEXT,
  -- email / sms need OTP verification before first send. slack_webhook
  -- is self-authenticating (only workspace admin owns the URL). in_app
  -- is auto-verified. NULL means "not yet verified — do not dispatch".
  verified_at   TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at    TIMESTAMPTZ
);

CREATE INDEX idx_notif_channels_user ON notification_channels(user_id) WHERE deleted_at IS NULL;

-- Which events route to which channels. Many-to-many: one event can
-- notify N channels, one channel can carry N events.
CREATE TABLE notification_subscriptions (
  id            TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  workspace_id  TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
  -- Event name matching events.EventXxx constants (post.failed, etc).
  event_type    TEXT NOT NULL,
  channel_id    TEXT NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
  enabled       BOOLEAN NOT NULL DEFAULT TRUE,
  -- Optional filter predicate — future use for {"platforms":["instagram"]}
  -- or {"severity":"critical"}. NULL = match every event of this type.
  filter        JSONB,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  -- A given (user, event, channel) pair is unique per scope.
  -- workspace_id NULL counts as its own scope.
  UNIQUE (user_id, workspace_id, event_type, channel_id)
);

-- Hot path: dispatcher looks up subscriptions by event_type during
-- fanout. workspace_id is part of the WHERE clause (OR workspace_id IS NULL).
CREATE INDEX idx_notif_subs_dispatch ON notification_subscriptions(event_type, user_id) WHERE enabled = TRUE;

-- Delivery queue. Mirrors webhook_deliveries pattern so the poller /
-- retry logic can look familiar to anyone who's read worker/webhook.go.
CREATE TABLE notification_deliveries (
  id              TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  subscription_id TEXT NOT NULL REFERENCES notification_subscriptions(id) ON DELETE CASCADE,
  channel_id      TEXT NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
  event_type      TEXT NOT NULL,
  -- Stable identifier for the logical event, set by the publisher.
  -- Combined with the UNIQUE below this makes fanout idempotent: if
  -- the publish path retries the same event, we won't double-notify.
  event_id        TEXT NOT NULL,
  -- Rendered template inputs. What the worker feeds to the mailer.
  payload         JSONB NOT NULL,
  status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sent','failed','dead')),
  attempts        INTEGER NOT NULL DEFAULT 0,
  next_retry_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_error      TEXT,
  delivered_at    TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  -- Idempotent fanout: same event_id to same channel = one delivery.
  UNIQUE (event_id, channel_id)
);

-- Hot path: worker polls for pending rows whose retry time has come.
CREATE INDEX idx_notif_deliveries_pending
  ON notification_deliveries(next_retry_at)
  WHERE status = 'pending';

-- +goose Down

DROP TABLE IF EXISTS notification_deliveries;
DROP TABLE IF EXISTS notification_subscriptions;
DROP TABLE IF EXISTS notification_channels;
