# Staging Trial Dialog and Sandbox Upgrade Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Admin Billing trial browser confirmations with UniPost modals and make verified sandbox Stripe prices project the paid plan after Checkout.

**Architecture:** Keep UI mutations unchanged behind a local confirmation-state boundary that renders the shared `ConfirmModal`. Add a reverse price lookup to `billing.Mode`, then centralize webhook plan resolution around `Manager.ByName(snapshot.StripeMode)` with the live database price lookup retained only as a legacy fallback.

**Tech Stack:** Go 1.24, stripe-go v82, pgx/sqlc, Next.js 16, React 19, TypeScript, Node test runner, Playwright.

---

### Task 1: Admin Billing Trial Confirmations

**Files:**
- Modify: `dashboard/tests/free-trial-contract-source.test.mjs`
- Modify: `dashboard/src/app/admin/billing/page.tsx`

- [ ] **Step 1: Write the failing source-contract test**

Replace the existing native-confirm assertion with a test that scopes the source between `GrantTrialForm` and `PlanFlipMenu`:

```js
test("Admin Billing uses UniPost confirmation modals for trial mutations", () => {
  const start = adminBillingSource.indexOf("function GrantTrialForm(");
  const end = adminBillingSource.indexOf("function PlanFlipMenu(");
  assert.ok(start >= 0 && end > start, "expected GrantTrialForm source boundary");
  const grantTrialFormSource = adminBillingSource.slice(start, end);

  assert.match(adminBillingSource, /import \{ ConfirmModal \} from "@\/components\/confirm-modal"/);
  assert.doesNotMatch(grantTrialFormSource, /(?:window\.)?confirm\s*\(/);
  assert.match(grantTrialFormSource, /title="Grant trial"[\s\S]*confirmLabel="Grant Trial"/);
  assert.match(grantTrialFormSource, /title="Revoke trial offer"[\s\S]*confirmLabel="Revoke"[\s\S]*variant="danger"/);
  assert.match(grantTrialFormSource, /onCancel=\{\(\) => setConfirmation\(null\)\}/);
});
```

- [ ] **Step 2: Run the contract test and verify RED**

Run:

```bash
cd dashboard
node --test tests/free-trial-contract-source.test.mjs
```

Expected: FAIL because `GrantTrialForm` still contains `window.confirm` and does not render `ConfirmModal`.

- [ ] **Step 3: Implement confirmation state and shared modals**

Import `ConfirmModal`, add a discriminated `TrialConfirmation` state, and separate request/open behavior from the existing async mutations:

```tsx
type TrialConfirmation =
  | { kind: "grant"; planId: string; durationDays: number; timeline: Array<{ label: string; value: string }> }
  | { kind: "revoke"; grantId: string; planId: string };

const [confirmation, setConfirmation] = useState<TrialConfirmation | null>(null);
```

The Grant and Revoke buttons set this state. Two `ConfirmModal` render branches call the existing async code with captured IDs and values. `onCancel` clears state, Grant uses the default variant, Revoke uses `variant="danger"`, and both use `confirmDisabled={interactionLocked}`.

- [ ] **Step 4: Run the contract test and verify GREEN**

Run the same Node test. Expected: all tests pass and the plan-flip browser confirmation remains outside the scoped assertion.

### Task 2: Reverse Stripe Price Lookup

**Files:**
- Modify: `api/internal/billing/manager_test.go`
- Modify: `api/internal/billing/manager.go`

- [ ] **Step 1: Write the failing `Mode.PlanID` tests**

```go
func TestModePlanIDResolvesConfiguredPrice(t *testing.T) {
	mode := &Mode{priceIDs: map[string]string{
		"basic":  "price_test_basic",
		"growth": "price_test_growth",
	}}
	if got := mode.PlanID(" price_test_growth "); got != "growth" {
		t.Fatalf("PlanID() = %q, want growth", got)
	}
	if got := mode.PlanID("price_live_growth"); got != "" {
		t.Fatalf("unknown PlanID() = %q, want empty", got)
	}
	var nilMode *Mode
	if got := nilMode.PlanID("price_test_growth"); got != "" {
		t.Fatalf("nil Mode.PlanID() = %q, want empty", got)
	}
}
```

- [ ] **Step 2: Run the billing package test and verify RED**

