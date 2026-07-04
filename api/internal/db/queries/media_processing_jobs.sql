-- name: CreateMediaProcessingJob :one
INSERT INTO media_processing_jobs (
  workspace_id,
  kind,
  status,
  input_video_media_id,
  input_audio_media_id,
  output_media_id,
  mode,
  fit,
  video_volume,
  audio_volume,
  audio_start_ms,
  request,
  idempotency_key,
  request_hash
) VALUES (
  $1,
  $2,
  $3,
  $4,
  $5,
  $6,
  $7,
  $8,
  $9,
  $10,
  $11,
  sqlc.arg(request_json)::jsonb,
  $12,
  $13
)
RETURNING *;

-- name: GetMediaProcessingJobByIDAndWorkspace :one
SELECT * FROM media_processing_jobs
WHERE id = $1 AND workspace_id = $2;

-- name: GetMediaProcessingJobByIdempotencyKey :one
SELECT * FROM media_processing_jobs
WHERE workspace_id = $1 AND idempotency_key = $2;

-- name: ClaimMediaProcessingJobs :many
WITH eligible AS (
  SELECT id
  FROM media_processing_jobs
  WHERE status = 'queued'
  ORDER BY created_at ASC, id ASC
  LIMIT sqlc.arg(batch_limit)::int
  FOR UPDATE SKIP LOCKED
)
UPDATE media_processing_jobs j
SET status = 'processing',
    attempts = j.attempts + 1,
    started_at = COALESCE(j.started_at, NOW()),
    updated_at = NOW()
FROM eligible
WHERE j.id = eligible.id
RETURNING j.*;

-- name: MarkMediaProcessingJobSucceeded :one
UPDATE media_processing_jobs
SET status = 'succeeded',
    output_media_id = $2,
    error_code = NULL,
    error_message = NULL,
    retryable = false,
    updated_at = NOW(),
    completed_at = NOW()
WHERE id = $1
RETURNING *;

-- name: MarkMediaProcessingJobFailed :one
UPDATE media_processing_jobs
SET status = 'failed',
    error_code = $2,
    error_message = $3,
    retryable = $4,
    updated_at = NOW(),
    completed_at = NOW()
WHERE id = $1
RETURNING *;

-- name: RequeueMediaProcessingJob :one
UPDATE media_processing_jobs
SET status = 'queued',
    error_code = NULL,
    error_message = NULL,
    retryable = false,
    updated_at = NOW(),
    completed_at = NULL
WHERE id = $1
RETURNING *;
