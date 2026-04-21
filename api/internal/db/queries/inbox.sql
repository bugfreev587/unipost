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
SELECT i.* FROM inbox_items i
JOIN social_accounts sa ON sa.id = i.social_account_id
WHERE i.workspace_id = $1
  AND sa.status = 'active'
  AND sa.disconnected_at IS NULL
  AND (sqlc.narg('source')::TEXT IS NULL OR i.source = sqlc.narg('source')::TEXT)
  AND (sqlc.narg('is_read')::BOOLEAN IS NULL OR i.is_read = sqlc.narg('is_read')::BOOLEAN)
ORDER BY i.received_at DESC
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
FROM inbox_items i
JOIN social_accounts sa ON sa.id = i.social_account_id
WHERE i.workspace_id = $1
  AND i.is_read = false
  AND sa.status = 'active'
  AND sa.disconnected_at IS NULL;

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

-- name: CleanupStaleInboxItems :execrows
-- Cron cleanup: delete inbox items for accounts that have been
-- disconnected for more than 7 days.
DELETE FROM inbox_items
WHERE social_account_id IN (
  SELECT id FROM social_accounts
  WHERE disconnected_at IS NOT NULL
    AND disconnected_at < NOW() - INTERVAL '7 days'
);

-- name: FindDMThreadKeyBySender :one
-- Find the thread_key and parent_external_id for an existing DM
-- conversation with a given sender, so webhook-delivered messages
-- can join the same thread as sync-fetched ones.
SELECT thread_key, parent_external_id, author_name
FROM inbox_items
WHERE social_account_id = $1
  AND source = 'ig_dm'
  AND author_id = $2
  AND thread_key != ''
ORDER BY received_at DESC
LIMIT 1;

-- name: GetInboxMediaCache :one
SELECT media_url, caption, timestamp, media_type, permalink, fetched_at
FROM inbox_media_cache
WHERE social_account_id = $1
  AND external_id = $2;

-- name: UpsertInboxMediaCache :exec
INSERT INTO inbox_media_cache (
  social_account_id, external_id, media_url, caption, timestamp, media_type, permalink, fetched_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
ON CONFLICT (social_account_id, external_id) DO UPDATE SET
  media_url = EXCLUDED.media_url,
  caption = EXCLUDED.caption,
  timestamp = EXCLUDED.timestamp,
  media_type = EXCLUDED.media_type,
  permalink = EXCLUDED.permalink,
  fetched_at = NOW();

-- name: ReconcileDMThreadKeys :execrows
-- When sync discovers the canonical conversation ID for a DM thread,
-- update any existing items (e.g. from webhooks) that used a fallback
-- thread_key (sender ID) so all messages share the same thread_key.
UPDATE inbox_items
SET thread_key = $3,
    parent_external_id = $4
WHERE social_account_id = $1
  AND source = 'ig_dm'
  AND thread_key = $2
  AND thread_key != $3;

-- name: FindAnyActiveAccountByPlatform :one
-- Webhook fallback: find any active account for a platform.
-- Used when Meta sends a different ID format than what we store.
SELECT sa.id, sa.external_account_id, p.workspace_id
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE sa.platform = $1
  AND sa.disconnected_at IS NULL
  AND sa.status = 'active'
ORDER BY sa.connected_at DESC
LIMIT 1;

-- name: FindAllActiveAccountsByPlatform :many
-- Returns ALL active accounts for a platform across all workspaces.
-- Used by webhooks to fan out comments/replies to every workspace.
SELECT sa.id, sa.external_account_id, p.workspace_id
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE sa.platform = $1
  AND sa.disconnected_at IS NULL
  AND sa.status = 'active'
ORDER BY sa.connected_at DESC;

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
-- All active IG / Threads / Facebook accounts across all workspaces,
-- for the background inbox sync worker. Returns account fields plus
-- workspace_id.
SELECT sa.id, sa.platform, sa.access_token, sa.external_account_id,
       sa.account_name, p.workspace_id
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE sa.disconnected_at IS NULL
  AND sa.status = 'active'
  AND sa.platform IN ('instagram', 'threads', 'facebook')
ORDER BY sa.connected_at DESC;

-- name: FindInboxAccountsByWorkspace :many
-- Distinct social accounts that have inbox items, for the sync handler.
SELECT DISTINCT sa.*
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE p.workspace_id = $1
  AND sa.disconnected_at IS NULL
  AND sa.platform IN ('instagram', 'threads', 'facebook');

-- name: FindAllSocialAccountsByPlatformAndExternalID :many
-- Webhook routing: find every active social account for platform +
-- external_account_id, joining to profiles for workspace_id.
SELECT sa.id, sa.external_account_id, p.workspace_id
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE sa.platform = $1
  AND sa.external_account_id = $2
  AND sa.disconnected_at IS NULL
  AND sa.status = 'active'
ORDER BY sa.connected_at DESC;
