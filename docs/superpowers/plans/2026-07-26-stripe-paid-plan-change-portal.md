# Stripe Paid Plan Change Portal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task.

**Goal:** Route ordinary paid-plan changes through a Stripe Customer Portal `subscription_update_confirm` flow so one existing subscription is updated with correct proration, while reserving Checkout for Free-to-paid activation and preserving managed-trial plan changes.

**Architecture:** Add a mode-aware plan-change service in `api/internal/billing` that validates the local customer/subscription binding against Stripe before creating a deep-linked Portal Session. Add an owner-only API endpoint that dispatches paid workspaces to this service and prevents paid workspaces from entering Subscription Checkout. Update Settings Billing to select Checkout, managed-trial mutation, or Portal plan change from authoritative billing state; Stripe webhooks remain the only entitlement authority.

**Tech Stack:** Go 1.24, stripe-go v82.5.1, chi, sqlc/Postgres, Next.js/React/TypeScript, Node source-contract tests, GitHub Actions, Railway, Vercel, Stripe sandbox.

---

## Preconditions and non-goals

- Work only in `/Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-admin-configurable-free-trial` on `dev-admin-configurable-free-trial`; verify both before every write, test, commit, push, merge, deployment, or promotion.
- The previously completed ConfirmModal and mode-aware webhook resolver changes remain prerequisites and are not reimplemented here.
- Do not create or mutate a live Stripe Portal configuration and do not set production environment variables during staging work.
- Before PR #265 can merge into `main`, live configuration provisioning and `STRIPE_PLAN_CHANGE_PORTAL_CONFIGURATION_ID` are mandatory release gates.
- Pending-downgrade display in UniPost and production duplicate-subscription auditing are out of scope.
- Preview Acceptance is skipped only because the user explicitly authorized it. Local CI, GitHub CI, staging deploy checks, full staging regression, and browser/Stripe acceptance remain required.

### Task 1: Load dedicated per-mode Portal configuration IDs

**Files:**

- Modify: `api/internal/billing/manager.go`
- Test: `api/internal/billing/manager_test.go`

**Step 1: Write failing manager tests**

Extend `TestNewManagerLoadsModeSpecificTrialPortalConfigurations` or add `TestNewManagerLoadsModeSpecificPlanChangePortalConfigurations` with:

```go
t.Setenv("STRIPE_PLAN_CHANGE_PORTAL_CONFIGURATION_ID", "bpc_live_plan_change")
t.Setenv("STRIPE_SANDBOX_PLAN_CHANGE_PORTAL_CONFIGURATION_ID", "bpc_sandbox_plan_change")

if got := manager.Live.PlanChangePortalConfigurationID(); got != "bpc_live_plan_change" {
    t.Fatalf("live PlanChangePortalConfigurationID() = %q", got)
}
if got := manager.Sandbox.PlanChangePortalConfigurationID(); got != "bpc_sandbox_plan_change" {
    t.Fatalf("sandbox PlanChangePortalConfigurationID() = %q", got)
}
```

Add a nil receiver assertion returning `""`.

**Step 2: Run the focused test and confirm RED**

Run from `api/`:

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/billing -run 'TestNewManagerLoadsModeSpecificPlanChangePortalConfigurations|TestNilModePlanChangePortalConfigurationIDIsEmpty'
```

Expected: compile failure because `PlanChangePortalConfigurationID` does not exist.

**Step 3: Implement the smallest manager change**

Add `planChangePortalConfigurationID string` to `Mode`, expose:

```go
func (m *Mode) PlanChangePortalConfigurationID() string {
    if m == nil {
        return ""
    }
    return m.planChangePortalConfigurationID
}
```

Extend `newMode` and both `NewManager` calls so live reads `STRIPE_PLAN_CHANGE_PORTAL_CONFIGURATION_ID` and sandbox reads `STRIPE_SANDBOX_PLAN_CHANGE_PORTAL_CONFIGURATION_ID`. Keep the existing trial-safe Portal configuration independent.

**Step 4: Run focused tests and confirm GREEN**

Run the command from Step 2 and then:

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/billing
```

Expected: all `internal/billing` tests pass.

**Step 5: Commit**

```bash
git add api/internal/billing/manager.go api/internal/billing/manager_test.go
git commit -m "feat: load plan change portal configurations"
```

### Task 2: Build a fail-closed Stripe plan-change service

**Files:**