Run `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/billing -run TestModePlanID -count=1`.

Expected: build failure because `Mode.PlanID` does not exist.

- [ ] **Step 3: Implement the minimal reverse lookup**

```go
func (m *Mode) PlanID(priceID string) string {
	priceID = strings.TrimSpace(priceID)
	if m == nil || priceID == "" {
		return ""
	}
	for planID, configuredPriceID := range m.priceIDs {
		if strings.TrimSpace(configuredPriceID) == priceID {
			return planID
		}
	}
	return ""
}
```

- [ ] **Step 4: Run the billing package test and verify GREEN**

Run the same focused Go test. Expected: PASS.

### Task 3: Mode-Aware Webhook Projection

**Files:**
- Modify: `api/internal/handler/stripe_webhook_test.go`
- Modify: `api/internal/handler/stripe_webhook.go`

- [ ] **Step 1: Write failing sandbox Checkout and subscription-update tests**

```go
func newModeAwareStripeTestManager(t *testing.T) *billing.Manager {
	t.Helper()
	t.Setenv("STRIPE_SECRET_KEY", "sk_live_test")
	t.Setenv("STRIPE_PRICE_ID_BASIC", "price_live_basic")
	t.Setenv("STRIPE_PRICE_ID_GROWTH", "price_live_growth")
	t.Setenv("STRIPE_SANDBOX_SECRET_KEY", "sk_test_sandbox")
	t.Setenv("STRIPE_SANDBOX_WEBHOOK_SECRET", "whsec_staging_test")
	t.Setenv("STRIPE_SANDBOX_PRICE_ID_BASIC", "price_basic")
	t.Setenv("STRIPE_SANDBOX_PRICE_ID_GROWTH", "price_sandbox_growth")
	manager, err := billing.NewManager(nil)
	if err != nil { t.Fatal(err) }
	return manager
}

func TestStripeSandboxCheckoutProjectsModeSpecificPrice(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	store.subscription.PlanID = "basic"
	store.plans["basic"] = db.Plan{ID: "basic", Name: "Basic", PriceCents: 1900, PostLimit: 2500, StripePriceID: pgtype.Text{String: "price_live_basic", Valid: true}}
	store.plans["growth"] = db.Plan{ID: "growth", Name: "Growth", PriceCents: 5900, PostLimit: 7500, StripePriceID: pgtype.Text{String: "price_live_growth", Valid: true}}
	manager := newModeAwareStripeTestManager(t)
	trial := &recordingTrialWebhookService{retrieve: trials.SubscriptionSnapshot{
		StripeMode: "sandbox", ID: "sub_staging", Status: "active", CustomerID: "cus_staging", PriceID: "price_sandbox_growth",
		CurrentPeriodStartAt: webhookPtrTime(time.Unix(1784822617, 0).UTC()), CurrentPeriodEndAt: webhookPtrTime(time.Unix(1787501017, 0).UTC()),
		Metadata: map[string]string{"workspace_id": "ws_staging", "plan_id": "growth", "unipost_environment": "staging"},
	}}
	h, _ := newTestStripeWebhookHandler(store, nil)
	h.stripe = manager
	h.SetTrialWebhookService(trial)

	response := postTestCheckoutWebhook(t, h, manager.Sandbox.WebhookSecret, map[string]string{
		"workspace_id": "ws_staging", "plan_id": "growth", "unipost_environment": "staging",
	}, stripe.CheckoutSessionPaymentStatusPaid)

	if response.Code != http.StatusOK || store.subscription.PlanID != "growth" {
		t.Fatalf("status=%d plan=%s body=%s", response.Code, store.subscription.PlanID, response.Body.String())
	}
}

func TestStripeSandboxSubscriptionUpdateProjectsModeSpecificPrice(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	store.plans["basic"] = db.Plan{ID: "basic", Name: "Basic", PriceCents: 1900, PostLimit: 2500, StripePriceID: pgtype.Text{String: "price_live_basic", Valid: true}}
	manager := newModeAwareStripeTestManager(t)
	h, _ := newTestStripeWebhookHandler(store, nil)
	h.stripe = manager

	response := postTestSubscriptionUpdatedWebhook(t, h, manager.Sandbox.WebhookSecret, map[string]string{
		"workspace_id": "ws_staging", "plan_id": "basic", "unipost_environment": "staging",
	})

	if response.Code != http.StatusOK || store.subscription.PlanID != "basic" {
		t.Fatalf("status=%d plan=%s body=%s", response.Code, store.subscription.PlanID, response.Body.String())
	}
}
```

