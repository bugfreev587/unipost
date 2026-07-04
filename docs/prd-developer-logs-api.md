# PRD - Developer Logs API and real-time workspace log access

**Status:** Planning
**Owner:** API / Dashboard / Developer Experience
**Created:** 2026-06-17
**Target:** Productized workspace-scoped logs API, documented setup flow, and real-time log access for customer integrations

---

## Problem

UniPost has a useful Developer Logs surface at:

```text
/projects/{profile_id}/logs
```

The current implementation is workspace-scoped and does not expose global public-user logs to normal users. However, the product surface is not yet clear enough for customers who want to set up and operate against the UniPost API:

- The dashboard route looks profile-scoped, but the page currently reads workspace logs.
- `GET /v1/logs` and `GET /v1/logs/{id}` exist and are workspace-scoped, but they are not documented in `/docs/api`.
- The REST list API is useful for polling, but it lacks cursor pagination for reliably fetching all logs over time.
- The live dashboard tail uses a Clerk-session WebSocket, not an API-key-accessible real-time API.
- Existing developer webhooks send post and account lifecycle events, but they do not stream every integration log event.
- SDKs and public examples do not expose a first-class `logs` client.
- There is no focused test suite proving workspace log isolation for normal customer routes.
- The shared response envelope already has cursor metadata fields, but the logs list endpoint still returns non-cursor list metadata.

This creates product uncertainty: customers can inspect logs in the dashboard, and can partially query logs through REST, but there is no clearly documented "use this to retrieve all logs for your account in near real time" story.

## Current verified behavior

### Workspace-scoped user logs

`GET /v1/logs` and `GET /v1/logs/{id}` are mounted in the workspace-authenticated route group. The auth layer accepts either a workspace API key or a Clerk session JWT and stamps `workspace_id` into request context.

The logs handler never accepts caller-supplied `workspace_id` for normal reads. It reads `workspace_id` from request context and passes it to the database query.

The SQL queries enforce:

```sql
WHERE workspace_id = sqlc.arg('workspace_id')::text
```

for list reads, and:

```sql
WHERE id = $1 AND workspace_id = $2
```

for detail reads.

### Global admin logs are separate

Global logs live behind `/v1/admin/logs` and `/v1/admin/logs/{id}`. Those routes are protected by the admin middleware plus a super-admin requirement.

Admin logs intentionally join workspaces, users, and subscriptions so operators can search by workspace and owner email. That shape is not used by the normal customer logs endpoint.

### Dashboard live tail is workspace-broadcasted

The current dashboard live tail uses `/v1/logs/ws?token=...`. New log notifications include `workspace_id`, and the WebSocket hub broadcasts messages only to connections registered under the matching workspace.

The real-time infrastructure already exists:

- `api/internal/ws/pgnotify.go` runs one shared PostgreSQL `LISTEN logs_events` loop per API process.
- `ws.NotifyLog()` publishes new log envelopes into `logs_events`.
- The shared `logsHub` broadcasts those envelopes by workspace.

The SSE implementation must reuse or generalize this shared pipeline. It must not create one PostgreSQL `LISTEN` connection per SSE client.

### Response envelope is partly ready for cursors

`api/internal/handler/response.go` already defines `MetaResponse.HasMore` and `MetaResponse.NextCursor`, plus `writeSuccessWithCursor`. The logs list endpoint still uses `writeSuccessWithListMeta`, so cursor pagination is a backend implementation gap, not a new response-envelope design.

### Dashboard HTTP request logs are intentionally limited

`api/internal/integrationlogs/middleware.go` skips dashboard Clerk-session HTTP request traffic when no API key id is present. This means customer Developer Logs currently focus on API-key requests, publishing/OAuth/webhook/worker events, and explicit integration-log writes. Dashboard clicks are not a full audit trail.

V1 should keep this positioning: Developer Logs are for customer integrations and delivery diagnostics, not every dashboard page request. If product wants dashboard-click observability later, that should be a separate audit/activity-log decision.

