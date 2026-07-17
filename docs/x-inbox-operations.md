# X Inbox operations runbook

This runbook covers X reply and DM ingestion, delivery resources, inbound-credit controls, manual backfill, and upstream cleanup in the development environment. Do not use it to promote changes to staging or production.

## Ownership and escalation

Fill these placeholders in the deployment inventory before enabling the integration:

| Responsibility | Development owner |
|---|---|
| X developer application | `<DEV_X_APP_OWNER>` |
| X API billing and spend limit | `<X_BILLING_OWNER>` |
| UniPost API and delivery worker | `<UNIPOST_BACKEND_OWNER>` |
| Customer support escalation | `<SUPPORT_ESCALATION_CHANNEL>` |
| Security incident escalation | `<SECURITY_ESCALATION_CHANNEL>` |

The X billing owner must confirm the active X subscription, account-level spending limit, and available balance before the API owner enables managed delivery or a manual backfill. Record the check time and approver in the internal incident/change record. Do not copy credentials, customer content, or upstream response bodies into that record.

Escalate immediately when any of these conditions persists through two reconciliation intervals:

- filtered-stream or Activity subscription capacity is at or above 95%;
- an accepted write remains in `needs_reconciliation`;
- provisional usage, notification claims, or cleanup leases are stale;
- a source remains paused after its cap/allowance/plan condition is resolved;
- the webhook CRC/signature path, filtered stream, or Activity delivery is unavailable;
- X reports a billing, entitlement, or developer-application restriction.

Capacity at 70% is an owner notification; 85% requires a capacity plan and change owner; 95% blocks adding managed delivery resources until the X billing/developer-app owner resolves capacity.

## Required development configuration

The development API/worker deployment requires the following variables. Values belong in the deployment secret store, never in source control, tickets, logs, screenshots, or this document.

| Variable | Purpose |
|---|---|
| `DATABASE_URL` | Development Postgres connection |
| `ENCRYPTION_KEY` | Encryption for stored access tokens and workspace-app credentials |
| `API_BASE_URL` | OAuth callback origin for the development API |
| `APP_BASE_URL` | Development application origin used by notifications |
| `TWITTER_CLIENT_ID` | Managed X OAuth application client ID |
| `TWITTER_CLIENT_SECRET` | Managed X OAuth application client secret |
| `TWITTER_BEARER_TOKEN` | Managed app bearer for filtered-stream/resource management |
| `TWITTER_CONSUMER_SECRET` | Managed X Activity webhook signature secret |
| `X_INBOX_WEBHOOK_ROUTE_SECRET` | Stable, independent webhook route-key secret |
| `X_INBOX_WEBHOOK_URL` | Absolute HTTPS base URL for the development X webhook |
| `X_INBOX_BACKFILL_SAFE_CREDITS` | Safety reserve retained during estimated backfill admission |
| `X_INBOX_MANAGED_FILTERED_STREAM_RULE_CAPACITY` | Managed developer-app rule capacity used for 70/85/95 alerts |
| `X_INBOX_MANAGED_ACTIVITY_SUBSCRIPTION_CAPACITY` | Managed developer-app Activity capacity used for 70/85/95 alerts |
| `X_INBOX_WORKSPACE_APP_CAPACITIES_JSON` | Per-workspace-app capacities keyed by the opaque `workspace_<16 hex>` scope emitted by reconciliation |

Workspace X applications additionally require their own OAuth client, consumer secret, and app bearer through the encrypted workspace-credential flow. They must not inherit managed-app credentials. Capacity is never aggregated across X applications: managed resources use the managed limits, while each workspace application and resource type has a separate opaque scope and configured limit. The JSON capacity value has the shape `{"workspace_<opaque>":{"filtered_stream_rules":25,"activity_subscriptions":10}}`; never use a raw X client ID as its key. Missing capacity emits `app_capacity_input_missing` and blocks the promotion gate.

Before deployment:

1. Confirm the callback and webhook URLs are development URLs only.
2. Confirm the X application has the OAuth scopes required for post reads/writes, offline access, and DMs.
3. Confirm the X API subscription, spend limit, and resource capacities with `<X_BILLING_OWNER>`.
4. Run migrations through migration 116 and verify the delivery, receipt, notification, durable-operation, exposure-reservation, cleanup-intent tables, current-state partial indexes, and completed-day evidence indexes exist.
5. Redeploy the API, then verify one `x_inbox_operations_snapshot` event arrives without customer identifiers or content.

## Safe observability contract

The reconciliation worker emits aggregate JSON events once per minute. Current nonterminal state is separate from promotion evidence. Promotion evidence always covers the previous completed UTC day, from `evidence_day_start` inclusive to `evidence_day_end` exclusive; it is never a rolling 24-hour window.

