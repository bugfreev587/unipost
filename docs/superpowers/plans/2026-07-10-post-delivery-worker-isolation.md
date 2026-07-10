# Post Delivery Worker Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move post delivery execution to an independently scalable worker mode, make job claiming fair across workspaces, and expose delivery phases that distinguish DB queue wait, worker wait, and platform execution.

**Architecture:** Keep the existing `post_delivery_jobs` table and sqlc data layer, adding timing columns and claim ordering instead of introducing an external queue. Refactor the existing API binary into explicit process modes (`api` and `post-delivery-worker`) so Railway can run separate services from the same artifact. Replace per-batch blocking with a bounded local dispatcher that keeps claiming while execution slots are available and enforces per-platform caps inside each worker replica.

**Tech Stack:** Go, chi, pgxpool, sqlc, PostgreSQL migrations, Railway process env, existing Next.js dashboard queue diagnostics.

---

## File Structure

- Modify `api/internal/db/migrations/104_post_delivery_job_phase_timestamps.sql`: add `first_claimed_at` and `platform_started_at`, plus indexes for reserved-job diagnostics.
- Modify `api/internal/db/queries/post_delivery_jobs.sql`: return new columns, set claim timestamps, set platform start timestamp, add fair claim ranking, add queue metric helpers.
- Regenerate `api/internal/db/post_delivery_jobs.sql.go` and `api/internal/db/models.go` with `cd api && sqlc generate`.
- Modify `api/internal/db/post_delivery_jobs_contract_test.go`: lock timestamp and fairness query contracts.
- Modify `api/internal/handler/social_post_queue.go`: derive and expose delivery phase/timing fields, mark platform start before adapter dispatch.
- Modify `api/internal/handler/social_post_queue_test.go`: cover phase derivation and platform-start write contract.
- Modify `api/internal/worker/post_delivery.go`: add configurable worker options, nonblocking execution slots, per-platform semaphores, active logging.
- Modify `api/internal/worker/post_delivery_worker_test.go`: cover bounded concurrency, nonblocking claim cadence, and platform cap behavior.
- Modify `api/cmd/api/main.go`: parse `UNIPOST_PROCESS`, configure DB pool sizes, start delivery workers only in worker mode and HTTP/API workers only in API mode.
- Modify `api/railway.toml`: keep API start command compatible; document worker start env using the same binary.
- Modify `dashboard/src/lib/api.ts` and `dashboard/src/components/posts/list/posts-legacy-list-view.tsx`: type and show derived delivery phase/timestamps without removing existing fields.
- Create or modify `docs/post-delivery-worker-runbook.md`: add support queries and phase interpretation.

---

### Task 1: Delivery Phase Timestamps and API Derivation

**Files:**
- Create: `api/internal/db/migrations/104_post_delivery_job_phase_timestamps.sql`
- Modify: `api/internal/db/queries/post_delivery_jobs.sql`
- Modify: `api/internal/db/post_delivery_jobs_contract_test.go`
- Modify: `api/internal/handler/social_post_queue.go`
- Modify: `api/internal/handler/social_post_queue_test.go`
- Regenerate: `api/internal/db/post_delivery_jobs.sql.go`, `api/internal/db/models.go`

- [ ] **Step 1: Write failing DB contract tests**

Add tests requiring the migration and generated SQL to contain the new timing columns and claim/start writes:

```go
func TestPostDeliveryJobPhaseTimestampMigrationContract(t *testing.T) {
	source, err := os.ReadFile("migrations/104_post_delivery_job_phase_timestamps.sql")
	if err != nil {
		t.Fatalf("read phase timestamp migration: %v", err)
	}
	sql := string(source)
	for _, want := range []string{
		"ADD COLUMN first_claimed_at TIMESTAMPTZ",
		"ADD COLUMN platform_started_at TIMESTAMPTZ",
		"post_delivery_jobs_reserved_idx",
		"DROP COLUMN IF EXISTS platform_started_at",
		"DROP COLUMN IF EXISTS first_claimed_at",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("phase timestamp migration missing %q:\n%s", want, sql)
		}
	}
}

func TestPostDeliveryJobPhaseTimestampQueryContract(t *testing.T) {
	source, err := os.ReadFile("post_delivery_jobs.sql.go")
	if err != nil {
		t.Fatalf("read generated post_delivery_jobs queries: %v", err)
	}
	sql := string(source)
	for _, want := range []string{
		"first_claimed_at = COALESCE(j.first_claimed_at, NOW())",
		"platform_started_at = NOW()",
		"MarkPostDeliveryJobPlatformStarted",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("phase timestamp query contract missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run 'TestPostDeliveryJobPhaseTimestamp' -count=1
```

Expected: FAIL because migration `104_post_delivery_job_phase_timestamps.sql` and generated SQL writes do not exist yet.

