# Admin Trial Review Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Correct the managed-trial cancellation projection and expose accurate renewal and price data in admin and user trial timelines before PR #265 is merged.

**Architecture:** Subscription deletion will reconcile the trial ledger directly and perform one final paid-to-free subscription mutation, without depending on Stripe price resolution or first projecting a canceled paid subscription. Managed metadata will fail closed before the legacy immediate-cancellation path. Schedule reconciliation will retain the metadata plan under a non-shadowing name, while admin and history projections will explicitly carry post-trial price and renewal-cancellation state.

**Tech Stack:** Go 1.24, pgx/sqlc, Stripe Go SDK, Next.js/TypeScript, Node contract tests.

---

### Task 1: Make subscription deletion a single terminal mutation

**Files:**
- Modify: `api/internal/handler/stripe_webhook_test.go`
- Modify: `api/internal/handler/stripe_webhook.go`

- [ ] **Step 1: Write failing deletion tests**

Add handler tests that send `customer.subscription.deleted` with a retired/unmapped Stripe price and assert HTTP 200 plus one `CancelSubscription` call, and a managed deletion test that installs a recording hold reconciler and asserts exactly one paid-to-free `ApplyPlanChange` call.

- [ ] **Step 2: Verify the tests fail**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestSubscriptionDeletedWithUnmappedPriceStillDowngrades|TestManagedSubscriptionDeletedAppliesSingleFreePlanMutation' -count=1`

Expected: FAIL because deletion currently calls `projectStripeSubscription`, which rejects the unmapped price and can perform a paid-plan mutation before the free downgrade.

- [ ] **Step 3: Implement deletion-specific trial reconciliation**

Replace the generic projector call in `handleSubscriptionDeleted` with a helper that calls `ReconcileSubscription` using the local or metadata plan ID, maps `ErrWebhookNotApplicable` to the existing acknowledgement behavior, and never writes the subscription projection. Preserve the existing final `applyPlanChangeMutation(currentPlan, freePlan, CancelSubscription)` as the only subscription/quota mutation.

- [ ] **Step 4: Verify the deletion tests pass**

Run the command from Step 2.

Expected: PASS.

### Task 2: Fail closed for managed trial metadata

**Files:**
- Modify: `api/internal/handler/stripe_webhook_test.go`
- Modify: `api/internal/handler/stripe_webhook.go`

- [ ] **Step 1: Write the failing guard test**

Add a `customer.subscription.updated` test with `trialing + cancel_at_period_end`, valid `trial_grant_id` metadata, and no configured trial reconciler. Assert HTTP 500 and no downgrade/cancel mutation.

- [ ] **Step 2: Verify the test fails**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run TestManagedTrialMetadataWithoutReconcilerFailsClosed -count=1`

Expected: FAIL because the zero-valued reconciliation result currently enters `cancelLegacyTrialImmediately`.

- [ ] **Step 3: Implement the metadata guard**

Before legacy immediate cancellation, return a configuration/state error when `trial_grant_id` metadata is present but reconciliation did not identify a managed grant. Keep metadata-free legacy trials on the existing immediate-downgrade path.

- [ ] **Step 4: Verify managed and legacy behavior**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestManagedTrialMetadataWithoutReconcilerFailsClosed|TestLegacyTrialCancelAtPeriodEndStillDowngradesImmediately|TestManagedTrialCancelAtPeriodEndRetainsPlanAndAccess' -count=1`

Expected: PASS.

### Task 3: Preserve the superseding plan on early schedule release

**Files:**
- Modify: `api/internal/trials/service_test.go`
- Modify: `api/internal/trials/service.go`

- [ ] **Step 1: Strengthen the early-release test**

Extend `TestReconcileEarlyReleasedScheduleIsSuperseded` to assert `result.Grant.SupersededByPlanID == "growth"`.

