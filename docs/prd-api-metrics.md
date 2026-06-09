# PRD - Developer API metrics

**Status:** Planning
**Owner:** API / Dashboard / Admin
**Created:** 2026-06-09
**Target:** Workspace-scoped and admin-visible metrics for UniPost Developer API usage

---

## Problem

UniPost customers integrate with the Developer API through API keys, but they do not yet have a complete way to answer basic operational questions:

- Which endpoints are slow?
- Did latency increase in the last 24 hours?
- How many calls did this workspace make?
- Are failures caused by client-side integration errors, rate limiting, or UniPost server failures?
- Which endpoints account for most API volume?

UniPost admins also need a product-wide view of Developer API health across all customer workspaces. Without that view, admin debugging relies on logs and manual database inspection instead of a focused metrics surface.

UniPost already has the foundation for this work, but the existing implementation is not safe to treat as production-ready metrics infrastructure without fixes:

- `api_metrics` stores per-request API metrics.
- `api/internal/metrics/middleware.go` records API-key-authenticated requests only.
- `api/internal/handler/api_metrics.go` exposes workspace-scoped metrics endpoints.
- `dashboard/src/app/(dashboard)/projects/[id]/analytics/api/page.tsx` already renders a basic API metrics page.

Known implementation gaps that V1 must fix before the data can be trusted:

- The current recorder starts an async insert using `r.Context()` after the handler returns. The request context is canceled when the response completes, so inserts can be silently dropped.
- The current recorder is mounted on the whole workspace route group and has no explicit exclusion for `/v1/api-metrics/*`, so metrics queries can pollute the metrics being measured.
- The current path normalizer uses string heuristics. It can collapse valid route segments such as `social-posts` while missing short IDs such as `/v1/posts/42/publish`.
- The current time-range parser silently ignores invalid timestamps and falls back to defaults.
- The current overall and trend queries do not expose the full V1 taxonomy, including `rate_limited_count`, `error_rate_pct`, `server_failure_rate_pct`, `avg_ms`, status-code distribution, or day-level trends.

This PRD turns that foundation into a first-class customer and admin product surface.

## Product direction

Developer API metrics should be available in two places:

1. **Dashboard UI** for workspace users who prefer visual inspection.
2. **Developer API endpoints** for customers who want to query, automate, or import UniPost API metrics into their own monitoring tools.

V1 should focus on API-key-authenticated Developer API calls only. It should not try to become a full internal APM system for dashboard, admin, OAuth callback, webhook callback, or health-check traffic.

Admin users should see the same metric family aggregated across all customer workspaces, still limited to Developer API traffic.

No feature flag is required for this work.

## V1 decisions

1. Add a nullable `api_key_id` column to `api_metrics` and write it from `auth.GetAPIKeyID`. V1 does not expose per-key metrics, but collecting the column now preserves future analytics without backfill gaps.
2. Workspace API metrics are readable by any current workspace role. The data is operational visibility for the workspace, not a destructive admin action.
3. API-key callers can read workspace metrics because the API key already authenticates to that workspace.
4. Raw-query-backed time ranges are capped at 90 days. Requests over 90 days return `400 VALIDATION_ERROR`; they are not silently clamped.
5. Admin API metrics use the existing admin gate, not super-admin-only access.
6. `429` responses are counted in `client_error_count` and `error_rate_pct` because they are non-success HTTP responses. They are also exposed separately as `rate_limited_count` so product copy and debugging do not confuse rate limiting with validation or auth errors.

## Goals

1. Let workspace users view Developer API latency in the dashboard.
2. Let workspace users query their Developer API metrics through public API endpoints.
3. Let admin users view product-wide Developer API metrics across all workspaces.
4. Include latency, volume, success, error, rate-limit, status-code, endpoint, and time-trend metrics.
5. Keep metrics workspace-isolated for non-admin users.
6. Prevent metrics-query endpoints from polluting the metrics being measured.
7. Normalize endpoint paths to avoid resource ID leakage and high-cardinality data.
8. Keep V1 simple enough to ship without external observability infrastructure.