- [ ] **Step 3: Add migration**

Create `api/internal/db/migrations/104_post_delivery_job_phase_timestamps.sql`:

```sql
-- +goose Up
ALTER TABLE post_delivery_jobs
  ADD COLUMN first_claimed_at    TIMESTAMPTZ,
  ADD COLUMN platform_started_at TIMESTAMPTZ;

CREATE INDEX post_delivery_jobs_reserved_idx
  ON post_delivery_jobs (last_attempt_at)
  WHERE state IN ('running', 'retrying') AND platform_started_at IS NULL;

CREATE INDEX post_delivery_jobs_platform_duration_idx
  ON post_delivery_jobs (platform_started_at)
  WHERE state IN ('running', 'retrying', 'succeeded', 'failed', 'dead');

-- +goose Down
DROP INDEX IF EXISTS post_delivery_jobs_platform_duration_idx;
DROP INDEX IF EXISTS post_delivery_jobs_reserved_idx;
ALTER TABLE post_delivery_jobs DROP COLUMN IF EXISTS platform_started_at;
ALTER TABLE post_delivery_jobs DROP COLUMN IF EXISTS first_claimed_at;
```

- [ ] **Step 4: Update SQL queries**

In both claim updates set:

```sql
first_claimed_at = COALESCE(j.first_claimed_at, NOW()),
last_attempt_at = NOW(),
platform_started_at = NULL,
```

Add:

```sql
-- name: MarkPostDeliveryJobPlatformStarted :one
UPDATE post_delivery_jobs
SET platform_started_at = COALESCE(platform_started_at, NOW()),
    updated_at = NOW()
WHERE id = $1
  AND state IN ('running', 'retrying')
RETURNING *;
```

- [ ] **Step 5: Regenerate sqlc**

Run:

```bash
cd api && sqlc generate
```

Expected: generated `PostDeliveryJob` includes `FirstClaimedAt pgtype.Timestamptz` and `PlatformStartedAt pgtype.Timestamptz`.

- [ ] **Step 6: Write failing handler phase tests**

Add table tests for:

```go
// pending dispatch + no first_claimed_at -> queued
// pending retry + future next_run_at -> waiting_retry
// pending retry + due next_run_at -> queued_retry
// running + no platform_started_at -> reserved
// running + platform_started_at -> dispatching
// retrying + platform_started_at -> retrying
// succeeded -> published
// dead/failed/cancelled -> failed/cancelled as appropriate
```

Use a helper that calls `postDeliveryJobResponseFromRow` and asserts `DeliveryPhase`.

- [ ] **Step 7: Run handler phase tests to verify they fail**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestPostDeliveryJobResponse.*Phase' -count=1
```

Expected: FAIL because `DeliveryPhase` and timing fields are not exposed.

- [ ] **Step 8: Implement phase derivation and response fields**

Add to `postDeliveryJobResponse`:

```go
DeliveryPhase     string     `json:"delivery_phase"`
QueuedAt          time.Time  `json:"queued_at"`
FirstClaimedAt    *time.Time `json:"first_claimed_at,omitempty"`
PlatformStartedAt *time.Time `json:"platform_started_at,omitempty"`
FinishedAt        *time.Time `json:"finished_at,omitempty"`
QueueWaitMS       *int64     `json:"queue_wait_ms,omitempty"`
WorkerWaitMS      *int64     `json:"worker_wait_ms,omitempty"`
PlatformDurationMS *int64    `json:"platform_duration_ms,omitempty"`
```

Implement a small `deriveDeliveryPhase(row db.PostDeliveryJob, now time.Time) string` helper in `social_post_queue.go`.

- [ ] **Step 9: Mark platform start before adapter dispatch**

In `ProcessPostDeliveryJob`, before the platform adapter call, call:

```go
if _, err := h.queries.MarkPostDeliveryJobPlatformStarted(ctx, job.ID); err != nil {
	return err
}
```

Keep existing pre-publish guards before this call so stale in-memory jobs do not write platform-start or publish.

- [ ] **Step 10: Verify Task 1**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/handler -count=1
```

Expected: PASS.

---

### Task 2: Fair Workspace Claiming

**Files:**
- Modify: `api/internal/db/queries/post_delivery_jobs.sql`
- Modify: `api/internal/db/post_delivery_jobs_contract_test.go`
- Regenerate: `api/internal/db/post_delivery_jobs.sql.go`

- [ ] **Step 1: Write failing fairness contract tests**

Add a test that reads generated SQL and asserts both dispatch and retry claims rank by workspace round before job age:

