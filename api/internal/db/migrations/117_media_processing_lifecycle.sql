-- +goose Up
--
-- Generalize Media Processing for multiple job kinds and add the lifecycle
-- ledger that prevents active inputs/outputs from being physically deleted.

ALTER TABLE media_processing_jobs
  ADD COLUMN input_media_id TEXT;

ALTER TABLE media_processing_jobs
  ALTER COLUMN input_video_media_id DROP NOT NULL,
  ALTER COLUMN input_audio_media_id DROP NOT NULL;

ALTER TABLE media_processing_jobs
  DROP CONSTRAINT IF EXISTS media_processing_jobs_kind_check;

ALTER TABLE media_processing_jobs
  ADD CONSTRAINT media_processing_jobs_kind_inputs_check CHECK (
    (
      kind = 'audio_overlay'
      AND input_video_media_id IS NOT NULL
      AND input_audio_media_id IS NOT NULL
      AND input_media_id IS NULL
    )
    OR
    (
      kind = 'gif_to_mp4'
      AND input_media_id IS NOT NULL
      AND input_video_media_id IS NULL
      AND input_audio_media_id IS NULL
    )
  );

CREATE INDEX media_processing_jobs_kind_claim_idx
  ON media_processing_jobs (kind, status, created_at, id)
  WHERE status = 'queued';

CREATE TABLE media_processing_usages (
  id               TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id     TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  job_id           TEXT NOT NULL REFERENCES media_processing_jobs(id) ON DELETE CASCADE,
  media_id         TEXT NOT NULL REFERENCES media(id) ON DELETE CASCADE,
  role             TEXT NOT NULL CHECK (role IN ('input', 'output')),
  status           TEXT NOT NULL CHECK (status IN ('active', 'succeeded', 'failed', 'cancelled')),
  cleanup_after_at TIMESTAMPTZ,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (job_id, media_id, role),
  CHECK (
    (status = 'active' AND cleanup_after_at IS NULL)
    OR status <> 'active'
  )
);

CREATE INDEX media_processing_usages_job_idx
  ON media_processing_usages (job_id);

CREATE INDEX media_processing_usages_active_media_idx
  ON media_processing_usages (media_id, job_id)
  WHERE status = 'active';

CREATE INDEX media_processing_usages_cleanup_due_idx
  ON media_processing_usages (cleanup_after_at, media_id)
  WHERE cleanup_after_at IS NOT NULL
    AND status IN ('succeeded', 'failed', 'cancelled');

-- Backfill Audio Overlay inputs and outputs. If an old terminal job has no
-- completion timestamp, start its retention window at migration time rather
-- than creating an already-expired deadline.
WITH migration_clock AS (
  SELECT NOW() AS migrated_at
), lifecycle_rows AS (
  SELECT
    j.workspace_id,
    j.id AS job_id,
    source.media_id,
    source.role,
    CASE
      WHEN j.status IN ('queued', 'processing') THEN 'active'
      ELSE j.status
    END AS lifecycle_status,
    CASE
      WHEN j.status IN ('queued', 'processing') THEN NULL
      WHEN j.status = 'succeeded' THEN
        COALESCE(j.completed_at, migration_clock.migrated_at) +
        CASE COALESCE(sub.plan_id, 'free')
          WHEN 'api' THEN INTERVAL '2 days'
          WHEN 'basic' THEN INTERVAL '4 days'
          WHEN 'growth' THEN INTERVAL '15 days'
          WHEN 'team' THEN INTERVAL '30 days'
          WHEN 'enterprise' THEN INTERVAL '30 days'
          ELSE INTERVAL '1 day'
        END
      ELSE
        COALESCE(j.completed_at, migration_clock.migrated_at) +
        CASE COALESCE(sub.plan_id, 'free')
          WHEN 'api' THEN INTERVAL '4 days'
          WHEN 'basic' THEN INTERVAL '8 days'
          WHEN 'growth' THEN INTERVAL '30 days'
          WHEN 'team' THEN INTERVAL '60 days'
          WHEN 'enterprise' THEN INTERVAL '60 days'
          ELSE INTERVAL '2 days'
        END
    END AS cleanup_after_at
  FROM media_processing_jobs j
  CROSS JOIN migration_clock
  LEFT JOIN subscriptions sub
    ON sub.workspace_id = j.workspace_id
  CROSS JOIN LATERAL (
    VALUES
      (j.input_video_media_id, 'input'::text),
      (j.input_audio_media_id, 'input'::text),
      (j.output_media_id, 'output'::text)
  ) AS source(media_id, role)
  WHERE j.kind = 'audio_overlay'
    AND source.media_id IS NOT NULL
)
INSERT INTO media_processing_usages (
  workspace_id,
  job_id,
  media_id,
  role,
  status,
  cleanup_after_at
)
SELECT DISTINCT
  workspace_id,
  job_id,
  media_id,
  role,
  lifecycle_status,
  cleanup_after_at
