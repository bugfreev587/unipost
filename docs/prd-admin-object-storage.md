# PRD - Admin Object Storage

**Status:** Planning
**Owner:** Admin / Dashboard / API / Worker
**Created:** 2026-07-07
**Target:** Admin visibility into tracked R2 object storage usage, cleanup worker activity, and object creation/deletion trends

---

## Problem

UniPost stores customer-uploaded media and internal media derivatives in Cloudflare R2, but admins do not currently have a dedicated operational page for storage health.

The existing system has the raw ingredients:

- `media` tracks uploaded objects, storage keys, size, content type, status, and timestamps.
- `media_post_usages` tracks post retention deadlines and determines which media can be cleaned.
- `MediaCleanupWorker` deletes due R2 objects and hard-deletes the corresponding `media` rows.
- Logs show cleanup progress, but cleanup results are not persisted in a queryable table.

This leaves admins without fast answers to common storage questions:

- How much tracked R2 storage is UniPost currently holding?
- Which bucket is configured and how much tracked data belongs to it?
- Did the cleanup worker run recently?
- When is the worker expected to run next?
- How many objects and bytes were added or deleted in a selected period?
- Is there a cleanup backlog?
- Are failures preventing cleanup from reducing storage?

Admins need a first-party admin page that makes storage usage and cleanup behavior visible without requiring shell access, direct database queries, Railway logs, or Cloudflare console access.

## Product Direction

Add a new admin page under Email:

```text
Object Storage -> /admin/object-storage
```

The page shows tracked R2 object counts, confirmed tracked storage size, cleanup worker telemetry, bucket-level confirmed tracked size, and selected-period object creation/deletion metrics.

V1 should use application-owned data as the source of truth:

- Current tracked objects come from `media`.
- Current tracked storage size comes from uploaded media rows only: `media.status = 'uploaded'`.
- Cleanup eligibility and backlog come from `media_post_usages` joined to `media`.
- Actual deletion history and worker run status come from a new `media_cleanup_runs` table written by `MediaCleanupWorker`.

V1 should not connect the browser directly to Cloudflare and should not expose R2 credentials. Metrics should be labeled as "tracked" when they come from UniPost database rows rather than Cloudflare's billing or bucket inventory APIs.

No feature flag is required.

## V1 Scope

### Admin Navigation

Add `Object Storage` immediately below `Email` in the admin sidebar.

Suggested route:

```text
/admin/object-storage
```

Suggested sidebar section:

```text
Overview
```

### Dashboard Cards

Show these top-level cards:

1. Current confirmed tracked R2 object storage size.
2. Current tracked object count.
3. Cleanup worker last run.
4. Cleanup worker estimated next run.
5. Deleted objects in the selected period.
6. Deleted size in the selected period.
7. Due cleanup backlog count.
8. Due cleanup backlog size.

The exact layout can follow existing admin dashboard patterns and does not need a custom design system.

### Bucket Table

Show R2 bucket rows:

| Column | Meaning |
| --- | --- |
| Bucket | Bucket name from server configuration, currently `R2_BUCKET_NAME`. |
| Tracked objects | Count of non-deleted tracked objects in `media`. |
| Confirmed tracked size | Sum of `media.size_bytes` for `status = 'uploaded'`. Pending rows are not counted as stored bytes. |
| Pending | Objects with `status = 'pending'`. |
| Uploaded | Objects with `status = 'uploaded'`. |
| Referenced | Distinct media objects with at least one active `media_post_usages` row where `cleanup_after_at IS NULL OR cleanup_after_at > now()`. This is derived from the usage ledger, not from `media.status`. |
| Due cleanup | Objects currently eligible for retention cleanup. |
| Due size | Total size of due cleanup objects. |

Current backend configuration supports one R2 bucket. The API response should use an array so future multi-bucket support does not require a frontend rewrite.

### Period Filter

Support these period filters:

- `yesterday`
- `last_7_days`
- `last_month`
- `this_week`
- `this_month`
- `this_year`

The default should be `last_7_days`.

Use server-side UTC timestamps for API queries. The UI can display human-readable local dates, but the backend should define each period deterministically.

