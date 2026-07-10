# Admin Post Duration and Time Metrics Design

**Date:** 2026-07-10  
**Branch:** `dev-admin-post-duration-metrics`  
**Status:** Approved for implementation

## Goal

Expose reliable end-to-end publishing duration in Admin Posts and phase-level timing in each dashboard platform result without compressing the existing admin table columns.

## Terminology

- **Task:** one parent `social_posts` row. A task may target multiple platforms/accounts.
- **Platform post:** one `social_post_results` row within a task.
- **Delivery job:** one `post_delivery_jobs` row associated with a platform post. A platform post may have an initial dispatch job and retry jobs.
- **Measurement baseline:** `social_posts.scheduled_at` for a scheduled task, otherwise `social_posts.created_at`.

## Confirmed Behavior

### Admin Posts columns

Add two columns to `/admin/posts`:

1. **Scheduled**, immediately after **Created**.
   - Show the localized scheduled publishing timestamp when `scheduled_at` exists.
   - Show `—` for immediate tasks.
   - Do not repeat a secondary `scheduled` label under the timestamp.
2. **Duration**, immediately after **Scheduled** and before **Publish Time**.
   - Show integer total seconds, such as `98 s`.
   - Only show a value when the task has at least one platform result and every platform result is `published` with a valid `published_at`.
   - Otherwise show `—`, including `publishing`, `partial`, `failed`, and `cancelled` tasks.

The task duration is:

```text
max(platform_result.published_at) - (post.scheduled_at ?? post.created_at)
```

This makes a multi-platform task's duration equal to the longest platform-post completion time measured from the shared task baseline.

Do not reduce the widths of existing columns other than **Post**. Reduce **Post** from its current 280px minimum to a smaller readable width. Give the table a fixed minimum content width and allow horizontal scrolling when the viewport cannot fit all columns.

### Dashboard platform result Time Metrics

In the expanded task view on the dashboard Posts page, add a **TIME METRICS** panel to every platform result card immediately above **Submitted Settings**.

- Match the border, radius, background, toggle, typography, and spacing of **Submitted Settings**.
- Default to collapsed.
- Show the platform post's total publishing time in the collapsed header when it can be calculated.
- When expanded, show:
  - Total publishing time.
  - Baseline type: `Scheduled` or `Created`.
  - Retry count.
  - Every available phase timestamp.
  - The elapsed duration between adjacent available phases.

The platform post total is:

```text
platform_result.published_at - (post.scheduled_at ?? post.created_at)
```

Display durations below one minute as seconds and longer durations as minutes plus seconds. Preserve sub-second precision for short phase intervals when the timestamp data supports it.

### Phase timestamps

Display the following events in chronological/semantic order:

1. `post.created_at` — task record created.
2. `post.scheduled_at` — requested publish time, when present.
3. `job.created_at` — delivery job queued.
4. `job.first_claimed_at` — first worker claim.
5. `job.platform_started_at` — platform adapter execution started.
6. `job.finished_at` — adapter job execution finished.
7. `result.published_at` — platform confirmed the destination was published.

For scheduled tasks, show the lead time from `created_at` to `scheduled_at`, but do not include it in total publishing time. The total starts at `scheduled_at`.

For synchronous platform success, use one captured completion instant for `job.finished_at` and `result.published_at`. For asynchronous platform processing, `result.published_at` may be later than `job.finished_at`. Never render a negative interval.

### Retry count

Do not add per-attempt history. Show an aggregate retry count for each platform post:

```text
sum(job.attempts for jobs where job.kind == "retry")
```

A retry job that is scheduled but has not been claimed has zero attempts and does not increase the displayed count.

### Historical and incomplete data

Existing records may predate `first_claimed_at` or `platform_started_at`.

- Display the timestamps that exist.
- Display `Not recorded` for a requested phase that is unavailable.
- Only calculate an adjacent phase duration when both endpoints exist and the end is not earlier than the start.
- Do not fabricate or backfill timestamps from unrelated fields.

## Data and API Design

### Admin list response

Extend `adminPostRow` and `AdminPostRow` with:

```text
duration_seconds?: number
```

`scheduled_at` already exists in the admin response.

Compute `duration_seconds` in the Admin Posts SQL rollup using result aggregates. The query must require:

- `COUNT(spr.id) > 0`.
- Every result status equals `published`.
- Every result has a non-null `published_at`.

Return `NULL` otherwise. Clamp invalid negative results to `NULL` rather than exposing negative durations.

### Platform Time Metrics data

Reuse `GET /v1/social-posts/{id}/queue`, which already returns the parent post plus all associated jobs. Expand published tasks to load this endpoint on demand when their task row is opened.

Each `PostDeliveryJob` response includes:

- `created_at` / `queued_at`.
- `first_claimed_at`.
- `platform_started_at`.
- `finished_at`.
- Existing state, kind, attempts, and retry information.

No additional request is needed per platform result. Filter the task-level job array by `social_post_result_id` in the client.

## Component Design

Create small pure helpers for:

- Task duration eligibility and seconds formatting.
- Platform result baseline and total duration.
- Retry count aggregation.
- Phase construction and safe adjacent-duration calculation.

Create a focused `TimeMetricsPanel` component. Keep queue diagnostics behavior separate; Time Metrics is user-facing performance information, while Queue Diagnostics is operational troubleshooting.

The panel handles loading, missing historical data, and complete data without shifting the surrounding result-card layout.

## Error Handling

- Admin duration query failures follow the existing Admin Posts error path.
- If the queue-detail request fails, show the Time Metrics panel with an inline unavailable message; do not hide Submitted Settings or the rest of the platform result.
- Invalid timestamps render as `Not recorded` and do not produce `NaN`, negative durations, or misleading totals.

## Testing

### Backend

- Admin SQL/API contract includes `duration_seconds`.
- Scheduled task duration uses `scheduled_at`.
- Immediate task duration uses `created_at`.
- Multi-platform task uses the latest result `published_at`.
- Partial, failed, active, zero-result, missing-published-time, and negative-duration cases return null.
- Successful delivery writes coherent `finished_at` and `published_at` semantics.

### Frontend

- Column order is Created, Scheduled, Duration, Publish Time.
- Scheduled immediate task displays `—`.
- Scheduled timestamp has no redundant sub-label.
- Duration is seconds in the admin table and `—` for non-eligible tasks.
- Post is the only deliberately narrowed admin column; the table can scroll horizontally.
- Time Metrics is above Submitted Settings and collapsed by default.
- Total duration uses the correct scheduled/immediate baseline.
- Retry count sums executed retry attempts.
- Phase durations are safe for missing, invalid, and asynchronous timestamps.
- Published tasks request queue timing data when expanded.

## Deployment Acceptance

After local checks pass, merge the task branch into local `dev`, rerun backend tests and dashboard build/regression checks, then push `dev` to `origin/dev`.

Wait for all GitHub, Railway, and Vercel development checks/deployments. Verify in the real development environment:

- `https://dev-app.unipost.dev/admin/posts` shows the two columns in the correct order and layout.
- Immediate and scheduled rows show the expected Scheduled and Duration values.
- A dashboard task with multiple published platform results shows the longest task duration.
- Each expanded platform result shows the default-collapsed Time Metrics panel above Submitted Settings.
- Expanded metrics show phase times, safe interval durations, and retry count.
