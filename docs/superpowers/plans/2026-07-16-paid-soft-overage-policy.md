# Paid Soft Overage Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep immediate publishing soft for API, Basic, and Growth plans while atomically preventing net-new scheduled quota above 100%, placing downgrade overflow on quota hold, and delivering deterministic paid-quota notifications at 80/90/100/105/110/115/120%.

**Architecture:** Extend `quota.Checker` into the single monthly snapshot source, then add a transaction-owning `paidquota` coordinator that locks each workspace/month before schedule mutations. Paid notifications use their own durable decision ledger and asynchronous worker, while existing Loops/email-policy/audit infrastructure handles delivery and the Dashboard consumes additive usage and hold fields.

**Tech Stack:** Go 1.25, PostgreSQL/pgx/sqlc/goose, Loops transactional email, Next.js/React/TypeScript, Vitest, Playwright, GitHub Actions, Railway, Vercel.

---

## File map

- `api/internal/db/migrations/105_paid_soft_overage.sql`: quota-hold columns, paid notification ledger, follow-up table, indexes, and rollback.
- `api/internal/db/queries/paid_plan_quota_notifications.sql`: decision insertion, superseding, worker claiming, retry/final-state updates, reconciliation candidates, and Admin retry.
- `api/internal/db/queries/paid_quota_follow_ups.sql`: idempotent 120% follow-up creation and listing.
- `api/internal/db/queries/social_posts.sql`: transaction-safe hold/release/revalidation mutations and scheduler exclusion.
- `api/internal/db/social_posts_ext.go`: authoritative committed scheduled-unit snapshot, including scheduled-origin publishing and quota hold.
- `api/internal/quota/checker.go`: plan-agnostic `MonthlySnapshot`, exact percentage/threshold arithmetic, Free compatibility wrappers, and usage warnings.
- `api/internal/quota/checker_test.go`: shared snapshot and projected-usage unit tests.
- `api/internal/paidquota/policy.go`: eligible-plan rules, admission request/decision types, sorted period locks, transaction boundary, and fail-closed behavior.
- `api/internal/paidquota/policy_test.go`: exact-100%, over-cap, cross-month release/reserve, unlimited/excluded-plan, and rollback tests.
- `api/internal/paidquota/postgres.go`: pgx transaction implementation and committed-unit query adapter.
- `api/internal/paidquota/holds.go`: deterministic downgrade hold/release reconciliation.
- `api/internal/paidquota/holds_test.go`: grandfathering, ordering, parent atomicity, past-due hold, upgrade/reset release tests.
- `api/internal/paidquotaemail/service.go`: threshold decision creation, preference classification, highest-crossed selection, and follow-up creation.
- `api/internal/paidquotaemail/service_test.go`: seven thresholds, superseded decisions, preference skips, missing recipient, no rejected-projection email, and idempotency tests.
- `api/internal/paidquotaemail/postgres.go`: notification snapshot, durable ledger, owner lookup, and follow-up persistence.
- `api/internal/worker/paid_plan_quota_email.go`: lease-based asynchronous delivery, retry schedule, daily reconciliation, and one-time rollout reconciliation.
- `api/internal/worker/paid_plan_quota_email_test.go`: lease/retry/final-state/audit tests.
- `api/internal/emailregistry/registry.go`: paid warning and required-alert event registrations.
- `api/internal/emailregistry/registry_test.go`: registry contract and shared template environment assertions.
- `api/internal/emailpolicy/service_test.go`: warning preference gating and required-alert bypass tests.
- `api/internal/handler/social_posts.go`: schedule-create admission, paid error headers/body, and mutation-triggered evaluation.
- `api/internal/handler/social_posts_drafts.go`: atomic reschedule/content/destination edits, cancellation release, and hold response fields/actions.
- `api/internal/handler/social_posts_quota_test.go`: create/error/header/exact-cap tests.
- `api/internal/handler/social_posts_drafts_test.go`: cross-month, destination changes, hold, cancellation, and reschedule contract tests.
- `api/internal/handler/social_post_queue.go`: retain scheduled reservation while publishing and ensure quota-hold rows never dispatch.
- `api/internal/handler/social_post_queue_test.go`: publishing-reservation and hold-exclusion tests.
- `api/internal/handler/stripe_webhook.go`: invoke hold reconciliation when a downgrade actually becomes effective and release reconciliation on upgrade.
- `api/internal/handler/stripe_webhook_test.go`: period-end timing and revalidation hook tests.
- `api/internal/handler/billing.go`: additive usage/billing response fields.
- `api/internal/handler/billing_test.go`: usage JSON contract and fractional percentage tests.
- `api/internal/handler/admin.go`: paid ledger/follow-up reporting and retry endpoint.
- `api/internal/handler/admin_test.go`: Admin union/filter/status/retry coverage.
- `api/cmd/api/main.go`: construct coordinator/service/worker, start worker, and inject dependencies.
- `dashboard/src/lib/api.ts`: usage, hold, paid email, and Admin follow-up types/API helpers.
- `dashboard/src/app/(dashboard)/settings/billing/page.tsx`: completed/scheduled/held/effective breakdown and warning/alert recovery guidance.
- `dashboard/src/components/posts/calendar/posts-calendar-view.tsx`: distinct quota-hold state and allowed actions.
- `dashboard/src/app/admin/email/page.tsx`: new thresholds/statuses and paid quota audit rendering.
- `dashboard/src/app/admin/paid-quota/page.tsx`: administrative 120% follow-up queue.
- `dashboard/src/app/admin/layout.tsx`: Admin navigation entry for paid quota follow-up.
- `dashboard/src/app/pricing/pricing-page-client.tsx`: scheduling-circuit-breaker wording.
- `dashboard/src/**/*.test.ts(x)`: API formatting and component behavior tests.
- `docs/api-reference.md`: new 402 error, headers, usage fields, and quota-hold semantics.
- `docs/superpowers/specs/2026-07-16-paid-soft-overage-policy-design.md`: remains the product contract and is not weakened during implementation.

