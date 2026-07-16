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

-- name: GetXInboxReplyByIdempotencyKey :one
SELECT * FROM inbox_items
WHERE workspace_id = @workspace_id
  AND social_account_id = @social_account_id
  AND source = @source
  AND is_own = TRUE
  AND metadata->>'reply_to_inbox_item_id' = @reply_to_inbox_item_id::TEXT
  AND metadata->>'idempotency_key' = @idempotency_key::TEXT
ORDER BY created_at DESC
LIMIT 1;

-- name: ClaimXInboxOutboundRequest :one
INSERT INTO x_inbox_outbound_requests (
  workspace_id, social_account_id, inbox_item_id, idempotency_key, payload_hash,
  encrypted_payload, body_hash, reconciliation_deadline
)
VALUES (
  @workspace_id, @social_account_id, @inbox_item_id, @idempotency_key, @payload_hash,
  NULLIF(@encrypted_payload::TEXT, ''), @body_hash, @reconciliation_deadline
)
ON CONFLICT (workspace_id, inbox_item_id, idempotency_key) DO NOTHING
RETURNING *;

-- name: GetXInboxOutboundRequest :one
SELECT *
FROM x_inbox_outbound_requests
WHERE workspace_id = @workspace_id
  AND inbox_item_id = @inbox_item_id
  AND idempotency_key = @idempotency_key;

-- name: GetXInboxOutboundRequestByID :one
SELECT *
FROM x_inbox_outbound_requests
WHERE id = @id;

-- name: GetXInboxOutboundRequestByIDForUpdate :one
SELECT *
FROM x_inbox_outbound_requests
WHERE id = @id
FOR UPDATE;

-- name: ListXInboxOutboundWebhookCandidates :many
SELECT
  o.id AS outbound_request_id,
  o.inbox_item_id,
  o.payload_hash,
  o.send_started_at,
  o.reconciliation_deadline
FROM x_inbox_outbound_requests o
JOIN inbox_items target
  ON target.id = o.inbox_item_id
WHERE o.social_account_id = @social_account_id
  AND o.status IN ('sending', 'outcome_unknown', 'needs_reconciliation')
  AND o.body_hash = @body_hash
  AND o.send_started_at IS NOT NULL
  AND o.reconciliation_deadline IS NOT NULL
  AND @event_at::TIMESTAMPTZ >= o.send_started_at - INTERVAL '5 minutes'
  AND @event_at::TIMESTAMPTZ <= o.reconciliation_deadline
  AND target.source = @source
  AND (
    (@source = 'x_reply' AND target.external_id = @parent_external_id)
    OR (
      @source = 'x_dm'
      AND (
        target.parent_external_id = @thread_key
        OR target.thread_key = @thread_key
      )
    )
  )
ORDER BY o.created_at DESC
FOR UPDATE OF o;

-- name: RecordXInboxOutboundRemoteSuccessFromWebhook :execrows
UPDATE x_inbox_outbound_requests
SET status = 'remote_succeeded',
    remote_external_id = @remote_external_id,
    remote_conversation_id = NULLIF(@remote_conversation_id::TEXT, ''),
    remote_url = NULLIF(@remote_url::TEXT, ''),
    remote_outcome_known_at = NOW(),
    last_error = NULL,
    next_attempt_at = NOW(),
    updated_at = NOW()
WHERE id = @id
  AND status IN ('sending', 'outcome_unknown', 'needs_reconciliation');

-- name: CompleteXInboxOutboundRequest :exec
UPDATE x_inbox_outbound_requests
SET status = 'completed',
    response_inbox_item_id = @response_inbox_item_id,
    updated_at = NOW()
WHERE id = @id
  AND status IN ('pending', 'sending', 'remote_succeeded');

-- name: MarkXInboxOutboundSending :execrows
UPDATE x_inbox_outbound_requests
SET status = 'sending',
    usage_event_id = NULLIF(@usage_event_id::TEXT, ''),
    operation_key = NULLIF(@operation_key::TEXT, ''),
    reserved_units = @reserved_units,
    send_started_at = COALESCE(send_started_at, NOW()),
    updated_at = NOW()
WHERE id = @id
  AND status IN ('pending', 'sending');

-- name: MarkXInboxOutboundUnknown :execrows
UPDATE x_inbox_outbound_requests
SET status = 'outcome_unknown',
    usage_event_id = COALESCE(NULLIF(@usage_event_id::TEXT, ''), usage_event_id),
    operation_key = COALESCE(NULLIF(@operation_key::TEXT, ''), operation_key),
    reserved_units = GREATEST(@reserved_units, reserved_units),
    last_error = LEFT(@last_error::TEXT, 1000),
    next_attempt_at = NOW(),
    updated_at = NOW()
