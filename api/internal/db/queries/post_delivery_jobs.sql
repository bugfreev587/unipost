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
-- Reap active jobs whose lease has expired (the owning worker died or
-- stopped renewing). The lease_expires_at IS NULL branch is a fallback for
-- jobs claimed before the lease migration deployed: those keep the old
-- static last_attempt_at cutoff until they drain.
SELECT * FROM post_delivery_jobs
WHERE state IN ('running', 'retrying')
  AND (
    lease_expires_at <= NOW()
    OR (
      lease_expires_at IS NULL
      AND last_attempt_at IS NOT NULL
      AND last_attempt_at <= sqlc.arg('stale_before')::timestamptz
    )
  )
ORDER BY COALESCE(lease_expires_at, last_attempt_at) ASC, id ASC;

-- name: RenewPostDeliveryJobLease :exec
-- Heartbeat: extend the lease while the worker still owns and is working
-- the job. Only touches jobs still in an active state.
UPDATE post_delivery_jobs
SET lease_expires_at = NOW() + make_interval(secs => sqlc.arg('lease_seconds')::int),
    updated_at = NOW()
WHERE id = sqlc.arg('id')
  AND state IN ('running', 'retrying')
  AND lease_owner IS NOT DISTINCT FROM sqlc.arg('lease_owner')
  AND last_attempt_at IS NOT DISTINCT FROM sqlc.arg('last_attempt_at')::timestamptz;

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

-- name: CountActiveDeliveryJobsByWorkspace :one
-- Active count for the queue-depth admission check (rate-limit PRD).
-- "Active" = not yet terminal: dispatch + retry rows the worker still
-- has work to do on. Hits the post_delivery_jobs_workspace_active_idx
-- partial index added in migration 054.
SELECT COUNT(*)::bigint AS active_count
FROM post_delivery_jobs
WHERE workspace_id = $1
  AND state IN ('pending', 'running', 'retrying');

-- name: ClaimPostDispatchJobs :many
-- $1 = batch limit (max jobs to claim in one tick)
-- $2 = per-workspace concurrent cap (max running+retrying allowed
--      per workspace; 0 disables the cap for backward compat)
--
-- The active_per_ws CTE snapshots active counts once per query.
-- The ranked CTE assigns a per-workspace ROW_NUMBER over the
-- pending queue so the per-workspace cap is enforced even within
-- a single batch — admitting the rn-th candidate makes the
-- workspace's in-flight count = active_cnt + rn, so we admit only
-- when (active_cnt + rn) <= cap. With ws_cap=0 the predicate is
-- always satisfied and behavior matches the pre-Phase-2 query.
WITH active_per_ws AS (
  SELECT workspace_id, COUNT(*)::int AS cnt
  FROM post_delivery_jobs
  WHERE state IN ('running', 'retrying')
  GROUP BY workspace_id
),
ranked AS (
  SELECT
    j.id,
    j.created_at,
    j.workspace_id,
    j.social_account_id,
    ROW_NUMBER() OVER (PARTITION BY j.workspace_id ORDER BY j.created_at, j.id) AS rn,
    COALESCE(a.cnt, 0) AS active_cnt
  FROM post_delivery_jobs j
  LEFT JOIN active_per_ws a ON a.workspace_id = j.workspace_id
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
),
eligible AS (
  SELECT id, social_account_id, created_at, rn FROM ranked
  WHERE sqlc.arg('workspace_concurrent_cap')::int = 0
     OR active_cnt + rn <= sqlc.arg('workspace_concurrent_cap')::int
  ORDER BY rn ASC, created_at ASC, id ASC
  LIMIT sqlc.arg('batch_limit')::int
),
locked_jobs AS (
  SELECT j.id, j.social_account_id
  FROM post_delivery_jobs j
  JOIN eligible e ON e.id = j.id
  WHERE j.kind = 'dispatch'
    AND j.state = 'pending'
  ORDER BY e.rn ASC, e.created_at ASC, e.id ASC
  FOR UPDATE OF j SKIP LOCKED
),
locked_accounts AS (
  SELECT sa.id
  FROM social_accounts sa
  WHERE EXISTS (
    SELECT 1
    FROM locked_jobs
    WHERE locked_jobs.social_account_id = sa.id
  )
  ORDER BY sa.id
  FOR UPDATE SKIP LOCKED
),
claimable AS (
  SELECT locked_jobs.id
  FROM locked_jobs
  JOIN locked_accounts ON locked_accounts.id = locked_jobs.social_account_id
)
UPDATE post_delivery_jobs j
SET state = 'running',
    attempts = j.attempts + 1,
    first_claimed_at = COALESCE(j.first_claimed_at, NOW()),
    last_attempt_at = NOW(),
    platform_started_at = NULL,
    finished_at = NULL,
    lease_expires_at = NOW() + make_interval(secs => sqlc.arg('lease_seconds')::int),
    lease_owner = sqlc.arg('lease_owner'),
    updated_at = NOW()
