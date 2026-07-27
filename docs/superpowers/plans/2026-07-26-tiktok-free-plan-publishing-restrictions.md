# TikTok Free-Plan Publishing Restrictions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the approved database-backed TikTok/Free publishing restriction, customer and Admin experiences, 60-day policy media retention, and separately operable restriction/recovery email campaigns with the restriction disabled by default.

**Architecture:** A new `publishingrestrictions` backend domain owns policy evaluation, optimistic Admin transitions, cycle/audit state, customer projection, and campaign orchestration. `SocialPostHandler`, scheduler dispatch, delivery execution, and result-level Retry all consume the same evaluator, while existing `media_post_usages` plus `media.usage_version` provide retention and cleanup safety. The Dashboard consumes a workspace projection and shared result contract; Admin follows the existing DB feature-flag authorization/transaction/UI patterns without storing this richer policy in `feature_flags`.

**Tech Stack:** Go 1.24, chi, pgx/sqlc, PostgreSQL/goose, React 19, Next.js 15, TypeScript, Clerk, Loops `AuditedClient`, Playwright, Vitest-style Node tests where already configured.

**Canonical requirements:** `docs/prd-tiktok-free-plan-publishing-restrictions.md`

**Safety invariants:** The migration seeds `tiktok` disabled. Tests use fakes/fixtures only. No implementation step enables a real restriction, sends an external email, mutates production customer state, deploys, promotes, or merges `staging`.

---

## File map

### Backend policy and persistence

- Create `api/internal/db/migrations/122_platform_publishing_restrictions.sql`: restriction, audit, campaign, recipient, and retention-reason schema; disabled seed.
- Create `api/internal/publishingrestrictions/model.go`: canonical constants and public/domain types.
- Create `api/internal/publishingrestrictions/postgres.go`: policy reads, Admin mutation, affected counts, campaign snapshots/claims/progress.
- Create `api/internal/publishingrestrictions/service.go`: evaluator, transition rules, projection, campaign preconditions, audience eligibility.
- Create `api/internal/publishingrestrictions/service_test.go`: policy, version/cycle, audience, and campaign unit tests.
- Create `api/internal/publishingrestrictions/postgres_contract_test.go`: SQL/schema/locking/idempotency contract tests.

### Backend HTTP and publishing integration

- Create `api/internal/handler/publishing_restrictions.go`: workspace projection and Super Admin restriction/campaign APIs.
- Create `api/internal/handler/publishing_restrictions_test.go`: exact envelopes, auth, version conflict, preview/confirmation tests.
- Modify `api/internal/handler/social_posts.go`: inject policy evaluator, gate create/publish-draft paths, expose restriction result fields.
- Modify `api/internal/handler/social_post_queue.go`: mixed immediate failures, scheduler/delivery recheck, zero-provider-call terminal policy failure.
- Modify `api/internal/handler/social_post_retry.go`: dynamic policy Retry block and atomic media activation.
- Modify `api/internal/handler/retry_policy.go`: policy-specific manual Retry states.
- Modify `api/internal/handler/social_posts_media_retention.go`: policy reason/deadline and Retry activation.
- Modify focused handler tests for immediate, mixed, scheduled, worker, Retry, plan upgrade, active-job uniqueness, and retention races.
- Modify `api/cmd/api/main.go`: construct one service, inject it everywhere, register APIs, start the campaign worker only with the normal worker lifecycle.

### Email communication milestone

- Create `api/internal/worker/publishing_restriction_email.go`: bounded recipient claims, eligibility recheck, `AuditedClient` sends, bounded retry, progress.
- Create `api/internal/worker/publishing_restriction_email_test.go`: exact copy, idempotency, crash/failure retry, recovery-subset tests.
- Modify `api/internal/emailregistry/registry.go` and tests: register two fixed service notices.
- Modify `docs/email-templates.md`: document the two required Loops transactional template IDs and exact variables.

### Dashboard customer and Admin surfaces

