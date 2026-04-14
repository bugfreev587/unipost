-- name: UpsertInboxItem :one
INSERT INTO inbox_items (
  social_account_id, workspace_id, source, external_id,
  parent_external_id, author_name, author_id, author_avatar_url,
  body, is_own, received_at, metadata, thread_key, thread_status,
  assigned_to, linked_post_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
ON CONFLICT (social_account_id, external_id) DO NOTHING
RETURNING *;

-- name: ListInboxItemsByWorkspace :many
SELECT * FROM inbox_items
WHERE workspace_id = $1
  AND (sqlc.narg('source')::TEXT IS NULL OR source = sqlc.narg('source')::TEXT)
  AND (sqlc.narg('is_read')::BOOLEAN IS NULL OR is_read = sqlc.narg('is_read')::BOOLEAN)
ORDER BY received_at DESC
LIMIT $2;

-- name: GetInboxItem :one
SELECT * FROM inbox_items
WHERE id = $1 AND workspace_id = $2;

-- name: GetInboxItemByExternalID :one
SELECT * FROM inbox_items
WHERE social_account_id = $1
  AND external_id = $2
LIMIT 1;

-- name: MarkInboxItemRead :exec
UPDATE inbox_items
SET is_read = true
WHERE id = $1 AND workspace_id = $2;

-- name: MarkAllInboxItemsRead :execrows
UPDATE inbox_items
SET is_read = true
WHERE workspace_id = $1 AND is_read = false;

-- name: UpdateInboxThreadState :execrows
UPDATE inbox_items
SET thread_status = $5,
    assigned_to = NULLIF($6, '')
WHERE workspace_id = $1
  AND social_account_id = $2
  AND source = $3
  AND thread_key = $4;

-- name: CountUnreadByWorkspace :one
SELECT COUNT(*)::INTEGER AS count
FROM inbox_items
WHERE workspace_id = $1 AND is_read = false;

-- name: ListInboxItemsByParent :many
SELECT * FROM inbox_items
WHERE social_account_id = $1
  AND parent_external_id = $2
ORDER BY received_at ASC;

-- name: FindLinkedPostIDForInboxParent :one
SELECT spr.post_id
FROM social_post_results spr
WHERE spr.social_account_id = $1
  AND spr.external_id = $2
LIMIT 1;

-- name: FindSocialAccountByPlatformAndExternalID :one
-- Webhook routing: find an active social account by platform + external_account_id,
-- joining to profiles for workspace_id. Returns the first match.
SELECT sa.id, sa.external_account_id, p.workspace_id
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE sa.platform = $1
  AND sa.external_account_id = $2
  AND sa.disconnected_at IS NULL
  AND sa.status = 'active'
LIMIT 1;

-- name: ListAllInboxAccounts :many
-- All active IG/Threads accounts across all workspaces, for the
-- background inbox sync worker. Returns account fields plus workspace_id.
SELECT sa.id, sa.platform, sa.access_token, sa.external_account_id,
       sa.account_name, p.workspace_id
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE sa.disconnected_at IS NULL
  AND sa.status = 'active'
  AND sa.platform IN ('instagram', 'threads')
ORDER BY sa.connected_at DESC;

-- name: FindInboxAccountsByWorkspace :many
-- Distinct social accounts that have inbox items, for the sync handler.
SELECT DISTINCT sa.*
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE p.workspace_id = $1
  AND sa.disconnected_at IS NULL
  AND sa.platform IN ('instagram', 'threads');
