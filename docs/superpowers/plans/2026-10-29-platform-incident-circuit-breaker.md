# Platform Incident Circuit Breaker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an audited, database-backed per-platform circuit breaker that can fully isolate TikTok while other UniPost platforms operate normally.

**Architecture:** A new `platformcontrols` domain owns states, operation evaluation, PostgreSQL persistence, and optimistic-lock administration. Existing orchestration points receive one evaluator and check it before OAuth/Hosted Connect, provider publishing, token refresh, analytics, or other authenticated provider reads. The global control is evaluated before the existing plan publishing restriction and always overrides publish-token resume behavior.

**Tech Stack:** Go, pgx/sqlc-style hand-written store queries, PostgreSQL Goose migrations, chi handlers, Go unit/integration tests.

---

### Task 1: Operational-control domain and schema

**Files:**
- Create: `api/internal/platformcontrols/model.go`
- Create: `api/internal/platformcontrols/service.go`
- Create: `api/internal/platformcontrols/service_test.go`
- Create: `api/internal/platformcontrols/postgres.go`
- Create: `api/internal/db/migrations/138_platform_operational_controls.sql`
- Create: `api/internal/db/platform_operational_controls_migration_test.go`

- [ ] **Step 1: Write failing state-matrix tests**

Test `Service.Evaluate` for `active`, `publishing_disabled`, and `fully_disabled`; require unknown platforms and store errors to return an error so callers fail closed.

```go
func TestServiceEvaluateStateMatrix(t *testing.T) {
    cases := []struct {
        state State
        op Operation
        blocked bool
    }{
        {StateActive, OperationConnect, false},
        {StateActive, OperationPublish, false},
        {StateActive, OperationRead, false},
        {StatePublishingDisabled, OperationConnect, false},
        {StatePublishingDisabled, OperationPublish, true},
        {StatePublishingDisabled, OperationRead, false},
        {StateFullyDisabled, OperationConnect, true},
        {StateFullyDisabled, OperationPublish, true},
        {StateFullyDisabled, OperationRead, true},
    }
    // Fake store returns a control for each case; assert Decision.Blocked.
}
```

- [ ] **Step 2: Run the tests and confirm RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/platformcontrols`

Expected: compilation fails because the package does not exist.

- [ ] **Step 3: Implement the domain and PostgreSQL store**

Define:

```go
type State string
const (
    StateActive State = "active"
    StatePublishingDisabled State = "publishing_disabled"
    StateFullyDisabled State = "fully_disabled"
)

type Operation string
const (
    OperationConnect Operation = "connect"
    OperationPublish Operation = "publish"
    OperationRead Operation = "read"
)

type Evaluator interface {
    Evaluate(context.Context, string, Operation) (Decision, error)
}
```

`Decision` includes platform, state, operation, blocked, reason code, and message. `Service.Evaluate` validates platform and operation and applies the exact matrix tested above. `PostgresStore` implements `ControlForPlatform`, `List`, and `SetState` with an optimistic version check plus an append-only event in one transaction.

Migration 138 creates `platform_operational_controls` and `platform_operational_control_events`, constrains states, and seeds the nine supported platforms as `active` with `ON CONFLICT DO NOTHING`.

- [ ] **Step 4: Add and run the migration source test**

The test reads migration 138 and asserts the two tables, all three states, all nine platform seeds, an event table, and a down migration exist.

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/platformcontrols`

Expected: PASS.

- [ ] **Step 5: Commit the domain**

```bash
git add api/internal/platformcontrols api/internal/db/migrations/138_platform_operational_controls.sql api/internal/db/platform_operational_controls_migration_test.go
git commit -m "feat: add audited platform operational controls"
```

### Task 2: Shared HTTP enforcement and administration

