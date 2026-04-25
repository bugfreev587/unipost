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
-- Hide dead/failed jobs whose social_post_result has already moved
-- past the failure (published via a later retry, or re-dispatching).
-- A dead dispatch job for a result that's now 'published' is history,
-- not something the queue page should offer "Retry now" on.
--
-- Also hide dismissed jobs by default; pass include_dismissed=true
-- to show the archive (no UI for that today; reserved for future
-- restore flow).
SELECT j.* FROM post_delivery_jobs j
LEFT JOIN social_post_results r ON r.id = j.social_post_result_id
WHERE j.workspace_id = $1
  AND (
    sqlc.narg('states')::text IS NULL
    OR j.state = ANY(string_to_array(sqlc.narg('states')::text, ','))
  )
  AND (
    j.state NOT IN ('dead', 'failed')
    OR COALESCE(r.status, 'failed') = 'failed'
  )
  AND (
    COALESCE(sqlc.narg('include_dismissed')::boolean, FALSE) = TRUE
    OR j.dismissed_at IS NULL
  )
ORDER BY j.created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetPostDeliveryJobsSummaryByWorkspace :one
-- dead_count mirrors the ListPostDeliveryJobsByWorkspace filter so the
-- card total and the table agree: a dead dispatch superseded by a
-- succeeded retry shouldn't show up in either surface, and dismissed
-- jobs are excluded so the dead-count tile follows the table.
SELECT
  COUNT(*) FILTER (WHERE j.state = 'pending'  AND j.dismissed_at IS NULL)::bigint AS pending_count,
  COUNT(*) FILTER (WHERE j.state = 'running'  AND j.dismissed_at IS NULL)::bigint AS running_count,
  COUNT(*) FILTER (WHERE j.state = 'retrying' AND j.dismissed_at IS NULL)::bigint AS retrying_count,
  COUNT(*) FILTER (
    WHERE j.state = 'dead'
      AND j.dismissed_at IS NULL
      AND COALESCE(r.status, 'failed') = 'failed'
  )::bigint AS dead_count,
  COUNT(*) FILTER (
    WHERE j.state = 'succeeded'
      AND j.kind = 'retry'
      AND j.finished_at >= date_trunc('day', NOW())
  )::bigint AS recovered_today_count
FROM post_delivery_jobs j
LEFT JOIN social_post_results r ON r.id = j.social_post_result_id
WHERE j.workspace_id = $1;

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

-- name: DismissPostDeliveryJob :one
-- User-driven archive of a dead delivery job. Idempotent: dismissing
-- an already-dismissed job is a no-op that returns the existing row.
-- We restrict to terminal states ('dead', 'failed', 'cancelled') so
-- a user can't accidentally hide an active delivery from the queue.
UPDATE post_delivery_jobs
SET dismissed_at = COALESCE(dismissed_at, NOW()),
    updated_at = NOW()
WHERE id = $1
  AND workspace_id = $2
  AND state IN ('dead', 'failed', 'cancelled')
RETURNING *;

-- name: AutoDismissOldDeadDeliveryJobs :exec
-- Auto-archive dead jobs whose terminal point (finished_at, falling
-- back to updated_at for legacy rows) is older than the supplied
-- threshold. Run periodically by a worker so workspaces that never
-- click Dismiss don't accumulate stale dead rows forever.
UPDATE post_delivery_jobs
SET dismissed_at = NOW(),
    updated_at = NOW()
WHERE state = 'dead'
  AND dismissed_at IS NULL
  AND COALESCE(finished_at, updated_at) < sqlc.arg('older_than')::timestamptz;
