-- +goose Up

-- UniPost-owned notification tables. The original public.notification_*
-- names collided with another service sharing the same database. Keep the
-- old tables untouched and move UniPost reads/writes to explicit names.

CREATE TABLE IF NOT EXISTS unipost_notification_channels (
  id            TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  workspace_id  TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
  kind          TEXT NOT NULL CHECK (kind IN ('email','slack_webhook','discord_webhook','sms','in_app')),
  config        JSONB NOT NULL DEFAULT '{}'::jsonb,
  label         TEXT,
  verified_at   TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_unipost_notif_channels_user
  ON unipost_notification_channels(user_id)
  WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS unipost_notification_subscriptions (
  id            TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  workspace_id  TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
  event_type    TEXT NOT NULL,
  channel_id    TEXT NOT NULL REFERENCES unipost_notification_channels(id) ON DELETE CASCADE,
  enabled       BOOLEAN NOT NULL DEFAULT TRUE,
  filter        JSONB,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, workspace_id, event_type, channel_id)
);

CREATE INDEX IF NOT EXISTS idx_unipost_notif_subs_dispatch
  ON unipost_notification_subscriptions(event_type, user_id)
  WHERE enabled = TRUE;

CREATE TABLE IF NOT EXISTS unipost_notification_deliveries (
  id              TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  subscription_id TEXT NOT NULL REFERENCES unipost_notification_subscriptions(id) ON DELETE CASCADE,
  channel_id      TEXT NOT NULL REFERENCES unipost_notification_channels(id) ON DELETE CASCADE,
  event_type      TEXT NOT NULL,
  event_id        TEXT NOT NULL,
  payload         JSONB NOT NULL,
  status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sent','failed','dead','skipped')),
  attempts        INTEGER NOT NULL DEFAULT 0,
  next_retry_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_error      TEXT,
  delivered_at    TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (event_id, channel_id)
);

CREATE INDEX IF NOT EXISTS idx_unipost_notif_deliveries_pending
  ON unipost_notification_deliveries(next_retry_at)
  WHERE status = 'pending';

-- +goose StatementBegin
DO $$
BEGIN
  IF to_regclass('public.notification_channels') IS NOT NULL
     AND EXISTS (
       SELECT 1
       FROM information_schema.columns
       WHERE table_schema = 'public'
         AND table_name = 'notification_channels'
         AND column_name = 'user_id'
     )
  THEN
    EXECUTE $copy_channels$
      INSERT INTO unipost_notification_channels (
        id, user_id, workspace_id, kind, config, label, verified_at, created_at, deleted_at
      )
      SELECT
        id::TEXT,
        user_id::TEXT,
        workspace_id::TEXT,
        kind,
        config,
        label,
        verified_at,
        created_at,
        deleted_at
      FROM notification_channels
      ON CONFLICT (id) DO NOTHING
    $copy_channels$;
  END IF;

  IF to_regclass('public.notification_subscriptions') IS NOT NULL
     AND EXISTS (
       SELECT 1
       FROM information_schema.columns
       WHERE table_schema = 'public'
         AND table_name = 'notification_subscriptions'
         AND column_name = 'user_id'
     )
  THEN
    EXECUTE $copy_subscriptions$
      INSERT INTO unipost_notification_subscriptions (
        id, user_id, workspace_id, event_type, channel_id, enabled, filter, created_at
      )
      SELECT
        s.id::TEXT,
        s.user_id::TEXT,
        s.workspace_id::TEXT,
        s.event_type,
        s.channel_id::TEXT,
        s.enabled,
        s.filter,
        s.created_at
      FROM notification_subscriptions s
      WHERE EXISTS (
        SELECT 1
        FROM unipost_notification_channels c
        WHERE c.id = s.channel_id::TEXT
      )
      ON CONFLICT (id) DO NOTHING
    $copy_subscriptions$;
  END IF;

  IF to_regclass('public.notification_deliveries') IS NOT NULL
     AND EXISTS (
       SELECT 1
       FROM information_schema.columns
       WHERE table_schema = 'public'
         AND table_name = 'notification_deliveries'
         AND column_name = 'subscription_id'
     )
     AND EXISTS (
       SELECT 1
       FROM information_schema.columns
       WHERE table_schema = 'public'
         AND table_name = 'notification_deliveries'
         AND column_name = 'next_retry_at'
     )
  THEN
    EXECUTE $copy_deliveries$
      INSERT INTO unipost_notification_deliveries (
        id,
        subscription_id,
        channel_id,
        event_type,
        event_id,
        payload,
        status,
        attempts,
        next_retry_at,
        last_error,
        delivered_at,
        created_at
      )
      SELECT
        d.id::TEXT,
        d.subscription_id::TEXT,
        d.channel_id::TEXT,
        d.event_type,
        d.event_id,
        d.payload,
        d.status,
        d.attempts,
        d.next_retry_at,
        d.last_error,
        d.delivered_at,
        d.created_at
      FROM notification_deliveries d
      WHERE EXISTS (
        SELECT 1
        FROM unipost_notification_subscriptions s
        WHERE s.id = d.subscription_id::TEXT
      )
        AND EXISTS (
          SELECT 1
          FROM unipost_notification_channels c
          WHERE c.id = d.channel_id::TEXT
        )
      ON CONFLICT (id) DO NOTHING
    $copy_deliveries$;
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down

DROP TABLE IF EXISTS unipost_notification_deliveries;
DROP TABLE IF EXISTS unipost_notification_subscriptions;
DROP TABLE IF EXISTS unipost_notification_channels;
