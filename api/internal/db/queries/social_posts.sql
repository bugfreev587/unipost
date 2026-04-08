-- name: CreateSocialPost :one
INSERT INTO social_posts (project_id, caption, media_urls, status, metadata, scheduled_at, idempotency_key)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetSocialPostByIdempotencyKey :one
-- Returns the row that already used this idempotency key for the
-- project, IF it was created within the 24h conceptual TTL. Beyond
-- that window the index entry is GC'd by a nightly worker, so this
-- query naturally returns no rows.
SELECT * FROM social_posts
WHERE project_id = $1
  AND idempotency_key = $2
  AND created_at > NOW() - INTERVAL '24 hours';

-- name: ExpireOldIdempotencyKeys :exec
-- Nullifies idempotency_key on rows older than 24h so the partial
-- unique index stays small. Run this from a periodic worker; it's
-- idempotent and safe to run on every tick.
UPDATE social_posts
SET idempotency_key = NULL
WHERE idempotency_key IS NOT NULL
  AND created_at <= NOW() - INTERVAL '24 hours';

-- name: GetSocialPostByIDAndProject :one
SELECT * FROM social_posts WHERE id = $1 AND project_id = $2;

-- name: ListSocialPostsByProject :many
SELECT * FROM social_posts
WHERE project_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateSocialPostStatus :exec
UPDATE social_posts SET status = $2, published_at = $3
WHERE id = $1;

-- name: DeleteSocialPost :exec
DELETE FROM social_posts WHERE id = $1;

-- name: GetDueScheduledPosts :many
SELECT * FROM social_posts
WHERE status = 'scheduled' AND scheduled_at <= NOW()
ORDER BY scheduled_at ASC
LIMIT 100;

-- name: ClaimScheduledPost :one
UPDATE social_posts SET status = 'publishing'
WHERE id = $1 AND status = 'scheduled'
RETURNING *;

-- name: GetScheduledPostsByProject :many
SELECT * FROM social_posts
WHERE project_id = $1 AND status = 'scheduled'
ORDER BY scheduled_at ASC;
