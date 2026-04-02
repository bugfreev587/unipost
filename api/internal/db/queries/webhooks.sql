-- name: CreateWebhook :one
INSERT INTO webhooks (project_id, url, secret, events)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListWebhooksByProject :many
SELECT * FROM webhooks WHERE project_id = $1 AND active = true;

-- name: GetWebhook :one
SELECT * FROM webhooks WHERE id = $1;

-- name: DeleteWebhook :exec
UPDATE webhooks SET active = false WHERE id = $1 AND project_id = $2;

-- name: ListWebhooksByProjectAndEvent :many
SELECT * FROM webhooks
WHERE project_id = $1 AND active = true AND @event::text = ANY(events);

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
