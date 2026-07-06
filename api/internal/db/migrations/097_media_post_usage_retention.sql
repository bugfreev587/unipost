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
    AND post_status IN ('published', 'partial', 'failed', 'cancelled');

-- Backfill existing terminal v2 posts so media already attached to
-- published/failed/cancelled history is not orphaned when the legacy
-- media.cleanup_after_at deadline is cleared below. Unknown or missing
-- subscriptions fall back to Free, matching runtime policy.
INSERT INTO media_post_usages (
  workspace_id,
  media_id,
  post_id,
  post_status,
  cleanup_after_at
)
SELECT DISTINCT
  sp.workspace_id,
  m.id AS media_id,
  sp.id AS post_id,
  sp.status AS post_status,
  COALESCE(sp.published_at, sp.created_at) +
    CASE
      WHEN sp.status = 'published' THEN
        CASE COALESCE(sub.plan_id, 'free')
          WHEN 'api' THEN INTERVAL '2 days'
          WHEN 'basic' THEN INTERVAL '4 days'
          WHEN 'growth' THEN INTERVAL '15 days'
          WHEN 'team' THEN INTERVAL '30 days'
          WHEN 'enterprise' THEN INTERVAL '30 days'
          ELSE INTERVAL '1 day'
        END
      ELSE
        CASE COALESCE(sub.plan_id, 'free')
          WHEN 'api' THEN INTERVAL '4 days'
          WHEN 'basic' THEN INTERVAL '8 days'
          WHEN 'growth' THEN INTERVAL '30 days'
          WHEN 'team' THEN INTERVAL '60 days'
          WHEN 'enterprise' THEN INTERVAL '60 days'
          ELSE INTERVAL '2 days'
        END
    END AS cleanup_after_at
FROM social_posts sp
LEFT JOIN subscriptions sub
  ON sub.workspace_id = sp.workspace_id
CROSS JOIN LATERAL jsonb_array_elements(COALESCE(sp.metadata->'platform_posts', '[]'::jsonb)) platform_post
CROSS JOIN LATERAL jsonb_array_elements_text(COALESCE(platform_post->'media_ids', '[]'::jsonb)) media_ref(media_id)
JOIN media m
  ON m.id = media_ref.media_id
 AND m.workspace_id = sp.workspace_id
WHERE sp.status IN ('published', 'partial', 'failed', 'cancelled')
ON CONFLICT (media_id, post_id) DO UPDATE
SET post_status = EXCLUDED.post_status,
    cleanup_after_at = EXCLUDED.cleanup_after_at,
    updated_at = NOW();

-- Stop the legacy 200MB / 2h cleanup policy from deleting rows after
-- the new retention ledger deploys.
UPDATE media SET cleanup_after_at = NULL WHERE cleanup_after_at IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS media_post_usages_cleanup_due_idx;
DROP INDEX IF EXISTS media_post_usages_media_idx;
DROP INDEX IF EXISTS media_post_usages_post_idx;
DROP TABLE IF EXISTS media_post_usages;
