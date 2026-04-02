-- name: CreateSocialPost :one
INSERT INTO social_posts (project_id, caption, media_urls, status, metadata)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

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
