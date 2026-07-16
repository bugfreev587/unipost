-- +goose Up
ALTER TABLE x_inbox_delivery_resources
  ADD COLUMN activity_webhook_route_key TEXT;

ALTER TABLE x_inbox_delivery_cleanup_intents
  ADD COLUMN webhook_route_key TEXT,
  ADD COLUMN consumer_secret TEXT;

-- Workspace subscriptions already point at the persisted route. Mark that
-- generation so a routine deploy does not recreate healthy subscriptions.
UPDATE x_inbox_delivery_resources r
SET activity_webhook_route_key = pc.webhook_route_key
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
JOIN platform_credentials pc
  ON pc.workspace_id = p.workspace_id
 AND pc.platform = 'twitter'
WHERE r.social_account_id = sa.id
  AND sa.platform = 'twitter'
  AND sa.x_app_mode = 'workspace_x_app'
  AND r.activity_dm_subscription_id IS NOT NULL
  AND pc.webhook_route_key IS NOT NULL;

-- The original cleanup triggers remain the authority for capturing exact
-- upstream resource IDs and encrypted app bearer tokens. These ordered
-- companion triggers run afterward and attach the old webhook signature
-- generation so its route stays valid only while cleanup is pending.
CREATE OR REPLACE FUNCTION augment_replaced_workspace_x_inbox_cleanup_route()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.client_id IS DISTINCT FROM OLD.client_id
    AND NEW.webhook_route_key IS NOT DISTINCT FROM OLD.webhook_route_key THEN
    NEW.webhook_route_key := NULL;
  END IF;

  UPDATE x_inbox_delivery_cleanup_intents i
  SET webhook_route_key = COALESCE(i.webhook_route_key, OLD.webhook_route_key),
      consumer_secret = COALESCE(i.consumer_secret, OLD.consumer_secret),
      updated_at = NOW()
  FROM profiles p
  JOIN social_accounts sa
    ON sa.profile_id = p.id
   AND sa.platform = 'twitter'
   AND sa.x_app_mode = 'workspace_x_app'
  WHERE p.workspace_id = OLD.workspace_id
    AND i.social_account_id = sa.id
    AND i.source_app_identity = OLD.client_id;
  RETURN NEW;
END;
$$;

CREATE TRIGGER zz_platform_credentials_x_inbox_cleanup_route_update
BEFORE UPDATE OF platform, client_id, app_bearer_token, consumer_secret, webhook_route_key
ON platform_credentials
FOR EACH ROW
WHEN (OLD.platform = 'twitter')
EXECUTE FUNCTION augment_replaced_workspace_x_inbox_cleanup_route();

CREATE OR REPLACE FUNCTION augment_deleted_workspace_x_credential_cleanup_route()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  UPDATE x_inbox_delivery_cleanup_intents i
  SET webhook_route_key = COALESCE(i.webhook_route_key, OLD.webhook_route_key),
      consumer_secret = COALESCE(i.consumer_secret, OLD.consumer_secret),
      updated_at = NOW()
  FROM profiles p
  JOIN social_accounts sa
    ON sa.profile_id = p.id
   AND sa.platform = 'twitter'
   AND sa.x_app_mode = 'workspace_x_app'
  WHERE p.workspace_id = OLD.workspace_id
    AND i.social_account_id = sa.id
    AND i.source_app_identity = OLD.client_id;
  RETURN OLD;
END;
$$;

CREATE TRIGGER zz_platform_credentials_x_inbox_cleanup_route_delete
BEFORE DELETE ON platform_credentials
FOR EACH ROW
WHEN (OLD.platform = 'twitter')
EXECUTE FUNCTION augment_deleted_workspace_x_credential_cleanup_route();

CREATE OR REPLACE FUNCTION augment_deleted_x_account_cleanup_route()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  UPDATE x_inbox_delivery_cleanup_intents i
  SET webhook_route_key = COALESCE(i.webhook_route_key, pc.webhook_route_key),
      consumer_secret = COALESCE(i.consumer_secret, pc.consumer_secret),
      updated_at = NOW()
  FROM profiles p
  LEFT JOIN platform_credentials pc
    ON pc.workspace_id = p.workspace_id
   AND pc.platform = 'twitter'
  WHERE p.id = OLD.profile_id
    AND i.social_account_id = OLD.id;
  RETURN OLD;
END;
$$;

CREATE TRIGGER zz_social_accounts_x_inbox_cleanup_route
BEFORE DELETE ON social_accounts
FOR EACH ROW
WHEN (OLD.platform = 'twitter')
EXECUTE FUNCTION augment_deleted_x_account_cleanup_route();

CREATE OR REPLACE FUNCTION augment_deleted_workspace_x_cleanup_routes()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  UPDATE x_inbox_delivery_cleanup_intents i
  SET webhook_route_key = COALESCE(i.webhook_route_key, pc.webhook_route_key),
      consumer_secret = COALESCE(i.consumer_secret, pc.consumer_secret),
      updated_at = NOW()
  FROM profiles p
  JOIN social_accounts sa
    ON sa.profile_id = p.id
   AND sa.platform = 'twitter'
  LEFT JOIN platform_credentials pc
    ON pc.workspace_id = OLD.id
   AND pc.platform = 'twitter'
  WHERE p.workspace_id = OLD.id
    AND i.social_account_id = sa.id;
  RETURN OLD;
END;
$$;

CREATE TRIGGER zz_workspaces_x_inbox_cleanup_routes
BEFORE DELETE ON workspaces
FOR EACH ROW
EXECUTE FUNCTION augment_deleted_workspace_x_cleanup_routes();

-- +goose Down
DROP TRIGGER IF EXISTS zz_workspaces_x_inbox_cleanup_routes ON workspaces;
DROP FUNCTION IF EXISTS augment_deleted_workspace_x_cleanup_routes();
DROP TRIGGER IF EXISTS zz_social_accounts_x_inbox_cleanup_route ON social_accounts;
DROP FUNCTION IF EXISTS augment_deleted_x_account_cleanup_route();
DROP TRIGGER IF EXISTS zz_platform_credentials_x_inbox_cleanup_route_delete ON platform_credentials;
DROP FUNCTION IF EXISTS augment_deleted_workspace_x_credential_cleanup_route();
DROP TRIGGER IF EXISTS zz_platform_credentials_x_inbox_cleanup_route_update ON platform_credentials;
DROP FUNCTION IF EXISTS augment_replaced_workspace_x_inbox_cleanup_route();

ALTER TABLE x_inbox_delivery_cleanup_intents
  DROP COLUMN IF EXISTS consumer_secret,
  DROP COLUMN IF EXISTS webhook_route_key;

ALTER TABLE x_inbox_delivery_resources
  DROP COLUMN IF EXISTS activity_webhook_route_key;