- [ ] **Step 2: Verify the assertion fails**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/trials -run TestReconcileEarlyReleasedScheduleIsSuperseded -count=1`

Expected: FAIL with an empty superseded plan.

- [ ] **Step 3: Remove the constant shadowing**

Rename the local metadata value to `snapshotPlanID` and reuse it in both updated and released schedule branches, so the metadata map is always indexed by the `metadataPlanID` constant exactly once.

- [ ] **Step 4: Verify the schedule test passes**

Run the command from Step 2.

Expected: PASS.

### Task 4: Supply accurate trial timeline fields

**Files:**
- Modify: `api/internal/trials/model.go`
- Modify: `api/internal/trials/service.go`
- Modify: `api/internal/trials/service_test.go`
- Modify: `api/internal/handler/admin.go`
- Modify: `api/internal/handler/admin_test.go`
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/lib/free-trial-contract-source.test.mjs`
- Test: `dashboard/src/lib/trial-format.test.mjs`

- [ ] **Step 1: Write failing API projection tests**

Require history JSON to contain `post_trial_price_cents` and `cancel_at_period_end`, require `ListTrialHistory` to resolve each plan price and derive cancellation from `grant.CanceledAt`, and require the admin embedded trial JSON contract to expose the same fields.

- [ ] **Step 2: Verify API projection tests fail**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/trials ./internal/handler -run 'TestHistoryProjection|TestListTrialHistory|TestAdminBillingTrial' -count=1`

Expected: FAIL because the fields are absent.

- [ ] **Step 3: Implement backend projections**

Add price and cancellation fields to `HistoryProjection` and `adminBillingTrialSummary`. Resolve history prices through the existing `GetPlanPrice` store method with a per-request plan cache. Extend `adminBillingSQL` to join the trial plan price and compute cancellation as subscription cancellation or a non-null grant `canceled_at`, then scan both values into the nested trial summary.

- [ ] **Step 4: Verify backend projection tests pass**

Run the command from Step 2.

Expected: PASS.

- [ ] **Step 5: Write failing dashboard contract assertions**

Require `WorkspaceTrialHistoryEntry` and `AdminWorkspaceTrial` to declare numeric `post_trial_price_cents` and boolean `cancel_at_period_end`, and verify `formatWorkspaceTrial` renders `Access ends` for cancellation and includes the monthly price for renewal.

- [ ] **Step 6: Verify dashboard assertions fail**

Run: `cd dashboard && node --test src/lib/free-trial-contract-source.test.mjs src/lib/trial-format.test.mjs`

Expected: FAIL until the TypeScript contracts expose the fields.

- [ ] **Step 7: Update the dashboard contracts**

Add the required fields to both interfaces; retain the formatter's existing optional input compatibility for mutation payloads.

- [ ] **Step 8: Verify dashboard assertions pass**

Run the command from Step 6.

Expected: PASS.

### Task 5: Full verification and staging PR

**Files:**
- Verify all files changed above.

- [ ] **Step 1: Run backend validation**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./...`

Expected: PASS.

- [ ] **Step 2: Run dashboard validation**

Run: `cd dashboard && npm run build && npm run test:regression:dashboard`

Expected: build PASS and regression PASS.

- [ ] **Step 3: Audit and commit**

Verify the exclusive worktree and branch, inspect `git diff --check`, list every changed file and commit only the reviewed fix scope.

- [ ] **Step 4: Push and open a Draft PR to staging**

Push `dev-admin-configurable-free-trial`, create a Draft PR targeting `staging`, and audit its unique commits and files.

- [ ] **Step 5: Complete Preview Acceptance**

Wait for GitHub CI, Railway PR Environment, Vercel Preview, deployed regression, and browser acceptance on the exact head SHA. Any failed, skipped, canceled, timed-out, or missing required result is a hard stop.

- [ ] **Step 6: Merge to staging and revalidate**

After every gate passes, mark ready, re-audit, merge, wait for staging deployments, run deployed regression, and verify the managed-trial cancellation and billing timeline flows. Leave PR #265 open for user review and never merge it to `main`.