### Product mismatch

`dashboard/src/app/(dashboard)/projects/[id]/logs/page.tsx` reads the route param as `profileId`, but the current list request does not include `profile_id={profileId}` by default. The page therefore behaves as a workspace logs console inside a profile/project shell.

## Product direction

Make Developer Logs a first-class workspace observability surface.

The product should have three supported access patterns:

1. **Dashboard:** human-friendly workspace log search, filtering, and log-detail inspection.
2. **REST API:** customer-controlled polling and backfill through API-key-authenticated endpoints.
3. **Real-time API:** API-key-authenticated server-to-server stream for near real-time log ingestion.

V1 should make REST and real-time access good enough for ordinary API customers. Webhook delivery for every log event should be treated carefully because log-created webhooks can recurse through webhook-delivery logs.

The real-time API should be built as a thin API-key-accessible streaming layer over the existing shared `logsHub`, not as a second database notification subsystem.

## Goals

1. Confirm and preserve workspace isolation for all normal customer log routes.
2. Document `GET /v1/logs` and `GET /v1/logs/{id}` in `/docs/api`.
3. Add cursor pagination to `GET /v1/logs` so customers can reliably backfill logs.
4. Add an API-key-authenticated real-time log stream endpoint over the existing shared logs notification hub.
5. Add SDK methods for list, get, and stream helpers where supported by the language.
6. Clarify Dashboard Logs scope as workspace logs, or default-filter the page to the route profile.
7. Add automated tests that fail if workspace log isolation regresses.
8. Keep redaction and payload-size guarantees explicit in public documentation.

## Non-goals

- No global logs access for normal users.
- No customer-supplied `workspace_id` filter on normal workspace log endpoints.
- No public access to `/v1/admin/logs`.
- No unredacted tokens, cookies, Authorization headers, client secrets, or webhook secrets in logs.
- No browser-direct connection to Unleash or internal observability providers.
- No replacement for Railway, Vercel, BetterStack, or internal operator logs.
- No alerting product in V1.
- No webhook delivery for every log event in the first implementation unless recursion controls are implemented and reviewed.
- No per-SSE-client PostgreSQL `LISTEN` connection.

## Feature flag decision

Before implementation starts, ask whether this should be protected by a feature flag, as required for API-layer and Dashboard-layer changes.

Recommended flag if approved:

```text
logs.public_api_v1
```

Suggested behavior if a flag is used:

- Production default: off.
- Development default: on only after backend fallback is safe.
- Frontend visibility: expose through `GET /v1/me/features`.
- Backend authority: all behavior checks run through `api/internal/featureflags`.
- Documentation update: add key, owner, production default, rollback action, and third-party dependencies to `docs/feature-flags-unleash.md`.

If no feature flag is approved, ship without adding a flag.

## Users and permissions

### Workspace users

Workspace members can view logs for their current workspace only.

For Clerk-session users:

- Resolve workspace from active membership.
- Return only logs whose `workspace_id` matches that resolved workspace.

For API-key users:

- Resolve workspace from the API key.
- Return only logs whose `workspace_id` matches the API key's workspace.

### UniPost admins

UniPost admins continue to use `/admin/logs` and `/v1/admin/logs`.

Admin endpoints may expose:

- `workspace_id`
- workspace name
- owner email
- plan
- global filters across all workspaces

Normal customer endpoints must not use the admin response shape.

## Retention and plan behavior

Integration log retention is already enforced by `api/internal/worker/integration_logs_retention.go`. The worker runs once at startup and then every 24 hours. It lists workspaces and plan ids, calls `integrationlogs.RetentionDaysForPlan`, and deletes expired rows with `DeleteExpiredIntegrationLogsForWorkspace`.

Current retention windows:

| Plan | Retention |
| --- | ---: |
| `free` | 1 day |
| `api` | 7 days |
| `basic` | 14 days |
| `growth` | 30 days |
| `team` | 90 days |
| `enterprise` | 180 days |
| Unknown / fallback | 7 days |

