# Staging Stripe Plan Upgrade Hotfix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a paid staging Stripe Checkout reconcile exactly once to the owning staging workspace, while foreign-environment, unpaid, and transient-failure deliveries remain safe and observable.

**Architecture:** Keep Stripe webhooks authoritative. Stamp new Checkout Sessions with `UNIPOST_ENV`, acknowledge events explicitly routed to another environment, and require a local workspace before mutating a subscription. Preserve Stripe retry semantics for real database failures and the existing `subscriptions.workspace_id` upsert for idempotency. After the code reaches staging, create a sandbox-only staging webhook endpoint, install its signing secret without displaying it, and replay the already-paid Checkout event.

**Tech Stack:** Go 1.25, chi HTTP handlers, stripe-go v82, sqlc/pgx, PostgreSQL, Railway, Stripe sandbox, GitHub Actions, Vercel.

---

### Task 1: Stamp Stripe Checkout ownership

**Files:**
- Modify: `api/internal/handler/billing.go`
- Test: `api/internal/handler/billing_test.go`

- [x] **Step 1: Write the failing metadata test**

Add a test that sets `UNIPOST_ENV=staging`, calls a small metadata builder, and requires all routing fields:

```go
func TestCheckoutMetadataIncludesRuntimeEnvironment(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, " staging ")

	got := stripeCheckoutMetadata("ws_staging", "basic", "sandbox")

	want := map[string]string{
		"workspace_id":        "ws_staging",
		"plan_id":             "basic",
		"mode":                "sandbox",
		"unipost_environment": "staging",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("metadata = %#v, want %#v", got, want)
	}
}
```

- [x] **Step 2: Run the test and verify RED**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run TestCheckoutMetadataIncludesRuntimeEnvironment -count=1
```

Expected: compile failure because `stripeCheckoutMetadata` does not exist.

- [x] **Step 3: Add the minimal metadata builder**

Add:

```go
const stripeCheckoutEnvironmentMetadataKey = "unipost_environment"

func stripeCheckoutMetadata(workspaceID, planID, mode string) map[string]string {
	return map[string]string{
		"workspace_id":                          workspaceID,
		"plan_id":                               planID,
		"mode":                                  mode,
		stripeCheckoutEnvironmentMetadataKey: runtimeenv.Current(),
	}
}
```

Use the helper as `stripe.CheckoutSessionParams.Params.Metadata`. Do not change prices, Checkout mode, line items, or URLs.

- [x] **Step 4: Run the focused test and verify GREEN**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run TestCheckoutMetadataIncludesRuntimeEnvironment -count=1
```

Expected: PASS.

### Task 2: Fail closed across environments and preserve retries

**Files:**
- Modify: `api/internal/handler/stripe_webhook.go`
- Test: `api/internal/handler/stripe_webhook_test.go`

- [x] **Step 1: Add signed-webhook regression coverage**

Add a `stripeWebhookStore` test double implementing `db.DBTX`. It must:

```go
type stripeWebhookStore struct {
	workspace    db.Workspace
	workspaceErr error
	subscription db.Subscription
	plans        map[string]db.Plan
	upserts      int
}
```

`QueryRow` must support the exact sqlc queries for `GetWorkspace`, `GetSubscriptionByWorkspace`, `GetPlan`, and `CreateSubscription`. `CreateSubscription` must update one in-memory `db.Subscription` and increment `upserts`. `Exec` and `Query` return errors if unexpectedly called.

Use `webhook.GenerateTestSignedPayload` to exercise `HandleStripe` with:

```json
{
  "id": "evt_checkout_basic",
  "object": "event",
  "created": 1784822622,
  "type": "checkout.session.completed",
  "data": {
    "object": {
      "id": "cs_test_basic",
      "object": "checkout.session",
      "mode": "subscription",
      "status": "complete",
      "payment_status": "paid",
      "customer": "cus_staging",
      "subscription": "sub_staging",
      "metadata": {
        "workspace_id": "ws_staging",
        "plan_id": "basic",
        "mode": "sandbox"
      }
    }
  }
}
```

Add these tests:

