# PR 270 Third-Round Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close every Critical and Important third-round review finding on PR 270 without running a real migration, sending email, enabling the restriction, or promoting an environment.

**Architecture:** The Railway gate and every historical migrator share Goose's fixed-session lock from preflight through migration. Publish admission carries one restriction snapshot into persistence, restricted finalization uses one lease-conditional SQL statement, and campaign email distinguishes unknown transport outcomes from definitive failures while API and worker readiness fail closed. Focused TDD commits are followed by isolated PostgreSQL, full API, Dashboard contract/build/browser, independent-review, and exact-SHA remote gates.

**Tech Stack:** Go 1.25, PostgreSQL 16, Goose v3 session locking, pgx/sqlc, `net/http`, Node.js 22 test runner, Next.js 16, Playwright, GitHub Actions, Railway PR Environments, Vercel Preview.

---

## File map

- `api/internal/db/migrate.go`, `migration_gate.go`, and their tests: one Goose session-lock boundary and historical-runner race.
- `api/internal/handler/social_posts*.go`, `social_post_queue.go`, and tests: one policy snapshot, draft quota, and lease-atomic finalization.
- `api/internal/db/queries/post_delivery_jobs.sql` plus generated `post_delivery_jobs.sql.go`: atomic job/result transition.
- `api/internal/loops/client.go` and tests: typed unknown send outcomes.
- `api/internal/worker/publishing_restriction_email*.go` and tests: terminal failures, readiness, stable manual retry identity.
- `api/internal/publishingrestrictions/service.go`, handler, `api/cmd/api/main.go`, and tests: shared campaign readiness and 503 mapping.
- `docs/publishing-restriction-email-campaign-runbook.md`: ambiguous-outcome manual review.
- `dashboard/package.json`, `.github/workflows/ci.yml`, and `scripts/preview/publishing-restriction-ci-contract.test.mjs`: required frontend contracts.

## Task 1: Share Goose's migration lock

**Files:**
- Modify: `api/internal/db/migrate.go`
- Modify: `api/internal/db/migrate_test.go`
- Modify: `api/internal/db/migration_gate.go`
- Test: `api/internal/db/migration_gate_postgres_integration_test.go`

- [ ] **Step 1: Write the failing historical-migrator race**

Add `TestMigrationGatePostgresExcludesHistoricalRunMigrationsUntilBackupVerified`. Block the fake backup client after creation but before readiness verification, start `RunMigrationsWithBackupGate`, then start historical `RunMigrations` against the same isolated schema. Before releasing the backup, require the legacy call not to complete and migration 125 not to be visible. After release, require both calls to finish, Goose version 125, the historical failed recipient to have `retryable=FALSE`, and one backup creation.

- [ ] **Step 2: Run RED**

```bash
PUBLISHING_RESTRICTION_TEST_DATABASE_URL='postgresql://postgres:test@127.0.0.1:5432/unipost_test?sslmode=disable' \
GOCACHE=/tmp/unipost-go-build go test -tags=integration ./internal/db \
  -run '^TestMigrationGatePostgresExcludesHistoricalRunMigrationsUntilBackupVerified$' -count=1 -v
```

Expected: FAIL because the custom UniPost lock does not exclude the legacy Goose locker.

- [ ] **Step 3: Split the internal migration runner**

Implement this boundary in `migrate.go`:

```go
func RunMigrations(databaseURL string) error {
    database, err := sql.Open("pgx", databaseURL)
    if err != nil {
        return fmt.Errorf("failed to open database for migrations: %w", err)
    }
    defer database.Close()
    return runMigrations(context.Background(), database, true)
}

func runMigrations(ctx context.Context, database *sql.DB, acquireSessionLock bool) error {
    migrationFS, err := fs.Sub(migrations, "migrations")
    if err != nil {
        return fmt.Errorf("failed to open embedded migrations: %w", err)
    }
    options := []goose.ProviderOption{}
    if acquireSessionLock {
        locker, err := lock.NewPostgresSessionLocker()
        if err != nil {
            return fmt.Errorf("failed to create migration session locker: %w", err)
        }
        options = append(options, goose.WithSessionLocker(locker))
    }
    provider, err := goose.NewProvider(goose.DialectPostgres, database, migrationFS, options...)
    if err != nil {
        return fmt.Errorf("failed to create migration provider: %w", err)
    }
    if _, err := provider.Up(ctx); err != nil {
        return fmt.Errorf("failed to run migrations: %w", err)
    }
    return nil
}
```

