# SDK API Coverage Matrix

This matrix tracks UniPost's current public API surface against the local JS, Python, Go, and Java SDKs, plus whether the `scripts/sdk-validation` live tests cover each area.

Legend:

- `Yes` = implemented in SDK or covered by validation
- `Conditional` = live validation runs it only when a safe fixture exists, or accepts a known gated/negative-path outcome
- `No` = not currently implemented / not currently validated
- `Helper` = SDK convenience method, not a 1:1 backend route

## Route coverage

| Public API surface | JS SDK | Python SDK | Go SDK | Java SDK | Live validation |
| --- | --- | --- | --- | --- | --- |
| `GET /v1/platforms/capabilities` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/plans` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/workspace` | Yes | Yes | Yes | Yes | Yes |
| `PATCH /v1/workspace` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/profiles` | Yes | Yes | Yes | Yes | Yes |
| `POST /v1/profiles` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/profiles/{id}` | Yes | Yes | Yes | Yes | Yes |
| `PATCH /v1/profiles/{id}` | Yes | Yes | Yes | Yes | Yes |
| `DELETE /v1/profiles/{id}` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/accounts` | Yes | Yes | Yes | Yes | Yes |
| `POST /v1/accounts/connect` | Yes | Yes | Yes | Yes | Yes, negative path |
| `DELETE /v1/accounts/{id}` | Yes | Yes | Yes | Yes | No direct live delete |
| `GET /v1/accounts/{id}/capabilities` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/accounts/{id}/health` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/accounts/{id}/metrics` | Yes | Yes | Yes | Yes | No direct live fixture |
| `GET /v1/accounts/{id}/tiktok/creator-info` | Yes | Yes | Yes | Yes | Conditional |
| `GET /v1/accounts/{id}/facebook/page-insights` | Yes | Yes | Yes | Yes | Conditional |
| `POST /v1/media` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/media/{id}` | Yes | Yes | Yes | Yes | Yes |
| `DELETE /v1/media/{id}` | Yes | Yes | Yes | Yes | Cleanup path |
| `POST /v1/connect/sessions` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/connect/sessions/{id}` | Yes | Yes | Yes | Yes | Yes |
| `POST /v1/platform-credentials` | Yes | Yes | Yes | Yes | Conditional, plan-gated |
| `GET /v1/platform-credentials` | Yes | Yes | Yes | Yes | Conditional, plan-gated |
| `DELETE /v1/platform-credentials/{platform}` | Yes | Yes | Yes | Yes | Conditional, plan-gated |
| `GET /v1/posts` | Yes | Yes | Yes | Yes | Yes |
| `POST /v1/posts` | Yes | Yes | Yes | Yes | Yes |
| `POST /v1/posts/validate` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/posts/{id}` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/posts/{id}/analytics` | Yes | Yes | Yes | Yes | Yes |
| `POST /v1/posts/{id}/archive` | Yes | Yes | Yes | Yes | Yes |
| `POST /v1/posts/{id}/restore` | Yes | Yes | Yes | Yes | Yes |
| `DELETE /v1/posts/{id}` | Yes | Yes | Yes | Yes | Cleanup path |
| `POST /v1/posts/{id}/results/{resultID}/retry` | Yes | Yes | Yes | Yes | Conditional |
| `GET /v1/posts/{id}/queue` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/post-delivery-jobs` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/post-delivery-jobs/summary` | Yes | Yes | Yes | Yes | Yes |
| `POST /v1/post-delivery-jobs/{jobID}/retry` | Yes | Yes | Yes | Yes | Conditional |
| `POST /v1/post-delivery-jobs/{jobID}/cancel` | Yes | Yes | Yes | Yes | Conditional |
| `POST /v1/posts/{id}/publish` | Yes | Yes | Yes | Yes | Conditional, opt-in |
| `PATCH /v1/posts/{id}` | Yes | Yes | Yes | Yes | Yes |
| `POST /v1/posts/{id}/cancel` | Yes | Yes | Yes | Yes | Yes |
| `POST /v1/posts/bulk` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/users` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/users/{external_user_id}` | Yes | Yes | Yes | Yes | Conditional |
| `POST /v1/posts/{id}/preview-link` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/analytics/summary` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/analytics/trend` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/analytics/by-platform` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/analytics/rollup` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/analytics/posts` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/analytics/posts/export` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/analytics/platforms` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/analytics/platforms/{platform}` | Yes | Yes | Yes | Yes | Yes |
| `POST /v1/analytics/refresh` | Yes | Yes | Yes | Yes | Yes, negative path |
| `GET /v1/logs` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/logs/{id}` | Yes | Yes | Yes | Yes | Conditional |
| `GET /v1/logs/stream` | Yes | Yes | Yes | Yes | Conditional |
| `POST /v1/webhooks` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/webhooks` | Yes | Yes | Yes | Yes | Yes |
| `GET /v1/webhooks/{id}` | Yes | Yes | Yes | Yes | Yes |
| `PATCH /v1/webhooks/{id}` | Yes | Yes | Yes | Yes | Yes |
| `DELETE /v1/webhooks/{id}` | Yes | Yes | Yes | Yes | Cleanup path |
| `POST /v1/webhooks/{id}/rotate` | Yes | Yes | Yes | Yes | Yes |
| `POST /v1/oauth/connect` | Yes | Yes | Yes | Yes | Yes, accepts current backend behavior |
| `GET /v1/usage` | Yes | Yes | Yes | Yes | Yes |

## SDK-only helpers

These are convenience methods that exist in the SDK surface but are not 1:1 public backend routes:

| Helper | JS | Python | Go | Java | Validation |
| --- | --- | --- | --- | --- | --- |
| `accounts.get(id)` | Yes | Yes | Yes | Yes | Yes |

`accounts.get(id)` is implemented client-side by listing accounts and selecting the matching item because the public API does not currently expose `GET /v1/accounts/{id}`.

## Known live-regression gaps

These routes exist in the backend but are not yet covered by the hourly published-SDK regression. Some are dashboard-only or Clerk-session routes and should be covered by a separate dashboard smoke suite rather than SDK validation.

| API surface | Current validation status | Notes |
| --- | --- | --- |
| `GET /v1/accounts/{id}/metrics` | Optional smoke | Covered for YouTube when `YOUTUBE_METRICS_ACCOUNT_ID` / `REGRESSION_YOUTUBE_METRICS_ACCOUNT_ID` is configured. Other platform fixtures remain conditional. |
| `GET /v1/accounts/{id}/youtube/analytics/summary` | Optional smoke | Covered when `YOUTUBE_ANALYTICS_ACCOUNT_ID` / `REGRESSION_YOUTUBE_ANALYTICS_ACCOUNT_ID` points at a connected YouTube fixture with `yt-analytics.readonly`. |
| `GET /v1/accounts/{id}/youtube/analytics/trend` | Optional smoke | Covered when `YOUTUBE_ANALYTICS_ACCOUNT_ID` / `REGRESSION_YOUTUBE_ANALYTICS_ACCOUNT_ID` points at a connected YouTube fixture with `yt-analytics.readonly`. |
| `GET /v1/accounts/{id}/youtube/analytics/videos` | Optional smoke | Covered when `YOUTUBE_ANALYTICS_ACCOUNT_ID` / `REGRESSION_YOUTUBE_ANALYTICS_ACCOUNT_ID` points at a connected YouTube fixture with `yt-analytics.readonly`. |
| `GET /v1/accounts/{id}/tiktok/profile` | No | Should be conditional on a connected TikTok fixture. |
| `GET /v1/accounts/{id}/tiktok/videos` | No | Should be conditional on a connected TikTok fixture. |
| `GET /v1/accounts/{id}/facebook/page-analytics` | No | Dashboard aggregate endpoint; should be conditional on a connected Facebook Page fixture and admin allowlist access. |
| `GET /v1/accounts/{id}/pinterest/boards` | No | Should be conditional on a connected Pinterest fixture. |
| `POST /v1/accounts/{id}/pinterest/boards` | No | Requires safe sandbox/fixture rules before live regression. |
| `GET /v1/accounts/{id}/facebook/webhook-status` | No | Dashboard operational endpoint. |
| `POST /v1/accounts/{id}/facebook/resubscribe-webhooks` | No | Mutating repair path; should not run hourly without an explicit fixture. |
| `GET /v1/posts/summaries` | Direct smoke | Covered by `scripts/smoke-test.sh`. |
| `POST /v1/post-delivery-jobs/{jobID}/retry-now` | No | Conditional mutating path. |
| `POST /v1/post-delivery-jobs/{jobID}/dismiss` | No | Conditional mutating path. |
| `GET /v1/limits` | Direct smoke | Covered by `scripts/smoke-test.sh`. |
| `GET /v1/members` | Direct smoke | Read path covered by `scripts/smoke-test.sh`; mutating invite/role paths are not covered. |
| `GET /v1/audit-log` | Direct smoke | Covered by `scripts/smoke-test.sh`. |
| `GET /v1/inbox/*` and `POST /v1/inbox/*` | Scoped smoke + deployed acceptance | `unread-count` is covered with explicit owner/admin `inbox_scope=workspace` and may pass as 200 or 402 plan-gated. `scripts/inbox-scope-acceptance.mjs` covers managed-user A/B list isolation, cross-scope 404s for get/read/reply/thread-state, missing-scope rejection, owner/admin aggregate reads, and WebSocket fan-out. |
| `GET /v1/me/*`, notifications, tutorials, activation | No | Clerk-session-only; not suitable for API-key SDK regression. |
| `GET /v1/admin/*` | No | Admin-session-only; separate admin smoke recommended. |

## Notes

- `facebook/page-insights`, `platform-credentials`, `retryResult`, `deliveryJobs.retry`, `deliveryJobs.cancel`, and live publish are environment-sensitive. Validation covers them conditionally or via safe negative-path checks so the scripts remain runnable in normal workspaces.
- Inbox deployed acceptance requires UniPost-owned, non-customer fixtures in one workspace: two managed-user IDs, one existing Inbox item per managed user, a server-side creator-bound API key whose creator is still an owner/admin, and an explicit target API URL. Fixture B must already be read and its thread-state mutation repeats its current value. The cross-scope reply sends an empty JSON object (`{}`), so any isolation regression reaches `400` validation before any provider call.
- WebSocket isolation acceptance additionally requires `INBOX_ACCEPT_EVENT_DATABASE_URL` and the explicit opt-in `INBOX_ACCEPT_ALLOW_PG_NOTIFY=1`. The script requires `psql`, sends only ephemeral `pg_notify` events on `inbox_events`, performs no database table reads or writes, never calls provider APIs, and must not be run with customer accounts. A retried A readiness probe proves both subscriptions are registered; then one psql transaction sends the B probe followed by an A barrier, making the negative managed-user assertion causal rather than time-window based.
- HTTP, WebSocket upgrade/event/readiness, and psql operations have finite defaults. Operators may tighten them with `INBOX_ACCEPT_HTTP_TIMEOUT_MS`, `INBOX_ACCEPT_WS_UPGRADE_TIMEOUT_MS`, `INBOX_ACCEPT_WS_EVENT_TIMEOUT_MS`, `INBOX_ACCEPT_WS_READY_TIMEOUT_MS`, `INBOX_ACCEPT_PSQL_TIMEOUT_MS`, and `INBOX_ACCEPT_PSQL_KILL_GRACE_MS`; invalid or out-of-range values fail closed.