```go
func TestStripeCheckoutReplayIsIdempotent(t *testing.T)
func TestStripeCheckoutIgnoresForeignEnvironment(t *testing.T)
func TestStripeCheckoutIgnoresLegacyForeignWorkspace(t *testing.T)
func TestStripeCheckoutReturns500ForWorkspaceLookupFailure(t *testing.T)
func TestStripeCheckoutIgnoresUnpaidSession(t *testing.T)
func TestStripeSubscriptionUpdateIgnoresForeignEnvironment(t *testing.T)
func TestStripeSubscriptionUpdateIgnoresLegacyForeignSubscription(t *testing.T)
func TestStripeSubscriptionUpdateRetriesLocalMissingSubscription(t *testing.T)
```

The replay test must send the same signed payload twice and assert two HTTP 200 responses, one subscription row in final `basic/active` state, the same customer/subscription IDs, and no duplicate plan-change lifecycle event. The foreign-environment test uses `unipost_environment=dev` while `UNIPOST_ENV=staging` and asserts no DB call. The legacy foreign-workspace test omits environment metadata, returns `pgx.ErrNoRows` from `GetWorkspace`, and asserts HTTP 200 with zero upserts. The database-failure test returns `errors.New("database unavailable")` and asserts HTTP 500 so Stripe will retry.

- [x] **Step 2: Run the webhook tests and verify RED**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestStripeCheckout(ReplayIsIdempotent|IgnoresForeignEnvironment|IgnoresLegacyForeignWorkspace|Returns500ForWorkspaceLookupFailure)' -count=1
```

Expected: the replay may pass existing upsert behavior, but foreign environment/workspace cases fail with a mutation attempt or HTTP 500 because no routing guard exists.

- [x] **Step 3: Add an explicit ignored-delivery path**

Add:

```go
var errStripeWebhookNotApplicable = errors.New("stripe webhook is not applicable to this environment")

func (h *StripeWebhookHandler) validateCheckoutTarget(ctx context.Context, session stripe.CheckoutSession) error {
	eventEnvironment := strings.ToLower(strings.TrimSpace(session.Metadata[stripeCheckoutEnvironmentMetadataKey]))
	if eventEnvironment != "" && eventEnvironment != runtimeenv.Current() {
		return errStripeWebhookNotApplicable
	}
	if session.Mode != stripe.CheckoutSessionModeSubscription ||
		session.Status != stripe.CheckoutSessionStatusComplete ||
		(session.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid &&
			session.PaymentStatus != stripe.CheckoutSessionPaymentStatusNoPaymentRequired) {
		return errStripeWebhookNotApplicable
	}
	_, err := h.queries.GetWorkspace(ctx, session.Metadata["workspace_id"])
	if errors.Is(err, pgx.ErrNoRows) {
		return errStripeWebhookNotApplicable
	}
	if err != nil {
		return fmt.Errorf("load checkout workspace: %w", err)
	}
	return nil
}
```

Call it after required metadata parsing and before plan/subscription queries. In `HandleStripe`, treat only `errStripeWebhookNotApplicable` as an acknowledged no-op and keep every other error on the existing HTTP 500 path. Include `event_id`, `workspace_id`, and the local/event environment in structured logs; never log webhook secrets or full payloads.

Reuse the same metadata map as `CheckoutSessionParams.SubscriptionData.Metadata`. For `customer.subscription.updated`, ignore an explicit foreign environment before any query, acknowledge a legacy subscription that does not exist locally, and keep HTTP 500 retry behavior when an explicitly local subscription is temporarily missing.

- [x] **Step 4: Run the focused tests and verify GREEN**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestStripeCheckout(ReplayIsIdempotent|IgnoresForeignEnvironment|IgnoresLegacyForeignWorkspace|Returns500ForWorkspaceLookupFailure)' -count=1
```

Expected: PASS.

