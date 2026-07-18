# TikTok Analytics Historical Recovery Runbook

This runbook re-arms affected TikTok analytics rows after the exact-video-ID fix is deployed. It does not fetch TikTok directly. It sets only `post_analytics.fetched_at` to the Unix epoch and lets the existing analytics worker recover rows with its normal batch and concurrency limits.

Do not run this script in production until the production release is explicitly authorized, deployed, and verified with at least one current TikTok post. Run the dry run and execution from the same checked-in release.

## Safety properties

- The script defaults to `execute=false` and rolls back.
- The execution path requires both a deployment timestamp and the dry-run candidate count.
- The mutation runs in one transaction.
- Missing `post_analytics` rows are inserted.
- Existing rows update only `fetched_at`.
- Metrics, `platform_specific`, and failure history are preserved.
- Rows already armed at the epoch or processed after the supplied deployment timestamp are skipped.
- Only active, connected TikTok accounts and published, non-deleted posts from the supported 90-day window are eligible.

## Prerequisites

1. The exact-ID fix is deployed in the target environment.
2. The deployment and all required health checks have completed.
3. Record the UTC deployment timestamp used by the fixed backend, for example `2026-07-18T00:15:00Z`.
4. Confirm the analytics worker is running and its concurrency remains at five or lower.
5. Confirm an authorized operator has database access through `DATABASE_URL`.
6. For production, obtain explicit production-release and recovery authorization.

Do not print, paste into tickets, or commit the value of `DATABASE_URL`.

## Validate TikTok publish-ID retention

Before promising a full 90-day recovery, identify an accessible 75-90-day-old row:

```sql
SELECT
  spr.id AS social_post_result_id,
  spr.external_id AS publish_id,
  spr.published_at,
  spr.social_account_id
FROM social_post_results spr
JOIN social_posts sp ON sp.id = spr.post_id
JOIN social_accounts sa ON sa.id = spr.social_account_id
WHERE sa.platform = 'tiktok'
  AND sa.status = 'active'
  AND sa.disconnected_at IS NULL
  AND spr.status = 'published'
  AND spr.external_id IS NOT NULL
  AND sp.deleted_at IS NULL
  AND spr.published_at BETWEEN NOW() - INTERVAL '90 days' AND NOW() - INTERVAL '75 days'
ORDER BY spr.published_at ASC
LIMIT 10;
```

Using development-owned or otherwise authorized account credentials, verify one selected `publish_id` through the deployed application flow:

1. TikTok publish status still resolves the `publish_id`.
2. The returned public video ID is preserved exactly.
3. TikTok video query returns a row with the same public video ID.

If no 75-90-day row is available, test the oldest accessible row and record the maximum verified age. If old publish IDs no longer resolve, limit the recovery promise to the verified age and do not repeatedly re-arm older rows.

## Dry run

Run from the repository root. Keep the timestamp single-quoted inside the psql variable value:

```bash
psql "$DATABASE_URL" \
  -v deployment_timestamp="'2026-07-18T00:15:00Z'" \
  -v execute=false \
  -f api/ops/tiktok_analytics_recovery.sql
```

Save the candidate total and reconcile the per-account and age-bucket counts. Investigate before proceeding if the total is unexpectedly zero, materially exceeds the incident estimate, includes an unexpected account, or differs from an independent eligibility query.

## Schedule recovery

Use the unchanged deployment timestamp and the exact dry-run candidate total:

```bash
psql "$DATABASE_URL" \
  -v deployment_timestamp="'2026-07-18T00:15:00Z'" \
  -v execute=true \
  -v expected_count=2126 \
  -f api/ops/tiktok_analytics_recovery.sql
```

The transaction commits only when the scheduled count matches `expected_count`. A mismatch exits the psql session and PostgreSQL rolls back the open transaction. Re-run the dry run before retrying; do not adjust the expected count without reconciling why it changed.

Rerunning the execution with the same timestamp does not re-arm rows already set to the epoch or rows with a post-deployment worker outcome.

## Monitor recovery

The analytics refresh worker reads at most 200 due rows per shared cross-platform cycle with concurrency five. The first cycle runs at application startup and subsequent cycles run hourly. TikTok throughput may be lower than 200 because the limit is shared with other platforms.

During each cycle, monitor:

- TikTok analytics attempts and matched-video successes;
- `analytics_scope_required`, `account_token_invalid`, `video_not_found`, and `video_not_ready` outcomes;
- TikTok 429, timeout, and 5xx rates;
- analytics worker batch duration and backlog;
- TikTok publishing success rate and latency.

Pause recovery before publishing is affected if TikTok 429s materially increase, TikTok timeouts or 5xx responses spike, or TikTok publishing failures regress. Pausing means stopping the analytics refresh worker or otherwise preventing additional recovery cycles; do not change metrics or mark publishing-capable accounts reconnect-required solely for an analytics-scope error.

## Completion query

Use the same deployment timestamp:

```sql
WITH eligible AS (
  SELECT spr.id
  FROM social_post_results spr
  JOIN social_posts sp ON sp.id = spr.post_id
  JOIN social_accounts sa ON sa.id = spr.social_account_id
  WHERE sa.platform = 'tiktok'
    AND sa.status = 'active'
    AND sa.disconnected_at IS NULL
    AND spr.status = 'published'
    AND spr.external_id IS NOT NULL
    AND spr.published_at IS NOT NULL
    AND spr.published_at > NOW() - INTERVAL '90 days'
    AND sp.deleted_at IS NULL
)
SELECT
  COUNT(*) AS eligible,
  COUNT(*) FILTER (
    WHERE pa.fetched_at >= '2026-07-18T00:15:00Z'::timestamptz
      AND pa.last_failure_reason IS NULL
      AND NULLIF(pa.platform_specific->>'tiktok_video_id', '') IS NOT NULL
  ) AS recovered,
  COUNT(*) FILTER (
    WHERE pa.fetched_at >= '2026-07-18T00:15:00Z'::timestamptz
      AND pa.last_failure_reason IS NOT NULL
  ) AS explicit_failure,
  COUNT(*) FILTER (
    WHERE pa.fetched_at < '2026-07-18T00:15:00Z'::timestamptz
       OR pa.fetched_at IS NULL
  ) AS pending
FROM eligible e
LEFT JOIN post_analytics pa ON pa.social_post_result_id = e.id;
```

Recovery is complete when every eligible row has either:

- a post-deployment success with an exact `platform_specific.tiktok_video_id`; or
- a post-deployment explicit failure with `last_failure_reason` populated.

Do not define success as “every metric is non-zero.” A matched TikTok video can legitimately have zero views or engagement.

## Stop and escalate

Stop without further mutations when:

- the dry-run and scheduled counts do not match;
- the candidate set includes disconnected, reconnect-required, deleted, non-TikTok, or older-than-90-day rows;
- a 75-90-day publish-ID retention check fails and the supported recovery age is undecided;
- the worker overwrites prior real metrics on failure;
- TikTok publishing health regresses;
- database, deployment, or production authorization is missing.