## Non-goals

- No full internal APM replacement.
- No customer-visible dashboard metrics for Clerk/session dashboard traffic.
- No metrics for OAuth callbacks, public hosted flows, inbound webhooks, or health checks.
- No alerting, SLO policy engine, anomaly detection, or incident workflow in V1.
- No per-request trace viewer in V1.
- No external provider latency metrics in V1, such as Meta, TikTok, X, Pinterest, or LinkedIn publishing latency.
- No request or response body capture.
- No display of raw resource IDs in metric paths.
- No feature flag.

## Users and permissions

### Workspace users

Workspace users can view metrics for the current workspace only.

The public metrics API must return metrics for the workspace resolved from authentication context:

- API key auth resolves `workspace_id` from the API key.
- Clerk session auth resolves `workspace_id` from the user's active workspace membership.

The API must never accept a caller-supplied `workspace_id` for normal workspace metrics reads.

### Admin users

UniPost admins can view aggregate Developer API metrics across all workspaces through `/v1/admin/api-metrics/*` and `/admin/api-metrics`.

Admin endpoints must use the existing Clerk admin middleware. Non-admin users receive `403`.

V1 may allow admins to drill down by workspace, but the admin API response should still use normalized paths and should not expose raw request URLs with customer resource IDs.

## Metrics taxonomy

V1 should expose a compact but complete observability bundle.

### Required metrics

| Metric | Definition | Customer value |
| --- | --- | --- |
| `total_calls` | Count of matching API requests | Measures API usage and endpoint volume |
| `success_count` | Count of responses with status `< 400` | Shows calls that completed successfully |
| `client_error_count` | Count of responses with status `400-499` | Separates integration/auth/validation issues from UniPost failures |
| `server_error_count` | Count of responses with status `>= 500` | Measures UniPost-side failures |
| `rate_limited_count` | Count of responses with status `429`; also included in `client_error_count` | Highlights quota or rate-limit pressure |
| `error_rate_pct` | `(client_error_count + server_error_count) / total_calls * 100` | Overall failed-request ratio |
| `server_failure_rate_pct` | `server_error_count / total_calls * 100` | UniPost reliability signal |
| `p50_ms` | 50th percentile of `duration_ms` | Typical latency |
| `p95_ms` | 95th percentile of `duration_ms` | Primary customer-facing latency signal |
| `p99_ms` | 99th percentile of `duration_ms` | Long-tail latency |
| `avg_ms` | Average `duration_ms` | Secondary latency reference |
| `status_code` distribution | Counts by exact HTTP status code | Explains whether errors are `401`, `403`, `422`, `429`, `500`, etc. |

### Naming guidance

Avoid calling all `4xx` responses "failures" in product copy. Some `4xx` responses are expected integration feedback. Use:

- **Errors** for `4xx + 5xx`.
- **Client errors** for `4xx`.
- **Server failures** for `5xx`.
- **Rate limited** for `429`.
- **Reliability** for non-`5xx` request ratio.

### Deferred metrics

| Metric | Reason to defer |
| --- | --- |
| Per API key breakdowns | V1 records `api_key_id`, but API/UI exposure needs a separate security and UX pass |
| Error-code breakdown | Requires consistent handler-level application error codes in responses and metrics writes |
| Request/response size | More storage and privacy considerations |
| External provider latency | Belongs to publish/job/platform metrics, not HTTP API latency |
| End-user scoped metrics | Requires reliable managed-user attribution per API request |
| Alerts and SLOs | Needs stable thresholds and notification rules after initial data collection |

## Metric scope

### Included in V1

Only API-key-authenticated Developer API traffic is included.

Examples:

- `GET /v1/workspace`
- `GET /v1/profiles`
- `POST /v1/media`
- `POST /v1/posts`
- `POST /v1/posts/{id}/publish`
- `GET /v1/webhooks`
- `GET /v1/usage`

