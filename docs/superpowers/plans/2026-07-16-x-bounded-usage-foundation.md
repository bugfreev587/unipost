# X Bounded Usage Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the approved Option B Phase 0–1 foundation to production: a versioned X Credits catalog, atomic monthly managed-X usage allowance, daily inbound protection storage, customer-visible usage surfaces, and phase-aware documentation without Top-up claims.

**Architecture:** Add an isolated `internal/xcredits` domain backed by append-only usage events and one period aggregate per workspace. Immediate and scheduled X publishing reserve weighted usage only after the existing 20/day account safety gate, then finalize or reverse the event idempotently. A canonical JSON catalog generates Go and TypeScript artifacts so API behavior, Pricing, Billing, and docs use the same allowances and operation weights.

**Tech Stack:** Go 1.24, pgx/sqlc/PostgreSQL, chi HTTP handlers, Node.js catalog generator, Next.js 16 App Router, React 19, existing Dashboard styles, Node source tests, Playwright, Railway, Vercel.

---

## File structure

- Create `config/x-credits-catalog.json` as the canonical plan allowance and operation-weight catalog.
- Create `scripts/generate-x-credits-catalog.mjs` to generate/check Go and TypeScript artifacts.
- Create `api/internal/xcredits/catalog_generated.go` and `dashboard/src/data/x-credits-catalog.generated.ts`.
- Create `api/internal/db/migrations/105_x_bounded_usage.sql` for periods, events, daily inbound usage, and constraints.
- Create `api/internal/db/queries/x_usage.sql`, then regenerate sqlc.
- Create `api/internal/xcredits/service.go`, `service_test.go`, and `catalog_test.go`.
- Modify `api/internal/handler/social_posts.go` to reserve/finalize/reverse usage for managed X writes.
- Modify `api/internal/handler/billing.go` and `api/cmd/api/main.go` to expose `GET /v1/billing/x-credits`.
- Modify `api/internal/handler/billing_test.go` or add `api/internal/handler/x_credits_test.go` for the public contract.
- Modify `api/internal/handler/billing.go` and `dashboard/src/lib/api.ts` so Enterprise serializes as custom pricing instead of `$0`.
- Modify `dashboard/src/app/(dashboard)/settings/billing/page.tsx` to show the monthly X allowance.
- Modify `dashboard/src/app/pricing/pricing-page-client.tsx` and `dashboard/src/app/docs/pricing/page.tsx` to show plan allowances and operation capacity without Top-up availability.
- Create `dashboard/src/app/docs/api/x-credits/page.tsx` and `dashboard/src/app/docs/guides/x/credits/page.tsx`.
- Modify docs navigation, search index, API index, Guides index, posts reference, and errors reference for bidirectional links.
- Create `dashboard/tests/x-credits-foundation-source.test.mjs`.

## Task 0: X developer and environment prerequisites

**Files:**
- Modify: `docs/prd-x-credits-dms-comments.md` only if official X documentation has materially changed since approval.
- Create outside the repository: environment-specific secret/config records in X Developer Console, Railway, and the deployment runbook.

- [ ] **Step 1: Re-verify current official X requirements**

Open the official sources linked in PRD Section 2.4 and confirm current pay-per-use resource prices, OAuth scopes, Activity/webhook availability, subscription capacity, callback rules, and funding/spend-limit behavior. Record the verification date and any differences. Do not copy upstream dollar prices into public product surfaces.

- [ ] **Step 2: Inventory the three environments**

For development, staging, and production, record the X Project/App identifier, owning account, billing owner, secret-rotation owner, funded status, spend limit, OAuth callbacks, webhook URL, CRC/signature status, and current Activity subscription capacity. Never paste secrets into the repository or task transcript.

- [ ] **Step 3: Configure bounded credentials**

Ensure development and staging use non-production apps or separately bounded credentials where X supports it. Configure the exact callback families from PRD Section 14.1. Do not enable paid Activity subscriptions before the monthly allowance and daily inbound cap enforcement are deployed in that environment.

- [ ] **Step 4: Prepare the access/capacity application**

Use the approved narrative in PRD Section 14.5 and current traffic projections. Submit only the permissions/capacity request required for the active release phase. Record external approval status as a release dependency; do not bypass a missing approval with production credentials in a lower environment.