- Create: `api/internal/billing/plan_change.go`
- Create: `api/internal/billing/plan_change_test.go`

**Step 1: Write failing service tests with a recording Stripe client**

Define a package-private client boundary matching stripe-go:

```go
type planChangeStripeClient interface {
    GetSubscription(string, *stripe.SubscriptionParams) (*stripe.Subscription, error)
    CreatePortalSession(*stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error)
}
```

Use an adapter for `mode.Client.Subscriptions.Get` and `mode.Client.BillingPortalSessions.New`, and inject a recording fake in tests. Cover:

1. Basic-to-Growth creates exactly one Portal Session and zero subscription/Checkout mutations.
2. Parameters include the mode-specific configuration ID, expected customer, `subscription_update_confirm`, the exact existing subscription ID/item ID, target Growth price, quantity 1, and redirect-after-completion URL.
3. Missing Portal configuration fails before a Stripe call.
4. Nil/incomplete retrieval, mismatched subscription ID, mismatched customer, multiple/no items, missing item ID, unknown current price, unknown target price, same current/target plan, and Stripe retrieval/session errors all fail closed.
5. `workspace_id`, `mode`, and `unipost_environment` metadata must match when present; absent legacy metadata is tolerated, conflicting metadata is rejected.
6. The returned Portal object must contain both ID and HTTPS URL.

Construct the Stripe subscription test fixture with one item:

```go
&stripe.Subscription{
    ID: "sub_sandbox_1",
    Customer: &stripe.Customer{ID: "cus_sandbox_1"},
    Metadata: map[string]string{
        "workspace_id": "ws_1",
        "mode": "sandbox",
        "unipost_environment": "staging",
    },
    Items: &stripe.SubscriptionItemList{Data: []*stripe.SubscriptionItem{{
        ID: "si_1",
        Price: &stripe.Price{ID: "price_sandbox_basic"},
        Quantity: 1,
    }}},
}
```

**Step 2: Run focused tests and confirm RED**

From `api/`:

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/billing -run PlanChange
```

Expected: compile failure because the service and request/result types do not exist.

**Step 3: Implement request, result, errors, adapter, and validation**

Use explicit internal IDs only:

```go
type PlanChangeRequest struct {
    StripeMode       string
    WorkspaceID      string
    CustomerID       string
    SubscriptionID   string
    CurrentPlanID    string
    TargetPlanID     string
    ReturnURL        string
}

type PlanChangeResult struct {
    URL string
}
```

Define sentinels for invalid input, unavailable configuration, binding conflict, and upstream Stripe failure so the handler can map them without exposing Stripe internals.

Validation order must be mutation-safe:

1. Validate request strings and an HTTPS return URL.
2. Resolve exact mode with `Manager.ByName`.
3. Require `Mode.PlanChangePortalConfigurationID()` and target/current prices from that same mode.
4. Retrieve the exact local subscription ID in that mode.
5. Validate ID, customer, optional metadata, exactly one item, item ID, item current price, and quantity 1.
6. Create one Portal Session with:

```go
FlowData: &stripe.BillingPortalSessionFlowDataParams{
    Type: stripe.String(string(stripe.BillingPortalSessionFlowTypeSubscriptionUpdateConfirm)),
    AfterCompletion: &stripe.BillingPortalSessionFlowDataAfterCompletionParams{
        Type: stripe.String(string(stripe.BillingPortalSessionFlowAfterCompletionTypeRedirect)),
        Redirect: &stripe.BillingPortalSessionFlowDataAfterCompletionRedirectParams{
            ReturnURL: stripe.String(req.ReturnURL),
        },
    },
    SubscriptionUpdateConfirm: &stripe.BillingPortalSessionFlowDataSubscriptionUpdateConfirmParams{
        Subscription: stripe.String(req.SubscriptionID),
        Items: []*stripe.BillingPortalSessionFlowDataSubscriptionUpdateConfirmItemParams{{
            ID: stripe.String(item.ID), Price: stripe.String(targetPriceID), Quantity: stripe.Int64(1),
        }},
    },
},
```

Set top-level `Configuration`, `Customer`, and `ReturnURL`. Do not accept any Stripe object ID from browser input. Return a wrapped upstream error and never mutate the local plan.

**Step 4: Run focused tests and confirm GREEN**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/billing -run PlanChange
GOCACHE=/tmp/unipost-go-build go test ./internal/billing
```