FROM lifecycle_rows
ON CONFLICT (job_id, media_id, role) DO NOTHING;

-- Audit historical soft-deleted Media and restore missing active post usages
-- before the new cleanup predicate is allowed to consider deleted rows.
INSERT INTO media_post_usages (
  workspace_id,
  media_id,
  post_id,
  post_status,
  cleanup_after_at
)
SELECT DISTINCT
  sp.workspace_id,
  m.id,
  sp.id,
  sp.status,
  NULL::timestamptz
FROM social_posts sp
CROSS JOIN LATERAL jsonb_array_elements(COALESCE(sp.metadata->'platform_posts', '[]'::jsonb)) platform_post
CROSS JOIN LATERAL jsonb_array_elements_text(COALESCE(platform_post->'media_ids', '[]'::jsonb)) media_ref(media_id)
JOIN media m
  ON m.id = media_ref.media_id
 AND m.workspace_id = sp.workspace_id
WHERE m.status = 'deleted'
  AND sp.deleted_at IS NULL
  AND sp.status IN ('draft', 'scheduled', 'publishing', 'quota_hold')
ON CONFLICT (media_id, post_id) DO UPDATE
SET post_status = EXCLUDED.post_status,
    cleanup_after_at = NULL,
    updated_at = NOW();

-- Start a conservative base success-retention window for existing uploaded
-- Media that has never been attached to a post or processing job. Preserve any
-- later deadline already written by an older policy.
WITH migration_clock AS (
  SELECT NOW() AS migrated_at
)
UPDATE media m
SET cleanup_after_at = GREATEST(
  COALESCE(m.cleanup_after_at, '-infinity'::timestamptz),
  migration_clock.migrated_at +
    CASE COALESCE((
      SELECT sub.plan_id
      FROM subscriptions sub
      WHERE sub.workspace_id = m.workspace_id
    ), 'free')
      WHEN 'api' THEN INTERVAL '2 days'
      WHEN 'basic' THEN INTERVAL '4 days'
      WHEN 'growth' THEN INTERVAL '15 days'
      WHEN 'team' THEN INTERVAL '30 days'
      WHEN 'enterprise' THEN INTERVAL '30 days'
      ELSE INTERVAL '1 day'
    END
)
FROM migration_clock
WHERE m.status = 'uploaded'
  AND NOT EXISTS (
    SELECT 1
    FROM media_post_usages post_usage
    WHERE post_usage.media_id = m.id
  )
  AND NOT EXISTS (
    SELECT 1
    FROM media_processing_usages processing_usage
    WHERE processing_usage.media_id = m.id
  );

-- +goose Down
DROP INDEX IF EXISTS media_processing_usages_cleanup_due_idx;
DROP INDEX IF EXISTS media_processing_usages_active_media_idx;
DROP INDEX IF EXISTS media_processing_usages_job_idx;
DROP TABLE IF EXISTS media_processing_usages;

DROP INDEX IF EXISTS media_processing_jobs_kind_claim_idx;

ALTER TABLE media_processing_jobs
  DROP CONSTRAINT IF EXISTS media_processing_jobs_kind_inputs_check;

-- Deployment A does not expose a GIF insertion path, so all rows are still
-- Audio Overlay rows when this down migration is valid to run.
ALTER TABLE media_processing_jobs
  ALTER COLUMN input_video_media_id SET NOT NULL,
  ALTER COLUMN input_audio_media_id SET NOT NULL;

ALTER TABLE media_processing_jobs
  ADD CONSTRAINT media_processing_jobs_kind_check
  CHECK (kind IN ('audio_overlay'));

ALTER TABLE media_processing_jobs
  DROP COLUMN IF EXISTS input_media_id;
