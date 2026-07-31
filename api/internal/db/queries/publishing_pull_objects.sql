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
  retention_reason,
  upload_state,
  upload_lease_expires_at
)
SELECT
  reserved_object.object_key,
  sqlc.arg(workspace_id),
  sqlc.arg(post_id),
  'publishing',
  NULL,
  'active_post',
  'pending',
  NOW() + INTERVAL '15 minutes'
FROM reserved_object
RETURNING id;

-- name: LockPublishingPullObjectForUsageActivation :one
SELECT usage.object_key
FROM publishing_pull_object_usages usage
JOIN publishing_pull_objects object
  ON object.object_key = usage.object_key
WHERE usage.id = sqlc.arg(usage_id)
  AND usage.upload_state = 'pending'
  AND usage.upload_lease_expires_at > NOW()
  AND object.cleanup_state = 'active'
FOR UPDATE OF object;

-- name: MarkPublishingPullObjectUsageActive :one
UPDATE publishing_pull_object_usages
SET upload_state = 'active',
    upload_lease_expires_at = NULL,
    updated_at = NOW()
WHERE id = sqlc.arg(usage_id)
  AND upload_state = 'pending'
  AND upload_lease_expires_at > NOW()
RETURNING id;

-- name: AbandonPublishingPullObjectUsage :exec
UPDATE publishing_pull_object_usages
SET post_status = 'failed',
    cleanup_after_at = NOW(),
    retention_reason = 'upload_failed',
    upload_state = 'abandoned',
    upload_lease_expires_at = NULL,
    updated_at = NOW()
WHERE id = sqlc.arg(usage_id);

-- name: UpdatePublishingPullObjectUsagesForPost :exec
UPDATE publishing_pull_object_usages
SET post_status = sqlc.arg(post_status),
    cleanup_after_at = sqlc.narg(cleanup_after_at),
    retention_reason = sqlc.arg(retention_reason),
    updated_at = NOW()
WHERE post_id = sqlc.arg(post_id);

-- name: LockPublishingPullObjectCandidates :many
SELECT candidate.*
FROM publishing_pull_objects candidate
WHERE candidate.cleanup_state IN ('active', 'deleting')
  AND NOT EXISTS (
    SELECT 1
    FROM publishing_pull_object_usages usage
    WHERE usage.object_key = candidate.object_key
      AND (
        (usage.upload_state = 'pending' AND usage.upload_lease_expires_at > NOW())
        OR (
          usage.upload_state = 'active'
          AND (
            usage.cleanup_after_at IS NULL
            OR usage.cleanup_after_at > NOW()
          )
        )
      )
  )
ORDER BY candidate.created_at ASC, candidate.object_key ASC
LIMIT sqlc.arg(batch_size)
FOR UPDATE OF candidate SKIP LOCKED;

-- name: PublishingPullObjectHasActiveUsage :one
SELECT EXISTS (
  SELECT 1
  FROM publishing_pull_object_usages usage
  WHERE usage.object_key = sqlc.arg(object_key)
    AND (
      (usage.upload_state = 'pending' AND usage.upload_lease_expires_at > NOW())
      OR (
        usage.upload_state = 'active'
        AND (
          usage.cleanup_after_at IS NULL
          OR usage.cleanup_after_at > NOW()
        )
      )
    )
);

-- name: MarkPublishingPullObjectDeleting :one
UPDATE publishing_pull_objects
SET cleanup_state = 'deleting',
    updated_at = NOW()
WHERE object_key = sqlc.arg(object_key)
  AND cleanup_state IN ('active', 'deleting')
RETURNING *;

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
