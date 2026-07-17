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

-- name: CreateAudioOverlayMediaProcessingJob :one
WITH created_job AS (
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
    sqlc.arg(workspace_id),
    'audio_overlay',
    'queued',
    sqlc.arg(input_video_media_id),
    sqlc.arg(input_audio_media_id),
    NULL,
    sqlc.arg(mode),
    sqlc.arg(fit),
    sqlc.arg(video_volume),
    sqlc.arg(audio_volume),
    sqlc.arg(audio_start_ms),
    sqlc.arg(request_json)::jsonb,
    sqlc.narg(idempotency_key),
    sqlc.narg(request_hash)
  )
  RETURNING *
), input_usages AS (
  INSERT INTO media_processing_usages (
    workspace_id,
    job_id,
    media_id,
    role,
    status,
    cleanup_after_at
  )
  SELECT
    created_job.workspace_id,
    created_job.id,
    input.media_id,
    'input',
    'active',
    NULL
  FROM created_job
  CROSS JOIN LATERAL (
    VALUES
      (created_job.input_video_media_id),
      (created_job.input_audio_media_id)
  ) AS input(media_id)
  ON CONFLICT (job_id, media_id, role) DO NOTHING
  RETURNING job_id
)
SELECT created_job.*
FROM created_job;

-- name: CreateGIFMediaProcessingJob :one
WITH created_job AS (
  INSERT INTO media_processing_jobs (
    workspace_id,
    kind,
    status,
    input_media_id,
    request,
    idempotency_key,
    request_hash
  ) VALUES (
    sqlc.arg(workspace_id),
    'gif_to_mp4',
    'queued',
    sqlc.arg(input_media_id),
    sqlc.arg(request_json)::jsonb,
    sqlc.narg(idempotency_key),
    sqlc.narg(request_hash)
  )
  RETURNING *
), input_usage AS (
  INSERT INTO media_processing_usages (
    workspace_id,
    job_id,
    media_id,
    role,
    status,
    cleanup_after_at
  )
  SELECT
    created_job.workspace_id,
    created_job.id,
    created_job.input_media_id,
    'input',
    'active',
    NULL
  FROM created_job
  ON CONFLICT (job_id, media_id, role) DO NOTHING
  RETURNING job_id
)
SELECT created_job.*
FROM created_job
WHERE EXISTS (
  SELECT 1 FROM input_usage WHERE input_usage.job_id = created_job.id
);

-- name: CountActiveMediaProcessingJobsByWorkspace :one
SELECT COUNT(*)::bigint
FROM media_processing_jobs
WHERE workspace_id = $1
  AND status IN ('queued', 'retry_wait', 'processing');

-- name: CountGIFConversionsSince :one
SELECT COUNT(*)::bigint
FROM media_processing_jobs
WHERE workspace_id = sqlc.arg(workspace_id)
  AND kind = 'gif_to_mp4'
  AND created_at >= sqlc.arg(created_since);

-- name: OldestGIFConversionCreatedSince :one
SELECT MIN(created_at)::timestamptz
FROM media_processing_jobs
WHERE workspace_id = sqlc.arg(workspace_id)
  AND kind = 'gif_to_mp4'
  AND created_at >= sqlc.arg(created_since);

-- name: GetMediaProcessingJobByIDAndWorkspace :one
SELECT * FROM media_processing_jobs
WHERE id = $1 AND workspace_id = $2;

-- name: GetMediaProcessingJobByIdempotencyKey :one
SELECT * FROM media_processing_jobs
WHERE workspace_id = $1 AND idempotency_key = $2;

-- name: ClaimMediaProcessingJobsByKind :many
WITH eligible AS (
  SELECT candidate.id
  FROM media_processing_jobs candidate
  WHERE candidate.kind = sqlc.arg(job_kind)
    AND candidate.status = 'queued'
    AND candidate.next_attempt_at <= NOW()
  ORDER BY candidate.next_attempt_at ASC, candidate.created_at ASC, candidate.id ASC
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

-- name: PromoteDueMediaProcessingRetriesByKind :execrows
UPDATE media_processing_jobs
SET status = 'queued',
    updated_at = NOW()
WHERE kind = sqlc.arg(job_kind)
  AND status = 'retry_wait'
  AND next_attempt_at <= NOW();

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
SET status = 'retry_wait',
    error_code = sqlc.arg(error_code),
    error_message = sqlc.arg(error_message),
    retryable = true,
    next_attempt_at = NOW() + LEAST(
      INTERVAL '5 minutes',
      INTERVAL '30 seconds' * POWER(2, GREATEST(attempts - 1, 0))
    ),
    updated_at = NOW(),
    completed_at = NULL
WHERE id = sqlc.arg(job_id)
  AND attempts < 3
RETURNING *;