V1 should not add a new plan gate for `GET /v1/logs`, `GET /v1/logs/{id}`, or `GET /v1/logs/stream`. The product limit is retention duration, not read access. If the business wants stream access to be paid-only, that must be a separate product decision before implementation.

Backfill means "all logs still retained for the authenticated workspace," not all historical logs ever created.

## API requirements

### `GET /v1/logs`

Returns log rows for the authenticated workspace.

Authentication:

```http
Authorization: Bearer <unipost_api_key>
```

or a Clerk session JWT from the dashboard.

Supported query parameters:

| Parameter | Type | Description |
| --- | --- | --- |
| `category` | string | One of `publishing`, `api_request`, `oauth`, `webhook`, `system`. |
| `action` | string | Exact log action, for example `post.publish.platform_failed`. |
| `source` | string | One of `api`, `dashboard`, `worker`, `webhook`, `oauth`. |
| `level` | string | One of `debug`, `info`, `warn`, `error`. |
| `status` | string | One of `success`, `warning`, `error`. |
| `platform` | string | Platform token such as `instagram`, `tiktok`, `youtube`, or `linkedin`. |
| `profile_id` | string | Filters logs associated with one profile. |
| `social_account_id` | string | Filters logs associated with one connected account. |
| `post_id` | string | Filters logs associated with one post. |
| `request_id` | string | Filters logs associated with one API request. |
| `error_code` | string | Filters logs by normalized error code. |
| `q` | string | Searches message, action, request id, post id, and error code. |
| `from` | RFC3339 timestamp | Inclusive lower bound. |
| `to` | RFC3339 timestamp | Inclusive upper bound. |
| `limit` | integer | Page size. Default `100`, maximum `500`. |
| `cursor` | string | Opaque cursor returned by the previous response. |

Response:

```json
{
  "data": [
    {
      "id": 110966,
      "workspace_id": "ae267ee2-298d-4fa8-b6a0-c386000b17af",
      "ts": "2026-06-17T20:16:34.476752Z",
      "level": "error",
      "status": "error",
      "category": "oauth",
      "action": "account.connect.callback_failed",
      "source": "oauth",
      "message": "Failed to persist connected account.",
      "request_id": "req_abc123",
      "profile_id": "4ec8ee48-9119-40ad-b4ca-99992e965316",
      "social_account_id": "sa_abc123",
      "post_id": "post_abc123",
      "platform": "instagram",
      "endpoint": "/v1/posts",
      "method": "POST",
      "http_status_code": 422,
      "remote_status_code": 400,
      "duration_ms": 132,
      "error_code": "account_save_failed",
      "metadata": {
        "connect_session_id": "c62ea650-a89e-4811-9842-e2858441c5fb"
      }
    }
  ],
  "meta": {
    "limit": 100,
    "has_more": false,
    "next_cursor": null
  },
  "request_id": "req_response_123"
}
```

List responses should not include `request_payload` or `response_payload`.

For cursor-paginated logs, `meta.total` should be omitted. Computing a full count is expensive on large log tables and does not help cursor iteration. `meta.limit`, `meta.has_more`, and `meta.next_cursor` are the authoritative pagination fields.

### `GET /v1/logs/{id}`

Returns one log row for the authenticated workspace.

The handler must query by both log id and authenticated workspace id. If the id exists in another workspace, return `404 NOT_FOUND`.

Detail responses may include redacted `request_payload` and `response_payload`.

Response:

```json
{
  "data": {
    "id": 110966,
    "workspace_id": "ae267ee2-298d-4fa8-b6a0-c386000b17af",
    "ts": "2026-06-17T20:16:34.476752Z",
    "level": "error",
    "status": "error",
    "category": "oauth",
    "action": "account.connect.callback_failed",
    "source": "oauth",
    "message": "Failed to persist connected account.",
    "profile_id": "4ec8ee48-9119-40ad-b4ca-99992e965316",
    "platform": "instagram",
    "error_code": "account_save_failed",
    "metadata": {
      "external_user_id": "0669764b-8862-4094-be5f-db7bb70361ad"
    },
    "request_payload": {
      "headers": {
        "Authorization": "[REDACTED]"
      }
    },
    "response_payload": {
      "error": "Provider returned validation_error"
    }
  },
  "request_id": "req_response_123"
}
```