### Task 1: Add durable schema and generated database access

**Files:**
- Create: `api/internal/db/migrations/105_paid_soft_overage.sql`
- Create: `api/internal/db/queries/paid_plan_quota_notifications.sql`
- Create: `api/internal/db/queries/paid_quota_follow_ups.sql`
- Modify: `api/internal/db/queries/social_posts.sql`
- Modify: `api/internal/db/migrate_test.go`
- Generated: `api/internal/db/*.sql.go`, `api/internal/db/models.go`, `api/internal/db/querier.go`

- [ ] **Step 1: Write the migration contract test**

Add `TestPaidSoftOverageMigrationExists` to `api/internal/db/migrate_test.go`. Read migration 105 and assert it contains:

```go
for _, required := range []string{
    "quota_hold_reason",
    "quota_hold_at",
    "quota_hold_original_scheduled_at",
    "paid_plan_quota_notifications",
    "skipped_superseded",
    "retry_wait",
    "paid_quota_follow_ups",
    "UNIQUE (workspace_id, period, threshold_percent)",
} {
    if !strings.Contains(sql, required) {
        t.Fatalf("migration 105 missing %q", required)
    }
}
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run TestPaidSoftOverageMigrationExists -count=1`

Expected: FAIL because migration 105 does not exist.

- [ ] **Step 3: Add migration 105**

Create:

```sql
-- +goose Up
ALTER TABLE social_posts
  ADD COLUMN quota_hold_reason TEXT,
  ADD COLUMN quota_hold_at TIMESTAMPTZ,
  ADD COLUMN quota_hold_original_scheduled_at TIMESTAMPTZ;

CREATE TABLE paid_plan_quota_notifications (
  id TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
  email TEXT,
  period TEXT NOT NULL,
  threshold_percent INTEGER NOT NULL CHECK (threshold_percent IN (80, 90, 100, 105, 110, 115, 120)),
  event_key TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
    'pending', 'processing', 'sent', 'retry_wait', 'failed',
    'skipped_superseded', 'skipped_preference_disabled', 'skipped_missing_recipient'
  )),
  transactional_id TEXT,
  idempotency_key TEXT NOT NULL,
  completed_usage INTEGER NOT NULL,
  scheduled_usage INTEGER NOT NULL,
  quota_hold_usage INTEGER NOT NULL,
  effective_usage INTEGER NOT NULL,
  post_limit INTEGER NOT NULL,
  attempt_count INTEGER NOT NULL DEFAULT 0,
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  lease_expires_at TIMESTAMPTZ,
  last_error TEXT,
  attempted_at TIMESTAMPTZ,
  sent_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (workspace_id, period, threshold_percent),
  UNIQUE (idempotency_key)
);

CREATE INDEX paid_plan_quota_notifications_worker_idx
  ON paid_plan_quota_notifications (status, next_attempt_at, lease_expires_at);
CREATE INDEX paid_plan_quota_notifications_admin_idx
  ON paid_plan_quota_notifications (period, threshold_percent, created_at DESC);

CREATE TABLE paid_quota_follow_ups (
  id TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  period TEXT NOT NULL,
  threshold_percent INTEGER NOT NULL DEFAULT 120 CHECK (threshold_percent = 120),
  notification_id TEXT REFERENCES paid_plan_quota_notifications(id) ON DELETE SET NULL,
  status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'contacted', 'resolved', 'dismissed')),
  completed_usage INTEGER NOT NULL,
  scheduled_usage INTEGER NOT NULL,
  quota_hold_usage INTEGER NOT NULL,
  effective_usage INTEGER NOT NULL,
  post_limit INTEGER NOT NULL,
  assignee_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (workspace_id, period, threshold_percent)
);

CREATE INDEX paid_quota_follow_ups_status_idx
  ON paid_quota_follow_ups (status, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS paid_quota_follow_ups;
DROP TABLE IF EXISTS paid_plan_quota_notifications;
ALTER TABLE social_posts
  DROP COLUMN IF EXISTS quota_hold_original_scheduled_at,
  DROP COLUMN IF EXISTS quota_hold_at,
  DROP COLUMN IF EXISTS quota_hold_reason;
```