Implementation should define the measurable route set deliberately. Prefer one of these approaches:

1. Mount the metrics recorder only around business Developer API routes.
2. Maintain an explicit measurable-route registry based on chi route patterns.

Do not rely on the current "entire workspace group plus implicit behavior" shape. If the implementation uses a deny-list as a transitional step, tests must cover every excluded prefix listed below.

### Excluded from V1

The metrics recorder must not count:

- `/v1/api-metrics/*`
- `/v1/admin/*`
- `/v1/me/*`
- `/v1/public/*`
- OAuth callbacks
- inbound provider webhooks
- health checks
- WebSocket routes
- dashboard-only Clerk/session traffic

`/v1/api-metrics/*` is especially important to exclude. A customer polling metrics should not create new metrics rows for the metrics API itself.

## Public workspace API

The workspace metrics API is part of the Developer API surface and should be documented for customers.

All endpoints require normal workspace authentication. API key auth is required for third-party/customer automation. Clerk session auth may continue to support the dashboard. Any active workspace role can read these metrics for the current workspace.

Time range behavior:

- If both `from` and `to` are omitted, default to the last 7 days.
- If either `from` or `to` is present but invalid, return `400 VALIDATION_ERROR`.
- If `from > to`, return `400 VALIDATION_ERROR`.
- If the range exceeds 90 days, return `400 VALIDATION_ERROR`.

### Overall

```http
GET /v1/api-metrics/overall?from=2026-06-01T00:00:00Z&to=2026-06-09T00:00:00Z
Authorization: Bearer <api_key>
```

Returns aggregate usage, error, and latency stats for the workspace.

Example response:

```json
{
  "success": true,
  "data": {
    "total_calls": 18423,
    "success_count": 17980,
    "client_error_count": 391,
    "server_error_count": 52,
    "rate_limited_count": 17,
    "error_rate_pct": 2.41,
    "server_failure_rate_pct": 0.28,
    "reliability_pct": 99.72,
    "p50_ms": 118,
    "p95_ms": 642,
    "p99_ms": 1484,
    "avg_ms": 184
  }
}
```

### Endpoint summary

```http
GET /v1/api-metrics/summary?from=2026-06-01T00:00:00Z&to=2026-06-09T00:00:00Z&sort=p95_ms_desc&limit=50
Authorization: Bearer <api_key>
```

Returns per-method and per-normalized-path stats.

Example row:

```json
{
  "method": "POST",
  "path": "/v1/posts/:id/publish",
  "total_calls": 3124,
  "success_count": 3061,
  "client_error_count": 52,
  "server_error_count": 11,
  "rate_limited_count": 3,
  "error_rate_pct": 2.02,
  "server_failure_rate_pct": 0.35,
  "p50_ms": 221,
  "p95_ms": 934,
  "p99_ms": 2140,
  "avg_ms": 310
}
```

Recommended query params:

| Param | Type | Default | Notes |
| --- | --- | --- | --- |
| `from` | RFC3339 timestamp | now - 7 days | Inclusive lower bound |
| `to` | RFC3339 timestamp | now | Inclusive upper bound |
| `method` | string | all | Optional `GET`, `POST`, `PATCH`, `DELETE` |
| `path` | string | all | Must match normalized path |
| `status_class` | string | all | Optional `2xx`, `3xx`, `4xx`, `5xx` |
| `sort` | string | `total_calls_desc` | `total_calls_desc`, `p95_ms_desc`, `p99_ms_desc`, `server_errors_desc`, `rate_limited_desc` |
| `limit` | integer | 50 | Max 200 |

### Time trend

```http
GET /v1/api-metrics/trend?from=2026-06-01T00:00:00Z&to=2026-06-09T00:00:00Z&interval=hour
Authorization: Bearer <api_key>
```

Returns time-bucketed volume, error, and latency stats.

Supported `interval` values:

- `hour`
- `day`

V1 should default to `hour` for ranges up to 7 days and `day` for longer ranges unless an explicit interval is provided.

