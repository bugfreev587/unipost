-- +goose Up
--
-- Sprint 5: video metadata columns on the media table.
--
-- Sprint 2 explicitly skipped these (see migration 019's comment) because
-- no validation rule needed dimensions yet. That changes now: Facebook
-- silently reclassifies feed videos as Reels based on aspect ratio, and
-- the reclassified post then fails for unrelated reasons (missing app
-- permission, scheduled publish, etc.). To stop that we need to know
-- the video's width / height / duration BEFORE we hand it to FB, so the
-- validator can reject "this video is 9:16 vertical, FB will reclassify
-- it as a Reel — switch placements" up front.
--
-- Columns are nullable on purpose:
--   - existing rows aren't backfilled (sweeper will reap unattached
--     pending uploads naturally; attached rows stay valid and the
--     validator falls back to a warning when metadata is NULL).
--   - non-video uploads (images, GIFs) leave them NULL.
--   - probe failures (unsupported container like webm, malformed file)
--     leave them NULL rather than failing the upload — the validator
--     surfaces a warning so the user can still publish at their own
--     risk if they know the file is fine.
--
-- duration_ms is milliseconds (not seconds) so we can express FB's
-- 90-second Reel cap and TikTok's 60-second Story cap precisely.
-- INTEGER ranges to ~24 days, plenty for any social video format.

ALTER TABLE media ADD COLUMN width INTEGER;
ALTER TABLE media ADD COLUMN height INTEGER;
ALTER TABLE media ADD COLUMN duration_ms INTEGER;

-- +goose Down
ALTER TABLE media DROP COLUMN IF EXISTS duration_ms;
ALTER TABLE media DROP COLUMN IF EXISTS height;
ALTER TABLE media DROP COLUMN IF EXISTS width;
