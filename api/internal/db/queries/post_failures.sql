-- name: CreatePostFailure :one
INSERT INTO post_failures (
  post_id,
  social_post_result_id,
  workspace_id,
  social_account_id,
  platform,
  failure_stage,
  error_code,
  platform_error_code,
  message,
  raw_error,
  is_retriable
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListPostFailuresByPost :many
SELECT *
FROM post_failures
WHERE post_id = $1
ORDER BY created_at ASC;
