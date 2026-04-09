-- +goose Up
--
-- Sprint 5 PR2: per-account monthly quota.
--
-- Customers who want to cap a single end user's posting volume
-- (e.g. so a runaway script doesn't blow through the project's
-- monthly quota in 30 seconds) set this column. NULL = unlimited
-- = existing behavior. The publish path enforces it at dispatch
-- time by counting social_post_results.published_at rows in the
-- current calendar month for the target social_account_id.
--
-- The limit is per-social-account, not per-end-user (where an end
-- user could have several accounts) and not per-project (where
-- /v1/usage already covers the project total). Per-account is the
-- granularity that prevents one runaway account from eating the
-- whole project quota.

ALTER TABLE projects
  ADD COLUMN per_account_monthly_limit INTEGER;

-- Index supporting the count-this-month query in the publish path.
-- We filter by social_account_id + published_at, so a composite is
-- the right shape. Including a partial WHERE published_at IS NOT NULL
-- keeps it tight — failed posts (published_at NULL) are excluded
-- from the count anyway and don't need to live in this index.
CREATE INDEX IF NOT EXISTS social_post_results_quota_count_idx
  ON social_post_results (social_account_id, published_at)
  WHERE published_at IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS social_post_results_quota_count_idx;
ALTER TABLE projects DROP COLUMN IF EXISTS per_account_monthly_limit;