- [ ] **Step 4: Add exact sqlc queries**

Add queries for:

```sql
-- name: InsertPaidPlanQuotaNotificationDecision :one
INSERT INTO paid_plan_quota_notifications (
  workspace_id,
  user_id,
  email,
  period,
  threshold_percent,
  event_key,
  status,
  transactional_id,
  idempotency_key,
  completed_usage,
  scheduled_usage,
  quota_hold_usage,
  effective_usage,
  post_limit
)
VALUES (
  sqlc.arg(workspace_id),
  sqlc.narg(user_id),
  sqlc.narg(email),
  sqlc.arg(period),
  sqlc.arg(threshold_percent),
  sqlc.arg(event_key),
  sqlc.arg(status),
  sqlc.narg(transactional_id),
  sqlc.arg(idempotency_key),
  sqlc.arg(completed_usage),
  sqlc.arg(scheduled_usage),
  sqlc.arg(quota_hold_usage),
  sqlc.arg(effective_usage),
  sqlc.arg(post_limit)
)
ON CONFLICT (workspace_id, period, threshold_percent) DO NOTHING
RETURNING *;

-- name: MarkLowerPaidQuotaNotificationsSuperseded :exec
UPDATE paid_plan_quota_notifications
SET status = 'skipped_superseded', updated_at = NOW()
WHERE workspace_id = $1 AND period = $2
  AND threshold_percent < $3
  AND status IN ('pending', 'retry_wait');

-- name: ClaimPaidPlanQuotaNotifications :many
WITH candidates AS (
  SELECT id
  FROM paid_plan_quota_notifications
  WHERE status IN ('pending', 'retry_wait')
    AND next_attempt_at <= NOW()
    AND (lease_expires_at IS NULL OR lease_expires_at < NOW())
  ORDER BY next_attempt_at, created_at, id
  FOR UPDATE SKIP LOCKED
  LIMIT $1
)
UPDATE paid_plan_quota_notifications n
SET status = 'processing',
    attempt_count = attempt_count + 1,
    attempted_at = NOW(),
    lease_expires_at = NOW() + INTERVAL '5 minutes',
    updated_at = NOW()
FROM candidates
WHERE n.id = candidates.id
RETURNING n.*;
```

Also add final-state updates for `sent`, `retry_wait`, `failed`, preference/missing-recipient skips, Admin retry, period threshold listing, reconciliation candidate listing, idempotent `InsertPaidQuotaFollowUp`, and list/update follow-up queries. Add `SetSocialPostQuotaHold`, `ReleaseSocialPostQuotaHold`, and ordered future scheduled-post listing with `ORDER BY scheduled_at, created_at, id`.

- [ ] **Step 5: Generate database code**

Run: `cd api && go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate`

Expected: generated Go files compile with migration 105 and all new queries.

- [ ] **Step 6: Run DB package tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/internal/db
git commit -m "feat: add paid quota persistence"
```

### Task 2: Make the shared monthly snapshot authoritative

**Files:**
- Modify: `api/internal/db/social_posts_ext.go`
- Modify: `api/internal/quota/checker.go`
- Modify: `api/internal/quota/checker_test.go`

- [ ] **Step 1: Write failing shared-snapshot tests**

Add tests asserting:

```go
snapshot := MonthlySnapshot{
    Completed: 98,
    Scheduled: 1,
    QuotaHold: 1,
    Limit: 100,
}
if snapshot.EffectiveUsage() != 100 { t.Fatal("want effective usage 100") }
if snapshot.WouldExceed(0, 1) != true { t.Fatal("101 must exceed") }
if snapshot.WouldExceed(1, 1) != false { t.Fatal("release and reserve must remain at 100") }
if !snapshot.Reached(100) || snapshot.Reached(105) { t.Fatal("exact arithmetic mismatch") }
```

Add a fake-query test confirming the checker exposes completed, total committed scheduled, and held subset for an arbitrary UTC period.

- [ ] **Step 2: Run tests and confirm failure**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/quota -count=1`

Expected: FAIL because `MonthlySnapshot` does not exist.

- [ ] **Step 3: Update the committed-unit query**

Change `CountScheduledQuotaUnitsByWorkspaceAndPeriod` to count:

```sql
AND (
  sp.status IN ('scheduled', 'quota_hold')
  OR (sp.status = 'publishing' AND sp.scheduled_at IS NOT NULL)
)
```

Add a second query/method for the `quota_hold` subset. Preserve disconnected-account filtering and the `admin_post_quota_resets` scheduled baseline.

- [ ] **Step 4: Implement shared snapshot primitives**

Add:

```go
type MonthlySnapshot struct {
    WorkspaceID string
    PlanID string
    Period string
    Completed int
    Scheduled int
    QuotaHold int
    Limit int
}

func (s MonthlySnapshot) EffectiveUsage() int { return s.Completed + s.Scheduled }
func (s MonthlySnapshot) EffectivePercentage() float64 {
    if s.Limit <= 0 { return 0 }
    return float64(s.EffectiveUsage()) / float64(s.Limit) * 100
}
func (s MonthlySnapshot) Reached(threshold int) bool {
    return s.Limit > 0 && s.EffectiveUsage()*100 >= threshold*s.Limit
}
func (s MonthlySnapshot) WouldExceed(released, requested int) bool {
    return s.Limit >= 0 && s.EffectiveUsage()-released+requested > s.Limit
}
```

