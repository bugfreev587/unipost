# Request Event Partition Safety Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent request-event writes from accumulating in default partitions by adding a static bridge, bounded automatic partition maintenance, deployment readiness enforcement, aggregate health inspection, and a reviewed missed-window recovery procedure.

**Architecture:** A new `internal/requesteventpartitions` package owns ISO-week planning, PostgreSQL partition creation/inspection, runtime scheduling, and aggregate status. Migration 133 supplies eight static bridge weeks through October 5, while both the Railway `migrate` command and the API worker call the same idempotent maintainer. Automation never moves or deletes default rows; any default occupancy fails closed and routes operators to a backup-gated recovery runbook.

**Tech Stack:** Go 1.24, pgx v5, PostgreSQL 16 declarative partitioning, Goose migrations, Chi HTTP, GitHub Actions, Railway pre-deploy.

---

## File map

- Create `api/internal/db/migrations/133_api_request_event_partition_bridge.sql` for the bounded W33-W40 bridge.
- Create `api/internal/db/api_request_event_partition_bridge_migration_test.go` for bridge and guarded-Down contracts.
- Modify `api/internal/db/migrate_test.go`, `api/internal/db/migration_gate_postgres_integration_test.go`, and `.github/workflows/ci.yml` for schema version 133 and explicit integration coverage.
- Create `api/internal/requesteventpartitions/planner.go` and `planner_test.go` for deterministic ISO-week plans.
- Create `api/internal/requesteventpartitions/postgres.go` and `postgres_integration_test.go` for fail-closed ensure/inspection.
- Create `api/internal/requesteventpartitions/worker.go` and `worker_test.go` for six-hour maintenance and process-local statistics.
- Create `api/internal/requesteventpartitions/handler.go` and `handler_test.go` for aggregate Super Admin status.
- Modify `api/cmd/api/migration_command.go` and `migration_command_test.go` for the bounded post-Goose deploy gate.
- Modify `api/cmd/api/main.go` and create `api/cmd/api/request_event_partition_wiring_test.go` for runtime wiring and route protection.
- Create `docs/operations/request-event-default-partition-recovery.md` for the dry-run-first missed-window procedure.
- Modify `docs/superpowers/specs/2026-07-29-request-event-partition-safety-design.md` only if implementation discovers a factual mismatch.

## Task 1: Add the static W33-W40 bridge migration

**Files:**
- Create: `api/internal/db/api_request_event_partition_bridge_migration_test.go`
- Create: `api/internal/db/migrations/133_api_request_event_partition_bridge.sql`
- Modify: `api/internal/db/migrate_test.go`
- Modify: `api/internal/db/migration_gate_postgres_integration_test.go`

- [ ] **Step 1: Write the failing migration contract tests**

Add tests that read migration 133 and require:

```go
func TestAPIRequestEventPartitionBridgeMigrationContract(t *testing.T) {
	body, err := os.ReadFile("migrations/133_api_request_event_partition_bridge.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(body))
	for _, required := range []string{
		"-- unipost:safety reversible",
		"set local lock_timeout = '5s'",
		"set local statement_timeout = '30s'",
		"api_request_events_default",
		"api_request_error_details_default",
		"2026-08-10 00:00:00+00",
		"2026-10-05 00:00:00+00",
		"api_request_events_2026w33",
		"api_request_error_details_2026w40",
		"api_request_partition_manifest",
		"bridge partition contains data",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration 133 missing %q", required)
		}
	}
	for _, week := range []string{"33", "34", "35", "36", "37", "38", "39", "40"} {
		if !strings.Contains(sql, "api_request_events_2026w"+week) ||
			!strings.Contains(sql, "api_request_error_details_2026w"+week) {
			t.Fatalf("migration 133 missing aligned week %s", week)
		}
	}
}

func TestAPIRequestEventPartitionBridgeDownIsDataPreserving(t *testing.T) {
	body, err := os.ReadFile("migrations/133_api_request_event_partition_bridge.sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(strings.ToLower(string(body)), "-- +goose down")
	if len(parts) != 2 {
		t.Fatal("migration 133 must have one Down section")
	}
	down := parts[1]
	for _, required := range []string{
		"api_request_events_2026w33",
		"api_request_error_details_2026w40",
		"raise exception",
		"delete from api_request_partition_manifest",
	} {
		if !strings.Contains(down, required) {
			t.Fatalf("migration 133 Down missing %q", required)
		}
	}
}
```

