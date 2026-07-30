# X Account Read Review Follow-ups Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the confirmed recovery-ownership, malformed-provider-response, tenant-scoping, and cursor-initialization gaps found after PR #310.

**Architecture:** Keep the shared Credits exposure table and settlement primitive, but give the legacy Inbox recovery worker explicit ownership of only `inbox_backfill` rows. Treat malformed required X post fields as a typed invalid upstream page, thread Workspace identity through token resolution, and make service construction return cursor configuration errors instead of retaining a nil codec.

**Tech Stack:** Go 1.x, pgx/PostgreSQL, sqlc queries, `httptest`, existing UniPost X Credits and account-read packages.

---

### Task 1: Scope Inbox exposure recovery

**Files:**
- Modify: `api/internal/xcredits/exposure_postgres.go`
- Test: `api/internal/xcredits/exposure_postgres_test.go`

- [ ] Add a failing test that reads the recovery SQL and requires `purpose = 'inbox_backfill'`, while preserving the terminal settlement mutation behavior used to complete an account-read receipt after another worker finalized its exposure.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./internal/xcredits -run 'TestPostgresExposureRecoveryScopesInboxPurpose|TestFinalizedExposureRunsMatchingMutation' -count=1` and verify the purpose assertion fails.
- [ ] Add the explicit purpose predicate to `ReconcilePendingExposures`; do not split the unified ledger or change exposure idempotency.
- [ ] Re-run the focused tests and verify they pass.

### Task 2: Fail malformed authored-post pages atomically

**Files:**
- Modify: `api/internal/platform/twitter.go`
- Test: `api/internal/platform/twitter_test.go`
- Test: `api/internal/xaccountreads/service_test.go`

- [ ] Add table-driven failing adapter tests for invalid `created_at`, missing post ID, and missing `conversation_id`; each must return a sanitized `TwitterAccountReadError{Class: "invalid_response"}` and no page/cursor.
- [ ] Add or preserve a service-level assertion that a typed invalid provider response releases the reservation and never finalizes Credits.
- [ ] Run the focused platform and service tests and verify the malformed-page cases fail under the current silent `continue` behavior.
- [ ] Replace the silent skips for required-field failures with the existing typed invalid-response error. Keep locally filtered reposts/replies in `ScannedCount`.
- [ ] Re-run the focused tests and verify they pass.

### Task 3: Re-scope token resolution by Workspace

**Files:**
- Modify: `api/internal/xaccountreads/service.go`
- Modify: `api/internal/xaccountreads/recovery.go`
- Modify: `api/internal/handler/x_account_reads.go`
- Modify: `api/internal/handler/x_account_reads_test.go`
- Modify: `api/internal/xaccountreads/service_test.go`

- [ ] Change the fake resolver test contract first to require `(workspaceID, accountID)` and assert both live reads and recovery pass the receipt Workspace.
- [ ] Add a handler test proving token resolution calls `GetSocialAccountByIDAndWorkspace` and cannot resolve an account from another Workspace.
- [ ] Run the focused handler and account-read tests and verify they fail to compile or fail the new scoping assertion.
- [ ] Change `TokenResolver` to `ResolveAccountReadToken(context.Context, string, string)`; pass both receipt fields from live and recovery paths; use `GetSocialAccountByIDAndWorkspace` in the handler.
- [ ] Preserve refresh behavior after the scoped account load, then re-run focused tests.

### Task 4: Make cursor construction fail startup

**Files:**
- Modify: `api/internal/xaccountreads/service.go`
- Modify: `api/internal/xaccountreads/service_test.go`
- Modify: `api/cmd/api/main.go`
- Test: `api/internal/xaccountreads/service_test.go`

- [ ] Add a failing constructor test asserting a short cursor secret is returned as an error.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./internal/xaccountreads -run TestServiceConstructionRejectsInvalidCursorSecret -count=1` and verify failure.
- [ ] Change `NewServiceWithSecrets` to return `(*Service, error)`, update the compatibility constructor and call sites, and make `main` log and exit on construction error.
- [ ] Re-run package tests and verify valid-key callers still construct normally.

### Task 5: Validate, audit, and deliver

**Files:**
- Verify all modified backend files and this plan.

- [ ] Run `gofmt` on changed Go files.
- [ ] From `api/`, run `GOCACHE=/tmp/unipost-go-build go test ./...`.
- [ ] Review `git diff --check`, `git status`, unique commits, and changed files; confirm no unrelated content.
- [ ] Commit the focused change, update `origin/dev` as explicitly requested, and monitor all triggered CI and dev deployments to completion.
- [ ] Exercise the X profile/posts feature against the real development API with the Credits flag both off and on where credentials and flag access permit; otherwise report the exact externally blocked acceptance case without claiming completion.
- [ ] After dev acceptance passes on the deployed SHA, create a `dev` to `staging` pull request and leave it unmerged for user review.
