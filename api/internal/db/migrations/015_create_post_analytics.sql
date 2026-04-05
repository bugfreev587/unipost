-- +goose Up
CREATE TABLE post_analytics (
  id                    TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  social_post_result_id TEXT NOT NULL REFERENCES social_post_results(id) ON DELETE CASCADE,
  views                 BIGINT DEFAULT 0,
  likes                 BIGINT DEFAULT 0,
  comments              BIGINT DEFAULT 0,
  shares                BIGINT DEFAULT 0,
  reach                 BIGINT DEFAULT 0,
  impressions           BIGINT DEFAULT 0,
  engagement_rate       DECIMAL(10,4) DEFAULT 0,
  raw_data              JSONB,
  fetched_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(social_post_result_id)
);

-- +goose Down
DROP TABLE IF EXISTS post_analytics;
