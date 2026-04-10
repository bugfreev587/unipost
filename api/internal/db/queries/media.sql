-- name: CreateMedia :one
INSERT INTO media (workspace_id, storage_key, content_type, size_bytes, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetMedia :one
SELECT * FROM media WHERE id = $1;

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
UPDATE media
SET status = 'uploaded',
    size_bytes = $2,
    content_type = $3,
    uploaded_at = NOW()
WHERE id = $1
RETURNING *;

-- name: SoftDeleteMedia :exec
UPDATE media SET status = 'deleted'
WHERE id = $1 AND workspace_id = $2;

-- name: HardDeleteMedia :exec
DELETE FROM media
WHERE id = $1;

-- name: ListAbandonedMedia :many
SELECT * FROM media
WHERE status = 'pending'
  AND created_at < NOW() - INTERVAL '7 days'
LIMIT 100;
