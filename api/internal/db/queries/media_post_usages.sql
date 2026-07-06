-- name: UpsertMediaPostUsage :exec
INSERT INTO media_post_usages (
  workspace_id,
  media_id,
  post_id,
  post_status,
  cleanup_after_at
) VALUES (
  $1, $2, $3, $4, $5
)
ON CONFLICT (media_id, post_id) DO UPDATE
SET post_status = EXCLUDED.post_status,
    cleanup_after_at = EXCLUDED.cleanup_after_at,
    updated_at = NOW();

-- name: DeleteMediaPostUsagesForPost :exec
DELETE FROM media_post_usages
WHERE post_id = $1;

-- name: DeleteMediaPostUsagesForPostExcept :exec
DELETE FROM media_post_usages
WHERE post_id = sqlc.arg(post_id)
  AND NOT (media_id = ANY(sqlc.arg(media_ids)::text[]));

-- name: CountActiveScheduledPostsByWorkspace :one
SELECT COUNT(*)::integer
FROM social_posts
WHERE workspace_id = $1
  AND status = 'scheduled'
  AND deleted_at IS NULL;

-- name: ListMediaDueForRetentionCleanup :many
SELECT m.*
FROM media m
WHERE m.status != 'deleted'
  AND EXISTS (
    SELECT 1
    FROM media_post_usages due
    WHERE due.media_id = m.id
      AND due.cleanup_after_at IS NOT NULL
      AND due.cleanup_after_at <= NOW()
  )
  AND NOT EXISTS (
    SELECT 1
    FROM media_post_usages blocker
    WHERE blocker.media_id = m.id
      AND (
        blocker.cleanup_after_at IS NULL
        OR blocker.cleanup_after_at > NOW()
      )
  )
ORDER BY (
  SELECT MAX(due.cleanup_after_at)
  FROM media_post_usages due
  WHERE due.media_id = m.id
) ASC
LIMIT $1;