```go
func TestPostDeliveryJobFairClaimQueryContract(t *testing.T) {
	source, err := os.ReadFile("post_delivery_jobs.sql.go")
	if err != nil {
		t.Fatalf("read generated post_delivery_jobs queries: %v", err)
	}
	sql := string(source)
	for _, want := range []string{
		"ROW_NUMBER() OVER (PARTITION BY j.workspace_id",
		"ORDER BY rn ASC, created_at ASC, id ASC",
		"ORDER BY rn ASC, sort_key ASC, id ASC",
		"active_cnt + rn <= $",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("fair claim query contract missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run TestPostDeliveryJobFairClaimQueryContract -count=1
```

Expected: FAIL because claims still order globally by `created_at` before workspace rank.

- [ ] **Step 3: Update claim ordering**

For dispatch claim, keep existing account serialization and workspace cap but change the eligible ordering:

```sql
eligible AS (
  SELECT id FROM ranked
  WHERE sqlc.arg('workspace_concurrent_cap')::int = 0
     OR active_cnt + rn <= sqlc.arg('workspace_concurrent_cap')::int
  ORDER BY rn ASC, created_at ASC, id ASC
  LIMIT sqlc.arg('batch_limit')::int
  FOR UPDATE SKIP LOCKED
)
```

For retry claim:

```sql
eligible AS (
  SELECT id FROM ranked
  WHERE sqlc.arg('workspace_concurrent_cap')::int = 0
     OR active_cnt + rn <= sqlc.arg('workspace_concurrent_cap')::int
  ORDER BY rn ASC, sort_key ASC, id ASC
  LIMIT sqlc.arg('batch_limit')::int
  FOR UPDATE SKIP LOCKED
)
```

- [ ] **Step 4: Regenerate and verify**

Run:

```bash
cd api && sqlc generate
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run 'TestPostDeliveryJob.*Claim' -count=1
```

Expected: PASS.

---

### Task 3: Bounded, Nonblocking Worker Execution

**Files:**
- Modify: `api/internal/worker/post_delivery.go`
- Modify: `api/internal/worker/post_delivery_worker_test.go`

- [ ] **Step 1: Write failing worker tests**

Add tests for:

```go
func TestPostDispatchWorkerRunOnceDoesNotWaitForClaimedJobsToFinish(t *testing.T) { /* runOnce returns while fake ProcessPostDeliveryJob blocks */ }
func TestPostDeliveryExecutorRespectsGlobalConcurrency(t *testing.T) { /* third job does not start while two slots are full */ }
func TestPostDeliveryExecutorRespectsPlatformConcurrency(t *testing.T) { /* second instagram job waits when instagram cap is 1, twitter can still start */ }
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/worker -run 'TestPost.*(RunOnceDoesNotWait|Respects.*Concurrency)' -count=1
```

Expected: FAIL because `runOnce` waits on the full batch and there is no executor/cap API.

- [ ] **Step 3: Add worker config**

Add:

```go
type PostDeliveryWorkerConfig struct {
	ClaimBatchLimit          int32
	WorkspaceConcurrentCap   int32
	GlobalConcurrency        int
	PlatformConcurrencyCaps  map[string]int
}
```

Add `DefaultPostDeliveryWorkerConfigFromEnv()` reading:

```text
POST_DELIVERY_CLAIM_BATCH_LIMIT=20
POST_DELIVERY_WORKSPACE_CONCURRENT_CAP=30
POST_DELIVERY_GLOBAL_CONCURRENCY=10
POST_DELIVERY_PLATFORM_CAP_INSTAGRAM=3
POST_DELIVERY_PLATFORM_CAP_TIKTOK=3
POST_DELIVERY_PLATFORM_CAP_TWITTER=5
```

- [ ] **Step 4: Add local executor**

Implement a shared executor with a global semaphore and per-platform semaphores. `runOnce` should claim only up to free global slots, enqueue claimed jobs, start heartbeat per job, and return without waiting for job completion.

- [ ] **Step 5: Verify worker tests**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/worker -count=1
```

Expected: PASS.

---

### Task 4: Process Mode and DB Pool Configuration

**Files:**
- Modify: `api/cmd/api/main.go`
- Modify: `api/railway.toml`
- Create: `api/cmd/api/main_process_mode_test.go` if the helper is testable from package `main`

- [ ] **Step 1: Write failing process config tests**

Add tests for helpers:

```go
func TestProcessModeDefaultsToAPI(t *testing.T) { /* unset -> api */ }
func TestProcessModeAcceptsPostDeliveryWorker(t *testing.T) { /* post-delivery-worker accepted */ }
func TestDBPoolMaxConnsUsesWorkerDefaultInWorkerMode(t *testing.T) { /* worker default >= global concurrency + support margin */ }
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./cmd/api -run 'Test(ProcessMode|DBPool)' -count=1
```

Expected: FAIL because helpers do not exist.

- [ ] **Step 3: Parse mode and pool config**

Add:

```go
const (
	processModeAPI                = "api"
	processModePostDeliveryWorker = "post-delivery-worker"
)
```

Use `pgxpool.ParseConfig(databaseURL)`, set `MaxConns` from:

```text
DATABASE_MAX_CONNS
API_DATABASE_MAX_CONNS
POST_DELIVERY_WORKER_DATABASE_MAX_CONNS
```

Log `process_mode` and `db_pool_max_conns` at startup.

- [ ] **Step 4: Split startup**

In API mode, start HTTP server and existing non-delivery workers. Do not start post dispatch, post retry, or delivery cleanup workers.

In worker mode, start scheduler only if explicitly configured with `POST_DELIVERY_WORKER_RUN_SCHEDULER=true`, plus post dispatch, post retry, and delivery cleanup workers. Do not start the HTTP server.

- [ ] **Step 5: Verify process tests**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./cmd/api -count=1
```

