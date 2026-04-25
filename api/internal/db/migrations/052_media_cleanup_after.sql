-- +goose Up
--
-- Sprint 5: post-publish R2 cleanup for large media uploads.
--
-- The 25 MB upload cap was raised to 4 GB so users can publish
-- long-form video to TikTok / YouTube. The cost guard for that is
-- this column: after a successful publish, the dispatch path sets
-- cleanup_after_at = NOW() + 2h on every consumed media row whose
-- size_bytes exceeds the LARGE_MEDIA_BYTES threshold (200 MB).
-- A dedicated 5-minute ticker (worker.MediaCleanupWorker) hard-
-- deletes the R2 object and the row when due.
--
-- 2h gives pull-mode platforms (Instagram, Facebook, Threads,
-- TikTok) plenty of time to async-fetch the URL we handed them —
-- Meta async video processing typically finishes in under 15 min,
-- but we add a generous safety margin since deleting the file
-- mid-fetch is unrecoverable.
--
-- NULL means "never reaped by this sweeper": small files stay in
-- the user's media library indefinitely. The existing pending-
-- abandonment sweeper still handles the 7-day orphan case
-- separately.

ALTER TABLE media ADD COLUMN cleanup_after_at TIMESTAMPTZ;

-- Sweeper target index. Partial so it stays tiny — most rows have
-- cleanup_after_at = NULL because they're under the threshold.
CREATE INDEX media_cleanup_due_idx ON media (cleanup_after_at)
  WHERE cleanup_after_at IS NOT NULL AND status != 'deleted';

-- +goose Down
DROP INDEX IF EXISTS media_cleanup_due_idx;
ALTER TABLE media DROP COLUMN IF EXISTS cleanup_after_at;
