# X Account Profile and Post History OpenAPI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose live, managed-user-scoped X profile and authored-post history APIs with opaque pagination, feature-flag-controlled X Credits accounting, idempotent replay, durable ambiguous-outcome recovery, and auditable Credits visibility.

**Architecture:** Generalize the existing X read-exposure state machine so Inbox and account reads share one authoritative reservation/settlement path while retaining source-specific deadlines and safety policy. Add an account-read receipt layer for request ownership, encrypted retry input, and 24-hour normalized-response replay; keep financial state solely in the generalized exposure. A focused account-read service owns authorization-independent orchestration, X normalization, cursor binding, idempotency, and recovery, while HTTP handlers enforce the public contract.

**Tech Stack:** Go 1.x, chi, pgx/PostgreSQL migrations, sqlc, existing UniPost feature flags and X Credits packages, Next.js/TypeScript documentation, Go tests, Node source tests, Playwright deployed acceptance.

---

## Task 1: Generalize the X read exposure and add durable receipts

**Files:**
- Create: `api/internal/db/migrations/135_x_account_reads.sql`
- Modify: `api/internal/db/x_inbox_durable_operations_test.go`
- Modify: `api/internal/xcredits/service.go`
- Modify: `api/internal/xcredits/exposure_postgres.go`
- Modify: `api/internal/xcredits/service_test.go`
- Modify: `api/internal/xcredits/rollout.go`
- Modify: `api/internal/xcredits/rollout_test.go`
- Create: `api/internal/xcredits/account_read_postgres_test.go`

- [ ] Add failing migration/source tests for a generalized `x_read_exposures` table and `x_read_receipts` with Workspace/account/Managed User ownership, purpose, hashed idempotency key, fingerprint, encrypted retry request, terminal response, attempts, lease, and 24-hour expiry.
- [ ] Add failing service tests proving strict whole-request admission, a 24-hour account-read deadline, zero-unit flag/app-mode bypass, unchanged 30-minute Inbox behavior, and atomic receipt-plus-exposure admission before an upstream call.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/xcredits` from `api/` and verify the new tests fail for the missing schema and API.
- [ ] Implement migration 135 by renaming/generalizing the existing exposure table, preserving its rows/indexes, adding source policy columns, and creating receipt constraints/indexes with cascading permanent-deletion ownership.
- [ ] Extend exposure request/store types with purpose, Managed User, safety policy, reconciliation deadline, and an optional same-transaction mutation. Preserve Inbox defaults and make account reads strict and independent of the Inbox daily cap.
- [ ] Route all customer-accounting decisions through the existing `x_credits_billing_v1` rollout service; persist zero-unit exposures and receipts for bypassed reads instead of returning an untracked in-memory bypass.
- [ ] Re-run the focused tests until they pass, then run existing Inbox exposure tests to prove backward compatibility.
- [ ] Commit: `feat: generalize X read exposure accounting`

## Task 2: Add status-aggregated Credits snapshots and the event ledger

**Files:**
- Modify: `api/internal/db/queries/x_usage.sql`
- Modify generated files under: `api/internal/db/`
- Modify: `api/internal/xcredits/service.go`
- Modify: `api/internal/xcredits/postgres.go`
- Create: `api/internal/xcredits/events.go`
- Modify: `api/internal/xcredits/postgres_test.go`
- Modify: `api/internal/handler/billing.go`
- Modify: `api/internal/handler/billing_test.go`
- Modify: `api/cmd/api/main.go`

- [ ] Add failing store and handler tests for finalized, pending, and effective monthly usage, backward-compatible `monthly_used`, balance based on effective usage, cursor-paginated event listing, every documented filter, and Workspace confinement.
- [ ] Add SQL queries that aggregate current-period usage by event/exposure status and seek-page generalized exposure events without returning idempotency material or retained response content; regenerate sqlc output with the repository-supported sqlc command.
- [ ] Extend `xcredits.Snapshot` and its PostgreSQL implementation with finalized/pending/effective fields while keeping `monthly_used == effective`.
- [ ] Implement `GET /v1/billing/x-credits/events` behind the same existing `x_credits_billing_v1` availability gate as the snapshot and map safe public ledger fields.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./internal/xcredits ./internal/handler ./cmd/api` and confirm all focused tests pass.
- [ ] Commit: `feat: expose X Credits ledger details`

## Task 3: Implement the X provider profile and authored-post client

**Files:**
- Modify: `api/internal/platform/twitter.go`
- Modify: `api/internal/platform/twitter_test.go`

