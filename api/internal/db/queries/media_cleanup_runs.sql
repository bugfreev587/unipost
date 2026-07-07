-- name: CreateMediaCleanupRun :one
INSERT INTO media_cleanup_runs (
  worker_name,
  status,
  started_at,
  next_run_at
) VALUES (
  $1, 'running', $2, $3
)
RETURNING *;

-- name: CompleteMediaCleanupRun :one
UPDATE media_cleanup_runs
SET status = $2,
    finished_at = $3,
    scanned_objects = $4,
    deleted_objects = $5,
    deleted_bytes = $6,
    failed_objects = $7,
    failed_bytes = $8,
    error_summary = $9,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: MarkStaleMediaCleanupRunsFailed :execrows
UPDATE media_cleanup_runs
SET status = 'failed',
    finished_at = NOW(),
    error_summary = 'stale running cleanup recovered on startup',
    updated_at = NOW()
WHERE worker_name = $1
  AND status = 'running'
  AND started_at < $2;

-- name: GetAdminObjectStorageCurrent :one
SELECT
  COUNT(*) FILTER (WHERE status != 'deleted')::bigint AS tracked_objects,
  COUNT(*) FILTER (WHERE status = 'pending')::bigint AS pending_objects,
  COUNT(*) FILTER (WHERE status = 'uploaded')::bigint AS uploaded_objects,
  COALESCE(SUM(size_bytes) FILTER (WHERE status = 'uploaded'), 0)::bigint AS confirmed_tracked_bytes
FROM media;

-- name: GetAdminObjectStoragePeriodAdditions :one
SELECT
  COUNT(*)::bigint AS added_objects,
  COALESCE(SUM(size_bytes) FILTER (WHERE status = 'uploaded'), 0)::bigint AS added_confirmed_bytes
FROM media
WHERE created_at >= $1
  AND created_at < $2;

-- name: GetAdminObjectStorageDueBacklog :one
SELECT
  COUNT(*)::bigint AS due_objects,
  COALESCE(SUM(m.size_bytes), 0)::bigint AS due_bytes
FROM media m
WHERE m.status != 'deleted'
  AND EXISTS (
    SELECT 1
    FROM media_post_usages due
    WHERE due.media_id = m.id
      AND due.cleanup_after_at IS NOT NULL
      AND due.cleanup_after_at <= NOW()
  )
  AND NOT EXISTS (
    SELECT 1
    FROM media_post_usages blocker
    WHERE blocker.media_id = m.id
      AND (
        blocker.cleanup_after_at IS NULL
        OR blocker.cleanup_after_at > NOW()
      )
  );

-- name: GetAdminObjectStorageReferencedObjects :one
SELECT COUNT(DISTINCT m.id)::bigint AS referenced_objects
FROM media m
JOIN media_post_usages mpu ON mpu.media_id = m.id
WHERE m.status != 'deleted'
  AND (
    mpu.cleanup_after_at IS NULL
    OR mpu.cleanup_after_at > NOW()
  );

-- name: GetAdminObjectStorageNextCleanupDeadline :one
SELECT MIN(mpu.cleanup_after_at)::timestamptz AS next_cleanup_deadline_at
FROM media_post_usages mpu
JOIN media m ON m.id = mpu.media_id
WHERE m.status != 'deleted'
  AND mpu.cleanup_after_at > NOW();

-- name: GetAdminObjectStoragePeriodCleanupRuns :one
SELECT
  COALESCE(SUM(deleted_objects), 0)::bigint AS deleted_objects,
  COALESCE(SUM(deleted_bytes), 0)::bigint AS deleted_bytes,
  COUNT(*)::bigint AS cleanup_runs,
  COALESCE(SUM(failed_objects), 0)::bigint AS failed_object_count,
  COUNT(*) FILTER (WHERE status IN ('failed', 'completed_with_errors'))::bigint AS failed_run_count
FROM media_cleanup_runs
WHERE finished_at >= $1
  AND finished_at < $2;

-- name: GetAdminObjectStorageRunningSummary :one
SELECT
  MIN(started_at) FILTER (WHERE started_at >= $1)::timestamptz AS active_run_started_at,
  COUNT(*) FILTER (WHERE started_at < $1)::bigint AS stale_running_runs
FROM media_cleanup_runs
WHERE worker_name = $2
  AND status = 'running';

-- name: ListAdminObjectStorageRecentRuns :many
SELECT *
FROM media_cleanup_runs
WHERE finished_at IS NOT NULL
ORDER BY finished_at DESC, started_at DESC
LIMIT $1;

-- name: GetAdminObjectStorageContentTypes :many
SELECT
  content_type,
  COUNT(*)::bigint AS tracked_objects,
  COALESCE(SUM(size_bytes), 0)::bigint AS confirmed_tracked_bytes
FROM media
WHERE status = 'uploaded'
GROUP BY content_type
ORDER BY confirmed_tracked_bytes DESC, tracked_objects DESC
LIMIT $1;

-- name: GetAdminObjectStorageStatusBreakdown :many
SELECT
  status,
  COUNT(*)::bigint AS tracked_objects,
  COALESCE(SUM(size_bytes) FILTER (WHERE status = 'uploaded'), 0)::bigint AS confirmed_tracked_bytes
FROM media
WHERE status != 'deleted'
GROUP BY status
ORDER BY tracked_objects DESC, status ASC
LIMIT $1;
