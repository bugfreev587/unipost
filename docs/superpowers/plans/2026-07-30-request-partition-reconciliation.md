# Request Partition Reconciliation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make request-event partition maintenance reconcile manifest metadata with PostgreSQL catalog state safely and remove all session-time-zone dependence from weekly duration validation.

**Architecture:** Treat PostgreSQL attachment plus exact range bound as the physical authority and the manifest as operational metadata. `Ensure` and `Inspect` will share one catalog-state reader; only a fully valid physical pair with a missing manifest is self-healed, while missing, partial, misattached, or wrong-bound relations fail closed with an actionable drift error. A forward migration replaces the calendar-day manifest constraint with an absolute 604800-second constraint.

**Tech Stack:** Go 1.x, pgx v5, PostgreSQL 16 partition catalogs, Goose SQL migrations, Node release-contract tests.

---

### Task 1: Reproduce catalog/manifest drift in PostgreSQL integration tests

**Files:**
- Modify: `api/internal/requesteventpartitions/postgres_integration_test.go`

- [ ] **Step 1: Start an isolated PostgreSQL 16 test server**

```bash
docker run --rm --detach \
  --name unipost-request-partition-reconcile-postgres \
  --env POSTGRES_PASSWORD=test \
  --publish 127.0.0.1:55437:5432 \
  postgres:16-alpine
until docker exec unipost-request-partition-reconcile-postgres \
  pg_isready -U postgres; do sleep 1; done
docker exec unipost-request-partition-reconcile-postgres \
  createdb -U postgres unipost_partition_test
docker exec unipost-request-partition-reconcile-postgres \
  createdb -U postgres unipost_migration_test
export REQUEST_EVENTS_TEST_DATABASE_URL='postgresql://postgres:test@127.0.0.1:55437/unipost_partition_test?sslmode=disable'
export GOOSE_MIGRATION_TEST_DATABASE_URL='postgresql://postgres:test@127.0.0.1:55437/unipost_migration_test?sslmode=disable'
```

- [ ] **Step 2: Add a failing test for manifest-present/catalog-missing drift**

Add a test that inserts a canonical manifest row without creating either child, calls `Ensure`, asserts a typed `PartitionDriftError`, and confirms neither child is silently recreated.

```go
func TestPostgresStoreEnsureRejectsManifestWhosePhysicalPairIsMissing(t *testing.T) {
	pool := openPartitionIntegrationPool(t)
	week := futureWeek(t, 1)
	insertManifestWeek(t, pool, week)

	err := NewPostgresStore(pool).Ensure(context.Background(), []Week{week})
	var drift *PartitionDriftError
	if !errors.As(err, &drift) {
		t.Fatalf("error = %v, want PartitionDriftError", err)
	}
	assertRelationMissing(t, pool, week.EventTable)
	assertRelationMissing(t, pool, week.DetailTable)
}
```

- [ ] **Step 3: Add a failing test for safe manifest repair**

Create the two canonical partitions manually with exact bounds but no manifest, call `Ensure`, and assert that one canonical manifest row is inserted without replacing either relation.

```go
func TestPostgresStoreEnsureRepairsMissingManifestForValidPhysicalPair(t *testing.T) {
	pool := openPartitionIntegrationPool(t)
	week := futureWeek(t, 8)
	createPhysicalPair(t, pool, week)

	if err := NewPostgresStore(pool).Ensure(context.Background(), []Week{week}); err != nil {
		t.Fatal(err)
	}
	assertPartitionAttached(t, pool, week.EventTable, "api_request_events")
	assertPartitionAttached(t, pool, week.DetailTable, "api_request_error_details")
	assertManifestWeek(t, pool, week)
}
```

- [ ] **Step 4: Add failing tests for partial and wrong-bound physical state**

Cover an event-only partition and an event/detail pair where one child uses a different week range. Both must return `PartitionDriftError`, leave the catalog untouched, and avoid inserting manifest metadata.

- [ ] **Step 5: Run the integration package and verify RED**

Run:

```bash
REQUEST_EVENTS_TEST_DATABASE_URL='postgresql://postgres:test@127.0.0.1:55437/unipost_partition_test?sslmode=disable' \
  go test -tags=integration ./internal/requesteventpartitions \
  -run 'TestPostgresStoreEnsure(RejectsManifestWhosePhysicalPairIsMissing|RepairsMissingManifestForValidPhysicalPair|RejectsPartialPhysicalPair|RejectsWrongBoundPhysicalPair)' \
  -count=1 -v
```

Expected: FAIL because `PartitionDriftError` and catalog reconciliation do not yet exist; the valid physical pair currently reaches duplicate `CREATE TABLE`.

