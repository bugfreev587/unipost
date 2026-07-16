-- +goose Up
-- Replacing a workspace X application's identity invalidates every upstream
-- rule and Activity subscription created by the previous application. Capture
-- the exact old resource IDs and old encrypted bearer in the same transaction
-- before the new credential row becomes visible.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION enqueue_replaced_workspace_x_inbox_delivery_cleanup()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  cleanup_reason TEXT;
  app_identity_changed BOOLEAN;
BEGIN
  IF OLD.platform <> 'twitter' THEN
    RETURN NEW;
  END IF;

  app_identity_changed :=
    NEW.platform IS DISTINCT FROM OLD.platform
    OR NEW.client_id IS DISTINCT FROM OLD.client_id;

  -- Older replicas preserve omitted optional X secrets by writing OLD values
  -- into NEW. Never carry those credentials across an application identity.
  IF app_identity_changed THEN
    IF NEW.app_bearer_token IS NOT DISTINCT FROM OLD.app_bearer_token THEN
      NEW.app_bearer_token := NULL;
    END IF;
    IF NEW.consumer_secret IS NOT DISTINCT FROM OLD.consumer_secret THEN
      NEW.consumer_secret := NULL;
    END IF;
  END IF;

  IF NOT (
    app_identity_changed
    OR (
      OLD.app_bearer_token IS NOT NULL
      AND NEW.app_bearer_token IS NULL
    )
    OR (
      OLD.consumer_secret IS NOT NULL
      AND NEW.consumer_secret IS NULL
    )
  ) THEN
    RETURN NEW;
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM profiles p
    JOIN social_accounts sa
      ON sa.profile_id = p.id
     AND sa.platform = 'twitter'
     AND sa.x_app_mode = 'workspace_x_app'
    JOIN x_inbox_delivery_resources r
      ON r.social_account_id = sa.id
    WHERE p.workspace_id = OLD.workspace_id
      AND (
        r.filtered_stream_rule_id IS NOT NULL
        OR r.activity_dm_subscription_id IS NOT NULL
      )
  ) THEN
    RETURN NEW;
  END IF;

  IF OLD.app_bearer_token IS NULL THEN
    RAISE EXCEPTION
      'cannot replace workspace X app identity while upstream resources exist without the old app bearer token';
  END IF;

  cleanup_reason := CASE
    WHEN app_identity_changed
      THEN 'workspace X app identity changed; upstream cleanup pending'
    ELSE 'workspace X app required credential removed; upstream cleanup pending'
  END;

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
   AND sa.x_app_mode = 'workspace_x_app'
  JOIN x_inbox_delivery_resources r
    ON r.social_account_id = sa.id
  WHERE p.workspace_id = OLD.workspace_id
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
      last_error = cleanup_reason,
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

  RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER platform_credentials_x_inbox_delivery_replacement_cleanup
BEFORE UPDATE OF platform, client_id, app_bearer_token, consumer_secret
ON platform_credentials
FOR EACH ROW
EXECUTE FUNCTION enqueue_replaced_workspace_x_inbox_delivery_cleanup();

-- +goose Down
DROP TRIGGER IF EXISTS platform_credentials_x_inbox_delivery_replacement_cleanup
  ON platform_credentials;
DROP FUNCTION IF EXISTS enqueue_replaced_workspace_x_inbox_delivery_cleanup();