**Files:**
- Create: `api/internal/handler/platform_operational_controls.go`
- Create: `api/internal/handler/platform_operational_controls_test.go`
- Modify: `api/internal/handler/oauth.go`
- Modify: `api/internal/handler/oauth_test.go`
- Modify: `api/internal/handler/connect_sessions.go`
- Modify: `api/internal/handler/connect_sessions_test.go`
- Modify: `api/internal/handler/connect_callback.go`
- Modify: `api/internal/handler/connect_callback_test.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write failing OAuth and Hosted Connect tests**

Use an evaluator returning `fully_disabled` for TikTok and spies whose `GetAuthURL`/`ExchangeCode` methods fail the test if called. Assert:

```go
assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
assert.Contains(t, recorder.Body.String(), "PLATFORM_OPERATION_DISABLED")
assert.Equal(t, 0, providerCalls)
```

For HTML callback routes, assert the existing error redirect is produced before `Connector.ExchangeCode`.

- [ ] **Step 2: Run focused tests and confirm RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'OperationalControl|OAuth.*Disabled|Connect.*Disabled'`

Expected: FAIL because handlers have no operational evaluator.

- [ ] **Step 3: Add shared handler guard and wire connect paths**

Add setters on `OAuthHandler`, `ConnectSessionHandler`, and `ConnectCallbackHandler`. The shared helper calls:

```go
decision, err := evaluator.Evaluate(ctx, platformName, platformcontrols.OperationConnect)
if err != nil || decision.Blocked {
    return &platformOperationDisabledError{Decision: decision, Cause: err}
}
```

Call it before creating OAuth state, generating provider URLs, consuming/exchanging callback codes, or creating Hosted Connect sessions. A DB evaluation error returns 503 and does not call a provider.

- [ ] **Step 4: Add super-admin list/update endpoints**

Implement `GET /v1/admin/platform-operational-controls` and `PATCH /v1/admin/platform-operational-controls/{platform}`. PATCH accepts `state` and `expected_version`, writes actor/request metadata, returns 409 on version conflict, and rejects unknown platform/state values.

- [ ] **Step 5: Wire one service instance in main**

Create the PostgreSQL store/service after the DB pool, pass it to all three handlers, and mount admin endpoints under `RequireSuperAdmin`.

- [ ] **Step 6: Run handler tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler ./cmd/api`

Expected: PASS.

- [ ] **Step 7: Commit connect enforcement**

```bash
git add api/internal/handler api/cmd/api/main.go
git commit -m "feat: enforce platform controls on account connection"
```

### Task 3: Publishing and worker enforcement

**Files:**
- Modify: `api/internal/handler/social_posts_publishing_restrictions.go`
- Modify: `api/internal/handler/social_post_queue.go`
- Modify: `api/internal/handler/social_post_queue_test.go`
- Modify: `api/internal/handler/social_posts_publishing_restrictions_test.go`
- Modify: `api/internal/handler/social_post_retry.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write failing API and persisted-token worker tests**

Add a TikTok adapter spy and a claimed job whose result already contains a `publish_id`. Configure TikTok as `publishing_disabled` and assert the job is finalized without calling `Post`. Add a mixed-platform API test proving TikTok is rejected while LinkedIn remains accepted.

- [ ] **Step 2: Run focused tests and confirm RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'OperationalControl|PersistedToken.*Disabled|MixedPlatform.*Disabled'`

Expected: FAIL because the current persisted-token resume path skips policy evaluation.

- [ ] **Step 3: Add the global control ahead of plan policy**

Add `SetPlatformOperationalControls`. Evaluate `OperationPublish` for every target before the plan restriction. In `ProcessPostDeliveryJob`, always evaluate the operational control before `shouldEvaluateDeliveryPublishingRestriction`; only the plan restriction retains the publish-token resume exception.

Map blocked new requests to `PLATFORM_OPERATION_DISABLED`. Finalize already-queued blocked jobs as temporary failures with `retry_after_platform_recovery`; do not invoke the adapter.

- [ ] **Step 4: Run publishing tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler`

Expected: PASS.

- [ ] **Step 5: Commit publishing enforcement**

```bash
git add api/internal/handler api/cmd/api/main.go
git commit -m "feat: stop disabled platforms before provider publish"
```

### Task 4: Token refresh and analytics enforcement