Legacy `RunMigrations` must retain Goose locking. Only the gate calls the internal runner with `false`.

- [ ] **Step 4: Hold Goose's locker on one connection through backup and migration**

Remove `migrationGateAdvisoryLockKey` and raw advisory SQL. On the reserved `*sql.Conn`, create `lock.NewPostgresSessionLocker()`, call `SessionLock(ctx, connection)`, and defer `SessionUnlock(context.Background(), connection)`. Keep version read, affected-row counts, backup creation/readiness/lock/reread, and `runMigrations(ctx, database, false)` inside that lifetime. Never hard-code `lock.DefaultLockID`.

- [ ] **Step 5: Run GREEN**

```bash
PUBLISHING_RESTRICTION_TEST_DATABASE_URL='postgresql://postgres:test@127.0.0.1:5432/unipost_test?sslmode=disable' \
GOCACHE=/tmp/unipost-go-build go test -tags=integration ./internal/db \
  -run 'TestRunMigrations|TestMigrationGatePostgres|TestRequireCurrentSchema' -count=1 -v
```

Expected: PASS, including one-backup concurrent gates and the historical migrator race.

- [ ] **Step 6: Commit**

```bash
git add api/internal/db/migrate.go api/internal/db/migrate_test.go \
  api/internal/db/migration_gate.go api/internal/db/migration_gate_postgres_integration_test.go
git commit -m "fix(db): share migration lock with backup gate"
```

## Task 2: Persist one restriction snapshot

**Files:**
- Modify: `api/internal/handler/social_posts.go`
- Modify: `api/internal/handler/social_posts_bulk.go`
- Modify: `api/internal/handler/social_posts_drafts.go`
- Modify: `api/internal/handler/social_post_queue.go`
- Test: `api/internal/handler/social_posts_publishing_restrictions_test.go`
- Test: `api/internal/handler/social_post_queue_test.go`

- [ ] **Step 1: Write failing behavior tests**

Make `fakePostRestrictionEvaluator` return `errors.New("unexpected second policy read")` after the admission calls. Cover immediate, bulk, draft-publish, and scheduled execution with the existing DB fake. A mixed TikTok/Instagram request must produce a restricted TikTok failure, one Instagram job, and a non-orphan parent without any second policy read.

- [ ] **Step 2: Run RED**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/handler \
  -run 'Test.*PolicySnapshot|TestQueuedPolicyPreflight' -count=1 -v
```

Expected: FAIL with `unexpected second policy read`.

- [ ] **Step 3: Make target evaluation pure**

Replace policy I/O inside `evaluateQueuedDeliveryTargets` with the supplied decision:

```go
decision := blockedTargets[pp.AccountID]
if validationErr == nil && decision.Restricted {
    validationErr = errors.New(publishingrestrictions.UserMessage)
}
```

Add `blockedTargets map[string]publishingrestrictions.Decision` to `enqueueParsedPostDeliveries`, `queueImmediatePost`, `executeImmediatePost`, `createImmediatePost`, `enqueueExistingPostDeliveries`, and `publishExistingPost`.

- [ ] **Step 4: Pass the admission map at every call site**

```go
h.createImmediatePost(w, r, workspaceID, parsed, accountMap, blockedTargets)
resp, err := h.executeImmediatePost(r, workspaceID, parsed, accountMap, blockedTargets)
h.publishExistingPost(w, r, workspaceID, claimed, parsed, accountMap, blockedTargets)
_, _, err = h.enqueueParsedPostDeliveries(ctx, post, parsed, accountMap, blockedTargets)
```

Scheduled execution keeps its one fresh evaluation and passes that map. The delivery worker's final pre-provider policy check stays intact.

- [ ] **Step 5: Run GREEN and commit**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/handler \
  -run 'PublishingRestriction|PolicySnapshot|QueuedPolicy|Bulk.*Policy|Scheduled.*Policy' -count=1 -v
git add api/internal/handler/social_posts.go api/internal/handler/social_posts_bulk.go \
  api/internal/handler/social_posts_drafts.go api/internal/handler/social_post_queue.go \
  api/internal/handler/social_posts_publishing_restrictions_test.go \
  api/internal/handler/social_post_queue_test.go
git commit -m "fix(publishing): persist one restriction snapshot"
```

