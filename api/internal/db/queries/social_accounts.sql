-- name: CreateSocialAccount :one
INSERT INTO social_accounts (project_id, platform, access_token, refresh_token, token_expires_at, external_account_id, account_name, account_avatar_url, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListSocialAccountsByProject :many
SELECT * FROM social_accounts
WHERE project_id = $1 AND disconnected_at IS NULL
ORDER BY connected_at DESC;

-- name: ListSocialAccountsByProjectFiltered :many
-- Sprint 3 PR1: list with optional external_user_id and platform filters.
-- Sprint 3 exit gate step 4 requires the external_user_id filter so the
-- customer can look up the row created by a Connect flow.
SELECT * FROM social_accounts
WHERE project_id = $1
  AND disconnected_at IS NULL
  AND (sqlc.narg('external_user_id')::TEXT IS NULL OR external_user_id = sqlc.narg('external_user_id')::TEXT)
  AND (sqlc.narg('platform')::TEXT IS NULL OR platform = sqlc.narg('platform')::TEXT)
ORDER BY connected_at DESC;

-- name: ListAllSocialAccountsByProject :many
-- Includes disconnected accounts. Used when resolving the platform for
-- historical post results, where the originating account may have been
-- disconnected after publishing.
SELECT * FROM social_accounts
WHERE project_id = $1
ORDER BY connected_at DESC;

-- name: GetSocialAccount :one
SELECT * FROM social_accounts WHERE id = $1;

-- name: GetSocialAccountByIDAndProject :one
SELECT * FROM social_accounts WHERE id = $1 AND project_id = $2;

-- name: DisconnectSocialAccount :one
UPDATE social_accounts SET disconnected_at = NOW()
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: GetExpiringTokens :many
SELECT * FROM social_accounts
WHERE disconnected_at IS NULL
  AND token_expires_at IS NOT NULL
  AND token_expires_at < NOW() + INTERVAL '24 hours';

-- name: UpdateSocialAccountTokens :exec
UPDATE social_accounts
SET access_token = $2, refresh_token = $3, token_expires_at = $4
WHERE id = $1;

-- name: UpsertManagedSocialAccount :one
-- Sprint 3 PR1: re-connect target for OAuth-flow Connect (Twitter, LinkedIn).
-- Reuses an existing row when (project_id, platform, external_user_id)
-- already exists, so historical post_results FK references stay intact.
-- The partial unique index excludes Bluesky — Bluesky upsert detection
-- happens in app code via GetManagedBlueskyAccount.
INSERT INTO social_accounts (
  project_id, platform, access_token, refresh_token, token_expires_at,
  external_account_id, account_name, account_avatar_url, metadata, scope,
  connection_type, connect_session_id, external_user_id, external_user_email,
  status, last_refreshed_at
)
VALUES (
  $1, $2, $3, $4, $5,
  $6, $7, $8, $9, $10,
  'managed', $11, $12, $13,
  'active', NOW()
)
ON CONFLICT (project_id, platform, external_user_id)
  WHERE external_user_id IS NOT NULL AND platform <> 'bluesky'
DO UPDATE SET
  access_token       = EXCLUDED.access_token,
  refresh_token      = EXCLUDED.refresh_token,
  token_expires_at   = EXCLUDED.token_expires_at,
  external_account_id= EXCLUDED.external_account_id,
  account_name       = EXCLUDED.account_name,
  account_avatar_url = EXCLUDED.account_avatar_url,
  metadata           = EXCLUDED.metadata,
  scope              = EXCLUDED.scope,
  connect_session_id = EXCLUDED.connect_session_id,
  external_user_email= EXCLUDED.external_user_email,
  status             = 'active',
  disconnected_at    = NULL,
  last_refreshed_at  = NOW()
RETURNING *;

-- name: GetManagedBlueskyAccount :one
-- Bluesky-specific upsert lookup. Bluesky allows the same external_user_id
-- to map to multiple handles, so app code looks up by (project_id,
-- external_account_id) — the handle/DID is the unique identity.
SELECT * FROM social_accounts
WHERE project_id = $1
  AND platform = 'bluesky'
  AND external_account_id = $2
  AND disconnected_at IS NULL
LIMIT 1;

-- name: UpdateManagedBlueskyAccount :one
-- Companion to GetManagedBlueskyAccount. Refreshes the encrypted app
-- password and account metadata for an existing Bluesky managed row.
UPDATE social_accounts
SET access_token       = $2,
    account_name       = $3,
    account_avatar_url = $4,
    external_user_id   = $5,
    external_user_email= $6,
    connect_session_id = $7,
    status             = 'active',
    disconnected_at    = NULL,
    last_refreshed_at  = NOW()
WHERE id = $1
RETURNING *;

-- name: MarkSocialAccountReconnectRequired :exec
-- Used by the token refresh worker (PR7) when a managed account's
-- refresh token is rejected by the platform. The dashboard / API
-- response surfaces this as status='reconnect_required'.
UPDATE social_accounts
SET status = 'reconnect_required'
WHERE id = $1;

-- name: UpdateManagedTokenRefresh :exec
-- Refresh worker happy path: stash new tokens + bump last_refreshed_at.
-- Does not fire any webhook on its own (the worker decides whether to
-- emit events; per Sprint 3 decision #5 the success path is silent).
UPDATE social_accounts
SET access_token      = $2,
    refresh_token     = $3,
    token_expires_at  = $4,
    last_refreshed_at = NOW()
WHERE id = $1;

-- name: ListManagedAccountsDueForRefresh :many
-- Refresh worker query. FOR UPDATE SKIP LOCKED so multiple API
-- instances ticking concurrently each get a disjoint slice.
-- Excludes Bluesky (no token_expires_at) and the BYO accounts
-- managed by the legacy worker.
SELECT *
FROM social_accounts
WHERE connection_type = 'managed'
  AND status = 'active'
  AND token_expires_at IS NOT NULL
  AND token_expires_at < NOW() + INTERVAL '30 minutes'
  AND platform <> 'bluesky'
ORDER BY token_expires_at ASC
LIMIT 50
FOR UPDATE SKIP LOCKED;