**Files:**
- Modify: `api/internal/worker/token_refresh.go`
- Modify: `api/internal/worker/token_refresh_test.go`
- Modify: `api/internal/worker/managed_token_refresh.go`
- Modify: `api/internal/worker/managed_token_refresh_test.go`
- Modify: `api/internal/worker/analytics_refresh.go`
- Modify: `api/internal/worker/analytics_refresh_test.go`
- Modify: `api/internal/handler/analytics.go`
- Modify: `api/internal/handler/social_account_metrics.go`
- Modify: `api/internal/handler/tiktok_analytics.go`
- Modify: `api/internal/handler/tiktok_creator_info.go`
- Modify: `api/internal/handler/youtube_analytics.go`
- Modify: `api/internal/handler/meta_platform_analytics.go`
- Modify: `api/internal/handler/pinterest_boards.go`
- Modify: `api/internal/handler/facebook_page_insights.go`
- Modify: `api/internal/handler/facebook_page_analytics.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write failing no-provider-call tests**

For token refresh and analytics workers, configure TikTok `fully_disabled`, provide adapter spies that count calls, run one work item, and assert zero refresh/analytics calls. Add one active LinkedIn case proving unaffected work still runs.

- [ ] **Step 2: Run focused tests and confirm RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/worker ./internal/handler -run 'OperationalControl|FullyDisabled'`

Expected: FAIL because provider methods are still called.

- [ ] **Step 3: Enforce `OperationRead` before authenticated outbound calls**

Add evaluator setters to the workers. Before decrypting tokens or resolving adapters, evaluate the platform. Blocked or evaluation-error rows are skipped and logged without marking the account disconnected. Add the same guard to synchronous analytics, creator-info, board, and page-insight handlers before refresh or provider calls.

- [ ] **Step 4: Run worker and handler suites**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/worker ./internal/handler`

Expected: PASS.

- [ ] **Step 5: Commit outbound-read enforcement**

```bash
git add api/internal/worker api/internal/handler api/cmd/api/main.go
git commit -m "feat: isolate disabled platforms from background provider calls"
```

### Task 5: Production-safe TikTok activation command and documentation

**Files:**
- Create: `api/cmd/platform-control/main.go`
- Create: `api/cmd/platform-control/main_test.go`
- Create: `docs/runbooks/platform-incident-circuit-breaker.md`

- [ ] **Step 1: Write failing command validation tests**

Test that the command rejects a missing platform, unsupported state, missing expected version, and unknown platform before mutation.

- [ ] **Step 2: Run and confirm RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./cmd/platform-control`

Expected: compilation fails because the command does not exist.

- [ ] **Step 3: Implement the explicit mutation command**

The command accepts `--platform`, `--state`, `--expected-version`, and `--reason`; requires `DATABASE_URL`; uses the same service/store; prints only platform, resulting state, version, and changed status; never prints credentials or tokens.

- [ ] **Step 4: Write the recovery runbook**

Document migration/deploy order, the exact TikTok `fully_disabled` command, read-only verification queries, API-first recovery, TikTok rejection probes, worker recovery, rollback, and the condition for returning TikTok to `active`.

- [ ] **Step 5: Run command tests and commit**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./cmd/platform-control`

Expected: PASS.

```bash
git add api/cmd/platform-control docs/runbooks/platform-incident-circuit-breaker.md
git commit -m "ops: add platform circuit breaker runbook"
```

### Task 6: Full verification and release evidence

**Files:**
- Modify only if verification reveals an in-scope defect.

- [ ] **Step 1: Verify worktree and audit branch content**

Run:

```bash
pwd
git branch --show-current
git status --short
git log --oneline origin/main..HEAD
git diff --name-status origin/main...HEAD
```

Expected: only this incident branch/worktree and circuit-breaker changes.

- [ ] **Step 2: Run backend CI-equivalent tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./...`

Expected: PASS with zero skipped/failed packages.

- [ ] **Step 3: Verify migration status and binary builds**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go build ./cmd/api
GOCACHE=/tmp/unipost-go-build go build ./cmd/platform-control
```

Expected: both builds exit 0.

- [ ] **Step 4: Push only the owned branch and monitor all checks**

Push `main-security-incident-investigation-20261029`. Do not update an integration branch directly. Record the exact head SHA and wait for every triggered GitHub, Railway, and Vercel check.

- [ ] **Step 5: Perform production recovery gates**

While API and delivery worker remain offline, apply the migration, set TikTok to `fully_disabled`, and verify the row and audit event. Restore API, verify `/health`, profile visibility, and TikTok rejection. Restore the worker only after those gates pass, then inspect queue health and verify no TikTok provider calls occur.

