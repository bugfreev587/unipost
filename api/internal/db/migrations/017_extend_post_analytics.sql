-- +goose Up
ALTER TABLE post_analytics
  ADD COLUMN saves             BIGINT DEFAULT 0,
  ADD COLUMN clicks            BIGINT DEFAULT 0,
  ADD COLUMN video_views       BIGINT DEFAULT 0,
  ADD COLUMN platform_specific JSONB;

CREATE INDEX IF NOT EXISTS idx_post_analytics_fetched_at
  ON post_analytics(fetched_at);

-- +goose Down
DROP INDEX IF EXISTS idx_post_analytics_fetched_at;

ALTER TABLE post_analytics
  DROP COLUMN IF EXISTS saves,
  DROP COLUMN IF EXISTS clicks,
  DROP COLUMN IF EXISTS video_views,
  DROP COLUMN IF EXISTS platform_specific;
