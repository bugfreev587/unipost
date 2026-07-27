# PR #270 Review Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make PR #270's email delivery and TikTok resume invariants safe across deployed migrations and real PostgreSQL concurrency.

**Architecture:** Preserve migration 124 as deployed history and converge databases through migration 125. Keep terminal audit state immutable inside the existing SQL upsert, test the real delivery handler with controlled dependencies, and add a build-tagged PostgreSQL suite backed by an isolated CI service.

**Tech Stack:** Go 1.24, pgx v5, sqlc, Goose SQL migrations, PostgreSQL 16, GitHub Actions, Next.js/Playwright validation.

---

### Task 1: Migration upgrade correction

**Files:**
- Modify: `api/internal/db/platform_publishing_restrictions_migration_test.go`
- Modify: `api/internal/db/migrations/124_publishing_restriction_email_send_gate.sql`
- Create: `api/internal/db/migrations/125_publishing_restriction_failed_recipient_retryability.sql`

- [ ] Add a failing contract test requiring migration 125 to set `retryable = FALSE` for every `status = 'failed'` row and requiring both migrations to document irreversible Down semantics.
- [ ] Run `go test ./internal/db -run 'TestPublishingRestriction.*Migration' -count=1` and confirm failure because migration 125 is absent.
- [ ] Add migration 125 and the Down comments without changing migration 124 Up.
- [ ] Re-run the focused migration tests and confirm success.

### Task 2: Preserve sent email audits

**Files:**
- Modify: `api/internal/db/queries/email_send_attempts.sql`
- Modify generated: `api/internal/db/email_send_attempts.sql.go`
- Create: `api/internal/db/email_send_attempts_postgres_integration_test.go`

- [ ] Add a tagged PostgreSQL test that inserts and marks an audit sent, replays `CreateEmailSendAttemptAudit` with conflicting snapshots, and requires all terminal data to stay unchanged.
- [ ] Add a companion failed-audit replay assertion requiring the row to become pending with an incremented attempt count.
- [ ] Run the tagged focused test against the isolated local PostgreSQL database and confirm the sent case fails with the current upsert.
- [ ] Change only the conflict assignments needed to preserve an existing sent row while retaining failed/pending retry behavior; regenerate or exactly synchronize sqlc output.
- [ ] Re-run both focused cases and confirm success.

### Task 3: Drive the persisted TikTok token through the worker path

**Files:**
- Modify: `api/internal/handler/social_post_queue_test.go`

- [ ] Add a database fake, a restriction evaluator spy, and a TikTok adapter spy around the real `ProcessPostDeliveryJob` call.
- [ ] Require the persisted token to reach `OptResumePublishToken`, restriction evaluation/finalization to remain at zero, and TikTok init/upload counters to remain zero.
- [ ] Run the focused handler test. If it fails, make the smallest production correction in `api/internal/handler/social_post_queue.go`; otherwise retain the existing implementation and record that the new behavioral coverage passes.

### Task 4: PostgreSQL SKIP LOCKED and stale-terminal behavior

**Files:**
- Create: `api/internal/worker/publishing_restriction_email_postgres_integration_test.go`
- Modify: `.github/workflows/ci.yml`

- [ ] Build a tagged test fixture that requires `PUBLISHING_RESTRICTION_TEST_DATABASE_URL`, creates an isolated schema/database, and applies only the real schema/migrations needed by the store.
- [ ] Add a transaction-level test where tx1 locks the first recipient and tx2 runs the production candidate claim statement, requiring tx2 to skip the locked row.
- [ ] Add a store-level test that ages a `sending` row, invokes `ClaimPublishingRestrictionEmailRecipients`, and requires `failed`, `retryable=FALSE`, no returned work, and no later reclaim.
- [ ] Run the tagged suite against local PostgreSQL and confirm behavior.
- [ ] Add a required GitHub Actions job with an isolated PostgreSQL 16 service and run the tagged tests with the service URL. Missing URL must fail rather than skip.

### Task 5: Full verification and review

**Files:**
- Verify all changed files from Tasks 1-4.

- [ ] Run focused API tests for migrations, audit, handler resume, and worker PostgreSQL integration.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./...` from `api/` and inspect the complete output.
- [ ] Run relevant Dashboard source contracts, `npm run build`, and `npm run test:regression:dashboard` with zero skipped cases.
- [ ] Inspect `git diff --check`, branch ownership, exact changed files, and worktree cleanliness expectations.
- [ ] Request an independent code review against base `3a4c661c3e85c3c6e858cc8676793ca12191bb40`; resolve all Critical and Important findings with fresh tests.

### Task 6: Commit, stacked Draft PR, and remote monitoring

**Files:**
- Stage only the files listed in this plan.

- [ ] Re-run the required verification immediately before committing.
- [ ] Commit focused changes on `codex/pr270-review-hardening` and push only that branch.
- [ ] Create a Draft PR with base `codex/staging-tiktok-free-publishing-restriction` and explicitly state that it must be merged into PR #270's head before user re-review.
- [ ] Monitor every check for the exact pushed SHA to a terminal success state; on any non-success, stop and report the required workflow/job/log evidence.
- [ ] Report unique commits/files and confirm the worktree is clean.
