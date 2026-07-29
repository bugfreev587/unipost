//go:build integration

package observabilityreads

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type rollupRow struct {
	WorkspaceID          string
	RoutePattern         string
	Method               string
	StatusCode           int
	RequestCount         int64
	ErrorCount           int64
	ServerErrorCount     int64
	ValidationErrorCount int64
	RateLimitCount       int64
	DurationSumMS        int64
	DurationMinMS        int
	DurationMaxMS        int
	Histogram            []int64
}

func TestPostgresRollupRecomputeHourIsIdempotentLateAwareAndDimensionSafe(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	store := NewPostgresRollupStore(pool)
	ctx := context.Background()
	hour := time.Date(2026, time.July, 29, 8, 0, 0, 0, time.UTC)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	workspaceA := "rollup-a-" + suffix
	workspaceB := "rollup-b-" + suffix
	eventIDs := make([]string, 0, 20)

	cleanup := func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_events WHERE workspace_id = ANY($1::TEXT[])`, []string{workspaceA, workspaceB})
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_metric_rollups_hourly WHERE workspace_id = ANY($1::TEXT[])`, []string{workspaceA, workspaceB})
	}
	cleanup()
	t.Cleanup(cleanup)

	insert := func(workspace, apiKey, route, method string, status, duration int, outcome string) string {
		t.Helper()
		id := fmt.Sprintf("rollup-%02d-%s", len(eventIDs), suffix)
		eventIDs = append(eventIDs, id)
		if _, err := pool.Exec(ctx, `
			INSERT INTO api_request_events (
				occurred_at, id, workspace_id, api_key_id, method, route_pattern,
				status_code, duration_ms, outcome
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, hour.Add(time.Duration(len(eventIDs))*time.Second), id, workspace, apiKey, method, route, status, duration, outcome); err != nil {
			t.Fatal(err)
		}
		return id
	}

	for index, duration := range []int{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 10001} {
		apiKey := "rollup-key-a"
		if index%2 == 1 {
			apiKey = "rollup-key-b"
		}
		insert(workspaceA, apiKey, "/v1/a", "GET", 200, duration, "success")
	}
	insert(workspaceA, "rollup-key-a", "/v1/a", "POST", 400, 6, "validation_error")
	insert(workspaceA, "rollup-key-a", "/v1/a", "POST", 429, 200, "rate_limited")
	insert(workspaceA, "rollup-key-a", "/v1/a", "POST", 500, 6000, "server_error")
	workspaceBEvent := insert(workspaceB, "rollup-key-b", "/v1/a", "GET", 200, 11, "success")
	insert(workspaceA, "rollup-key-a", "/v1/b", "GET", 200, 10001, "success")

	if err := store.RecomputeHour(ctx, hour); err != nil {
		t.Fatal(err)
	}
	assertRollupRows(t, readRollupRows(t, pool, hour, workspaceA, workspaceB), []rollupRow{
		{WorkspaceID: workspaceA, RoutePattern: "/v1/a", Method: "GET", StatusCode: 200, RequestCount: 12, DurationSumMS: 29441, DurationMinMS: 5, DurationMaxMS: 10001, Histogram: []int64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}},
		{WorkspaceID: workspaceA, RoutePattern: "/v1/a", Method: "POST", StatusCode: 400, RequestCount: 1, ErrorCount: 1, ValidationErrorCount: 1, DurationSumMS: 6, DurationMinMS: 6, DurationMaxMS: 6, Histogram: []int64{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{WorkspaceID: workspaceA, RoutePattern: "/v1/a", Method: "POST", StatusCode: 429, RequestCount: 1, ErrorCount: 1, RateLimitCount: 1, DurationSumMS: 200, DurationMinMS: 200, DurationMaxMS: 200, Histogram: []int64{0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0}},
		{WorkspaceID: workspaceA, RoutePattern: "/v1/a", Method: "POST", StatusCode: 500, RequestCount: 1, ErrorCount: 1, ServerErrorCount: 1, DurationSumMS: 6000, DurationMinMS: 6000, DurationMaxMS: 6000, Histogram: []int64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0}},
		{WorkspaceID: workspaceA, RoutePattern: "/v1/b", Method: "GET", StatusCode: 200, RequestCount: 1, DurationSumMS: 10001, DurationMinMS: 10001, DurationMaxMS: 10001, Histogram: []int64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}},
		{WorkspaceID: workspaceB, RoutePattern: "/v1/a", Method: "GET", StatusCode: 200, RequestCount: 1, DurationSumMS: 11, DurationMinMS: 11, DurationMaxMS: 11, Histogram: []int64{0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
	})

	if err := store.RecomputeHour(ctx, hour); err != nil {
		t.Fatal(err)
	}
	rowsAfterSecondRun := readRollupRows(t, pool, hour, workspaceA, workspaceB)
	if rowsAfterSecondRun[0].RequestCount != 12 {
		t.Fatalf("second recomputation doubled request count: %#v", rowsAfterSecondRun[0])
	}

	insert(workspaceA, "rollup-key-late", "/v1/a", "GET", 200, 4, "success")
	if _, err := pool.Exec(ctx, `DELETE FROM api_request_events WHERE id = $1`, workspaceBEvent); err != nil {
		t.Fatal(err)
	}
	if err := store.RecomputeHour(ctx, hour); err != nil {
		t.Fatal(err)
	}
	finalRows := readRollupRows(t, pool, hour, workspaceA, workspaceB)
	for _, row := range finalRows {
		if row.WorkspaceID == workspaceB {
			t.Fatalf("stale removed dimension group remains after replacement recomputation: %#v", row)
		}
	}
	if finalRows[0].RequestCount != 13 || finalRows[0].DurationSumMS != 29445 || !reflect.DeepEqual(finalRows[0].Histogram, []int64{2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}) {
		t.Fatalf("late event not incorporated exactly: %#v", finalRows[0])
	}
}

func TestPostgresRollupConcurrentRecomputationsSerializePerHour(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	stores := []*PostgresRollupStore{NewPostgresRollupStore(pool), NewPostgresRollupStore(pool)}
	ctx := context.Background()
	hour := time.Date(2026, time.July, 29, 4, 0, 0, 0, time.UTC)
	workspace := fmt.Sprintf("rollup-concurrent-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_events WHERE workspace_id = $1`, workspace)
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_metric_rollups_hourly WHERE workspace_id = $1`, workspace)
	})
	for index, duration := range []int{5, 10} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO api_request_events (
				occurred_at, id, workspace_id, api_key_id, method, route_pattern,
				status_code, duration_ms, outcome
			) VALUES ($1, $2, $3, $4, 'GET', '/v1/concurrent', 200, $5, 'success')
		`, hour.Add(time.Duration(index)*time.Minute), fmt.Sprintf("rollup-concurrent-%d-%s", index, workspace), workspace, fmt.Sprintf("key-%d", index), duration); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO api_request_metric_rollups_hourly (
			hour_bucket, workspace_id, route_pattern, method, status_code,
			request_count, error_count, server_error_count,
			validation_error_count, rate_limit_count,
			duration_sum_ms, duration_min_ms, duration_max_ms, duration_histogram
		) VALUES ($1, $2, '/v1/concurrent', 'GET', 200, 99, 0, 0, 0, 0, 99, 1, 1, $3)
	`, hour, workspace, []int64{99, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}

	blocker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(context.Background())
	if _, err := blocker.Exec(ctx, `
		SELECT 1 FROM api_request_metric_rollups_hourly
		WHERE hour_bucket = $1 AND workspace_id = $2
		FOR UPDATE
	`, hour, workspace); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, len(stores))
	for _, store := range stores {
		go func(store *PostgresRollupStore) {
			<-start
			results <- store.RecomputeHour(context.Background(), hour)
		}(store)
	}
	close(start)
	waitForBlockedRollupRecomputations(t, pool, len(stores))
	if err := blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for range stores {
		if err := <-results; err != nil {
			t.Fatalf("concurrent recomputation failed: %v", err)
		}
	}

	var count, sum, minimum, maximum int64
	var histogram []int64
	if err := pool.QueryRow(ctx, `
		SELECT request_count, duration_sum_ms, duration_min_ms, duration_max_ms, duration_histogram
		FROM api_request_metric_rollups_hourly
		WHERE hour_bucket = $1 AND workspace_id = $2
	`, hour, workspace).Scan(&count, &sum, &minimum, &maximum, &histogram); err != nil {
		t.Fatal(err)
	}
	if count != 2 || sum != 15 || minimum != 5 || maximum != 10 || !reflect.DeepEqual(histogram, []int64{1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}) {
		t.Fatalf("final concurrent rollup = count:%d sum:%d min:%d max:%d histogram:%v", count, sum, minimum, maximum, histogram)
	}
}

func waitForBlockedRollupRecomputations(t *testing.T, pool *pgxpool.Pool, want int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var count int
		err := pool.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM pg_stat_activity
			WHERE datname = current_database()
			  AND pid <> pg_backend_pid()
			  AND wait_event_type = 'Lock'
			  AND (
			    query ILIKE '%DELETE FROM api_request_metric_rollups_hourly%'
			    OR query ILIKE '%pg_advisory_xact_lock%'
			  )
		`).Scan(&count)
		if err == nil && count >= want {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %d blocked concurrent recomputations; last count=%d err=%v", want, count, err)
		case <-ticker.C:
		}
	}
}

