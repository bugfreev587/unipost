-- name: CreateSocialPost :one
INSERT INTO social_posts (workspace_id, caption, media_urls, status, metadata, scheduled_at, idempotency_key, source, profile_ids)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: SetSocialPostProfileIDs :exec
-- Lazy-populate profile_ids on posts that were created before the
-- source/profile_ids migration landed. Called from the publish/claim
-- paths when the parent row has an empty profile_ids.
UPDATE social_posts
SET profile_ids = $2
WHERE id = $1;

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
SELECT * FROM social_posts
WHERE id = $1
  AND workspace_id = $2
  AND deleted_at IS NULL;

-- name: GetSocialPostByID :one
-- Cross-workspace lookup. Used by the public preview endpoint where
-- the JWT signature IS the authorization (the caller doesn't have
-- a session). Do NOT use from any auth-required handler — those
-- should always join via workspace_id.
SELECT * FROM social_posts
WHERE id = $1
  AND deleted_at IS NULL;

-- name: ListSocialPostsByWorkspace :many
SELECT * FROM social_posts
WHERE workspace_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListSocialPostsFiltered :many
SELECT * FROM social_posts
WHERE workspace_id = $1
  AND deleted_at IS NULL
  AND ($2::text = ''  OR status     = ANY(string_to_array($2, ',')))
  AND ($3::timestamptz IS NULL OR created_at >= $3)
  AND ($4::timestamptz IS NULL OR created_at <  $4)
  AND (created_at, id) < ($5::timestamptz, $6::text)
ORDER BY created_at DESC, id DESC
LIMIT $7;

-- name: UpdateSocialPostStatus :exec
UPDATE social_posts SET status = $2, published_at = $3
WHERE id = $1;

-- name: UpdateSocialPostErrorMetadata :exec
-- Merges an error_summary field into the post's metadata JSONB.
-- Used when the publish loop fails before any result row could be persisted
-- (e.g., FK violation from a deleted social account).
UPDATE social_posts
SET metadata = COALESCE(metadata, '{}'::jsonb) || jsonb_build_object('error_summary', $2::TEXT)
WHERE id = $1;

-- name: SoftDeleteSocialPost :one
UPDATE social_posts
SET deleted_at = NOW(),
    archived_at = NULL
WHERE id = $1
  AND workspace_id = $2
  AND deleted_at IS NULL
RETURNING *;

-- name: ArchiveSocialPost :one
UPDATE social_posts
SET archived_at = NOW()
WHERE id = $1
  AND workspace_id = $2
  AND deleted_at IS NULL
RETURNING *;

-- name: RestoreSocialPost :one
UPDATE social_posts
SET archived_at = NULL
WHERE id = $1
  AND workspace_id = $2
  AND deleted_at IS NULL
RETURNING *;

-- name: GetDueScheduledPosts :many
SELECT * FROM social_posts
WHERE status = 'scheduled'
  AND deleted_at IS NULL
  AND scheduled_at <= NOW()
ORDER BY scheduled_at ASC
LIMIT 100;

-- name: ClaimScheduledPost :one
UPDATE social_posts SET status = 'publishing'
WHERE id = $1 AND status = 'scheduled'
RETURNING *;

-- name: GetScheduledPostsByWorkspace :many
SELECT * FROM social_posts
WHERE workspace_id = $1
  AND deleted_at IS NULL
  AND status = 'scheduled'
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
UPDATE social_posts
SET deleted_at = NOW(),
    archived_at = NULL
WHERE id = $1
  AND workspace_id = $2
  AND deleted_at IS NULL
  AND status = 'draft';

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