WHERE id = @id
  AND status IN ('pending', 'sending');

-- name: MarkXInboxOutboundUsageReversalPending :execrows
UPDATE x_inbox_outbound_requests
SET status = 'usage_reversal_pending',
    usage_event_id = COALESCE(NULLIF(@usage_event_id::TEXT, ''), usage_event_id),
    operation_key = COALESCE(NULLIF(@operation_key::TEXT, ''), operation_key),
    reserved_units = GREATEST(@reserved_units, reserved_units),
    last_error = LEFT(@last_error::TEXT, 1000),
    next_attempt_at = NOW(),
    updated_at = NOW()
WHERE id = @id
  AND status IN ('pending', 'sending');

-- name: RecordXInboxOutboundRemoteSuccess :execrows
UPDATE x_inbox_outbound_requests
SET status = 'remote_succeeded',
    usage_event_id = COALESCE(NULLIF(@usage_event_id::TEXT, ''), usage_event_id),
    operation_key = COALESCE(NULLIF(@operation_key::TEXT, ''), operation_key),
    reserved_units = GREATEST(@reserved_units, reserved_units),
    remote_external_id = @remote_external_id,
    remote_conversation_id = NULLIF(@remote_conversation_id::TEXT, ''),
    remote_url = NULLIF(@remote_url::TEXT, ''),
    remote_outcome_known_at = NOW(),
    last_error = NULL,
    next_attempt_at = NOW(),
    updated_at = NOW()
WHERE id = @id
  AND status IN ('pending', 'sending', 'remote_succeeded');

-- name: ListRecoverableXInboxOutboundRequests :many
SELECT *
FROM x_inbox_outbound_requests
WHERE (
	status IN ('sending', 'outcome_unknown', 'remote_succeeded', 'usage_reversal_pending')
	OR (status = 'pending' AND encrypted_payload IS NULL)
	OR (
	  status = 'pending'
	  AND encrypted_payload IS NOT NULL
	  AND reconciliation_deadline <= NOW()
	)
  )
  AND next_attempt_at <= NOW()
ORDER BY created_at
LIMIT @row_limit;

-- name: MarkXInboxOutboundNeedsReconciliation :exec
UPDATE x_inbox_outbound_requests
SET status = 'needs_reconciliation',
    last_error = LEFT(@last_error::TEXT, 1000),
    updated_at = NOW()
WHERE id = @id
  AND (
    (status IN ('sending', 'outcome_unknown') AND reconciliation_deadline <= NOW())
    OR (status = 'pending' AND encrypted_payload IS NULL)
  );

-- name: DeferXInboxOutboundCompletion :exec
UPDATE x_inbox_outbound_requests
SET completion_attempts = completion_attempts + 1,
    next_attempt_at = @next_attempt_at,
    last_error = LEFT(@last_error::TEXT, 1000),
    updated_at = NOW()
WHERE id = @id
  AND status = 'remote_succeeded';

-- name: DeferXInboxUsageReversal :exec
UPDATE x_inbox_outbound_requests
SET completion_attempts = completion_attempts + 1,
    next_attempt_at = @next_attempt_at,
    last_error = LEFT(@last_error::TEXT, 1000),
    updated_at = NOW()
WHERE id = @id
  AND status = 'usage_reversal_pending';

-- name: DeferPendingXInboxOutboundRecovery :exec
UPDATE x_inbox_outbound_requests
SET completion_attempts = completion_attempts + 1,
    next_attempt_at = @next_attempt_at,
    last_error = LEFT(@last_error::TEXT, 1000),
    updated_at = NOW()
WHERE id = @id
  AND status = 'pending';

-- name: DeleteXInboxOutboundAfterUsageReversal :execrows
DELETE FROM x_inbox_outbound_requests
WHERE id = @id
  AND status = 'usage_reversal_pending';