- [ ] **Step 5: Verify readiness or record the blocker**

Phase 0 is ready when at least the development app is funded, bounded, correctly callback-configured, and usable for managed-X publishing acceptance. Staging/production approval may remain pending during local implementation, but it must be resolved before the corresponding promotion.

## Task 1: Canonical catalog and generated artifacts

**Files:**
- Create: `config/x-credits-catalog.json`
- Create: `scripts/generate-x-credits-catalog.mjs`
- Create: `api/internal/xcredits/catalog_generated.go`
- Create: `api/internal/xcredits/catalog_test.go`
- Create: `dashboard/src/data/x-credits-catalog.generated.ts`

- [ ] **Step 1: Write the failing Go catalog contract**

Create tests asserting:

```go
func TestPlanAllowance(t *testing.T) {
	tests := map[string]int64{
		"free": 0, "api": 1500, "basic": 4000,
		"growth": 12000, "team": 30000,
	}
	for plan, want := range tests {
		got, ok := PlanAllowance(plan)
		if !ok || got != want {
			t.Fatalf("PlanAllowance(%q) = %d, %v; want %d, true", plan, got, ok, want)
		}
	}
	if _, ok := PlanAllowance("enterprise"); ok {
		t.Fatal("enterprise allowance must remain contract-defined")
	}
}

func TestOperationWeights(t *testing.T) {
	for operation, want := range map[string]int64{
		"post.create": 15,
		"post.create_url": 200,
		"post.reply_summoned": 10,
		"post.read": 5,
		"user.read": 10,
		"dm.read": 10,
		"dm.send": 15,
		"post.mention.received": 5,
		"dm.received": 10,
	} {
		if got := OperationWeight(operation); got != want {
			t.Fatalf("OperationWeight(%q) = %d, want %d", operation, got, want)
		}
	}
}
```

- [ ] **Step 2: Run the test and verify RED**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/xcredits -run 'TestPlanAllowance|TestOperationWeights'
```

Expected: FAIL because `internal/xcredits` and its catalog functions do not exist.

- [ ] **Step 3: Add the canonical catalog**

The JSON must contain catalog version `x-credits-2026-07-16-v1`, plan allowances `0/1500/4000/12000/30000/custom`, daily inbound defaults `0/0/400/1200/3000/custom`, and the operation weights tested above plus provisional XChat values marked `later_phase`.

- [ ] **Step 4: Add generator and generated files**

The generator supports:

```bash
node scripts/generate-x-credits-catalog.mjs
node scripts/generate-x-credits-catalog.mjs --check
```

`--check` compares generated content in memory and exits non-zero on drift. Generated Go exports `CatalogVersion`, `PlanAllowance`, `InboundDailyLimit`, `OperationWeight`, and `OperationCatalog`. Generated TypeScript exports equivalent readonly data plus floor-rounded plan capacity rows.

- [ ] **Step 5: Verify GREEN**

Run:

```bash
node scripts/generate-x-credits-catalog.mjs --check
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/xcredits -run 'TestPlanAllowance|TestOperationWeights'
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add config scripts api/internal/xcredits dashboard/src/data/x-credits-catalog.generated.ts
git commit -m "feat: add canonical X credits catalog"
```

## Task 2: Atomic bounded-usage persistence

**Files:**
- Create: `api/internal/db/migrations/105_x_bounded_usage.sql`
- Create: `api/internal/db/queries/x_usage.sql`
- Create: `api/internal/xcredits/service.go`
- Create: `api/internal/xcredits/service_test.go`
- Modify generated files under `api/internal/db/` through `sqlc generate`

- [ ] **Step 1: Write failing service tests**

Use a small test store interface rather than mocking SQL. Cover:

1. First reservation creates one provisional event and increments the period.
2. Same idempotency key returns the existing event without another increment.
3. Reservation that would exceed the monthly limit returns `ErrMonthlyLimitExceeded`.
4. `Finalize` changes provisional to finalized once.
5. `Reverse` decrements used units exactly once.
6. Existing `connection_type=byo` returns zero counted units without store calls; only `managed` consumes UniPost allowance.
7. Stripe subscription period is preferred; missing period falls back to the current UTC month.

The service API is:

```go
type ReserveRequest struct {
	WorkspaceID, SocialAccountID, ConnectionType string
	OperationKey, Source, IdempotencyKey          string
	RequestedUnits                               int64
	Now                                          time.Time
}

