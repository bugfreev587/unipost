# Observability Reads v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Move Logs and API Metrics reads to the canonical request-event model behind the global `observability_reads_v2` kill-switch while preserving a bounded, compatible legacy fallback and leaving every non-observability customer flow unchanged.

**Architecture:** Add one observability-owned PostgreSQL read store that projects canonical request events and non-API integration events into a shared log model, merges them with a source-qualified cursor, and computes recent metrics from raw request events plus idempotent hourly rollups. Existing HTTP handlers keep their legacy query dependencies and select v2 only through `featureflags.Evaluator.Public`; evaluator errors fail safely to legacy. Live Logs use a separate process-level atomic mode cache with a one-second refresh interval and a 500ms evaluation timeout, giving a worst-case 1.5-second fallback SLA; one refresh serves every connection and each broadcast envelope uses one consistent projection. Admin Errors retains its already-contained metadata/detail implementation, so the flag does not touch publishing failure behavior or business records.

**Tech Stack:** Go 1.x, chi, pgx/v5, PostgreSQL 15+, Next.js/TypeScript, Go unit and PostgreSQL integration tests, Vitest, Playwright.

---

## Hard scope and compatibility contract

- Files may change only under observability-owned API handlers, `internal/requestevents`, a new `internal/observabilityreads` package, database migrations/queries for event and rollup relations, Admin/Workspace Logs and Metrics dashboard code, and their tests/docs.
- Do not change publishing, provider adapters, OAuth/account connection, retries, billing, quota, authentication decisions, business transactions, or business-table schemas.
- `observability_reads_v2=OFF`, a missing evaluator, or a flag-store error must execute the existing legacy read path.
- HTTP reads return to legacy on the next request. Every open or new Logs WebSocket returns to legacy within 1.5 seconds without reconnecting; event and connection counts never add selector queries, and SSE remains numeric legacy throughout.
- Before `ActivationReady` may become true, complete a historical backfill or sufficient warm-up for every supported 7-90-day Metrics window. Prove completion coverage for all target hours and legacy parity for counts, status, trends, and percentile behavior where applicable. Before physical raw retention, a sealed workspace-hour or equivalently strong completion contract is required and prevents destructive recompute and data loss.
- The flag controls reads only. Canonical and legacy writers continue unchanged in this workstream.
- v2 list payloads never load request/response details. Details are loaded only after workspace/admin authorization.
- v2 log IDs are source-qualified opaque strings (`request:<event-id>` and `integration:<numeric-id>`). Dashboard clients accept both the old numeric IDs and the new strings during the reversible migration.
- Global merge order is `(timestamp DESC, source_kind ASC, source_id DESC)` with source kinds ordered `request` then `integration`. The cursor contains all three components, IDs are compared only within their own source, and every page uses `limit + 1` without `OFFSET`.
- Admin Logs defaults to 24 hours, 100 rows, and caps at 200. Customer retention/range behavior remains unchanged.
- API Metrics keeps its existing response fields and 90-day maximum range. Raw canonical events are exact for recent, unrolled time; completed hours use idempotent hourly rollups with a fixed 12-bucket duration histogram.

### Hard prerequisite before physical raw-event retention

- The current 48-hour recomputation window is not a retention seal. A later worker must never delete a completed rollup and replace it from already-expired raw input.
- Before any retention step deletes `api_request_events`, add an explicit sealed workspace-hour (or equivalently strong) contract proving the aggregate is complete. Retention may delete a workspace-hour only after that seal exists.
- Once a workspace-hour is sealed, the rollup worker must not destructively recompute it from raw events. Any late-event policy must preserve the sealed aggregate and be defined and tested before retention is enabled.
- The retention/backfill/reclaim task must include a PostgreSQL integration test that seals a workspace-hour, deletes its raw events, reruns the worker, and proves the historical rollup remains unchanged. Task 4's planner boundary tests do not substitute for this seal test.

### Task 1: Lock the v2 log contract with failing unit tests

