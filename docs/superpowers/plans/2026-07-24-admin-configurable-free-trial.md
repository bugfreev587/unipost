# Admin-Granted Configurable Free Trial Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement admin-granted, configurable 1–730 day trials for Free and currently paid workspaces, including Stripe lifecycle safety, reminders, user/admin UI, and Trial History.

**Architecture:** Add an additive `workspace_trial_grants` ledger and a focused `internal/trials` domain service. HTTP handlers delegate lifecycle decisions to that service; a narrow Stripe gateway performs mode-aware Checkout, Subscription, Schedule, and Portal calls; Stripe webhooks remain the authority for final remote-state projection. Pricing and Settings → Billing consume one shared trial projection and formatter.

**Tech Stack:** Go 1.24, pgx/sqlc, stripe-go v82, chi, Loops lifecycle events, Next.js 16, React 19, TypeScript, Node source-contract tests, Playwright.

---

## File Map

- Create `api/internal/db/migrations/121_workspace_trial_grants.sql` for the additive trial ledger and indexes.
- Create `api/internal/db/queries/workspace_trial_grants.sql` for grant creation, compare-and-swap transitions, correlation, and history.
- Create `api/internal/db/workspace_trial_grants_contract_test.go` for executable migration/SQL contracts.
- Create `api/internal/trials/model.go` for status/kind constants, validation, state transitions, and API projections.
- Create `api/internal/trials/service.go` for grant, revoke, checkout claim, plan change, cancellation, and webhook reconciliation orchestration.
- Create `api/internal/trials/service_test.go` for deterministic domain/Stripe race tests.
- Create `api/internal/trials/stripe_gateway.go` for the production stripe-go adapter.
- Modify `api/internal/billing/manager.go` to expose mode-specific trial Portal configuration and Stripe lookup by stored mode.
- Modify `api/internal/handler/admin.go` for grant/revoke endpoints and trial summaries in Admin Billing.
- Modify `api/internal/handler/billing.go` for trial projection/history, trial-aware Checkout, Portal, change-plan, and cancel-renewal.
- Modify `api/internal/handler/stripe_webhook.go` for real Subscription projection and grant-aware cancellation.
- Modify `api/internal/handler/stripe_webhook_test.go`, `billing_test.go`, and `admin_test.go` for handler regressions.
- Modify `api/internal/handler/loops_lifecycle.go`, `api/internal/emailregistry/registry.go`, and `docs/email-templates.md` for the trial-ending email.
- Modify `api/cmd/api/main.go` for service construction and routes.
- Create `dashboard/src/lib/trial-format.ts` for shared card/history labels and dates.
- Create `dashboard/tests/free-trial-contract-source.test.mjs` for frontend TDD source contracts.
- Modify `dashboard/src/lib/api.ts`, Admin Billing, Pricing, and Settings → Billing pages.
- Modify `dashboard/tests/regression/dashboard.spec.ts` and `dashboard/tests/regression/mobile-layout.spec.ts` for rendered acceptance.

### Task 1: Persist the grant lifecycle

**Files:**
- Create: `api/internal/db/workspace_trial_grants_contract_test.go`
- Create: `api/internal/db/migrations/121_workspace_trial_grants.sql`
- Create: `api/internal/db/queries/workspace_trial_grants.sql`
- Generate: `api/internal/db/models.go`, `api/internal/db/workspace_trial_grants.sql.go`

- [ ] **Step 1: Write the failing migration contract test**

```go
func TestWorkspaceTrialGrantMigrationContract(t *testing.T) {
    migration, err := os.ReadFile("migrations/121_workspace_trial_grants.sql")
    if err != nil { t.Fatal(err) }
    body := string(migration)
    for _, want := range []string{
        "CREATE TABLE workspace_trial_grants",
        "duration_days BETWEEN 1 AND 730",
        "checkout_pending",
        "workspace_trial_grants_one_open_per_workspace",
        "stripe_checkout_session_id",
        "stripe_subscription_id",
        "stripe_schedule_id",
    } {
        if !strings.Contains(body, want) { t.Fatalf("missing %q", want) }
    }
}
```

