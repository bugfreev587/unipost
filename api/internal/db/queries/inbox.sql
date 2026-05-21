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
  AND (sqlc.narg('is_own')::BOOLEAN IS NULL OR i.is_own = sqlc.narg('is_own')::BOOLEAN)
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

-- name: UpdateInboxItemAuthorMetadata :execrows
UPDATE inbox_items
SET author_name = NULLIF(@author_name::TEXT, ''),
    author_id = NULLIF(@author_id::TEXT, ''),
    author_avatar_url = NULLIF(@author_avatar_url::TEXT, '')
WHERE id = @id
  AND workspace_id = @workspace_id;

-- name: MergeInboxItemAuthorMetadataByExternalID :execrows
WITH incoming AS (
  SELECT
    NULLIF(@author_name::TEXT, '') AS author_name,
    NULLIF(@author_id::TEXT, '') AS author_id,
    NULLIF(@author_avatar_url::TEXT, '') AS author_avatar_url
)
UPDATE inbox_items AS i
SET
  author_name = CASE
    WHEN incoming.author_name IS NOT NULL
      AND LOWER(incoming.author_name) <> 'facebook user'
      AND (i.author_name IS NULL OR i.author_name = '' OR LOWER(i.author_name) = 'facebook user')
    THEN incoming.author_name
    ELSE i.author_name
  END,
  author_id = CASE
    WHEN incoming.author_id IS NOT NULL
      AND (i.author_id IS NULL OR i.author_id = '')
    THEN incoming.author_id
    ELSE i.author_id
  END,
  author_avatar_url = CASE
    WHEN incoming.author_avatar_url IS NOT NULL
      AND (i.author_avatar_url IS NULL OR i.author_avatar_url = '')
    THEN incoming.author_avatar_url
    ELSE i.author_avatar_url
  END
FROM incoming
WHERE i.social_account_id = @social_account_id
  AND i.external_id = @external_id
  AND (
    (
      incoming.author_name IS NOT NULL
      AND LOWER(incoming.author_name) <> 'facebook user'
      AND (i.author_name IS NULL OR i.author_name = '' OR LOWER(i.author_name) = 'facebook user')
    )
    OR (
      incoming.author_id IS NOT NULL
      AND (i.author_id IS NULL OR i.author_id = '')
    )
    OR (
      incoming.author_avatar_url IS NOT NULL
      AND (i.author_avatar_url IS NULL OR i.author_avatar_url = '')
    )
  );

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
-- Mirrors the inbox UI's "unread" filter exactly: items the user
-- HASN'T read AND wasn't authored by them. Without the is_own = false
-- guard, our own replies (posted via UniPost) get counted as unread
-- even though they never appear in the unread badges on the inbox
-- page itself, and the sidebar count would drift higher than the
-- per-tab counts on the page. is_own is set in the webhook / sync
-- upsert based on author_id == account.external_account_id.
SELECT COUNT(*)::INTEGER AS count
FROM inbox_items i
JOIN social_accounts sa ON sa.id = i.social_account_id
WHERE i.workspace_id = $1
  AND i.is_read = false
  AND i.is_own = false
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