Add a PostgreSQL integration test,
`TestMigration133UpgradeAndGuardedDown`, that runs the embedded migrations in
an isolated schema, verifies W33-W40 are attached and manifest-aligned, inserts
one W33 event, and proves migration 133 Down raises
`bridge partition contains data`. Delete the fixture row, rerun Down, and prove
all sixteen bridge children and eight manifest rows are removed.

Update version assertions from 132 to 133 and rename
`TestRequireCurrentSchemaRejects124AndAccepts132` to
`TestRequireCurrentSchemaRejects124AndAccepts133`.

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/db \
  -run 'TestAPIRequestEventPartitionBridge|TestLatestEmbeddedMigrationVersion' \
  -count=1 -v
```

Expected: FAIL because migration 133 does not exist and the latest embedded
version is still 132.

- [ ] **Step 3: Implement migration 133**

Use one Goose transaction with:

```sql
-- +goose Up
-- unipost:safety reversible

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

DO $bridge$
DECLARE
  item RECORD;
BEGIN
  FOR item IN
    SELECT *
    FROM (VALUES
      ('2026-08-10 00:00:00+00'::TIMESTAMPTZ, '2026-08-17 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w33', 'api_request_error_details_2026w33'),
      ('2026-08-17 00:00:00+00'::TIMESTAMPTZ, '2026-08-24 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w34', 'api_request_error_details_2026w34'),
      ('2026-08-24 00:00:00+00'::TIMESTAMPTZ, '2026-08-31 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w35', 'api_request_error_details_2026w35'),
      ('2026-08-31 00:00:00+00'::TIMESTAMPTZ, '2026-09-07 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w36', 'api_request_error_details_2026w36'),
      ('2026-09-07 00:00:00+00'::TIMESTAMPTZ, '2026-09-14 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w37', 'api_request_error_details_2026w37'),
      ('2026-09-14 00:00:00+00'::TIMESTAMPTZ, '2026-09-21 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w38', 'api_request_error_details_2026w38'),
      ('2026-09-21 00:00:00+00'::TIMESTAMPTZ, '2026-09-28 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w39', 'api_request_error_details_2026w39'),
      ('2026-09-28 00:00:00+00'::TIMESTAMPTZ, '2026-10-05 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w40', 'api_request_error_details_2026w40')
    ) AS weeks(week_start, week_end, event_partition, detail_partition)
  LOOP
    IF EXISTS (
      SELECT 1 FROM api_request_events_default
      WHERE occurred_at >= item.week_start AND occurred_at < item.week_end
    ) OR EXISTS (
      SELECT 1 FROM api_request_error_details_default
      WHERE occurred_at >= item.week_start AND occurred_at < item.week_end
    ) THEN
      RAISE EXCEPTION 'bridge partition contains data for [% - %)', item.week_start, item.week_end;
    END IF;

    EXECUTE format(
      'CREATE TABLE %I PARTITION OF api_request_events FOR VALUES FROM (%L) TO (%L)',
      item.event_partition, item.week_start, item.week_end
    );
    EXECUTE format(
      'CREATE TABLE %I PARTITION OF api_request_error_details FOR VALUES FROM (%L) TO (%L)',
      item.detail_partition, item.week_start, item.week_end
    );
    INSERT INTO api_request_partition_manifest (
      week_start, week_end, event_partition, detail_partition
    ) VALUES (
      item.week_start, item.week_end, item.event_partition, item.detail_partition
    );
  END LOOP;
END
$bridge$;
```

The Down block must inspect all sixteen bridge children and raise
`bridge partition contains data` if any row exists. Only when all are empty,
delete the eight manifest rows and drop detail children before event children.

- [ ] **Step 4: Run migration tests and verify GREEN**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/db \
  -run 'TestAPIRequestEventPartitionBridge|TestLatestEmbeddedMigrationVersion' \
  -count=1 -v
```

Expected: PASS.

- [ ] **Step 5: Commit the bridge**

```bash
git add \
  api/internal/db/api_request_event_partition_bridge_migration_test.go \
  api/internal/db/migrations/133_api_request_event_partition_bridge.sql \
  api/internal/db/migrate_test.go \
  api/internal/db/migration_gate_postgres_integration_test.go
git commit -m "feat: bridge request event partitions through October"
```

## Task 2: Implement deterministic ISO-week planning

