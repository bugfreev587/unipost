-- +goose Up
-- Upstream X rules and private Activity subscriptions outlive local rows.
-- Preserve their exact IDs and encrypted cleanup credentials before a
-- social-account cascade removes the normal delivery-resource row.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION x_inbox_delivery_cleanup_key(
  social_account_id TEXT,
  x_app_mode TEXT,
  source_app_identity TEXT,
  filtered_stream_rule_id TEXT,
  activity_dm_subscription_id TEXT
)
RETURNS TEXT
LANGUAGE SQL
IMMUTABLE
AS $$
  SELECT
    length(social_account_id)::TEXT || ':' || social_account_id || '|' ||
    length(x_app_mode)::TEXT || ':' || x_app_mode || '|' ||
    length(source_app_identity)::TEXT || ':' || source_app_identity || '|' ||
    COALESCE(
      length(filtered_stream_rule_id)::TEXT || ':' || filtered_stream_rule_id,
      '-1:'
    ) || '|' ||
    COALESCE(
      length(activity_dm_subscription_id)::TEXT || ':' || activity_dm_subscription_id,
      '-1:'
    )
$$;
-- +goose StatementEnd

CREATE TABLE x_inbox_delivery_cleanup_intents (
  id                          TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  cleanup_key                 TEXT NOT NULL UNIQUE,
  social_account_id           TEXT NOT NULL,
  x_app_mode                  TEXT NOT NULL
    CHECK (x_app_mode IN ('unipost_managed_app', 'workspace_x_app')),
  source_app_identity         TEXT NOT NULL,
  app_bearer_token            TEXT,
  filtered_stream_rule_id     TEXT,
  activity_dm_subscription_id TEXT,
  attempts                    INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  last_error                  TEXT,
  lease_owner                 TEXT,
  lease_until                 TIMESTAMPTZ,
  next_attempt_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (
    filtered_stream_rule_id IS NOT NULL
    OR activity_dm_subscription_id IS NOT NULL
  ),
  CHECK (
    x_app_mode = 'unipost_managed_app' OR app_bearer_token IS NOT NULL
  )
);

CREATE INDEX x_inbox_delivery_cleanup_intents_pending_idx
  ON x_inbox_delivery_cleanup_intents(next_attempt_at, created_at, id);

-- Capture every eligible resource before PostgreSQL begins cascading the
-- workspace delete. Child-table cascade order is not defined, so neither
-- profiles/social_accounts nor platform_credentials can be assumed visible
-- from a trigger on the other child.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION enqueue_deleted_workspace_x_inbox_delivery_cleanup()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  INSERT INTO x_inbox_delivery_cleanup_intents (
    cleanup_key,
    social_account_id,
    x_app_mode,
    source_app_identity,
    app_bearer_token,
    filtered_stream_rule_id,
    activity_dm_subscription_id
  )
  SELECT
    x_inbox_delivery_cleanup_key(
      sa.id,
      sa.x_app_mode,
      CASE
        WHEN sa.x_app_mode = 'unipost_managed_app' THEN 'unipost_managed_app'
        ELSE pc.client_id
      END,
      r.filtered_stream_rule_id,
      r.activity_dm_subscription_id
    ),
    sa.id,
    sa.x_app_mode,
    CASE
      WHEN sa.x_app_mode = 'unipost_managed_app' THEN 'unipost_managed_app'
      ELSE pc.client_id
    END,
    pc.app_bearer_token,
    r.filtered_stream_rule_id,
    r.activity_dm_subscription_id
  FROM profiles p
  JOIN social_accounts sa
    ON sa.profile_id = p.id
   AND sa.platform = 'twitter'
  JOIN x_inbox_delivery_resources r
    ON r.social_account_id = sa.id
  LEFT JOIN platform_credentials pc
    ON pc.workspace_id = OLD.id
   AND pc.platform = 'twitter'
  WHERE p.workspace_id = OLD.id
    AND sa.x_app_mode IN ('unipost_managed_app', 'workspace_x_app')
    AND (
      sa.x_app_mode = 'unipost_managed_app'
      OR (
        pc.client_id IS NOT NULL
        AND pc.app_bearer_token IS NOT NULL
      )
    )
    AND (
      r.filtered_stream_rule_id IS NOT NULL
      OR r.activity_dm_subscription_id IS NOT NULL
    )
  ON CONFLICT (cleanup_key) DO UPDATE
  SET app_bearer_token = COALESCE(
        EXCLUDED.app_bearer_token,
        x_inbox_delivery_cleanup_intents.app_bearer_token
      ),
      updated_at = NOW();

  RETURN OLD;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER workspaces_x_inbox_delivery_cleanup