- Modify `dashboard/src/lib/api.ts`: projection/Admin/campaign types and client functions, `media_retained_until` result field.
- Create `dashboard/src/lib/publishing-restrictions.ts`: exact customer constants and pure selection/retry helpers.
- Create `dashboard/src/lib/publishing-restrictions.test.ts`: exact-copy and helper tests.
- Modify `dashboard/src/components/posts/create-post/create-post-drawer.tsx`: open/focus projection refresh, selection disabling, inline notice/error.
- Modify `dashboard/src/components/posts/create-post/account-card-grid.tsx` and `account-card.tsx`: accessible disabled account state.
- Modify shared Posts/Calendar result components under `dashboard/src/components/posts/details/`: title/message/deadline/Retry behavior.
- Create `dashboard/src/app/admin/publishing-restrictions/page.tsx`: generic restriction rows, optimistic conflict, confirmations, campaign preview/progress/retry.
- Modify `dashboard/src/app/admin/_components/admin-ui.tsx`: navigation item.
- Add focused Dashboard/Playwright coverage for Composer, Posts, Calendar, and Admin.

---

### Task 1: Add the disabled-by-default schema and retention reason

**Files:**
- Create: `api/internal/db/migrations/122_platform_publishing_restrictions.sql`
- Create: `api/internal/db/platform_publishing_restrictions_migration_test.go`
- Modify: `api/internal/db/migrate_test.go`

- [ ] **Step 1: Write the failing migration contract test**

Assert the migration contains the four dedicated tables, unique cycle/type and campaign/recipient constraints, `media_post_usages.retention_reason`, and the disabled TikTok/Free seed:

```go
func TestPlatformPublishingRestrictionsMigrationContract(t *testing.T) {
    body, err := os.ReadFile("migrations/122_platform_publishing_restrictions.sql")
    if err != nil { t.Fatal(err) }
    sql := string(body)
    for _, required := range []string{
        "CREATE TABLE platform_publishing_restrictions",
        "CREATE TABLE platform_publishing_restriction_events",
        "CREATE TABLE platform_publishing_restriction_email_campaigns",
        "CREATE TABLE platform_publishing_restriction_email_recipients",
        "ADD COLUMN retention_reason",
        "VALUES ('tiktok', FALSE, ARRAY['free']::TEXT[]",
        "UNIQUE (cycle_id, campaign_type)",
    } {
        if !strings.Contains(sql, required) { t.Fatalf("missing %q", required) }
    }
}
```