Implement `Checker.MonthlySnapshotForPeriod`, and make `CheckForPeriod` plus Free hard-block wrappers derive from it. For unlimited plans use `Limit = -1`. Snapshot-read errors must be returned by the new strict method; legacy display methods may retain their existing safe fallback.

- [ ] **Step 5: Run quota tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/quota -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/db/social_posts_ext.go api/internal/quota
git commit -m "refactor: centralize monthly quota snapshots"
```

### Task 3: Add atomic paid schedule admission

**Files:**
- Create: `api/internal/paidquota/policy.go`
- Create: `api/internal/paidquota/policy_test.go`
- Create: `api/internal/paidquota/postgres.go`
- Modify: `api/internal/handler/social_posts.go`
- Modify: `api/internal/handler/social_posts_quota_test.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write failing policy tests**

Cover:

```go
tests := []struct{
    name string
    plan string
    current, released, requested, limit int
    allowed bool
}{
    {"exactly 100 allowed", "basic", 2499, 0, 1, 2500, true},
    {"over 100 rejected", "basic", 2500, 0, 1, 2500, false},
    {"atomic replacement allowed", "basic", 2500, 2, 2, 2500, true},
    {"api included", "api", 999, 0, 1, 1000, true},
    {"growth included", "growth", 7500, 0, 1, 7500, false},
    {"team excluded", "team", 999999, 0, 10, -1, true},
    {"enterprise excluded", "enterprise", 999999, 0, 10, 1000, true},
    {"free delegated", "free", 100, 0, 1, 100, true},
}
```

Also assert two affected periods are lock-sorted and a mutation error rolls the transaction back.

- [ ] **Step 2: Run policy tests and confirm failure**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/paidquota -count=1`

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement the transaction-owning coordinator**

Define:

```go
type PeriodDelta struct {
    Period string
    ReleasedUnits int
    RequestedUnits int
}

type AdmissionError struct {
    Snapshot quota.MonthlySnapshot
    RequestedUnits int
}

type Coordinator interface {
    Mutate(ctx context.Context, workspaceID string, deltas []PeriodDelta, mutation func(*db.Queries) error) error
}
```

`PostgresCoordinator.Mutate` must:

1. Begin a `pgx.Tx`.
2. Normalize/dedupe periods and acquire `pg_advisory_xact_lock(hashtext(workspace_id), hashtext('paid_schedule_quota:' || period))` in ascending period order.
3. Bind `db.New(tx)`.
4. Load the current plan and fail closed for API/Basic/Growth if the plan or snapshot cannot be read.
5. Recalculate each period inside the transaction.
6. Return `AdmissionError` when `effective - released + requested > limit`.
7. Run the mutation callback and commit.

Free, Team, and Enterprise skip the paid limit test but still run the mutation. Existing Free-specific admission remains authoritative.

- [ ] **Step 4: Wire scheduled create**

Inject `paidquota.Coordinator` into `SocialPostHandler`. Replace the paid create path with one coordinator call using:

```go
deltas := []paidquota.PeriodDelta{{
    Period: quota.PeriodForTime(*parsed.ScheduledAt),
    RequestedUnits: countPublishQuotaUnits(parsed.Posts, accountMap),
}}
```

Inside the callback call `CreateSocialPost` or `CreateSocialPostWithActiveScheduledCap` using the transaction-bound queries. Do not run a paid pre-check outside the transaction.

- [ ] **Step 5: Add the 402 contract**

Map `AdmissionError` to:

```go
code := "PLAN_MONTHLY_SCHEDULING_CAPACITY_EXCEEDED"
normalized := "plan_monthly_scheduling_capacity_exceeded"
```

Set `X-UniPost-Usage`, `X-UniPost-Scheduled-Usage`, `X-UniPost-Quota-Hold-Usage`, `X-UniPost-Effective-Usage`, and `X-UniPost-Warning: scheduled_quota_reached`. Return the exact details fields from the spec, with `resets_at` equal to the first UTC instant of the next month.

- [ ] **Step 6: Run handler and policy tests**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/paidquota ./internal/handler -run 'Paid|MonthlyScheduling|CreateScheduled|Quota' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/internal/paidquota api/internal/handler/social_posts.go api/internal/handler/social_posts_quota_test.go api/cmd/api/main.go
git commit -m "feat: atomically cap paid scheduled quota"
```

### Task 4: Cover every schedule-increasing edit and close the publishing hole

**Files:**
- Modify: `api/internal/handler/social_posts_drafts.go`
- Modify: `api/internal/handler/social_posts_drafts_test.go`
- Modify: `api/internal/handler/social_post_queue.go`
- Modify: `api/internal/handler/social_post_queue_test.go`
- Modify: `api/internal/db/queries/social_posts.sql`

