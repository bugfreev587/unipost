-- +goose Up

-- Track who triggered the publish (dashboard UI vs external API) and
-- which profiles the post landed under. profile_ids is the set of
-- distinct profile_ids across every social_account the post targeted
-- — a single post can target accounts across multiple profiles.
--
-- Existing rows:
--   source        defaults to 'ui' (historic best-guess; no way to
--                 reconstruct reliably from existing data).
--   profile_ids   backfilled from social_post_results → social_accounts.
--                 Drafts / scheduled rows that have no results yet
--                 stay empty here; the publish path lazy-populates
--                 them on claim (see ensureProfileIDsForPost).
ALTER TABLE social_posts
  ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'ui'
    CHECK (source IN ('ui', 'api')),
  ADD COLUMN IF NOT EXISTS profile_ids TEXT[] NOT NULL DEFAULT '{}';

-- Backfill profile_ids for rows that already have published results.
UPDATE social_posts sp
SET profile_ids = sub.profile_ids
FROM (
  SELECT spr.post_id,
         ARRAY_AGG(DISTINCT sa.profile_id) AS profile_ids
  FROM social_post_results spr
  JOIN social_accounts sa ON sa.id = spr.social_account_id
  GROUP BY spr.post_id
) sub
WHERE sp.id = sub.post_id
  AND cardinality(sp.profile_ids) = 0;

-- GIN index powers `profile_id = ANY(profile_ids)` and `profile_ids @> ARRAY[...]`
-- filters used by the profile detail page + any future per-profile views.
CREATE INDEX IF NOT EXISTS idx_social_posts_profile_ids
  ON social_posts USING GIN (profile_ids);

-- +goose Down

DROP INDEX IF EXISTS idx_social_posts_profile_ids;

ALTER TABLE social_posts
  DROP COLUMN IF EXISTS profile_ids,
  DROP COLUMN IF EXISTS source;