type UsageEvent struct {
	ID, Status, OperationKey, CatalogVersion string
	WeightedUnits                            int64
	Duplicate                                bool
}

func (s *Service) Reserve(ctx context.Context, req ReserveRequest) (UsageEvent, error)
func (s *Service) Finalize(ctx context.Context, eventID string, finalUnits int64) error
func (s *Service) Reverse(ctx context.Context, eventID string) error
func (s *Service) Snapshot(ctx context.Context, workspaceID string, now time.Time) (Snapshot, error)
```

- [ ] **Step 2: Verify RED**

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/xcredits -run 'TestService'
```

Expected: FAIL because `Service` is undefined.

- [ ] **Step 3: Add migration and SQL queries**

The migration creates:

```sql
x_usage_periods(
  workspace_id TEXT,
  period_start TIMESTAMPTZ,
  period_end TIMESTAMPTZ,
  weighted_units_used BIGINT CHECK (weighted_units_used >= 0),
  weighted_units_limit BIGINT CHECK (weighted_units_limit >= 0),
  PRIMARY KEY(workspace_id, period_start, period_end)
)
```

```sql
x_usage_events(
  id TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id TEXT,
  social_account_id TEXT NULL,
  operation_key TEXT,
  catalog_version TEXT,
  source TEXT,
  idempotency_key TEXT,
  weighted_units BIGINT CHECK (weighted_units >= 0),
  status TEXT CHECK (status IN ('provisional','finalized','reversed')),
  UNIQUE(workspace_id, idempotency_key)
)
```

```sql
x_inbound_daily_usage(
  workspace_id TEXT,
  utc_date DATE,
  weighted_units_used BIGINT,
  weighted_units_limit BIGINT,
  events_accepted BIGINT,
  events_suppressed BIGINT,
  PRIMARY KEY(workspace_id, utc_date)
)
```

All workspace foreign keys cascade. Social-account deletion sets `social_account_id` null. Add indexes for period lookup, event reconciliation, and source/operation metrics.

- [ ] **Step 4: Generate sqlc**

```bash
cd api && sqlc generate
```

Expected: generated models and query methods include the three tables.

- [ ] **Step 5: Implement transaction-backed store and service**

Use `pgxpool.Pool.BeginTx` with row locking. Insert the unique provisional event before incrementing the period; duplicate keys return the existing event. If the conditional period increment cannot fit the requested units, roll back so no event remains. Finalize/reverse lock the event row and become no-ops after the first terminal transition. Map the repository's existing `social_accounts.connection_type` values directly: `managed` is billable by UniPost, `byo` bypasses the counter.

- [ ] **Step 6: Verify GREEN and database contract**

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/xcredits ./internal/db
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/internal/db api/internal/xcredits
git commit -m "feat: add atomic X usage accounting"
```

## Task 3: Enforce managed-X usage in publishing

**Files:**
- Modify: `api/internal/handler/social_posts.go`
- Modify: `api/internal/handler/social_posts_test.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write failing publish-path tests**

Add tests around `publishOneContext` proving:

- The existing 20/day gate rejects before X usage reservation.
- BYO accounts (`connection_type=byo`) bypass UniPost X usage.
- Managed X normal text reserves 15.
- Managed X URL text reserves 200 using the conservative classifier.
- Adapter failure reverses the provisional event.
- Adapter success finalizes once.
- `first_comment` uses a separate stable idempotency key and 15 units when attempted.
- Limit exhaustion returns an error containing `x_monthly_usage_limit_exceeded` and makes no adapter call.

Use a narrow `xUsageService` interface injected with `SetXUsageService`.

