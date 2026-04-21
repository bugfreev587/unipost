-- name: CreatePostDeliveryJob :one
INSERT INTO post_delivery_jobs (
  post_id,
  social_post_result_id,
  workspace_id,
  social_account_id,
  platform,
  post_input_index,
  kind,
  state,
  attempts,
  max_attempts,
  failure_stage,
  error_code,
  platform_error_code,
  last_error,
  next_run_at,
  last_attempt_at,
  finished_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
)
RETURNING *;

-- name: GetPostDeliveryJobByIDAndWorkspace :one
SELECT * FROM post_delivery_jobs
WHERE id = $1 AND workspace_id = $2;

-- name: ListPostDeliveryJobsByPost :many
SELECT * FROM post_delivery_jobs
WHERE post_id = $1
ORDER BY created_at DESC;

-- name: ListPostDeliveryJobsByPostIDs :many
SELECT * FROM post_delivery_jobs
WHERE post_id = ANY($1::text[])
ORDER BY created_at DESC;

-- name: ListPostDeliveryJobsByResult :many
SELECT * FROM post_delivery_jobs
WHERE social_post_result_id = $1
ORDER BY created_at DESC;

-- name: ListStaleActivePostDeliveryJobs :many
SELECT * FROM post_delivery_jobs
WHERE state IN ('running', 'retrying')
  AND last_attempt_at IS NOT NULL
  AND last_attempt_at <= sqlc.arg('stale_before')::timestamptz
ORDER BY last_attempt_at ASC, id ASC;

-- name: ListPostDeliveryJobsByWorkspace :many
SELECT * FROM post_delivery_jobs
WHERE workspace_id = $1
  AND (
    sqlc.narg('states')::text IS NULL
    OR state = ANY(string_to_array(sqlc.narg('states')::text, ','))
  )
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetPostDeliveryJobsSummaryByWorkspace :one
SELECT
  COUNT(*) FILTER (WHERE state = 'pending')::bigint   AS pending_count,
  COUNT(*) FILTER (WHERE state = 'running')::bigint   AS running_count,
  COUNT(*) FILTER (WHERE state = 'retrying')::bigint  AS retrying_count,
  COUNT(*) FILTER (WHERE state = 'dead')::bigint      AS dead_count,
  COUNT(*) FILTER (
    WHERE state = 'succeeded'
      AND kind = 'retry'
      AND finished_at >= date_trunc('day', NOW())
  )::bigint AS recovered_today_count
FROM post_delivery_jobs
WHERE workspace_id = $1;

-- name: ClaimPostDispatchJobs :many
WITH eligible AS (
  SELECT j.id
  FROM post_delivery_jobs j
  WHERE j.kind = 'dispatch'
    AND j.state = 'pending'
    AND NOT EXISTS (
      SELECT 1
      FROM post_delivery_jobs active
      WHERE active.social_account_id = j.social_account_id
        AND active.state IN ('running', 'retrying')
    )
    AND NOT EXISTS (
      SELECT 1
      FROM post_delivery_jobs earlier
      WHERE earlier.social_account_id = j.social_account_id
        AND earlier.kind = 'dispatch'
        AND earlier.state = 'pending'
        AND (
          earlier.created_at < j.created_at
          OR (earlier.created_at = j.created_at AND earlier.id < j.id)
        )
    )
  ORDER BY j.created_at ASC, j.id ASC
  LIMIT $1
  FOR UPDATE SKIP LOCKED
)
UPDATE post_delivery_jobs j
SET state = 'running',
    attempts = j.attempts + 1,
    last_attempt_at = NOW(),
    updated_at = NOW()
FROM eligible
WHERE j.id = eligible.id
RETURNING j.*;

-- name: ClaimPostRetryJobs :many
WITH eligible AS (
  SELECT j.id
  FROM post_delivery_jobs j
  WHERE j.kind = 'retry'
    AND j.state = 'pending'
    AND (j.next_run_at IS NULL OR j.next_run_at <= NOW())
    AND NOT EXISTS (
      SELECT 1
      FROM post_delivery_jobs active
      WHERE active.social_account_id = j.social_account_id
        AND active.state IN ('running', 'retrying')
    )
    AND NOT EXISTS (
      SELECT 1
      FROM post_delivery_jobs earlier
      WHERE earlier.social_account_id = j.social_account_id
        AND earlier.kind = 'retry'
        AND earlier.state = 'pending'
        AND (earlier.next_run_at IS NULL OR earlier.next_run_at <= NOW())
        AND (
          COALESCE(earlier.next_run_at, earlier.created_at) < COALESCE(j.next_run_at, j.created_at)
          OR (
            COALESCE(earlier.next_run_at, earlier.created_at) = COALESCE(j.next_run_at, j.created_at)
            AND earlier.id < j.id
          )
        )
    )
  ORDER BY COALESCE(j.next_run_at, j.created_at) ASC, j.id ASC
  LIMIT $1
  FOR UPDATE SKIP LOCKED
)
UPDATE post_delivery_jobs j
SET state = 'retrying',
    attempts = j.attempts + 1,
    last_attempt_at = NOW(),
    updated_at = NOW()
FROM eligible
WHERE j.id = eligible.id
RETURNING j.*;

-- name: MarkPostDeliveryJobSucceeded :one
UPDATE post_delivery_jobs
SET state = 'succeeded',
    last_error = NULL,
    failure_stage = NULL,
    error_code = NULL,
    platform_error_code = NULL,
    next_run_at = NULL,
    updated_at = NOW(),
    finished_at = NOW()
WHERE id = $1
RETURNING *;

-- name: MarkPostDeliveryJobFailed :one
UPDATE post_delivery_jobs
SET state = $2,
    failure_stage = $3,
    error_code = $4,
    platform_error_code = $5,
    last_error = $6,
    next_run_at = $7,
    updated_at = NOW(),
    finished_at = CASE
      WHEN $2 IN ('dead', 'cancelled') THEN NOW()
      ELSE finished_at
    END
WHERE id = $1
RETURNING *;

-- name: CancelPostDeliveryJob :one
UPDATE post_delivery_jobs
SET state = 'cancelled',
    updated_at = NOW(),
    finished_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteOldSucceededPostDeliveryJobs :exec
DELETE FROM post_delivery_jobs
WHERE state = 'succeeded'
  AND finished_at < NOW() - sqlc.arg('max_age')::interval;