- [ ] **Step 1: Write failing edit-path tests**

Add handler tests for:

- scheduled destination 1→2 at 100% returns 402 and leaves metadata unchanged;
- 2→1 is allowed at 100%;
- July→August locks/releases July and reserves August atomically;
- moving into an over-cap month returns 402 and preserves the July timestamp;
- copy/media-only edit at 100% is allowed;
- cancellation is allowed and releases committed units;
- a scheduled-origin row in `publishing` remains counted;
- a `quota_hold` row is never returned by `GetDueScheduledPosts` and cannot be claimed.

- [ ] **Step 2: Run targeted tests and confirm failure**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'ScheduledEdit|Reschedule|QuotaHold|PublishingReservation' -count=1
```

Expected: FAIL on missing atomic paid-policy wiring.

- [ ] **Step 3: Calculate old and new deltas from persisted metadata**

For scheduled edits, decode the existing and proposed platform posts using the same account snapshot. Calculate connected-account units. Use:

```go
oldPeriod := quota.PeriodForTime(existing.ScheduledAt.Time)
newPeriod := quota.PeriodForTime(*scheduledAt)
```

If periods match, submit one delta with `ReleasedUnits=oldUnits` and `RequestedUnits=newUnits`. If they differ, submit one release-only old-period delta and one reserve-only new-period delta.

- [ ] **Step 4: Run each mutation inside the coordinator transaction**

Move `RescheduleSocialPost`, `UpdateDraftContent`, cancellation, deletion, and scheduled destination changes to transaction-bound queries. Release-only actions must remain available even if the workspace is already over quota. Keep immediate draft publishing on existing paid soft-overage behavior.

- [ ] **Step 5: Preserve reservation during scheduler execution**

Keep `scheduled_at` non-null when `ClaimScheduledPost` changes `scheduled`→`publishing`. Ensure the snapshot query counts it until terminal status. Scheduler queries remain `status='scheduled'`, so quota-hold rows cannot execute.

- [ ] **Step 6: Run handler tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/internal/handler api/internal/db/queries/social_posts.sql api/internal/db
git commit -m "feat: enforce paid quota across schedule edits"
```

### Task 5: Implement downgrade quota hold and release reconciliation

**Files:**
- Create: `api/internal/paidquota/holds.go`
- Create: `api/internal/paidquota/holds_test.go`
- Modify: `api/internal/paidquota/postgres.go`
- Modify: `api/internal/handler/stripe_webhook.go`
- Modify: `api/internal/handler/stripe_webhook_test.go`
- Modify: `api/internal/handler/social_posts_drafts.go`

- [ ] **Step 1: Write failing deterministic hold tests**

Use a fixture with completed usage, scheduled-origin publishing rows, grandfathered scheduled rows, and later scheduled rows. Assert:

```go
// Capacity is consumed in this order:
// 1. completed usage
// 2. scheduled-origin publishing
// 3. scheduled rows created before effectiveAt
// 4. remaining parents ordered scheduled_at, created_at, id
```

Assert a multi-platform parent is all scheduled or all held, hold units still count in effective usage, upgrade/cancel/destination removal/month reset can release holds, and a past-due hold stays held.

- [ ] **Step 2: Run tests and confirm failure**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/paidquota -run Hold -count=1`

Expected: FAIL because hold reconciliation does not exist.

- [ ] **Step 3: Implement `HoldReconciler`**

Define:

```go
type HoldReconciler interface {
    ReconcileWorkspace(ctx context.Context, workspaceID, reason string, effectiveAt time.Time) error
}
```

For each UTC month in `[effectiveAt, effectiveAt+90 days]`, acquire the same workspace/month advisory lock, load the new plan limit, consume capacity in the approved order, and set non-fitting parent rows to:

```sql
status = 'quota_hold',
quota_hold_reason = $reason,
quota_hold_at = NOW(),
quota_hold_original_scheduled_at = COALESCE(quota_hold_original_scheduled_at, scheduled_at)
```

Release a future hold only when the whole parent fits. For past-due holds, preserve `quota_hold` and expose “reschedule or publish now”.

- [ ] **Step 4: Hook effective plan changes**

In `handleSubscriptionUpdated`, call reconciliation only after `UpdateSubscriptionStripe` applies a lower plan. Preserve the existing period-end downgrade logic: if `shouldKeepCurrentPlanForDowngrade` returns true, no hold runs yet. On a real upgrade, run release reconciliation.

- [ ] **Step 5: Expose hold actions**

Allow held rows to be cancelled/deleted, edited to remove destinations, rescheduled into an available future month, or explicitly published now. Publishing now uses immediate soft-overage and clears hold fields without silently publishing late.

- [ ] **Step 6: Run paidquota and Stripe tests**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/paidquota ./internal/handler -run 'Hold|Downgrade|Upgrade|Stripe' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/internal/paidquota api/internal/handler/stripe_webhook* api/internal/handler/social_posts_drafts.go
git commit -m "feat: hold excess schedules after downgrades"
```

### Task 6: Create deterministic paid quota decisions

