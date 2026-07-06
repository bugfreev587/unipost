-- +goose Up
--
-- R2 media retention review fixes.
--
-- Migration 097 introduced the media_post_usages ledger and cleared
-- media.cleanup_after_at. That version may already be applied in
-- development, so the terminal-post backfill and cancelled-status
-- cleanup index need a follow-up migration instead of only editing 097.

DROP INDEX IF EXISTS media_post_usages_cleanup_due_idx;

CREATE INDEX media_post_usages_cleanup_due_idx
  ON media_post_usages (cleanup_after_at)
  WHERE cleanup_after_at IS NOT NULL
    AND post_status IN ('published', 'partial', 'failed', 'cancelled');

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

-- +goose Down
DROP INDEX IF EXISTS media_post_usages_cleanup_due_idx;

CREATE INDEX media_post_usages_cleanup_due_idx
  ON media_post_usages (cleanup_after_at)
  WHERE cleanup_after_at IS NOT NULL
    AND post_status IN ('published', 'partial', 'failed');
