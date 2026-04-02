-- name: CreatePlatformCredential :one
INSERT INTO platform_credentials (project_id, platform, client_id, client_secret)
VALUES ($1, $2, $3, $4)
ON CONFLICT (project_id, platform) DO UPDATE
SET client_id = EXCLUDED.client_id, client_secret = EXCLUDED.client_secret
RETURNING *;

-- name: GetPlatformCredential :one
SELECT * FROM platform_credentials
WHERE project_id = $1 AND platform = $2;

-- name: ListPlatformCredentialsByProject :many
SELECT * FROM platform_credentials
WHERE project_id = $1
ORDER BY platform;

-- name: DeletePlatformCredential :exec
DELETE FROM platform_credentials
WHERE project_id = $1 AND platform = $2;