**Files:**
- Create: `api/internal/paidquotaemail/service.go`
- Create: `api/internal/paidquotaemail/service_test.go`
- Create: `api/internal/paidquotaemail/postgres.go`
- Modify: `api/internal/emailregistry/registry.go`
- Modify: `api/internal/emailregistry/registry_test.go`
- Modify: `api/internal/emailpolicy/service_test.go`

- [ ] **Step 1: Write failing threshold tests**

Assert exact integer threshold behavior for 80, 90, 100, 105, 110, 115, and 120. Assert crossing 90→111 creates only 110 as deliverable and records 100/105 as `skipped_superseded`; a repeated evaluation creates no duplicate final decisions; a rejected projected schedule does not evaluate; 80/90 can skip on preference; 100+ cannot.

- [ ] **Step 2: Run tests and confirm failure**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/paidquotaemail ./internal/emailregistry ./internal/emailpolicy -count=1`

Expected: FAIL because paid quota email service/events do not exist.

- [ ] **Step 3: Register two events**

Add:

```go
email.quota.paid_plan_warning.v1
```

with `PreferenceGated: true`, `UsageQuotaAlerts`, `FooterManagePreferences`, and:

```go
email.quota.paid_plan_alert.v1
```

with `PreferenceGated: false`, `FooterRequiredNotice`, and a required-service reason. Both resolve `LOOPS_PAID_PLAN_QUOTA_TRANSACTIONAL_ID`.

- [ ] **Step 4: Implement decision creation**

`Service.Evaluate(ctx, workspaceID, period)` loads the shared monthly snapshot. Eligibility is only `api`, `basic`, or `growth`, with finite positive limits. It finds the highest currently reached threshold that has no final decision, inserts lower newly crossed thresholds as `skipped_superseded`, and inserts the highest as:

- `pending` when deliverable;
- `skipped_preference_disabled` for optional warnings with disabled preference;
- `skipped_missing_recipient` when owner email is absent.

At 120%, insert `paid_quota_follow_ups` regardless of email delivery outcome.

- [ ] **Step 5: Run service/registry/policy tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/paidquotaemail ./internal/emailregistry ./internal/emailpolicy -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/paidquotaemail api/internal/emailregistry api/internal/emailpolicy
git commit -m "feat: decide paid quota notifications"
```

### Task 7: Deliver paid notifications asynchronously with retries and reconciliation

**Files:**
- Create: `api/internal/worker/paid_plan_quota_email.go`
- Create: `api/internal/worker/paid_plan_quota_email_test.go`
- Modify: `api/cmd/api/main.go`
- Modify: `api/internal/handler/social_posts.go`
- Modify: `api/internal/handler/social_posts_drafts.go`
- Modify: `api/internal/handler/social_post_queue.go`
- Modify: `api/internal/handler/stripe_webhook.go`

- [ ] **Step 1: Write failing worker tests**

Cover claim leasing, stable idempotency key, success→`sent`, failures→5m/1h/6h `retry_wait`, fourth failure→`failed`, abandoned processing lease reclaim, shared `email_send_attempts` audit, daily 00:15 UTC reconciliation, and a one-time rollout reconciliation guard.

- [ ] **Step 2: Run tests and confirm failure**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/worker -run PaidPlanQuota -count=1`

Expected: FAIL because the worker does not exist.

- [ ] **Step 3: Implement the worker**

Use a short polling ticker to claim durable rows and a UTC schedule for reconciliation. Retry delays are:

```go
var paidQuotaRetryDelays = []time.Duration{
    5 * time.Minute,
    1 * time.Hour,
    6 * time.Hour,
}
```

Send through the audited Loops sender and email-policy decision. Use `paid_plan_quota:{workspace}:{period}:{threshold}` as the provider idempotency key. Never recalculate a rejected projection.

- [ ] **Step 4: Trigger evaluation after committed state changes**

Queue evaluation after successful publish increments, scheduled create/edit/cancel/delete, hold/release, effective plan change, and administrative reset. Evaluation reads committed state after mutation; it is not part of the user transaction and cannot roll back a successful publish/schedule action.

- [ ] **Step 5: Start the worker**

In `api/cmd/api/main.go`, create the service whenever `LOOPS_PAID_PLAN_QUOTA_TRANSACTIONAL_ID` is configured, inject the evaluator into handlers, and start the worker with the existing `workerCtx`.

- [ ] **Step 6: Run worker and handler tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/worker ./internal/handler -run 'PaidPlanQuota|QuotaNotification' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/internal/worker api/internal/handler api/cmd/api/main.go
git commit -m "feat: deliver paid quota alerts asynchronously"
```

### Task 8: Extend usage, billing, Admin reporting, and retry controls