### Task 2: Share catalog truth between Ensure and Inspect

**Files:**
- Modify: `api/internal/requesteventpartitions/postgres.go`
- Modify: `api/internal/requesteventpartitions/postgres_integration_test.go`

- [ ] **Step 1: Add typed drift state and a shared relation-state reader**

Introduce these internal states:

```go
type PartitionDriftError struct {
	Week   Week
	Reason string
}

type partitionRelationState struct {
	Exists       bool
	Attached     bool
	BoundMatches bool
}

func (state partitionRelationState) valid() bool {
	return state.Exists && state.Attached && state.BoundMatches
}
```

The catalog query must resolve the canonical child in `current_schema()`, require `relispartition`, require an exact `pg_inherits` parent in the same schema, and compare `pg_get_expr(relpartbound, child.oid, true)` with the canonical UTC `FOR VALUES FROM ... TO ...` expression. Both `Ensure` and `Inspect` call this reader.

- [ ] **Step 2: Force UTC for catalog rendering while retaining an absolute data model**

After the transaction settings, add:

```go
if _, err := tx.Exec(ctx, `SET LOCAL TIME ZONE 'UTC'`); err != nil {
	return fmt.Errorf("set partition maintenance time zone: %w", err)
}
```

Apply the equivalent read-only error path in `Inspect` so catalog bound rendering is deterministic.

- [ ] **Step 3: Implement the finite reconciliation state machine**

Use these exact decisions in `ensureWeek`:

```text
manifest present + both physical children valid -> success
manifest present + any physical child invalid -> PartitionDriftError
manifest absent + both physical children absent -> check defaults, create pair, insert manifest
manifest absent + both physical children valid -> insert manifest only
manifest absent + any other physical state -> PartitionDriftError
```

Do not use `CREATE TABLE IF NOT EXISTS`; do not recreate a missing child when manifest history says it previously existed.

- [ ] **Step 4: Make Inspect validate the same physical state and detect orphan children**

Load manifest rows, validate each row's canonical ISO week, absolute duration, names, attachment, and exact bound with the shared reader. Count any non-default child attached to either parent but absent from the corresponding manifest column as drift. Append `ReasonPartitionMismatch` once whenever either class of drift exists.

- [ ] **Step 5: Run reconciliation integration tests and verify GREEN**

Run the Task 1 command plus:

```bash
REQUEST_EVENTS_TEST_DATABASE_URL='postgresql://postgres:test@127.0.0.1:55437/unipost_partition_test?sslmode=disable' \
  go test -tags=integration ./internal/requesteventpartitions -count=1 -v
```

Expected: PASS.

- [ ] **Step 6: Commit the catalog reconciliation**

```bash
git add api/internal/requesteventpartitions/postgres.go \
  api/internal/requesteventpartitions/postgres_integration_test.go
git commit -m "fix: reconcile request partition catalog state"
```

### Task 3: Reproduce and remove DST dependence

**Files:**
- Create: `api/internal/db/migrations/134_api_request_partition_manifest_duration.sql`
- Create: `api/internal/db/api_request_event_partition_duration_migration_test.go`
- Modify: `api/internal/requesteventpartitions/postgres.go`
- Modify: `api/internal/requesteventpartitions/postgres_integration_test.go`
- Modify: `api/internal/db/migrate_test.go`
- Modify: `api/internal/db/migration_gate_postgres_integration_test.go`
- Modify: `api/internal/db/migration_gate_test.go`
- Modify: `.github/workflows/ci.yml`
- Modify: `scripts/preview/release-guardrails.test.mjs`

- [ ] **Step 1: Add a failing runtime test across a DST transition**

Acquire one PostgreSQL connection, set `TIME ZONE 'America/Los_Angeles'`, and run `Ensure` for the UTC week beginning `2027-03-08`. Assert that `Ensure` and `Inspect` succeed. On the current constraint, inserting the exact UTC Monday-to-Monday manifest row fails because `INTERVAL '7 days'` is calendar-based.

- [ ] **Step 2: Add a failing migration contract test**

Require migration 134 to drop the old check and add a named constraint using an absolute duration:

```sql
CHECK (EXTRACT(EPOCH FROM (week_end - week_start)) = 604800)
```

The test must also require a reversible Down section and reject `week_start + INTERVAL '7 days'` in the Up section.

- [ ] **Step 3: Verify RED**

Run:

```bash
go test ./internal/db -run TestAPIRequestEventPartitionDurationMigrationContract -count=1 -v
```

Expected: FAIL because migration 134 does not exist.

Run the targeted DST integration test with `REQUEST_EVENTS_TEST_DATABASE_URL`; expected: FAIL on the existing time-zone-sensitive manifest constraint.

