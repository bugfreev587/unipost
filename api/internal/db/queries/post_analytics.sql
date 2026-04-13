-- name: UpsertPostAnalytics :one
INSERT INTO post_analytics (
  social_post_result_id, views, likes, comments, shares, reach, impressions,
  saves, clicks, video_views, platform_specific, engagement_rate, raw_data
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT (social_post_result_id) DO UPDATE
SET views                = EXCLUDED.views,
    likes                = EXCLUDED.likes,
    comments             = EXCLUDED.comments,
    shares               = EXCLUDED.shares,
    reach                = EXCLUDED.reach,
    impressions          = EXCLUDED.impressions,
    saves                = EXCLUDED.saves,
    clicks               = EXCLUDED.clicks,
    video_views          = EXCLUDED.video_views,
    platform_specific    = EXCLUDED.platform_specific,
    engagement_rate      = EXCLUDED.engagement_rate,
    raw_data             = EXCLUDED.raw_data,
    fetched_at           = NOW(),
    consecutive_failures = 0,
    last_failure_reason  = NULL
RETURNING *;

-- name: TouchPostAnalyticsFetchedAt :exec
-- Bump fetched_at so the tier-based refresh TTL kicks in after a
-- failed platform fetch. Inserts a minimal row if none exists yet;
-- when a row already exists the ON CONFLICT only touches fetched_at
-- and the failure counter, preserving any real metrics from a prior
-- successful fetch.
INSERT INTO post_analytics (social_post_result_id, fetched_at, consecutive_failures, last_failure_reason)
VALUES ($1, NOW(), 1, $2)
ON CONFLICT (social_post_result_id) DO UPDATE
SET fetched_at           = NOW(),
    consecutive_failures = post_analytics.consecutive_failures + 1,
    last_failure_reason  = $2;

-- name: GetPostAnalytics :one
SELECT * FROM post_analytics WHERE social_post_result_id = $1;

-- name: GetAnalyticsSummaryByWorkspace :one
SELECT
  COUNT(DISTINCT sp.id)::BIGINT                                              AS total_posts,
  COUNT(DISTINCT sp.id) FILTER (WHERE sp.status = 'published')::BIGINT       AS published_posts,
  COUNT(DISTINCT sp.id) FILTER (WHERE sp.status = 'failed')::BIGINT          AS failed_posts,
  COUNT(DISTINCT sp.id) FILTER (WHERE sp.status = 'scheduled')::BIGINT       AS scheduled_posts,
  COALESCE(SUM(pa.impressions), 0)::BIGINT                                   AS impressions,
  COALESCE(SUM(pa.reach), 0)::BIGINT                                         AS reach,
  COALESCE(SUM(pa.likes), 0)::BIGINT                                         AS likes,
  COALESCE(SUM(pa.comments), 0)::BIGINT                                      AS comments,
  COALESCE(SUM(pa.shares), 0)::BIGINT                                        AS shares,
  COALESCE(SUM(pa.saves), 0)::BIGINT                                         AS saves,
  COALESCE(SUM(pa.clicks), 0)::BIGINT                                        AS clicks,
  COALESCE(SUM(pa.video_views), 0)::BIGINT                                   AS video_views
FROM social_posts sp
LEFT JOIN social_post_results spr ON spr.post_id = sp.id
LEFT JOIN social_accounts sa      ON sa.id = spr.social_account_id
LEFT JOIN post_analytics pa       ON pa.social_post_result_id = spr.id
WHERE sp.workspace_id = $1
  AND sp.deleted_at IS NULL
  AND sp.created_at >= $2
  AND sp.created_at <  $3
  AND ($4::text = '' OR sa.platform = $4)
  AND ($5::text = '' OR sp.status   = $5);

-- name: GetAnalyticsTrendByWorkspace :many
SELECT
  date_trunc('day', sp.created_at)::TIMESTAMPTZ                              AS day,
  COUNT(DISTINCT sp.id)::BIGINT                                              AS posts,
  COALESCE(SUM(pa.impressions), 0)::BIGINT                                   AS impressions,
  COALESCE(SUM(pa.likes), 0)::BIGINT                                         AS likes,
  COALESCE(SUM(pa.comments), 0)::BIGINT                                      AS comments,
  COALESCE(SUM(pa.shares), 0)::BIGINT                                        AS shares
FROM social_posts sp
LEFT JOIN social_post_results spr ON spr.post_id = sp.id
LEFT JOIN social_accounts sa      ON sa.id = spr.social_account_id
LEFT JOIN post_analytics pa       ON pa.social_post_result_id = spr.id
WHERE sp.workspace_id = $1
  AND sp.deleted_at IS NULL
  AND sp.created_at >= $2
  AND sp.created_at <  $3
  AND ($4::text = '' OR sa.platform = $4)
  AND ($5::text = '' OR sp.status   = $5)
GROUP BY day
ORDER BY day ASC;

-- name: GetDuePostAnalyticsRefresh :many
SELECT
  spr.id                  AS social_post_result_id,
  spr.social_account_id,
  spr.external_id,
  sa.platform,
  sa.access_token,
  sa.refresh_token,
  sa.token_expires_at
FROM social_post_results spr
JOIN social_posts sp       ON sp.id = spr.post_id
JOIN social_accounts sa     ON sa.id = spr.social_account_id
LEFT JOIN post_analytics pa ON pa.social_post_result_id = spr.id
WHERE spr.status = 'published'
  AND sp.deleted_at IS NULL
  AND spr.external_id IS NOT NULL
  AND spr.published_at IS NOT NULL
  AND spr.published_at > NOW() - INTERVAL '90 days'
  AND sa.disconnected_at IS NULL
  AND (
    pa.fetched_at IS NULL
    OR (spr.published_at >  NOW() - INTERVAL '24 hours' AND pa.fetched_at < NOW() - INTERVAL '1 hour')
    OR (spr.published_at <= NOW() - INTERVAL '24 hours' AND spr.published_at > NOW() - INTERVAL '7 days' AND pa.fetched_at < NOW() - INTERVAL '6 hours')
    OR (spr.published_at <= NOW() - INTERVAL '7 days'  AND pa.fetched_at < NOW() - INTERVAL '24 hours')
  )
ORDER BY pa.fetched_at NULLS FIRST
LIMIT 200;

-- name: GetAnalyticsByPlatformByWorkspace :many
SELECT
  sa.platform::TEXT                                                          AS platform,
  COUNT(DISTINCT sp.id)::BIGINT                                              AS posts,
  COUNT(DISTINCT sa.id)::BIGINT                                              AS accounts,
  COALESCE(SUM(pa.impressions), 0)::BIGINT                                   AS impressions,
  COALESCE(SUM(pa.reach), 0)::BIGINT                                         AS reach,
  COALESCE(SUM(pa.likes), 0)::BIGINT                                         AS likes,
  COALESCE(SUM(pa.comments), 0)::BIGINT                                      AS comments,
  COALESCE(SUM(pa.shares), 0)::BIGINT                                        AS shares,
  COALESCE(SUM(pa.saves), 0)::BIGINT                                         AS saves,
  COALESCE(SUM(pa.clicks), 0)::BIGINT                                        AS clicks,
  COALESCE(SUM(pa.video_views), 0)::BIGINT                                   AS video_views
FROM social_posts sp
JOIN social_post_results spr ON spr.post_id = sp.id
JOIN social_accounts sa      ON sa.id = spr.social_account_id
LEFT JOIN post_analytics pa  ON pa.social_post_result_id = spr.id
WHERE sp.workspace_id = $1
  AND sp.deleted_at IS NULL
  AND sp.created_at >= $2
  AND sp.created_at <  $3
  AND ($4::text = '' OR sa.platform = $4)
  AND ($5::text = '' OR sp.status   = $5)
GROUP BY sa.platform
ORDER BY sa.platform ASC;
