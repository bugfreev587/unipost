# X Inbox Webhook Hotfix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Restore X Inbox real-time ingestion for replies/comments and legacy unencrypted dm.received events while preserving exact managed-user isolation, existing workspace aggregate access, bounded provider spend, and fail-closed DM rollout behavior.

**Architecture:** Keep comments/replies on the existing app-scoped Filtered Stream and legacy DMs on X Account Activity webhooks. The worker computes the two desired states independently, evaluates x_dms_v1 through the shared backend feature-flag evaluator, gates DM provisioning through a strict account canary, and stores a dedicated persistent 403 latch. Ingestion resolves legacy DM recipients only by exact provider user ID and rejects zero or multiple matches before any write or notification. XChat chat.received is excluded.

**Tech Stack:** Go, PostgreSQL/sqlc, X API v2 Filtered Stream, X Account Activity webhooks, Railway, GitHub Actions.

## Scope and workflow invariants

Expected implementation surfaces:

- api/internal/worker/x_inbox_dm_canary.go and tests
- api/internal/worker/x_inbox_delivery.go and tests
- api/internal/xinbox/client.go, subscriptions.go, ingest.go, postgres_ingest.go and tests
- api/internal/db/queries/inbox.sql and x_inbox.sql, generated sqlc files, and contract tests
- api/internal/db/migrations/120_x_inbox_dm_forbidden_latch.sql
- api/cmd/api/main.go and x_inbox_delivery_wiring_test.go

No publishing, analytics, XChat, or generalized Inbox redesign is in scope.

- [ ] Work only in /Users/xiaoboyu/.codex/worktrees/hotfix-x-inbox-webhooks/unipost on hotfix-x-inbox-webhooks.
- [ ] Before every write, test, commit, push, deployment, merge, or promotion, run git fetch origin --prune and verify the absolute worktree path, branch, and expected status.
- [ ] Never touch /Users/xiaoboyu/.config/superpowers/worktrees/unipost/hotfix-inbox-comments-idor.
- [ ] Treat any required failure, skip, timeout, cancellation, missing result, or SHA mismatch as a hard stop.
- [ ] Never push directly to staging, main, or dev.
- [ ] Never print, reload, or reuse the production UniPost API key previously pasted into chat. Revoke/rotate it before production acceptance.
- [ ] Do not run DM backfill in this hotfix. Acceptance uses fresh provider-side events only.

## Task 1: Add a whole-list-fail-closed DM canary parser

**Files:**

- Create: api/internal/worker/x_inbox_dm_canary.go
- Create: api/internal/worker/x_inbox_dm_canary_test.go

- [ ] Write table-driven failing tests for a missing value, whitespace only, one UUID, trimmed UUIDs, duplicates, an empty member, and one malformed member among valid UUIDs.
- [ ] Define ParseXInboxDMCanary(raw string) (map[string]struct{}, error). Missing or whitespace-only input returns an empty set and no error. Every non-empty comma-separated member must parse as UUID. Any blank or malformed member rejects the whole list and returns an empty set plus error.
- [ ] Run the red test: cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/worker -run TestParseXInboxDMCanary -count=1. Require failure because the parser is absent.
- [ ] Implement only trimming, UUID validation, deduplication, and whole-list rejection.
- [ ] Rerun the focused test and require PASS.
- [ ] Commit: git commit -m \"fix: validate X DM canary configuration\".

## Task 2: Make provider HTTP failures status-aware and secret-safe

**Files:**

- Modify: api/internal/xinbox/client.go
- Modify: api/internal/xinbox/subscriptions.go
- Modify: api/internal/xinbox/subscriptions_test.go

- [ ] Add failing tests proving non-2xx responses expose method, URL path, status, provider error code, and title, but never authorization headers, bearer values, raw body, query string, or provider detail.
- [ ] Add failing tests proving webhook/subscription delete treats 404 and 410 as idempotent success while 403 remains an error.
- [ ] Add ProviderHTTPError with Method, Path, StatusCode, Code, and Title, plus IsProviderHTTPStatus(err, status).
- [ ] Run the red tests: cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/xinbox -run 'TestProviderHTTPError|TestDelete.*Idempotent' -count=1.
- [ ] Implement bounded JSON error decoding and remove query parameters. Never retain raw provider bodies or detail fields.
- [ ] Update delete behavior to accept only 404/410 as already absent.
- [ ] Run all xinbox tests and require PASS.
- [ ] Commit: git commit -m \"fix: classify X provider HTTP failures safely\".