Expected: PASS.

---

### Task 5: Dashboard Queue Diagnostics and Runbook

**Files:**
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/components/posts/list/posts-legacy-list-view.tsx`
- Create: `docs/post-delivery-worker-runbook.md`

- [ ] **Step 1: Update dashboard types**

Add optional fields to `PostDeliveryJob`:

```ts
delivery_phase: "queued" | "waiting_retry" | "queued_retry" | "reserved" | "dispatching" | "retrying" | "published" | "failed" | "cancelled";
queued_at: string;
first_claimed_at?: string;
platform_started_at?: string;
finished_at?: string;
queue_wait_ms?: number;
worker_wait_ms?: number;
platform_duration_ms?: number;
```

- [ ] **Step 2: Update diagnostics display**

Show `Delivery phase`, `First claimed`, `Platform started`, `Finished`, and wait durations when present. Keep existing `state`, `kind`, `attempts`, and failure fields.

- [ ] **Step 3: Add runbook**

Create `docs/post-delivery-worker-runbook.md` with SQL snippets for:

```sql
SELECT * FROM post_delivery_jobs WHERE state = 'pending' ORDER BY created_at ASC LIMIT 20;
SELECT * FROM post_delivery_jobs WHERE state IN ('running','retrying') AND platform_started_at IS NULL ORDER BY last_attempt_at ASC LIMIT 20;
SELECT lease_owner, COUNT(*) FROM post_delivery_jobs WHERE state IN ('running','retrying') GROUP BY lease_owner;
```

- [ ] **Step 4: Verify dashboard build**

Run:

```bash
cd dashboard && npm run build
```

Expected: PASS.

---

### Task 6: Full Local Verification and Dev Merge

**Files:**
- All changed files

- [ ] **Step 1: Run backend focused tests**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/handler ./internal/worker ./cmd/api -count=1
```

Expected: PASS.

- [ ] **Step 2: Run backend full tests**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 3: Run dashboard build**

Run:

```bash
cd dashboard && npm run build
```

Expected: PASS.

- [ ] **Step 4: Merge into local dev**

Run:

```bash
git fetch origin
git switch dev
git pull --ff-only origin dev
git merge --no-ff dev-post-delivery-worker-isolation
cd api && GOCACHE=/tmp/unipost-go-build go test ./... -count=1
cd ../dashboard && npm run build
```

Expected: merge succeeds and validation passes on local `dev`.

---

### Task 7: Standard Release Flow Verification

**Files:**
- Remote branches and deployment environments

- [ ] **Step 1: Push development**

Run:

```bash
git push origin dev
```

Expected: push succeeds and triggers development deployments/checks.

- [ ] **Step 2: Monitor development**

Use GitHub/Vercel/Railway/CLI checks until the development API, app, and worker deployment are complete. Verify on development domains:

```text
https://dev-api.unipost.dev
https://dev-app.unipost.dev
```

Expected: API health works, queue responses include `delivery_phase`, and a synthetic contended queue shows the unrelated workspace job claimed promptly.

- [ ] **Step 3: Promote dev to staging**

Create PR `dev -> staging`, wait for checks, merge, then wait for staging deployment.

Expected: staging domains are healthy and the changed queue/worker behavior is verified at:

```text
https://staging-api.unipost.dev
https://staging-app.unipost.dev
```

- [ ] **Step 4: Promote staging to production**

Create PR `staging -> main`, wait for checks, merge, then wait for production deployment.

Expected: production domains are healthy and the critical queue response path works at:

```text
https://api.unipost.dev
https://app.unipost.dev
```

---

## Self-Review

- Spec coverage: PRD phases 1-5 are represented. Phase 5 dashboard polish is kept narrow to queue diagnostics and runbook so the first release remains backend-safe.
- Placeholder scan: No `TBD`, `TODO`, or undefined task owner remains.
- Type consistency: API response names match PRD field names and dashboard type names.
- TDD coverage: Each behavior-changing task starts with a failing test and an expected red/green command.
