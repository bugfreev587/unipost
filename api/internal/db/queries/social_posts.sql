-- name: CreateSocialPost :one
INSERT INTO social_posts (workspace_id, caption, media_urls, status, metadata, scheduled_at, idempotency_key)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetSocialPostByIdempotencyKey :one
SELECT * FROM social_posts
WHERE workspace_id = $1
  AND idempotency_key = $2
  AND created_at > NOW() - INTERVAL '24 hours';

-- name: ExpireOldIdempotencyKeys :exec
UPDATE social_posts
SET idempotency_key = NULL
WHERE idempotency_key IS NOT NULL
  AND created_at <= NOW() - INTERVAL '24 hours';

-- name: GetSocialPostByIDAndWorkspace :one
SELECT * FROM social_posts WHERE id = $1 AND workspace_id = $2;

-- name: GetSocialPostByID :one
-- Cross-workspace lookup. Used by the public preview endpoint where
-- the JWT signature IS the authorization (the caller doesn't have
-- a session). Do NOT use from any auth-required handler — those
-- should always join via workspace_id.
SELECT * FROM social_posts WHERE id = $1;

-- name: ListSocialPostsByWorkspace :many
SELECT * FROM social_posts
WHERE workspace_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListSocialPostsFiltered :many
SELECT * FROM social_posts
WHERE workspace_id = $1
  AND ($2::text = ''  OR status     = ANY(string_to_array($2, ',')))
  AND ($3::timestamptz IS NULL OR created_at >= $3)
  AND ($4::timestamptz IS NULL OR created_at <  $4)
  AND (created_at, id) < ($5::timestamptz, $6::text)
ORDER BY created_at DESC, id DESC
LIMIT $7;

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

-- name: GetScheduledPostsByWorkspace :many
SELECT * FROM social_posts
WHERE workspace_id = $1 AND status = 'scheduled'
ORDER BY scheduled_at ASC;

-- name: ClaimDraftForPublish :one
UPDATE social_posts
SET status = 'publishing'
WHERE id = $1 AND workspace_id = $2 AND status = 'draft'
RETURNING *;

-- name: UpdateDraftContent :one
UPDATE social_posts
SET caption = $3,
    media_urls = $4,
    metadata = $5,
    scheduled_at = $6
WHERE id = $1 AND workspace_id = $2 AND status = 'draft'
RETURNING *;

-- name: DeleteDraft :exec
DELETE FROM social_posts
WHERE id = $1 AND workspace_id = $2 AND status = 'draft';

-- name: RescheduleSocialPost :one
UPDATE social_posts
SET scheduled_at = $3
WHERE id = $1 AND workspace_id = $2 AND status = 'scheduled'
RETURNING *;

-- name: CancelSocialPost :one
UPDATE social_posts
SET status = 'cancelled'
WHERE id = $1 AND workspace_id = $2 AND status IN ('draft', 'scheduled')
RETURNING *;