**Files:**
- Create: `api/internal/observabilityreads/logs.go`
- Create: `api/internal/observabilityreads/logs_test.go`
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/logs/page.tsx`
- Modify: `dashboard/src/app/admin/logs/page.tsx`

1. Add tests for source-qualified ID encode/decode and a versioned opaque cursor containing timestamp, source kind, and source-local ID.
2. Add table tests for merge ordering at identical timestamps, request-only/integration-only pages, deleted rows between pages, and page-size transitions. Assert no duplicate or missing record across consecutive pages.
3. Run `go test ./internal/observabilityreads -run 'Test(LogID|Cursor|Merge)'`; confirm the new tests fail because the implementation is absent.
4. Implement only the typed log projection, ID/cursor codecs, comparator, merge, and `limit + 1` page calculation.
5. Rerun the focused package tests and require PASS.
6. Change dashboard log IDs and selected IDs to `number | string`; URL-encode detail IDs and preserve numeric legacy responses.
7. Run the relevant dashboard unit/type checks and require PASS.

### Task 2: Add workspace-safe canonical and integration log reads

**Files:**
- Create: `api/internal/observabilityreads/postgres.go`
- Create: `api/internal/observabilityreads/postgres_integration_test.go`
- Modify: `api/internal/handler/logs.go`
- Modify: `api/internal/handler/logs_test.go`
- Modify: `api/internal/handler/admin.go`
- Modify: `api/internal/handler/admin_test.go`
- Modify: `api/cmd/api/main.go`

1. Write PostgreSQL integration tests that seed both sources and assert workspace isolation, exclusion of duplicate `integration_logs.category='api_request'` rows, metadata-only list reads, globally stable ordering, cursor continuity, 404 for cross-workspace/unknown detail, and `detail_status=expired` when a failed base request event has no retained detail.
2. Run the focused integration tests and confirm failure because no v2 store exists.
3. Implement indexed, bounded per-source candidate queries in the new store. Apply source-compatible filters before merge, select no payload columns for lists, and fetch only the source-specific detail after base-row authorization.
4. Add a tiny global read-path selector around `featureflags.Evaluator.Public`. Return legacy on OFF, nil evaluator, or evaluation error and emit one structured internal warning on error.
5. Inject the selector/store into customer Logs and Admin Logs handlers without changing their legacy constructors. Route list/detail to v2 only when selected.
6. Preserve legacy parsing and responses on OFF. On v2, enforce Admin defaults of 24 hours/100/max 200 and the existing customer range/limit contract.
7. Run handler unit tests and PostgreSQL integration tests; require PASS.

### Task 3: Implement idempotent hourly rollups test-first

**Files:**
- Create: `api/internal/observabilityreads/rollup.go`
- Create: `api/internal/observabilityreads/rollup_test.go`
- Create: `api/internal/observabilityreads/rollup_integration_test.go`
- Modify: `api/cmd/api/main.go`

1. Add unit tests for the 12 duration buckets, percentile approximation boundaries, completed-hour windows, and 48-hour recomputation range.
2. Add PostgreSQL tests proving a second recomputation replaces rather than doubles counts, late events are incorporated, workspace dimensions remain isolated, and status/error/rate-limit counts match raw events.
3. Run the focused tests and confirm the expected missing-implementation failures.
4. Implement a rollup store using one bounded `INSERT ... SELECT ... ON CONFLICT ... DO UPDATE` recomputation per hour. Never touch customer business tables.
5. Implement a background worker that recomputes completed hours from the most recent 48 hours at startup and hourly thereafter, records freshness/failures, obeys context cancellation, and never affects API readiness or customer responses.
6. Wire the worker only in API process mode and keep its failure behavior internal.
7. Rerun focused unit/integration tests and require PASS.

### Task 4: Route Metrics reads through raw events and rollups

**Files:**
- Create: `api/internal/observabilityreads/metrics.go`
- Create: `api/internal/observabilityreads/metrics_test.go`
- Create: `api/internal/observabilityreads/metrics_integration_test.go`
- Modify: `api/internal/handler/api_metrics.go`
- Modify: `api/internal/handler/api_metrics_test.go`
- Modify: `api/cmd/api/main.go`

1. Add contract tests for Overall, Summary, Trend, Status Codes, and Admin Workspaces responses; assert exact recent raw results, rollup-backed completed hours, filter parity, 90-day rejection, and an explicit data-delay state when neither source is fresh enough.
2. Run focused tests and confirm failure because the v2 metrics reader/routing is absent.
3. Implement observability-owned DTOs matching existing JSON field names. Query raw request events for open/recent hours and rollups for completed historical hours without double counting the boundary.
4. Compute exact percentiles from raw durations and histogram-bounded percentiles from rollups. Keep all existing response fields; add only backward-compatible freshness metadata where needed.
5. Inject the global selector and v2 reader into customer/admin Metrics handlers. OFF/error must call the unchanged sqlc legacy queries.
6. Add parity tests over a controlled cohort comparing legacy and canonical totals, status counts, error rates, route/method filters, and percentile tolerance.
7. Run handler, unit, and PostgreSQL integration tests; require PASS.

### Task 5: Keep Errors contained and activate the reversible read switch

**Files:**
- Modify: `api/internal/featureflags/featureflags.go`
- Modify: `api/internal/featureflags/featureflags_test.go`
- Modify: `api/internal/handler/admin_feature_flags_test.go`
- Modify: `docs/feature-flags-unleash.md`

1. Add tests proving `observability_reads_v2` remains internal/global, defaults OFF, remains activation-locked pending compatible releases and opaque log ID validation from all four public SDKs, is never exposed through customer feature endpoints, and every change remains audited.
2. Confirm Admin Errors list/detail tests still prove metadata-only lists and on-demand debug details. Do not modify publishing failure writes or result semantics.
3. Keep `ActivationReady=false` because the public Go, Java, and JavaScript/TypeScript SDK log models and the current Python/JavaScript validators still assume numeric IDs. Document owner, OFF default, read-only effect, rollback, no third-party dependency, and the requirement that all four SDK repositories release and pass opaque list/get ID validation before a future change may unlock activation. That future activation PR must also wire a bounded `OnPersistedBatch` canonical WebSocket publisher and validate that ON delivers exactly one canonical request event, OFF and SSE preserve numeric no-gap delivery, and selector failure falls back to legacy.
4. Run feature flag and Admin Errors tests; require PASS.

### Task 6: Update observability UI/API compatibility and documentation

**Files:**
- Modify: `dashboard/src/app/docs/api/logs/list/page.tsx`
- Modify: `dashboard/src/app/docs/api/logs/get/page.tsx`
- Modify: `dashboard/src/app/docs/api/logs/page.tsx`
- Modify: `dashboard/src/lib/log-search.ts`
- Modify tests adjacent to each changed dashboard module.

1. While activation remains locked, keep public Logs docs and all JavaScript/TypeScript, Python, Go, and Java examples on the currently shipped positive-integer log ID contract. Keep the future opaque source-qualified ID contract only in this internal plan and internal flag documentation; public docs may change only after all four compatible SDK releases validate. Continue documenting metadata-only lists, failed-only bounded details, expired detail status, and cursor ordering without exposing physical partition keys.
2. Ensure Workspace/Admin Logs display canonical request events without assuming numeric IDs or always-present payloads.
3. Keep Metrics response rendering unchanged unless the optional freshness state is present.
4. Run the repository's explicit observability-adjacent gates, `npm run test:docs-ai` and `npm run test:hosted-connect`, then `npm run build` and `npm run test:regression:dashboard:local`; require PASS with zero skips. The local regression script starts the built dashboard at the RFC localhost name `dev-app.localhost` and lets the suite derive the separate `dev.localhost` landing host, so all 65 tests execute without public DNS or a skipped landing-host check. This dashboard has no generic `npm test` script.

### Task 7: Full safety, parity, and exact-SHA verification

**Files:**
- Modify only tests or observability-owned code necessary to fix proven failures.

1. Verify absolute worktree path, branch, clean intended diff, and audit every changed file against the hard scope above.
2. Run from `api/`: `GOCACHE=/tmp/unipost-go-build go test ./...`, the full PostgreSQL integration suite, `go test -race` for changed packages, and `go vet ./...`.
3. Run from `dashboard/`: `npm run test:docs-ai`, `npm run test:hosted-connect`, the feature-flag source test, `npm run build`, and `npm run test:regression:dashboard:local` with installed browsers; zero skips are required. The named local regression gate supplies the distinct RFC localhost app and landing hostnames `dev-app.localhost` and `dev.localhost`; do not replace it with a public wildcard DNS host, the single-host regression command, or a nonexistent generic `npm test` command.
4. Run protected-flow regression assertions proving identical posting/account-linking/business-table behavior with the read flag OFF, ON, evaluator unavailable, and rollup worker failing.
5. Search the final diff for protected packages/tables and for accidental payload selection in list queries. Any hit outside test fixtures is a blocker.
6. Commit focused changes, push the owned branch, open a Draft PR to `dev`, and record the exact head SHA.
7. Wait for GitHub CI, Railway PR Environment, Vercel Preview, deployed regression, and browser acceptance on that exact SHA. Any non-success or skipped required check is a hard stop.
8. In Preview, verify flag OFF parity and prove `/admin/feature-flags` rejects an ON attempt with `409 FLAG_NOT_READY` without mutating state. Do not run the ON acceptance path until a future activation PR has released compatible JavaScript/TypeScript, Python, Go, and Java SDKs, wired the bounded canonical WebSocket publisher, passed its ON/OFF/SSE/selector-failure safety matrix, and independently unlocked `ActivationReady`.
9. Only after exact-SHA Preview acceptance, re-audit commits/files, mark ready, merge to `dev`, wait for all persistent dev deployments, and repeat real dev acceptance on the official dev domains.
10. Stop after dev deployment and safety acceptance. Do not create or merge a `dev` to `staging` promotion PR; hand the exact dev SHA and acceptance checklist to the user for final dev review.

## Self-review checklist

- Every Stage 2 PRD requirement is assigned above: merged logs/cursor, raw-plus-rollup Metrics, Errors containment, global read flag, OFF rollback, parity, exact-SHA acceptance.
- No task changes a protected customer business schema, provider call, retry rule, transaction, status, billing/quota rule, or account-linking path.
- Every production change follows a preceding failing test and a focused passing test before the full suite.
- Future v2 identities and cursor fields have one consistent `string` representation in internal Go, JSON, TypeScript compatibility code, flag documentation, and this plan; public API docs remain on the shipped numeric ID contract while activation is locked.
- No placeholder, TODO, TBD, skipped test, or unresolved product decision remains in this plan.
