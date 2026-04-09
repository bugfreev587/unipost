-- +goose Up
--
-- Sprint 5 PR1: indexes that support GET /v1/analytics/rollup.
--
-- The rollup query joins social_post_results → social_posts
-- → social_accounts and groups by date_trunc(granularity, created_at)
-- + an optional dimension (platform / social_account_id /
-- external_user_id / status). To stay under the 300ms target on a
-- 10k-post project (~30k results), the query needs:
--
--   1. A way to filter social_post_results by post_id without a
--      seq scan. social_posts_project_created_idx (added in Sprint
--      2 PR7) covers the social_posts side; we need the matching
--      side on social_post_results.
--   2. A way to filter by status when grouping by published vs
--      failed counts. Including status in a composite index lets
--      Postgres skip rows without touching the heap.
--
-- This is a small write-time cost (one extra btree per insert)
-- and a meaningful read-time speedup at >10k results.

-- Composite index supporting the JOIN + status filter. post_id is
-- the FK column we join on; status is the most-common filter and
-- the smallest distinct set so it goes second.
CREATE INDEX IF NOT EXISTS social_post_results_post_status_idx
  ON social_post_results (post_id, status);

-- Composite for grouping by account_id over a date range. The
-- /v1/analytics/rollup endpoint with group_by=social_account_id
-- benefits from this; without it the query falls back to the
-- post_id index above.
CREATE INDEX IF NOT EXISTS social_post_results_account_idx
  ON social_post_results (social_account_id, post_id);

-- +goose Down
DROP INDEX IF EXISTS social_post_results_account_idx;
DROP INDEX IF EXISTS social_post_results_post_status_idx;
