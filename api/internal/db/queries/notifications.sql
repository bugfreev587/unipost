-- Queries for the user-facing notification system.
-- See migration 040_notifications.sql for the schema.

-- ─── Channels ────────────────────────────────────────────────────────

-- name: CreateNotificationChannel :one
INSERT INTO notification_channels (user_id, workspace_id, kind, config, label, verified_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListNotificationChannelsByUser :many
SELECT * FROM notification_channels
WHERE user_id = $1 AND deleted_at IS NULL
ORDER BY created_at ASC;

-- name: GetNotificationChannel :one
SELECT * FROM notification_channels
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: SoftDeleteNotificationChannel :exec
UPDATE notification_channels
SET deleted_at = NOW()
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: MarkNotificationChannelVerified :exec
UPDATE notification_channels
SET verified_at = NOW()
WHERE id = $1 AND user_id = $2;

-- ─── Subscriptions ───────────────────────────────────────────────────

-- name: CreateNotificationSubscription :one
INSERT INTO notification_subscriptions (user_id, workspace_id, event_type, channel_id, enabled, filter)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id, workspace_id, event_type, channel_id)
DO UPDATE SET enabled = EXCLUDED.enabled, filter = EXCLUDED.filter
RETURNING *;

-- name: ListNotificationSubscriptionsByUser :many
SELECT s.*, c.kind AS channel_kind, c.config AS channel_config, c.label AS channel_label
FROM notification_subscriptions s
JOIN notification_channels c ON c.id = s.channel_id
WHERE s.user_id = $1 AND c.deleted_at IS NULL
ORDER BY s.event_type, s.created_at;

-- name: SetNotificationSubscriptionEnabled :exec
UPDATE notification_subscriptions
SET enabled = $3
WHERE id = $1 AND user_id = $2;

-- name: DeleteNotificationSubscription :exec
DELETE FROM notification_subscriptions
WHERE id = $1 AND user_id = $2;

-- ─── Dispatch / fanout ───────────────────────────────────────────────

-- Resolve every subscription that should receive an event, returning
-- the delivery row we'd need to insert. Used by NotificationDispatcher
-- during event.Publish(). A subscription matches when:
--   - its workspace_id matches the firing workspace, OR
--   - workspace_id is NULL (account-level) AND its user_id owns that
--     workspace. The ownership join is critical — without it Alice's
--     account-level sub would fire on Bob's workspace events.
--
-- name: ResolveNotificationTargets :many
SELECT s.id AS subscription_id, s.channel_id, s.event_type, c.kind AS channel_kind
FROM notification_subscriptions s
JOIN notification_channels c ON c.id = s.channel_id
JOIN workspaces w ON w.id = $2
WHERE s.event_type = $1
  AND s.enabled = TRUE
  AND c.deleted_at IS NULL
  AND c.verified_at IS NOT NULL
  AND (s.workspace_id = w.id OR (s.workspace_id IS NULL AND s.user_id = w.user_id));

-- Idempotent insert — if (event_id, channel_id) already exists from a
-- retried publish we silently skip. The UNIQUE constraint on the table
-- enforces one delivery per logical event per channel.
--
-- name: CreateNotificationDelivery :exec
INSERT INTO notification_deliveries (subscription_id, channel_id, event_type, event_id, payload)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (event_id, channel_id) DO NOTHING;

-- ─── Delivery worker ─────────────────────────────────────────────────

-- name: GetPendingNotificationDeliveries :many
SELECT d.*, c.kind AS channel_kind, c.config AS channel_config, c.label AS channel_label
FROM notification_deliveries d
JOIN notification_channels c ON c.id = d.channel_id
WHERE d.status = 'pending'
  AND d.next_retry_at <= NOW()
  AND c.deleted_at IS NULL
ORDER BY d.created_at ASC
LIMIT 100;

-- name: MarkNotificationDeliverySent :exec
UPDATE notification_deliveries
SET status = 'sent', attempts = attempts + 1, delivered_at = NOW(), last_error = NULL
WHERE id = $1;

-- name: ScheduleNotificationDeliveryRetry :exec
UPDATE notification_deliveries
SET attempts = attempts + 1, next_retry_at = $2, last_error = $3
WHERE id = $1;

-- name: MarkNotificationDeliveryDead :exec
UPDATE notification_deliveries
SET status = 'dead', attempts = attempts + 1, last_error = $2, delivered_at = NOW()
WHERE id = $1;
