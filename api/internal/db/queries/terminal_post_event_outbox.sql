-- name: CreatePostStatusTransitionEvent :one
INSERT INTO terminal_post_event_outbox (
  post_id,
  workspace_id,
  parent_status,
  parent_version,
  event_type,
  payload
) VALUES (
  sqlc.arg(post_id),
  sqlc.arg(workspace_id),
  sqlc.arg(parent_status),
  sqlc.arg(parent_version),
  CAST(sqlc.arg(event_type) AS TEXT),
  sqlc.arg(payload)
)
ON CONFLICT (post_id, parent_version) DO NOTHING
RETURNING *;

-- name: ElectWorkspaceFirstPostPublished :one
INSERT INTO terminal_post_first_publish_elections (workspace_id, post_id)
SELECT sqlc.arg(workspace_id), sqlc.arg(post_id)
WHERE (
  SELECT COUNT(*)
  FROM social_posts
  WHERE workspace_id = sqlc.arg(workspace_id)
    AND status = 'published'
) = 1
ON CONFLICT (workspace_id) DO NOTHING
RETURNING workspace_id;

-- name: CreateTerminalPostEventDelivery :exec
INSERT INTO terminal_post_event_deliveries (event_id, sink)
VALUES ($1, $2)
ON CONFLICT (event_id, sink) DO NOTHING;

-- name: ClaimPendingTerminalPostEventDeliveries :many
WITH candidates AS (
  SELECT delivery.event_id, delivery.sink
  FROM terminal_post_event_deliveries delivery
  JOIN terminal_post_event_outbox event ON event.id = delivery.event_id
  WHERE (
      delivery.status = 'pending'
      OR (delivery.status = 'processing' AND delivery.lease_expires_at <= NOW())
    )
    AND delivery.next_attempt_at <= NOW()
    AND NOT EXISTS (
      SELECT 1
      FROM terminal_post_event_outbox earlier_event
      JOIN terminal_post_event_deliveries earlier_delivery
        ON earlier_delivery.event_id = earlier_event.id
      WHERE earlier_event.post_id = event.post_id
        AND earlier_event.sequence_number < event.sequence_number
        AND earlier_delivery.status <> 'enqueued'
    )
  ORDER BY event.sequence_number, delivery.sink
  FOR UPDATE OF delivery SKIP LOCKED
  LIMIT $1
), claimed AS (
  UPDATE terminal_post_event_deliveries delivery
  SET status = 'processing',
      attempts = delivery.attempts + 1,
      lease_expires_at = NOW() + INTERVAL '5 minutes',
      last_error = NULL,
      updated_at = NOW()
  FROM candidates
  WHERE delivery.event_id = candidates.event_id
    AND delivery.sink = candidates.sink
  RETURNING delivery.*
)
SELECT
  claimed.event_id,
  claimed.sink,
  claimed.attempts,
  event.workspace_id,
  event.post_id,
  event.parent_status,
  event.parent_version,
  event.event_type,
  event.payload,
  event.sequence_number
FROM claimed
JOIN terminal_post_event_outbox event ON event.id = claimed.event_id
ORDER BY event.sequence_number, claimed.sink;

-- name: MarkTerminalPostEventDeliveryEnqueued :execrows
UPDATE terminal_post_event_deliveries
SET status = 'enqueued',
    enqueued_at = NOW(),
    lease_expires_at = NULL,
    last_error = NULL,
    updated_at = NOW()
WHERE event_id = sqlc.arg(event_id)
  AND sink = sqlc.arg(sink)
  AND status = 'processing'
  AND attempts = sqlc.arg(expected_attempts);

-- name: RetryTerminalPostEventDelivery :execrows
UPDATE terminal_post_event_deliveries
SET status = 'pending',
    next_attempt_at = sqlc.arg(next_attempt_at),
    lease_expires_at = NULL,
    last_error = sqlc.arg(last_error),
    updated_at = NOW()
WHERE event_id = sqlc.arg(event_id)
  AND sink = sqlc.arg(sink)
  AND status = 'processing'
  AND attempts = sqlc.arg(expected_attempts);
