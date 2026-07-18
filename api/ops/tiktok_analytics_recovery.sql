\set ON_ERROR_STOP on

\if :{?deployment_timestamp}
\else
  \echo 'deployment_timestamp is required (UTC ISO-8601, quoted for PostgreSQL)'
  \quit
\endif

\if :{?execute}
\else
  \set execute false
\endif

\if :execute
  \if :{?expected_count}
  \else
    \echo 'expected_count is required when execute=true; use the preceding dry-run total'
    \quit
  \endif
\endif

BEGIN;

CREATE TEMP TABLE tiktok_analytics_recovery_eligible
ON COMMIT DROP
AS
SELECT
  spr.id AS social_post_result_id,
  spr.social_account_id,
  COALESCE(sa.account_name, spr.social_account_id) AS account_name,
  spr.published_at,
  pa.fetched_at,
  CASE
    WHEN spr.published_at >= NOW() - INTERVAL '7 days' THEN '00-07 days'
    WHEN spr.published_at >= NOW() - INTERVAL '30 days' THEN '08-30 days'
    WHEN spr.published_at >= NOW() - INTERVAL '60 days' THEN '31-60 days'
    WHEN spr.published_at >= NOW() - INTERVAL '75 days' THEN '61-75 days'
    ELSE '76-90 days'
  END AS age_bucket
FROM social_post_results spr
JOIN social_posts sp
  ON sp.id = spr.post_id
JOIN social_accounts sa
  ON sa.id = spr.social_account_id
LEFT JOIN post_analytics pa
  ON pa.social_post_result_id = spr.id
WHERE sa.platform = 'tiktok'
  AND sa.status = 'active'
  AND sa.disconnected_at IS NULL
  AND spr.status = 'published'
  AND spr.external_id IS NOT NULL
  AND BTRIM(spr.external_id) <> ''
  AND spr.published_at IS NOT NULL
  AND spr.published_at > NOW() - INTERVAL '90 days'
  AND sp.deleted_at IS NULL
  AND (
    pa.social_post_result_id IS NULL
    OR (
      (pa.fetched_at IS NULL OR pa.fetched_at < :deployment_timestamp::timestamptz)
      AND pa.fetched_at IS DISTINCT FROM '1970-01-01 00:00:00+00'::timestamptz
    )
  );

\echo 'TikTok analytics recovery candidates by account and age bucket:'
SELECT
  social_account_id,
  account_name,
  age_bucket,
  COUNT(*) AS candidate_count
FROM tiktok_analytics_recovery_eligible
GROUP BY social_account_id, account_name, age_bucket
ORDER BY social_account_id, age_bucket;

\echo 'TikTok analytics recovery candidate total:'
SELECT COUNT(*) AS candidate_count
FROM tiktok_analytics_recovery_eligible;

\if :execute
  WITH scheduled AS (
    INSERT INTO post_analytics (social_post_result_id, fetched_at)
    SELECT
      social_post_result_id,
      '1970-01-01 00:00:00+00'::timestamptz
    FROM tiktok_analytics_recovery_eligible
    ON CONFLICT (social_post_result_id) DO UPDATE
    SET fetched_at = EXCLUDED.fetched_at
    WHERE post_analytics.fetched_at IS NULL
       OR (
         post_analytics.fetched_at < :deployment_timestamp::timestamptz
         AND post_analytics.fetched_at IS DISTINCT FROM EXCLUDED.fetched_at
       )
    RETURNING social_post_result_id
  )
  SELECT COUNT(*) AS scheduled_count
  FROM scheduled
  \gset

  \echo 'Scheduled rows:' :scheduled_count
  SELECT (:scheduled_count::bigint = :expected_count::bigint) AS recovery_count_matches
  \gset
  \if :recovery_count_matches
    COMMIT;
    \echo 'TikTok analytics recovery rows committed.'
  \else
    \echo 'Scheduled count did not match expected_count; transaction will roll back.'
    \quit
  \endif
\else
  ROLLBACK;
  \echo 'Dry run complete. No rows were changed.'
\endif
