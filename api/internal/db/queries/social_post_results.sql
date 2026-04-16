-- name: CreateSocialPostResult :one
INSERT INTO social_post_results (post_id, social_account_id, caption, status, external_id, error_message, published_at, url)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListSocialPostResultsByPost :many
SELECT * FROM social_post_results WHERE post_id = $1;

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