### Period Metrics

For the selected period, show:

1. Newly added objects.
2. Newly added confirmed total size.
3. Deleted objects.
4. Deleted total size.
5. Failed object count.
6. Failed run count.
7. Cleanup run count.

Newly added objects are counted from `media.created_at`.

Newly added confirmed size is counted from `media.size_bytes` for rows created in the period and currently confirmed uploaded. Pending rows can count toward `added_objects`, but their bytes must not inflate the confirmed R2 storage total.

Deleted objects and deleted bytes are counted from `media_cleanup_runs.finished_at` and the run totals. This means deletion metrics become accurate from the deployment that starts writing cleanup runs. The PRD does not require reconstructing historical deletions that happened before telemetry existed.

### Additional Useful Metrics

V1 should also include these if inexpensive from existing tables:

- Current due cleanup backlog count and size.
- Next cleanup deadline from `media_post_usages.cleanup_after_at`.
- Storage by content type.
- Storage by media status.
- Recent cleanup runs table.

Do not add expensive R2 bucket scans in V1.

## Goals

1. Give admins a reliable admin page for R2-related operational visibility.
2. Show current confirmed tracked storage size and object count.
3. Show configured bucket name and bucket-level confirmed tracked size.
4. Show selected-period object additions and confirmed total added size.
5. Show selected-period object deletions and total deleted size.
6. Persist cleanup worker run telemetry so deletion metrics survive hard deletes.
7. Show whether the cleanup worker is healthy enough to trust.
8. Show due cleanup backlog so admins can spot stuck cleanup.
9. Avoid exposing R2 secrets to the dashboard.
10. Keep the API and UI extensible for future multi-bucket support.

## Non-goals

- No direct Cloudflare R2 browser integration.
- No R2 credentials or signed admin R2 URLs in frontend responses.
- No per-object delete UI.
- No manual object deletion from the admin page in V1.
- No Cloudflare billing reconciliation in V1.
- No full bucket inventory scan in V1.
- No feature flag.
- No replacement of the media retention policy.
- No change to cleanup eligibility rules except persisting run telemetry.

## Users and Permissions

Only authenticated UniPost admins can view the Object Storage page.

The backend API must live under the existing admin middleware:

```text
GET /v1/admin/object-storage
```

The route does not need super-admin-only access in V1 unless the implementation discovers that bucket names are considered super-admin-sensitive. The page should not expose object keys by default.

## Current Codebase Findings

### Admin UI

- Admin shell and sidebar live in `dashboard/src/app/admin/_components/admin-ui.tsx`.
- Existing pages use `AdminShell`, `StatCard`, `fmtNumber`, and compact admin table styles.
- `Email` currently links to `/admin/email`; Object Storage should be inserted immediately after it.
- `API Metrics` is a good reference for filter bars, stat cards, trend rows, and admin API client patterns.

### API

- Admin routes are registered in `api/cmd/api/main.go` under Clerk session plus `auth.AdminMiddleware`.
- Existing admin aggregate handlers use either `AdminHandler` with raw `pgxpool` or dedicated handlers such as `AdminAPIMetricsHandler`.
- API response shape should use existing response helpers such as `writeSuccess`.

### Storage and Retention

- R2 config is currently a single configured bucket via `R2_BUCKET_NAME`.
- `storage.Client` does not expose bucket inventory or bucket size APIs.
- `media` contains tracked object metadata and size.
- `media.status` is currently written as `pending`, `uploaded`, or `deleted`. The legacy/commented `attached` state is not written by current code and must not be used as a status-based metric.
- `media` has `created_at` and `uploaded_at`, but no `updated_at`.
- `media_post_usages` contains retention cleanup deadlines.
- `MediaCleanupWorker` currently logs cleanup activity but does not persist run metrics.
- `MediaCleanupWorker` currently uses a 24-hour interval and runs once immediately on process start.
- Cleanup hard-deletes media rows after deleting R2 objects, so historical deletion totals must be recorded before rows disappear.

## Data Model

Add a new table:

```sql
media_cleanup_runs
- id text primary key default gen_random_uuid()::text
- worker_name text not null default 'media_cleanup'
- status text not null
- started_at timestamptz not null default now()
- finished_at timestamptz
- next_run_at timestamptz
- scanned_objects integer not null default 0
- deleted_objects integer not null default 0
- deleted_bytes bigint not null default 0
- failed_objects integer not null default 0
- failed_bytes bigint not null default 0
- error_summary text
- created_at timestamptz not null default now()
- updated_at timestamptz not null default now()
```

Allowed statuses:

```text
running
completed
completed_with_errors
failed
skipped
```

Suggested indexes:

```sql
index media_cleanup_runs_started_at_idx on media_cleanup_runs (started_at desc)
index media_cleanup_runs_finished_at_idx on media_cleanup_runs (finished_at desc)
index media_cleanup_runs_status_idx on media_cleanup_runs (status, started_at desc)
unique index media_cleanup_runs_one_running_idx on media_cleanup_runs (worker_name) where status = 'running'
```

Notes:

- `deleted_bytes` should sum `media.size_bytes` for objects that were successfully deleted from R2 and hard-deleted from DB.
- `failed_bytes` should sum `media.size_bytes` for rows the worker attempted but could not fully delete.
- `scanned_objects` should count objects the worker considered in that run, not all rows in the table.
- A run that finds no due rows should still be recorded as `completed` with zero counts. This lets admins verify the worker is alive.
- `next_run_at` is an estimated next run time, not a scheduling guarantee. It should be `started_at + mediaCleanupInterval` for the current 24-hour ticker or another deterministic worker-supplied value. Because the worker also runs immediately on process start, a service restart can make the next real run earlier than the stored estimate.
- Admin summaries should treat rows stuck in `running` for more than two cleanup intervals as stale. The next worker start should mark stale running rows as `failed` with a concise recovery summary before creating a new run.
- `last_run_*` fields in the admin API must be based on the most recent row with `finished_at IS NOT NULL`, not the most recent `started_at`, so stale running rows do not hide the last completed run.

V1 does not require a per-object run item table. If later admins need object-level audit, add `media_cleanup_run_items` separately.

## Backend API

### Get object storage summary

```http
GET /v1/admin/object-storage?period=last_7_days
```

Response:

```json
{
  "data": {
    "period": {
      "key": "last_7_days",
      "from": "2026-06-30T12:00:00Z",
      "to": "2026-07-07T12:00:00Z"
    },
    "current": {
      "tracked_objects": 1280,
      "confirmed_tracked_bytes": 4837219442,
      "pending_objects": 12,
      "uploaded_objects": 640,
      "referenced_objects": 628
    },
    "worker": {
      "last_run_started_at": "2026-07-07T08:00:00Z",
      "last_run_finished_at": "2026-07-07T08:00:12Z",
      "last_run_status": "completed",
      "estimated_next_run_at": "2026-07-08T08:00:00Z",
      "last_deleted_objects": 42,
      "last_deleted_bytes": 91238444,
      "last_failed_objects": 0,
      "active_run_started_at": null,
      "stale_running_runs": 0
    },
    "period_metrics": {
      "added_objects": 320,
      "added_confirmed_bytes": 721944221,
      "deleted_objects": 91,
      "deleted_bytes": 184912333,
      "cleanup_runs": 7,
      "failed_object_count": 1,
      "failed_run_count": 1
    },
    "backlog": {
      "due_objects": 18,
      "due_bytes": 67123420,
      "next_cleanup_deadline_at": "2026-07-07T19:20:00Z"
    },
    "buckets": [
      {
        "bucket_name": "unipost-media",
        "tracked_objects": 1280,
        "confirmed_tracked_bytes": 4837219442,
        "pending_objects": 12,
        "uploaded_objects": 640,
        "referenced_objects": 628,
        "due_objects": 18,
        "due_bytes": 67123420
      }
    ],
    "content_types": [
      {
        "content_type": "video/mp4",
        "tracked_objects": 312,
        "confirmed_tracked_bytes": 3921000000
      }
    ],
    "status_breakdown": [
      {
        "status": "uploaded",
        "tracked_objects": 640,
        "confirmed_tracked_bytes": 1210000000
      }
    ],
    "recent_runs": [
      {
        "id": "6fd5f2c3-2d83-4e42-8ec0-565260a06156",
        "status": "completed",
        "started_at": "2026-07-07T08:00:00Z",
        "finished_at": "2026-07-07T08:00:12Z",
        "deleted_objects": 42,
        "deleted_bytes": 91238444,
        "failed_objects": 0,
        "error_summary": ""
      }
    ]
  },
  "request_id": "req_123"
}
```