Expected: PASS and no persistence helper calls `publishingRestrictions.Evaluate`.

## Task 3: Make restricted finalization lease-atomic

**Files:**
- Modify: `api/internal/db/queries/post_delivery_jobs.sql`
- Regenerate: `api/internal/db/post_delivery_jobs.sql.go`
- Modify: `api/internal/handler/social_post_queue.go`
- Test: `api/internal/handler/social_post_queue_test.go`

- [ ] **Step 1: Write a failing lost-lease test**

Add `TestFinalizeRestrictedDeliveryJobLostLeasePreservesNewerSuccess`. Simulate `pgx.ErrNoRows` for the lease transition while the result already contains a newer published external ID, URL, and timestamp. Require zero result mutation, failure detail, failure record, parent refresh, and retention transition.

- [ ] **Step 2: Run RED**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/handler \
  -run '^TestFinalizeRestrictedDeliveryJobLostLeasePreservesNewerSuccess$' -count=1 -v
```

Expected: FAIL because the result is currently changed before lease ownership is checked.

- [ ] **Step 3: Add `FinalizeRestrictedPostDeliveryJob`**

In `post_delivery_jobs.sql`, use a `transitioned` CTE that updates the job only when ID, running/retrying state, lease owner, and last-attempt timestamp match. An `updated_result` CTE must update status/error/failure fields and clear `publish_token` only from `transitioned`. Return the transitioned job by joining the updated result. No transition must produce no result and no mutation.

The dependency shape must be:

```sql
WITH transitioned AS (
  UPDATE post_delivery_jobs
  SET state='dead', failure_stage=sqlc.arg(failure_stage),
      error_code=sqlc.arg(error_code), last_error=sqlc.arg(last_error),
      next_run_at=NULL, updated_at=NOW(), finished_at=NOW()
  WHERE id=sqlc.arg(job_id)
    AND state IN ('running','retrying')
    AND lease_owner IS NOT DISTINCT FROM sqlc.arg(lease_owner)
    AND last_attempt_at IS NOT DISTINCT FROM sqlc.arg(last_attempt_at)::timestamptz
  RETURNING *
), updated_result AS (
  UPDATE social_post_results result
  SET status='failed', external_id=NULL, error_message=sqlc.arg(last_error),
      published_at=NULL, url=NULL, debug_curl=NULL,
      error_code=sqlc.arg(error_code), failure_stage=sqlc.arg(failure_stage),
      is_retriable=FALSE, next_action=sqlc.arg(next_action),
      error_source='unipost', error_temporality='temporary',
      provider_error=NULL, publish_token=NULL
  FROM transitioned
  WHERE result.id=transitioned.social_post_result_id
  RETURNING result.id
)
SELECT transitioned.* FROM transitioned
JOIN updated_result ON updated_result.id=transitioned.social_post_result_id;
```

- [ ] **Step 4: Generate, implement, test, and commit**

```bash
cd api
/Users/xiaoboyu/go/bin/sqlc generate
GOCACHE=/tmp/unipost-go-build go test ./internal/handler ./internal/db \
  -run 'FinalizeRestricted|PostDeliveryJob.*Contract|ProcessPostDeliveryJob.*Restriction' -count=1 -v