- [ ] Add failing fixture tests for live profile fields, optional fields, original posts, replies to others, self-replies, Quote Posts, Reposts, media, public metrics, pagination, empty timelines, over-limit provider responses, rate limits, definite failures, malformed/truncated responses, and sanitized errors.
- [ ] Add typed profile and authored-post page methods using `/2/users/{id}` and `/2/users/{id}/tweets`, bounded response reads, explicit X field/expansion lists, and provider retry metadata.
- [ ] Normalize provider data into provider-neutral profile/post structs without contaminating authored text with quoted content; cap admitted resources at the requested limit and report the raw count only to telemetry.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./internal/platform` and confirm the focused tests pass.
- [ ] Commit: `feat: read X profiles and authored posts`

## Task 4: Add encrypted, scope-bound public cursors

**Files:**
- Create: `api/internal/xaccountreads/cursor.go`
- Create: `api/internal/xaccountreads/cursor_test.go`

- [ ] Add failing tests for round trips and rejection of modified, expired, cross-Workspace, cross-account, cross-Managed User, time-range, and filter-mismatched cursors; assert the upstream token never appears in cursor plaintext or JSON.
- [ ] Implement authenticated encryption using a dedicated derived key from existing server-side secret material, canonical filter binding, seven-day expiry, and safe key/version rotation metadata.
- [ ] Add retry-cursor refresh behavior that preserves the exact upstream token without exposing it.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./internal/xaccountreads -run Cursor` and confirm all cursor tests pass.
- [ ] Commit: `feat: add scoped X history cursors`

## Task 5: Implement account-read orchestration, receipts, and recovery

**Files:**
- Create: `api/internal/xaccountreads/types.go`
- Create: `api/internal/xaccountreads/store.go`
- Create: `api/internal/xaccountreads/postgres.go`
- Create: `api/internal/xaccountreads/service.go`
- Create: `api/internal/xaccountreads/service_test.go`
- Create: `api/internal/xaccountreads/postgres_test.go`
- Create: `api/internal/worker/x_account_read_recovery.go`
- Create: `api/internal/worker/x_account_read_recovery_test.go`

- [ ] Add failing service tests for exact request fingerprints, one-owner execution, completed replay, in-progress and settlement-pending responses, mismatched-key conflicts, strict preauthorization, actual scanned-resource settlement, shorter-page release, definite-failure release, and finalization-pending recovery.
- [ ] Add failing ambiguity tests proving timeout/truncation persists `outcome_unknown`, the reconciler leases one operation, retries the exact encrypted request with bounded backoff, makes success replayable, releases at 24 hours, and never back-charges after forced release.
- [ ] Implement keyed idempotency hashes, canonical fingerprints, atomic receipt/exposure creation, execution leases, response serialization capped to 24 hours, cleanup, and safe reconciliation callbacks into the shared exposure service.
- [ ] Add bounded per-Workspace and per-account admission (initial defaults: 60 requests/minute per Workspace, 20 requests/minute per account, and two concurrent reads per account), independent of customer Credits state.
- [ ] Add structured content-free logs/metrics for operation state, scan counts, Credits, replay/conflict, rate limits, and reconciliation age.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./internal/xaccountreads ./internal/worker` and confirm all focused tests pass.
- [ ] Commit: `feat: orchestrate durable X account reads`

## Task 6: Expose profile, posts, and capability contracts

**Files:**
- Create: `api/internal/handler/x_account_reads.go`
- Create: `api/internal/handler/x_account_reads_test.go`
- Modify: `api/internal/handler/platforms.go`
- Modify: `api/internal/handler/platforms_x_inbox_test.go`
- Modify: `api/internal/platform/capabilities.go`
- Modify: `api/internal/platform/capabilities_test.go`
- Modify: `api/cmd/api/main.go`
- Modify: `api/cmd/api/x_inbox_outbound_recovery_wiring_test.go`

- [ ] Add failing handler tests for missing Workspace, cross-Workspace 404, exact Managed User checks, missing selector, null ownership, wrong platform, required scopes/refresh capability, required idempotency key, query validation, stable errors, Credits details, filters, empty pages, and no X call on every precondition failure.
- [ ] Add failing capability tests for schema `1.8`, optional selector compatibility, mismatch denial, authorization state, app-mode policy, flag-on effective weights, flag-off `feature_disabled`, Workspace-app `customer_x_app`, and fail-closed flag evaluation.
- [ ] Implement `GET /v1/accounts/{id}/profile` and `GET /v1/accounts/{id}/posts`, reusing existing encrypted-token refresh/persistence behavior and mapping all service outcomes to the documented status/code/retry contract.
- [ ] Extend account capabilities additively with `x_account_reads`, catalog/effective pricing, required scopes and reauthorization state; do not call X or consume Credits.
- [ ] Wire handlers and the receipt recovery/cleanup worker in `main.go`, deriving cursor/idempotency encryption keys from configured server secrets and failing startup when production-safe key material is absent.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./internal/handler ./internal/platform ./cmd/api` and confirm all focused tests pass.
- [ ] Commit: `feat: expose X profile and history APIs`