### Period validation

Accepted values:

All backend period windows use half-open intervals: `[from, to)`. The response `to` is exclusive and should match the query boundary.

Let `now` be the server's current UTC timestamp and `today_utc` be the current UTC date at `00:00:00`.

| Period | Definition |
| --- | --- |
| `yesterday` | Previous UTC calendar day: `[today_utc - 1 day, today_utc)`. |
| `last_7_days` | Rolling seven-day window: `[now - 7 days, now)`. |
| `last_month` | Previous UTC calendar month: `[first day of previous month 00:00 UTC, first day of current month 00:00 UTC)`. |
| `this_week` | Current ISO week in UTC: `[Monday 00:00 UTC of current ISO week, now)`. |
| `this_month` | Current UTC calendar month: `[first day of current month 00:00 UTC, now)`. |
| `this_year` | Current UTC calendar year: `[January 1 00:00 UTC, now)`. |

Invalid periods return `400 VALIDATION_ERROR`.

## Backend Queries

### Current tracked storage

Count non-deleted rows, but count confirmed stored bytes only from uploaded rows:

```sql
SELECT
  COUNT(*) FILTER (WHERE status != 'deleted') AS tracked_objects,
  COUNT(*) FILTER (WHERE status = 'pending') AS pending_objects,
  COUNT(*) FILTER (WHERE status = 'uploaded') AS uploaded_objects,
  COALESCE(SUM(size_bytes) FILTER (WHERE status = 'uploaded'), 0) AS confirmed_tracked_bytes
FROM media;
```

### Added objects for selected period

Use `media.created_at`. Count newly created rows, but sum confirmed bytes only from rows that have been uploaded:

```sql
SELECT
  COUNT(*) AS added_objects,
  COALESCE(SUM(size_bytes) FILTER (WHERE status = 'uploaded'), 0) AS added_confirmed_bytes
FROM media
WHERE created_at >= $1
  AND created_at < $2;
```

This counts rows still present in `media`. It will not count objects that were created and hard-deleted within the same period unless an object creation audit table is added later. V1 accepts this limitation because the page's primary deletion history is measured from cleanup runs.

### Due cleanup backlog

Use the same eligibility semantics as `ListMediaDueForRetentionCleanup`, but aggregate instead of listing rows. This query must live next to the list query in sqlc and must keep the same eligibility conditions. Future retention-rule changes should update both queries together.

```sql
SELECT COUNT(*), COALESCE(SUM(m.size_bytes), 0)
FROM media m
WHERE m.status != 'deleted'
  AND EXISTS (
    SELECT 1
    FROM media_post_usages due
    WHERE due.media_id = m.id
      AND due.cleanup_after_at IS NOT NULL
      AND due.cleanup_after_at <= NOW()
  )
  AND NOT EXISTS (
    SELECT 1
    FROM media_post_usages blocker
    WHERE blocker.media_id = m.id
      AND (
        blocker.cleanup_after_at IS NULL
        OR blocker.cleanup_after_at > NOW()
      )
  );
```

### Referenced objects

`referenced_objects` is derived from the retention ledger. It must not be computed from `media.status = 'attached'`.

```sql
SELECT COUNT(DISTINCT m.id)
FROM media m
JOIN media_post_usages mpu ON mpu.media_id = m.id
WHERE m.status != 'deleted'
  AND (
    mpu.cleanup_after_at IS NULL
    OR mpu.cleanup_after_at > NOW()
  );
```

### Next cleanup deadline

`next_cleanup_deadline_at` is the next future retention deadline, not the current due backlog:

```sql
SELECT MIN(mpu.cleanup_after_at)
FROM media_post_usages mpu
JOIN media m ON m.id = mpu.media_id
WHERE m.status != 'deleted'
  AND mpu.cleanup_after_at > NOW();
```

### Deleted objects for selected period

Use `media_cleanup_runs`:

```sql
SELECT
  COALESCE(SUM(deleted_objects), 0),
  COALESCE(SUM(deleted_bytes), 0),
  COUNT(*) AS cleanup_runs,
  COALESCE(SUM(failed_objects), 0) AS failed_object_count,
  COUNT(*) FILTER (WHERE status IN ('failed', 'completed_with_errors')) AS failed_run_count
FROM media_cleanup_runs
WHERE finished_at >= $1
  AND finished_at < $2;
```

## Worker Behavior

`MediaCleanupWorker` should record every sweep attempt.

The worker runs every 24 hours and also runs once immediately on process start.

Before a sweep:

1. Mark stale `running` rows older than two cleanup intervals as `failed` with `finished_at = now()` and an `error_summary` such as `stale running cleanup recovered on startup`.
2. Use the partial unique index on `(worker_name) WHERE status = 'running'` as the single-writer guard.
3. If creating the `running` row fails because another active run exists, do not delete objects and do not create a normal cleanup run. A skipped telemetry row is optional, but skipped rows must not affect deleted or failed object totals.

At the start of a sweep:

1. Insert `media_cleanup_runs` row with `status = 'running'`.
2. Set `started_at = now()`.
3. Set `next_run_at = now() + mediaCleanupInterval`.

During the sweep:

1. Track `scanned_objects`.
2. Increment `deleted_objects` and `deleted_bytes` only after both R2 delete and DB hard delete succeed.
3. Increment `failed_objects` and `failed_bytes` when R2 delete, pull-copy delete, or DB hard delete fails.
4. Store a short `error_summary` when failures occur.

At the end:

1. Set `finished_at = now()`.
2. Set status:
   - `completed` when the run completes with no failed objects.
   - `completed_with_errors` when some objects failed but the worker completed.
   - `failed` when the run-level query or setup fails.
   - `skipped` when storage is not configured and the worker intentionally did no work.
3. Update aggregate counters.

The worker should avoid writing unbounded error text. `error_summary` should be concise and redacted.

The admin page should display:

- `last_run_*` from the most recent row where `finished_at IS NOT NULL`.
- `active_run_started_at` from a current non-stale `running` row, if one exists.
- `stale_running_runs` as a small health signal if stale rows had to be recovered.
- `estimated_next_run_at`, not a guaranteed next run time.

## Frontend UX

The page should follow existing admin dashboard conventions:

- Use `AdminShell`.
- Use existing `StatCard` styling where possible.
- Use compact tables rather than large marketing-style cards.
- Use existing admin colors and spacing.
- Avoid new frontend dependencies.

### Header

Title:

```text
Object Storage
```

Description:

```text
Tracked R2 usage, cleanup worker health, and object lifecycle metrics.
```

### Filter Bar

Use a select or segmented control for period:

```text
Yesterday | Last 7 days | Last month | This week | This month | This year
```

The page should reload data when the period changes.

### States

Loading:

- Show the normal admin shell.
- Show stable skeleton or compact loading rows.

Empty:

- If no media exists, show zero metrics and an empty bucket table.
- If no cleanup runs exist yet, show `No cleanup runs recorded yet`.

Error:

- Show inline error banner using existing admin error styling.
- Keep the last successful data visible when possible.

## Formatting Rules

Display bytes as human-readable units:

- B
- KB
- MB
- GB
- TB

Use base 1024 formatting. There is no shared admin byte formatter today, so implementation should add a small shared formatter such as `fmtBytes` to the admin UI utilities instead of copying page-local helpers.

Display dates with:

- Relative time for cards, such as `3h ago`.
- Absolute timestamp in table tooltips or secondary text.

Use clear labels:

- `Confirmed tracked size`, not only `R2 size`, when the value comes from uploaded DB rows.
- `Estimated next run`, not `next run`, because the worker also runs on process start.
- `Deleted in period`, not total all-time deletion unless the period is explicit.
- `Due cleanup`, not `overdue`, unless the deadline has passed.