The current SQL hardcodes `date_trunc('hour', ...)`. V1 must parameterize interval selection through a safe server-side switch, not by interpolating arbitrary query strings into SQL.

### Status codes

```http
GET /v1/api-metrics/status-codes?from=2026-06-01T00:00:00Z&to=2026-06-09T00:00:00Z
Authorization: Bearer <api_key>
```

Returns counts by exact HTTP status code, optionally grouped by endpoint.

Example row:

```json
{
  "status_code": 422,
  "total_calls": 184,
  "method": "POST",
  "path": "/v1/posts"
}
```

## Admin API

Admin metrics endpoints live under `/v1/admin/api-metrics/*`.

They use the same response concepts as the workspace API, but aggregate across all workspaces by default.

Required endpoints:

```http
GET /v1/admin/api-metrics/overall
GET /v1/admin/api-metrics/summary
GET /v1/admin/api-metrics/trend
GET /v1/admin/api-metrics/status-codes
GET /v1/admin/api-metrics/workspaces
```

Admin-only query params:

| Param | Type | Notes |
| --- | --- | --- |
| `workspace_id` | string | Optional drilldown to one workspace |
| `workspace_query` | string | Optional search by workspace name or id if supported efficiently |
| `min_calls` | integer | Hide noisy low-volume rows |
| `only_server_failures` | boolean | Focus on `5xx` rows |

Admin workspace rows should include:

- workspace id
- workspace name when available
- total calls
- p95 latency
- p99 latency
- error rate
- server failure rate
- 429 count
- slowest endpoint by p95

## Dashboard UX

### Workspace dashboard

Use the existing `Analytics -> API` page as the customer entry point.

Required content:

- Time range segmented control: `24h`, `7d`, `30d`, `90d`
- Metric cards:
  - Total calls
  - Reliability
  - Client errors
  - Server failures
  - Rate limited
  - Latency p95 with p50 and p99 as supporting text
- Trend view:
  - Calls over time
  - Error count or error rate over time
  - p95 latency over time
- Per-endpoint table:
  - method
  - normalized endpoint
  - calls
  - success
  - 4xx
  - 429
  - 5xx
  - error rate
  - p50
  - p95
  - p99

The page should be dense and operational. It should avoid marketing-style hero treatment, oversized cards, or decorative visuals.

Empty state:

- If no API-key calls exist, explain that metrics appear after API keys are used.
- Include a link to API keys or docs if existing navigation supports it.

Error state:

- Show an inline load failure and a retry action.

### Admin dashboard

Add an admin sidebar entry:

```text
API Metrics -> /admin/api-metrics
```

Recommended section: `System`.

Required content:

- Global overview cards.
- Product-wide trend chart.
- Endpoint summary table.
- Workspace impact table.
- Filters for range, method, endpoint, status class, and workspace.
- Sort controls for p95, p99, total calls, server failures, and 429s.

The admin page should prioritize scanning and diagnosis:

- Use monospace for numbers.
- Keep tables compact.
- Highlight high p95, high p99, high 5xx rate, and high 429 volume.
- Avoid exposing raw request paths with customer IDs.

## Backend requirements

### Recording

The metrics middleware should continue to be non-blocking for responses.

Requirements:

- Record only API-key-authenticated Developer API requests in the measurable route set.
- Skip the excluded paths listed in this PRD.
- Fix the current async insert context bug. The goroutine must not use the cancelable request context after the handler returns. Use `context.WithoutCancel(r.Context())` plus a short timeout, or an independent background context with timeout.
- Capture:
  - workspace id
  - API key id
  - method
  - normalized path
  - status code
  - duration ms
  - created at
- Preserve response behavior if metrics insert fails.
- Do not silently swallow insert failures during development and tests. Production logging should be low-noise and should not expose secrets.
- Add tests that prove fast requests still persist metrics rows after the response has completed.

### Path normalization

The current string heuristic normalizer must be replaced for V1.

Use the matched chi route pattern from the request route context as the primary normalized path source. For example:

