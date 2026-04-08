-- +goose Up
--
-- Sprint 1: per-account captions + idempotent retries.
--
-- Background: AgentPost-style requests carry one POST whose CONTENTS
-- differ per platform (different caption per account, possibly
-- different media). The legacy schema stored the caption only on the
-- parent social_posts row, which forced every fanned-out platform post
-- to share that one string. We now persist the per-platform caption
-- on social_post_results so analytics, reads, and the scheduler all
-- see the truth of what each platform actually received.
--
-- The parent social_posts.caption stays for backwards compatibility:
-- it holds the canonical / "first" caption (typically equal to
-- platform_posts[0].caption when the new shape is used) so existing
-- read paths and dashboards keep working without a code change.
--
-- Idempotency: AgentPost CLIs need safe retries. A nightly worker
-- nulls keys older than 24h so the lookup index stays small.

ALTER TABLE social_post_results
  ADD COLUMN caption TEXT;

-- Backfill existing rows from the parent post so the column can later
-- be made NOT NULL. This is idempotent — running the migration twice
-- (e.g. local dev replays) is safe.
UPDATE social_post_results spr
SET caption = COALESCE(sp.caption, '')
FROM social_posts sp
WHERE spr.post_id = sp.id
  AND spr.caption IS NULL;

ALTER TABLE social_post_results
  ALTER COLUMN caption SET NOT NULL;

-- Idempotency key on the parent post.
ALTER TABLE social_posts
  ADD COLUMN idempotency_key TEXT;

-- Partial unique index — only enforces uniqueness when the column is
-- non-null, so historical rows (key = NULL) don't trip the constraint
-- and the index stays small.
CREATE UNIQUE INDEX social_posts_project_idempotency_uniq
  ON social_posts (project_id, idempotency_key)
  WHERE idempotency_key IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS social_posts_project_idempotency_uniq;
ALTER TABLE social_posts DROP COLUMN IF EXISTS idempotency_key;
ALTER TABLE social_post_results DROP COLUMN IF EXISTS caption;