- [ ] **Step 2: Verify RED**

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestPublishOne.*XUsage'
```

Expected: FAIL because the handler does not call an X usage service.

- [ ] **Step 3: Implement gate ordering**

After plan validation, daily safety cap, and per-account quota, but before token decryption/media resolution/upstream work:

```go
event, err := h.xUsage.Reserve(ctx, xcredits.ReserveRequest{...})
```

Classify URL candidates using final caption text. For managed X only, finalize on confirmed success and reverse on confirmed failure. Use a stable key derived from post/result/account/thread position; do not use a random request-scoped key.

- [ ] **Step 4: Wire the service**

Construct `xcredits.NewService(pool, queries)` in `main.go` and attach it to the shared `SocialPostHandler`, so immediate, scheduled, dispatch, and retry workers use the same enforcement path.

- [ ] **Step 5: Verify GREEN**

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler ./internal/worker
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/handler api/cmd/api/main.go
git commit -m "feat: enforce monthly X usage allowance"
```

## Task 4: Public API and Enterprise serialization

**Files:**
- Modify: `api/internal/handler/billing.go`
- Create: `api/internal/handler/x_credits_test.go`
- Modify: `api/cmd/api/main.go`
- Modify: `dashboard/src/lib/api.ts`

- [ ] **Step 1: Write failing handler tests**

Cover:

- `GET /v1/billing/x-credits` returns `mode=monthly_allowance`, allowance, used, remaining, period bounds, catalog version, daily inbound usage/limit, and managed-vs-BYO note.
- Enterprise returns `monthly_allowance: null`.
- `GET /v1/plans` returns `price_cents: null` and `pricing_model: custom` for Enterprise, while self-serve plans remain fixed.

- [ ] **Step 2: Verify RED**

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'Test.*XCredits|TestListPlansEnterprise'
```

- [ ] **Step 3: Implement endpoint and response types**

Register the authenticated route beside existing billing routes. Preserve the standard success/error envelope and `request_id`.

- [ ] **Step 4: Update Dashboard API types**

Add:

```ts
export interface XCreditsAllowance {
  mode: "monthly_allowance";
  monthly_allowance: number | null;
  monthly_used: number;
  monthly_remaining: number | null;
  billing_period_start: string;
  billing_period_end: string;
  catalog_version: string;
  inbound_daily_usage: number;
  inbound_daily_limit: number | null;
  connection_mode_note: string;
}
```

Make `Plan.price_cents` nullable and add `pricing_model: "fixed" | "custom"`.

- [ ] **Step 5: Verify GREEN**

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler
```

- [ ] **Step 6: Commit**

```bash
git add api/internal/handler api/cmd/api/main.go dashboard/src/lib/api.ts
git commit -m "feat: expose X usage allowance API"
```

## Task 5: Billing, Pricing, Reference, and Guidance

**Files:**
- Modify: `dashboard/src/app/(dashboard)/settings/billing/page.tsx`
- Modify: `dashboard/src/app/pricing/pricing-page-client.tsx`
- Modify: `dashboard/src/app/docs/pricing/page.tsx`
- Create: `dashboard/src/app/docs/api/x-credits/page.tsx`
- Create: `dashboard/src/app/docs/guides/x/credits/page.tsx`
- Modify: `dashboard/src/app/docs/_components/docs-shell.tsx`
- Modify: `dashboard/src/app/docs/api/page.tsx`
- Modify: `dashboard/src/app/docs/guides/page.tsx`
- Modify: `dashboard/src/app/docs/api/posts/create/content.tsx`
- Modify: `dashboard/src/app/docs/api/posts/validate/page.tsx`
- Modify: `dashboard/src/app/docs/api/errors/page.tsx`
- Modify: `dashboard/src/lib/docs-ai-search-index.ts`
- Create: `dashboard/tests/x-credits-foundation-source.test.mjs`

- [ ] **Step 1: Write the failing source contract**

Assert that:

- Pricing imports generated catalog data and displays `What your included X Credits can do`.
- The plan capacities are 266/20/200/160 for Basic, 800/60/600/480 for Growth, and 2000/150/1500/1200 for Team.
- API comments/DM capacity says `Inbox not included`.
- Pricing and docs explicitly say the allowance resets each billing period, is separate from posts/month, and stops managed-X work at the hard limit.
- No MVP surface contains `Buy more`, `Top-up available`, or an active Top-up CTA.
- Billing calls `getXCreditsAllowance` and displays loading, error, custom, zero, and finite states.
- `/docs/api/x-credits` links to `/docs/guides/x/credits` and `/docs/pricing`.
- The Guidance page links back to the exact Reference page and create/validate references.
- Navigation, API index, Guides index, search, sitemap-by-route, and errors reference include the new routes/error.