### Error response envelope

Logs endpoints must use the existing API error envelope:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "normalized_code": "validation_error",
    "message": "Invalid cursor"
  },
  "request_id": "req_response_123"
}
```

For rate limits, use the existing rate-limit envelope shape and `Retry-After` header:

```json
{
  "error": {
    "code": "RATE_LIMITED",
    "normalized_code": "rate_limited",
    "message": "Too many concurrent log streams"
  },
  "request_id": "req_response_123"
}
```

### `GET /v1/logs/stream`

Adds an API-key-authenticated real-time stream for server-to-server ingestion.

Use Server-Sent Events for V1 because:

- Standard HTTP clients can set the `Authorization` header.
- It is easy to test with `curl`.
- It is simpler than requiring browser-style WebSocket token query params.
- It works well for append-only event streams.

Authentication:

```http
Authorization: Bearer <unipost_api_key>
Accept: text/event-stream
```

Supported query parameters:

| Parameter | Type | Description |
| --- | --- | --- |
| `category` | string | Optional exact category filter. |
| `status` | string | Optional exact status filter. |
| `level` | string | Optional exact level filter. |
| `platform` | string | Optional exact platform filter. |
| `profile_id` | string | Optional profile filter. |
| `social_account_id` | string | Optional account filter. |
| `post_id` | string | Optional post filter. |
| `request_id` | string | Optional request filter. |
| `error_code` | string | Optional error-code filter. |
| `after_id` | integer | Optional log id for replay before entering live mode. Replays retained rows with `id > after_id` in ascending id order. |

The stream endpoint must not accept the REST list cursor. REST list cursors are `ts,id` page positions for reverse-chronological backfill. Stream replay is append-only and should use monotonic log ids.

SSE reconnection behavior:

- If the request includes `after_id`, use that value for replay.
- If `after_id` is absent and the request includes `Last-Event-ID`, parse `Last-Event-ID` as the last seen log id and replay rows with `id > Last-Event-ID`.
- If both are present, `after_id` wins because it is the explicit query parameter.
- If neither is present, enter live mode without historical replay.
- Invalid `after_id` or invalid `Last-Event-ID` returns `422 VALIDATION_ERROR`.

Event format:

```text
event: log.created
id: 110966
data: {"id":110966,"workspace_id":"ae267ee2-298d-4fa8-b6a0-c386000b17af","ts":"2026-06-17T20:16:34.476752Z","level":"error","status":"error","category":"oauth","action":"account.connect.callback_failed","source":"oauth","message":"Failed to persist connected account.","platform":"instagram","error_code":"account_save_failed"}
```

Connection behavior:

- Send keepalive comments every 25 seconds:

```text
: keepalive
```

- Return `401` for missing or invalid auth.
- Return `403` only for authenticated users without workspace access.
- Return `422 VALIDATION_ERROR` for invalid filters.
- Return `429 RATE_LIMITED` if a configured stream concurrency policy is exceeded.
- Close gracefully when the request context is canceled.

Replay-to-live correctness:

The implementation must avoid dropping log events created between replay and live subscription. Required sequence:

1. Register the SSE connection as a subscriber to the existing process-local `logsHub` or a generalized hub abstraction.
2. Start buffering matching live hub messages for the authenticated workspace.
3. Query retained replay rows from the database using `id > after_id` or `id > Last-Event-ID`, ordered by `id ASC`.
4. Write replay rows to the SSE response.
5. Drop buffered rows whose id is less than or equal to the highest replayed id or has already been sent.
6. Flush remaining buffered rows in ascending id order.
7. Enter live mode and write future hub messages directly.

Architecture note:

`api/internal/ws/pgnotify.go` already owns PostgreSQL `LISTEN logs_events` and forwards notifications into `logsHub`. V1 should extend the existing hub with a channel subscription API, or extract a shared broker used by both WebSocket and SSE handlers. The SSE handler should not acquire its own PostgreSQL listener connection per client.

Concurrency note:

After reusing the shared hub, a low concurrency cap is no longer needed to protect the database connection pool. V1 may still add a soft abuse cap, but an in-process counter is only per API replica. A strict cross-replica cap requires Redis or another shared store.

## Webhook requirements

V1 should not add a `log.created` webhook by default.

Reason:

Webhook delivery itself creates integration logs. A naive `log.created` webhook can recursively generate more `webhook.delivery.*` logs, which create more `log.created` deliveries. This is solvable, but it needs careful delivery-loop suppression and product copy.

Recommended V1 position:

- Use webhooks for post and account lifecycle events.
- Use `GET /v1/logs/stream` for real-time log ingestion.
- Document that `GET /v1/logs/stream` is the supported real-time logs API.

V2 option:

Add a `log.created` webhook only if all of these are true:

1. `log.created` deliveries do not emit another `log.created` delivery.
2. Webhook-delivery logs for `delivery_event=log.created` are excluded from log-created webhook fanout.
3. The docs explicitly warn customers about volume.
4. The event can be filtered by category, status, level, and platform at subscription time.

## Dashboard requirements

The dashboard should make the scope obvious.

Choose one of these product options before implementation:

### Option A: Workspace Logs

Keep the current workspace-wide behavior and update UI copy:

- Page title: `Workspace Logs`
- Empty state copy: "Logs for API, publishing, OAuth, webhook, and worker activity in this workspace."
- Keep account/profile filters.
- Keep route for compatibility, but treat `/projects/{id}/logs` as the workspace logs view inside the current profile shell.

### Option B: Profile Logs by default

Default-filter the page by the route profile id:

- Add `profile_id: profileId` to list requests unless the user explicitly clears the filter.
- Add an active filter chip: `profile: {profileName}`.
- Add a visible control to switch to `All workspace logs`.
- Apply the same profile filter to live-tail matching.

Recommended V1 choice: Option A.

Reason: current logs are workspace-operational, and many relevant API logs do not always have a profile id. A strict default profile filter can hide setup errors that customers need during API onboarding.

## Documentation requirements

Add docs pages:

```text
dashboard/src/app/docs/api/logs/page.tsx
dashboard/src/app/docs/api/logs/list/page.tsx
dashboard/src/app/docs/api/logs/get/page.tsx
dashboard/src/app/docs/api/logs/stream/page.tsx
```

Update API navigation to include `Logs` under the developer operations section.

Docs must explain:

- Logs are scoped to the authenticated workspace.
- Normal logs API never accepts `workspace_id`.
- Admin logs are separate and not available to customers.
- List endpoint omits raw payloads.
- Detail endpoint includes redacted payloads.
- Redaction currently matches key fragments by lowercased substring. Public docs may state these fragments only if tests cover each one: `token`, `secret`, `authorization`, `cookie`, `password`, `refresh_token`, `access_token`, and `client_secret`.
- Payloads are truncated.
- Retention depends on plan and follows the table in this PRD.
- Use `request_id` to correlate API responses with logs.
- Use SSE stream for real-time ingestion.

Add examples:

```bash
curl "https://api.unipost.dev/v1/logs?status=error&limit=50" \
  -H "Authorization: Bearer $UNIPOST_API_KEY"