- `/v1/posts/42/publish` should record as `/v1/posts/{id}/publish` or `/v1/posts/:id/publish`.
- `/v1/social-posts` should remain `/v1/social-posts`, not collapse to `/v1/:id`.
- `/v1/api-metrics/summary` should be excluded before it is recorded.

Requirements:

- Prefer `chi.RouteContext(r.Context()).RoutePattern()` after the downstream handler has matched the route.
- Convert placeholder style consistently if the product wants `:id` instead of `{id}`.
- Use a conservative fallback only when no route pattern exists.
- Never store query strings.
- Treat path normalization correctness as a privacy requirement, not only a display concern.

### Data model

The existing table should be extended for V1:

```sql
api_metrics (
  id,
  workspace_id,
  api_key_id,
  method,
  path,
  status_code,
  duration_ms,
  created_at
)
```

Required schema updates:

- Add nullable `api_key_id TEXT REFERENCES api_keys(id) ON DELETE SET NULL`.
- Preserve or create the workspace-scoped time index for the customer query path:

```sql
CREATE INDEX IF NOT EXISTS idx_api_metrics_workspace_time
  ON api_metrics (workspace_id, created_at DESC);
```

- Preserve or create the workspace endpoint/time index for per-endpoint customer summaries:

```sql
CREATE INDEX IF NOT EXISTS idx_api_metrics_workspace_path_time
  ON api_metrics (workspace_id, path, created_at DESC);
```

- Add an index for admin/global time-range queries:

```sql
CREATE INDEX IF NOT EXISTS idx_api_metrics_time_path
  ON api_metrics (created_at DESC, path, method);
```

### Query behavior

Time range:

- Default range is last 7 days.
- Maximum range is 90 days for raw-query-backed V1.
- Invalid `from` or `to` returns `400 VALIDATION_ERROR`, not silent fallback.
- `from > to` returns `400 VALIDATION_ERROR`.
- Ranges longer than 90 days return `400 VALIDATION_ERROR`, not clamped results.
- Replace the current silent-fallback `parseTimeRange` behavior.

Aggregation:

- Use percentile calculations in SQL for `p50`, `p95`, and `p99`.
- Extend existing overall queries to include `rate_limited_count`, `error_rate_pct`, `server_failure_rate_pct`, and `avg_ms`.
- Add status-code distribution queries.
- Add trend queries for both hourly and daily buckets.
- Return zeroes for empty overall responses.
- Return empty arrays for empty list responses.

Performance:

- Limit per-endpoint summary responses.
- Use indexes that support workspace-scoped and admin time-range queries.
- If raw queries become too slow after launch, add a rollup table in a later PRD.

## Privacy and security

- Store normalized paths only.
- Do not store query strings.
- Do not store request bodies.
- Do not store response bodies.
- Do not expose raw resource IDs in metrics responses.
- Do not allow normal users to pass arbitrary `workspace_id`.
- Do not expose API key secret values or full API key identifiers in V1 metrics responses.
- Admin views may include workspace names and IDs because they are already admin-visible elsewhere.
- Route-pattern-based normalization is a launch blocker. If route normalization leaks short numeric IDs, long IDs, UUIDs, or customer resource identifiers, the implementation does not meet this PRD.

## Documentation requirements

Add or update public docs for:

- What API metrics measure.
- Which requests are included and excluded.
- Metric definitions.
- Endpoint examples.
- Query params.
- Response examples.
- Guidance on interpreting `4xx`, `5xx`, and `429`.

The docs should be explicit that API metrics are delayed only by normal request/write latency and do not include external social-platform publishing latency.

## Testing requirements

Backend tests:

- API-key request records a metrics row.
- API-key request records the `api_key_id` internally.
- Clerk/dashboard request does not record a metrics row.
- `/v1/api-metrics/*` request does not record a metrics row.
- Fast requests still record metrics after the HTTP response returns; async writes must not be canceled by `r.Context()`.
- Path normalization uses route templates instead of string-length heuristics.
- Path normalization keeps valid long route segments such as `/v1/social-posts`.
- Path normalization normalizes short IDs such as `/v1/posts/42/publish`.
- Path normalization removes UUID-like and long ID-like resource segments.
- Workspace metrics endpoints only return current workspace data.
- Admin metrics endpoints aggregate across workspaces.
- Non-admin caller receives `403` for admin metrics endpoints.
- Invalid time ranges return `400`.
- Ranges over 90 days return `400`.
- Trend supports both `interval=hour` and `interval=day`.
- Status-code distribution separates `2xx`, `4xx`, `429`, and `5xx`.
- `429` is included in `client_error_count` and also returned as `rate_limited_count`.

Frontend tests or regression coverage:

- Workspace API metrics page renders empty, loading, error, and populated states.
- Admin API metrics page blocks non-admins.
- Admin API metrics page renders global overview, endpoint table, and workspace table.
- Range and sort controls trigger the correct API calls.

Local validation:

- Backend/API changes: from `api/`, run `GOCACHE=/tmp/unipost-go-build go test ./...`.
- Dashboard changes: from `dashboard/`, run `npm run build`.
- Dashboard shell/admin routing changes: from `dashboard/`, run `npm run test:regression:dashboard` when Playwright browsers are installed.

Deployed dev validation:

- Push to `origin/dev`.
- Wait for development deployment to finish.
- Use development domains only.
- Generate at least one API-key Developer API request in dev.
- Verify the workspace dashboard shows the request in API metrics.
- Verify the public metrics API returns the request.
- Verify the admin page shows the request in product-wide metrics.
- Verify polling `/v1/api-metrics/*` does not increase business endpoint counts.

## Acceptance criteria

1. A customer can call `GET /v1/api-metrics/overall` with an API key and receive metrics for their workspace.
2. A customer can call `GET /v1/api-metrics/summary` and see per-endpoint latency and error metrics.
3. A customer can call `GET /v1/api-metrics/trend` and see time-bucketed API volume and latency.
4. A customer can call `GET /v1/api-metrics/status-codes` and see status-code counts.
5. The dashboard `Analytics -> API` page shows the same workspace-scoped metrics.
6. Metrics-query endpoints do not create metrics rows for themselves.
7. Admin users can view product-wide Developer API metrics at `/admin/api-metrics`.
8. Non-admin users cannot access admin metrics APIs or pages.
9. Dashboard/session traffic is not included in customer Developer API metrics.
10. Raw resource IDs and query strings are not exposed in metrics responses.
11. Local validation passes for changed backend and dashboard surfaces.
12. Dev deployment is monitored and self-accepted before the task is reported complete after implementation.

## Rollout plan

1. Fix recorder correctness:
   - replace cancelable async insert context
   - add measurable-route inclusion or explicit route registry
   - exclude metrics endpoints and non-business routes
   - replace string heuristic path normalization with chi route-pattern normalization
2. Add schema migration:
   - nullable `api_key_id`
   - workspace query indexes
   - admin time-range index
3. Extend backend queries and handler responses for the full V1 metric taxonomy.
4. Replace silent time-range parsing with validation and 90-day limit handling.
5. Add hourly and daily trend support.
6. Add status-code distribution support.
7. Add admin metrics queries and admin handlers.
8. Update dashboard workspace API metrics page.
9. Add admin API metrics page and sidebar entry.
10. Add public API documentation.
11. Run local validation.
12. Merge into local `dev`, rerun validation, push `dev`, wait for dev deployment, and self-accept in the development environment.

## Resolved decisions

1. `api_key_id` is added and recorded in V1, but not exposed in V1 API responses.
2. Workspace metrics are readable by any workspace role.
3. Raw metrics query range is capped at 90 days.
4. Admin metrics require normal admin access, not super-admin access.

## Remaining open decisions

No product decisions are blocking implementation. Future PRDs can revisit per-key analytics, longer retention through rollups, external provider latency, and alerting.