FROM claimable
WHERE j.id = claimable.id
RETURNING j.*;

-- name: ClaimPostRetryJobs :many
-- $1 = batch limit (max jobs to claim in one tick)
-- $2 = per-workspace concurrent cap (see ClaimPostDispatchJobs notes)
WITH active_per_ws AS (
  SELECT workspace_id, COUNT(*)::int AS cnt
  FROM post_delivery_jobs
  WHERE state IN ('running', 'retrying')
  GROUP BY workspace_id
),
ranked AS (
  SELECT
    j.id,
    COALESCE(j.next_run_at, j.created_at) AS sort_key,
    j.workspace_id,
    j.social_account_id,
    ROW_NUMBER() OVER (
      PARTITION BY j.workspace_id
      ORDER BY COALESCE(j.next_run_at, j.created_at), j.id
    ) AS rn,
    COALESCE(a.cnt, 0) AS active_cnt
  FROM post_delivery_jobs j
  LEFT JOIN active_per_ws a ON a.workspace_id = j.workspace_id
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
),
eligible AS (
  SELECT id, social_account_id, sort_key, rn FROM ranked
  WHERE sqlc.arg('workspace_concurrent_cap')::int = 0
     OR active_cnt + rn <= sqlc.arg('workspace_concurrent_cap')::int
  ORDER BY rn ASC, sort_key ASC, id ASC
  LIMIT sqlc.arg('batch_limit')::int
),
locked_jobs AS (
  SELECT j.id, j.social_account_id
  FROM post_delivery_jobs j
  JOIN eligible e ON e.id = j.id
  WHERE j.kind = 'retry'
    AND j.state = 'pending'
    AND (j.next_run_at IS NULL OR j.next_run_at <= NOW())
  ORDER BY e.rn ASC, e.sort_key ASC, e.id ASC
  FOR UPDATE OF j SKIP LOCKED
),
locked_accounts AS (
  SELECT sa.id
  FROM social_accounts sa
  WHERE EXISTS (
    SELECT 1
    FROM locked_jobs
    WHERE locked_jobs.social_account_id = sa.id
  )
  ORDER BY sa.id
  FOR UPDATE SKIP LOCKED
),
claimable AS (
  SELECT locked_jobs.id
  FROM locked_jobs
  JOIN locked_accounts ON locked_accounts.id = locked_jobs.social_account_id
)
UPDATE post_delivery_jobs j
SET state = 'retrying',
    attempts = j.attempts + 1,
    first_claimed_at = COALESCE(j.first_claimed_at, NOW()),
    last_attempt_at = NOW(),
    platform_started_at = NULL,
    finished_at = NULL,
    lease_expires_at = NOW() + make_interval(secs => sqlc.arg('lease_seconds')::int),
    lease_owner = sqlc.arg('lease_owner'),
    updated_at = NOW()
FROM claimable
WHERE j.id = claimable.id
RETURNING j.*;

-- name: MarkPostDeliveryJobPlatformStarted :one
UPDATE post_delivery_jobs
SET platform_started_at = COALESCE(platform_started_at, NOW()),
    updated_at = NOW()
WHERE id = sqlc.arg('id')
  AND state IN ('running', 'retrying')
  AND lease_owner IS NOT DISTINCT FROM sqlc.arg('lease_owner')
  AND last_attempt_at IS NOT DISTINCT FROM sqlc.arg('last_attempt_at')::timestamptz
RETURNING *;

-- name: MarkPostDeliveryJobSucceeded :one
UPDATE post_delivery_jobs
SET state = 'succeeded',
    last_error = NULL,
    failure_stage = NULL,
    error_code = NULL,
    platform_error_code = NULL,
    next_run_at = NULL,
    updated_at = NOW(),
    finished_at = sqlc.arg('finished_at')
WHERE id = sqlc.arg('id')
  AND state IN ('running', 'retrying')
  AND lease_owner IS NOT DISTINCT FROM sqlc.arg('lease_owner')
  AND last_attempt_at IS NOT DISTINCT FROM sqlc.arg('last_attempt_at')::timestamptz
RETURNING *;

-- name: MarkPostDeliveryJobFailed :one
UPDATE post_delivery_jobs
SET state = sqlc.arg('state'),
    failure_stage = sqlc.arg('failure_stage'),
    error_code = sqlc.arg('error_code'),
    platform_error_code = sqlc.arg('platform_error_code'),
    last_error = sqlc.arg('last_error'),
    next_run_at = sqlc.arg('next_run_at'),
    updated_at = NOW(),
    finished_at = CASE
      WHEN sqlc.arg('state') IN ('pending', 'failed', 'dead', 'cancelled') THEN NOW()
      ELSE finished_at
    END
WHERE id = sqlc.arg('id')
  AND state IN ('running', 'retrying')
  AND lease_owner IS NOT DISTINCT FROM sqlc.arg('lease_owner')
  AND last_attempt_at IS NOT DISTINCT FROM sqlc.arg('last_attempt_at')::timestamptz
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