**Files:**
- Create: `api/internal/requesteventpartitions/planner_test.go`
- Create: `api/internal/requesteventpartitions/planner.go`

- [ ] **Step 1: Write planner tests**

Define the public contract through tests:

```go
func TestPlanWeeksUsesISOWeekYearAndEightWeekHorizon(t *testing.T) {
	got, err := PlanWeeks(time.Date(2026, 12, 31, 23, 0, 0, 0, time.UTC), 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 9 {
		t.Fatalf("weeks = %d, want 9", len(got))
	}
	if got[0].Start != time.Date(2026, 12, 28, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("first start = %v", got[0].Start)
	}
	if got[0].EventTable != "api_request_events_2026w53" {
		t.Fatalf("first event table = %q", got[0].EventTable)
	}
	if got[1].EventTable != "api_request_events_2027w01" {
		t.Fatalf("second event table = %q", got[1].EventTable)
	}
}

func TestExplicitCoverageDaysUsesWholeFutureDays(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	if got := ExplicitCoverageDays(now, end); got != 18 {
		t.Fatalf("coverage = %d, want 18", got)
	}
}

func TestPartitionSuffixRejectsInvalidIdentifiers(t *testing.T) {
	if _, err := weekForStart(time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("non-Monday boundary must fail")
	}
}
```

- [ ] **Step 2: Run planner tests and verify RED**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/requesteventpartitions \
  -run 'TestPlanWeeks|TestExplicitCoverage|TestPartitionSuffix' \
  -count=1 -v
```

Expected: FAIL because the package and functions do not exist.

- [ ] **Step 3: Implement the planner**

Create:

```go
package requesteventpartitions

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

const (
	MaintenanceWeeksAhead = 8
	MinimumCoverageDays    = 14
	PartitionLockNamespace int32 = 0x52515054
	PartitionLockKey       int32 = 1
)

var partitionSuffixPattern = regexp.MustCompile(`^[0-9]{4}w[0-9]{2}$`)

type Week struct {
	Start       time.Time
	End         time.Time
	EventTable  string
	DetailTable string
}

func PlanWeeks(now time.Time, weeksAhead int) ([]Week, error) {
	if weeksAhead < 0 {
		return nil, errors.New("weeks ahead must be nonnegative")
	}
	start := mondayUTC(now)
	result := make([]Week, 0, weeksAhead+1)
	for offset := 0; offset <= weeksAhead; offset++ {
		week, err := weekForStart(start.AddDate(0, 0, 7*offset))
		if err != nil {
			return nil, err
		}
		result = append(result, week)
	}
	return result, nil
}

func mondayUTC(value time.Time) time.Time {
	utc := value.UTC()
	day := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	offset := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -offset)
}

func weekForStart(start time.Time) (Week, error) {
	start = start.UTC()
	if start.Hour() != 0 || start.Minute() != 0 || start.Second() != 0 ||
		start.Nanosecond() != 0 || start.Weekday() != time.Monday {
		return Week{}, errors.New("week start must be Monday 00:00:00 UTC")
	}
	year, number := start.ISOWeek()
	suffix := fmt.Sprintf("%04dw%02d", year, number)
	if !partitionSuffixPattern.MatchString(suffix) {
		return Week{}, errors.New("generated partition suffix is invalid")
	}
	return Week{
		Start:       start,
		End:         start.AddDate(0, 0, 7),
		EventTable:  "api_request_events_" + suffix,
		DetailTable: "api_request_error_details_" + suffix,
	}, nil
}

func ExplicitCoverageDays(now, latestEnd time.Time) int {
	duration := latestEnd.UTC().Sub(now.UTC())
	if duration <= 0 {
		return 0
	}
	return int(duration / (24 * time.Hour))
}
```

- [ ] **Step 4: Run planner tests and verify GREEN**

Run the command from Step 2. Expected: PASS.

- [ ] **Step 5: Commit the planner**

```bash
git add api/internal/requesteventpartitions/planner.go \
  api/internal/requesteventpartitions/planner_test.go
