-- name: UpsertMediaPostUsage :one
WITH locked_media AS MATERIALIZED (
  UPDATE media parent
  SET usage_version = usage_version + 1
  WHERE parent.id = sqlc.arg(media_id)
    AND parent.workspace_id = sqlc.arg(workspace_id)
    AND parent.status = 'uploaded'
  RETURNING parent.id
), updated_usage AS (
  UPDATE media_post_usages usage
  SET post_status = sqlc.arg(post_status),
      cleanup_after_at = sqlc.narg(cleanup_after_at),
      updated_at = NOW()
  FROM locked_media
  WHERE usage.media_id = locked_media.id
    AND usage.post_id = sqlc.arg(post_id)
  RETURNING usage.id
), inserted_usage AS (
  INSERT INTO media_post_usages (
    workspace_id, media_id, post_id, post_status, cleanup_after_at
  )
  SELECT
    sqlc.arg(workspace_id),
    sqlc.arg(media_id),
    sqlc.arg(post_id),
    sqlc.arg(post_status),
    sqlc.narg(cleanup_after_at)
  FROM locked_media
  WHERE NOT EXISTS (SELECT 1 FROM updated_usage)
  ON CONFLICT (media_id, post_id) DO UPDATE
  SET post_status = EXCLUDED.post_status,
      cleanup_after_at = EXCLUDED.cleanup_after_at,
      updated_at = NOW()
  RETURNING id
)
SELECT true::boolean AS applied FROM updated_usage
UNION ALL
SELECT true::boolean AS applied FROM inserted_usage
LIMIT 1;

-- name: DeleteMediaPostUsagesForPost :exec
DELETE FROM media_post_usages
WHERE post_id = $1;

-- name: DeleteMediaPostUsagesForPostExcept :exec
DELETE FROM media_post_usages
WHERE post_id = sqlc.arg(post_id)
  AND NOT (media_id = ANY(sqlc.arg(media_ids)::text[]));

-- name: CountActiveScheduledPostsByWorkspace :one
SELECT COUNT(*)::integer
FROM social_posts
WHERE workspace_id = $1
  AND status = 'scheduled'
  AND deleted_at IS NULL;

-- name: ClaimMediaDueForRetentionCleanup :many
WITH snapshot_candidates AS MATERIALIZED (
  SELECT
    m.id,
    m.usage_version,
    GREATEST(
      COALESCE(m.cleanup_after_at, '-infinity'::timestamptz),
      COALESCE((
        SELECT MAX(post_due.cleanup_after_at)
        FROM media_post_usages post_due
        WHERE post_due.media_id = m.id
          AND post_due.cleanup_after_at <= NOW()
      ), '-infinity'::timestamptz),
      COALESCE((
        SELECT MAX(processing_due.cleanup_after_at)
        FROM media_processing_usages processing_due
        WHERE processing_due.media_id = m.id
          AND processing_due.cleanup_after_at <= NOW()
      ), '-infinity'::timestamptz)
    ) AS due_at
  FROM media m
  WHERE (
    m.status = 'deleted'
    OR m.cleanup_after_at <= NOW()
    OR EXISTS (
      SELECT 1
      FROM media_post_usages post_due
      WHERE post_due.media_id = m.id
        AND post_due.cleanup_after_at <= NOW()
    )
    OR EXISTS (
      SELECT 1
      FROM media_processing_usages processing_due
      WHERE processing_due.media_id = m.id
        AND processing_due.cleanup_after_at <= NOW()
    )
  )
  AND (m.cleanup_after_at IS NULL OR m.cleanup_after_at <= NOW())
  AND NOT EXISTS (
    SELECT 1
    FROM media_post_usages post_blocker
    WHERE post_blocker.media_id = m.id
      AND (
        post_blocker.cleanup_after_at IS NULL
        OR post_blocker.cleanup_after_at > NOW()
      )
  )
  AND NOT EXISTS (
    SELECT 1
    FROM media_processing_usages processing_blocker
    WHERE processing_blocker.media_id = m.id
      AND (
        processing_blocker.cleanup_after_at IS NULL
        OR processing_blocker.cleanup_after_at > NOW()
      )
  )
  ORDER BY due_at ASC
  LIMIT $1
), eligible AS (
  SELECT m.id
  FROM media m
  JOIN snapshot_candidates snapshot
    ON snapshot.id = m.id
   AND snapshot.usage_version = m.usage_version
  ORDER BY snapshot.due_at ASC
  FOR UPDATE OF m SKIP LOCKED
)
UPDATE media AS m
SET status = 'deleted',
    cleanup_after_at = NOW()
FROM eligible
WHERE m.id = eligible.id
RETURNING m.*;
