# Object storage daily size chart

## Goal

Add an admin-only grouped bar chart to `/admin/object-storage` that shows daily storage movement for the selected period:

- **Confirm size** is the total size of media whose R2 upload confirmation completed that day. It is rendered in red.
- **DELETE size** is the total size recorded by cleanup runs that finished that day. It is rendered in green.

The chart makes daily additions and deletions directly comparable without changing the existing summary cards or tables.

## Scope and placement

The chart appears immediately after the window and cleanup-deadline note and before the Buckets section. It uses the same period selector already present on the page, so changing the period reloads both existing metrics and the daily series.

Each selected UTC calendar day is represented, including days without activity. The chart is a grouped bar chart: a red Confirm bar and green DELETE bar share a date group. The x-axis omits intermediate date labels for long periods while preserving the complete daily data series.

The card includes:

- title: `Daily storage movement`
- a red/green legend
- byte-scaled y-axis grid lines
- hover/focus tooltip with the UTC date and exact Confirm and DELETE sizes
- an empty state when all daily values are zero
- responsive horizontal sizing that remains legible on narrow screens

## Data contract

Extend the existing `GET /v1/admin/object-storage?period=...` response with:

```json
{
  "daily_activity": [
    {
      "date": "2026-07-10",
      "confirmed_bytes": 1048576,
      "deleted_bytes": 524288
    }
  ]
}
```

The server owns UTC date boundaries and zero-fills the period, so the browser does not infer historical dates or silently omit zero days.

`confirmed_bytes` is aggregated from `media.size_bytes` for `media.uploaded_at` in the selected half-open UTC period. Only confirmed media (`status = 'uploaded'`) count. `deleted_bytes` is aggregated from `media_cleanup_runs.deleted_bytes` for cleanup runs with `finished_at` in the same selected half-open UTC period. The date of each cleanup row is its UTC `finished_at` date.

No migration is required: the source timestamps and byte counters already exist. The SQL query returns grouped records; the handler maps them onto every UTC day in the requested period.

## Implementation boundaries

- **Database query layer:** add grouped daily confirmation and deletion queries next to the existing object-storage queries and regenerate the checked-in sqlc output.
- **Handler:** request both daily aggregates, construct a zero-filled `daily_activity` response, and preserve all existing response fields and error behavior.
- **Dashboard API types:** add a typed `AdminObjectStorageDailyActivity` array to the existing response model.
- **Dashboard chart component:** use a local, accessible SVG/HTML implementation rather than adding a charting dependency. It accepts only normalized API rows, calculates a shared scale from both series, supports keyboard focus, and exposes values in text for assistive technology.
- **Page:** render the chart card only after a successful response; while data is loading, retain a stable card footprint. When every value is zero, show a clear no-activity state.

## Error handling and compatibility

The existing endpoint-level error banner remains the failure surface. A daily aggregation query failure fails the endpoint consistently with other object-storage queries; the page never renders misleading partial activity data. Existing clients remain compatible because `daily_activity` is additive.

## Validation

Tests cover:

- database query definitions and generated query-method presence;
- handler response values for multiple days, UTC boundaries, and zero-filled inactive days;
- dashboard source semantics for the new response field, red Confirm and green DELETE rendering, and accessible chart labels;
- the existing Go suite, dashboard regression suite, and dashboard production build.

Before delivery, verify the changed page visually in a local browser. After the eventual `origin/dev` push, wait for the development deployment and verify the real dev application at `https://dev-app.unipost.dev/admin/object-storage` with an authenticated admin session.