git commit -m "feat: plan request event partitions by ISO week"
```

## Task 3: Implement fail-closed PostgreSQL Ensure

**Files:**
- Create: `api/internal/requesteventpartitions/postgres.go`
- Create: `api/internal/requesteventpartitions/postgres_integration_test.go`

- [ ] **Step 1: Write PostgreSQL integration tests for Ensure**

Use `REQUEST_EVENTS_TEST_DATABASE_URL` through `testdbguard.OpenValidated`.
Create unique future weeks in 2027 and clean them up only when empty. Cover:

```go
func TestPostgresStoreEnsureCreatesAlignedPairsAndIsIdempotent(t *testing.T)
func TestPostgresStoreEnsureRejectsEventDefaultRows(t *testing.T)
func TestPostgresStoreEnsureRejectsAlignedEventAndDetailDefaultRows(t *testing.T)
func TestPostgresStoreEnsureRejectsMismatchedManifest(t *testing.T)
func TestPostgresStoreEnsureConcurrentCallsConverge(t *testing.T)
func TestPostgresStoreEnsureRollsBackPartialPair(t *testing.T)
```

For the default-row tests, insert an event through the parent with an
`occurred_at` inside an unpartitioned 2027 week. For the aligned event/detail
case, insert its matching detail as well. Assert:

```go
var occupied *DefaultPartitionOccupiedError
if !errors.As(err, &occupied) {
	t.Fatalf("error = %v, want DefaultPartitionOccupiedError", err)
}
if occupied.EventRows != 1 {
	t.Fatalf("event default rows = %d, want 1", occupied.EventRows)
}
```

- [ ] **Step 2: Run integration tests and verify RED**

Run against an isolated local PostgreSQL fixture:

```bash
cd api
REQUEST_EVENTS_TEST_DATABASE_URL="$REQUEST_EVENTS_TEST_DATABASE_URL" \
  go test -tags=integration ./internal/requesteventpartitions \
  -run 'TestPostgresStoreEnsure' -count=1 -v
```

Expected: FAIL because `PostgresStore` and `Ensure` do not exist.

- [ ] **Step 3: Implement Ensure**

Define:

```go
type Beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

type PostgresStore struct {
	db Beginner
}

func NewPostgresStore(db Beginner) *PostgresStore {
	return &PostgresStore{db: db}
}

type DefaultPartitionOccupiedError struct {
	Week       Week
	EventRows  int64
	DetailRows int64
}

func (e *DefaultPartitionOccupiedError) Error() string {
	return fmt.Sprintf(
		"default partitions contain rows for [%s, %s): events=%d details=%d",
		e.Week.Start.Format(time.RFC3339),
		e.Week.End.Format(time.RFC3339),
		e.EventRows,
		e.DetailRows,
	)
}
```

`Ensure` must:

```go
func (s *PostgresStore) Ensure(ctx context.Context, weeks []Week) error {
	if s == nil || s.db == nil {
		return errors.New("partition store is not configured")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin partition ensure: %w", err)
	}
	defer tx.Rollback(context.Background())

	if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout = '5s'`); err != nil {
		return fmt.Errorf("set partition lock timeout: %w", err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL statement_timeout = '30s'`); err != nil {
		return fmt.Errorf("set partition statement timeout: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock($1::INTEGER, $2::INTEGER)`,
		PartitionLockNamespace, PartitionLockKey,
	); err != nil {
		return fmt.Errorf("lock request-event partition maintenance: %w", err)
	}

	for _, week := range weeks {
		if err := ensureWeek(ctx, tx, week); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit partition ensure: %w", err)
	}
	return nil
}
```

`ensureWeek` first reads the manifest row. Exact matching rows are idempotent;
mismatches fail. Missing rows trigger exact default counts, then DDL with
`pgx.Identifier{week.EventTable}.Sanitize()` and UTC RFC3339 bounds. Create the
event child, create the detail child, and insert the manifest row in that order.

- [ ] **Step 4: Run Ensure integration tests and verify GREEN**

Run the command from Step 2. Expected: all Ensure tests PASS.

- [ ] **Step 5: Commit Ensure**

```bash
git add api/internal/requesteventpartitions/postgres.go \
  api/internal/requesteventpartitions/postgres_integration_test.go
