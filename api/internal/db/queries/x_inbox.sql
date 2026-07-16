-- name: GetXInboxDeliveryResource :one
SELECT * FROM x_inbox_delivery_resources
WHERE social_account_id = $1;

-- name: UpsertXInboxDeliveryResource :one
INSERT INTO x_inbox_delivery_resources (
  social_account_id,
  filtered_stream_rule_id,
  activity_dm_subscription_id,
  delivery_status,
  last_error,
  last_synced_at
)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (social_account_id) DO UPDATE
SET filtered_stream_rule_id = EXCLUDED.filtered_stream_rule_id,
    activity_dm_subscription_id = EXCLUDED.activity_dm_subscription_id,
    delivery_status = EXCLUDED.delivery_status,
    last_error = EXCLUDED.last_error,
    last_synced_at = EXCLUDED.last_synced_at,
    updated_at = NOW()
RETURNING *;

-- name: UpdateXInboxDeliveryResource :one
UPDATE x_inbox_delivery_resources
SET filtered_stream_rule_id = $2,
    activity_dm_subscription_id = $3,
    delivery_status = $4,
    last_error = $5,
    last_synced_at = $6,
    updated_at = NOW()
WHERE social_account_id = $1
RETURNING *;

-- name: UpdateXInboxFilteredStreamRule :one
UPDATE x_inbox_delivery_resources
SET filtered_stream_rule_id = $2,
    updated_at = NOW()
WHERE social_account_id = $1
RETURNING *;

-- name: UpdateXInboxActivityDMSubscription :one
UPDATE x_inbox_delivery_resources
SET activity_dm_subscription_id = $2,
    updated_at = NOW()
WHERE social_account_id = $1
RETURNING *;

-- name: DeleteXInboxDeliveryResource :exec
DELETE FROM x_inbox_delivery_resources
WHERE social_account_id = $1;