- [ ] **Step 2: Run focused handler tests and verify RED**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestStripeSandbox(Checkout|SubscriptionUpdate)' -count=1
```

Expected: FAIL with `resolve Stripe subscription price` / HTTP 500 because the database cannot resolve the sandbox price ID.

- [ ] **Step 3: Add one canonical resolver and migrate both live paths**

```go
func (h *StripeWebhookHandler) resolveStripePlan(ctx context.Context, modeName, priceID string) (db.Plan, error) {
	mode := h.stripe.ByName(modeName)
	if mode == nil {
		return db.Plan{}, fmt.Errorf("Stripe mode %q is unavailable", modeName)
	}
	if planID := mode.PlanID(priceID); planID != "" {
		plan, err := h.queries.GetPlan(ctx, planID)
		if err != nil { return db.Plan{}, fmt.Errorf("load mode-mapped plan %q: %w", planID, err) }
		return plan, nil
	}
	plan, err := h.queries.GetPlanByStripePriceID(ctx, pgtype.Text{String: priceID, Valid: priceID != ""})
	if err != nil { return db.Plan{}, fmt.Errorf("resolve legacy Stripe price: %w", err) }
	return plan, nil
}
```

Use this resolver in `validateCheckoutSubscriptionBinding` and `projectStripeSubscription`. Delete unused `resolvePlanIDFromStripeSubscription`; do not change the unrelated Admin plan-flip helper.

- [ ] **Step 4: Run focused handler tests and verify GREEN**

Run the same focused command. Expected: PASS for Checkout and subscription update.

- [ ] **Step 5: Run package-level regression tests**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/billing ./internal/handler -count=1
```

Expected: PASS.

### Task 4: Full Local Verification and Commit

**Files:**
- Verify all files changed by Tasks 1–3 and the approved spec/plan documents.

- [ ] **Step 1: Format and inspect**

Run `gofmt` on modified Go files, then `git diff --check` and review the complete diff.

- [ ] **Step 2: Run full API tests**

Run `cd api && GOCACHE=/tmp/unipost-go-build go test ./...`. Expected: PASS.

- [ ] **Step 3: Run Dashboard contract, build, and local regression**

Run:

```bash
cd dashboard
node --test tests/free-trial-contract-source.test.mjs
npm run build
DASHBOARD_BASE_URL=http://app.lvh.me:3000 DASHBOARD_WEB_SERVER=1 npm run test:regression:dashboard
```

Expected: contract passes, build succeeds, and dashboard regression reports 60 passed with no skipped tests.

- [ ] **Step 4: Commit the focused implementation**

Commit production and test changes with `fix: restore sandbox plan upgrades` after confirming the branch contains no unrelated files.

### Task 5: Staging Promotion and Acceptance

**Files:**
- No additional source changes unless a verified failure requires a TDD fix.

- [ ] **Step 1: Audit and push the owned branch**

List commits and changed files unique to `origin/staging`, push `dev-admin-configurable-free-trial`, and open a PR to `staging`. Explicitly record that Preview Acceptance is skipped by user authorization.

- [ ] **Step 2: Monitor GitHub CI and merge**

Wait for API and Dashboard CI on the exact head SHA. Any non-success is a hard stop. Re-audit commits/files, then merge the PR to staging.

- [ ] **Step 3: Monitor staging deployments**

Wait for Railway API/worker/MCP and Vercel staging to succeed on the exact merge SHA.

- [ ] **Step 4: Run deployed staging acceptance**

Run the public staging dashboard regression. In the existing authenticated staging browser session, verify Grant and Revoke open UniPost modals and Cancel causes no mutation. Complete a Basic-to-Growth sandbox Checkout for the existing staging admin workspace and confirm Settings Billing shows Growth after webhook reconciliation.

- [ ] **Step 5: Confirm production PR state**

Verify PR #265 remains open, clean, mergeable, points to the new staging SHA, and has successful checks. Do not merge it.