## Task 7: Document the public OpenAPI and rollout policy

**Files:**
- Create: `dashboard/src/app/docs/api/accounts/profile/page.tsx`
- Create: `dashboard/src/app/docs/api/accounts/posts/page.tsx`
- Modify: `dashboard/src/app/docs/api/accounts/capabilities/page.tsx`
- Modify: `dashboard/src/app/docs/api/x-credits/page.tsx`
- Modify: `dashboard/src/app/docs/api/page.tsx`
- Modify: `dashboard/src/lib/docs-ai-search-index.ts`
- Modify docs navigation/sitemap files discovered by `rg` during implementation
- Modify: `dashboard/tests/x-feature-flag-docs-source.test.mjs`
- Modify: `dashboard/tests/x-credits-foundation-source.test.mjs`
- Create: `dashboard/tests/x-account-reads-docs-source.test.mjs`
- Modify: `docs/feature-flags-unleash.md`
- Modify: `docs/api-reference-gap-audit.md`
- Modify: `docs/sdk-api-coverage-matrix.md`

- [ ] Before editing dashboard docs, read and apply the installed `design-taste-frontend` skill as required by repository instructions.
- [ ] Add failing source tests for discoverability and all public contract details: server-side API keys, Managed User selector, scanned-resource limit, one-page cursors, per-page idempotency, filters, deduplication, thread limitations, flag/app-mode matrix, 402 behavior, and ledger privacy.
- [ ] Implement concise, navigable API reference pages and examples; update search/navigation/sitemap and the feature-flag runbook with new accounting call sites and rollback behavior.
- [ ] Run the focused Node source tests and `npm run build` from `dashboard/`.
- [ ] Commit: `docs: publish X account read API reference`

## Task 8: Run complete local validation and security review

**Files:**
- Modify only files required to fix failures found by validation.

- [ ] Verify the exclusive worktree path and branch before every validation or fix.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./...` from `api/`.
- [ ] Run `npm run build` from `dashboard/`.
- [ ] Run `npm run test:regression:dashboard` from `dashboard/` when Playwright browsers are available, as required for docs/shared shell changes.
- [ ] Search generated responses, error paths, logs, fixtures, and ledger DTOs for raw access/refresh tokens, raw idempotency keys, upstream pagination tokens, provider bodies, profile descriptions, and post text leakage.
- [ ] Re-audit changed files against the design acceptance criteria and confirm no unrelated changes.
- [ ] Commit any validation-only corrections as focused commits.

## Task 9: Draft PR and exact-SHA Preview Acceptance

- [ ] Push only `dev-x-account-profile-history-openapi` and open a Draft PR to `dev`.
- [ ] Record the exact PR head SHA and audit its unique commits and files.
- [ ] Wait for GitHub CI, the Railway PR Environment, the Vercel Preview, deployed regression, and every visibly triggered check to finish on that SHA.
- [ ] Perform browser/API acceptance on the isolated Preview: capability scoping, profile, two post pages, filters, same-key replay, insufficient balance with zero upstream calls, flag-off bypass, Workspace-app bypass, balance/ledger agreement, and secret/content-free logs.
- [ ] Treat any failure, timeout, cancellation, skipped/missing result, or different-SHA result as a hard stop; fix on the task branch and repeat the complete gate on the replacement SHA.

## Task 10: Merge to development and verify the real dev environment

- [ ] After all exact-SHA Preview gates pass, mark the PR ready, repeat the commit/file content audit, and merge only the task PR into `dev`.
- [ ] Wait for all development deployments and triggered checks to complete successfully.
- [ ] Personally verify the expected account-read and Credits behavior on `https://dev-api.unipost.dev` and relevant docs on the official development frontend.
- [ ] Report completion only after real-dev acceptance succeeds; do not promote to staging or production without a separate explicit user instruction.