**Files:**
- Modify: `api/internal/handler/billing.go`
- Create or Modify: `api/internal/handler/billing_test.go`
- Modify: `api/internal/handler/admin.go`
- Modify: `api/internal/handler/admin_test.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write failing API contract tests**

Assert `GET /v1/usage` and `/v1/billing` include:

```json
{
  "scheduled_count": 12,
  "quota_hold_count": 2,
  "effective_usage": 2500,
  "effective_percentage": 100,
  "scheduling_allowed": false,
  "resets_at": "2026-08-01T00:00:00Z"
}
```

Preserve `post_count`/`usage`, `post_limit`/`limit`, `percentage`, plan, status, and existing fields. Test 2488/2500 serializes `percentage=99.52` while threshold decisions remain integer-exact.

- [ ] **Step 2: Write failing Admin tests**

Assert the email CTE includes both paid event keys, all new statuses, scheduled and hold usage, threshold filters through 120, and an Admin retry converts only `failed` paid decisions to `pending`. Assert follow-up listing/status updates are workspace/period scoped.

- [ ] **Step 3: Run tests and confirm failure**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'Usage|Billing|AdminEmail|PaidQuotaFollowUp' -count=1`

Expected: FAIL on missing fields and queries.

- [ ] **Step 4: Implement additive responses**

Derive both handlers from `MonthlySnapshot`. `scheduled_count` includes holds; `quota_hold_count` is the held subset. `scheduling_allowed` is false only for API/Basic/Growth when effective usage is at or above the finite limit. Use `approaching_limit` at 80–<100 and `scheduled_quota_reached` at ≥100 for paid plans.

- [ ] **Step 5: Extend Admin endpoints**

Union `paid_plan_quota_notifications` into `adminEmailNotificationsCTESQL`, preserve its exact status string, and add routes for:

```text
POST /v1/admin/email/paid-quota/{id}/retry
GET  /v1/admin/paid-quota-follow-ups
PATCH /v1/admin/paid-quota-follow-ups/{id}
```

- [ ] **Step 6: Run handler tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/internal/handler api/cmd/api/main.go
git commit -m "feat: expose paid quota usage and admin controls"
```

### Task 9: Update Dashboard usage, held posts, and Admin surfaces

**Required skill before edits:** `design-taste-frontend`

**Files:**
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/app/(dashboard)/settings/billing/page.tsx`
- Modify: `dashboard/src/components/posts/calendar/posts-calendar-view.tsx`
- Modify: `dashboard/src/app/admin/email/page.tsx`
- Create: `dashboard/src/app/admin/paid-quota/page.tsx`
- Modify: `dashboard/src/app/admin/layout.tsx`
- Modify: `dashboard/src/app/pricing/pricing-page-client.tsx`
- Test: relevant colocated tests and `dashboard/tests/regression`

- [ ] **Step 1: Read the frontend design skill**

Run: `sed -n '1,260p' /Users/xiaoboyu/.codex/skills/taste-skill/SKILL.md`

Expected: design constraints are understood before TSX/CSS edits.

- [ ] **Step 2: Write failing frontend tests**

Add tests for:

- effective usage totals and fractional percentage formatting;
- paid warning at 80/90;
- severe alert at ≥100 with immediate publishing still available;
- held post badge/copy and actions “Reschedule”, “Publish now”, “Cancel”;
- 402 error message recommends upgrade/cancel/wait and does not imply all publishing is blocked;
- Admin thresholds include 105/110/115/120 and statuses include processing/retry/final skips.

- [ ] **Step 3: Run tests and confirm failure**

Run: `cd dashboard && npm test -- --run`

Expected: FAIL on missing types/rendering.

- [ ] **Step 4: Extend API types**

Add explicit fields:

```ts
scheduled_count: number;
quota_hold_count: number;
effective_usage: number;
effective_percentage: number;
scheduling_allowed: boolean;
resets_at: string;
```

Add `quota_hold_reason`, `quota_hold_at`, and `quota_hold_original_scheduled_at` to post types, plus paid follow-up list/update helpers.

- [ ] **Step 5: Implement billing and scheduling UI**

Show four labeled values: completed, committed scheduled, held, effective/limit. Keep the visual hierarchy compact. At ≥100%, say:

```text
Monthly scheduling capacity reached. Existing scheduled posts and immediate publishing remain available. Upgrade, cancel scheduled work, or wait for the monthly reset to schedule more.
```

Held posts must be visually distinct from failed posts and explain that they will not auto-publish until released or rescheduled.

- [ ] **Step 6: Implement Admin pages**

Extend Email filters/status rendering and add the paid follow-up queue with workspace, period, effective usage, plan limit, age, status, assignee, and notes.

- [ ] **Step 7: Update pricing copy**

Replace the old generic paid soft-overage statement with explicit immediate-publish soft overage plus a 100% scheduling circuit breaker.

- [ ] **Step 8: Run frontend validation**

Run:

```bash
cd dashboard
npm test -- --run
npm run build
```

Expected: PASS.

If Playwright browsers are installed, also run: `npm run test:regression:dashboard`

- [ ] **Step 9: Commit**

```bash
git add dashboard
git commit -m "feat: show paid quota capacity and holds"
```

### Task 10: Document contracts and run full local verification

**Files:**
- Modify: `docs/api-reference.md`
- Modify: any environment-example documentation that enumerates Loops template IDs
- Modify: plan checkboxes in this file as tasks complete

- [ ] **Step 1: Update API documentation**

