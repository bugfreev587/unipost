-- name: CreateWebhook :one
INSERT INTO webhooks (workspace_id, name, url, secret, events, active)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListWebhooksByWorkspace :many
SELECT * FROM webhooks WHERE workspace_id = $1 ORDER BY created_at DESC;

-- name: CountActiveWebhooksByWorkspace :one
SELECT COUNT(*)::INTEGER AS total
FROM webhooks
WHERE workspace_id = $1
  AND active = true;

-- name: GetWebhook :one
SELECT * FROM webhooks WHERE id = $1;

-- name: GetWebhookByIDAndWorkspace :one
SELECT * FROM webhooks WHERE id = $1 AND workspace_id = $2;

-- name: DeleteWebhook :exec
UPDATE webhooks SET active = false WHERE id = $1 AND workspace_id = $2;

-- name: HardDeleteWebhook :exec
DELETE FROM webhooks WHERE id = $1 AND workspace_id = $2;

-- name: UpdateWebhookURLEventsActive :one
UPDATE webhooks
SET name   = $3,
    url    = $4,
    events = $5,
    active = $6
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: RotateWebhookSecret :one
UPDATE webhooks SET secret = $3
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: ListWebhooksByWorkspaceAndEvent :many
SELECT * FROM webhooks
WHERE workspace_id = $1 AND active = true AND @event::text = ANY(events);

-- name: CreateWebhookDelivery :one
INSERT INTO webhook_deliveries (webhook_id, event, payload)
VALUES ($1, $2, $3)
RETURNING *;

-- name: CreateWebhookDeliveryWithEventID :exec
WITH input AS (
  SELECT
    CAST(sqlc.arg(webhook_id) AS TEXT) AS webhook_id,
    CAST(sqlc.arg(event) AS TEXT) AS event,
    CAST(sqlc.arg(payload) AS JSONB) AS payload,
    CAST(sqlc.narg(event_id) AS TEXT) AS event_id
), reserved AS (
  INSERT INTO terminal_post_event_webhook_deliveries (event_id, webhook_id)
  SELECT input.event_id, input.webhook_id FROM input
  ON CONFLICT (event_id, webhook_id) DO NOTHING
  RETURNING event_id, webhook_id
), created AS (
  INSERT INTO webhook_deliveries (webhook_id, event, payload)
  SELECT reserved.webhook_id, input.event, input.payload
  FROM reserved
  CROSS JOIN input
  RETURNING id, webhook_id
)
UPDATE terminal_post_event_webhook_deliveries mapping
SET webhook_delivery_id = created.id
FROM created
WHERE mapping.event_id = (SELECT input.event_id FROM input)
  AND mapping.webhook_id = created.webhook_id;

-- name: UpdateWebhookDelivery :exec
UPDATE webhook_deliveries
SET status_code = $2, attempts = $3, next_retry_at = $4, delivered_at = $5
WHERE id = $1;

-- name: GetPendingWebhookDeliveries :many
SELECT * FROM webhook_deliveries
WHERE delivered_at IS NULL AND (next_retry_at IS NULL OR next_retry_at <= NOW())
ORDER BY created_at ASC
LIMIT 100;
