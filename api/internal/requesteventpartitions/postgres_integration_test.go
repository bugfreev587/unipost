//go:build integration

package requesteventpartitions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/testdbguard"
)

const requestEventPartitionsIntegrationDatabaseEnv = "REQUEST_EVENTS_TEST_DATABASE_URL"

func openPartitionIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv(requestEventPartitionsIntegrationDatabaseEnv))
	if databaseURL == "" {
		t.Fatalf("%s is required", requestEventPartitionsIntegrationDatabaseEnv)
	}
	ctx := context.Background()
	admin, config, err := testdbguard.OpenValidated(
		requestEventPartitionsIntegrationDatabaseEnv,
		databaseURL,
		func(validatedURL string) (*pgxpool.Pool, error) {
			return pgxpool.New(ctx, validatedURL)
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("request_event_partitions_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		admin.Close()
		t.Fatalf("create isolated schema: %v", err)
	}

	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Errorf("drop isolated schema: %v", err)
		}
		admin.Close()
	})
	seedPartitionSchema(t, pool)
	return pool
}

func seedPartitionSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		CREATE TABLE api_request_events (
			occurred_at TIMESTAMPTZ NOT NULL,
			id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			api_key_id TEXT NOT NULL,
			method TEXT NOT NULL,
			route_pattern TEXT NOT NULL,
			status_code INTEGER NOT NULL,
			duration_ms INTEGER NOT NULL,
			outcome TEXT NOT NULL,
			has_error_detail BOOLEAN NOT NULL DEFAULT FALSE,
			PRIMARY KEY (occurred_at, id, workspace_id)
		) PARTITION BY RANGE (occurred_at);
		CREATE TABLE api_request_events_default
			PARTITION OF api_request_events DEFAULT;

		CREATE TABLE api_request_error_details (
			occurred_at TIMESTAMPTZ NOT NULL,
			event_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			redaction_policy_version INTEGER NOT NULL,
			PRIMARY KEY (occurred_at, event_id, workspace_id),
			FOREIGN KEY (occurred_at, event_id, workspace_id)
				REFERENCES api_request_events (occurred_at, id, workspace_id)
				ON DELETE CASCADE
		) PARTITION BY RANGE (occurred_at);
		CREATE TABLE api_request_error_details_default
			PARTITION OF api_request_error_details DEFAULT;

		CREATE TABLE api_request_partition_manifest (
			week_start TIMESTAMPTZ PRIMARY KEY,
			week_end TIMESTAMPTZ NOT NULL UNIQUE,
			event_partition TEXT NOT NULL UNIQUE,
			detail_partition TEXT NOT NULL UNIQUE,
			state TEXT NOT NULL DEFAULT 'active',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CHECK (week_end = week_start + INTERVAL '7 days')
		);
	`)
	if err != nil {
		t.Fatalf("seed partition schema: %v", err)
	}
}

func futureWeek(t *testing.T, day int) Week {
	t.Helper()
	week, err := weekForStart(time.Date(2027, 2, day, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return week
}

func insertDefaultEvent(t *testing.T, pool *pgxpool.Pool, week Week, withDetail bool) {
	t.Helper()
	ctx := context.Background()
	occurredAt := week.Start.Add(time.Hour)
	if _, err := pool.Exec(ctx, `
		INSERT INTO api_request_events (
			occurred_at, id, workspace_id, api_key_id, method, route_pattern,
			status_code, duration_ms, outcome, has_error_detail
		) VALUES ($1, 'default-event', 'workspace', 'api-key', 'GET', '/v1/test', 500, 1, 'server_error', $2)
	`, occurredAt, withDetail); err != nil {
		t.Fatalf("insert default event: %v", err)
	}
	if withDetail {
		if _, err := pool.Exec(ctx, `
			INSERT INTO api_request_error_details (
				occurred_at, event_id, workspace_id, redaction_policy_version
			) VALUES ($1, 'default-event', 'workspace', 1)
		`, occurredAt); err != nil {
			t.Fatalf("insert default detail: %v", err)
		}
	}
}

func assertPartitionAttached(t *testing.T, pool *pgxpool.Pool, child, parent string) {
	t.Helper()
	var attached bool
	if err := pool.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1
			FROM pg_inherits AS inheritance
			JOIN pg_class AS child ON child.oid = inheritance.inhrelid
			JOIN pg_class AS parent ON parent.oid = inheritance.inhparent
			JOIN pg_namespace AS namespace ON namespace.oid = child.relnamespace
			WHERE namespace.nspname = current_schema()
			  AND child.relname = $1
			  AND parent.relname = $2
		)
	`, child, parent).Scan(&attached); err != nil {
		t.Fatal(err)
	}
	if !attached {
		t.Fatalf("partition %s is not attached to %s", child, parent)
	}
}

