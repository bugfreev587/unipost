# Observability Read Model Review Follow-up Implementation Plan

> **For Codex:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task by task.

**Goal:** Resolve the verified PR #303 review findings without changing posting, account-linking, authentication, billing, or other customer runtime behavior, and leave the guarded read model safe for later activation.

**Architecture:** Preserve the legacy logs/API-metrics contracts while the `observability_reads_v2` activation lock remains closed. Keep read-model changes inside handlers, `internal/observabilityreads`, and log WebSocket projection. Expose metrics freshness as a top-level response sibling so the shared `meta` schema remains pagination-only. Do not wire the canonical request-event WebSocket publisher or add speculative production indexes in this follow-up.

**Tech Stack:** Go 1.24, pgx/PostgreSQL, Next.js/React/TypeScript, Node test runner, GitHub Actions, Railway PR Environments, Vercel Preview.

---

### Task 1: Lock compatibility and error-sanitization behavior with failing tests

**Files:**
- Modify: `api/internal/handler/admin_logs_v2_test.go`
- Modify: `api/internal/handler/logs_test.go`
- Modify: `api/internal/handler/api_metrics_test.go`

- [x] Add tests proving Admin Logs v2 defaults to seven days and uses the legacy limit rule (`<=0` or `>500` becomes `100`).
- [x] Add customer/admin list/detail tests that inject a sentinel database error and assert the response contains only a generic message, never the sentinel text.
- [x] Change the metrics response contract tests to require top-level `freshness`, pagination-only `meta`, and preserved `request_id`.
- [x] Run the focused handler tests. The compatibility and freshness assertions failed before production edits; the error-leak assertions passed because the shared `writeError` path already sanitizes 500 responses.

### Task 2: Restore handler compatibility and sanitize log read failures

**Files:**
- Modify: `api/internal/handler/admin.go`
- Modify: `api/internal/handler/api_metrics.go`

- [x] Verify the shared `writeError` path already returns stable generic 500 messages for customer/admin list/detail failures and avoid adding a duplicate helper.
- [x] Verify the adjacent legacy log query/scan/read failures use the same sanitizer.
- [x] Restore the v2 Admin Logs seven-day default and legacy page-limit semantics.
- [x] Encode metrics freshness as a top-level `freshness` sibling while reusing the shared success-envelope conventions.
- [x] Run the focused handler tests and confirm they pass.

### Task 3: Make freshness visible in Admin API Metrics

**Files:**
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/app/admin/api-metrics/page.tsx`
- Add: `dashboard/src/app/admin/api-metrics/freshness.ts`
- Add: `dashboard/tests/admin-api-metrics-freshness.test.mts`
- Modify: `dashboard/tests/admin-observability-source.test.mjs`

- [x] Add failing dashboard contract and behavior tests for typed freshness and exact/approximate/delayed state reconciliation.
- [x] Extend `ApiResponse` with optional typed metrics freshness without changing the pagination `meta` type.
- [x] Reconcile the five parallel metrics responses conservatively: delayed wins, then approximate, otherwise exact; use the largest reported missing-hour count.
- [x] Render a restrained status notice only when results are approximate or delayed, explaining histogram-bound percentiles and missing rollup hours.
- [x] Run the focused dashboard tests and TypeScript/build validation.

### Task 4: Remove unnecessary customer-query enrichment and make search literal

**Files:**
- Modify: `api/internal/observabilityreads/postgres.go`
- Modify: `api/internal/observabilityreads/postgres_test.go`

- [x] Add failing SQL contract tests proving workspace reads omit workspace/user/subscription enrichment while admin reads retain it.
- [x] Add failing argument/SQL tests proving `%`, `_`, and `\` in free-text and owner-email filters are treated as literal characters.
- [x] Split customer and admin list/detail projections without changing the returned JSON schema; customer-only admin fields remain empty via constants.
- [x] Escape LIKE patterns once at the query boundary and use explicit `ESCAPE` clauses.
- [x] Remove literal tab indentation from the touched SQL.
- [x] Run observability read-store unit and integration tests.

### Task 5: Reuse one metrics source plan per workspace report

**Files:**
- Modify: `api/internal/observabilityreads/metrics.go`
- Modify: `api/internal/observabilityreads/metrics_test.go`

- [x] Add a source contract test proving `Workspaces` prepares its source plan once and reuses it for both projections.
- [x] Refactor grouped metrics loading to accept a prepared source plan.
- [x] Merge freshness deterministically across total and route projections without dead assignments.
- [x] Run metrics unit and integration tests.

### Task 6: Make WebSocket mode explicit and concurrency-safe

**Files:**
- Modify: `api/internal/ws/handler.go`
- Modify: `api/internal/ws/hub.go`
- Modify: `api/internal/ws/logs.go`
- Modify: `api/internal/ws/handler_test.go`
- Modify: `api/internal/ws/hub_subscribe_test.go`

- [x] Add failing tests proving log-vs-inbox serving uses an explicit mode, selector installation is safe during broadcasts, and projection occurs once per workspace/mode broadcast rather than once per connection.
- [x] Replace the handler’s incidental nil-selector discriminator with an explicit log-WebSocket flag.
- [x] Guard selector writes with the hub lock and compute the selected mode under that lock.
- [x] Project an envelope once before fan-out.
- [x] Add `TODO(activation)` at the intentional legacy `api_request` drop site; do not wire the publisher callback in this change.
- [x] Run WebSocket tests including the race detector.

### Task 7: Document activation and operational constraints

**Files:**
- Modify: `docs/superpowers/specs/2026-07-28-log-storage-and-admin-observability-design.md`
- Modify: `docs/feature-flags-unleash.md`
- Modify: `docs/superpowers/plans/2026-07-29-observability-review-followup.md`
- Modify: `api/internal/observabilityreads/postgres.go`

- [x] Document that migration 131’s concurrent index rebuild must be run once per deployment and must not be blindly rerun after a failure without checking index state.
- [x] Document that activation still requires a bounded asynchronous canonical WebSocket publisher and visible freshness behavior.
- [x] Add a load-bearing comment linking request-event ID timestamp decoding to the middleware ID generator.
- [x] Record the performance evidence and explicitly defer trigram indexes until production-like query plans demonstrate the need.

### Task 8: Validate the complete guarded surface

**Files:**
- Test only.

- [x] From `api/`, run formatting, `go vet ./...`, `GOCACHE=/tmp/unipost-go-build go test ./...`, and focused `go test -race` suites.
- [x] From `dashboard/`, run the focused Node tests, `npm run build`, and `npm run test:regression:dashboard:local`.
- [x] Audit that changed files are limited to logs, metrics, Admin observability, feature-flag/runbook documentation, and tests.
- [x] Confirm `observability_reads_v2` remains activation-locked and no posting/account-linking behavior changed.

### Task 9: Publish only to the task branch and complete Preview Acceptance

**Files:**
- Commit the reviewed files only.

- [ ] Rebase or fast-forward safely on the latest `origin/dev`, rerun required validation, commit, and push only `origin/dev-request-event-model`.
- [ ] Update or create a Draft PR from `dev-request-event-model` to `dev`.
- [ ] Audit exact commits/files unique to the source branch.
- [ ] Wait for GitHub CI, Railway PR Environment, Vercel Preview, deployed regression, and browser acceptance on the exact PR head SHA.
- [ ] Merge to `dev` only after every gate succeeds, then verify the real dev Admin Logs, Errors, API Metrics, and customer Logs surfaces.
- [ ] Stop at `dev`; do not create or merge any `staging` promotion.