```

```bash
curl "https://api.unipost.dev/v1/logs/110966" \
  -H "Authorization: Bearer $UNIPOST_API_KEY"
```

```bash
curl -N "https://api.unipost.dev/v1/logs/stream?status=error" \
  -H "Authorization: Bearer $UNIPOST_API_KEY" \
  -H "Accept: text/event-stream"
```

## SDK requirements

Add first-class logs clients to supported SDKs.

Required methods:

```text
logs.list(params)
logs.get(id)
logs.stream(params)
```

Language-specific guidance:

- JavaScript: expose `client.logs.list`, `client.logs.get`, and an async iterator for `client.logs.stream`.
- Python: expose `client.logs.list`, `client.logs.get`, and an iterator for `client.logs.stream`.
- Go: expose `client.Logs.List`, `client.Logs.Get`, and `client.Logs.Stream(ctx, params)`.
- Java: expose `client.logs().list`, `client.logs().get`, and a stream callback helper if it fits the existing SDK style.

Validation scripts should cover list and get. Stream validation should be a manual or opt-in test unless a safe fixture can create a log during the test run.

## Security and privacy requirements

1. Normal endpoints must never accept `workspace_id`.
2. Detail reads must return `404` for logs outside the authenticated workspace.
3. Real-time streams must filter by authenticated workspace before sending any event.
4. API-key stream auth must use the `Authorization` header, not query-string secrets.
5. Dashboard WebSocket query-token behavior may remain for browser Clerk sessions.
6. Redaction must happen before persistence.
7. Sensitive header and payload keys must remain redacted in list, detail, WebSocket, and SSE responses.
8. Every sensitive key fragment listed in public docs must have a redaction unit test.
9. Cursor values must not expose raw SQL or leak cross-workspace state.
10. `Last-Event-ID` and `after_id` replay must always be workspace-filtered before any row is emitted.
11. Stream responses must not include admin-only enrichment such as owner email or workspace name.
12. If stream concurrency limits are product requirements, define whether they are per process or globally enforced through Redis/shared state.

## Implementation plan

### Phase 1: Lock down current isolation behavior

Files:

- Create: `api/internal/handler/logs_test.go`
- Modify or reuse test helpers from existing handler tests.
- No dashboard changes in this phase.

Steps:

1. Add tests for `GET /v1/logs` returning only rows for `auth.GetWorkspaceID`.
2. Add tests for `GET /v1/logs/{id}` returning `404` when the id exists in another workspace.
3. Add tests proving list responses omit request and response payload fields.
4. Add tests proving detail responses include redacted payload fields.
5. Add table-driven redaction tests for each public-doc key fragment: `token`, `secret`, `authorization`, `cookie`, `password`, `refresh_token`, `access_token`, and `client_secret`.
6. Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test -count=1 ./internal/handler ./internal/integrationlogs
```