func TestPostgresStoreEnsureCreatesAlignedPairsAndIsIdempotent(t *testing.T) {
	pool := openPartitionIntegrationPool(t)
	store := NewPostgresStore(pool)
	week := futureWeek(t, 1)

	if err := store.Ensure(context.Background(), []Week{week}); err != nil {
		t.Fatal(err)
	}
	if err := store.Ensure(context.Background(), []Week{week}); err != nil {
		t.Fatalf("idempotent ensure: %v", err)
	}
	assertPartitionAttached(t, pool, week.EventTable, "api_request_events")
	assertPartitionAttached(t, pool, week.DetailTable, "api_request_error_details")

	var eventTable, detailTable string
	if err := pool.QueryRow(context.Background(), `
		SELECT event_partition, detail_partition
		FROM api_request_partition_manifest
		WHERE week_start = $1 AND week_end = $2
	`, week.Start, week.End).Scan(&eventTable, &detailTable); err != nil {
		t.Fatal(err)
	}
	if eventTable != week.EventTable || detailTable != week.DetailTable {
		t.Fatalf("manifest pair = (%q, %q), want (%q, %q)", eventTable, detailTable, week.EventTable, week.DetailTable)
	}
}

func TestPostgresStoreEnsureRejectsEventDefaultRows(t *testing.T) {
	pool := openPartitionIntegrationPool(t)
	week := futureWeek(t, 8)
	insertDefaultEvent(t, pool, week, false)

	err := NewPostgresStore(pool).Ensure(context.Background(), []Week{week})
	var occupied *DefaultPartitionOccupiedError
	if !errors.As(err, &occupied) {
		t.Fatalf("error = %v, want DefaultPartitionOccupiedError", err)
	}
	if occupied.EventRows != 1 || occupied.DetailRows != 0 {
		t.Fatalf("default rows = events:%d details:%d, want 1 and 0", occupied.EventRows, occupied.DetailRows)
	}
}

func TestPostgresStoreEnsureRejectsAlignedEventAndDetailDefaultRows(t *testing.T) {
	pool := openPartitionIntegrationPool(t)
	week := futureWeek(t, 15)
	insertDefaultEvent(t, pool, week, true)

	err := NewPostgresStore(pool).Ensure(context.Background(), []Week{week})
	var occupied *DefaultPartitionOccupiedError
	if !errors.As(err, &occupied) {
		t.Fatalf("error = %v, want DefaultPartitionOccupiedError", err)
	}
	if occupied.EventRows != 1 || occupied.DetailRows != 1 {
		t.Fatalf("default rows = events:%d details:%d, want 1 and 1", occupied.EventRows, occupied.DetailRows)
	}
}

func TestPostgresStoreEnsureRejectsMismatchedManifest(t *testing.T) {
	pool := openPartitionIntegrationPool(t)
	week := futureWeek(t, 22)
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO api_request_partition_manifest (
			week_start, week_end, event_partition, detail_partition
		) VALUES ($1, $2, 'unexpected_event_partition', 'unexpected_detail_partition')
	`, week.Start, week.End); err != nil {
		t.Fatal(err)
	}

	err := NewPostgresStore(pool).Ensure(context.Background(), []Week{week})
	if err == nil || !strings.Contains(err.Error(), "manifest") {
		t.Fatalf("error = %v, want manifest mismatch", err)
	}
}

func TestPostgresStoreEnsureConcurrentCallsConverge(t *testing.T) {
	pool := openPartitionIntegrationPool(t)
	store := NewPostgresStore(pool)
	week := futureWeek(t, 1)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			errs <- store.Ensure(context.Background(), []Week{week})
		}()
	}
	close(start)
	workers.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent ensure: %v", err)
		}
	}
	var rows int
	if err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM api_request_partition_manifest WHERE week_start = $1
	`, week.Start).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("manifest rows = %d, want 1", rows)
	}
}

func TestPostgresStoreEnsureRollsBackPartialPair(t *testing.T) {
	pool := openPartitionIntegrationPool(t)
	week := futureWeek(t, 8)
	if _, err := pool.Exec(context.Background(), "CREATE TABLE "+pgx.Identifier{week.DetailTable}.Sanitize()+" (id INTEGER)"); err != nil {
		t.Fatal(err)
	}

	err := NewPostgresStore(pool).Ensure(context.Background(), []Week{week})
	if err == nil {
		t.Fatal("ensure must fail when the detail relation name is already occupied")
	}
	var eventRelation *string
	if err := pool.QueryRow(context.Background(), `SELECT to_regclass(current_schema() || '.' || $1)::TEXT`, week.EventTable).Scan(&eventRelation); err != nil {
		t.Fatal(err)
	}
	if eventRelation != nil {
		t.Fatalf("event child survived rolled-back pair creation: %q", *eventRelation)
	}
}

func containsReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}

func TestPostgresStoreInspectReportsReadyCoverageAndSizes(t *testing.T) {
	pool := openPartitionIntegrationPool(t)
	store := NewPostgresStore(pool)
	now := time.Date(2027, 2, 1, 12, 0, 0, 0, time.UTC)
	weeks, err := PlanWeeks(now, MaintenanceWeeksAhead)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Ensure(context.Background(), weeks); err != nil {
		t.Fatal(err)
	}

	inspection, err := store.Inspect(context.Background(), now, MinimumCoverageDays)
	if err != nil {
		t.Fatal(err)
	}
	if !inspection.Ready || len(inspection.Reasons) != 0 {
		t.Fatalf("inspection readiness = %v reasons=%v, want ready", inspection.Ready, inspection.Reasons)
	}
	if inspection.PartitionPairs != int64(len(weeks)) {
		t.Fatalf("partition pairs = %d, want %d", inspection.PartitionPairs, len(weeks))
	}
	if !inspection.LatestExplicitEnd.Equal(weeks[len(weeks)-1].End) {
		t.Fatalf("latest explicit end = %v, want %v", inspection.LatestExplicitEnd, weeks[len(weeks)-1].End)
	}
	if inspection.CoverageDays < MinimumCoverageDays {
		t.Fatalf("coverage days = %d, want at least %d", inspection.CoverageDays, MinimumCoverageDays)
	}
	if inspection.EventTotalBytes <= 0 || inspection.DetailTotalBytes <= 0 {
		t.Fatalf("partition bytes = events:%d details:%d, want positive", inspection.EventTotalBytes, inspection.DetailTotalBytes)
	}
}

func TestPostgresStoreInspectFailsReadinessForDefaultRows(t *testing.T) {
	pool := openPartitionIntegrationPool(t)
	store := NewPostgresStore(pool)
	week := futureWeek(t, 1)
	insertDefaultEvent(t, pool, week, true)

	inspection, err := store.Inspect(context.Background(), week.Start, MinimumCoverageDays)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Ready {
		t.Fatal("inspection with default rows must not be ready")
	}
	if inspection.EventDefaultRows != 1 || inspection.DetailDefaultRows != 1 {
		t.Fatalf("default rows = events:%d details:%d, want 1 and 1", inspection.EventDefaultRows, inspection.DetailDefaultRows)
	}
	if !containsReason(inspection.Reasons, ReasonEventDefaultRows) ||
		!containsReason(inspection.Reasons, ReasonDetailDefaultRows) {
		t.Fatalf("reasons = %v, want both default occupancy reasons", inspection.Reasons)
	}
}

func TestPostgresStoreInspectFailsReadinessForLowHorizon(t *testing.T) {
	pool := openPartitionIntegrationPool(t)
	store := NewPostgresStore(pool)
	now := time.Date(2027, 2, 1, 12, 0, 0, 0, time.UTC)
	weeks, err := PlanWeeks(now, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Ensure(context.Background(), weeks); err != nil {
		t.Fatal(err)
	}

	inspection, err := store.Inspect(context.Background(), now, MinimumCoverageDays)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Ready || !containsReason(inspection.Reasons, ReasonCoverageLow) {
		t.Fatalf("ready=%v reasons=%v, want low coverage", inspection.Ready, inspection.Reasons)
	}
}

func TestPostgresStoreInspectFailsReadinessForMetadataMismatch(t *testing.T) {
	pool := openPartitionIntegrationPool(t)
	store := NewPostgresStore(pool)
	week := futureWeek(t, 1)
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO api_request_partition_manifest (
			week_start, week_end, event_partition, detail_partition
		) VALUES ($1, $2, $3, $4)
	`, week.Start, week.End, week.EventTable, week.DetailTable); err != nil {
		t.Fatal(err)
	}

	inspection, err := store.Inspect(context.Background(), week.Start, MinimumCoverageDays)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Ready || !containsReason(inspection.Reasons, ReasonPartitionMismatch) {
		t.Fatalf("ready=%v reasons=%v, want metadata mismatch", inspection.Ready, inspection.Reasons)
	}
}
