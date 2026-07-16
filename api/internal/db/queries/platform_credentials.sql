-- name: CreatePlatformCredential :one
INSERT INTO platform_credentials (
  workspace_id, platform, client_id, client_secret,
  app_bearer_token, consumer_secret
)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (workspace_id, platform) DO UPDATE
SET client_id = EXCLUDED.client_id,
    client_secret = EXCLUDED.client_secret,
    app_bearer_token = CASE
      WHEN sqlc.arg(app_bearer_token_supplied)::BOOLEAN THEN EXCLUDED.app_bearer_token
      ELSE platform_credentials.app_bearer_token
    END,
    consumer_secret = CASE
      WHEN sqlc.arg(consumer_secret_supplied)::BOOLEAN THEN EXCLUDED.consumer_secret
      ELSE platform_credentials.consumer_secret
    END
RETURNING id, platform, client_id, client_secret, created_at, workspace_id,
  app_bearer_token, consumer_secret;

-- name: GetPlatformCredential :one
SELECT id, platform, client_id, client_secret, created_at, workspace_id,
  app_bearer_token, consumer_secret
FROM platform_credentials
WHERE workspace_id = $1 AND platform = $2;

-- name: ListPlatformCredentialsByWorkspace :many
SELECT id, platform, client_id, client_secret, created_at, workspace_id,
  app_bearer_token, consumer_secret
FROM platform_credentials
WHERE workspace_id = $1
ORDER BY platform;

-- name: DeletePlatformCredential :exec
DELETE FROM platform_credentials
WHERE workspace_id = $1 AND platform = $2;
