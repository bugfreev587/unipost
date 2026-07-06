-- +goose Up
--
-- R2 media retention v1.
--
-- Keep a business-owned ledger that links uploaded media to the post
-- lifecycle that decides when the R2 object can be removed. This
-- replaces the old media.cleanup_after_at policy, which only deleted
-- large successful uploads after a fixed 2h window.

CREATE TABLE media_post_usages (
  id               TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id     TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  media_id         TEXT NOT NULL REFERENCES media(id) ON DELETE CASCADE,
  post_id          TEXT NOT NULL REFERENCES social_posts(id) ON DELETE CASCADE,
  post_status      TEXT NOT NULL,
  cleanup_after_at TIMESTAMPTZ,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (media_id, post_id)
);

CREATE INDEX media_post_usages_post_idx
  ON media_post_usages (post_id);

CREATE INDEX media_post_usages_media_idx
  ON media_post_usages (media_id);

CREATE INDEX media_post_usages_cleanup_due_idx
  ON media_post_usages (cleanup_after_at)
  WHERE cleanup_after_at IS NOT NULL
    AND post_status IN ('published', 'partial', 'failed');

-- Stop the legacy 200MB / 2h cleanup policy from deleting rows after
-- the new retention ledger deploys.
UPDATE media SET cleanup_after_at = NULL WHERE cleanup_after_at IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS media_post_usages_cleanup_due_idx;
DROP INDEX IF EXISTS media_post_usages_media_idx;
DROP INDEX IF EXISTS media_post_usages_post_idx;
DROP TABLE IF EXISTS media_post_usages;
