-- +goose Up

CREATE TABLE email_preferences (
  id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  email        TEXT NOT NULL,
  category_key TEXT NOT NULL,
  enabled      BOOLEAN NOT NULL DEFAULT TRUE,
  source       TEXT NOT NULL DEFAULT 'settings',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, category_key)
);

CREATE INDEX email_preferences_user_idx
  ON email_preferences (user_id, category_key);

CREATE INDEX email_preferences_email_idx
  ON email_preferences (LOWER(email));

INSERT INTO email_preferences (
  user_id,
  email,
  category_key,
  enabled,
  source
)
SELECT
  c.user_id,
  COALESCE(NULLIF(u.email, ''), NULLIF(c.config->>'address', ''), '') AS email,
  CASE s.event_type
    WHEN 'post.failed' THEN 'publishing_failures'
    WHEN 'account.disconnected' THEN 'account_connection_alerts'
  END AS category_key,
  BOOL_OR(s.enabled) AS enabled,
  'notification_subscription_backfill' AS source
FROM unipost_notification_subscriptions s
JOIN unipost_notification_channels c ON c.id = s.channel_id
LEFT JOIN users u ON u.id = c.user_id
WHERE c.kind = 'email'
  AND c.deleted_at IS NULL
  AND s.workspace_id IS NULL
  AND s.event_type IN ('post.failed', 'account.disconnected')
GROUP BY c.user_id, COALESCE(NULLIF(u.email, ''), NULLIF(c.config->>'address', ''), ''), s.event_type
ON CONFLICT (user_id, category_key) DO NOTHING;

-- +goose Down

DROP TABLE IF EXISTS email_preferences;