## Task 3: Persist a dedicated DM 403 latch

**Files:**

- Create: api/internal/db/migrations/120_x_inbox_dm_forbidden_latch.sql
- Create: api/internal/db/x_inbox_dm_latch_contract_test.go
- Modify: api/internal/db/queries/x_inbox.sql
- Regenerate: api/internal/db/x_inbox.sql.go and api/internal/db/models.go when changed
- Modify: api/internal/worker/x_inbox_delivery.go and tests

- [ ] Write a failing contract test requiring a nullable dm_subscription_forbidden_fingerprint TEXT column and requiring state queries to read and write it.
- [ ] Extend worker store tests so the saved latch survives ListAccounts independently of last_error.
- [ ] Run the red tests: cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/worker -run 'TestXInboxDMLatchContract|Test.*ForbiddenFingerprint' -count=1.
- [ ] Add the additive nullable migration without altering or deleting delivery rows.
- [ ] Update sqlc queries and worker account/state models. Update the fake store.
- [ ] Regenerate with: cd api && go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate.
- [ ] Rerun focused tests and require PASS.
- [ ] Commit: git commit -m \"fix: persist X DM forbidden provisioning latch\".

## Task 4: Restore independent comment and legacy-DM reconciliation

**Files:**

- Modify: api/internal/worker/x_inbox_delivery.go
- Modify: api/internal/worker/x_inbox_delivery_test.go

- [ ] Add failing tests for the full DM conjunction: account active, plan permits Inbox, dm.read present, workspace x_dms_v1 true, account in strict canary, managed app bearer present, consumer secret configured, webhook URL present, and spend safety permitted.
- [ ] Prove comment and DM desired states are independent. Flag-off, evaluator error, missing DM scope, missing consumer secret, or missing webhook must not disable a valid comment stream.
- [ ] Prove evaluator errors fail closed for DMs and make no DM creation call.
- [ ] Prove eligible state calls EnsureWebhook before EnsureDMSubscription with dm.received, exact provider user ID, and stable account tag.
- [ ] Prove route replacement deletes only the exact recorded account subscription, persists the cleared subscription and route, then ensures the app-level webhook and replacement subscription, and converges idempotently on the next cycle.
- [ ] Prove the per-account worker does not directly enumerate app-scoped webhooks to identify stale resources and never calls DeleteWebhook. EnsureWebhook may internally list, reuse, revalidate, or create the exact configured URL; stale app-webhook cleanup requires a future generation-aware, leased app-level design.
- [ ] Prove subscription-create 403 stores the dedicated fingerprint, preserves comments, and suppresses later provider calls while unchanged.
- [ ] Prove flag-off, canary removal, and deliberate off-to-on clear the latch.
- [ ] Prove app mode, app identity, non-secret webhook URL, or provider-user changes allow one controlled retry. Bearer/consumer-secret-only replacement requires deliberate off-to-on.
- [ ] Prove shared stream semantics with two accounts on one app: one stream serves both, disabling one preserves it, disabling the last stops it, DM changes do not churn it, and incomplete discovery preserves existing streams.
- [ ] Run focused red tests: cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/worker -run 'TestXInboxDelivery|Test.*DM.*Desired|Test.*SharedStream' -count=1.
- [ ] Add DMsAvailable func(context.Context, string) (bool, error) and DMCanaryAccountIDs map[string]struct{} to XInboxDeliveryConfig.
- [ ] Replace the hard-coded DM false value with independent eligibility. Require consumer secret and webhook only for DMs; apply spend safety when either source is desired.
- [ ] Restore EnsureWebhook then EnsureDMSubscription using the existing app-bearer client. Do not switch to user OAuth.
- [ ] Build the fingerprint only from non-secret app mode, app identity, social account, provider user, non-secret webhook URL, and event. Never include bearer or consumer secret values; a webhook URL change allows one controlled retry.
- [ ] Keep last_error as the latest sanitized human summary only; never parse it for control flow. Preserve source-specific logs/metrics.
- [ ] Run all worker tests and require PASS.
- [ ] Commit: git commit -m \"fix: reconcile X comments and legacy DMs independently\".

