-- +goose Up
--
-- Generalize Media Processing for multiple job kinds and add the lifecycle
-- ledger that prevents active inputs/outputs from being physically deleted.

ALTER TABLE media_processing_jobs
  ADD COLUMN input_media_id TEXT,
  ADD COLUMN next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

ALTER TABLE media
  ADD COLUMN usage_version BIGINT NOT NULL DEFAULT 0;

ALTER TABLE media_processing_jobs
  ALTER COLUMN input_video_media_id DROP NOT NULL,
  ALTER COLUMN input_audio_media_id DROP NOT NULL;

ALTER TABLE media_processing_jobs
  DROP CONSTRAINT IF EXISTS media_processing_jobs_kind_check;

ALTER TABLE media_processing_jobs
  DROP CONSTRAINT IF EXISTS media_processing_jobs_status_check;

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

ALTER TABLE media_processing_jobs
  ADD CONSTRAINT media_processing_jobs_status_check CHECK (
    status IN ('queued', 'retry_wait', 'processing', 'succeeded', 'failed', 'cancelled')
  );

CREATE INDEX media_processing_jobs_kind_claim_idx
  ON media_processing_jobs (kind, status, next_attempt_at, created_at, id)
  WHERE status = 'queued';

CREATE INDEX media_processing_jobs_retry_due_idx
  ON media_processing_jobs (kind, next_attempt_at, created_at, id)
  WHERE status = 'retry_wait';

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

-- Usage creation and cleanup both lock the parent Media row. Cleanup claims
-- use SKIP LOCKED, so whichever operation arrives first wins without leaving
-- a window in which an object can be deleted after gaining a new reference.
-- +goose StatementBegin
CREATE FUNCTION protect_media_usage_insert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  media_status TEXT;
BEGIN
  UPDATE media
  SET usage_version = usage_version + 1
  WHERE id = NEW.media_id
    AND status = 'uploaded'
  RETURNING status INTO media_status;
  IF media_status IS NULL THEN
    RAISE EXCEPTION 'media % is not available for a new usage', NEW.media_id USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER media_post_usages_protect_media
BEFORE INSERT OR UPDATE OF media_id ON media_post_usages
FOR EACH ROW EXECUTE FUNCTION protect_media_usage_insert();

CREATE TRIGGER media_processing_usages_protect_media
BEFORE INSERT OR UPDATE OF media_id ON media_processing_usages
FOR EACH ROW EXECUTE FUNCTION protect_media_usage_insert();

-- During a rolling deployment, an old API process can still use the legacy
-- job insert query. Mirror those Audio Overlay inputs into the new ledger.
-- +goose StatementBegin
CREATE FUNCTION track_legacy_media_processing_job_inputs()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.kind = 'audio_overlay' THEN
    INSERT INTO media_processing_usages (
      workspace_id, job_id, media_id, role, status, cleanup_after_at
    )
    SELECT NEW.workspace_id, NEW.id, source.media_id, 'input', 'active', NULL
    FROM (
      VALUES (NEW.input_video_media_id), (NEW.input_audio_media_id)
    ) AS source(media_id)
    WHERE source.media_id IS NOT NULL
    ON CONFLICT (job_id, media_id, role) DO NOTHING;
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER media_processing_jobs_track_legacy_inputs
AFTER INSERT ON media_processing_jobs
FOR EACH ROW EXECUTE FUNCTION track_legacy_media_processing_job_inputs();

-- Old workers represent retryable failures as failed rows. Normalize those
-- writes into retry-wait with the same bounded policy used by new code; old
-- generic claimers only select queued rows and therefore cannot bypass it.
-- +goose StatementBegin
CREATE FUNCTION normalize_legacy_media_processing_retry()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.status = 'failed' AND NEW.retryable THEN
    IF NEW.attempts < 3 THEN
      NEW.status := 'retry_wait';
      NEW.next_attempt_at := NOW() + LEAST(
        INTERVAL '5 minutes',
        INTERVAL '30 seconds' * POWER(2, GREATEST(NEW.attempts - 1, 0))
      );
      NEW.completed_at := NULL;
    ELSE
      NEW.retryable := false;
      NEW.error_code := 'media_processing_attempts_exhausted';
      NEW.error_message := 'processing attempts exhausted: ' || COALESCE(NEW.error_message, 'retryable failure');
    END IF;
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER media_processing_jobs_normalize_legacy_retry
BEFORE UPDATE OF status, retryable ON media_processing_jobs
FOR EACH ROW EXECUTE FUNCTION normalize_legacy_media_processing_retry();

-- Also mirror terminal writes made by an old worker into the lifecycle
-- ledger. GREATEST prevents overlap with new workers from shortening a
-- deadline already calculated by the application.
-- +goose StatementBegin
CREATE FUNCTION transition_legacy_media_processing_usages()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  plan_id TEXT;
  retention_window INTERVAL;
  cleanup_deadline TIMESTAMPTZ;
