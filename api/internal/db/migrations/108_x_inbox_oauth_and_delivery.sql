-- +goose Up
ALTER TABLE oauth_states
  ADD COLUMN pkce_verifier TEXT,
  ADD COLUMN x_app_mode TEXT;

ALTER TABLE connect_sessions
  ADD COLUMN x_app_mode TEXT;

ALTER TABLE social_accounts
  ADD COLUMN x_app_mode TEXT;

ALTER TABLE platform_credentials
  ADD COLUMN app_bearer_token TEXT,
  ADD COLUMN consumer_secret TEXT;

ALTER TABLE x_usage_events
  DROP CONSTRAINT IF EXISTS x_usage_events_connection_mode_check;
UPDATE x_usage_events
SET connection_mode = CASE
  WHEN connection_mode = 'managed' THEN 'unipost_managed_app'
  ELSE 'workspace_x_app'
END;
ALTER TABLE x_usage_events
  ALTER COLUMN connection_mode SET DEFAULT 'unipost_managed_app',
  ADD CONSTRAINT x_usage_events_connection_mode_check
    CHECK (connection_mode IN ('unipost_managed_app', 'workspace_x_app'));

ALTER TABLE social_post_results
  DROP CONSTRAINT IF EXISTS social_post_results_x_credit_billing_mode_check;
UPDATE social_post_results
SET x_credit_billing_mode = 'workspace_x_app'
WHERE x_credit_billing_mode = 'customer_x_app';
ALTER TABLE social_post_results
  ADD CONSTRAINT social_post_results_x_credit_billing_mode_check
    CHECK (x_credit_billing_mode IS NULL OR x_credit_billing_mode IN ('unipost_managed_app', 'workspace_x_app'));

UPDATE social_accounts sa
SET x_app_mode = CASE
  WHEN EXISTS (
    SELECT 1
    FROM profiles p
    JOIN platform_credentials pc
      ON pc.workspace_id = p.workspace_id
     AND pc.platform = 'twitter'
    WHERE p.id = sa.profile_id
  ) THEN 'workspace_x_app'
  WHEN sa.connection_type = 'managed' THEN 'unipost_managed_app'
  ELSE 'workspace_x_app'
END
WHERE sa.platform = 'twitter';

ALTER TABLE oauth_states
  ADD CONSTRAINT oauth_states_x_app_mode_check
  CHECK (
    x_app_mode IS NULL
    OR (platform = 'twitter' AND x_app_mode IN ('unipost_managed_app', 'workspace_x_app'))
  );

ALTER TABLE connect_sessions
  ADD CONSTRAINT connect_sessions_x_app_mode_check
  CHECK (
    x_app_mode IS NULL
    OR (platform = 'twitter' AND x_app_mode IN ('unipost_managed_app', 'workspace_x_app'))
  );

ALTER TABLE social_accounts
  ADD CONSTRAINT social_accounts_x_app_mode_check
  CHECK (
    (platform = 'twitter' AND x_app_mode IN ('unipost_managed_app', 'workspace_x_app'))
    OR (platform <> 'twitter' AND x_app_mode IS NULL)
  );

CREATE TABLE x_inbox_delivery_resources (
  social_account_id            TEXT PRIMARY KEY REFERENCES social_accounts(id) ON DELETE CASCADE,
  filtered_stream_rule_id      TEXT,
  activity_dm_subscription_id  TEXT,
  delivery_status              TEXT NOT NULL DEFAULT 'pending'
    CHECK (delivery_status IN ('pending', 'active', 'paused_cap', 'paused_allowance', 'paused_plan', 'error')),
  last_error                   TEXT,
  last_synced_at               TIMESTAMPTZ,
  created_at                   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at                   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS x_inbox_delivery_resources;

ALTER TABLE social_accounts
  DROP CONSTRAINT IF EXISTS social_accounts_x_app_mode_check,
  DROP COLUMN IF EXISTS x_app_mode;

ALTER TABLE connect_sessions
  DROP CONSTRAINT IF EXISTS connect_sessions_x_app_mode_check,
  DROP COLUMN IF EXISTS x_app_mode;

ALTER TABLE oauth_states
  DROP CONSTRAINT IF EXISTS oauth_states_x_app_mode_check,
  DROP COLUMN IF EXISTS x_app_mode,
  DROP COLUMN IF EXISTS pkce_verifier;

ALTER TABLE platform_credentials
  DROP COLUMN IF EXISTS consumer_secret,
  DROP COLUMN IF EXISTS app_bearer_token;

ALTER TABLE social_post_results
  DROP CONSTRAINT IF EXISTS social_post_results_x_credit_billing_mode_check;
UPDATE social_post_results
SET x_credit_billing_mode = 'customer_x_app'
WHERE x_credit_billing_mode = 'workspace_x_app';
ALTER TABLE social_post_results
  ADD CONSTRAINT social_post_results_x_credit_billing_mode_check
    CHECK (x_credit_billing_mode IS NULL OR x_credit_billing_mode IN ('unipost_managed_app', 'customer_x_app'));

ALTER TABLE x_usage_events
  DROP CONSTRAINT IF EXISTS x_usage_events_connection_mode_check;
UPDATE x_usage_events
SET connection_mode = CASE
  WHEN connection_mode = 'unipost_managed_app' THEN 'managed'
  ELSE 'byo'
END;
ALTER TABLE x_usage_events
  ALTER COLUMN connection_mode SET DEFAULT 'managed',
  ADD CONSTRAINT x_usage_events_connection_mode_check
    CHECK (connection_mode IN ('managed', 'byo'));
