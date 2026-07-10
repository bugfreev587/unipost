# Post Delivery Worker Runbook

Use this when posts stay in `publishing` longer than expected.

## Phase Checks

Oldest DB-queued jobs:

```sql
SELECT id, post_id, workspace_id, social_account_id, platform, kind, state, created_at, next_run_at
FROM post_delivery_jobs
WHERE state = 'pending'
ORDER BY COALESCE(next_run_at, created_at) ASC
LIMIT 20;
```

Oldest jobs reserved by a worker but not yet dispatched to an adapter:

```sql
SELECT id, post_id, workspace_id, social_account_id, platform, kind, lease_owner, last_attempt_at, first_claimed_at
FROM post_delivery_jobs
WHERE state IN ('running', 'retrying')
  AND platform_started_at IS NULL
ORDER BY last_attempt_at ASC
LIMIT 20;
```

Active work by worker replica:

```sql
SELECT COALESCE(lease_owner, 'unowned') AS lease_owner, COUNT(*) AS active_jobs
FROM post_delivery_jobs
WHERE state IN ('running', 'retrying')
GROUP BY COALESCE(lease_owner, 'unowned')
ORDER BY active_jobs DESC;
```

Active work by workspace and platform:

```sql
SELECT workspace_id, platform, state, COUNT(*) AS jobs
FROM post_delivery_jobs
WHERE state IN ('pending', 'running', 'retrying')
GROUP BY workspace_id, platform, state
ORDER BY jobs DESC
LIMIT 50;
```

## Timing Interpretation

- Queue wait: `last_attempt_at - created_at` for dispatch jobs, or `last_attempt_at - next_run_at` for due retry jobs.
- Worker wait: `platform_started_at - last_attempt_at`.
- Platform duration: `finished_at - platform_started_at`.

If `platform_started_at` is empty while `state` is `running` or `retrying`, the job is reserved by a worker but has not reached the platform adapter.
