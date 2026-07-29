# Canonical Request Event Write Model — Implementation Plan

> **Workstream:** 2 of 5 in the approved Log Storage / Admin Observability PRD
> **Branch:** `dev-request-event-model`
> **Base:** `origin/dev` at `470f995b6999004fccf836a278d87f5d4a0e6d88`

## Outcome

Introduce the canonical, partitioned API request-event write model as a dark dual-write path. Existing Logs, Metrics, and Errors reads remain unchanged. Existing `api_metrics` and `integration_logs(category='api_request')` writers remain active for a bounded comparison window. The new `observability_reads_v2` flag is registered OFF in the existing PostgreSQL feature-flag control plane and is not evaluated by a read path in this workstream.

The recorder observes only completed API-key requests. Telemetry queue saturation, redaction failure, serialization failure, database failure, and shutdown cannot alter a customer response, business transaction, job, retry decision, provider call, or business-table mutation.

## Explicit allowlist

Only the following surfaces may change:

- `api/internal/requestevents/**` — new telemetry-only event, bounded capture/redaction, queue, PostgreSQL store, and tests.
- `api/internal/db/migrations/130_api_request_events.sql` — new observability relations and the OFF flag seed/check-constraint extension.
- `api/internal/db/**request_event*_test.go`, `migrate_test.go`, and `migration_gate_postgres_integration_test.go` — new-relation contracts and mechanical latest-version test updates only.
- `api/internal/featureflags/featureflags.go`, `api/internal/featureflags/postgres.go`, and their tests — register/list the internal global flag only.
- `api/internal/handler/feature_flags.go` and its test — mark the control as internal in the Super Admin response and prove it remains absent from customer/public feature maps.
- `api/cmd/api/main.go` and narrowly scoped tests — construct/start/stop the recorder and attach its middleware after authentication.
- `dashboard/src/app/admin/feature-flags/page.tsx`, `dashboard/src/lib/api.ts`, and the existing source-contract test — display the authorized internal control with infrastructure-specific semantics and no customer-feature/Super Admin-bypass wording.
- `docs/feature-flags-unleash.md` — document the new internal flag contract.
- this plan and the approved umbrella PRD.

Allowed database relations:

- `api_request_events` plus weekly/default partitions;
- `api_request_error_details` plus aligned weekly/default partitions;
- `api_request_metric_rollups_hourly`;
- `api_request_partition_manifest`;
- `feature_flags` and its existing key check constraint, solely to seed `observability_reads_v2 = FALSE`;
- `feature_flag_changes` only through the existing Admin mutation path; the migration must not fabricate a change record.

## Protected denylist

No schema, query, handler decision, transaction, maintenance statement, or write behavior may change for:

- `social_posts`, `social_accounts`, `social_post_results`, `post_delivery_jobs`;
- outbox, receipt, idempotency, quota, billing, OAuth/Connect, token, webhook, auth, or onboarding relations;
- publishing, scheduling, delivery, retry, account-linking, provider-call, billing/quota, or authentication code paths;
- customer Logs/Metrics/Errors read handlers in this workstream;
- the existing `api_metrics` and `integration_logs` writers during the comparison window.

Any discovered need to cross this boundary is a hard stop requiring a new PRD decision and explicit approval.

## Design decisions

### Schema and partitions

- Use weekly UTC range partitions on `occurred_at` for event and detail parents.
- Use a composite primary key `(occurred_at, id, workspace_id)` so PostgreSQL partition uniqueness rules and tenant-consistent detail references are satisfied without a second redundant unique index; the external identifier remains the opaque `id`.
- Generate time-sortable event IDs in the application without adding a new runtime dependency.
- Keep parent/detail partition bounds identical and record them in `api_request_partition_manifest`.
- Create a current-week and next-week partition plus a fail-closed default partition for deployment safety. Partition lifecycle automation and destructive detach/drop belong to Workstream 4.
- Do not add foreign keys to protected business relations. `workspace_id`, `api_key_id`, profile/account/post IDs are immutable correlation values only.
- Store failure event and its optional detail atomically in one short telemetry-only transaction.
- Enforce detail-to-event workspace identity with a composite database foreign key, not only application validation.
- Create the hourly rollup relation and idempotent uniqueness contract now; rollup population belongs to Workstream 3.

### Capture and redaction

- Only API-key-authenticated workspace requests are eligible, matching current Metrics behavior and skip rules.
- Use the matched Chi route pattern; the fallback normalizes resource-like path segments and excludes query strings.
- A capped tee observes at most 32 KB while the handler reads JSON/text request bodies. It never pre-reads the request.
- Binary and multipart bodies retain only content type, observed byte count, a SHA-256 only when the complete body fits within the 32 KB observation budget, and an omission reason; zero body bytes are persisted. Oversized bodies are not hashed beyond the cap and explicitly record `hash_omitted_size`.
- Query values are omitted. The detail may store only allowlisted query-key names.
- Request and response headers use a fixed allowlist. Authorization, cookies, tokens, API keys, signatures, and secrets can never enter the detail object.
- JSON redaction is recursive and versioned. Text storage is limited to an explicit content-type allowlist.
- Successful responses are not buffered. Response capture starts only after a `4xx` or `5xx` status is established.
- Request and response excerpts are capped at 32 KB each, with a 70 KB serialized-detail database constraint.
- `402` remains metadata-only. `429` may retain a bounded response excerpt but never a request excerpt.

### Reliability and isolation