```

Change `finalizeRestrictedDeliveryJob` to call the generated query first. Return nil immediately on `pgx.ErrNoRows`; perform failure records, logs, parent refresh, and retention only after success.

```bash
git add api/internal/db/queries/post_delivery_jobs.sql api/internal/db/post_delivery_jobs.sql.go \
  api/internal/handler/social_post_queue.go api/internal/handler/social_post_queue_test.go
git commit -m "fix(worker): make restriction finalization lease atomic"
```

Expected: PASS and generated code derives only from the SQL source.

## Task 4: Terminalize unknown email outcomes

**Files:**
- Modify: `api/internal/loops/client.go`
- Modify: `api/internal/loops/client_test.go`
- Modify: `api/internal/worker/publishing_restriction_email.go`
- Modify: `api/internal/worker/publishing_restriction_email_postgres.go`
- Modify: `api/internal/worker/publishing_restriction_email_test.go`
- Test: `api/internal/worker/publishing_restriction_email_postgres_integration_test.go`
- Create: `docs/publishing-restriction-email-campaign-runbook.md`

- [ ] **Step 1: Write failing ambiguity tests**

Use a `RoundTripper` that records one request then returns `errors.New("response lost")`. Require a typed unknown outcome. In the worker require one sender call and terminal failure, not bounded retry. An explicit 503 response must remain definitive and retryable.

- [ ] **Step 2: Run RED**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/loops ./internal/worker \
  -run 'SendOutcomeUnknown|Ambiguous|DefinitiveFailure' -count=1 -v
```

Expected: FAIL because both error classes are currently identical.

- [ ] **Step 3: Add the typed classification**

```go
type SendOutcomeUnknownError struct{ Err error }

func (e *SendOutcomeUnknownError) Error() string {
    return "loops: send outcome unknown: " + e.Err.Error()
}
func (e *SendOutcomeUnknownError) Unwrap() error { return e.Err }

func IsSendOutcomeUnknown(err error) bool {
    var target *SendOutcomeUnknownError
    return errors.As(err, &target)
}
```

Wrap only `http.Client.Do` errors. Nil client/key, marshal/request construction, audit linkage, and explicit non-2xx responses remain definitive.

- [ ] **Step 4: Add terminal recipient persistence**

Extend `PublishingRestrictionEmailStore` with `MarkPublishingRestrictionEmailRecipientTerminalFailed(context.Context,string,string) error`. Its PostgreSQL update must require `status='sending'` and set `status='failed'`, `retryable=FALSE`, `claimed_at=NULL`, and manual-review wording. Leave the schema-required, non-null `next_attempt_at` value intact; `retryable=FALSE` is the authoritative condition that makes the row ineligible for automatic claim. Branch on `loops.IsSendOutcomeUnknown(err)` in the worker.

- [ ] **Step 5: Prove manual retry identity**

In isolated PostgreSQL, terminalize a recipient, record `idempotency_key` and `attempt_generation`, call `RetryFailedCampaign`, and claim again. Assert:

```go
if retried.IdempotencyKey != originalKey {
    t.Fatalf("provider key changed: got %q want %q", retried.IdempotencyKey, originalKey)
}
if retried.AttemptGeneration != originalGeneration+1 {
    t.Fatalf("attempt generation=%d", retried.AttemptGeneration)
}
```

The provider request keeps `cycle:type:canonical-user`; only audit identity appends `:g<generation>:a<count>`.

- [ ] **Step 6: Create the runbook**

Document: unknown outcomes are terminal; inspect recipient/audit/provider evidence; verify delivery before `retry-failed`; stable provider idempotency is defense in depth; never bulk retry unknown outcomes without provider confirmation.

- [ ] **Step 7: Run GREEN and commit**

