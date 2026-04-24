-- name: CreateSocialPostResult :one
INSERT INTO social_post_results (post_id, social_account_id, caption, status, external_id, error_message, published_at, url, debug_curl, fb_media_type)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListSocialPostResultsByPost :many
SELECT * FROM social_post_results WHERE post_id = $1;

-- name: ListSocialPostResultsByPostIDs :many
SELECT * FROM social_post_results
WHERE post_id = ANY($1::text[]);

-- name: GetSocialPostResultByIDAndPost :one
SELECT * FROM social_post_results WHERE id = $1 AND post_id = $2;

-- name: UpdateSocialPostResultAfterRetry :one
-- Overwrites the diagnostic columns on a failed result row after a
-- successful or failed per-platform retry, reusing the same row so
-- the UI doesn't grow N rows per retry attempt. debug_curl is always
-- replaced (including with NULL on success) so a published row never
-- carries the curl dump from its last failure.
UPDATE social_post_results
SET
  status = $2,
  external_id = $3,
  error_message = $4,
  published_at = $5,
  url = $6,
  debug_curl = $7
WHERE id = $1
RETURNING *;

-- name: DeleteSocialPostResultsByPost :exec
DELETE FROM social_post_results WHERE post_id = $1;

-- name: CountPublishedThisMonthByAccount :one
-- Sprint 5 PR2: per-account monthly quota enforcement. Counts the
-- successful posts for one social account in the current calendar
-- month (UTC — same boundary projects.usage uses, so the per-project
-- and per-account counts agree). Used at dispatch time, so it must
-- be one indexed lookup; the partial index
-- social_post_results_quota_count_idx is shaped exactly for this
-- WHERE clause. Failed rows (published_at NULL) are excluded both
-- by the WHERE and by the partial index.
SELECT COUNT(*)::INTEGER AS count
FROM social_post_results
WHERE social_account_id = $1
  AND published_at IS NOT NULL
  AND published_at >= date_trunc('month', NOW() AT TIME ZONE 'UTC')
  AND published_at <  date_trunc('month', NOW() AT TIME ZONE 'UTC') + INTERVAL '1 month';

-- name: ListRecentResultsByAccount :many
-- Most recent N results for an account, for the account health
-- endpoint. The PR7 health derivation walks the latest 10 of these
-- to compute "ok" / "degraded" status. published_at is the canonical
-- "when did this happen" timestamp; for failed results without one,
-- callers should fall back to a join against social_posts.created_at
-- (the health handler does the fallback in Go).
SELECT * FROM social_post_results
WHERE social_account_id = $1
ORDER BY published_at DESC NULLS LAST
LIMIT $2;

-- name: ListFacebookVideosAwaitingStatus :many
-- Lists Facebook video results still in 'processing' state so the
-- facebook_video_status worker can re-check them without waiting for
-- a dashboard Get to trigger the existing on-demand re-poll. Joined
-- to the account so the worker has the page id and encrypted token in
-- one round-trip. Scheduled/dispatched posts are included; deleted
-- and disconnected rows are excluded. post_created_at is surfaced so
-- the worker can apply a staleness cap and fail rows that have been
-- processing for absurdly long.
SELECT
  spr.id                     AS social_post_result_id,
  spr.external_id,
  spr.url,
  spr.fb_media_type,
  sa.id                      AS social_account_id,
  sa.external_account_id     AS page_id,
  sa.access_token,
  sp.created_at              AS post_created_at
FROM social_post_results spr
JOIN social_posts sp    ON sp.id = spr.post_id
JOIN social_accounts sa ON sa.id = spr.social_account_id
WHERE spr.status = 'processing'
  AND spr.external_id IS NOT NULL
  AND sa.platform = 'facebook'
  AND sa.disconnected_at IS NULL
  AND sp.deleted_at IS NULL
ORDER BY sp.created_at ASC
LIMIT 100;

-- name: ListPublishedExternalIDsForInboxSync :many
-- Returns platform post external ids for an account that were
-- successfully published via UniPost and whose published_at falls
-- within the given window. The Facebook inbox sync walks this list
-- (instead of all recent Page posts) so we only fetch comments on
-- content the user created through us — matches the Q&A decision
-- to keep FB comment polling scoped to UniPost-managed content.
--
-- remotely_deleted_at IS NULL excludes posts the sync worker has
-- already discovered are gone from the platform side (user deleted
-- them on Facebook). Without that filter the worker would keep
-- hitting "#100 subcode 33" on every tick forever.
SELECT external_id
FROM social_post_results
WHERE social_account_id = $1
  AND status = 'published'
  AND external_id IS NOT NULL
  AND remotely_deleted_at IS NULL
  AND published_at >= NOW() - ($2::INT * INTERVAL '1 day')
ORDER BY published_at DESC;

-- name: MarkSocialPostResultRemotelyDeleted :exec
-- Records that a previously-published post no longer exists on the
-- platform side. The inbox sync worker calls this when Graph
-- returns "#100 subcode 33" on a comment fetch. Looked up by
-- (account, external_id) since that's the natural key the sync
-- loop already has; no-op if we somehow see the same delete twice.
UPDATE social_post_results
SET remotely_deleted_at = COALESCE(remotely_deleted_at, NOW()),
    error_message       = COALESCE(error_message, $3)
WHERE social_account_id = $1
  AND external_id       = $2;