Expected:

```text
ok   github.com/xiaoboyu/unipost-api/internal/handler
ok   github.com/xiaoboyu/unipost-api/internal/integrationlogs
```

### Phase 2: Add cursor pagination to REST logs

Files:

- Modify: `api/internal/db/queries/integration_logs.sql`
- Modify generated sqlc output after running sqlc: `api/internal/db/integration_logs.sql.go`.
- Modify: `api/internal/handler/logs.go`
- Modify: `api/internal/handler/logs_test.go`
- Modify: `dashboard/src/lib/api.ts` only if dashboard needs to consume cursor fields. The shared `ApiResponse` type already includes `has_more` and `next_cursor`.

API behavior:

- Keep default order `ts DESC, id DESC`.
- Cursor encodes last seen `ts` and `id`.
- Next page condition:

```sql
AND (
  sqlc.arg('cursor_ts')::timestamptz IS NULL
  OR ts < sqlc.arg('cursor_ts')::timestamptz
  OR (ts = sqlc.arg('cursor_ts')::timestamptz AND id < sqlc.arg('cursor_id')::bigint)
)
```

- Fetch `limit + 1`.
- Return `has_more=true` and `next_cursor` when an extra row exists.
- Use `writeSuccessWithCursor`, not `writeSuccessWithListMeta`, so cursor responses omit `meta.total`.