```bash
PUBLISHING_RESTRICTION_TEST_DATABASE_URL='postgresql://postgres:test@127.0.0.1:5432/unipost_test?sslmode=disable' \
GOCACHE=/tmp/unipost-go-build go test -tags=integration \
  ./internal/loops ./internal/publishingrestrictions ./internal/worker \
  -run 'SendOutcomeUnknown|Ambiguous|DefinitiveFailure|PublishingRestrictionEmail|RetryFailedCampaign' -count=1 -v
git add api/internal/loops/client.go api/internal/loops/client_test.go \
  api/internal/worker/publishing_restriction_email.go \
  api/internal/worker/publishing_restriction_email_postgres.go \
  api/internal/worker/publishing_restriction_email_test.go \
  api/internal/worker/publishing_restriction_email_postgres_integration_test.go \
  docs/publishing-restriction-email-campaign-runbook.md
git commit -m "fix(email): terminalize unknown send outcomes"
```

Expected: PASS with no real network or email send.

## Task 5: Gate campaign actions on complete readiness

**Files:**
- Modify: `api/internal/publishingrestrictions/service.go`
- Modify: `api/internal/publishingrestrictions/service_test.go`
- Modify: `api/internal/handler/publishing_restrictions.go`
- Modify: `api/internal/handler/publishing_restrictions_test.go`
- Modify: `api/internal/worker/publishing_restriction_email.go`
- Modify: `api/internal/worker/publishing_restriction_email_test.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write failing readiness tests**

Remove preview secret, audited sender, restriction template, recovery template, and campaign store one at a time. Preview, confirm, and retry must return `ErrCampaignNotConfigured` before store counters change. Handlers must return 503 `NOT_CONFIGURED`. Worker `ProcessBatch` must return a stable configuration error with `claimCalls==0`; `Start` must warn once and exit.

- [ ] **Step 2: Run RED**

```bash
GOCACHE=/tmp/unipost-go-build go test \
  ./internal/publishingrestrictions ./internal/handler ./internal/worker \
  -run 'Campaign.*NotConfigured|PublishingRestrictionEmail.*NotConfigured|CampaignReadiness' -count=1 -v
```

Expected: FAIL because readiness is currently fragmented.

- [ ] **Step 3: Add immutable readiness**

```go
type CampaignDeliveryReadiness struct {
    PreviewSecret       bool
    AuditedSender       bool
    RestrictionTemplate bool
    RecoveryTemplate    bool
}
func (r CampaignDeliveryReadiness) Ready() bool {
    return r.PreviewSecret && r.AuditedSender &&
        r.RestrictionTemplate && r.RecoveryTemplate
}
var ErrCampaignNotConfigured =
    errors.New("publishing restriction email campaign is not configured")
```

Store readiness on `Service`; additionally require `campaignStore != nil`. Check before preview, confirmation, or retry reads/writes.

- [ ] **Step 4: Map 503 and guard before claim**

Handlers map `ErrCampaignNotConfigured` to:

```go
writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED",
    "Publishing restriction email campaigns are not configured")
```

Pass the same readiness to the worker. `ProcessBatch` checks readiness, non-nil store/sender, and templates before claim. `Start` checks immutable readiness once, logs one warning, and returns instead of entering the ticker.

- [ ] **Step 5: Wire exact startup inputs**

Read the preview secret and template IDs once and construct:

```go
campaignReadiness := publishingrestrictions.CampaignDeliveryReadiness{
    PreviewSecret:       strings.TrimSpace(previewSecret) != "",
    AuditedSender:       auditedLoopsClient != nil,
    RestrictionTemplate: strings.TrimSpace(restrictionTemplateID) != "",
    RecoveryTemplate:    strings.TrimSpace(recoveryTemplateID) != "",
}
```

Pass it to service and worker. Log missing variable names only, never values.

- [ ] **Step 6: Run GREEN and commit**

```bash
GOCACHE=/tmp/unipost-go-build go test \
  ./internal/publishingrestrictions ./internal/handler ./internal/worker ./cmd/api \
  -run 'Campaign|PublishingRestrictionEmail|MissingPublishingRestriction' -count=1 -v
