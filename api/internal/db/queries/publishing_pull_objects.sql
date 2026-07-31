-- name: ReservePublishingPullObjectUsage :one
WITH reserved_object AS (
  INSERT INTO publishing_pull_objects (
    object_key,
    content_type,
    size_bytes,
    cleanup_state
  ) VALUES (
    sqlc.arg(object_key),
    sqlc.arg(content_type),
    sqlc.arg(size_bytes),
    'active'
  )
  ON CONFLICT (object_key) DO UPDATE
  SET content_type = EXCLUDED.content_type,
      size_bytes = EXCLUDED.size_bytes,
      updated_at = NOW()
  WHERE publishing_pull_objects.cleanup_state = 'active'
  RETURNING object_key
)
INSERT INTO publishing_pull_object_usages (
  object_key,
  workspace_id,
  post_id,
  post_status,
  cleanup_after_at,
  retention_reason
)
SELECT
  reserved_object.object_key,
  sqlc.arg(workspace_id),
  sqlc.arg(post_id),
  'publishing',
  NULL,
  'active_post'
FROM reserved_object
RETURNING id;

-- name: AbandonPublishingPullObjectUsage :exec
UPDATE publishing_pull_object_usages
SET post_status = 'failed',
    cleanup_after_at = NOW(),
    retention_reason = 'upload_failed',
    updated_at = NOW()
WHERE id = sqlc.arg(usage_id);

-- name: UpdatePublishingPullObjectUsagesForPost :exec
UPDATE publishing_pull_object_usages
SET post_status = sqlc.arg(post_status),
    cleanup_after_at = sqlc.narg(cleanup_after_at),
    retention_reason = sqlc.arg(retention_reason),
    updated_at = NOW()
WHERE post_id = sqlc.arg(post_id);

-- name: ClaimPublishingPullObjectsDue :many
WITH eligible AS (
  SELECT candidate.object_key
  FROM publishing_pull_objects candidate
  WHERE candidate.cleanup_state IN ('active', 'deleting')
    AND NOT EXISTS (
      SELECT 1
      FROM publishing_pull_object_usages usage
      WHERE usage.object_key = candidate.object_key
        AND (
          usage.cleanup_after_at IS NULL
          OR usage.cleanup_after_at > NOW()
        )
    )
  ORDER BY candidate.created_at ASC, candidate.object_key ASC
  LIMIT sqlc.arg(batch_size)
  FOR UPDATE OF candidate SKIP LOCKED
)
UPDATE publishing_pull_objects candidate
SET cleanup_state = 'deleting',
    updated_at = NOW()
FROM eligible
WHERE candidate.object_key = eligible.object_key
RETURNING candidate.*;

-- name: ReleasePublishingPullObjectClaim :exec
UPDATE publishing_pull_objects
SET cleanup_state = 'active',
    updated_at = NOW()
WHERE object_key = sqlc.arg(object_key)
  AND cleanup_state = 'deleting';

-- name: HardDeletePublishingPullObject :exec
DELETE FROM publishing_pull_objects
WHERE object_key = sqlc.arg(object_key)
  AND cleanup_state = 'deleting';
