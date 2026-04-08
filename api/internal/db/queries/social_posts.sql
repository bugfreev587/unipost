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

-- name: GetSocialPostByID :one
-- Cross-project lookup. Used by the public preview endpoint where
-- the JWT signature IS the authorization (the caller doesn't have
-- a session). Do NOT use from any auth-required handler — those
-- should always join via project_id.
SELECT * FROM social_posts WHERE id = $1;

-- name: ListSocialPostsByProject :many
SELECT * FROM social_posts
WHERE project_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListSocialPostsFiltered :many
-- Sprint 2 PR7 — keyset pagination + multi-filter list. Each
-- optional filter uses the empty-string / empty-array sentinel
-- pattern (the WHERE clause checks both the slot value and a
-- "filter active" hint) so a single query handles every combo.
--
-- Cursor encoding: the caller passes the (created_at, id) of the
-- last row from the previous page. The WHERE clause uses Postgres
-- tuple comparison (`(created_at, id) < (cursor_at, cursor_id)`)
-- which matches the (created_at DESC, id DESC) index added in
-- migration 019, giving a clean keyset seek.
--
-- For the "no cursor" case (page 1), the caller passes a sentinel
-- max-future timestamp + a high-sorting id; the tuple comparison
-- naturally returns the first page.
SELECT * FROM social_posts
WHERE project_id = $1
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

-- name: GetScheduledPostsByProject :many
SELECT * FROM social_posts
WHERE project_id = $1 AND status = 'scheduled'
ORDER BY scheduled_at ASC;

-- name: ClaimDraftForPublish :one
-- Optimistic lock for the POST /v1/social-posts/{id}/publish
-- transition. Two clients clicking publish simultaneously is the
-- canonical race; the loser sees no rows and returns 409. We
-- restrict to status='draft' so re-publishing an already-published
-- post is also a no-op (the second call gets 0 rows back).
UPDATE social_posts
SET status = 'publishing'
WHERE id = $1 AND project_id = $2 AND status = 'draft'
RETURNING *;

-- name: UpdateDraftContent :one
-- PATCH /v1/social-posts/{id} for drafts. Replaces the canonical
-- caption + media + metadata + scheduled_at in one shot. Refuses to
-- touch non-draft rows so a race against publish can't sneak in
-- under the rug.
UPDATE social_posts
SET caption = $3,
    media_urls = $4,
    metadata = $5,
    scheduled_at = $6
WHERE id = $1 AND project_id = $2 AND status = 'draft'
RETURNING *;

-- name: DeleteDraft :exec
-- DELETE /v1/social-posts/{id} for drafts. Hard delete since drafts
-- never made it out the door — there's no platform state to clean up
-- and no analytics to preserve.
DELETE FROM social_posts
WHERE id = $1 AND project_id = $2 AND status = 'draft';

-- name: RescheduleSocialPost :one
-- Sprint 3 PR8: PATCH /v1/social-posts/{id} for status='scheduled' rows.
-- Only scheduled_at is editable in this state. Optimistic-locked on
-- status='scheduled' so a row that just flipped to 'publishing' (or
-- already published) loses cleanly with pgx.ErrNoRows → 409.
UPDATE social_posts
SET scheduled_at = $3
WHERE id = $1 AND project_id = $2 AND status = 'scheduled'
RETURNING *;

-- name: CancelSocialPost :one
-- Sprint 3 PR8: POST /v1/social-posts/{id}/cancel. Allowed for drafts
-- and scheduled posts; anything else is in-flight or already done and
-- cannot be cancelled. Same optimistic lock pattern as the publish
-- transition. Cancelled rows are filtered out by the scheduler's
-- WHERE status='scheduled' clause on the next tick.
UPDATE social_posts
SET status = 'cancelled'
WHERE id = $1 AND project_id = $2 AND status IN ('draft', 'scheduled')
RETURNING *;