BEGIN
  IF NEW.status NOT IN ('succeeded', 'failed', 'cancelled') THEN
    RETURN NEW;
  END IF;

  SELECT COALESCE((
    SELECT subscription.plan_id
    FROM subscriptions subscription
    WHERE subscription.workspace_id = NEW.workspace_id
    LIMIT 1
  ), 'free') INTO plan_id;

  IF NEW.status = 'succeeded' THEN
    retention_window := CASE plan_id
      WHEN 'api' THEN INTERVAL '2 days'
      WHEN 'basic' THEN INTERVAL '4 days'
      WHEN 'growth' THEN INTERVAL '15 days'
      WHEN 'team' THEN INTERVAL '30 days'
      WHEN 'enterprise' THEN INTERVAL '30 days'
      ELSE INTERVAL '1 day'
    END;
  ELSE
    retention_window := CASE plan_id
      WHEN 'api' THEN INTERVAL '4 days'
      WHEN 'basic' THEN INTERVAL '8 days'
      WHEN 'growth' THEN INTERVAL '30 days'
      WHEN 'team' THEN INTERVAL '60 days'
      WHEN 'enterprise' THEN INTERVAL '60 days'
      ELSE INTERVAL '2 days'
    END;
  END IF;

  cleanup_deadline := COALESCE(NEW.completed_at, NOW()) + retention_window;

  UPDATE media_processing_usages usage
  SET status = NEW.status,
      cleanup_after_at = GREATEST(
        COALESCE(usage.cleanup_after_at, '-infinity'::timestamptz),
        cleanup_deadline
      ),
      updated_at = NOW()
  WHERE usage.job_id = NEW.id
    AND usage.role = 'input';

  IF NEW.status = 'succeeded' AND NEW.output_media_id IS NOT NULL THEN
    INSERT INTO media_processing_usages (
      workspace_id, job_id, media_id, role, status, cleanup_after_at
    ) VALUES (
      NEW.workspace_id, NEW.id, NEW.output_media_id, 'output', 'succeeded', cleanup_deadline
    )
    ON CONFLICT (job_id, media_id, role) DO UPDATE
    SET status = 'succeeded',
        cleanup_after_at = GREATEST(
          COALESCE(media_processing_usages.cleanup_after_at, '-infinity'::timestamptz),
          EXCLUDED.cleanup_after_at
        ),
        updated_at = NOW();
  END IF;

  RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER media_processing_jobs_transition_legacy_usages
AFTER UPDATE OF status, output_media_id, retryable ON media_processing_jobs
FOR EACH ROW EXECUTE FUNCTION transition_legacy_media_processing_usages();

-- +goose Down
DROP TRIGGER IF EXISTS media_processing_jobs_transition_legacy_usages ON media_processing_jobs;
DROP FUNCTION IF EXISTS transition_legacy_media_processing_usages();
DROP TRIGGER IF EXISTS media_processing_jobs_normalize_legacy_retry ON media_processing_jobs;
DROP FUNCTION IF EXISTS normalize_legacy_media_processing_retry();
DROP TRIGGER IF EXISTS media_processing_jobs_track_legacy_inputs ON media_processing_jobs;
DROP FUNCTION IF EXISTS track_legacy_media_processing_job_inputs();
DROP TRIGGER IF EXISTS media_processing_usages_protect_media ON media_processing_usages;
DROP TRIGGER IF EXISTS media_post_usages_protect_media ON media_post_usages;
DROP FUNCTION IF EXISTS protect_media_usage_insert();

DROP INDEX IF EXISTS media_processing_jobs_retry_due_idx;
DROP INDEX IF EXISTS media_processing_usages_cleanup_due_idx;
DROP INDEX IF EXISTS media_processing_usages_active_media_idx;
DROP INDEX IF EXISTS media_processing_usages_job_idx;
DROP TABLE IF EXISTS media_processing_usages;

DROP INDEX IF EXISTS media_processing_jobs_kind_claim_idx;

UPDATE media_processing_jobs
SET status = 'queued',
    next_attempt_at = NOW()
WHERE status = 'retry_wait';

ALTER TABLE media_processing_jobs
  DROP CONSTRAINT IF EXISTS media_processing_jobs_status_check;

ALTER TABLE media_processing_jobs
  ADD CONSTRAINT media_processing_jobs_status_check
  CHECK (status IN ('queued', 'processing', 'succeeded', 'failed', 'cancelled'));

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
  DROP COLUMN IF EXISTS next_attempt_at,
  DROP COLUMN IF EXISTS input_media_id;

ALTER TABLE media
  DROP COLUMN IF EXISTS usage_version;
