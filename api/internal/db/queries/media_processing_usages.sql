-- name: CreateMediaProcessingUsage :one
INSERT INTO media_processing_usages (
  workspace_id,
  job_id,
  media_id,
  role,
  status,
  cleanup_after_at
) VALUES (
  $1,
  $2,
  $3,
  $4,
  $5,
  $6
)
RETURNING *;

-- name: ListMediaProcessingUsagesByJob :many
SELECT *
FROM media_processing_usages
WHERE job_id = $1
ORDER BY created_at ASC, id ASC;

-- name: HasActiveMediaProcessingUsage :one
SELECT EXISTS (
  SELECT 1
  FROM media_processing_usages
  WHERE media_id = $1
    AND status = 'active'
)::boolean;

-- name: TransitionMediaProcessingUsages :exec
UPDATE media_processing_usages
SET status = $2,
    cleanup_after_at = $3,
    updated_at = NOW()
WHERE job_id = $1
  AND status = 'active';

-- name: DeleteMediaProcessingUsagesByJob :exec
DELETE FROM media_processing_usages
WHERE job_id = $1;