git commit -m "feat: ensure aligned request event partitions"
```

## Task 4: Add persistent inspection and readiness

**Files:**
- Modify: `api/internal/requesteventpartitions/postgres.go`
- Modify: `api/internal/requesteventpartitions/postgres_integration_test.go`

- [ ] **Step 1: Write failing inspection tests**

Add:

```go
func TestPostgresStoreInspectReportsReadyCoverageAndSizes(t *testing.T)
func TestPostgresStoreInspectFailsReadinessForDefaultRows(t *testing.T)
func TestPostgresStoreInspectFailsReadinessForLowHorizon(t *testing.T)
func TestPostgresStoreInspectFailsReadinessForMetadataMismatch(t *testing.T)
```

Assert a response shaped as:

```go
type Inspection struct {
	InspectedAt         time.Time `json:"inspected_at"`
	LatestExplicitEnd   time.Time `json:"latest_explicit_end"`
	CoverageDays        int       `json:"coverage_days"`
	PartitionPairs      int64     `json:"partition_pairs"`
	EventDefaultRows    int64     `json:"event_default_rows"`
	DetailDefaultRows   int64     `json:"detail_default_rows"`
	EventEstimatedRows  int64     `json:"event_estimated_rows"`
	DetailEstimatedRows int64     `json:"detail_estimated_rows"`
	EventTotalBytes     int64     `json:"event_total_bytes"`
	DetailTotalBytes    int64     `json:"detail_total_bytes"`
	Ready               bool      `json:"ready"`
	Reasons             []string  `json:"reasons"`
}
```

- [ ] **Step 2: Run inspection tests and verify RED**

Run:

```bash
cd api
REQUEST_EVENTS_TEST_DATABASE_URL="$REQUEST_EVENTS_TEST_DATABASE_URL" \
  go test -tags=integration ./internal/requesteventpartitions \
  -run 'TestPostgresStoreInspect' -count=1 -v
```

Expected: FAIL because `Inspect` does not exist.

- [ ] **Step 3: Implement Inspect**

Implement:

```go
func (s *PostgresStore) Inspect(
	ctx context.Context,
	now time.Time,
	minimumCoverageDays int,
) (Inspection, error)
```

Use one read-only transaction and query:

- manifest count and maximum `week_end`;
- exact counts from both default children;
- partition-tree relation OIDs through `pg_partition_tree`;
- estimated rows from `pg_stat_user_tables.n_live_tup`;
- total bytes through `pg_total_relation_size(relid)`;
- manifest rows joined to `pg_class` and `pg_inherits` to prove both named
  children remain attached to the expected parents.

Sort and deduplicate readiness reasons. Use these stable reason codes:

```go
const (
	ReasonCoverageLow          = "explicit_coverage_below_14_days"
	ReasonEventDefaultRows     = "event_default_partition_occupied"
	ReasonDetailDefaultRows    = "detail_default_partition_occupied"
	ReasonPartitionMismatch    = "partition_manifest_mismatch"
)
```

- [ ] **Step 4: Run inspection tests and verify GREEN**

Run the command from Step 2. Expected: PASS.

- [ ] **Step 5: Commit inspection**

```bash
git add api/internal/requesteventpartitions/postgres.go \
  api/internal/requesteventpartitions/postgres_integration_test.go
git commit -m "feat: inspect request event partition readiness"
```

## Task 5: Add the six-hour runtime worker

**Files:**
- Create: `api/internal/requesteventpartitions/worker_test.go`
- Create: `api/internal/requesteventpartitions/worker.go`

- [ ] **Step 1: Write worker tests**

Use a fake clock and fake maintainer to cover:

```go
func TestWorkerRunsImmediatelyAndEverySixHours(t *testing.T)
func TestWorkerBoundsEachAttemptAtThirtySeconds(t *testing.T)
func TestWorkerRetainsLastSuccessAfterFailure(t *testing.T)
func TestWorkerStopsOnContextCancellation(t *testing.T)
func TestWorkerClassifiesDefaultOccupancy(t *testing.T)
```

Define the interface in tests:

```go
type Maintainer interface {
	Ensure(context.Context, []Week) error
	Inspect(context.Context, time.Time, int) (Inspection, error)
}
```

Assert `Stats` contains:

```go
type WorkerStats struct {
	LastAttemptAt  time.Time  `json:"last_attempt_at"`
	LastSuccessAt  time.Time  `json:"last_success_at"`
	FailureCount   uint64     `json:"failure_count"`
	LastErrorClass string     `json:"last_error_class,omitempty"`
	LastInspection Inspection `json:"last_inspection"`
}
```

- [ ] **Step 2: Run worker tests and verify RED**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/requesteventpartitions \
  -run 'TestWorker' -count=1 -v
```

Expected: FAIL because `Worker` does not exist.

- [ ] **Step 3: Implement the worker**

Use constants:

```go
const (
	workerCadence = 6 * time.Hour
	attemptTimeout = 30 * time.Second
)
```

