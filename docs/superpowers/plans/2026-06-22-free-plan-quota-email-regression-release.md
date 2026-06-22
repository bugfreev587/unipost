# Free Plan Quota Email Regression Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add regression coverage for the automatic Free plan quota email trigger, then promote the already-validated dev change through staging and production.

**Architecture:** Keep the regression at the handler boundary so it verifies the deployed publish flow invokes the quota email service after a scheduled post reserves quota. Use the existing fake pgx DB patterns in `api/internal/handler` and avoid production behavior changes.

**Tech Stack:** Go, sqlc-generated DB facade, chi/http handler tests, GitHub Actions, Vercel, Railway.

---

### Task 1: Handler Regression Test

**Files:**
- Modify: `api/internal/handler/social_posts_quota_test.go`
- Test: `api/internal/handler/social_posts_quota_test.go`

- [ ] **Step 1: Add a failing regression test**

Add `TestCreateScheduledPostTriggersFreePlanQuotaEmailEvaluation` that creates a scheduled LinkedIn post through `SocialPostHandler.Create` with `SetQuotaEmailService`.

- [ ] **Step 2: Run the targeted test to verify it is meaningful**

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run TestCreateScheduledPostTriggersFreePlanQuotaEmailEvaluation -count=1`

Expected for red proof: after temporarily disabling the scheduled trigger call, FAIL with a missing quota email evaluation.

- [ ] **Step 3: Restore the implementation and verify green**

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run TestCreateScheduledPostTriggersFreePlanQuotaEmailEvaluation -count=1`

Expected: PASS.

- [ ] **Step 4: Run full API validation**

Run: `GOCACHE=/tmp/unipost-go-build go test ./...`

Expected: PASS.

### Task 2: Environment Promotion

**Files:**
- No code files beyond Task 1.

- [ ] **Step 1: Merge branch to local dev and rerun API tests**

Run: `GOCACHE=/tmp/unipost-go-build go test ./...`

- [ ] **Step 2: Push dev and monitor GitHub Actions, Vercel dev, and Railway dev**

Expected: all checks/deployments complete successfully.

- [ ] **Step 3: Promote dev to staging**

Merge/push `dev` into `staging`, then monitor GitHub Actions, Vercel staging, and Railway staging.

- [ ] **Step 4: Verify staging**

Use staging domains only: `https://staging-api.unipost.dev`, `https://staging.unipost.dev`, and `https://staging-app.unipost.dev`.

- [ ] **Step 5: Promote staging to main**

Merge/push `staging` into `main`, then monitor GitHub Actions, Vercel production, and Railway production.

- [ ] **Step 6: Verify production**

Use production domains only: `https://api.unipost.dev`, `https://unipost.dev`, and `https://app.unipost.dev`. Repeat the quota-email acceptance path against production only after confirming the production deployment contains the promoted commit.