## Telemetry Accuracy

V1 metrics are application-tracked metrics, not Cloudflare billing inventory.

Known limitations:

- Objects created before size hydration may report `size_bytes = 0` until hydrated.
- Objects created and deleted before `media_cleanup_runs` ships cannot be reconstructed.
- Objects that exist in R2 without a `media` row are not counted.
- Branding assets under `branding/` may be tracked through profile fields, not `media`, and are outside V1 unless implementation adds a safe aggregate.
- Pull-copy objects under `pull/` are deleted with source media by the cleanup worker but may not be separately represented in `media`.

The UI should make these limitations clear through labels, not through a large warning block.

## Testing

### Backend tests

Add tests for:

1. Admin object-storage route registration.
2. Period parser accepts all required filter values.
3. Period parser rejects invalid values.
4. API response includes current, worker, period metrics, backlog, buckets, breakdowns, and recent runs.
5. Cleanup worker writes a `completed` run when no rows are due.
6. Cleanup worker writes deleted object and byte totals after successful deletion.
7. Cleanup worker records object failures without counting failed rows as deleted.
8. Admin summary uses the latest finished run for `last_run_*` when a stale `running` row exists.
9. Cleanup worker does not sweep concurrently when the single-writer guard is unavailable.

### Frontend tests

Add source or component tests for:

1. Admin nav contains `Object Storage` immediately after `Email`.
2. Page calls `GET /v1/admin/object-storage`.
3. Page exposes the required period filters.
4. Page renders cards for confirmed current size, worker last run, estimated next run, added objects, deleted objects, and backlog.

### Manual QA

1. Sign in as an admin on the development app.
2. Open `/admin/object-storage`.
3. Confirm Object Storage appears below Email in the sidebar.
4. Confirm the default period is Last 7 days.
5. Change each period filter and confirm the request changes.
6. Confirm current tracked size and object counts render.
7. Confirm bucket table renders the configured R2 bucket.
8. Confirm recent cleanup runs render when data exists.
9. Confirm a no-run environment shows an empty state instead of failing.

## Rollout

Normal development flow:

1. Implement on `dev-<task-slug>` from latest `origin/dev`.
2. Validate backend tests with `GOCACHE=/tmp/unipost-go-build go test ./...` from `api/`.
3. Validate dashboard build with `npm run build` from `dashboard/`.
4. Merge into local `dev`.
5. Rerun required validation on local `dev`.
6. Push `dev` to `origin/dev`.
7. Wait for development deployment.
8. Verify the page in the real development environment at `https://dev-app.unipost.dev/admin/object-storage`.

No staging or production promotion is included unless explicitly requested.

## Acceptance Criteria

1. Admin sidebar shows `Object Storage` immediately below `Email`.
2. `/admin/object-storage` is restricted to authenticated admins.
3. The page shows current confirmed tracked R2 size and tracked object count.
4. The page shows cleanup worker last run and estimated next run.
5. The page shows selected-period added object count and confirmed added size.
6. The page shows selected-period deleted object count and deleted size from persisted cleanup runs.
7. The page shows R2 bucket name and confirmed tracked bucket size.
8. The page supports the required period filters.
9. The backend persists cleanup worker runs in `media_cleanup_runs`.
10. The worker records zero-delete successful runs so admins can tell it is alive.
11. Stale `running` rows do not hide the most recent finished cleanup run.
12. Concurrent cleanup workers cannot double-count deletion telemetry.
13. The page handles no cleanup run history gracefully.
14. Required backend and dashboard validation pass.
15. Development deployment is verified in the real dev environment before reporting completion of implementation.

## Future Enhancements

- Add Cloudflare bucket inventory or billing reconciliation if admins need actual R2-side object counts.
- Add `media_cleanup_run_items` for per-object audit.
- Include long-lived branding assets in tracked storage metrics.
- Show top workspaces by current storage usage.
- Add storage anomaly alerts.
- Add manual cleanup dry-run preview.
- Add worker health alerting when `last_run_finished_at` becomes stale.
- Add caching or materialized aggregates if full-table content type and status breakdowns become expensive at higher data volume.
