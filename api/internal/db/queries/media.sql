-- name: CreateMedia :one
INSERT INTO media (workspace_id, storage_key, content_type, size_bytes, status, content_hash)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetMedia :one
SELECT * FROM media WHERE id = $1;

-- name: GetMediaByHash :one
SELECT * FROM media
WHERE workspace_id = $1 AND content_hash = $2 AND status = 'uploaded'
LIMIT 1;

-- name: GetActiveMediaByHash :one
SELECT * FROM media
WHERE workspace_id = $1 AND content_hash = $2 AND status != 'deleted'
ORDER BY
  CASE status
    WHEN 'uploaded' THEN 0
    WHEN 'attached' THEN 1
    WHEN 'pending' THEN 2
    ELSE 3
  END,
  created_at DESC
LIMIT 1;

-- name: UpdateMediaStorageKey :one
UPDATE media SET storage_key = $2
WHERE id = $1
RETURNING *;

-- name: GetMediaByIDAndWorkspace :one
SELECT * FROM media WHERE id = $1 AND workspace_id = $2;

-- name: ListMediaByWorkspace :many
SELECT * FROM media
WHERE workspace_id = $1 AND status != 'deleted'
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: MarkMediaUploaded :one
-- Hydrates a pending row after the client's PUT lands. width / height /
-- duration_ms come from storage.ProbeVideo for video uploads (see
-- migration 056); pass NULL for images or for any case where probing
-- couldn't extract them. NULLs leave the columns at NULL — the validator
-- treats unknown dimensions as "warn, don't block" rather than 0×0.
UPDATE media m
SET status = 'uploaded',
    size_bytes = $2,
    content_type = $3,
    width = $4,
    height = $5,
    duration_ms = $6,
    uploaded_at = NOW(),
    cleanup_after_at = GREATEST(
      COALESCE(m.cleanup_after_at, '-infinity'::timestamptz),
      NOW() + CASE COALESCE((
        SELECT subscriptions.plan_id
        FROM subscriptions
        WHERE subscriptions.workspace_id = m.workspace_id
      ), 'free')
        WHEN 'api' THEN INTERVAL '2 days'
        WHEN 'basic' THEN INTERVAL '4 days'
        WHEN 'growth' THEN INTERVAL '15 days'
        WHEN 'team' THEN INTERVAL '30 days'
        WHEN 'enterprise' THEN INTERVAL '30 days'
        ELSE INTERVAL '1 day'
      END
    )
WHERE m.id = $1
RETURNING m.*;

-- name: HasBlockingMediaUsage :one
SELECT (
  EXISTS (
    SELECT 1
    FROM media_post_usages post_usage
    WHERE post_usage.media_id = $1
      AND (
        post_usage.cleanup_after_at IS NULL
        OR post_usage.cleanup_after_at > NOW()
      )
  )
  OR EXISTS (
    SELECT 1
    FROM media_processing_usages processing_usage
    WHERE processing_usage.media_id = $1
      AND (
        processing_usage.cleanup_after_at IS NULL
        OR processing_usage.cleanup_after_at > NOW()
      )
  )
)::boolean;

-- name: SoftDeleteMedia :exec
UPDATE media
SET status = 'deleted',
    cleanup_after_at = NOW()
WHERE id = $1
  AND workspace_id = $2
  AND status != 'deleted';

-- name: SoftDeleteUnusedMedia :execrows
UPDATE media candidate
SET status = 'deleted',
    cleanup_after_at = NOW()
WHERE candidate.id = sqlc.arg(id)
  AND candidate.workspace_id = sqlc.arg(workspace_id)
  AND candidate.status != 'deleted'
  AND NOT EXISTS (
    SELECT 1
    FROM media_post_usages post_blocker
    WHERE post_blocker.media_id = candidate.id
      AND (
        post_blocker.cleanup_after_at IS NULL
        OR post_blocker.cleanup_after_at > NOW()
      )
  )
  AND NOT EXISTS (
    SELECT 1
    FROM media_processing_usages processing_blocker
    WHERE processing_blocker.media_id = candidate.id
      AND (
        processing_blocker.cleanup_after_at IS NULL
        OR processing_blocker.cleanup_after_at > NOW()
      )
  );

-- name: HardDeleteMedia :exec
DELETE FROM media
WHERE id = $1;

-- name: ListAbandonedMedia :many
SELECT * FROM media
WHERE status = 'pending'
  AND created_at < NOW() - INTERVAL '7 days'
LIMIT 100;
