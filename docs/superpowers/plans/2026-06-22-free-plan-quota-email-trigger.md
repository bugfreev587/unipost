# Free Plan Quota Email Trigger Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Automatically send Loops transactional emails when Free plan workspaces reach quota reminder thresholds.

**Architecture:** Add a small backend service that evaluates effective quota usage, records threshold sends in a DB ledger, and sends the already-created Loops transactional template with deterministic idempotency. Wire the service into publish success and Free plan quota rejection paths; no feature flag per product decision.

**Tech Stack:** Go, pgx/sqlc, PostgreSQL migrations, existing `internal/loops` client, existing quota checker.

---

### Task 1: Quota Email Service

**Files:**
- Create: `api/internal/quotaemail/service.go`
- Test: `api/internal/quotaemail/service_test.go`

- [ ] Write failing tests for highest-unsent-threshold selection, Free-plan eligibility, duplicate suppression, provider failure marking, and 100% blocked copy variables.
- [ ] Run `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/quotaemail` and verify the package fails because it does not exist.
- [ ] Implement the minimal service interfaces, threshold selection, data variables, and provider/ledger calls.
- [ ] Run `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/quotaemail` and verify it passes.

### Task 2: Ledger Persistence

**Files:**
- Create: `api/internal/db/migrations/089_free_plan_quota_email_reminders.sql`
- Create: `api/internal/db/queries/free_plan_quota_email_reminders.sql`
- Generate/modify: `api/internal/db/free_plan_quota_email_reminders.sql.go`

- [ ] Add a migration for `free_plan_quota_email_reminders` keyed by `(workspace_id, period, threshold_percent)`, with `pending`, `sent`, and `failed` status values.
- [ ] Add sqlc queries to list sent/pending thresholds, create a pending row idempotently, and mark rows sent or failed.
- [ ] Run `cd api && sqlc generate` and verify generated DB code compiles.
- [ ] Re-run `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/quotaemail`.

### Task 3: API Wiring

**Files:**
- Modify: `api/cmd/api/main.go`
- Modify: `api/internal/handler/social_posts.go`
- Modify: `api/internal/handler/social_posts_bulk.go`
- Modify: `api/internal/handler/social_posts_drafts.go`
- Modify: `api/internal/handler/social_post_queue.go`

- [ ] Add `LOOPS_FREE_PLAN_QUOTA_REMINDER_TRANSACTIONAL_ID` wiring from main into the new service.
- [ ] Attach the service to `SocialPostHandler`.
- [ ] Call it after successful quota increments.
- [ ] Call it when a Free plan publish is rejected for quota, so the 100% blocked email can be sent even if usage was already at limit.
- [ ] Keep failures best-effort: log them, never fail publishing because email delivery failed.

### Task 4: Validation and Dev Release

**Files:**
- All changed files above.

- [ ] Run `cd api && GOCACHE=/tmp/unipost-go-build go test ./...`.
- [ ] Merge task branch into local `dev` after updating local `dev` from `origin/dev`.
- [ ] Re-run `cd api && GOCACHE=/tmp/unipost-go-build go test ./...` on local `dev`.
- [ ] Push local `dev` to `origin/dev`.
- [ ] Monitor GitHub Actions, Vercel dev, and Railway dev deployments.
- [ ] Verify `https://dev-api.unipost.dev/health` and perform a dev-environment acceptance check for the quota email trigger path.