- [ ] **Step 2: Run the test and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run TestWorkspaceTrialGrantMigrationContract -count=1`

Expected: FAIL because migration 121 does not exist.

- [ ] **Step 3: Add the additive migration**

```sql
-- +goose Up
CREATE TABLE workspace_trial_grants (
  id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  kind TEXT NOT NULL CHECK (kind IN ('free_to_paid', 'paid_same_plan')),
  plan_id TEXT NOT NULL REFERENCES plans(id),
  duration_days INTEGER NOT NULL CHECK (duration_days BETWEEN 1 AND 730),
  status TEXT NOT NULL CHECK (status IN (
    'provisioning','pending_activation','checkout_pending','scheduled','active',
    'completed','canceled','revoked','superseded','failed'
  )),
  granted_by_user_id TEXT NOT NULL,
  stripe_mode TEXT,
  stripe_customer_id TEXT,
  stripe_subscription_id TEXT,
  stripe_schedule_id TEXT,
  stripe_checkout_session_id TEXT,
  granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  scheduled_start_at TIMESTAMPTZ,
  started_at TIMESTAMPTZ,
  ends_at TIMESTAMPTZ,
  activated_at TIMESTAMPTZ,
  canceled_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  superseded_at TIMESTAMPTZ,
  superseded_by_plan_id TEXT REFERENCES plans(id),
  completed_at TIMESTAMPTZ,
  failure_code TEXT,
  failure_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX workspace_trial_grants_one_open_per_workspace
  ON workspace_trial_grants(workspace_id)
  WHERE status IN ('provisioning','pending_activation','checkout_pending','scheduled','active');
CREATE UNIQUE INDEX workspace_trial_grants_checkout_session_unique
  ON workspace_trial_grants(stripe_checkout_session_id)
  WHERE stripe_checkout_session_id IS NOT NULL;
CREATE INDEX workspace_trial_grants_subscription_idx ON workspace_trial_grants(stripe_subscription_id);
CREATE INDEX workspace_trial_grants_schedule_idx ON workspace_trial_grants(stripe_schedule_id);
CREATE INDEX workspace_trial_grants_history_idx ON workspace_trial_grants(workspace_id, granted_at DESC);
CREATE INDEX workspace_trial_grants_end_idx ON workspace_trial_grants(status, ends_at);

-- +goose Down
DROP TABLE workspace_trial_grants;
```

- [ ] **Step 4: Add sqlc queries with compare-and-swap guards**

```sql
-- name: CreateWorkspaceTrialGrant :one
INSERT INTO workspace_trial_grants (
  workspace_id, kind, plan_id, duration_days, status, granted_by_user_id,
  stripe_mode, stripe_customer_id, stripe_subscription_id, scheduled_start_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
RETURNING *;

-- name: GetOpenWorkspaceTrialGrant :one
SELECT * FROM workspace_trial_grants
WHERE workspace_id = $1
  AND status IN ('provisioning','pending_activation','checkout_pending','scheduled','active')
ORDER BY granted_at DESC LIMIT 1;

-- name: ClaimWorkspaceTrialCheckout :one
UPDATE workspace_trial_grants
SET status='checkout_pending', updated_at=NOW()
WHERE id=$1 AND workspace_id=$2 AND plan_id=$3 AND status='pending_activation'
RETURNING *;

-- name: RecordWorkspaceTrialCheckoutSession :one
UPDATE workspace_trial_grants
SET stripe_checkout_session_id=$2, updated_at=NOW()
WHERE id=$1 AND status='checkout_pending' AND stripe_checkout_session_id IS NULL
RETURNING *;

-- name: ReleaseExpiredWorkspaceTrialCheckout :one
UPDATE workspace_trial_grants
SET status='pending_activation', stripe_checkout_session_id=NULL, updated_at=NOW()
WHERE id=$1 AND status='checkout_pending' AND stripe_checkout_session_id=$2
RETURNING *;

-- name: ListWorkspaceTrialHistory :many
SELECT * FROM workspace_trial_grants WHERE workspace_id=$1 ORDER BY granted_at DESC;
```

Use these generic correlation and compare-and-swap queries; the domain transition table validates the requested pair before calling `TransitionWorkspaceTrialGrant`:

```sql
-- name: GetWorkspaceTrialGrant :one
SELECT * FROM workspace_trial_grants WHERE id=$1;

-- name: GetWorkspaceTrialGrantByCheckoutSession :one
SELECT * FROM workspace_trial_grants WHERE stripe_checkout_session_id=$1;

-- name: GetWorkspaceTrialGrantByStripeSubscription :one
SELECT * FROM workspace_trial_grants
WHERE stripe_subscription_id=$1 ORDER BY granted_at DESC LIMIT 1;

-- name: GetWorkspaceTrialGrantByStripeSchedule :one
SELECT * FROM workspace_trial_grants
WHERE stripe_schedule_id=$1 ORDER BY granted_at DESC LIMIT 1;

-- name: TransitionWorkspaceTrialGrant :one
UPDATE workspace_trial_grants
SET status=sqlc.arg(next_status), updated_at=NOW()
WHERE id=sqlc.arg(id) AND status=sqlc.arg(expected_status)
RETURNING *;
```

Add focused update queries that record remote identifiers/timestamps in the same statement as each transition, so `scheduled`, `active`, `completed`, `canceled`, `revoked`, `superseded`, and `failed` never require a second local write.

- [ ] **Step 5: Generate sqlc and verify GREEN**

Run: `cd api && sqlc generate && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run 'TestWorkspaceTrialGrant|TestEmbeddedMigrationVersions' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/db
git commit -m "feat: add workspace trial grant ledger"
```

### Task 2: Define and test the trial state machine

**Files:**
- Create: `api/internal/trials/model.go`
- Create: `api/internal/trials/service.go`
- Create: `api/internal/trials/service_test.go`

- [ ] **Step 1: Write failing validation and transition tests**

```go
func TestValidateGrantInput(t *testing.T) {
    for _, days := range []int32{1, 730} {
        if err := ValidateGrantInput("growth", days); err != nil { t.Fatal(err) }
    }
    for _, tc := range []struct{ plan string; days int32 }{
        {"enterprise", 30}, {"free", 30}, {"growth", 0}, {"growth", 731},
    } {
        if ValidateGrantInput(tc.plan, tc.days) == nil { t.Fatalf("accepted %#v", tc) }
    }
}

func TestCanTransitionCheckoutExpiryOnlyFromCheckoutPending(t *testing.T) {
    if !CanTransition(StatusCheckoutPending, StatusPendingActivation) { t.Fatal("expiry must reopen offer") }
    if CanTransition(StatusRevoked, StatusPendingActivation) { t.Fatal("terminal state moved backward") }
}
```

- [ ] **Step 2: Run and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/trials -count=1`

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement constants, validation, projections, and transition table**

```go
type Kind string
const (
    KindFreeToPaid Kind = "free_to_paid"
    KindPaidSamePlan Kind = "paid_same_plan"
)
type Status string
const (
    StatusProvisioning Status = "provisioning"
    StatusPendingActivation Status = "pending_activation"
    StatusCheckoutPending Status = "checkout_pending"
    StatusScheduled Status = "scheduled"
    StatusActive Status = "active"
    StatusCompleted Status = "completed"
    StatusCanceled Status = "canceled"
    StatusRevoked Status = "revoked"
    StatusSuperseded Status = "superseded"
    StatusFailed Status = "failed"
)

func ValidateGrantInput(planID string, days int32) error {
    if days < 1 || days > 730 { return ErrInvalidDuration }
    switch planID { case "api", "basic", "growth", "team": return nil }
    return ErrInvalidPlan
}
```

Define `TrialProjection` and `HistoryProjection` without admin identity or raw failure fields. Implement an explicit transition map, with only `checkout_pending -> pending_activation` allowed as the documented reopening transition.

Add deterministic fakes used by every later service test:

```go
type serviceHarness struct {
    service *Service
    store *fakeStore
    stripe *fakeStripeGateway
}

func newServiceHarness(t *testing.T) *serviceHarness {
    t.Helper()
    store := &fakeStore{grants: map[string]Grant{}}
    stripe := &fakeStripeGateway{}
    return &serviceHarness{
        service: NewService(store, stripe, func() time.Time {
            return time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
        }),
        store: store,
        stripe: stripe,
    }
}
```

- [ ] **Step 4: Run and verify GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/trials -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/trials
git commit -m "feat: define trial grant state machine"
```

### Task 3: Add the narrow mode-aware Stripe gateway

**Files:**
- Modify: `api/internal/billing/manager.go`
- Modify: `api/internal/billing/manager_test.go`
- Create: `api/internal/trials/stripe_gateway.go`
- Modify: `api/internal/trials/service_test.go`

- [ ] **Step 1: Write failing mode/configuration tests**

```go
func TestManagerLoadsTrialPortalConfigurationsPerMode(t *testing.T) {
    t.Setenv("STRIPE_SECRET_KEY", "sk_live_test")
    t.Setenv("STRIPE_BILLING_PORTAL_TRIAL_CONFIGURATION_ID", "bpc_live")
    t.Setenv("STRIPE_SANDBOX_SECRET_KEY", "sk_test_test")
    t.Setenv("STRIPE_SANDBOX_BILLING_PORTAL_TRIAL_CONFIGURATION_ID", "bpc_test")
    mgr, err := NewManager(nil)
    if err != nil { t.Fatal(err) }
    if mgr.Live.TrialPortalConfigurationID() != "bpc_live" { t.Fatal("live config") }
    if mgr.Sandbox.TrialPortalConfigurationID() != "bpc_test" { t.Fatal("sandbox config") }
}
```

- [ ] **Step 2: Run and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/billing -run TestManagerLoadsTrialPortalConfigurationsPerMode -count=1`

Expected: FAIL because Mode has no Portal configuration.

- [ ] **Step 3: Add configuration and gateway interface**

```go
type StripeGateway interface {
    CreatePaidTrialSchedule(context.Context, PaidTrialScheduleRequest) (ScheduleSnapshot, error)
    CreateTrialCheckout(context.Context, TrialCheckoutRequest) (CheckoutSnapshot, error)
    RetrieveCheckout(context.Context, string, string) (CheckoutSnapshot, error)
    ExpireCheckout(context.Context, string, string) (CheckoutSnapshot, error)
    RetrieveSubscription(context.Context, string, string) (SubscriptionSnapshot, error)
    ChangeFreeTrialPlanNow(context.Context, ChangePlanRequest) (SubscriptionSnapshot, error)
    ChangeScheduledTrialPlanNow(context.Context, ChangePlanRequest) (ScheduleSnapshot, error)
    CancelFreeTrialAtEnd(context.Context, CancelRequest) error
    CancelPaidScheduleAtTrialEnd(context.Context, CancelRequest) error
    CreatePortal(context.Context, PortalRequest) (string, error)
}
```

Add `trialPortalConfigurationID` to each `billing.Mode`, load the exact environment variables from the PRD, and add `Manager.ByName("live"|"sandbox")`. The production gateway uses only `mode.Client` and attaches `workspace_id`, `plan_id`, `trial_grant_id`, `trial_kind`, and `unipost_environment` metadata to every supported Stripe object.

- [ ] **Step 4: Add fake-gateway tests for idempotency keys and single mutations**

Assert operation keys are `trial:{grantID}:schedule`, `trial:{grantID}:checkout`, `trial:{grantID}:change_plan`, and `trial:{grantID}:cancel_renewal`. Assert the paid plan-change method performs Schedule Update and never Schedule Release.

- [ ] **Step 5: Run and verify GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/billing ./internal/trials -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/billing api/internal/trials
git commit -m "feat: add Stripe trial gateway"
```

### Task 4: Implement admin grant and revoke

**Files:**
- Modify: `api/internal/trials/service.go`
- Modify: `api/internal/trials/service_test.go`
- Modify: `api/internal/handler/admin.go`
- Modify: `api/internal/handler/admin_test.go`
- Modify: `api/internal/audit/audit.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write failing service tests for Free and paid grants**

```go
func TestGrantFreeCreatesPendingWithoutStripe(t *testing.T) {
    h := newServiceHarness(t)
    h.store.subscription = SubscriptionSnapshot{WorkspaceID: "ws_1", PlanID: "free", Status: "active"}
    got, err := h.service.Grant(context.Background(), GrantRequest{WorkspaceID: "ws_1", PlanID: "growth", DurationDays: 30, ActorUserID: "admin_1"})
    if err != nil { t.Fatal(err) }
    if got.Kind != KindFreeToPaid || got.Status != StatusPendingActivation { t.Fatalf("grant=%#v", got) }
    if h.stripe.createScheduleCalls != 0 { t.Fatal("Free grant called Stripe") }
}

func TestGrantPaidSchedulesCurrentPlanAfterPeriodEnd(t *testing.T) {
    h := newServiceHarness(t)
    periodEnd := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
    h.store.subscription = SubscriptionSnapshot{WorkspaceID: "ws_1", PlanID: "basic", Status: "active", CurrentPeriodEnd: periodEnd, StripeSubscriptionID: "sub_1"}
    h.stripe.schedule = ScheduleSnapshot{ID: "sub_sched_1", TrialStart: periodEnd, TrialEnd: periodEnd.AddDate(0, 0, 60)}
    got, err := h.service.Grant(context.Background(), GrantRequest{WorkspaceID: "ws_1", PlanID: "basic", DurationDays: 60, ActorUserID: "admin_1"})
    if err != nil { t.Fatal(err) }
    if got.Kind != KindPaidSamePlan || got.Status != StatusScheduled || got.StripeScheduleID != "sub_sched_1" { t.Fatalf("grant=%#v", got) }
}

func TestGrantRejectsPaidDifferentPlan(t *testing.T) {
    h := newServiceHarness(t)
    h.store.subscription = SubscriptionSnapshot{WorkspaceID: "ws_1", PlanID: "basic", Status: "active", StripeSubscriptionID: "sub_1"}
    _, err := h.service.Grant(context.Background(), GrantRequest{WorkspaceID: "ws_1", PlanID: "growth", DurationDays: 30, ActorUserID: "admin_1"})
    if !errors.Is(err, ErrPaidPlanMismatch) { t.Fatalf("err=%v", err) }
    if h.stripe.createScheduleCalls != 0 { t.Fatal("ineligible grant mutated Stripe") }
}
```

Add separate table tests for `cancel_at_period_end=true` and an unrelated attached Schedule; both return `ErrIneligibleSubscription` with zero Stripe calls.

- [ ] **Step 2: Run and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/trials -run 'TestGrant' -count=1`

Expected: FAIL because `Service.Grant` is missing.

- [ ] **Step 3: Implement grant behavior**

```go
type GrantRequest struct {
    WorkspaceID string
    PlanID string
    DurationDays int32
    ActorUserID string
}

func (s *Service) Grant(ctx context.Context, req GrantRequest) (Grant, error) {
    // validate, reject another open grant, inspect local subscription;
    // Free inserts pending_activation; paid inserts provisioning before Stripe;
    // paid uses the workspace owner's Stripe mode and only current plan;
    // Stripe success records schedule ID and scheduled state, failure records failed.
}
```

- [ ] **Step 4: Write failing revoke race tests**

Cover pending local revoke, open Checkout expiry then revoke, completed Checkout expiry error returning conflict, and stale `checkout.session.expired` not moving `revoked` backward.

- [ ] **Step 5: Implement `GrantTrial` and `RevokeTrial` handlers/routes**

Use `POST /v1/admin/workspaces/{workspaceID}/trials` and `/trials/{trialID}/revoke`. Return 422 for plan/duration, 409 for eligibility/open-grant/race conflicts, and write `TRIAL.GRANTED` / `TRIAL.REVOKED` audit events.

- [ ] **Step 6: Run handler and domain tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/trials ./internal/handler -run 'Test(Admin.*Trial|Grant|Revoke)' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/internal/trials api/internal/handler api/internal/audit api/cmd/api/main.go
git commit -m "feat: grant and revoke admin trials"
```

### Task 5: Make Free Checkout trial-aware and race-safe

**Files:**
- Modify: `api/internal/trials/service.go`
- Modify: `api/internal/trials/service_test.go`
- Modify: `api/internal/handler/billing.go`
- Modify: `api/internal/handler/billing_test.go`

- [ ] **Step 1: Write failing Checkout claim tests**

```go
func TestPrepareCheckoutClaimsMatchingPendingGrant(t *testing.T) {
    h := newServiceHarness(t)
    h.store.openGrant = Grant{ID: "trial_1", WorkspaceID: "ws_1", PlanID: "growth", DurationDays: 30, Status: StatusPendingActivation}
    h.stripe.checkout = CheckoutSnapshot{ID: "cs_1", Status: "open", URL: "https://checkout.stripe.test/cs_1"}
    got, err := h.service.PrepareCheckout(context.Background(), CheckoutRequest{WorkspaceID: "ws_1", PlanID: "growth"})
    if err != nil { t.Fatal(err) }
    if got.TrialDays != 30 || got.Metadata["trial_grant_id"] != "trial_1" || h.store.openGrant.Status != StatusCheckoutPending { t.Fatalf("checkout=%#v grant=%#v", got, h.store.openGrant) }
}

func TestPrepareCheckoutResumesRecordedOpenSession(t *testing.T) {
    h := newServiceHarness(t)
    h.store.openGrant = Grant{ID: "trial_1", WorkspaceID: "ws_1", PlanID: "growth", Status: StatusCheckoutPending, StripeCheckoutSessionID: "cs_1"}
    h.stripe.checkout = CheckoutSnapshot{ID: "cs_1", Status: "open", URL: "https://checkout.stripe.test/cs_1"}
    got, err := h.service.PrepareCheckout(context.Background(), CheckoutRequest{WorkspaceID: "ws_1", PlanID: "growth"})
    if err != nil { t.Fatal(err) }
    if got.URL != h.stripe.checkout.URL || h.stripe.createCheckoutCalls != 0 { t.Fatalf("checkout=%#v calls=%d", got, h.stripe.createCheckoutCalls) }
}

func TestPrepareCheckoutMismatchedPlanCreatesOrdinaryCheckout(t *testing.T) {
    h := newServiceHarness(t)
    h.store.openGrant = Grant{ID: "trial_1", WorkspaceID: "ws_1", PlanID: "growth", Status: StatusPendingActivation}
    h.stripe.checkout = CheckoutSnapshot{ID: "cs_paid", Status: "open", URL: "https://checkout.stripe.test/cs_paid"}
    got, err := h.service.PrepareCheckout(context.Background(), CheckoutRequest{WorkspaceID: "ws_1", PlanID: "basic"})
    if err != nil { t.Fatal(err) }
    if got.TrialDays != 0 || got.Metadata["trial_grant_id"] != "" || h.store.openGrant.Status != StatusPendingActivation { t.Fatalf("checkout=%#v grant=%#v", got, h.store.openGrant) }
}
```

Add `TestPrepareCheckoutReopensExpiredSession` with an expired retrieved Session; assert one guarded reopening, one replacement Checkout call, and the replacement Session ID stored on the grant.

- [ ] **Step 2: Run and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/trials -run TestPrepareCheckout -count=1`

Expected: FAIL because checkout orchestration is missing.

- [ ] **Step 3: Implement claim/resume/expiry and wire `CreateCheckout`**

For a matching grant, pass `trial_period_days`, require payment-method collection, attach grant metadata to Session and SubscriptionData, and persist the Session ID. On Stripe create failure, compare-and-swap `checkout_pending -> pending_activation`. A matching `checkout_pending` retrieves and resumes an open Session rather than creating another.

- [ ] **Step 4: Verify handler metadata and Checkout tests GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/trials ./internal/handler -run 'Test(PrepareCheckout|CheckoutMetadata|CheckoutSubscriptionData)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/trials api/internal/handler/billing.go api/internal/handler/billing_test.go
git commit -m "feat: activate free grants through Checkout"
```

### Task 6: Refactor webhook projection and protect managed cancellation

**Files:**
- Modify: `api/internal/handler/stripe_webhook.go`
- Modify: `api/internal/handler/stripe_webhook_test.go`
- Modify: `api/internal/trials/service.go`
- Modify: `api/internal/trials/service_test.go`

- [ ] **Step 1: Replace the old replay expectation with a failing real-status test**

```go
func TestStripeTrialCheckoutProjectsRetrievedSubscriptionAsTrialing(t *testing.T) {
    // checkout.session.completed references sub_trial;
    // fake verified mode retrieves status=trialing and target price;
    // assert local subscription status trialing and grant active.
}
```

Also add tests for `customer.subscription.created` arriving before and after Checkout, and retrieval failure returning HTTP 500 so Stripe retries.

- [ ] **Step 2: Run and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestStripeTrialCheckout|TestStripeSubscriptionCreated' -count=1`

Expected: FAIL because Checkout still hard-codes `active` and created events are ignored.

- [ ] **Step 3: Implement one subscription projector**

Pass the verified `*billing.Mode` into Checkout handling, retrieve the Subscription, and route Checkout/created/updated through `projectStripeSubscription`. Correlate by `trial_grant_id` metadata first and Stripe ID second. Persist actual status, price, period, trial timestamps, customer/subscription IDs, and cancellation fields.

- [ ] **Step 4: Write the blocker regression tests**

```go
func TestManagedTrialCancelAtPeriodEndRetainsPlanAndAccess(t *testing.T) {
    store := newStripeWebhookStore("ws_1")
    store.activeGrant = db.WorkspaceTrialGrant{ID: "trial_1", WorkspaceID: "ws_1", PlanID: "growth", Status: "active"}
    response := postTestSubscriptionUpdatedWebhookWithState(t, store, stripe.SubscriptionStatusTrialing, true, map[string]string{"trial_grant_id": "trial_1"})
    if response.Code != http.StatusOK { t.Fatalf("status=%d body=%s", response.Code, response.Body.String()) }
    if store.subscription.PlanID != "growth" || store.subscription.Status != "trialing" { t.Fatalf("subscription=%#v", store.subscription) }
    if store.activeGrant.Status != "active" || !store.activeGrant.CanceledAt.Valid { t.Fatalf("grant=%#v", store.activeGrant) }
}

func TestLegacyTrialCancelAtPeriodEndStillDowngradesImmediately(t *testing.T) {
    store := newStripeWebhookStore("ws_1")
    response := postTestSubscriptionUpdatedWebhookWithState(t, store, stripe.SubscriptionStatusTrialing, true, nil)
    if response.Code != http.StatusOK || store.subscription.PlanID != "free" { t.Fatalf("status=%d subscription=%#v", response.Code, store.subscription) }
}
```

Add `TestTrialingPaidPlanUnlocksPlanGateWithoutCountingMRR`: persist a `trialing` Growth subscription, assert `PlanIDFor` returns `growth` and plan-gated Analytics is allowed, while the Admin Billing MRR query's `status='active'` filter excludes it.

- [ ] **Step 5: Make `isTrialCancellation` grant-aware**

Replace the unconditional helper with a service lookup. Only unmatched legacy trials enter the immediate downgrade branch. Matching managed trials update projection and retain the target plan through `ends_at`.

- [ ] **Step 6: Handle Checkout expiry and Schedule lifecycle events**

Wire `checkout.session.expired`, `subscription_schedule.created/updated/completed/canceled/released/aborted`, and terminal Subscription events to guarded grant transitions. Foreign-environment events remain acknowledged without mutation.

- [ ] **Step 7: Run webhook, quota, and handler tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler ./internal/trials ./internal/quota -count=1`

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add api/internal/handler api/internal/trials
git commit -m "feat: project managed trial webhooks safely"
```

### Task 7: Implement plan changes, cancellation, and Portal isolation

**Files:**
- Modify: `api/internal/trials/service.go`
- Modify: `api/internal/trials/service_test.go`
- Modify: `api/internal/handler/billing.go`
- Modify: `api/internal/handler/billing_test.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write failing dispatch tests**

```go
func TestChangePlanFreeTrialUsesOneSubscriptionUpdate(t *testing.T) {
    h := newServiceHarness(t)
    h.store.openGrant = Grant{ID: "trial_1", Kind: KindFreeToPaid, Status: StatusActive, PlanID: "basic", StripeSubscriptionID: "sub_1"}
    _, err := h.service.ChangePlan(context.Background(), ChangePlanRequest{WorkspaceID: "ws_1", TargetPlanID: "growth"})
    if err != nil { t.Fatal(err) }
    if h.stripe.subscriptionUpdateCalls != 1 || h.stripe.scheduleUpdateCalls != 0 { t.Fatalf("subscription=%d schedule=%d", h.stripe.subscriptionUpdateCalls, h.stripe.scheduleUpdateCalls) }
    if h.stripe.lastChange.TrialEnd != "now" || h.stripe.lastChange.BillingCycleAnchor != "now" || h.stripe.lastChange.ProrationBehavior != "none" { t.Fatalf("change=%#v", h.stripe.lastChange) }
}

func TestChangePlanPaidTrialUsesOneScheduleUpdateAndNeverRelease(t *testing.T) {
    h := newServiceHarness(t)
    h.store.openGrant = Grant{ID: "trial_1", Kind: KindPaidSamePlan, Status: StatusScheduled, PlanID: "basic", StripeScheduleID: "sub_sched_1"}
    _, err := h.service.ChangePlan(context.Background(), ChangePlanRequest{WorkspaceID: "ws_1", TargetPlanID: "growth"})
    if err != nil { t.Fatal(err) }
    if h.stripe.scheduleUpdateCalls != 1 || h.stripe.scheduleReleaseCalls != 0 || h.stripe.subscriptionUpdateCalls != 0 { t.Fatalf("gateway=%#v", h.stripe) }
}

func TestCancelRenewalDispatchesByKind(t *testing.T) {
    for _, tc := range []struct{ kind Kind; wantSubscription, wantSchedule int }{
        {KindFreeToPaid, 1, 0}, {KindPaidSamePlan, 0, 1},
    } {
        h := newServiceHarness(t)
        h.store.openGrant = Grant{ID: "trial_1", Kind: tc.kind, Status: StatusActive, StripeSubscriptionID: "sub_1", StripeScheduleID: "sub_sched_1"}
        if _, err := h.service.CancelRenewal(context.Background(), CancelRequest{WorkspaceID: "ws_1", GrantID: "trial_1"}); err != nil { t.Fatal(err) }
        if h.stripe.cancelSubscriptionCalls != tc.wantSubscription || h.stripe.cancelScheduleCalls != tc.wantSchedule { t.Fatalf("kind=%s stripe=%#v", tc.kind, h.stripe) }
    }
}
```

- [ ] **Step 2: Run and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/trials -run 'Test(ChangePlan|CancelRenewal)' -count=1`

Expected: FAIL because the operations are missing.

- [ ] **Step 3: Implement single-mutation operations and handlers**

Add owner-only `POST /v1/billing/change-plan` and `POST /v1/billing/trials/{trialID}/cancel-renewal`. Do not mark `superseded` until webhook projection confirms the new target price. Cancellation records intent/time but leaves access active until Stripe terminal events.

- [ ] **Step 4: Write and implement Portal configuration test**

Assert scheduled/active grants set `BillingPortalSessionParams.Configuration` to the mode's trial-safe ID, while ordinary subscriptions omit it. Missing trial-safe config for an open managed trial returns a configuration error rather than exposing the default mutable Portal.

- [ ] **Step 5: Run tests GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/trials ./internal/handler ./internal/billing -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/trials api/internal/handler api/internal/billing api/cmd/api/main.go
git commit -m "feat: manage trial plan changes and cancellation"
```

### Task 8: Add trial-ending lifecycle email

**Files:**
- Modify: `api/internal/emailregistry/registry.go`
- Modify: `api/internal/emailregistry/registry_test.go`
- Modify: `api/internal/handler/loops_lifecycle.go`
- Modify: `api/internal/handler/loops_lifecycle_test.go`
- Modify: `api/internal/handler/stripe_webhook.go`
- Modify: `api/cmd/api/main.go`
- Modify: `docs/email-templates.md`

- [ ] **Step 1: Add the failing registry contract**

Add `email.billing.trial_ending.v1 -> LOOPS_BILLING_TRIAL_ENDING_TRANSACTIONAL_ID` to the expected registry map and assert essential billing, critical transactional, non-unsubscribe-gated policy.

- [ ] **Step 2: Run and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/emailregistry -run Trial -count=1`

Expected: FAIL because the event is absent.

- [ ] **Step 3: Register and build the event**

```go
func buildLoopsBillingTrialEndingEvent(owner db.User, workspace db.Workspace, grant db.WorkspaceTrialGrant, priceCents int64, appBaseURL string) loops.LifecycleEvent {
    return loops.LifecycleEvent{
        UserID: owner.ID, Email: owner.Email, WorkspaceID: workspace.ID,
        EventName: "billing_trial_ending",
        IdempotencyKey: fmt.Sprintf("billing_trial_ending:%s:%s", grant.ID, grant.EndsAt.Time.UTC().Format(time.RFC3339)),
        Properties: map[string]any{
            "workspace_name": workspace.Name, "plan_id": grant.PlanID,
            "trial_end": grant.EndsAt.Time.UTC().Format(time.RFC3339),
            "billing_url": normalizeAppBaseURL(appBaseURL)+"/settings/billing",
            "cancel_url": normalizeAppBaseURL(appBaseURL)+"/settings/billing",
        },
    }
}
```

Include `plan_name`, `days_remaining`, and formatted `post_trial_price`. Send on `customer.subscription.trial_will_end`; for 1–3 day trials call the same idempotent sender immediately after activation.

- [ ] **Step 4: Test duplicate delivery and Loops failure acknowledgment**

Assert one idempotency key per grant/end time and webhook HTTP 200 even if Loops returns an error.

- [ ] **Step 5: Run tests and commit**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/emailregistry ./internal/handler -run 'Test.*Trial.*(Email|Ending|Registry)' -count=1`

Expected: PASS.

```bash
git add api/internal/emailregistry api/internal/handler api/cmd/api/main.go docs/email-templates.md
git commit -m "feat: send trial ending billing email"
```

### Task 9: Expose billing projection, history, and Admin Billing data

**Files:**
- Modify: `api/internal/handler/billing.go`
- Modify: `api/internal/handler/billing_test.go`
- Modify: `api/internal/handler/admin.go`
- Modify: `api/internal/handler/admin_test.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write failing projection tests**

Assert `GET /v1/billing` returns `trial` for pending/checkout_pending/scheduled/active, including dates, post-trial price, cancellation, and forfeiture warning. Assert `GET /v1/billing/trials` is newest-first and excludes `granted_by_user_id`, `failure_code`, and `failure_message`.

- [ ] **Step 2: Run and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'Test(GetBillingTrial|ListTrialHistory|AdminBillingTrial)' -count=1`

Expected: FAIL because the projections are absent.

- [ ] **Step 3: Implement user projections and route**

Add `Trial *trials.TrialProjection` to `billingResponse`, implement `GET /v1/billing/trials`, and keep billing reads available to all workspace roles.

- [ ] **Step 4: Extend Admin Billing query and row**

Use a lateral join selecting the open/latest grant. Return ID, kind, plan, duration, status, dates, schedule/subscription IDs, and safe provisioning failure. Preserve MRR calculation as `subscriptions.status='active'`; trialing rows are not revenue.

- [ ] **Step 5: Run tests and commit**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'Test(GetBillingTrial|ListTrialHistory|AdminBillingTrial)' -count=1`

Expected: PASS.

```bash
git add api/internal/handler api/cmd/api/main.go
git commit -m "feat: expose trial billing projections"
```

### Task 10: Add typed frontend contracts and shared formatting

**Files:**
- Create: `dashboard/tests/free-trial-contract-source.test.mjs`
- Create: `dashboard/src/lib/trial-format.ts`
- Modify: `dashboard/src/lib/api.ts`

- [ ] **Step 1: Write the failing source-contract test**

```js
test("billing API exposes managed trial contracts", () => {
  const source = readFileSync("src/lib/api.ts", "utf8");
  for (const value of [
    "WorkspaceTrial", "checkout_pending", "getTrialHistory",
    "grantAdminTrial", "revokeAdminTrial", "cancelTrialRenewal", "changeTrialPlan",
  ]) assert.match(source, new RegExp(value));
});
```

- [ ] **Step 2: Run and verify RED**

Run: `cd dashboard && node --test tests/free-trial-contract-source.test.mjs`

Expected: FAIL because the contracts do not exist.

- [ ] **Step 3: Add API types/functions and formatter**

```ts
export type TrialStatus = "provisioning"|"pending_activation"|"checkout_pending"|"scheduled"|"active"|"completed"|"canceled"|"revoked"|"superseded"|"failed";
export interface WorkspaceTrial {
  id: string; kind: "free_to_paid"|"paid_same_plan"; plan_id: string;
  duration_days: number; status: TrialStatus; scheduled_start_at?: string;
  started_at?: string; ends_at?: string; canceled_at?: string;
  post_trial_price_cents: number; changing_plan_forfeits_trial: boolean;
}
```

Add typed GET/history/admin/mutation functions. In `trial-format.ts`, export one formatter used by both pages for badge, headline, timeline, remaining days, and terminal reason.

- [ ] **Step 4: Run test and TypeScript build**

Run: `cd dashboard && node --test tests/free-trial-contract-source.test.mjs && npm run build`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add dashboard/src/lib dashboard/tests/free-trial-contract-source.test.mjs
git commit -m "feat: add frontend trial contracts"
```

### Task 11: Implement Admin Billing grant UI

**Files:**
- Modify: `dashboard/tests/free-trial-contract-source.test.mjs`
- Modify: `dashboard/src/app/admin/billing/page.tsx`

- [ ] **Step 1: Extend the failing UI contract**

Assert source contains `Grant Trial`, plan select for Free only, integer days input with min 1/max 730, confirmation timelines, revoke action, loading/error state, and pending/scheduled/active/failed badges.

- [ ] **Step 2: Run and verify RED**

Run: `cd dashboard && node --test tests/free-trial-contract-source.test.mjs`

Expected: FAIL because Admin Billing has only PlanFlipMenu.

- [ ] **Step 3: Implement the admin interaction**

Add a compact row-level `GrantTrialForm`. Free rows select API/Basic/Growth/Team; paid rows show the current plan read-only. Validate before sending, render the calculated timeline, disable duplicates, show inline failures, and expose Revoke for pending/checkout-pending grants.

- [ ] **Step 4: Run contract and build**

Run: `cd dashboard && node --test tests/free-trial-contract-source.test.mjs && npm run build`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add dashboard/src/app/admin/billing/page.tsx dashboard/tests/free-trial-contract-source.test.mjs
git commit -m "feat: add admin trial controls"
```

### Task 12: Implement Pricing and Billing trial UX/history

**Files:**
- Modify: `dashboard/tests/free-trial-contract-source.test.mjs`
- Modify: `dashboard/src/app/pricing/pricing-page-client.tsx`
- Modify: `dashboard/src/app/(dashboard)/settings/billing/page.tsx`
- Modify: `dashboard/tests/regression/dashboard.spec.ts`
- Modify: `dashboard/tests/regression/mobile-layout.spec.ts`

- [ ] **Step 1: Extend failing source and rendered contracts**

Cover the matching card only, `Start N-day free trial`, `Activation in progress`, `Continue checkout`, scheduled/active dates, forfeiture warning on other plans, cancel renewal, FAQ correction, and newest-first Trial History with empty/loading/error/mobile states.

- [ ] **Step 2: Run and verify RED**

Run: `cd dashboard && node --test tests/free-trial-contract-source.test.mjs`

Expected: FAIL because neither page renders grant state.

- [ ] **Step 3: Implement Pricing with shared formatter**

Store the full `billing.data.trial`, decorate only its matching tier, reuse ordinary Checkout for activation/resume, and require confirmation before other plan actions while scheduled/active.

- [ ] **Step 4: Implement Settings → Billing and Trial History**

Load billing and history together, render the current timeline near subscription state, keep upgrade/downgrade available, route cancellation through the trial-aware endpoint, and add the responsive history section below existing billing content.

- [ ] **Step 5: Run source tests, build, and local regression**

Run: `cd dashboard && node --test tests/free-trial-contract-source.test.mjs && npm run build && npm run test:regression:dashboard`

Expected: PASS with no failed/skipped/missing tests.

- [ ] **Step 6: Commit**

```bash
git add dashboard/src/app dashboard/tests
git commit -m "feat: show managed trials in billing surfaces"
```

### Task 13: Full local verification and documentation audit

**Files:**
- Modify only files required by failures found in this task.

- [ ] **Step 1: Verify the exact worktree and branch**

Run: `pwd && git branch --show-current && git status --short`

Expected: the conversation-owned worktree and `dev-admin-configurable-free-trial`.

- [ ] **Step 2: Run complete backend CI-equivalent tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./...`

Expected: PASS with zero failed, skipped, timed-out, or missing suites.

- [ ] **Step 3: Run frontend build and full dashboard regression**

Run: `cd dashboard && npm run build && npm run test:regression:dashboard`

Expected: PASS with zero failed, skipped, timed-out, or missing tests.

- [ ] **Step 4: Audit implementation against all 17 PRD acceptance criteria**

Use `rg` and test names to map every criterion to code plus automated evidence. Verify no runtime caller made `subscriptions.trial_used` authoritative and no default Portal mutation path remains for open managed trials.

- [ ] **Step 5: Check content scope and commit final fixes**

Run: `git diff origin/staging...HEAD --stat && git diff origin/staging...HEAD --name-only && git log --oneline origin/staging..HEAD`

Expected: only PRD, plan, trial implementation, tests, and required docs.

```bash
git add <only verified task files>
git commit -m "test: verify configurable free trials"
```

### Task 14: Draft PR and Preview Acceptance

**Files:** None unless a failing gate requires a source fix.

- [ ] **Step 1: Push only the owned branch and open a Draft PR to `dev`**

Before push, list unique commits/files again. Push `dev-admin-configurable-free-trial` and create a Draft PR targeting `dev`; do not merge.

- [ ] **Step 2: Monitor exact-head checks**

Wait for GitHub CI, Railway PR Environment, Vercel Preview wired to the PR API, and deployed regression on the exact PR head SHA. Any failure/timeout/cancel/skip/missing result is a hard stop.

- [ ] **Step 3: Perform browser acceptance**

Using the isolated Preview and Stripe Sandbox/Test Clock, verify Free activation from Pricing and Billing, paid scheduling, active/scheduled plan changes, both cancellation kinds, reminder audit, revoke race, Portal isolation, and Trial History.

- [ ] **Step 4: Stop for user acceptance before integration**

Report the Draft PR, exact SHA, Preview URLs, check results, and acceptance evidence. Do not merge into `dev`, `staging`, or `main` without the repository-required acceptance and user direction.
