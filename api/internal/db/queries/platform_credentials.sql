-- name: CreatePlatformCredential :one
INSERT INTO platform_credentials (
  workspace_id, platform, client_id, client_secret,
  app_bearer_token, consumer_secret, webhook_route_key
)
VALUES (
  $1, $2, $3, $4,
  sqlc.narg(app_bearer_token)::TEXT,
  sqlc.narg(consumer_secret)::TEXT,
  CASE
    WHEN $2 = 'twitter'
      AND sqlc.narg(app_bearer_token)::TEXT IS NOT NULL
      AND sqlc.narg(consumer_secret)::TEXT IS NOT NULL
      THEN sqlc.arg(webhook_route_key)::TEXT
    ELSE NULL
  END
)
ON CONFLICT (workspace_id, platform) DO UPDATE
SET client_id = EXCLUDED.client_id,
    client_secret = EXCLUDED.client_secret,
    app_bearer_token = CASE
      WHEN sqlc.arg(app_bearer_token_supplied)::BOOLEAN THEN EXCLUDED.app_bearer_token
      WHEN platform_credentials.client_id = EXCLUDED.client_id THEN platform_credentials.app_bearer_token
      ELSE NULL
    END,
    consumer_secret = CASE
      WHEN sqlc.arg(consumer_secret_supplied)::BOOLEAN THEN EXCLUDED.consumer_secret
      WHEN platform_credentials.client_id = EXCLUDED.client_id THEN platform_credentials.consumer_secret
      ELSE NULL
    END,
    webhook_route_key = CASE
      WHEN platform_credentials.client_id = EXCLUDED.client_id
        AND platform_credentials.webhook_route_key IS NOT NULL
        THEN platform_credentials.webhook_route_key
      WHEN (
        CASE
          WHEN sqlc.arg(app_bearer_token_supplied)::BOOLEAN THEN EXCLUDED.app_bearer_token
          WHEN platform_credentials.client_id = EXCLUDED.client_id THEN platform_credentials.app_bearer_token
          ELSE NULL
        END
      ) IS NOT NULL
      AND (
        CASE
          WHEN sqlc.arg(consumer_secret_supplied)::BOOLEAN THEN EXCLUDED.consumer_secret
          WHEN platform_credentials.client_id = EXCLUDED.client_id THEN platform_credentials.consumer_secret
          ELSE NULL
        END
      ) IS NOT NULL
        THEN EXCLUDED.webhook_route_key
      ELSE NULL
    END
RETURNING id, platform, client_id, client_secret, created_at, workspace_id,
  app_bearer_token, consumer_secret, webhook_route_key;

-- name: GetPlatformCredential :one
SELECT id, platform, client_id, client_secret, created_at, workspace_id,
  app_bearer_token, consumer_secret, webhook_route_key
FROM platform_credentials
WHERE workspace_id = $1 AND platform = $2;

-- name: ListPlatformCredentialsByWorkspace :many
SELECT id, platform, client_id, client_secret, created_at, workspace_id,
  app_bearer_token, consumer_secret, webhook_route_key
FROM platform_credentials
WHERE workspace_id = $1
ORDER BY platform;

-- name: DeletePlatformCredential :exec
DELETE FROM platform_credentials
WHERE workspace_id = $1 AND platform = $2;

-- name: ListTwitterConsumerSecretsByWebhookRouteKey :many
SELECT consumer_secret
FROM (
  SELECT workspace_id, consumer_secret
  FROM platform_credentials pc
  WHERE pc.platform = 'twitter'
    AND pc.webhook_route_key = $1
    AND pc.consumer_secret IS NOT NULL
    AND pc.consumer_secret <> ''
  UNION ALL
  SELECT social_account_id AS workspace_id, consumer_secret
  FROM x_inbox_delivery_cleanup_intents ci
  WHERE ci.webhook_route_key = $1
    AND ci.consumer_secret IS NOT NULL
    AND ci.consumer_secret <> ''
) route_secrets
ORDER BY workspace_id;

-- name: ListTwitterCredentialsMissingWebhookRouteKey :many
SELECT workspace_id, client_id
FROM platform_credentials
WHERE platform = 'twitter'
  AND webhook_route_key IS NULL
  AND app_bearer_token IS NOT NULL
  AND app_bearer_token <> ''
  AND consumer_secret IS NOT NULL
  AND consumer_secret <> ''
ORDER BY workspace_id;

-- name: SetTwitterWebhookRouteKeyIfMissing :exec
UPDATE platform_credentials
SET webhook_route_key = $3
WHERE workspace_id = $1
  AND platform = 'twitter'
  AND client_id = $2
  AND webhook_route_key IS NULL;