Document:

- API/Basic/Growth applicability;
- effective usage formula;
- exact 402 code and normalized code;
- all four quota headers plus warning values;
- additive usage fields;
- quota-hold lifecycle and allowed actions;
- immediate publishing soft-overage behavior;
- email thresholds and optional/required distinction;
- `LOOPS_PAID_PLAN_QUOTA_TRANSACTIONAL_ID`.

- [ ] **Step 2: Run formatting and generated-code checks**

Run:

```bash
cd api
gofmt -w internal/paidquota internal/paidquotaemail internal/worker internal/handler internal/quota
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate
git diff --check
```

Expected: no formatting or generated-code drift.

- [ ] **Step 3: Run backend CI-equivalent**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./...`

Expected: PASS.

- [ ] **Step 4: Run Dashboard CI-equivalent**

Run: `cd dashboard && npm run build`

Expected: PASS.

Run `npm run test:regression:dashboard` when Playwright browsers are installed.

- [ ] **Step 5: Self-review against the approved spec**

Check every acceptance criterion in `docs/superpowers/specs/2026-07-16-paid-soft-overage-policy-design.md`, search for duplicate quota arithmetic, verify no feature flag was added, and confirm no schedule-increasing paid write bypasses `paidquota.Coordinator`.

- [ ] **Step 6: Commit documentation and verification fixes**

```bash
git add docs api dashboard
git commit -m "docs: document paid soft overage policy"
```

### Task 11: Merge to dev, deploy, and verify the real development environment

**Files:** no new source files unless deployment verification finds a defect.

- [ ] **Step 1: Fetch and update local dev safely**

Run:

```bash
git status --short
git fetch origin
git switch dev
git pull --ff-only origin dev
git merge --no-ff dev-paid-soft-overage-policy
```

Expected: unrelated untracked user files remain untouched.

- [ ] **Step 2: Re-run required validation on updated local dev**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./...
cd ../dashboard && npm run build
```

Run Dashboard regression when browsers are installed.

- [ ] **Step 3: Push dev**

Run: `git push origin dev`

Expected: push succeeds and triggers GitHub, Railway dev, and Vercel dev checks/deployments.

- [ ] **Step 4: Monitor every triggered check**

Use GitHub/Vercel/Railway status and logs until no required or visible deployment is queued/running. If any check fails, fix on the correct source branch, rerun local validation, merge into dev again, push, and restart monitoring.

- [ ] **Step 5: Verify real dev behavior**

Against `https://dev-api.unipost.dev` and `https://dev-app.unipost.dev`, verify:

- usage response exposes all additive fields;
- an API/Basic/Growth workspace can schedule exactly to 100%;
- the next net-new scheduled unit returns the exact 402 contract;
- immediate publishing remains available;
- cancellation/removal releases capacity;
- a held post cannot dispatch and exposes recovery actions;
- paid quota email/Admin rows show the correct threshold/status.

Use controlled test workspace/data and clean up created posts after verification.

- [ ] **Step 6: Record dev evidence**

Capture request/response status, relevant response headers/body, deployment URLs/IDs, and the Dashboard state needed for staging comparison.

### Task 12: Promote through staging and production

**Files:** no new source files unless environment verification finds a defect.

- [ ] **Step 1: Create promotion PR dev→staging**

Before the PR, confirm backend tests and Dashboard build are still green. Create the PR from `dev` to `staging`, wait for checks, merge it, and monitor all staging deployments.

- [ ] **Step 2: Verify real staging**

Repeat the dev acceptance set against:

```text
https://staging-api.unipost.dev
https://staging-app.unipost.dev
```

Do not use production domains for staging validation.

- [ ] **Step 3: Create production PR staging→main**

Only after staging passes, create a PR from `staging` to `main`. Wait for all checks, merge, and monitor production deployments.

- [ ] **Step 4: Verify production health and critical flow**

Against:

```text
https://api.unipost.dev
https://app.unipost.dev
```

Verify health, additive usage compatibility, schedule admission, immediate publishing availability, and Admin/email observability using a controlled production-safe test. Do not mutate unrelated customer data.

- [ ] **Step 5: Final audit**

Confirm:

- `origin/dev`, `origin/staging`, and `origin/main` contain the intended commits;
- no check/deployment remains pending;
- production behavior matches the approved spec;
- the rollout reconciliation created at most the expected paid threshold decisions/follow-ups;
- unrelated local files remain unchanged.

## Plan self-review

- Spec coverage: monthly model, atomic admission, all schedule paths, publishing reservation, error/header/usage contracts, seven thresholds, optional vs required email policy, durable worker/retries, reconciliation, 120% follow-up, downgrade hold, Dashboard/Admin, observability, and standard release are each assigned above.
- Placeholder scan: no `TBD`, `TODO`, “implement later”, or unspecified “write tests” steps remain.
- Type consistency: `MonthlySnapshot`, `PeriodDelta`, `AdmissionError`, `Coordinator.Mutate`, `HoldReconciler`, paid event keys, status names, usage fields, and environment variable names match the approved design throughout.
