-- name: CreateWebhook :one
INSERT INTO webhooks (workspace_id, name, url, secret, events, active)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListWebhooksByWorkspace :many
SELECT * FROM webhooks WHERE workspace_id = $1 ORDER BY created_at DESC;

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

-- name: UpdateWebhookDelivery :exec
UPDATE webhook_deliveries
SET status_code = $2, attempts = $3, next_retry_at = $4, delivered_at = $5
WHERE id = $1;

-- name: GetPendingWebhookDeliveries :many
SELECT * FROM webhook_deliveries
WHERE delivered_at IS NULL AND (next_retry_at IS NULL OR next_retry_at <= NOW())
ORDER BY created_at ASC
LIMIT 100;
