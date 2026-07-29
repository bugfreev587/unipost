# UniPost internal feature flags

UniPost no longer uses Unleash. Rollout flags are stored in the UniPost database and evaluated by the backend so Dashboard, API-key traffic, managed users, background workers, and public marketing surfaces share one authority.

## Admin control

- Page: `/admin/feature-flags`
- API: `GET /v1/admin/feature-flags` and `PATCH /v1/admin/feature-flags/{key}`
- Access: Super Admin only
- Scope: one global value per deployed environment; the data model has no test/production variant dimension
- Audit: actual state changes append an immutable row to `feature_flag_changes`

The Admin UI uses these semantics:

- **ON:** the feature is available to regular users.
- **OFF:** the feature is unavailable to regular users. Workspaces owned by a Super Admin remain enabled for acceptance testing.

The frontend may hide customer UI using `GET /v1/me/features` or `GET /v1/public/features`, but sensitive behavior must remain backend-enforced.

## Registered flags

| Key | Owner area | Default | OFF behavior | Rollback action | Third-party dependency |
|---|---|---:|---|---|---|
| `x_dms_v1` | X Inbox | OFF | Regular workspaces cannot list, sync, or send `x_dm`; DM-only missing scopes do not require reconnect. X comments and publishing remain available. | Turn OFF in Admin; the backend stops DM access and removes any stale DM delivery intent. | X OAuth 2.0 supports direct DM reads/writes, but private Activity subscription creation is not production-ready. |
| `x_credits_billing_v1` | Billing | OFF | Managed X calls do not count against or block on the customer monthly X Credits balance. The independent 20 X publishes/account/day limit and internal inbound cost-safety cap remain active. | Turn OFF in Admin; customer monthly accounting and UI stop immediately while safety accounting continues. | X pay-per-use pricing and UniPost cost reconciliation. |
| `observability_reads_v2` | API / Admin Observability | OFF and activation-locked. | Logs and API Metrics use their compatible legacy read projections. Admin Errors remains on its contained metadata-list/bounded-detail path. Canonical and legacy writers remain active and are never gated by this flag. | It cannot be enabled while locked. After a future SDK-compatible release, turn OFF on the Admin feature-flags page: HTTP reads return to legacy on their next request, and every open or new Logs WebSocket uses the legacy projection within 1.5 seconds without reconnecting. | None. |

## Reconnect behavior

The OAuth 2.0 connection request continues to include `tweet.read`, `tweet.write`, `users.read`, `offline.access`, `media.write`, `dm.read`, and `dm.write`.

- Turning `x_dms_v1` OFF does not revoke already-granted scopes.
- Turning it ON does not require another reconnect for accounts that already granted the DM scopes.
- Accounts missing `dm.read` or `dm.write` reconnect once after the flag is ON.
- X comments require their existing read/write scopes and are not gated by `x_dms_v1`.

## Production isolation

All seeded flags are OFF. A migration, deploy, or process restart must not turn them ON. The only supported global mutation is the Super Admin API/UI. Customer feature enforcement uses the workspace evaluator; environment-global infrastructure switches such as `observability_reads_v2` use the public/global evaluator so the Super Admin workspace bypass cannot change their meaning. No frontend value or environment variable is authoritative.

`observability_reads_v2` controls Logs and API Metrics reads only. It never gates telemetry writes, Admin Errors storage or result semantics, retention, backfill, migration, cleanup, publishing, account connection, billing, quota, or authentication behavior. Its registration remains `activation_ready=false`, so the audited Admin API returns `409 FLAG_NOT_READY` for attempts to enable it and does not mutate stored state.

HTTP Logs and API Metrics evaluate the global selector on each request. Live Logs use one fail-closed process-level mode cache with a one-second refresh interval and a 500ms evaluation timeout, giving a worst-case 1.5-second fallback SLA. Each broadcast envelope reads that atomic mode once for all connected clients; timeout, cancellation, or selector error atomically sets the cached mode to legacy. Event volume and WebSocket connection count do not increase feature-flag queries. While activation is locked, each refresh returns legacy before querying the flag store.

Before `ActivationReady` may become true, complete a historical backfill or sufficient warm-up for every supported 7-90-day Metrics window. Prove completion coverage for all target hours and legacy parity for counts, status, trends, and percentile behavior where applicable. Before physical raw retention, a sealed workspace-hour or equivalently strong completion contract is required and prevents destructive recompute and data loss.

Activation may be unlocked only after all four public SDKs (JavaScript/TypeScript, Python, Go, and Java) have released and validated support for string opaque IDs from Logs list responses and string IDs passed to Logs get. Current legacy SDK validation remains numeric-only and therefore deliberately documents the blocker; it must not be rewritten to claim compatibility before those SDK releases exist. The public SSE stream remains a separate numeric integration-log contract during this migration.

Before activation, a dedicated activation PR must wire a bounded `OnPersistedBatch` canonical WebSocket publisher and prove that ON delivers exactly one canonical request event, OFF and SSE preserve numeric no-gap delivery, and selector failure falls back to legacy. Only after that live-stream safety gate and the four-SDK matrix pass may exact-SHA Preview and deployed dev acceptance unlock `ActivationReady`. There is no third-party runtime dependency for this switch.
