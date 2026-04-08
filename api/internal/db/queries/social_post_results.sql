-- name: CreateSocialPostResult :one
INSERT INTO social_post_results (post_id, social_account_id, caption, status, external_id, error_message, published_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListSocialPostResultsByPost :many
SELECT * FROM social_post_results WHERE post_id = $1;

-- name: DeleteSocialPostResultsByPost :exec
DELETE FROM social_post_results WHERE post_id = $1;

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