- [ ] **Step 2: Run the test and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run TestPlatformPublishingRestrictionsMigrationContract -count=1`

Expected: FAIL because migration 122 does not exist.

- [ ] **Step 3: Add the migration**

Create schema with these enforced values:

```sql
CREATE TABLE platform_publishing_restrictions (
  id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
  platform TEXT NOT NULL UNIQUE,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  restricted_plan_ids TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
  reason_code TEXT NOT NULL,
  user_message TEXT NOT NULL,
  cycle_id TEXT,
  version BIGINT NOT NULL DEFAULT 1,
  enabled_at TIMESTAMPTZ,
  disabled_at TIMESTAMPTZ,
  updated_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Add append-only event, fixed campaign-type/status, recipient-status, normalized-email uniqueness, indexes for claims/progress, and:

```sql
ALTER TABLE media_post_usages
  ADD COLUMN retention_reason TEXT NOT NULL DEFAULT 'plan_status'
  CHECK (retention_reason IN ('active_post','plan_status','publishing_restriction'));

INSERT INTO platform_publishing_restrictions (
  platform, enabled, restricted_plan_ids, reason_code, user_message
) VALUES (
  'tiktok', FALSE, ARRAY['free']::TEXT[], 'platform_capacity_limit',
  'TikTok publishing is temporarily unavailable on the Free plan due to platform capacity limits. We’re working with TikTok to increase capacity. Upgrade your plan or try again after the restriction is lifted.'
) ON CONFLICT (platform) DO NOTHING;
```

- [ ] **Step 4: Run migration/database contract tests and verify GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run 'TestPlatformPublishingRestrictionsMigrationContract|TestMigrations' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/db/migrations/122_platform_publishing_restrictions.sql api/internal/db/platform_publishing_restrictions_migration_test.go api/internal/db/migrate_test.go
git commit -m "feat: add publishing restriction schema"
```

### Task 2: Build the centralized policy and Admin transition domain

**Files:**
- Create: `api/internal/publishingrestrictions/model.go`
- Create: `api/internal/publishingrestrictions/service.go`
- Create: `api/internal/publishingrestrictions/postgres.go`
- Create: `api/internal/publishingrestrictions/service_test.go`
- Create: `api/internal/publishingrestrictions/postgres_contract_test.go`

- [ ] **Step 1: Write failing evaluator and transition tests**

Cover Free/TikTok blocked, Paid bypass, other-platform bypass, missing subscription as Free, read error, disabled seed, cycle creation/preservation, same-state no-op, 409-style version conflict, actor/event atomicity, and affected counts.

```go
func TestEvaluatorRestrictsOnlyConfiguredPlanAndPlatform(t *testing.T) {
    store := &fakeStore{restriction: Restriction{Platform: "tiktok", Enabled: true, RestrictedPlanIDs: []string{"free"}}}
    evaluator := NewService(store)
    decision, err := evaluator.Evaluate(context.Background(), Subject{WorkspaceID: "ws_1", Platform: "tiktok", PlanID: "free"})
    if err != nil { t.Fatal(err) }
    if !decision.Restricted || decision.Code != NormalizedCode { t.Fatalf("decision=%+v", decision) }
}
```

- [ ] **Step 2: Run tests and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/publishingrestrictions -count=1`

Expected: FAIL because the package is absent.

- [ ] **Step 3: Implement canonical model and service**

Use one immutable source for the exact contract:

```go
const (
    APICode = "PLAN_PLATFORM_PUBLISHING_RESTRICTED"
    NormalizedCode = "plan_platform_publishing_restricted"
    UserMessage = "TikTok publishing is temporarily unavailable on the Free plan due to platform capacity limits. We’re working with TikTok to increase capacity. Upgrade your plan or try again after the restriction is lifted."
    NextAction = "upgrade_or_wait_then_retry"
    FailureStage = "publishing_policy"
    ResultTitle = "Publishing restricted"
)

type Decision struct {
    Restricted bool
    Platform string
    PlanID string
    ReasonCode string
    CycleID string // internal only
    Version int64
    Code string
    Message string
    NextAction string
}
```

Implement `Evaluate`, `WorkspaceProjection`, `ListAdmin`, and `SetEnabled(ctx, platform, enabled, expectedVersion, actor, requestMeta)` using a store transaction.

- [ ] **Step 4: Implement PostgreSQL row locking and audit**

Use `SELECT ... FOR UPDATE`, compare explicit version, update and insert the event in one transaction. Reuse `auth.SuperAdminChecker` at the handler layer; do not add provider flags or environment toggles.

- [ ] **Step 5: Run focused package tests and verify GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/publishingrestrictions -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/publishingrestrictions api/internal/db/migrations/122_platform_publishing_restrictions.sql
git commit -m "feat: add publishing restriction policy service"
```

### Task 3: Add customer projection and Super Admin restriction APIs

**Files:**
- Create: `api/internal/handler/publishing_restrictions.go`
- Create: `api/internal/handler/publishing_restrictions_test.go`
- Modify: `api/cmd/api/main.go`
- Modify: `api/internal/handler/response.go`
- Modify: `api/internal/handler/response_test.go`

- [ ] **Step 1: Write failing HTTP tests**

Test `GET /v1/publishing-restrictions`, Super Admin-only list/patch, ordinary Admin 403, exact response projection, `expected_version`, 409 current-state response, confirmation preconditions, idempotent no-op, and no email/job side effect.

- [ ] **Step 2: Run tests and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'PublishingRestriction|PlanPlatformPublishingRestricted' -count=1`

Expected: FAIL because handlers/routes/error mapping are absent.

- [ ] **Step 3: Add exact error writer**

Return 402 only when every requested target is blocked. Public details contain only platform and plan:

```go
writeDetailedError(w, http.StatusPaymentRequired, publishingrestrictions.APICode,
    publishingrestrictions.UserMessage, ErrorOptions{
        NormalizedCode: publishingrestrictions.NormalizedCode,
        NextAction: publishingrestrictions.NextAction,
        IsRetriable: boolPtr(false),
        ErrorSource: "unipost",
        ErrorTemporality: "temporary",
        Details: map[string]any{"platform": "tiktok", "plan_id": "free"},
    })
```

Do not expose cycle ID.

- [ ] **Step 4: Implement routes and wiring**

Register:

```go
r.Get("/v1/publishing-restrictions", restrictionHandler.WorkspaceProjection)
r.Get("/v1/admin/publishing-restrictions", restrictionHandler.AdminList)
r.Patch("/v1/admin/publishing-restrictions/{platform}", restrictionHandler.AdminSet)
```

The Admin methods call `SuperAdminChecker.IsSuperAdmin`; ordinary workspace role is insufficient.

- [ ] **Step 5: Run focused tests and verify GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'PublishingRestriction|PlanPlatformPublishingRestricted' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/handler/publishing_restrictions.go api/internal/handler/publishing_restrictions_test.go api/internal/handler/response.go api/internal/handler/response_test.go api/cmd/api/main.go
git commit -m "feat: expose publishing restriction APIs"
```

### Task 4: Gate create admission and preserve mixed-platform progress

**Files:**
- Modify: `api/internal/handler/social_posts.go`
- Modify: `api/internal/handler/social_post_queue.go`
- Create: `api/internal/handler/social_posts_publishing_restrictions_test.go`
- Modify: `api/internal/handler/social_posts_drafts.go`

- [ ] **Step 1: Write failing create-flow tests**

Cover fully blocked immediate 402/no persistence, fully blocked scheduled 402/no persistence, mixed immediate 202 with failed TikTok plus queued allowed result, mixed scheduled 201, Paid bypass, other-platform bypass, draft save allowed, draft publish gated, zero quota for policy failure, and internal cycle metadata only.

- [ ] **Step 2: Run tests and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'Test.*PublishingRestriction.*(Create|Draft|Mixed|Paid)' -count=1`

Expected: FAIL because `SocialPostHandler` does not evaluate the policy.

- [ ] **Step 3: Inject one evaluator and partition targets after trusted account resolution**

```go
type publishingRestrictionEvaluator interface {
    Evaluate(context.Context, publishingrestrictions.Subject) (publishingrestrictions.Decision, error)
}

func (h *SocialPostHandler) SetPublishingRestrictions(e publishingRestrictionEvaluator) *SocialPostHandler {
    h.publishingRestrictions = e
    return h
}
```

Resolve platform from `accountMap`, plan from the server, and partition `parsed.Posts`. A policy-read failure returns 503 and persists nothing.

- [ ] **Step 4: Implement mixed immediate persistence**

Extend the queue transaction to create terminal failed results for restricted targets with:

```go
db.CreateSocialPostResultParams{
    Status: "failed",
    ErrorMessage: pgtype.Text{String: publishingrestrictions.UserMessage, Valid: true},
}
```

Then persist matching failure taxonomy fields, skip delivery jobs for restricted results, queue allowed results, and set parent state to `partial` when an allowed result is active alongside a restriction failure.

- [ ] **Step 5: Preserve scheduled metadata and correct statuses**

For a mixed schedule, retain all target metadata so due-time policy/plan changes are authoritative and return 201. A TikTok-only Free schedule active now returns 402. Draft storage remains unrestricted.

- [ ] **Step 6: Run focused and existing quota/idempotency tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'PublishingRestriction|SocialPostsQuota|SocialPostsIdempotency|Draft' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/internal/handler/social_posts.go api/internal/handler/social_post_queue.go api/internal/handler/social_posts_drafts.go api/internal/handler/social_posts_publishing_restrictions_test.go
git commit -m "feat: gate restricted publish admission"
```

### Task 5: Recheck scheduled and delivery work before TikTok

**Files:**
- Modify: `api/internal/handler/social_post_queue.go`
- Modify: `api/internal/handler/social_post_queue_test.go`
- Modify: `api/internal/worker/scheduler.go`
- Modify: `api/internal/worker/scheduler_test.go`
- Modify: `api/internal/worker/post_delivery.go`
- Modify: `api/internal/worker/post_delivery_test.go`

- [ ] **Step 1: Write failing due-time and worker tests**

Use a counting fake TikTok adapter. Cover existing due Free schedule terminal policy failure, mixed due continuation, disable/upgrade before due, queued-before-enable block, and re-enable between Retry enqueue/dispatch. Assert adapter call count zero and no automatic retry job.

- [ ] **Step 2: Run tests and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler ./internal/worker -run 'PublishingRestriction|WorkerRecheck|ScheduledRestriction' -count=1`

Expected: FAIL because scheduler/worker do not recheck policy.

- [ ] **Step 3: Add due-time evaluation**

At scheduler fan-out, evaluate each target using current plan. Create a failed result and `post_failures` row for restricted TikTok; create no job for it; continue every allowed target; derive parent status deterministically.

- [ ] **Step 4: Add the final delivery check at the verified seam**

After duplicate/result guards and input preparation, but before `platform_started_at` and `publishOneContext`, evaluate again. On restriction, atomically mark result/job terminal with canonical fields and return without entering the adapter.

- [ ] **Step 5: Run focused worker tests and verify GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler ./internal/worker -run 'PublishingRestriction|WorkerRecheck|ScheduledRestriction|PlatformStarted' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/handler/social_post_queue.go api/internal/handler/social_post_queue_test.go api/internal/worker/scheduler.go api/internal/worker/scheduler_test.go api/internal/worker/post_delivery.go api/internal/worker/post_delivery_test.go
git commit -m "feat: recheck restrictions before delivery"
```

### Task 6: Implement manual Retry eligibility and atomic media activation

**Files:**
- Modify: `api/internal/handler/social_post_retry.go`
- Modify: `api/internal/handler/social_post_retry_test.go`
- Modify: `api/internal/handler/retry_policy.go`
- Modify: `api/internal/handler/retry_policy_test.go`
- Modify: `api/internal/db/queries/post_delivery_jobs.sql`

- [ ] **Step 1: Write failing Retry tests**

Cover the exact route `POST /v1/posts/{id}/results/{resultID}/retry`, active Free restriction 402, dynamic blocked policy, disabled availability, Paid-upgrade availability while Free remains restricted, media missing/expired, one-active-job uniqueness, and no Admin/bulk Retry.

- [ ] **Step 2: Run tests and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'Retry.*PublishingRestriction|PublishingRestriction.*Retry' -count=1`

Expected: FAIL because policy Retry states are absent.

- [ ] **Step 3: Add policy-aware retry projection**

Return:

```go
retryPolicyResponse{
    State: "blocked",
    ManualRetryAllowed: false,
    Reason: "publishing_restriction_active",
}
```

When disabled or upgraded, retain existing failed-result/job/media checks and return `manual_only` only if media remains.

- [ ] **Step 4: Make enqueue plus media activation one transaction**

Lock the result, re-evaluate policy/plan, verify media, preserve the active-job partial unique index, create one job, set parent usages to `active_post` with null cleanup, and increment existing `media.usage_version` in one transaction.

- [ ] **Step 5: Run focused Retry tests and verify GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'Retry|PublishingRestriction' -count=1`

Expected: PASS.

- [ ] **Step 6: Regenerate sqlc if query signatures changed**

Run: `cd api && sqlc generate`

Expected: generated DB files match query/schema changes and compile.

- [ ] **Step 7: Commit**

```bash
git add api/internal/handler/social_post_retry.go api/internal/handler/social_post_retry_test.go api/internal/handler/retry_policy.go api/internal/handler/retry_policy_test.go api/internal/db
git commit -m "feat: gate customer retry by publishing policy"
```

### Task 7: Add 60-day policy retention and result deadline projection

**Files:**
- Modify: `api/internal/db/queries/media_post_usages.sql`
- Modify generated `api/internal/db/media_post_usages.sql.go` and models via sqlc
- Modify: `api/internal/handler/social_posts_media_retention.go`
- Modify: `api/internal/handler/social_posts_media_retention_test.go`
- Modify: `api/internal/handler/social_posts.go`
- Modify: `api/internal/worker/media_cleanup_test.go`

- [ ] **Step 1: Write failing retention/race tests**

Cover failure time + 60 days, reason override, parent-wide partial override, `media_retained_until`, Retry clear, success current-plan recalculation, repeated policy failure restart, both cleanup/Retry race winners, shared media, and expired re-upload requirement.

- [ ] **Step 2: Run tests and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler ./internal/worker -run 'Media.*PublishingRestriction|PublishingRestriction.*Media|CleanupRetryRace' -count=1`

Expected: FAIL because the reason/deadline projection is absent.

- [ ] **Step 3: Extend the media usage upsert**

Add `retention_reason` to the upsert/update arguments. Policy failure uses:

```go
cleanupAt := failedAt.UTC().Add(60 * 24 * time.Hour)
reason := "publishing_restriction"
```

All parent media usages receive the longer window intentionally. Do not add `usage_version` to `media_post_usages`; reuse the existing media-row version increment in the query.

- [ ] **Step 4: Expose deadline safely**

Add `media_retained_until` to result responses only when a policy-retained usage exists. After cleanup it remains historical; Retry returns `media_reupload_required` if objects are absent.

- [ ] **Step 5: Regenerate sqlc and run focused tests**

Run: `cd api && sqlc generate && GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/handler ./internal/worker -run 'Media|Retention|Cleanup|PublishingRestriction' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/db api/internal/handler/social_posts_media_retention.go api/internal/handler/social_posts_media_retention_test.go api/internal/handler/social_posts.go api/internal/worker/media_cleanup_test.go
git commit -m "feat: retain policy-failed media for 60 days"
```

### Task 8: Add Dashboard policy helpers and customer Composer/result UX

**Required skill:** `design-taste-frontend`

**Files:**
- Create: `dashboard/src/lib/publishing-restrictions.ts`
- Create: `dashboard/src/lib/publishing-restrictions.test.ts`
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/components/posts/create-post/create-post-drawer.tsx`
- Modify: `dashboard/src/components/posts/create-post/account-card-grid.tsx`
- Modify: `dashboard/src/components/posts/create-post/account-card.tsx`
- Modify shared result components in `dashboard/src/components/posts/details/`
- Modify relevant Playwright specs under `dashboard/tests/regression/`

- [ ] **Step 1: Write failing exact-copy/helper tests**

```ts
export const PLAN_PLATFORM_PUBLISHING_RESTRICTED_MESSAGE =
  "TikTok publishing is temporarily unavailable on the Free plan due to platform capacity limits. We’re working with TikTok to increase capacity. Upgrade your plan or try again after the restriction is lifted.";
```

Test Free/TikTok selection exclusion, Paid bypass, normalized error recognition, title `Publishing restricted`, and expired deadline state.

- [ ] **Step 2: Run helper tests and verify RED**

Run the repository's focused Dashboard unit-test command for `publishing-restrictions.test.ts` discovered from `dashboard/package.json`.

Expected: FAIL because helper module is absent.

- [ ] **Step 3: Implement API types and shared helpers**

Add `getPublishingRestrictions(token)`, Admin types later reused by Task 10, and `media_retained_until?: string` to result types.

- [ ] **Step 4: Implement open/focus projection refresh and selection safety**

In the shared `CreatePostDrawer`, fetch on `open`, attach a `window.focus` listener only while open, deselect/disable restricted TikTok, exclude it from Toggle All/preselection/restoration, and retain the server-authoritative inline submit fallback.

- [ ] **Step 5: Render persistent accessible notice and result details**

Place the exact notice beneath TikTok account selection using existing tokens, native disabled controls, visible reason, and `aria-describedby`. Shared Posts/Calendar details show title, exact message, next action, deadline, blocked/enabled Retry, and re-upload state.

- [ ] **Step 6: Run focused tests and Dashboard build**

Run: `cd dashboard && npm run build`

Run the focused test command and relevant Playwright spec.

Expected: all PASS; no skipped/no-result checks.

- [ ] **Step 7: Commit**

```bash
git add dashboard/src/lib dashboard/src/components/posts dashboard/tests
git commit -m "feat: show publishing restrictions to customers"
```

### Task 9: Build the generic Admin Publishing Restrictions center

**Required skill:** `design-taste-frontend`

**Files:**
- Create: `dashboard/src/app/admin/publishing-restrictions/page.tsx`
- Create focused tests for the page
- Modify: `dashboard/src/app/admin/_components/admin-ui.tsx`
- Modify: `dashboard/src/lib/api.ts`

- [ ] **Step 1: Write failing Admin interaction tests**

Test generic platform row rendering, state/plans/reason/counts/timestamps/actor, Super Admin error, enable/disable confirmations containing no-email/no-auto-retry language, expected-version body, 409 reload, and accessible loading/error/empty states.

- [ ] **Step 2: Run tests and verify RED**

Run the focused Admin test command from `dashboard/package.json`.

Expected: FAIL because the route is absent.

- [ ] **Step 3: Implement typed API calls**

```ts
updateAdminPublishingRestriction(token, platform, {
  enabled,
  expected_version: restriction.version,
});
```

Handle 409 by refetching and rendering an inline conflict.

- [ ] **Step 4: Implement the Admin page and navigation**

Reuse the Admin shell/tokens and confirmation patterns from `/admin/feature-flags`, but render data-driven platform rows, affected metrics, audit history, and campaign sections. No real toggle is executed in tests or acceptance.

- [ ] **Step 5: Run focused tests and build**

Run focused Admin tests and `cd dashboard && npm run build`.

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add dashboard/src/app/admin/publishing-restrictions dashboard/src/app/admin/_components/admin-ui.tsx dashboard/src/lib/api.ts dashboard/tests
git commit -m "feat: add publishing restrictions admin center"
```

### Task 10: Add minimal fixed campaign persistence and Admin APIs

**Files:**
- Modify: `api/internal/publishingrestrictions/model.go`
- Modify: `api/internal/publishingrestrictions/postgres.go`
- Modify: `api/internal/publishingrestrictions/service.go`
- Modify: `api/internal/publishingrestrictions/service_test.go`
- Modify: `api/internal/handler/publishing_restrictions.go`
- Modify: `api/internal/handler/publishing_restrictions_test.go`

- [ ] **Step 1: Write failing campaign orchestration tests**

Cover preview with no persistence, signed short-lived token, second explicit confirmation, restriction-only-while-enabled, recovery-only-after-disabled/successful restriction sends, current owner/Free/active-TikTok audience, user/email dedupe, snapshot count drift, one cycle/type, immutable copy, and retry-failed without resnapshot.

- [ ] **Step 2: Run tests and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/publishingrestrictions ./internal/handler -run 'Campaign|Recipient|EmailPreview' -count=1`

Expected: FAIL because campaign APIs are absent.

- [ ] **Step 3: Implement only the two fixed campaign types**

```go
const (
    RestrictionNotice CampaignType = "restriction_notice"
    RecoveryNotice CampaignType = "recovery_notice"
)
```

No arbitrary copy, segmentation, scheduling, A/B tests, or cross-channel abstractions. New rows exist only for snapshot, confirmation, progress, cycle dedupe, and failed-recipient retry.

- [ ] **Step 4: Add APIs**

Register preview, create, list, and retry-failed routes exactly as the PRD. Store exact subject/body snapshots. Preview persists no recipients; confirmation snapshots in one transaction.

- [ ] **Step 5: Run focused tests and verify GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/publishingrestrictions ./internal/handler -run 'Campaign|Recipient|EmailPreview' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/publishingrestrictions api/internal/handler/publishing_restrictions.go api/internal/handler/publishing_restrictions_test.go api/cmd/api/main.go
git commit -m "feat: add manual restriction email campaigns"
```

### Task 11: Deliver campaigns through existing audited email infrastructure

**Files:**
- Create: `api/internal/worker/publishing_restriction_email.go`
- Create: `api/internal/worker/publishing_restriction_email_test.go`
- Modify: `api/internal/emailregistry/registry.go`
- Modify: `api/internal/emailregistry/registry_test.go`
- Modify: `api/cmd/api/main.go`
- Modify: `docs/email-templates.md`

- [ ] **Step 1: Write failing exact-copy/idempotency worker tests**

Use a fake `loops.TransactionalSender`. Assert exact subjects/bodies, first-name fallback, eligibility recheck, bounded claims, durable progress, stable key `{cycle}:{type}:{canonical_user}`, `AuditedClient` use, failed-only retry, crash replay safety, and recovery subset/coverage gap.

- [ ] **Step 2: Run tests and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/worker ./internal/emailregistry -run 'PublishingRestriction|RestrictionNotice|RecoveryNotice' -count=1`

Expected: FAIL because worker/events are absent.

- [ ] **Step 3: Register two fixed service notices**

Use environment keys:

```go
LOOPS_TIKTOK_FREE_RESTRICTION_NOTICE_TRANSACTIONAL_ID
LOOPS_TIKTOK_FREE_RECOVERY_NOTICE_TRANSACTIONAL_ID
```

Delivery class is `service_alert`; audit uses `email_send_attempts`; no fallback send.

- [ ] **Step 4: Implement worker with existing `AuditedClient`**

Claim bounded recipients, recheck eligibility, call `SendTransactional` with stable provider idempotency, correlate `TriggerReferenceID` to recipient ID, and persist sent/failed/skipped progress. Automatic retries are bounded; Admin retry resets failed rows only.

- [ ] **Step 5: Run focused tests and verify GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/worker ./internal/emailregistry ./internal/loops -run 'PublishingRestriction|RestrictionNotice|RecoveryNotice|Audited' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/worker/publishing_restriction_email.go api/internal/worker/publishing_restriction_email_test.go api/internal/emailregistry api/cmd/api/main.go docs/email-templates.md
git commit -m "feat: send audited restriction campaigns"
```

### Task 12: Add Admin campaign preview, confirmation, progress, and retry UI

**Required skill:** `design-taste-frontend`

**Files:**
- Modify: `dashboard/src/app/admin/publishing-restrictions/page.tsx`
- Modify: `dashboard/src/lib/api.ts`
- Add focused Admin/Playwright tests

- [ ] **Step 1: Write failing campaign UI tests**

Cover exact preview copy/count, second irreversible confirmation, restriction/recovery eligibility, preview-vs-snapshot count, queued/running/completed/failure progress, retry failed, repeat confirmation idempotency, and no toggle-coupled send.

- [ ] **Step 2: Run tests and verify RED**

Run the focused Admin campaign test command.

Expected: FAIL because campaign UI is absent.

- [ ] **Step 3: Implement the fixed campaign panels**

Render only Restriction notice and Recovery notice actions. Require preview before confirmation; label Send irreversible; show explicit no-toggle coupling. Poll durable status while queued/running and expose Retry failed recipients only when failures exist.

- [ ] **Step 4: Run focused tests and build**

Run focused UI tests and `cd dashboard && npm run build`.

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add dashboard/src/app/admin/publishing-restrictions/page.tsx dashboard/src/lib/api.ts dashboard/tests
git commit -m "feat: manage restriction email campaigns"
```

### Task 13: Run complete local verification and independent code review

**Required skills:** `superpowers:verification-before-completion`, `superpowers:requesting-code-review`

**Files:**
- Modify only files required by discovered defects.

- [ ] **Step 1: Run all focused exact-contract tests**

Run every focused command from Tasks 1-12 again. Any fail/skip/no-result is a hard stop.

- [ ] **Step 2: Run full API CI**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./...`

Expected: PASS with zero failures/skips/timeouts.

- [ ] **Step 3: Run Dashboard build**

Run: `cd dashboard && npm run build`

Expected: PASS.

- [ ] **Step 4: Verify Playwright browser availability and run full regression**

Run: `cd dashboard && npx playwright --version && npm run test:regression:dashboard`

Expected: browsers are available and the suite passes. A missing browser, skip, timeout, or no result is failure and must be fixed before push.

- [ ] **Step 5: Run source and copy audits**

Run exact-message/subject/body searches, migration disabled-seed checks, no-provider-call tests, no external send tests, `git diff --check`, secret scan, and unique-commit/file audit against `origin/staging`.

- [ ] **Step 6: Request independent code review and commit verified fixes**

Review the exact branch diff against the approved PRD. Resolve every actionable issue using `superpowers:receiving-code-review`, stage each exact file named by `git status --short`, commit with `git commit -m "test: complete publishing restriction acceptance"`, and rerun the full suite. If the review finds no issue, do not create an empty commit.

### Task 14: Push the owned branch and open an unmerged Draft PR to staging

**Required skill:** `superpowers:finishing-a-development-branch`

**Files:** None unless remote checks expose a verified defect.

- [ ] **Step 1: Audit promotion content before push**

List exact commits and changed files unique to `origin/staging`; confirm every file belongs to this PRD and the worktree is clean.

- [ ] **Step 2: Push only the owned branch**

Run: `git push -u origin codex/staging-tiktok-free-publishing-restriction`

Expected: only the task branch is updated.

- [ ] **Step 3: Open Draft PR targeting staging**

Create a Draft PR with base `staging`, include safety defaults, local results, template prerequisites, and explicit no-enable/no-email/no-merge statements.

- [ ] **Step 4: Monitor exact-head checks and previews**

Wait for every GitHub, Vercel, Railway, deployed regression, and browser acceptance gate attached to the exact PR head SHA. If an isolated preview exists, verify Composer, Posts, Calendar, and Admin using safe fixtures without enabling the real restriction or sending email.

- [ ] **Step 5: Handle failures as hard stops**

For any fail/skip/timeout/no-result, capture workflow/job/test/log/URL evidence, diagnose with `superpowers:systematic-debugging`, fix on this branch with TDD, rerun the full required local suite, push the replacement SHA, and monitor again.

- [ ] **Step 6: Stop with Draft PR unmerged**

Report branch, latest staging base, commits/files, Draft PR URL, exact head SHA, local/remote results, preview URLs/acceptance, required email template IDs/config, and explicit confirmation that staging was not merged, no restriction was enabled, and no customer email was sent.