BEFORE DELETE ON workspaces
FOR EACH ROW
EXECUTE FUNCTION enqueue_deleted_workspace_x_inbox_delivery_cleanup();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION enqueue_x_inbox_delivery_cleanup()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  INSERT INTO x_inbox_delivery_cleanup_intents (
    cleanup_key,
    social_account_id,
    x_app_mode,
    source_app_identity,
    app_bearer_token,
    filtered_stream_rule_id,
    activity_dm_subscription_id
  )
  SELECT
    x_inbox_delivery_cleanup_key(
      OLD.id,
      OLD.x_app_mode,
      CASE
        WHEN OLD.x_app_mode = 'unipost_managed_app' THEN 'unipost_managed_app'
        ELSE pc.client_id
      END,
      r.filtered_stream_rule_id,
      r.activity_dm_subscription_id
    ),
    OLD.id,
    OLD.x_app_mode,
    CASE
      WHEN OLD.x_app_mode = 'unipost_managed_app' THEN 'unipost_managed_app'
      ELSE pc.client_id
    END,
    pc.app_bearer_token,
    r.filtered_stream_rule_id,
    r.activity_dm_subscription_id
  FROM profiles p
  JOIN x_inbox_delivery_resources r
    ON r.social_account_id = OLD.id
  LEFT JOIN platform_credentials pc
    ON pc.workspace_id = p.workspace_id
   AND pc.platform = 'twitter'
  WHERE p.id = OLD.profile_id
    AND OLD.x_app_mode IN ('unipost_managed_app', 'workspace_x_app')
    AND (
      OLD.x_app_mode = 'unipost_managed_app'
      OR (
        pc.client_id IS NOT NULL
        AND pc.app_bearer_token IS NOT NULL
      )
    )
    AND (
      r.filtered_stream_rule_id IS NOT NULL
      OR r.activity_dm_subscription_id IS NOT NULL
    )
  ON CONFLICT (cleanup_key) DO UPDATE
  SET app_bearer_token = COALESCE(
        EXCLUDED.app_bearer_token,
        x_inbox_delivery_cleanup_intents.app_bearer_token
      ),
      updated_at = NOW();

  RETURN OLD;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER social_accounts_x_inbox_delivery_cleanup
BEFORE DELETE ON social_accounts
FOR EACH ROW
WHEN (OLD.platform = 'twitter')
EXECUTE FUNCTION enqueue_x_inbox_delivery_cleanup();

-- Workspace deletion can cascade platform_credentials before profiles (or
-- vice versa). Capture the same cleanup material from the credential side
-- so either cascade order preserves the encrypted workspace app bearer.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION enqueue_workspace_x_inbox_delivery_cleanup()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  INSERT INTO x_inbox_delivery_cleanup_intents (
    cleanup_key,
    social_account_id,
    x_app_mode,
    source_app_identity,
    app_bearer_token,
    filtered_stream_rule_id,
    activity_dm_subscription_id
  )
  SELECT
    x_inbox_delivery_cleanup_key(
      sa.id,
      sa.x_app_mode,
      OLD.client_id,
      r.filtered_stream_rule_id,
      r.activity_dm_subscription_id
    ),
    sa.id,
    sa.x_app_mode,
    OLD.client_id,
    OLD.app_bearer_token,
    r.filtered_stream_rule_id,
    r.activity_dm_subscription_id
  FROM profiles p
  JOIN social_accounts sa
    ON sa.profile_id = p.id
   AND sa.platform = 'twitter'
  JOIN x_inbox_delivery_resources r
    ON r.social_account_id = sa.id
  WHERE p.workspace_id = OLD.workspace_id
    AND sa.x_app_mode = 'workspace_x_app'
    AND OLD.app_bearer_token IS NOT NULL
    AND (
      r.filtered_stream_rule_id IS NOT NULL
      OR r.activity_dm_subscription_id IS NOT NULL
    )
  ON CONFLICT (cleanup_key) DO UPDATE
  SET app_bearer_token = COALESCE(
        EXCLUDED.app_bearer_token,
        x_inbox_delivery_cleanup_intents.app_bearer_token
      ),
      updated_at = NOW();

  UPDATE x_inbox_delivery_resources r
  SET filtered_stream_rule_id = NULL,
      activity_dm_subscription_id = NULL,
      delivery_status = 'error',
      last_error = 'workspace X app credential deleted; upstream cleanup pending',
      last_synced_at = NOW(),
      updated_at = NOW()
  FROM social_accounts sa
  JOIN profiles p
    ON p.id = sa.profile_id
  WHERE r.social_account_id = sa.id
    AND p.workspace_id = OLD.workspace_id
    AND sa.platform = 'twitter'
    AND sa.x_app_mode = 'workspace_x_app'
    AND (
      r.filtered_stream_rule_id IS NOT NULL
      OR r.activity_dm_subscription_id IS NOT NULL
    );

  RETURN OLD;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER platform_credentials_x_inbox_delivery_cleanup
BEFORE DELETE ON platform_credentials
FOR EACH ROW
WHEN (OLD.platform = 'twitter')
EXECUTE FUNCTION enqueue_workspace_x_inbox_delivery_cleanup();

-- +goose Down
DROP TRIGGER IF EXISTS platform_credentials_x_inbox_delivery_cleanup ON platform_credentials;
DROP FUNCTION IF EXISTS enqueue_workspace_x_inbox_delivery_cleanup();
DROP TRIGGER IF EXISTS social_accounts_x_inbox_delivery_cleanup ON social_accounts;
DROP FUNCTION IF EXISTS enqueue_x_inbox_delivery_cleanup();
DROP TRIGGER IF EXISTS workspaces_x_inbox_delivery_cleanup ON workspaces;
DROP FUNCTION IF EXISTS enqueue_deleted_workspace_x_inbox_delivery_cleanup();
DROP TABLE IF EXISTS x_inbox_delivery_cleanup_intents;
DROP FUNCTION IF EXISTS x_inbox_delivery_cleanup_key(TEXT, TEXT, TEXT, TEXT, TEXT);
