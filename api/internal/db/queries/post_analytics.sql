-- name: UpsertPostAnalytics :one
INSERT INTO post_analytics (social_post_result_id, views, likes, comments, shares, reach, impressions, engagement_rate, raw_data)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (social_post_result_id) DO UPDATE
SET views = EXCLUDED.views, likes = EXCLUDED.likes, comments = EXCLUDED.comments,
    shares = EXCLUDED.shares, reach = EXCLUDED.reach, impressions = EXCLUDED.impressions,
    engagement_rate = EXCLUDED.engagement_rate, raw_data = EXCLUDED.raw_data, fetched_at = NOW()
RETURNING *;

-- name: GetPostAnalytics :one
SELECT * FROM post_analytics WHERE social_post_result_id = $1;