-- name: DeletePendingXInboxOutboundRequest :execrows
DELETE FROM x_inbox_outbound_requests
WHERE id = @id
  AND status = 'pending';

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
-- All active Inbox accounts across all workspaces,
-- for the background inbox sync worker. Returns account fields plus
-- workspace eligibility context. X real-time delivery is handled by the
-- dedicated worker; including it here keeps shared Inbox discovery complete.
SELECT sa.id, sa.platform, sa.access_token, sa.external_account_id,
       sa.account_name, p.workspace_id, sa.scope, sa.connection_type,
       sa.x_app_mode, COALESCE(sub.plan_id, 'free') AS plan_id,
       COALESCE(pl.allow_inbox, FALSE) AS plan_allows_inbox
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
LEFT JOIN subscriptions sub ON sub.workspace_id = p.workspace_id
LEFT JOIN plans pl ON pl.id = COALESCE(sub.plan_id, 'free')
WHERE sa.disconnected_at IS NULL
  AND sa.status = 'active'
  AND sa.platform IN ('instagram', 'threads', 'facebook', 'twitter')
ORDER BY sa.connected_at DESC;

-- name: FindInboxAccountsByWorkspace :many
-- Distinct social accounts that have inbox items, for the sync handler.
SELECT DISTINCT sa.id, sa.profile_id, sa.platform, sa.access_token,
       sa.refresh_token, sa.token_expires_at, sa.external_account_id,
       sa.account_name, sa.account_avatar_url, sa.connected_at,
       sa.disconnected_at, sa.metadata, sa.scope, sa.status,
       sa.connection_type, sa.connect_session_id, sa.external_user_id,
       sa.external_user_email, sa.last_refreshed_at, sa.x_app_mode
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE p.workspace_id = $1
  AND sa.disconnected_at IS NULL
  AND sa.platform IN ('instagram', 'threads', 'facebook', 'twitter');

-- name: FindXInboxAccountForApp :one
SELECT
  sa.id,
  p.workspace_id,
  sa.external_user_id,
  sa.external_account_id,
  COALESCE(sa.account_name, '') AS account_name,
  COALESCE(sa.x_app_mode, 'legacy_unknown') AS x_app_mode,
  sa.scope,
  sa.connection_type,
  COALESCE(sub.plan_id, 'free') AS plan_id,
  COALESCE(pl.allow_inbox, FALSE) AS plan_allows_inbox
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
LEFT JOIN subscriptions sub ON sub.workspace_id = p.workspace_id
LEFT JOIN plans pl ON pl.id = COALESCE(sub.plan_id, 'free')
LEFT JOIN platform_credentials pc
  ON pc.workspace_id = p.workspace_id AND pc.platform = 'twitter'
WHERE sa.id = sqlc.arg(account_id)
  AND sa.platform = 'twitter'
  AND sa.disconnected_at IS NULL
  AND sa.status = 'active'
  AND (
    (
      sa.x_app_mode = 'unipost_managed_app'
      AND sqlc.arg(webhook_route_key)::TEXT = sqlc.arg(managed_webhook_route_key)::TEXT
    )
    OR (
      sa.x_app_mode = 'workspace_x_app'
      AND pc.webhook_route_key = sqlc.arg(webhook_route_key)::TEXT
    )
  )
LIMIT 1;

-- name: FindXInboxAccountsForExternalUserApp :many
SELECT
  sa.id,
  p.workspace_id,
  sa.external_user_id,
  sa.external_account_id,
  COALESCE(sa.account_name, '') AS account_name,
  COALESCE(sa.x_app_mode, 'legacy_unknown') AS x_app_mode,
  sa.scope,
  sa.connection_type,
  COALESCE(sub.plan_id, 'free') AS plan_id,
  COALESCE(pl.allow_inbox, FALSE) AS plan_allows_inbox
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
LEFT JOIN subscriptions sub ON sub.workspace_id = p.workspace_id
LEFT JOIN plans pl ON pl.id = COALESCE(sub.plan_id, 'free')
LEFT JOIN platform_credentials pc
  ON pc.workspace_id = p.workspace_id AND pc.platform = 'twitter'
WHERE sa.platform = 'twitter'
  AND (
    sa.external_user_id = sqlc.arg(external_user_id)
    OR sa.external_account_id = sqlc.arg(external_user_id)::TEXT
  )
  AND sa.disconnected_at IS NULL
  AND sa.status = 'active'
  AND (
    (
      sa.x_app_mode = 'unipost_managed_app'
      AND sqlc.arg(webhook_route_key)::TEXT = sqlc.arg(managed_webhook_route_key)::TEXT
    )
    OR (
      sa.x_app_mode = 'workspace_x_app'
      AND pc.webhook_route_key = sqlc.arg(webhook_route_key)::TEXT
    )
  )
ORDER BY sa.connected_at DESC, sa.id;

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
