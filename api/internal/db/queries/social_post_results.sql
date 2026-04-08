-- name: CreateSocialPostResult :one
INSERT INTO social_post_results (post_id, social_account_id, caption, status, external_id, error_message, published_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListSocialPostResultsByPost :many
SELECT * FROM social_post_results WHERE post_id = $1;

-- name: DeleteSocialPostResultsByPost :exec
DELETE FROM social_post_results WHERE post_id = $1;