Validation:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test -count=1 ./internal/handler
```

### Phase 3: Add API-key-accessible SSE stream

Files:

- Create: `api/internal/handler/logs_stream.go`
- Create: `api/internal/handler/logs_stream_test.go`
- Modify: `api/internal/ws/hub.go`
- Modify: `api/cmd/api/main.go`
- Modify: `api/internal/handler/logs.go` if shared filter parsing is extracted.

Route:

```go
r.Get("/v1/logs/stream", logsStreamHandler.Stream)
```

Mount `/v1/logs/stream` before `/v1/logs/{id}` to avoid route ambiguity.

Handler requirements:

- Runs inside the same `DualAuthMiddleware` workspace group.
- Reads `workspace_id` from context.
- Validates filters once at connection start.
- Reuses the existing shared `logsHub` populated by `ws.PGListener`; it must not create a PostgreSQL `LISTEN` connection per client.
- Adds or extracts a hub subscription method that can return a channel of raw log envelope bytes and an unsubscribe function for SSE use.
- Sets:

```http
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no
```

- Drops notifications with non-matching `workspace_id`.
- Drops notifications that do not match requested filters.
- Supports replay with `after_id` and `Last-Event-ID` as defined above.
- Subscribes and buffers before querying replay rows to prevent replay-to-live gaps.
- Writes `event: log.created`, `id: {log.id}`, and compact JSON `data`.
- Sends keepalive comments.
- Applies a stream concurrency policy only if the product decision requires one; document whether the policy is per process or shared across replicas.

Validation:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test -count=1 ./internal/handler ./internal/ws
```

Manual smoke:

```bash
curl -N "https://dev-api.unipost.dev/v1/logs/stream?status=error" \
  -H "Authorization: Bearer $UNIPOST_API_KEY" \
  -H "Accept: text/event-stream"
```

### Phase 4: Clarify dashboard scope

Files:

- Modify: `dashboard/src/app/(dashboard)/projects/[id]/logs/page.tsx`
- Modify: `dashboard/src/lib/api.ts` if cursor metadata is surfaced.
- Add or update Playwright regression if dashboard regression suite covers logs.

Recommended V1 changes:

- Change page title from `Logs` to `Workspace Logs`.
- Keep existing filters.
- Add a small scope label near the title:

```text
Current workspace
```

- Preserve existing URL filters.
- Do not default to `profile_id` filtering unless the product chooses Option B.

Validation:

```bash
cd dashboard
npm run build
```

If Playwright browsers are installed:

```bash
cd dashboard
npm run test:regression:dashboard
```

### Phase 5: Add public docs

Files:

- Create: `dashboard/src/app/docs/api/logs/page.tsx`
- Create: `dashboard/src/app/docs/api/logs/list/page.tsx`
- Create: `dashboard/src/app/docs/api/logs/get/page.tsx`
- Create: `dashboard/src/app/docs/api/logs/stream/page.tsx`
- Modify API docs navigation components used by `dashboard/src/app/docs/api/layout.tsx` or shared docs components.
- Modify: `docs/sdk-api-coverage-matrix.md`

Docs content:

- List endpoint page.
- Get endpoint page.
- Stream endpoint page.
- Redaction and retention notes.
- Examples in cURL, JavaScript, Python, Go, and Java where SDKs support it.

Validation:

```bash
cd dashboard
npm run build
```

### Phase 6: Add SDK surface and validation

Files depend on the SDK repositories or generated source layout used by release tooling.

Expected local validation files:

- Modify: `scripts/sdk-validation/js/unipost-sdk-test.mjs`
- Modify: `scripts/sdk-validation/python/unipost_sdk_test.py`
- Modify: `scripts/sdk-validation/go/main.go`
- Modify: `scripts/sdk-validation/java/src/main/java/dev/unipost/validation/UnipostSdkTest.java`
- Modify: `docs/sdk-api-coverage-matrix.md`

Validation:

```bash
scripts/sdk-source-validation/run-suite.sh
```

If published package validation is appropriate:

```bash
scripts/sdk-published-regression/run-suite.sh
```

### Phase 7: Dev deployment and acceptance

Required by UniPost workflow after merging to `dev` and pushing `origin/dev`.

Steps:

1. Run backend tests:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
```

2. Run dashboard build:

```bash
cd dashboard
npm run build
```

3. Merge task branch into local `dev`.
4. Rerun changed-surface validation on local `dev`.
5. Push local `dev` to `origin/dev`.
6. Monitor GitHub Actions, Vercel, Railway, and triggered development deployments until complete.
7. Validate in development domains only:

```text
https://dev-api.unipost.dev
https://dev-app.unipost.dev
```

8. Acceptance checks:

- Customer dashboard Logs page shows workspace logs and no admin-only owner email fields.
- `GET /v1/logs` with a workspace API key returns only that workspace's logs.
- `GET /v1/logs/{id}` returns `404` for another workspace's log id.
- `GET /v1/logs?limit=2` returns a cursor when more rows exist.
- Following `next_cursor` returns the next page without duplicate rows.
- Cursor-paginated responses include `meta.limit`, `meta.has_more`, and `meta.next_cursor`; they do not include `meta.total`.
- `GET /v1/logs/stream` emits a new matching log for the authenticated workspace.
- `GET /v1/logs/stream?after_id={id}` replays retained rows with a greater id before live events.
- A reconnect with `Last-Event-ID: {id}` replays retained rows with a greater id before live events.
- Stream does not emit logs from another workspace.
- Docs pages are reachable and examples are copy-pasteable.

## Testing matrix

| Area | Required check |
| --- | --- |
| Workspace isolation | Handler tests for list, detail, and stream. |
| Payload behavior | List omits payloads; detail includes redacted payloads. |
| Redaction docs | Every public redaction key fragment has a unit test. |
| Pagination | Cursor order, duplicate prevention, invalid cursor errors. |
| Filters | Category, status, platform, post id, request id, error code. |
| SSE | Auth required, workspace filtering, keepalive, disconnect handling, `after_id`, `Last-Event-ID`, and replay-to-live no-gap behavior. |
| Dashboard | Build and visual smoke on dev app. |
| Docs | Dashboard build and route reachability. |
| SDK | Source validation suite for list/get; stream opt-in smoke. |

## Rollout plan

1. Ship tests and docs first if product wants low-risk visibility before stream work.
2. Ship cursor pagination and REST docs.
3. Ship SSE stream in development.
4. Validate with one internal workspace API key.
5. Enable or release according to the feature flag decision.
6. Update SDKs and public docs.
7. Announce customer-facing usage pattern:

```text
Use REST for backfill and SSE for real-time logs. Use webhooks for post/account lifecycle events.
```

## Open decisions for review

1. Should Dashboard Logs be branded as `Workspace Logs` in V1?
2. Should `/projects/{id}/logs` default-filter by `profile_id`, or remain workspace-wide?
3. Should `logs.public_api_v1` be created before implementation?
4. Should normal customer log responses keep returning `workspace_id`, or omit it because auth already scopes the response? Recommendation: keep it for client-side attribution in multi-workspace tooling.
5. Should SSE have a product-level concurrency cap after reusing the shared hub? If yes, should it be per process or globally enforced with Redis/shared state?
6. Should `GET /v1/logs/stream` be included in SDKs immediately, or documented as REST/SSE first?
7. Should a `log.created` webhook stay deferred until after SSE adoption?
8. Should stream access be available on every plan within each plan's retention window, or restricted to paid plans?

## Acceptance criteria

The work is complete when:

1. Normal users and API keys can only access logs for their authenticated workspace.
2. Global admin logs remain available only through super-admin routes.
3. Customers can backfill all retained logs through documented cursor pagination.
4. Customers can receive near real-time workspace logs through API-key-authenticated SSE.
5. Docs explain REST, detail, stream, redaction, retention, and request-id correlation.
6. SSE replay honors `after_id` and `Last-Event-ID` without dropping events during replay-to-live transition.
7. SDK or direct API examples are available for ordinary setup.
8. Automated tests cover workspace isolation, pagination, redaction key fragments, and stream replay behavior.
9. Development deployment has been monitored and self-accepted against this PRD.
