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

-- name: CompleteMediaProcessingJobSucceeded :one
WITH target_job AS (
  SELECT job_source.id, job_source.workspace_id
  FROM media_processing_jobs job_source
  WHERE job_source.id = sqlc.arg(job_id)
  FOR UPDATE
), transitioned_inputs AS (
  UPDATE media_processing_usages usage
  SET status = 'succeeded',
      cleanup_after_at = sqlc.arg(cleanup_after_at),
      updated_at = NOW()
  FROM target_job
  WHERE usage.job_id = target_job.id
    AND usage.role = 'input'
    AND usage.status = 'active'
  RETURNING usage.id
), output_usage AS (
  INSERT INTO media_processing_usages (
    workspace_id,
    job_id,
    media_id,
    role,
    status,
    cleanup_after_at
  )
  SELECT
    target_job.workspace_id,
    target_job.id,
    sqlc.arg(output_media_id),
    'output',
    'succeeded',
    sqlc.arg(cleanup_after_at)
  FROM target_job
  ON CONFLICT (job_id, media_id, role) DO UPDATE
  SET status = EXCLUDED.status,
      cleanup_after_at = EXCLUDED.cleanup_after_at,
      updated_at = NOW()
  RETURNING job_id
)
UPDATE media_processing_jobs job
SET status = 'succeeded',
    output_media_id = sqlc.arg(output_media_id),
    error_code = NULL,
    error_message = NULL,
    retryable = false,
    updated_at = NOW(),
    completed_at = NOW()
FROM target_job
WHERE job.id = target_job.id
  AND EXISTS (
    SELECT 1
    FROM output_usage
    WHERE output_usage.job_id = job.id
  )
  AND (SELECT COUNT(*) FROM transitioned_inputs) >= 0
RETURNING job.*;

-- name: CompleteMediaProcessingJobFailed :one
WITH transitioned_inputs AS (
  UPDATE media_processing_usages usage
  SET status = 'failed',
      cleanup_after_at = sqlc.arg(cleanup_after_at),
      updated_at = NOW()
  WHERE usage.job_id = sqlc.arg(job_id)
    AND usage.role = 'input'
    AND usage.status = 'active'
  RETURNING usage.id
)
UPDATE media_processing_jobs job
SET status = 'failed',
    error_code = sqlc.arg(error_code),
    error_message = sqlc.arg(error_message),
    retryable = false,
    updated_at = NOW(),
    completed_at = NOW()
WHERE job.id = sqlc.arg(job_id)
  AND (SELECT COUNT(*) FROM transitioned_inputs) >= 0
RETURNING job.*;