## Task 5: Enforce exact single-account legacy DM routing and exclude XChat

**Files:**

- Modify: api/internal/db/queries/inbox.sql and generated api/internal/db/inbox.sql.go
- Modify: api/internal/db/x_inbox_ingest_contract_test.go
- Modify: api/internal/xinbox/ingest.go, postgres_ingest.go, and their tests

- [ ] Change the SQL contract test first so fallback routing uses only sa.external_account_id = sqlc.arg(provider_user_id)::TEXT.
- [ ] Keep the query as :many and do not add LIMIT 1; service code must detect ambiguity.
- [ ] Add failing tests for zero, one, and multiple matches plus row-conversion failure. Zero/multiple/error cases write no Inbox item and emit no notification.
- [ ] Add failing tests proving dm.received and legacy direct_message_events route exactly, while chat.received is not admitted as x_dm.
- [ ] Run red tests: cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/xinbox -run 'TestXInboxIngestContract|Test.*ProviderUser|Test.*Ambiguous|Test.*XChat' -count=1.
- [ ] Rename the store boundary to AccountsForProviderUser and fail the whole lookup on conversion error instead of skipping a row.
- [ ] Add a sentinel ambiguity error and enforce exactly one candidate before persistence or notification.
- [ ] Regenerate sqlc, run all xinbox and DB contract tests, and require PASS.
- [ ] Commit: git commit -m \"fix: isolate legacy X DMs by exact provider user\".

## Task 6: Wire shared feature evaluation and fail-closed canary

**Files:**

- Modify: api/cmd/api/main.go
- Modify: api/cmd/api/x_inbox_delivery_wiring_test.go

- [ ] Add failing wiring tests proving the worker receives a callback using featureflags.XDMSV1, receives the parsed canary, and contains no scattered feature environment read.
- [ ] Prove invalid canary config yields an empty DM canary and only a sanitized configuration-class error.
- [ ] Run red test: cd api && GOCACHE=/tmp/unipost-go-build go test ./cmd/api -run TestXInboxDeliveryWiring -count=1.
- [ ] Parse the canary environment value once at startup.
- [ ] Wire featureFlagEvaluator.ForWorkspace(ctx, workspaceID, featureflags.XDMSV1) into DMsAvailable.
- [ ] Preserve backend authority and existing Super Admin owner fallback; do not copy evaluator policy into the worker.
- [ ] Run the focused test and require PASS.
- [ ] Commit: git commit -m \"fix: wire X DM feature gating into delivery worker\".

## Task 7: Local verification and implementation review

- [ ] Re-fetch and verify owned worktree/branch.
- [ ] Run: cd api && GOCACHE=/tmp/unipost-go-build go test -race ./internal/worker ./internal/xinbox -count=1.
- [ ] Run: cd api && GOCACHE=/tmp/unipost-go-build go test ./....
- [ ] Regenerate sqlc, then require git diff --exit-code -- api/internal/db.
- [ ] Run gofmt on all touched Go files and git diff --check.
- [ ] Audit with git log --oneline origin/staging..HEAD and git diff --name-status origin/staging...HEAD.
- [ ] Use superpowers:requesting-code-review. Resolve behavior changes with a new failing test first.
- [ ] Repeat the complete required suite after the final review change. Any failure/skip/timeout/cancellation/missing result is a hard stop.

## Task 8: PR to staging and bounded staging canary