`runOnce` plans the current plus eight future weeks, calls `Ensure`, calls
`Inspect`, stores stats under a mutex, and emits exactly one of:

```text
request_event_partition_maintenance_failed
request_event_partition_default_occupied
request_event_partition_horizon_low
request_event_partition_maintenance_succeeded
```

Do not return maintenance errors to HTTP request paths.

- [ ] **Step 4: Run worker tests and verify GREEN**

Run the command from Step 2. Expected: PASS.

- [ ] **Step 5: Commit the worker**

```bash
git add api/internal/requesteventpartitions/worker.go \
  api/internal/requesteventpartitions/worker_test.go
git commit -m "feat: maintain request event partitions in runtime"
```

## Task 6: Expose aggregate Super Admin partition status

**Files:**
- Create: `api/internal/requesteventpartitions/handler_test.go`
- Create: `api/internal/requesteventpartitions/handler.go`
- Modify: `api/cmd/api/main.go`
- Create: `api/cmd/api/request_event_partition_wiring_test.go`

- [ ] **Step 1: Write handler and wiring tests**

Test:

```go
func TestStatusHandlerReturnsInspectionAndWorkerStats(t *testing.T)
func TestStatusHandlerReturnsServiceUnavailableWhenInspectionFails(t *testing.T)
```

Expected JSON:

```json
{
  "data": {
    "inspection": {
      "ready": true,
      "coverage_days": 60,
      "event_default_rows": 0,
      "detail_default_rows": 0
    },
    "worker": {
      "failure_count": 0
    }
  }
}
```

The wiring contract reads `main.go` and requires:

```go
requesteventpartitions.NewPostgresStore(pool)
requesteventpartitions.NewWorker(partitionStore)
go requestEventPartitionWorker.Start(workerCtx)
Get("/v1/admin/observability/request-event-partitions"
auth.RequireSuperAdmin
```

- [ ] **Step 2: Run handler/wiring tests and verify RED**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/requesteventpartitions ./cmd/api \
  -run 'TestStatusHandler|TestRequestEventPartitionWiring' -count=1 -v
```

Expected: FAIL because the handler and wiring do not exist.

- [ ] **Step 3: Implement handler and main wiring**

Create:

```go
type Inspector interface {
	Inspect(context.Context, time.Time, int) (Inspection, error)
}

func StatusHandler(inspector Inspector, worker *Worker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		inspection, err := inspector.Inspect(ctx, time.Now(), MinimumCoverageDays)
		if err != nil {
			http.Error(w, "request-event partition inspection is unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"inspection": inspection,
				"worker": worker.Stats(),
			},
		})
	}
}
```

In `main.go`, create the store/worker after `pool` exists, start the worker only
for `processModeAPI`, and add the route beside the existing request-event stats
route under `RequireSuperAdmin`.

- [ ] **Step 4: Run handler/wiring tests and verify GREEN**

Run the command from Step 2. Expected: PASS.

- [ ] **Step 5: Commit the status surface**

```bash
git add api/internal/requesteventpartitions/handler.go \
  api/internal/requesteventpartitions/handler_test.go \
  api/cmd/api/main.go \
  api/cmd/api/request_event_partition_wiring_test.go
git commit -m "feat: expose request event partition health"
```

## Task 7: Enforce partition readiness in Railway pre-deploy

**Files:**
- Modify: `api/cmd/api/migration_command_test.go`
- Modify: `api/cmd/api/migration_command.go`
- Create: `api/internal/requesteventpartitions/predeploy.go`
- Create: `api/internal/requesteventpartitions/predeploy_test.go`

- [ ] **Step 1: Write failing pre-deploy tests**

Extend `handleMigrationCommand` with:

```go
type partitionReadinessRunner func(context.Context, string) error
```

Add tests proving:

```go
func TestHandleMigrationCommandRunsPartitionReadinessAfterMigrations(t *testing.T)
func TestHandleMigrationCommandSkipsPartitionReadinessWhenMigrationFails(t *testing.T)
func TestHandleMigrationCommandFailsWhenPartitionReadinessFails(t *testing.T)
func TestHandleMigrationCommandBoundsPartitionReadinessAtFortyFiveSeconds(t *testing.T)
```

Use an ordered slice to assert `[]string{"migrations", "partitions"}`. The
timeout test blocks until `<-ctx.Done()` and requires `context.DeadlineExceeded`.

- [ ] **Step 2: Run migration-command tests and verify RED**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./cmd/api \
  -run 'TestHandleMigrationCommand.*Partition' -count=1 -v
```