func TestPostgresRollupNormalizesContainingHourAndPreservesAdjacentHours(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	store := NewPostgresRollupStore(pool)
	ctx := context.Background()
	hour := time.Date(2026, time.July, 29, 6, 0, 0, 0, time.UTC)
	workspace := fmt.Sprintf("rollup-boundary-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_events WHERE workspace_id = $1`, workspace)
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_metric_rollups_hourly WHERE workspace_id = $1`, workspace)
	})

	for index, at := range []time.Time{
		hour.Add(-time.Nanosecond),
		hour,
		hour.Add(time.Hour - time.Nanosecond),
		hour.Add(time.Hour),
	} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO api_request_events (
				occurred_at, id, workspace_id, api_key_id, method, route_pattern,
				status_code, duration_ms, outcome
			) VALUES ($1, $2, $3, $4, 'GET', '/v1/boundary', 200, $5, 'success')
		`, at, fmt.Sprintf("rollup-boundary-%d-%s", index, workspace), workspace, fmt.Sprintf("boundary-key-%d", index%2), []int{1, 5, 10, 25}[index]); err != nil {
			t.Fatal(err)
		}
	}

	for _, seed := range []struct {
		at    time.Time
		count int64
	}{
		{at: hour.Add(-time.Hour), count: 77},
		{at: hour, count: 99},
		{at: hour.Add(time.Hour), count: 88},
	} {
		histogram := []int64{seed.count, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		if _, err := pool.Exec(ctx, `
			INSERT INTO api_request_metric_rollups_hourly (
				hour_bucket, workspace_id, route_pattern, method, status_code,
				request_count, error_count, server_error_count,
				validation_error_count, rate_limit_count,
				duration_sum_ms, duration_min_ms, duration_max_ms, duration_histogram
			) VALUES ($1, $2, '/v1/boundary', 'GET', 200, $3, 0, 0, 0, 0, $3, 1, 1, $4)
		`, seed.at, workspace, seed.count, histogram); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.RecomputeHour(ctx, hour.Add(37*time.Minute)); err != nil {
		t.Fatalf("recompute containing hour: %v", err)
	}

	rows, err := pool.Query(ctx, `
		SELECT hour_bucket, request_count, duration_sum_ms, duration_min_ms,
		       duration_max_ms, duration_histogram
		FROM api_request_metric_rollups_hourly
		WHERE workspace_id = $1
		ORDER BY hour_bucket
	`, workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type boundaryRollup struct {
		hour      time.Time
		count     int64
		sum       int64
		minimum   int
		maximum   int
		histogram []int64
	}
	var got []boundaryRollup
	for rows.Next() {
		var row boundaryRollup
		if err := rows.Scan(&row.hour, &row.count, &row.sum, &row.minimum, &row.maximum, &row.histogram); err != nil {
			t.Fatal(err)
		}
		row.hour = row.hour.UTC()
		got = append(got, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := []boundaryRollup{
		{hour: hour.Add(-time.Hour), count: 77, sum: 77, minimum: 1, maximum: 1, histogram: []int64{77, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{hour: hour, count: 2, sum: 15, minimum: 5, maximum: 10, histogram: []int64{1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{hour: hour.Add(time.Hour), count: 88, sum: 88, minimum: 1, maximum: 1, histogram: []int64{88, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("adjacent/target rollups = %#v, want %#v", got, want)
	}
}

func TestPostgresRollupRejectsNormalizedOpenAndFutureHours(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	store := NewPostgresRollupStore(pool)
	ctx := context.Background()
	currentHour := time.Now().UTC().Truncate(time.Hour)
	workspace := fmt.Sprintf("rollup-open-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_events WHERE workspace_id = $1`, workspace)
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_metric_rollups_hourly WHERE workspace_id = $1`, workspace)
	})
	if _, err := pool.Exec(ctx, `
		INSERT INTO api_request_events (
			occurred_at, id, workspace_id, api_key_id, method, route_pattern,
			status_code, duration_ms, outcome
		) VALUES ($1, $2, $3, 'rollup-key', 'GET', '/v1/open', 200, 1, 'success')
	`, currentHour.Add(time.Minute), "rollup-open-event-"+workspace, workspace); err != nil {
		t.Fatal(err)
	}
	if err := store.RecomputeHour(ctx, currentHour.Add(37*time.Minute)); err == nil {
		t.Fatal("current/open hour recomputation error = nil")
	}
	if err := store.RecomputeHour(ctx, currentHour.Add(time.Hour+37*time.Minute)); err == nil {
		t.Fatal("future hour recomputation error = nil")
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM api_request_metric_rollups_hourly WHERE workspace_id = $1`, workspace).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("open hour rollup rows = %d, want 0", count)
	}
}

func readRollupRows(t *testing.T, pool *pgxpool.Pool, hour time.Time, workspaces ...string) []rollupRow {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT workspace_id, route_pattern, method, status_code,
		       request_count, error_count, server_error_count,
		       validation_error_count, rate_limit_count,
		       duration_sum_ms, duration_min_ms, duration_max_ms,
		       duration_histogram
		FROM api_request_metric_rollups_hourly
		WHERE hour_bucket = $1 AND workspace_id = ANY($2::TEXT[])
		ORDER BY workspace_id, route_pattern, method, status_code
	`, hour, workspaces)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var result []rollupRow
	for rows.Next() {
		var row rollupRow
		if err := rows.Scan(
			&row.WorkspaceID, &row.RoutePattern, &row.Method, &row.StatusCode,
			&row.RequestCount, &row.ErrorCount, &row.ServerErrorCount,
			&row.ValidationErrorCount, &row.RateLimitCount,
			&row.DurationSumMS, &row.DurationMinMS, &row.DurationMaxMS,
			&row.Histogram,
		); err != nil {
			t.Fatal(err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func assertRollupRows(t *testing.T, got, want []rollupRow) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rollup rows =\n%#v\nwant\n%#v", got, want)
	}
}