- `x_inbox_operations_snapshot`: provisional/stale/reversed usage; durable outbound, confirmation, and exposure states; cap suppression; notifications; pause/restore; webhook and paid-backfill latency; deduplication; outbound success; aggregate customer demand; and cleanup state;
- `x_inbox_capacity_metric` / `x_inbox_capacity_alert`: opaque application scope, resource type, used/capacity, and the highest crossed 70/85/95 threshold—never a cross-application total;
- `x_inbox_usage_metric`: controlled operation key, catalog version, settlement state, event count, and weighted units;
- `x_inbox_cost_variance`: externally supplied provider cost compared with expected cost derived from finalized catalog usage;
- `x_inbox_reconciliation_alert`: aggregate alert kind and count;
- `x_inbox_reconciliation_failed`: a fixed `error_class` without the underlying query/provider error.

Logs and alerts must never contain DM/comment bodies, user handles, account/workspace IDs, OAuth codes, confirmation tokens, access/refresh/bearer tokens, consumer secrets, webhook signatures, request headers, or raw X response/error bodies. Use database IDs only in an access-controlled manual investigation; do not paste query results into shared logs.

Recommended alerts:

| Signal | Alert condition | Initial action |
|---|---|---|
| `stale_provisional_usage_events` | greater than zero | Inspect durable outbound/backfill recovery before changing balances |
| `suppressed_daily_cap_events` / `suppressed_allowance_events` | unexpected increase | Confirm cap, allowance, plan, and notification delivery |
| notification claims minus enqueued | positive for two intervals | Inspect notification outbox worker and retry lease |
| pause or restore-pending age | above 10 minutes | Run source pause/restore procedure |
| `stale_delivery_resources` | greater than zero | Reconcile stream rule and Activity subscription state |
| cleanup overdue/stale lease | greater than zero | Run disconnect/deletion cleanup procedure |
| outbound outcome-unknown / needs-reconciliation | greater than zero | Do not resend; reconcile the durable write, including BYO writes with no usage event |
| confirmation or exposure stale/reconciliation state | greater than zero | Inspect lease/recovery state before starting another paid read |
| `daily_cost_input_missing` | present | Import and validate the daily X billing export; promotion remains blocked |

## Capacity and delivery-resource reconciliation

1. Check the latest aggregate snapshot. Do not add counts across developer applications unless their configured capacities are also combined on the same basis.
2. In the X developer console, confirm the actual rule/subscription counts and current plan limits with the application owner.
3. Compare X state with `x_inbox_delivery_resources`: filtered-stream rules cover X replies/mentions and Activity subscriptions cover DMs.
4. Treat an `active` resource with no recent `last_synced_at`, or an old `pending`/`error` resource, as stale. Let the delivery worker reconcile once before manual intervention.
5. At 85%, identify unused/disconnected resources and verify their durable cleanup intents. At 95%, stop provisioning managed resources and escalate.
6. Never delete an upstream rule or subscription based only on a label. Match the stable UniPost tag/application identity and the exact stored upstream resource ID.

Live resources and durable cleanup intents are de-duplicated by exact upstream resource ID inside each app scope. A missing workspace credential falls back to an isolated opaque account scope; it must never be merged into another workspace application's count.

## Promotion-gate monitoring and daily cost input

Review the previous completed UTC-day window before promotion. The gate requires operation/catalog usage, finalized and reversed usage, paid-backfill confirmation count and duration, paid-backfill dedup rate (`duplicates / read` from persisted completed results), webhook delivery latency, outbound completion rate, distinct-workspace demand, and every durable uncertain/stale state. Zero traffic is reported as zero, not silently treated as a successful demand or latency sample.

Finalized/reversed totals and every operation/catalog row use the same settlement cohort: `x_usage_events.updated_at >= evidence_day_start AND updated_at < evidence_day_end`, restricted to the terminal `finalized` and `reversed` statuses. Do not compare a created-at breakdown with settled totals. Current provisional, outbound, confirmation, exposure, notification, pause, delivery, and cleanup health remain point-in-time signals backed by partial current-state indexes.

X console/export cost is external; UniPost does not claim a live cost comparison when it is absent. The injected daily-cost boundary must supply both provider cost in micros and expected cost in micros computed from finalized usage with the approved catalog cost mapping. Until an authorized adapter supplies that UTC-day record, `daily_cost_input_missing` is expected and promotion is blocked.

Daily cost procedure:

1. `<X_BILLING_OWNER>` exports the completed UTC day's X provider charges and records the export time in the restricted change record.
2. `<UNIPOST_BACKEND_OWNER>` computes expected cost from finalized operation/catalog usage using the approved internal cost mapping; provisional or reversed units are excluded.
3. Feed both micro-unit amounts through the daily-cost input implementation. Never place export rows, account IDs, or credentials in application logs or environment variables.
4. Verify `x_inbox_cost_variance` reports provider, expected, signed variance, and basis points. Investigate any non-zero unexplained variance before promotion.
5. If the export is late, malformed, or incomplete, retain the missing-data alert and stop the promotion rather than substituting an estimate.

## Inbound cap, allowance, and notifications

For every accepted or suppressed upstream event, admission is recorded without retaining the event body in the receipt. `suppressed_daily_cap` pauses paid sources for the UTC day; `suppressed_monthly_allowance` pauses until allowance is available. Plan ineligibility uses `paused_plan`.

At 80% and 100%:

1. Verify exactly one `x_inbound_cap_notifications` claim exists per workspace, UTC date, and threshold.
2. Confirm the claim transitions to `enqueued`; a `processing` row with an expired lease is stale and must be retried by the notification worker.
3. Confirm delivery to an eligible owner/admin channel without copying its payload into logs.
4. At 100%, verify paid sources are paused and the API reports the cap condition. A duplicate upstream event must not consume credits or create another notification.

To restore a source, first resolve the causal condition: wait for UTC reset, increase an acknowledged cap through the authorized billing API, restore monthly allowance, or move to an eligible plan. Then allow the delivery reconciler to move the resource through `pending` to `active`. Confirm restore-pending age falls and `last_synced_at` advances. Do not bypass admission or directly edit usage totals.

## Manual backfill confirmation

Manual X backfill is a paid read and requires an estimate/confirmation operation.

1. Request an estimate for the exact selected account set and requested source.
2. Present the estimated X credits and expiration to the authorized operator.
3. Execute only with the returned one-time confirmation token. The operation is bound to the exact account set, request, estimate, workspace, and expiry.
4. A repeated execution returns the stored result; it must not authorize another paid X read.
5. If the operation is running or outcome-unknown, inspect the durable operation and exposure reservation. Never create a replacement confirmation merely to bypass reconciliation.

Do not log or send confirmation tokens through support channels. Never perform a real reply or DM acceptance test without dedicated test accounts and explicit approval.

## Secret rotation

Rotate one credential class at a time so failures remain attributable.

1. Record the rotation owner and maintenance window without recording the old or new secret.
2. Create the replacement in the X/deployment console, update the development secret store, and redeploy.
3. Verify OAuth callback health, webhook CRC/signature validation, filtered-stream reconciliation, Activity delivery, and an aggregate operations snapshot.
4. Revoke the old credential only after verification.

The stable webhook route key is intentionally independent of `TWITTER_CONSUMER_SECRET`; rotating the consumer secret must not change the route. Rotating `X_INBOX_WEBHOOK_ROUTE_SECRET` changes derived route keys and therefore requires coordinated creation of replacement webhook resources plus durable cleanup of the old generation. A workspace-app client-ID replacement similarly creates a new route/resource generation; retain the cleanup intent until the old upstream resource is deleted.

## Disconnect and workspace-deletion cleanup

Disconnect and workspace deletion enqueue durable `x_inbox_delivery_cleanup_intents` before local cascades remove delivery resources or encrypted credentials.

1. Confirm an intent exists for every stored filtered-stream rule or Activity subscription being removed. The intent may contain encrypted cleanup material; never display it.
2. Allow the delivery cleanup worker to claim the intent. A claim whose lease has expired is stale and safe for a later worker to reclaim.
3. Verify the exact upstream rule/subscription ID is removed, then confirm the intent is deleted.
4. If cleanup fails, retain the intent and its encrypted credentials, inspect only the sanitized error category, and follow retry/backoff. Do not mark it complete manually while the upstream resource may remain.
5. If workspace deletion has completed but an intent remains overdue, escalate to the X application owner before provisioning more capacity.

Never purge cleanup intents to make an alert disappear. Never query or log `app_bearer_token`, customer content, or raw provider responses during routine reconciliation.

## Incident closure

An incident is resolved only when:

- the current aggregate snapshot has no unexplained stale provisional, notification, delivery, or cleanup state;
- capacity is below the agreed threshold or an approved capacity plan is active;
- paused sources are intentionally paused, or restored sources are active with advancing sync timestamps;
- 80/100 notifications are enqueued as expected;
- durable confirmation/outbound operations have a terminal, reconciled state;
- exposure reservations are finalized/released and no BYO uncertain write lacks explicit reconciliation;
- the daily provider-cost input is present and any cost-versus-finalized-usage variance is explained;
- app-scoped capacity, dedup, latency, outbound success, and customer-demand promotion signals have been reviewed;
- no credential or customer content was copied into logs or incident artifacts.
