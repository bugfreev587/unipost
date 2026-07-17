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

-- name: RecoverStaleMediaProcessingJobs :many
WITH stale AS MATERIALIZED (
  SELECT job.id, job.workspace_id, job.attempts
  FROM media_processing_jobs job
  WHERE job.status = 'processing'
    AND job.updated_at < NOW() - INTERVAL '5 minutes'
  ORDER BY job.updated_at ASC, job.id ASC
  LIMIT sqlc.arg(batch_limit)::int
  FOR UPDATE SKIP LOCKED
), terminal_deadlines AS (
  SELECT
    stale.id AS job_id,
    NOW() + CASE COALESCE(subscription.plan_id, 'free')
      WHEN 'api' THEN INTERVAL '4 days'
      WHEN 'basic' THEN INTERVAL '8 days'
      WHEN 'growth' THEN INTERVAL '30 days'
      WHEN 'team' THEN INTERVAL '60 days'
      WHEN 'enterprise' THEN INTERVAL '60 days'
      ELSE INTERVAL '2 days'
    END AS cleanup_after_at
  FROM stale
  LEFT JOIN subscriptions subscription
    ON subscription.workspace_id = stale.workspace_id
  WHERE stale.attempts >= 3
), transitioned_usages AS (
  UPDATE media_processing_usages usage
  SET status = 'failed',
      cleanup_after_at = terminal_deadlines.cleanup_after_at,
      updated_at = NOW()
  FROM terminal_deadlines
  WHERE usage.job_id = terminal_deadlines.job_id
    AND usage.status = 'active'
  RETURNING usage.job_id
)
UPDATE media_processing_jobs job
SET status = CASE WHEN stale.attempts < 3 THEN 'retry_wait' ELSE 'failed' END,
    error_code = CASE WHEN stale.attempts < 3 THEN 'processing_timeout' ELSE 'media_processing_worker_lost' END,
    error_message = CASE
      WHEN stale.attempts < 3 THEN 'Media processing worker heartbeat expired; the job will be retried.'
      ELSE 'Media processing worker was lost and the attempt limit was exhausted.'
    END,
    retryable = stale.attempts < 3,
    next_attempt_at = CASE
      WHEN stale.attempts < 3 THEN NOW() + LEAST(
        INTERVAL '5 minutes',
        INTERVAL '30 seconds' * POWER(2, GREATEST(stale.attempts - 1, 0))
      )
      ELSE job.next_attempt_at
    END,
    updated_at = NOW(),
    completed_at = CASE WHEN stale.attempts < 3 THEN NULL ELSE NOW() END
FROM stale
WHERE job.id = stale.id
RETURNING job.*;

-- name: TouchMediaProcessingJobHeartbeat :execrows
UPDATE media_processing_jobs
SET updated_at = NOW()
WHERE id = $1
  AND status = 'processing';

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