- [ ] **Step 2: Verify RED**

```bash
cd dashboard && node --test tests/x-credits-foundation-source.test.mjs
```

Expected: FAIL because the pages and generated-data usage do not exist.

- [ ] **Step 3: Implement Billing UI**

Add one X Credits usage section below existing post usage. Reuse current stat, progress, warning, and error patterns. Use no new dependency. Show:

- allowance used / total;
- remaining credits;
- reset date;
- normal-post, URL-post, complete-comment, and complete-DM examples;
- the independent 20/day X publish safety-cap note;
- hard-limit upgrade/contact guidance;
- `Custom` for Enterprise.

- [ ] **Step 4: Implement Pricing and Plans docs**

Render the desktop table and mobile plan cards from the generated catalog. Explain that each column assumes the whole shared allowance is spent on one operation type. Do not publish upstream X prices, margin, the internal `$0.001` mapping, Top-up packs, or Auto top-up.

- [ ] **Step 5: Implement bidirectionally linked Reference and Guidance**

Reference documents the MVP endpoint fields, operation weights, managed-vs-BYO behavior, reset semantics, and errors. Guidance provides task steps for estimating, inspecting, and handling exhaustion. Both link to each other and to Plans and limits.

- [ ] **Step 6: Verify GREEN**

```bash
cd dashboard && node --test tests/x-credits-foundation-source.test.mjs tests/docs-pricing-guide-source.test.mjs tests/enterprise-pricing-source.test.mjs tests/team-unlimited-posts-source.test.mjs
npm run build
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add dashboard
git commit -m "feat: publish X allowance surfaces"
```

## Task 6: Full validation and standard production release

**Files:**
- No new source files expected.

- [ ] **Step 1: Regenerate and verify clean generated output**

```bash
node scripts/generate-x-credits-catalog.mjs --check
git status --short
```

Expected: no generated drift and only intentional changes.

- [ ] **Step 2: Run backend validation**

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./...
```

Expected: PASS.

- [ ] **Step 3: Run frontend validation**

```bash
cd dashboard
npm run build
npm run test:regression:dashboard
```

Expected: PASS. If Playwright browsers are unavailable, report the exact skipped reason before promotion.

- [ ] **Step 4: Merge task branch into local dev**

Fetch `origin`, update local `dev` from `origin/dev`, merge `dev-x-inbox-production-rollout`, and rerun Steps 1–3 on local `dev`.

- [ ] **Step 5: Push and verify development**

Push local `dev` to `origin/dev`. Wait for GitHub, Railway, and Vercel checks/deployments. Verify:

- `https://dev-api.unipost.dev/v1/plans`
- authenticated `GET https://dev-api.unipost.dev/v1/billing/x-credits`
- `https://dev.unipost.dev/pricing`
- `https://dev.unipost.dev/docs/pricing`
- `https://dev.unipost.dev/docs/api/x-credits`
- `https://dev.unipost.dev/docs/guides/x/credits`
- authenticated Billing UI at `https://dev-app.unipost.dev/settings/billing`
- one real managed-X publish decrements the allowance once; a failed validation consumes zero; the existing 20/day cap remains independent.

- [ ] **Step 6: Promote and verify staging**

Create and merge `dev -> staging` PR after checks pass. Wait for deployments and repeat the relevant API, public-page, Billing, and controlled managed-X acceptance checks on staging domains.

- [ ] **Step 7: Promote and verify production**

Create and merge `staging -> main` PR after checks pass. Wait for production deployments. Verify production health, public Pricing/docs, authenticated allowance API/Billing, and a controlled managed-X flow. Stop if X credentials, funding, or approval are missing; report the exact external blocker rather than bypassing acceptance.

- [ ] **Step 8: Record production result**

Update the main rollout tracker with Phase 0–1 production commit, deployment URLs, acceptance evidence, catalog version, and any external X permission/capacity follow-up. Only then begin the X Comments production phase.