- Use bounded normal and high-priority queues; never spawn a goroutine per request.
- Success events enqueue to the normal queue and may be dropped only on saturation, with an explicit counter and structured warning.
- Failure events use the high-priority queue. If full, the middleware starts a detached, short-timeout fallback attempt without awaiting it; inability to persist increments a dedicated failure counter.
- Batch success writes; write each failure event/detail atomically.
- Queue admission copies immutable telemetry values after the handler outcome is established.
- Recorder shutdown stops admission, drains within a bounded deadline, and reports incomplete drain without blocking customer shutdown indefinitely.
- Expose aggregate snapshots for queue depth, accepted/written/dropped/failed counts, batch latency, and insert latency through a Super Admin-only `/v1/admin/observability/request-events` endpoint. It contains no event, workspace, request, or payload data and introduces no customer API.

### Shadow comparison

- The existing legacy writers stay unchanged and authoritative for reads.
- The new recorder counts eligible events by workspace/hour/route/method/status/outcome.
- Comparison code reads aggregate legacy and canonical counts only in tests and controlled operational validation; it does not add a synchronous query to a request. Exact acceptance uses a newly created, otherwise-unused API key and one known route/method/status lifetime cohort after both async queues settle, so no time boundary can split the same request. Time-window comparison remains advisory only. No production background hourly exact-parity alert is started because the two existing asynchronous writers can cross an hour boundary at different insert times.
- Acceptance requires zero new-writer drops and exact eligible-count parity after accounting for the existing intentional legacy exclusions (notably `402` in `integration_logs`).

### Feature flag

- Register `observability_reads_v2` in the repository's current PostgreSQL feature-flag system and seed it OFF.
- The flag is environment-global. Future read code must use `Evaluator.Public`, not `ForWorkspace`, so the Super Admin workspace bypass cannot force it ON.
- This workstream does not evaluate the flag and does not switch reads, writes, retention, backfill, or cleanup.
- The backend rejects an attempt to enable the flag until a later read-migration workstream marks the definition activation-ready; the Admin page renders the control as **Prepared** rather than claiming canonical reads are active.

## Test-driven implementation sequence

1. **Migration contracts first**
   - Add failing source-contract tests for the relation allowlist, composite identity, aligned partition bounds, detail limits, hourly-rollup uniqueness, and OFF flag seed.
   - Add a disposable-PostgreSQL integration test proving current/next-week routing, atomic failure detail insertion, parent row deletion cascading inside a partition, workspace isolation predicates, and absence of protected-relation DDL/locks.
   - Add migration 130 and make only those tests pass.

2. **Pure normalization and redaction**
   - Add failing table tests for deterministic outcomes, matched/fallback route normalization, query-value omission, header allowlisting, recursive secret redaction, text limits, binary/multipart omission, hashes, and total-detail bounds.
   - Implement the pure functions without database or handler dependencies.

3. **Response/request observation compatibility**
   - Add failing middleware tests for implicit/explicit status, streaming/flushing, successful response no-capture, failure-only capture, `402` metadata-only, `429` response-only, and preserved response bytes/status/headers.
   - Verify supported optional `http.ResponseWriter` behavior remains available to handlers.

4. **Bounded recorder and store**
   - Add failing tests for success batching, atomic event+detail writes, high-priority preference, saturation counters, failed direct fallback, bounded shutdown, and writer failure isolation.
   - Implement the recorder using telemetry-owned contexts and short timeouts.

5. **Dark dual-write wiring**
   - Add failing integration tests showing one eligible request still produces the legacy writes and independently attempts exactly one canonical event.
   - Wire the recorder in `main.go` after auth, without changing handlers or existing writers.
   - Add forced saturation/store-failure tests that compare exact handler status, headers, body, and a fake business mutation/provider-call count against the recorder-disabled baseline.

6. **Flag registration**
   - Extend feature-flag tests first.
   - Seed `observability_reads_v2` OFF and document owner, OFF/ON contract, rollback, and no third-party dependency.
   - Prove migrations/restarts cannot turn an existing value ON or OFF.

7. **Local verification**
   - Run formatter and focused package tests.
   - Run `GOCACHE=/tmp/unipost-go-build go test ./...`.
   - Run relevant race tests for the recorder and redaction/capture package.
   - Run all migrations 0→130 against disposable PostgreSQL.
   - Inspect migration plans/locks and prove no protected relation appears in migration DDL.
   - Run dashboard build and regression suite because shared API middleware can affect authenticated dashboard routes, even though no dashboard file changes.

8. **Preview and dev acceptance**
   - Audit exact commits and changed files against this allowlist.
   - Push only `dev-request-event-model`; create a Draft PR to `dev`.
   - Require GitHub CI, Railway PR Environment, Vercel Preview, deployed regression, and browser acceptance on the exact PR head SHA.
   - Validate Preview schema/partitions/constraints, flag OFF state, queue health, canonical-versus-legacy count parity, and unauthenticated/auth boundaries.
   - Exercise publishing, scheduling/retry, connected-account listing, OAuth/Connect callback validation, auth, quota/billing, and idempotency regressions with healthy, saturated, and unavailable telemetry dependencies.
   - Merge only after all gates pass; then repeat real-dev acceptance on the exact merged SHA.

## Rollback

- Application rollback is safe because all existing reads and writers remain active and unchanged.
- The new middleware can be removed without data conversion.
- `observability_reads_v2` remains OFF; no toggle is required to restore legacy reads.
- Migration downgrade is not required for application rollback. New observability tables may remain unused until a separately reviewed cleanup.

## Completion evidence for this workstream

- Exact branch and deployed SHA.
- Complete changed-file and changed-relation audit matching the allowlist.
- Full local and remote test/deployment results.
- Preview and real-dev schema/partition/flag evidence.
- Zero dropped success events and zero failed failure events in the acceptance window.
- Eligible canonical/legacy event parity report with documented legacy exclusions.
- Customer core-flow non-regression evidence showing identical outcomes under healthy, saturated, and failing telemetry conditions.