- [ ] Re-fetch, verify ownership, and audit exact commits/files.
- [ ] Push only hotfix-x-inbox-webhooks and open a PR targeting staging.
- [ ] Monitor every GitHub/Railway check on the exact head SHA. Merge only after all required checks succeed.
- [ ] Wait for staging deployment and verify deployed SHA.
- [ ] Start with an empty DM canary and confirm comments remain healthy.
- [ ] Verify the acceptance workspace is Super Admin-owned and x_dms_v1 evaluates true through the backend. Otherwise stop before provider mutation.
- [ ] Verify required credentials and app-specific webhook URL exist without revealing them.
- [ ] Run a synthetic CRC probe against the deployed staging route before provider webhook creation/revalidation. Failure is a hard stop.
- [ ] Reconfirm X pricing/credits before any billable call and obtain approval for the maximum bounded cost.
- [ ] Add exactly one user-owned staging account to the canary and execute one bounded reconciliation.
- [ ] On provider 403, confirm the latch, preserve comments, disable canary/flag, hard stop, and do not retry.
- [ ] Require one app-scoped Filtered Stream rule for the intended account set, one valid app-specific webhook, and one exact dm.received subscription.
- [ ] Ask for one fresh reply/comment and one fresh legacy unencrypted DM; do not backfill.
- [ ] Verify owner/admin aggregate reads, owning managed-user reads, all other managed users return 404, no cross-account duplicate exists, and no XChat event is stored as x_dm.
- [ ] Run publishing and analytics smoke checks.

## Task 9: Promote staging to production and verify

- [ ] Revoke/rotate the production UniPost API key previously pasted into chat before production acceptance. Never display, reload, or reuse it.
- [ ] Audit staging-only commits/files and open staging to main PR.
- [ ] Monitor exact-SHA checks; merge only after all succeed. Wait for production deployment and verify its SHA/health.
- [ ] Configure only missing production secrets without revealing values or rotating unrelated X credentials. Keep DM canary empty initially.
- [ ] Verify workspace/profile 16202f3f-0c3c-4b92-afae-177f279c692a is Super Admin-owned and evaluator-eligible. Otherwise hard stop.
- [ ] Run synthetic CRC against production app-specific route before provider mutation.
- [ ] Enable only social account bc507960-aed6-4ae7-8568-27ad63cf5c58 and provider user 2039562772455809024.
- [ ] Execute one bounded reconciliation. On failure, timeout, 403, scope ambiguity, or spend ambiguity, disable canary/flag, preserve comments, hard stop, and do not retry.
- [ ] Verify exactly one production streaming rule for @unipostdev, one valid app-specific webhook, and one dm.received subscription for provider user 2039562772455809024.
- [ ] Ask for one fresh reply/comment and one fresh legacy unencrypted DM between user-owned accounts. Do not expect old events.
- [ ] Verify both enter only sdk-inbox-x; owner/admin aggregate reads work; every other managed user gets 404.
- [ ] Run publishing and analytics smoke checks.
- [ ] Keep the exposed key revoked and remind the user to update downstream clients with the replacement.

## Task 10: Sync the same hotfix to dev after production

- [ ] Fetch and verify owned branch/worktree.
- [ ] Merge latest origin/dev into the same owned hotfix branch. On conflicts or unclear attribution, hard stop and ask the user.
- [ ] Rerun the full backend suite and exact commit/file audit.
- [ ] Push the owned branch and open a PR targeting dev.
- [ ] Complete GitHub CI, Railway PR Environment, deployed regression, and Codex browser Preview Acceptance on the exact head SHA.
- [ ] Merge only after every gate succeeds.
- [ ] Wait for development deployment, verify SHA, and repeat gating/isolation acceptance using only user-owned accounts.
- [ ] Report final SHAs, PR/deployment URLs, safely redacted provider resource identifiers, exact tests, and environment acceptance matrix.

## Completion criteria

The hotfix is complete only when comments and legacy DMs reconcile independently; every DM gate fails closed; shared stream and resource replacement remain idempotent; legacy DM routing resolves exactly one account; XChat is excluded; staging, production, and dev verification pass on exact SHAs; managed-user 404 isolation and owner/admin aggregation are proven; publishing/analytics smoke checks pass; and the previously exposed production API key is revoked/rotated.