- [x] **Step 5: Run handler and full API regression suites**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -count=1
GOCACHE=/tmp/unipost-go-build go test ./... -count=1
```

Expected: both commands exit 0 with no failed, skipped, timed-out, or missing results.

### Task 3: Publish the isolated hotfix

**Files:**
- Modify only the files listed in Tasks 1 and 2 plus this plan.

- [ ] **Step 1: Audit branch content**

Run:

```bash
git status --short
git diff --check
git diff --stat origin/staging...HEAD
git log --oneline origin/staging..HEAD
git diff --name-only origin/staging...HEAD
```

Expected changed files:

```text
api/internal/handler/billing.go
api/internal/handler/billing_test.go
api/internal/handler/stripe_webhook.go
api/internal/handler/stripe_webhook_test.go
docs/superpowers/plans/2026-07-23-staging-stripe-plan-upgrade-hotfix.md
```

- [ ] **Step 2: Commit and push only the owned branch**

Run:

```bash
git add api/internal/handler/billing.go api/internal/handler/billing_test.go api/internal/handler/stripe_webhook.go api/internal/handler/stripe_webhook_test.go docs/superpowers/plans/2026-07-23-staging-stripe-plan-upgrade-hotfix.md
git commit -m "fix(billing): isolate staging Stripe reconciliation"
git push -u origin hotfix-staging-stripe-plan-upgrade
```

- [ ] **Step 3: Open a pull request to staging**

Create a pull request from `hotfix-staging-stripe-plan-upgrade` to `staging`. Record the exact head SHA and monitor all GitHub Actions, Railway, and Vercel checks until terminal success. Any failure, skip, cancellation, timeout, or absent result is a hard stop.

- [ ] **Step 4: Promotion content audit and merge**

Immediately before merge, re-run:

```bash
git fetch origin
git log --oneline origin/staging..origin/hotfix-staging-stripe-plan-upgrade
git diff --name-only origin/staging...origin/hotfix-staging-stripe-plan-upgrade
```

Merge only if the commits and files exactly match this hotfix and every required check passed on the exact head SHA.

### Task 4: Configure and reconcile the paid staging fixture

**External scope:** Stripe sandbox and Railway `UniPost/staging/unipost` only. Do not touch live-mode endpoints, production variables, customer accounts, X resources, or databases directly.

- [ ] **Step 1: Create the missing sandbox staging endpoint**

Using `STRIPE_SANDBOX_SECRET_KEY` injected by Railway, create exactly one enabled endpoint:

```text
https://staging-api.unipost.dev/webhooks/stripe
```

Enabled events:

```text
checkout.session.completed
customer.subscription.updated
customer.subscription.deleted
invoice.payment_failed
invoice.payment_succeeded
```

Capture the returned endpoint ID and signing secret in shell variables. Pass the secret directly to Railway as `STRIPE_SANDBOX_WEBHOOK_SECRET` without printing, copying, or writing it to disk. The variable update may redeploy staging; monitor that deployment to terminal success.

- [ ] **Step 2: Prove the exact historical event before replay**

Retrieve and compare:

```text
workspace:    c2bb1186-6bf2-4743-9c2a-51154a6b16cd
session:      cs_test_a1ZgMhlBMFPCaHqGi0CHh8TMeBTYqYS9cXzLgYHvE8ThuFONXmFfsrxqPC
event:        evt_1TwP62CvcddfMNGWxpQlgxGn
subscription: sub_1TwP60CvcddfMNGWrm2tDhuI
plan:         basic
amount:       1900 USD
payment:      paid
mode:         sandbox
```

Abort if any field differs.

- [ ] **Step 3: Replay without charging**

Store the endpoint creation response ID in `staging_endpoint_id`, then use `stripe events resend evt_1TwP62CvcddfMNGWxpQlgxGn --webhook-endpoint "$staging_endpoint_id"` with the already-injected sandbox API key. This re-delivers the legal event and must not create a Checkout Session, PaymentIntent, invoice, subscription, or charge.

- [ ] **Step 4: Verify authoritative staging state**

In a `BEGIN READ ONLY` transaction, join `subscriptions`, `plans`, and the exact workspace. Require:

```text
plan_id                = basic
status                 = active
stripe_customer_id     = cus_UwHf8IjkU5Peen
stripe_subscription_id = sub_1TwP60CvcddfMNGWrm2tDhuI
allow_inbox             = true
```

Replay the same event once more and prove the subscription row, plan, customer, subscription, and row count remain unchanged.

- [ ] **Step 5: Browser acceptance on real staging**

Using the existing staging fixture session, reload:

```text
https://staging-app.unipost.dev/settings/billing
```

Require the authoritative Current Plan to display Basic, the X/Inbox allowance API to report the Basic allowance, and Inbox access gates to be enabled. Confirm `GET /v1/billing` and `GET /v1/billing/x-credits` agree with the database. Do not interact with X provider resources or run an X backfill.

- [ ] **Step 6: Stop at staging**

Do not create or merge a `staging -> main` pull request. Report staging acceptance evidence and direct the user back to the `hotfix-x-inbox-webhooks` task for the X Inbox canary.