Expected: FAIL because `handleMigrationCommand` has no partition runner.

- [ ] **Step 3: Implement pre-deploy maintenance**

Change `handleMigrationCommand` to accept the readiness runner after the
migration runner:

```go
if err := runMigrations(ctx, databaseURL, config, backupClient); err != nil {
	return true, err
}
partitionCtx, partitionCancel := context.WithTimeout(ctx, 45*time.Second)
defer partitionCancel()
if err := ensurePartitions(partitionCtx, databaseURL); err != nil {
	return true, fmt.Errorf("ensure request-event partition readiness: %w", err)
}
return true, nil
```

Create:

```go
func EnsureDatabaseReady(ctx context.Context, databaseURL string) error {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("parse partition database URL: %w", err)
	}
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("open partition database: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping partition database: %w", err)
	}
	store := NewPostgresStore(pool)
	weeks, err := PlanWeeks(time.Now(), MaintenanceWeeksAhead)
	if err != nil {
		return err
	}
	if err := store.Ensure(ctx, weeks); err != nil {
		return err
	}
	inspection, err := store.Inspect(ctx, time.Now(), MinimumCoverageDays)
	if err != nil {
		return err
	}
	if !inspection.Ready {
		return fmt.Errorf("request-event partitions are not ready: %s", strings.Join(inspection.Reasons, ","))
	}
	return nil
}
```

Pass `requesteventpartitions.EnsureDatabaseReady` from `main.go`.

- [ ] **Step 4: Run pre-deploy tests and verify GREEN**

Run the command from Step 2. Expected: PASS.

- [ ] **Step 5: Commit the deployment gate**

```bash
git add api/cmd/api/migration_command.go \
  api/cmd/api/migration_command_test.go \
  api/cmd/api/main.go \
  api/internal/requesteventpartitions/predeploy.go \
  api/internal/requesteventpartitions/predeploy_test.go
git commit -m "feat: gate deploys on request partition readiness"
```

## Task 8: Add the missed-window recovery runbook

**Files:**
- Create: `docs/operations/request-event-default-partition-recovery.md`

- [ ] **Step 1: Write the runbook**

The document must contain:

```markdown
# Request Event Default Partition Recovery

## Authorization boundary

Use only when `request-event-partitions` reports default occupancy and automatic
Ensure has failed closed. Do not use this procedure for retention or routine
partition creation.

## Required evidence

- exact environment and database identity;
- exact application SHA;
- Railway backup ID created and locked for this attempt;
- affected UTC week;
- event/detail default counts;
- proposed aligned child names;
- declared maintenance window and operator approval.

## Dry run

Run count-only queries for both defaults and verify the proposed ISO-week
boundaries and identifiers. Abort on any row outside the one intended week.

## Transaction

Use `BEGIN`, local lock/statement timeouts, the two-key `RQPT` advisory lock,
bounded parent/default locks, transaction-local copies, detail-before-event
deletes, event-before-detail reinserts, manifest insertion, and count equality
assertions. Any failed assertion executes `ROLLBACK`.

## Acceptance

Both default counts for the week are zero, both child counts equal their source
counts, manifest metadata matches, partition health is ready, and recorder
drop/write-failure deltas are recorded.
```

Include a psql transaction template using variables `week_start`, `week_end`,
`event_partition`, and `detail_partition`. Validate identifiers against
`^[0-9]{4}w[0-9]{2}$` before dynamic SQL. The template must copy both default
row sets before deleting either one and must never use `CASCADE`.

- [ ] **Step 2: Run documentation safety checks**

Run:

```bash
rg -n "backup|ROLLBACK|default|detail.*before.*event|event.*before.*detail|RQPT|never use.*CASCADE" \
  docs/operations/request-event-default-partition-recovery.md
git diff --check
```

Expected: every safety phrase is present and `git diff --check` is silent.

- [ ] **Step 3: Commit the runbook**

```bash
git add docs/operations/request-event-default-partition-recovery.md
git commit -m "docs: add request partition recovery runbook"
```

## Task 9: Make PostgreSQL integration coverage non-skippable

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `api/internal/db/migration_gate_test.go`

- [ ] **Step 1: Write a failing CI source contract**

Extend `TestCIRequiresMigrationGatePostgresIntegration` to require:

```go
for _, required := range []string{
	"go test -tags=integration ./internal/requestevents -count=1",
	"go test -tags=integration ./internal/requesteventpartitions -count=1",
	"TestRequireCurrentSchemaRejects124AndAccepts133",
} {
	if !strings.Contains(workflow, required) {
		t.Fatalf("required PostgreSQL CI job does not run %s", required)
	}
}
```

- [ ] **Step 2: Run the CI contract and verify RED**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/db \
  -run TestCIRequiresMigrationGatePostgresIntegration -count=1 -v
```

Expected: FAIL because CI does not explicitly run either integration package.

- [ ] **Step 3: Update CI**

After the Goose migration test, run:

```yaml
GOOSE_MIGRATION_TEST_DATABASE_URL="$REQUEST_EVENTS_TEST_DATABASE_URL" \
  go test ./internal/db -run '^TestRunMigrationsAppliesAllEmbeddedMigrationsWithGoose$' -count=1
go test -tags=integration ./internal/requestevents -count=1
go test -tags=integration ./internal/requesteventpartitions -count=1
go test -tags=integration ./internal/observabilityreads -count=1
```

Rename every exact selector reference from
`TestRequireCurrentSchemaRejects124AndAccepts132` to
`TestRequireCurrentSchemaRejects124AndAccepts133`.

- [ ] **Step 4: Run the CI contract and verify GREEN**

Run the command from Step 2. Expected: PASS.

- [ ] **Step 5: Commit CI coverage**

```bash
git add .github/workflows/ci.yml api/internal/db/migration_gate_test.go
git commit -m "test: require request partition PostgreSQL coverage"
```

## Task 10: Full local verification and review preparation

**Files:**
- Modify only files required by failures directly caused by Tasks 1-9.

- [ ] **Step 1: Verify worktree ownership**

Run:

```bash
pwd
git branch --show-current
git status --short --branch
```

Expected path:
`/Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-youtube-icon-source-system`

Expected branch: `dev-youtube-icon-source-system`.

- [ ] **Step 2: Run formatting and focused unit tests**

Run:

```bash
cd api
gofmt -w \
  internal/requesteventpartitions/*.go \
  cmd/api/main.go \
  cmd/api/migration_command.go \
  cmd/api/migration_command_test.go \
  cmd/api/request_event_partition_wiring_test.go
GOCACHE=/tmp/unipost-go-build go test ./internal/requesteventpartitions ./internal/db ./cmd/api -count=1
```

Expected: PASS.

- [ ] **Step 3: Run PostgreSQL integration tests**

Run:

```bash
cd api
REQUEST_EVENTS_TEST_DATABASE_URL="$REQUEST_EVENTS_TEST_DATABASE_URL" \
  go test -tags=integration ./internal/requestevents ./internal/requesteventpartitions ./internal/observabilityreads -count=1
GOOSE_MIGRATION_TEST_DATABASE_URL="$REQUEST_EVENTS_TEST_DATABASE_URL" \
  go test ./internal/db -run '^TestRunMigrationsAppliesAllEmbeddedMigrationsWithGoose$' -count=1
```

Expected: PASS with no skipped package.

- [ ] **Step 4: Run the complete API suite**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
```

Expected: PASS.

- [ ] **Step 5: Run repository diff checks**

Run:

```bash
git diff --check
git status --short
git diff --stat origin/dev...HEAD
git log --oneline origin/dev..HEAD
```

Expected: no whitespace errors, only partition-safety scope files, and focused
commits.

- [ ] **Step 6: Request code review before publishing**

Review specifically:

- migration 133 guarded Down and date bounds;
- default occupancy fail-closed behavior;
- DDL identifier safety;
- advisory-lock namespace and timeout behavior;
- pre-deploy order after Goose;
- no request/publishing/business-path coupling;
- recovery runbook destructive-operation safety.

- [ ] **Step 7: Push the task branch and open a Draft PR to dev**

After review findings are resolved and all local verification passes:

```bash
git push origin dev-youtube-icon-source-system
gh pr create \
  --base dev \
  --head dev-youtube-icon-source-system \
  --draft \
  --title "feat: automate request event partition safety" \
  --body-file /tmp/request-event-partition-pr-body.md
```

The PR body must list the exact commits/files, the August 10 deadline, migration
133 bounds, readiness behavior, tests, recovery boundary, and explicit
non-goals. Do not merge until exact-head Preview Acceptance, Railway PR
environment, Vercel Preview, deployed regression, and browser/API acceptance
all pass.