Expected: all pass.

**Step 5: Commit**

```bash
git add api/internal/billing/plan_change.go api/internal/billing/plan_change_test.go
git commit -m "feat: create verified stripe plan change sessions"
```

### Task 3: Add owner-only endpoint and make paid Checkout impossible

**Files:**

- Modify: `api/internal/handler/billing.go`
- Test: `api/internal/handler/billing_test.go`
- Modify: `api/cmd/api/main.go`

**Step 1: Add failing handler tests**

Introduce a narrow handler dependency:

```go
type billingPlanChangeService interface {
    CreateSession(context.Context, billing.PlanChangeRequest) (billing.PlanChangeResult, error)
}
```

Add tests for `CreatePlanChangeSession`:

- Missing workspace or malformed/empty `plan_id` returns 401/422 without service invocation.
- Free local plan returns `CHECKOUT_REQUIRED` without service invocation.
- Scheduled/active managed trial returns `TRIAL_PLAN_CHANGE_REQUIRED` without service invocation.
- Ordinary paid Basic calls the service with DB-derived workspace, customer, subscription, current plan, authenticated user's Stripe mode, target internal plan, and Settings Billing return URL; response is `{data:{url:"https://billing.stripe.test/..."}}`.
- Missing local customer/subscription or invalid target plan returns validation/conflict before service invocation.
- Service errors map to stable `BILLING_PLAN_CHANGE_UNAVAILABLE` or `BILLING_PLAN_CHANGE_CONFLICT` responses without leaking Stripe errors.

Add tests for `CreateCheckout` proving a paid local plan returns `PAID_PLAN_CHANGE_REQUIRED` before customer or Checkout creation. Keep free pending-activation trial Checkout behavior green.

Change `BillingHandler.queries` to a package-private `billingQueries` interface containing the five methods already used by this file: `GetWorkspace`, `GetSubscriptionByWorkspace`, `GetPlan`, `GetUser`, and `ListPlans`. `*db.Queries` satisfies it unchanged; a focused fake implements it in `billing_test.go`, allowing full handler requests without a real database.

**Step 2: Run focused tests and confirm RED**

From `api/`:

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'PlanChangeSession|PaidCheckout'
```

Expected: compile/test failure because the endpoint, service setter, and paid guard do not exist.

**Step 3: Implement handler dispatch and route wiring**

Add `planChanges billingPlanChangeService` and `SetPlanChangeService`. In `main.go`, construct `billing.NewPlanChangeService(stripeMgr, runtimeenv.Current())` and attach it to `BillingHandler`.

Add owner-only:

```go
r.With(auth.RequireRole(auth.RoleOwner)).Post("/v1/billing/plan-change-session", billingHandler.CreatePlanChangeSession)
```

In `CreatePlanChangeSession`:

1. Derive workspace and user from auth context.
2. Accept only `{ "plan_id": "basic|growth|team|api" }` and verify the canonical plan exists.
3. Load the local subscription.
4. Return `CHECKOUT_REQUIRED` for Free.
5. If current trial projection is scheduled/active, return `TRIAL_PLAN_CHANGE_REQUIRED`.
6. Require local customer/subscription IDs and a paid/current state.
7. Resolve mode through `h.stripe.For(ctx, userID)` and invoke the service with server-derived Stripe IDs.
8. Return `{ "url": result.URL }` only; do not update the local plan.

In `CreateCheckout`, load the local subscription before any Stripe Customer or Checkout call. Preserve free managed-trial activation; for a non-Free plan return `TRIAL_PLAN_CHANGE_REQUIRED` when applicable, otherwise `PAID_PLAN_CHANGE_REQUIRED`. There is no paid fallback to Checkout.

**Step 4: Run focused and package tests**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'PlanChangeSession|PaidCheckout|Trial'
GOCACHE=/tmp/unipost-go-build go test ./internal/handler
```

Expected: all pass.

**Step 5: Commit**

```bash
git add api/internal/handler/billing.go api/internal/handler/billing_test.go api/cmd/api/main.go
git commit -m "feat: route paid plan changes through stripe portal"
```

### Task 4: Dispatch Settings Billing by authoritative state

**Files:**

- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/app/(dashboard)/settings/billing/page.tsx`
- Modify: `dashboard/tests/free-trial-contract-source.test.mjs`

**Step 1: Write failing source-contract assertions**

Require:

- `createPlanChangeSession(token, planId)` posts only `{plan_id}` to `/v1/billing/plan-change-session` and returns `{url}`.
- `handleUpgrade` keeps the managed scheduled/active trial branch first and calls `changeTrialPlan` after its existing confirmation.
- Free billing calls `createCheckout` and redirects to `checkout_url`.
- Ordinary paid billing calls `createPlanChangeSession` and redirects to `url`.
- Settings Billing contains no optimistic `setBilling` plan mutation after Portal redirect.
- Pricing pending-activation/checkout-pending offers still use Checkout; ordinary Pricing actions continue through Settings Billing's dispatcher.

Example assertions:

```js
assert.match(apiSource, /createPlanChangeSession[\s\S]*\/v1\/billing\/plan-change-session[\s\S]*plan_id/);
assert.match(settingsBillingSource, /billing\?\.plan === "free"[\s\S]*createCheckout/);
assert.match(settingsBillingSource, /createPlanChangeSession\(token, planId\)[\s\S]*window\.location\.href = res\.data\.url/);
```

**Step 2: Run the focused contract test and confirm RED**

From `dashboard/`:

```bash
node --test tests/free-trial-contract-source.test.mjs
```

Expected: fails because paid dispatcher/API helper are missing.

**Step 3: Implement API helper and dispatcher**

Add:

```ts
export async function createPlanChangeSession(
  token: string,
  planId: string,
): Promise<ApiResponse<{ url: string }>> {
  return request(`/v1/billing/plan-change-session`, token, {
    method: "POST",
    body: JSON.stringify({ plan_id: planId }),
  });
}
```

In `handleUpgrade`, preserve the managed-trial branch. Then use `billing?.plan === "free"` for Checkout; otherwise call `createPlanChangeSession`. Both hosted flows navigate with `window.location.href`; neither updates local entitlements before a webhook and subsequent billing reload.

**Step 4: Run focused tests and build**

```bash
node --test tests/free-trial-contract-source.test.mjs
npm run build
```

Expected: contract test and production build pass.

**Step 5: Commit**

```bash
git add dashboard/src/lib/api.ts 'dashboard/src/app/(dashboard)/settings/billing/page.tsx' dashboard/tests/free-trial-contract-source.test.mjs
git commit -m "feat: dispatch paid upgrades to stripe portal"
```

### Task 5: Verify all local behavior and review the exact diff

**Files:**

- Verify only; modify tests/implementation only when a real failure identifies a defect.

**Step 1: Run complete backend CI-equivalent tests**

From `api/`:

```bash
GOCACHE=/tmp/unipost-go-build go test ./...
```

Expected: all packages pass with no skip/error/timeout.

**Step 2: Run Dashboard contracts, build, and full local regression**

From `dashboard/`:

```bash
node --test tests/free-trial-contract-source.test.mjs
npm run build
npm run test:regression:dashboard
```

Expected: all pass. Any failed, timed-out, canceled, skipped, or missing test result is a hard stop.

**Step 3: Run formatting and inspect the promotion content**

```bash
gofmt -w api/internal/billing/manager.go api/internal/billing/manager_test.go api/internal/billing/plan_change.go api/internal/billing/plan_change_test.go api/internal/handler/billing.go api/internal/handler/billing_test.go api/cmd/api/main.go
git diff --check
git status --short
git log --oneline origin/staging..HEAD
git diff --name-status origin/staging...HEAD
```

Re-run affected tests after `gofmt`. Confirm every unique commit/file belongs to this trial/plan-change line; any unrelated or unidentified content blocks delivery.

**Step 4: Request a code review and resolve findings**

Use `superpowers:requesting-code-review` on the complete local diff. Apply only verified findings using `superpowers:receiving-code-review`, then repeat the complete checks on the replacement SHA.

**Step 5: Commit any final test-only or review fix**

Use a focused message and stage the concrete files reported by `git status --short`, for example when only the two test files changed:

```bash
git add api/internal/billing/plan_change_test.go api/internal/handler/billing_test.go
git commit -m "test: cover stripe portal plan changes"
```

### Task 6: Provision sandbox configuration and deliver to staging

**Files:**

- No repository file changes unless provisioning documentation is found to be missing during review.
- External scope: Stripe sandbox only; Railway staging only; GitHub PRs #267/#268 lineage and PR #265.

**Step 1: Read and verify the current sandbox catalog without exposing secrets**

Use Railway staging variables to obtain sandbox API key and `STRIPE_SANDBOX_PRICE_ID_{API,BASIC,GROWTH,TEAM}` in-process without printing the key. Retrieve all four Prices from Stripe, confirm each is recurring/active and collect its Product ID. Reject duplicate, missing, inactive, live-mode, or mismatched objects.

**Step 2: Create the dedicated sandbox Portal configuration**

Create one sandbox configuration named `UniPost Plan Changes (Sandbox)` whose exact allowlist contains only the four retrieved product/price pairs. Configure:

- `features.subscription_update.enabled=true`
- `features.subscription_update.default_allowed_updates[]=price`
- `features.subscription_update.proration_behavior=always_invoice`
- scheduled updates at period end for `decreasing_item_amount`
- quantity changes and promotion codes disabled
- no unrelated cancellation capability in this dedicated flow

Immediately retrieve it and compare the complete price set and billing rules to the mode-specific catalog. A mismatch is a hard stop; do not set the environment variable.

**Step 3: Push the owned branch and open a focused PR to staging**

Before push, verify path/branch and run the promotion content audit. Push only `dev-admin-configurable-free-trial`. Create or update the focused PR to `staging`; Preview checks remain explicitly skipped, but all visible GitHub CI checks must pass on the exact head SHA.

**Step 4: Merge only after CI and audit pass, then set the staging config ID**

After the staging PR merge succeeds, set only `STRIPE_SANDBOX_PLAN_CHANGE_PORTAL_CONFIGURATION_ID` in Railway staging. Monitor every triggered Railway/Vercel deployment until success and confirm deployments reference the exact new `origin/staging` SHA.

If any check/deployment is failed, canceled, timed out, skipped, absent, or for a different SHA, stop and report the required evidence before any further promotion.

**Step 5: Perform one-time sandbox cleanup**

Retrieve the known current Growth subscription and old Basic subscription for the staging admin. Reconfirm same customer, current database binding, current statuses, and mode. If the old Basic subscription remains active, cancel only that exact sandbox subscription with proration/invoice finalization according to the user authorization. Re-fetch Stripe and staging billing data and require exactly one active paid subscription. If it is already canceled, treat cleanup as idempotently complete.

**Step 6: Verify the deployed paid plan-change flow**

Using the staging admin account:

1. Open Settings Billing.
2. Choose a paid target plan different from the current effective plan.
3. Confirm the browser goes to Stripe Portal `subscription_update_confirm`, not Checkout.
4. Confirm the hosted page shows the prorated invoice impact for an upgrade, or period-end scheduling for a downgrade.
5. Complete the change and return to Settings Billing.
6. Wait for the verified webhook projection and confirm target plan/quotas.
7. Compare Stripe subscription IDs before/after and require the same ID.
8. Require exactly one active paid subscription.

Do not perform a live Stripe change.

**Step 7: Run full staging regression on the final SHA**

From `dashboard/` with the repository's staging regression environment:

```bash
npm run test:regression:dashboard
```

Expected: complete 60-test result passes. The earlier jsDelivr incident was authorized only as evidence for that run; it does not waive this final replacement-SHA run.

**Step 8: Update PR #265 and stop for user review**

Confirm PR #265 is `staging` to `main`, its head equals the fully validated staging SHA, and all its triggered checks finish successfully. Add/restate the release blocker in its description or review note:

> Before merge: create and verify the live plan-change Portal configuration, then set `STRIPE_PLAN_CHANGE_PORTAL_CONFIGURATION_ID` in production. Do not merge while either is missing.

Do not merge PR #265. Report the exact staging SHA, CI/deployment/regression results, sandbox configuration ID (not secret), same-subscription verification, cleanup result, PR URL, and the outstanding live-configuration gate to the user.

## Final self-review checklist

- No placeholder markers remain in production code or test assertions.
- Browser requests contain only internal plan IDs.
- Free, ordinary paid, and managed-trial dispatches are mutually exclusive in both API and Dashboard.
- Portal creation cannot fall back to Checkout.
- Portal configuration, subscription, customer, item, price, workspace, mode, and runtime environment are all verified before creating a Session.
- Redirect does not mutate local entitlements; webhook remains authoritative.
- Staging provisioning touches sandbox only.
- PR #265 stays open until separately authorized live provisioning is complete.
