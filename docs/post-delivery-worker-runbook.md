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

## Database reachability and queue age

The worker `/health` endpoint checks Postgres with a two-second deadline and returns
503 with `database: unavailable` if the connection cannot be used. This is readiness,
not proof of a successful dispatch; Railway's deploy health check is not a continuous
runtime alert. Monitor the endpoint and `post dispatch worker: claim failed` logs.
Connection attempts are capped at five seconds; recovery and claim operations each
have a ten-second deadline. Provider execution retains its own context and leases.
These bounds do not repair a private-network outage, but allow polling to resume
promptly after connectivity recovers.

Workspace clients can poll `GET /v1/post-delivery-jobs/summary` for
`oldest_pending_at` (null with no ready pending work) and `oldest_pending_age_ms`
(zero with no ready pending work). Future retry backoffs and dismissed jobs are
excluded from age; pending_count still includes future retries. An increasing age
indicates a backlog, not necessarily a stopped worker. Alert operationally when ready
pending age exceeds your delivery SLO, and inspect connection/claim errors before
retrying. Never resend a successful job to diagnose a historical queue delay.

Account health ignores intentional pre-dispatch cancellations and follows the newest
non-cancelled terminal result, ordered by published time or delivery completion time,
with post creation as the legacy fallback. A newer success clears the displayed old
error; a newer failure returns degraded with its actual occurrence time. A failed
health query returns 503 rather than reporting a false healthy account.
