-- name: UpsertPostAnalytics :one
INSERT INTO post_analytics (
  social_post_result_id, views, likes, comments, shares, reach, impressions,
  saves, clicks, video_views, platform_specific, engagement_rate, raw_data
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT (social_post_result_id) DO UPDATE
SET views             = EXCLUDED.views,
    likes             = EXCLUDED.likes,
    comments          = EXCLUDED.comments,
    shares            = EXCLUDED.shares,
    reach             = EXCLUDED.reach,
    impressions       = EXCLUDED.impressions,
    saves             = EXCLUDED.saves,
    clicks            = EXCLUDED.clicks,
    video_views       = EXCLUDED.video_views,
    platform_specific = EXCLUDED.platform_specific,
    engagement_rate   = EXCLUDED.engagement_rate,
    raw_data          = EXCLUDED.raw_data,
    fetched_at        = NOW()
RETURNING *;

-- name: GetPostAnalytics :one
SELECT * FROM post_analytics WHERE social_post_result_id = $1;

-- name: GetAnalyticsSummaryByProject :one
-- Aggregate post counts and engagement totals for a project over a date range.
-- Filtering uses social_posts.created_at; analytics rows are joined via results.
-- Empty-string sentinel for the platform/status params disables that filter.
-- LEFT JOIN to social_accounts ensures posts with no results still count when
-- the platform filter is unset, but are correctly excluded when it is set
-- (NULL platform fails the equality test).
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
WHERE sp.project_id = $1
  AND sp.created_at >= $2
  AND sp.created_at <  $3
  AND ($4::text = '' OR sa.platform = $4)
  AND ($5::text = '' OR sp.status   = $5);

-- name: GetAnalyticsTrendByProject :many
-- Daily time series. Days with no posts are NOT returned by SQL — the
-- handler zero-fills them in Go to keep the query simple. Same platform/status
-- filter convention as the summary query.
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
WHERE sp.project_id = $1
  AND sp.created_at >= $2
  AND sp.created_at <  $3
  AND ($4::text = '' OR sa.platform = $4)
  AND ($5::text = '' OR sp.status   = $5)
GROUP BY day
ORDER BY day ASC;

-- name: GetDuePostAnalyticsRefresh :many
-- Returns published results whose cached analytics are stale per the PRD §9.3
-- tier policy. NULL fetched_at (never fetched) is always due. Older-than-90-day
-- posts are excluded so the worker doesn't spin forever on a long backfill.
-- LIMIT bounds the work per tick.
SELECT
  spr.id                  AS social_post_result_id,
  spr.social_account_id,
  spr.external_id,
  sa.platform,
  sa.access_token,
  sa.refresh_token,
  sa.token_expires_at
FROM social_post_results spr
JOIN social_accounts sa     ON sa.id = spr.social_account_id
LEFT JOIN post_analytics pa ON pa.social_post_result_id = spr.id
WHERE spr.status = 'published'
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

-- name: GetAnalyticsByPlatformByProject :many
-- Per-platform aggregates. Inner-joins social_accounts so that posts with
-- no results (still publishing or all-failed at validation) are excluded —
-- a post can't have a platform breakdown without a result.
-- platform/status filters use the same empty-string sentinel as the other
-- aggregation queries; passing platform='tiktok' degenerates to a single row.
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
WHERE sp.project_id = $1
  AND sp.created_at >= $2
  AND sp.created_at <  $3
  AND ($4::text = '' OR sa.platform = $4)
  AND ($5::text = '' OR sp.status   = $5)
GROUP BY sa.platform
ORDER BY sa.platform ASC;