git add api/internal/publishingrestrictions/service.go api/internal/publishingrestrictions/service_test.go \
  api/internal/handler/publishing_restrictions.go api/internal/handler/publishing_restrictions_test.go \
  api/internal/worker/publishing_restriction_email.go api/internal/worker/publishing_restriction_email_test.go \
  api/cmd/api/main.go
git commit -m "fix(email): gate campaigns on delivery readiness"
```

Expected: PASS; unready workers claim zero and do not log every tick.

## Task 6: Correct draft quota accounting

**Files:**
- Modify: `api/internal/handler/social_posts_drafts.go`
- Test: `api/internal/handler/social_posts_quota_test.go`
- Test: `api/internal/handler/social_posts_publishing_restrictions_test.go`

- [ ] **Step 1: Write the mixed-target boundary test**

A claimed draft with restricted TikTok plus allowed Instagram at one remaining unit must be accepted with one quota unit, one restricted result, and one job. Adding a second allowed target must exceed the same boundary.

- [ ] **Step 2: Run RED**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/handler \
  -run '^TestPublishDraftQuotaCountsOnlyAllowedPublishingTargets$' -count=1 -v
```

Expected: FAIL because current draft publish counts all targets.

- [ ] **Step 3: Implement, run GREEN, and commit**

```go
allowedTargets := allowedPublishingTargets(posts, blockedTargets)
quotaUnits := countPublishQuotaUnits(allowedTargets, accountMap)
```

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/handler \
  -run 'PublishDraft|Quota|PublishingRestriction' -count=1 -v
git add api/internal/handler/social_posts_drafts.go \
  api/internal/handler/social_posts_quota_test.go \
  api/internal/handler/social_posts_publishing_restrictions_test.go
git commit -m "fix(quota): exclude restricted draft targets"
```

Expected: PASS. Do not change scheduled-draft edit reservation semantics.

## Task 7: Require frontend contracts in CI

**Files:**
- Modify: `dashboard/package.json`
- Modify: `.github/workflows/ci.yml`
- Create: `scripts/preview/publishing-restriction-ci-contract.test.mjs`

- [ ] **Step 1: Write the failing CI contract**

Parse `dashboard/package.json` and read the workflow. Require the dedicated script to contain all four files and its workflow step to precede build.

```js
const command = packageJson.scripts["test:publishing-restrictions"] ?? "";
for (const file of requiredFiles) assert.match(command, new RegExp(file.replaceAll(".", "\\.")));
assert.ok(workflow.indexOf("npm run test:publishing-restrictions") <
          workflow.indexOf("npm run build"));
```

- [ ] **Step 2: Run RED**

```bash
node --test scripts/preview/publishing-restriction-ci-contract.test.mjs
```

Expected: FAIL because the script and workflow step are absent.

- [ ] **Step 3: Add the package command and CI step**

```json
"test:publishing-restrictions": "node --experimental-strip-types --test src/lib/publishing-restrictions.test.ts tests/admin-publishing-restrictions-source.test.mjs tests/publishing-restrictions-customer-source.test.mjs tests/post-result-errors.test.mts"
```

Add before Dashboard build:

```yaml
- name: Run publishing restriction contracts
  run: npm run test:publishing-restrictions