- [ ] **Step 4: Add migration 134**

Create a reversible migration that replaces `api_request_partition_manifest_check` with `api_request_partition_manifest_week_duration_check` using the 604800-second expression. Down restores the original constraint name and expression for Goose reversibility.

- [ ] **Step 5: Replace runtime and test-schema duration checks**

Use absolute subtraction/EPOCH in `Inspect`, the integration seed schema, and the full embedded-migration contract. The full migration test must set `America/Los_Angeles`, insert a correct DST-crossing week, and reject a 167-hour week.

- [ ] **Step 6: Advance schema-version contracts to 134**

Update final-version assertions, rename `TestRequireCurrentSchemaRejects124AndAccepts133` to `...Accepts134`, update expected error messages, and synchronize the exact CI selector and release-guardrail fixtures. Preserve `TestMigration133UpgradeAndGuardedDown` because migration 133 rollback behavior remains independently required.

- [ ] **Step 7: Run targeted unit, migration, and release-contract tests**

```bash
go test ./internal/db ./internal/requesteventpartitions -count=1
node --test scripts/preview/release-guardrails.test.mjs
```

Expected: PASS.

- [ ] **Step 8: Commit the DST-safe migration**

```bash
git add api/internal/db api/internal/requesteventpartitions \
  .github/workflows/ci.yml scripts/preview/release-guardrails.test.mjs
git commit -m "fix: make request partition weeks timezone independent"
```

### Task 4: Document catalog-drift recovery

**Files:**
- Modify: `docs/operations/request-event-default-partition-recovery.md`

- [ ] **Step 1: Add a manifest/catalog drift decision table**

Document the five reconciliation states from Task 2. Explicitly authorize automatic manifest repair only when both canonical children are attached to the expected parents with exact UTC bounds. Require a locked backup and operator review for all destructive or data-recovery work.

- [ ] **Step 2: Replace the runbook's calendar-day boundary check**

Use:

```sql
EXTRACT(EPOCH FROM (
  :'week_end'::timestamptz - :'week_start'::timestamptz
)) = 604800
```

- [ ] **Step 3: Add read-only catalog evidence queries**

Include queries for relation existence, exact parent attachment, `relispartition`, and `pg_get_expr(relpartbound, oid, true)`. State that operators must not use `IF NOT EXISTS`, `CASCADE`, or recreate a manifest-backed missing partition without a data-loss investigation.

- [ ] **Step 4: Commit the runbook update**

```bash
git add docs/operations/request-event-default-partition-recovery.md \
  docs/superpowers/plans/2026-07-30-request-partition-reconciliation.md
git commit -m "docs: add request partition drift recovery"
```

### Task 5: Verify, publish, and refresh the promotion PR

**Files:**
- Verify all files changed by Tasks 1–4

- [ ] **Step 1: Run formatting and diff checks**

```bash
gofmt -w api/internal/requesteventpartitions/postgres.go \
  api/internal/requesteventpartitions/postgres_integration_test.go \
  api/internal/db/api_request_event_partition_duration_migration_test.go
git diff --check
```

- [ ] **Step 2: Run complete local validation**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./... -count=1
go vet ./...
REQUEST_EVENTS_TEST_DATABASE_URL='postgresql://postgres:test@127.0.0.1:55437/unipost_partition_test?sslmode=disable' \
  go test -tags=integration ./internal/requesteventpartitions -count=1 -v
GOOSE_MIGRATION_TEST_DATABASE_URL='postgresql://postgres:test@127.0.0.1:55437/unipost_migration_test?sslmode=disable' \
  go test ./internal/db -run TestRunMigrationsAppliesAllEmbeddedMigrationsWithGoose -count=1 -v
cd ..
node --test scripts/preview/*.test.mjs
```

Expected: all commands PASS.

- [ ] **Step 3: Audit branch content**

Confirm every commit and changed file relative to `origin/dev` belongs to catalog reconciliation, DST safety, CI synchronization, or the recovery runbook.

- [ ] **Step 4: Push and create a Draft PR to dev**

Push only `dev-request-partition-reconciliation` and create a Draft PR targeting `dev`. Do not merge until CI, PostgreSQL integration, Railway PR Environment, Vercel Preview, deployed regression, and browser acceptance all succeed on the exact head SHA.

- [ ] **Step 5: Merge to dev only after Preview Acceptance**

After every gate passes, re-audit commits/files, mark ready, merge to `dev`, wait for persistent dev deployments, and verify official dev. PR #307 must remain unmerged; it will refresh automatically to the new dev merge SHA, after which its counts, migration range, risks, and validation evidence must be updated again.