```

If a new integration-tag handler test is added, include `./internal/handler` and its exact test name in the isolated PostgreSQL CI command.

- [ ] **Step 4: Run GREEN and commit**

```bash
node --test scripts/preview/*.test.mjs
cd dashboard && npm run test:publishing-restrictions
git add dashboard/package.json .github/workflows/ci.yml \
  scripts/preview/publishing-restriction-ci-contract.test.mjs
git commit -m "test(ci): require publishing restriction contracts"
```

Expected: all four frontend files execute with zero skips.

## Task 8: Complete local verification

**Files:**
- Modify only a file required by a proven failure; do not refactor unrelated code.

- [ ] **Step 1: Verify ownership before every suite**

```bash
pwd
git branch --show-current
git status --short
```

Expected path `/Users/xiaoboyu/.codex/worktrees/68f4/unipost`, branch `codex/pr270-review-hardening`, no unrelated state.

- [ ] **Step 2: Run focused and isolated PostgreSQL suites**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/handler ./internal/loops \
  ./internal/publishingrestrictions ./internal/worker ./cmd/api -count=1
PUBLISHING_RESTRICTION_TEST_DATABASE_URL='postgresql://postgres:test@127.0.0.1:5432/unipost_test?sslmode=disable' \
GOCACHE=/tmp/unipost-go-build go test -tags=integration \
  ./internal/db ./internal/handler ./internal/publishingrestrictions ./internal/worker \
  -run 'TestMigrationGatePostgres|TestRequireCurrentSchema|TestPublishingRestriction|TestFinalizeRestricted|TestRetryFailedCampaign' \
  -count=1 -v
```

Expected: PASS. Missing DB, skip, timeout, cancellation, or no result is failure. Never use the shared Railway URL.

- [ ] **Step 3: Run full API checks**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
GOCACHE=/tmp/unipost-go-build go vet ./...
```

Expected: both exit zero.

- [ ] **Step 4: Run Dashboard contracts, build, and browser regression**

```bash
cd dashboard
npm run test:publishing-restrictions
npm run test:docs-ai
npm run test:seo
npm run build
npm run test:regression:dashboard
```

Expected: every command passes and Playwright has zero required skips.

- [ ] **Step 5: Audit the complete local content**

```bash
git diff --check
git status --short
git log --oneline origin/codex/staging-tiktok-free-publishing-restriction..HEAD
git diff --name-status origin/codex/staging-tiktok-free-publishing-restriction...HEAD
```

Expected: only approved commits/files, no secrets, artifacts, environment output, or unrelated changes.

- [ ] **Step 6: Fix only proven failures**

Use `superpowers:systematic-debugging`, reproduce the root cause, add or retain RED coverage, apply the narrow fix, and rerun the complete affected suite. Commit exact files with `fix: resolve third-round verification failure`. If no file changed, create no empty commit.

## Task 9: Independent review and remote delivery

**Files:**
- No source changes unless a reviewer identifies a verified actionable finding.

- [ ] **Step 1: Request fresh complete-diff review**

Use `superpowers:requesting-code-review` and a new read-only reviewer for the exact base-to-HEAD diff. Require all original PRD surfaces, both prior hardening rounds, the backup gate, and all seven third-round fixes. Critical or Important findings return to TDD and block push/integration.

- [ ] **Step 2: Repeat verification on the final tree**

Use `superpowers:verification-before-completion`. Repeat Task 8 after the last review fix and record exact commands, counts, exit status, and SHA.

- [ ] **Step 3: Audit and push only the owned branch**

```bash
pwd
git branch --show-current
git status --short
git log --oneline origin/codex/staging-tiktok-free-publishing-restriction..HEAD
git diff --name-status origin/codex/staging-tiktok-free-publishing-restriction...HEAD
git rev-parse HEAD
git push origin codex/pr270-review-hardening
```

Never push the old base branch, `dev`, `staging`, or `main`.

- [ ] **Step 4: Open a Draft stacked PR**

If the previously merged stacked PR cannot represent new commits, open a new Draft PR with base `codex/staging-tiktok-free-publishing-restriction` and head `codex/pr270-review-hardening`. State that it must merge into PR 270's head before PR 270 is re-reviewed. Do not merge either PR.

- [ ] **Step 5: Monitor the exact SHA**

Wait for every triggered and required GitHub CI, isolated PostgreSQL, Railway Preview, Vercel Preview, deployed regression, and browser acceptance result. A failure, error, timeout, cancellation, skip, missing result, or different SHA is a hard stop.

- [ ] **Step 6: Report**

Report branch, exact SHA, Draft stacked PR URL, unique commits/files, every local and remote result/URL, review verdict, and clean worktree. Explicitly state PR 270 remains unmerged and no staging/main merge, staging/production migration, real email, restriction enablement, promotion, restore, or backup deletion occurred.
