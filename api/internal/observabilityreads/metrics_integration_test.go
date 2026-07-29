//go:build integration

package observabilityreads

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type countingMetricsDB struct {
	pool    *pgxpool.Pool
	queries int
}

type countingMetricsTx struct {
	pgx.Tx
	db *countingMetricsDB
}

func (tx *countingMetricsTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx.db.queries++
	return tx.Tx.Query(ctx, sql, args...)
}

func (db *countingMetricsDB) BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	tx, err := db.pool.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	return &countingMetricsTx{Tx: tx, db: db}, nil
}

func (db *countingMetricsDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	db.queries++
	return db.pool.Query(ctx, sql, args...)
}

func (db *countingMetricsDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return db.pool.QueryRow(ctx, sql, args...)
}

type snapshotRecordingMetricsDB struct {
	pool    *pgxpool.Pool
	begins  int
	options []pgx.TxOptions
}

type hookedMetricsDB struct {
	pool              *pgxpool.Pool
	beforeSecondQuery func()
}

type hookedMetricsTx struct {
	pgx.Tx
	queries           int
	beforeSecondQuery func()
	rawSQL            *string
	rollupSQL         *string
	rollupRows        *int
}

type countedMetricsRows struct {
	pgx.Rows
	count *int
}

func (rows *countedMetricsRows) Next() bool {
	ok := rows.Rows.Next()
	if ok {
		*rows.count++
	}
	return ok
}

func (tx *hookedMetricsTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx.queries++
	if tx.queries == 2 && tx.beforeSecondQuery != nil {
		tx.beforeSecondQuery()
	}
	rows, err := tx.Tx.Query(ctx, sql, args...)
	if err == nil && tx.rawSQL != nil && strings.Contains(sql, "api_request_events e") {
		*tx.rawSQL = sql
	}
	if err == nil && tx.rollupSQL != nil && strings.Contains(sql, "api_request_metric_rollups_hourly") {
		*tx.rollupSQL = sql
		return &countedMetricsRows{Rows: rows, count: tx.rollupRows}, nil
	}
	return rows, err
}

func (db *hookedMetricsDB) BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	tx, err := db.pool.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	return &hookedMetricsTx{Tx: tx, beforeSecondQuery: db.beforeSecondQuery}, nil
}

type rollupShapeMetricsDB struct {
	pool       *pgxpool.Pool
	rawSQL     string
	rollupSQL  string
	rollupRows int
}

func (db *rollupShapeMetricsDB) BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	tx, err := db.pool.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	return &hookedMetricsTx{Tx: tx, rawSQL: &db.rawSQL, rollupSQL: &db.rollupSQL, rollupRows: &db.rollupRows}, nil
}

type timezoneMetricsDB struct{ pool *pgxpool.Pool }

func (db timezoneMetricsDB) BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	tx, err := db.pool.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SET LOCAL TIME ZONE 'Asia/Kolkata'`); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
}

func (db *snapshotRecordingMetricsDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return db.pool.Query(ctx, sql, args...)
}

func (db *snapshotRecordingMetricsDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return db.pool.QueryRow(ctx, sql, args...)
}

func (db *snapshotRecordingMetricsDB) BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	db.begins++
	db.options = append(db.options, options)
	return db.pool.BeginTx(ctx, options)
}

func TestPostgresMetricsPinsOneNowAndOneRepeatableReadSnapshot(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	recording := &snapshotRecordingMetricsDB{pool: pool}
	store := NewPostgresMetricsStore(recording)
	nowCalls := 0
	store.now = func() time.Time {
		nowCalls++
		return time.Date(2026, time.July, 29, 18, 30, 0, 0, time.UTC)
	}

	_, _, err := store.Overall(context.Background(), MetricsQuery{
		From: time.Date(2026, time.July, 29, 17, 30, 0, 0, time.UTC),
		To:   time.Date(2026, time.July, 29, 18, 30, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if nowCalls != 1 || recording.begins != 1 || len(recording.options) != 1 {
		t.Fatalf("now calls/begins/options = %d/%d/%d, want 1/1/1", nowCalls, recording.begins, len(recording.options))
	}
	want := pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}
	if recording.options[0] != want {
		t.Fatalf("transaction options = %#v, want %#v", recording.options[0], want)
	}
}

func TestPostgresMetricsWorkspaceSnapshotIgnoresInsertBetweenTotalAndRouteReads(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	ctx := context.Background()
	now := time.Date(2026, time.July, 29, 18, 30, 0, 0, time.UTC)
	workspaceID := fmt.Sprintf("metrics-snapshot-%d", time.Now().UnixNano())
	insert := func(id, path string, duration int) error {
		_, err := pool.Exec(ctx, `INSERT INTO api_request_events(occurred_at,id,workspace_id,api_key_id,method,route_pattern,status_code,duration_ms,outcome) VALUES($1,$2,$3,'key','GET',$4,200,$5,'success')`, now.Add(-time.Minute), id, workspaceID, path, duration)
		return err
	}
	if err := insert("snapshot-initial-"+workspaceID, "/v1/initial", 10); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_events WHERE workspace_id=$1`, workspaceID)
	})
	inserted := make(chan error, 1)
	db := &hookedMetricsDB{pool: pool, beforeSecondQuery: func() {
		go func() { inserted <- insert("snapshot-late-"+workspaceID, "/v1/late", 5000) }()
		if err := <-inserted; err != nil {
			t.Errorf("async insert: %v", err)
		}
	}}
	store := NewPostgresMetricsStore(db)
	nowCalls := 0
	store.now = func() time.Time {
		nowCalls++
		return now
	}
	rows, _, err := store.Workspaces(ctx, MetricsQuery{From: now.Add(-time.Hour), To: now, WorkspaceID: workspaceID, Limit: 10, MinCalls: 1})
	if err != nil {
		t.Fatal(err)
	}
	if nowCalls != 1 || len(rows) != 1 || rows[0].TotalCalls != 1 || rows[0].SlowestEndpoint != "/v1/initial" {
		t.Fatalf("now calls/workspace row=%d/%#v, want one pinned now and total/route from the same pre-insert snapshot", nowCalls, rows)
	}
}

func TestPostgresMetricsRollupsAreGroupedToResponseCardinalityInSQL(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	ctx := context.Background()
	now := time.Date(2026, time.July, 29, 18, 37, 0, 0, time.UTC)
	from := now.Truncate(time.Hour).Add(-72 * time.Hour)
	boundary := now.Truncate(time.Hour).Add(-time.Duration(metricsGuaranteedRawCompletedHours) * time.Hour)
	workspaceID := fmt.Sprintf("metrics-rollup-shape-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_events WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_metric_rollups_hourly WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_metric_rollup_hours WHERE hour_bucket >= $1 AND hour_bucket < $2`, from, boundary)
	})
	if _, err := pool.Exec(ctx, `INSERT INTO api_request_metric_rollup_hours(hour_bucket) SELECT generate_series($1::timestamptz,$2::timestamptz-interval '1 hour',interval '1 hour') ON CONFLICT DO NOTHING`, from, boundary); err != nil {
		t.Fatal(err)
	}
	rollups := []struct {
		hour             time.Time
		path             string
		status, duration int
		hist             []int64
	}{
		{from, "/v1/a", 200, 10, []int64{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{from.Add(time.Hour), "/v1/a", 500, 100, []int64{0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0}},
		{from, "/v1/b", 429, 250, []int64{0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0}},
	}
	for _, row := range rollups {
		errorCount, serverCount, rateCount := 0, 0, 0
		if row.status >= 400 {
			errorCount = 1
		}
		if row.status >= 500 {
			serverCount = 1
		}
		if row.status == 429 {
			rateCount = 1
		}
		if _, err := pool.Exec(ctx, `INSERT INTO api_request_metric_rollups_hourly(hour_bucket,workspace_id,route_pattern,method,status_code,request_count,error_count,server_error_count,validation_error_count,rate_limit_count,duration_sum_ms,duration_min_ms,duration_max_ms,duration_histogram) VALUES($1,$2,$3,'GET',$4,1,$5,$6,0,$7,$8::BIGINT,$8::INTEGER,$8::INTEGER,$9)`, row.hour, workspaceID, row.path, row.status, errorCount, serverCount, rateCount, row.duration, row.hist); err != nil {
			t.Fatal(err)
		}
	}
	for index := range 3 {
		if _, err := pool.Exec(ctx, `INSERT INTO api_request_events(occurred_at,id,workspace_id,api_key_id,method,route_pattern,status_code,duration_ms,outcome) VALUES($1,$2,$3,'key','GET','/v1/raw',200,53,'success')`, now.Add(-time.Duration(index+1)*time.Minute), fmt.Sprintf("rollup-shape-raw-%d-%s", index, workspaceID), workspaceID); err != nil {
			t.Fatal(err)
		}
	}
	shape := &rollupShapeMetricsDB{pool: pool}
	store := NewPostgresMetricsStore(shape)
	store.now = func() time.Time { return now }
	rows, freshness, err := store.Summary(ctx, MetricsQuery{From: from, To: now, WorkspaceID: workspaceID, Sort: "total_calls_desc", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].Path != "/v1/raw" || rows[0].TotalCalls != 3 || rows[1].Path != "/v1/a" || rows[1].TotalCalls != 2 {
		t.Fatalf("sorted/limited summary=%#v", rows)
	}
	if !freshness.Approximate || shape.rollupRows != 2 || !strings.Contains(shape.rollupSQL, "GROUP BY r.route_pattern, r.method") || !strings.Contains(shape.rollupSQL, "SUM(r.request_count)") || strings.Contains(shape.rollupSQL, "JOIN workspaces") {
		t.Fatalf("freshness/rollup rows/sql=%#v/%d/%s, want two response-level DB groups", freshness, shape.rollupRows, shape.rollupSQL)
	}
	limited, limitedFreshness, err := store.Summary(ctx, MetricsQuery{From: from, To: now, WorkspaceID: workspaceID, Sort: "total_calls_desc", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 1 || limited[0].Path != "/v1/raw" || limited[0].P95MS != 53 || limitedFreshness.Approximate || limitedFreshness.DataState != MetricsDataExact {
		t.Fatalf("limited raw-only summary/freshness=%#v/%#v, want actual returned provenance exact", limited, limitedFreshness)
	}

	shape.rawSQL, shape.rollupSQL, shape.rollupRows = "", "", 0
	statuses, statusFreshness, err := store.StatusCodes(ctx, MetricsQuery{From: from, To: now, WorkspaceID: workspaceID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 4 || statuses[0].Path != "/v1/raw" || statuses[0].StatusCode != 200 || statuses[0].TotalCalls != 3 || !statusFreshness.Approximate {
		t.Fatalf("status parity/freshness=%#v/%#v", statuses, statusFreshness)
	}
	if shape.rollupRows != 3 {
		t.Fatalf("status rollup DB rows = %d, want three response-level status groups", shape.rollupRows)
	}
	for source, querySQL := range map[string]string{"raw": shape.rawSQL, "rollup": shape.rollupSQL} {
		for _, forbidden := range []string{"percentile_cont", "duration_histogram", "duration_sum", "duration_min", "duration_max"} {
			if strings.Contains(querySQL, forbidden) {
				t.Fatalf("%s status SQL contains forbidden duration work %q:\n%s", source, forbidden, querySQL)
			}
		}
	}
	limitedStatuses, limitedStatusFreshness, err := store.StatusCodes(ctx, MetricsQuery{From: from, To: now, WorkspaceID: workspaceID, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(limitedStatuses) != 1 || limitedStatuses[0].Path != "/v1/raw" || limitedStatuses[0].TotalCalls != 3 || limitedStatusFreshness.Approximate || limitedStatusFreshness.DataState != MetricsDataExact {
		t.Fatalf("limited raw-only statuses/freshness=%#v/%#v, want count parity and returned provenance exact", limitedStatuses, limitedStatusFreshness)
	}
}

func TestPostgresMetricsCompletedZeroRollupsKeepRecentRawPercentileExact(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	ctx := context.Background()
	now := time.Date(2026, time.July, 29, 18, 37, 0, 0, time.UTC)
	from := now.Truncate(time.Hour).Add(-72 * time.Hour)
	boundary := now.Truncate(time.Hour).Add(-time.Duration(metricsGuaranteedRawCompletedHours) * time.Hour)
	workspaceID := fmt.Sprintf("metrics-zero-rollups-%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, `INSERT INTO api_request_metric_rollup_hours(hour_bucket) SELECT generate_series($1::timestamptz,$2::timestamptz-interval '1 hour',interval '1 hour') ON CONFLICT DO NOTHING`, from, boundary); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO api_request_events(occurred_at,id,workspace_id,api_key_id,method,route_pattern,status_code,duration_ms,outcome) VALUES($1,$2,$3,'key','GET','/v1/recent',200,53,'success')`, now.Add(-time.Hour), "zero-rollup-"+workspaceID, workspaceID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_events WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_metric_rollup_hours WHERE hour_bucket >= $1 AND hour_bucket < $2`, from, boundary)
	})
	store := NewPostgresMetricsStore(pool)
	store.now = func() time.Time { return now }
	got, freshness, err := store.Overall(ctx, MetricsQuery{From: from, To: now, WorkspaceID: workspaceID})
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalCalls != 1 || got.P50MS != 53 || got.P95MS != 53 || got.P99MS != 53 || freshness.Approximate || freshness.DataState != MetricsDataExact {
		t.Fatalf("metrics/freshness=%#v/%#v, want recent exact 53 with actual-provenance exact meta", got, freshness)
	}
}

func TestPostgresMetricsUTCAlignmentUnderFractionalOffsetTimezone(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	ctx := context.Background()
	checkHour := time.Date(2035, time.January, 1, 12, 0, 0, 0, time.UTC)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SET LOCAL TIME ZONE 'Asia/Kolkata'`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO api_request_metric_rollup_hours(hour_bucket) VALUES($1)`, checkHour); err != nil {
		t.Fatalf("valid UTC hour rejected under Asia/Kolkata: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, time.July, 29, 18, 37, 0, 0, time.UTC)
	oldHour := now.Truncate(time.Hour).Add(-30 * time.Hour)
	boundary := now.Truncate(time.Hour).Add(-time.Duration(metricsGuaranteedRawCompletedHours) * time.Hour)
	workspaceID := fmt.Sprintf("metrics-timezone-%d", time.Now().UnixNano())
	for index, row := range []struct {
		at       time.Time
		duration int
	}{{oldHour.Add(time.Minute), 10}, {now.Add(-time.Hour), 53}} {
		if _, err := pool.Exec(ctx, `INSERT INTO api_request_events(occurred_at,id,workspace_id,api_key_id,method,route_pattern,status_code,duration_ms,outcome) VALUES($1,$2,$3,'key','GET','/v1/timezone',200,$4,'success')`, row.at, fmt.Sprintf("timezone-%d-%s", index, workspaceID), workspaceID, row.duration); err != nil {
			t.Fatal(err)
		}
	}
	rollups := NewPostgresRollupStore(pool)
	rollups.now = func() time.Time { return now }
	if err := rollups.RecomputeHour(ctx, oldHour); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO api_request_metric_rollup_hours(hour_bucket) SELECT generate_series($1::timestamptz,$2::timestamptz-interval '1 hour',interval '1 hour') ON CONFLICT DO NOTHING`, oldHour, boundary); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_events WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_metric_rollups_hourly WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_metric_rollup_hours WHERE hour_bucket >= $1 AND hour_bucket < $2`, oldHour, boundary)
	})
	store := NewPostgresMetricsStore(timezoneMetricsDB{pool: pool})
	store.now = func() time.Time { return now }
	trend, _, err := store.Trend(ctx, MetricsQuery{From: oldHour, To: now, WorkspaceID: workspaceID, Interval: "hour"})
	if err != nil {
		t.Fatal(err)
	}
	if len(trend) != 2 || !trend[0].Bucket.Equal(oldHour) || !trend[1].Bucket.Equal(now.Add(-time.Hour).Truncate(time.Hour)) {
		t.Fatalf("UTC mixed buckets=%#v", trend)
	}
}

func TestPostgresMetricsRejectsCorruptRollupHistogram(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	ctx := context.Background()
	now := time.Date(2026, time.July, 29, 18, 37, 0, 0, time.UTC)
	from := now.Truncate(time.Hour).Add(-30 * time.Hour)
	boundary := now.Truncate(time.Hour).Add(-time.Duration(metricsGuaranteedRawCompletedHours) * time.Hour)
	workspaceID := fmt.Sprintf("metrics-corrupt-%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, `INSERT INTO api_request_metric_rollup_hours(hour_bucket) SELECT generate_series($1::timestamptz,$2::timestamptz-interval '1 hour',interval '1 hour') ON CONFLICT DO NOTHING`, from, boundary); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO api_request_metric_rollups_hourly(hour_bucket,workspace_id,route_pattern,method,status_code,request_count,error_count,server_error_count,validation_error_count,rate_limit_count,duration_sum_ms,duration_min_ms,duration_max_ms,duration_histogram) VALUES($1,$2,'/v1/corrupt','GET',200,1,0,0,0,0,5,5,5,$3)`, from, workspaceID, []int64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_metric_rollups_hourly WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_metric_rollup_hours WHERE hour_bucket >= $1 AND hour_bucket < $2`, from, boundary)
	})
	store := NewPostgresMetricsStore(pool)
	store.now = func() time.Time { return now }
	if _, _, err := store.Overall(ctx, MetricsQuery{From: from, To: now, WorkspaceID: workspaceID}); err == nil {
		t.Fatal("corrupt rollup histogram was silently accepted")
	}
}

func TestPostgresMetricsRecentParityFiltersAndAdminWorkspace(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userID, workspaceID, apiKeyID := "metrics-user-"+suffix, "metrics-workspace-"+suffix, "metrics-key-"+suffix
	now := time.Date(2026, time.July, 29, 18, 30, 0, 0, time.UTC)
	from, to := now.Add(-time.Hour), now

	if _, err := pool.Exec(ctx, `INSERT INTO users (id,email) VALUES ($1,$1||'@example.test')`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id,user_id,name) VALUES ($1,$2,'Metrics Workspace')`, workspaceID, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO api_keys (id,name,prefix,key_hash,environment,workspace_id,created_by_user_id) VALUES ($1,'Metrics Key','up_test','hash-'||$1,'test',$2,$3)`, apiKeyID, workspaceID, userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_metrics WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_events WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID)
	})

	cohort := []struct {
		method, path     string
		status, duration int
		outcome          string
	}{
		{"GET", "/v1/metrics-a", 200, 10, "success"},
		{"GET", "/v1/metrics-a", 200, 20, "success"},
		{"GET", "/v1/metrics-a", 429, 250, "rate_limited"},
		{"GET", "/v1/metrics-a", 500, 1000, "server_error"},
		{"POST", "/v1/metrics-b", 201, 53, "success"},
	}
	for i, item := range cohort {
		at := from.Add(time.Duration(i+1) * time.Minute)
		if _, err := pool.Exec(ctx, `INSERT INTO api_metrics(workspace_id,api_key_id,method,path,status_code,duration_ms,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, workspaceID, apiKeyID, item.method, item.path, item.status, item.duration, at); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO api_request_events(occurred_at,id,workspace_id,api_key_id,method,route_pattern,status_code,duration_ms,outcome) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, at, fmt.Sprintf("metrics-%d-%s", i, suffix), workspaceID, apiKeyID, item.method, item.path, item.status, item.duration, item.outcome); err != nil {
			t.Fatal(err)
		}
	}

	store := NewPostgresMetricsStore(pool)
	store.now = func() time.Time { return now }
	query := MetricsQuery{From: from, To: to, WorkspaceID: workspaceID, Limit: 50, MinCalls: 1, Sort: "total_calls_desc", Interval: "hour"}
	got, freshness, err := store.Overall(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := db.New(pool).GetAPIMetricsOverall(ctx, db.GetAPIMetricsOverallParams{WorkspaceID: workspaceID, CreatedAt: pgtype.Timestamptz{Time: from, Valid: true}, CreatedAt_2: pgtype.Timestamptz{Time: to, Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	if freshness.DataState != MetricsDataExact || freshness.Approximate {
		t.Fatalf("freshness=%#v, want exact", freshness)
	}
	if got.TotalCalls != int64(legacy.TotalCalls) || got.SuccessCount != int64(legacy.SuccessCount) || got.ClientErrorCount != int64(legacy.ClientErrorCount) || got.ServerErrorCount != int64(legacy.ServerErrorCount) || got.RateLimitedCount != int64(legacy.RateLimitedCount) {
		t.Fatalf("v2=%#v legacy=%#v", got, legacy)
	}
	if math.Abs(got.ErrorRatePct-legacy.ErrorRatePct) > 0.0001 || got.P50MS != int64(legacy.P50Ms) || got.P95MS != int64(legacy.P95Ms) || got.P99MS != int64(legacy.P99Ms) || got.AvgMS != int64(legacy.AvgMs) {
		t.Fatalf("percentile/rate parity v2=%#v legacy=%#v", got, legacy)
	}

	query.Method = "GET"
	query.Path = "/v1/metrics-a"
	query.StatusClass = "4xx"
	filtered, _, err := store.Overall(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if filtered.TotalCalls != 1 || filtered.RateLimitedCount != 1 || filtered.ClientErrorCount != 1 {
		t.Fatalf("filtered=%#v", filtered)
	}

	query.Method = ""
	query.Path = ""
	query.StatusClass = ""
	summary, _, err := store.Summary(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary) != 2 || summary[0].Path != "/v1/metrics-a" || summary[0].TotalCalls != 4 {
		t.Fatalf("summary=%#v, want two sorted endpoint rows", summary)
	}
	countingDB := &countingMetricsDB{pool: pool}
	countingStore := NewPostgresMetricsStore(countingDB)
	countingStore.now = func() time.Time { return now }
	countedSummary, _, err := countingStore.Summary(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(countedSummary) != 2 || countingDB.queries != 1 {
		t.Fatalf("summary groups/query count = %d/%d, want 2 groups with one raw aggregate query", len(countedSummary), countingDB.queries)
	}
	statusRows, _, err := store.StatusCodes(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(statusRows) != 4 || statusRows[0].StatusCode != 200 || statusRows[0].TotalCalls != 2 {
		t.Fatalf("status rows=%#v, want exact distribution", statusRows)
	}
	trend, _, err := store.Trend(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(trend) != 1 || trend[0].TotalCalls != 5 || trend[0].P95MS != got.P95MS {
		t.Fatalf("trend=%#v, want exact recent bucket", trend)
	}
	workspaces, _, err := store.Workspaces(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(workspaces) != 1 || workspaces[0].WorkspaceID != workspaceID || workspaces[0].TotalCalls != 5 || workspaces[0].SlowestEndpoint != "/v1/metrics-a" {
		t.Fatalf("workspaces=%#v", workspaces)
	}
	legacyWorkspaces, err := db.New(pool).GetAdminAPIMetricsWorkspaces(ctx, db.GetAdminAPIMetricsWorkspacesParams{
		CreatedAt: pgtype.Timestamptz{Time: from, Valid: true}, CreatedAt_2: pgtype.Timestamptz{Time: to, Valid: true},
		SortKey: "total_calls_desc", RowLimit: 50, WorkspaceFilter: workspaceID, MinCalls: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(legacyWorkspaces) != 1 || workspaces[0].P95MS != int64(legacyWorkspaces[0].P95Ms) || workspaces[0].P99MS != int64(legacyWorkspaces[0].P99Ms) {
		t.Fatalf("workspace percentile parity v2=%#v legacy=%#v", workspaces, legacyWorkspaces)
	}
}

func TestPostgresMetricsEmptyRecentRangeMatchesLegacyZeroResult(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	now := time.Date(2026, time.July, 29, 18, 30, 0, 0, time.UTC)
	store := NewPostgresMetricsStore(pool)
	store.now = func() time.Time { return now }

	got, freshness, err := store.Overall(context.Background(), MetricsQuery{
		From: now.Add(-time.Hour), To: now, WorkspaceID: fmt.Sprintf("empty-metrics-%d", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != (MetricsOverall{}) || freshness.DataState != MetricsDataExact {
		t.Fatalf("overall/freshness = %#v/%#v, want zero/exact", got, freshness)
	}
}

func TestPostgresMetricsIncludesExactToBoundaryAndExcludesLaterEvent(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	ctx := context.Background()
	now := time.Date(2026, time.July, 29, 18, 30, 0, 0, time.UTC)
	workspaceID := fmt.Sprintf("metrics-to-boundary-%d", time.Now().UnixNano())
	for index, at := range []time.Time{now, now.Add(time.Microsecond)} {
		if _, err := pool.Exec(ctx, `INSERT INTO api_request_events(occurred_at,id,workspace_id,api_key_id,method,route_pattern,status_code,duration_ms,outcome) VALUES($1,$2,$3,'key','GET','/v1/boundary',200,10,'success')`, at, fmt.Sprintf("metrics-boundary-%d-%s", index, workspaceID), workspaceID); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_events WHERE workspace_id=$1`, workspaceID)
	})
	store := NewPostgresMetricsStore(pool)
	store.now = func() time.Time { return now }
	got, _, err := store.Overall(ctx, MetricsQuery{From: now.Add(-time.Hour), To: now, WorkspaceID: workspaceID})
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalCalls != 1 {
		t.Fatalf("total_calls = %d, want exact-to included and later event excluded", got.TotalCalls)
	}
	statuses, _, err := store.StatusCodes(ctx, MetricsQuery{From: now.Add(-time.Hour), To: now, WorkspaceID: workspaceID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 1 || statuses[0].StatusCode != 200 || statuses[0].TotalCalls != 1 {
		t.Fatalf("status boundary rows=%#v, want exact-to included and later event excluded", statuses)
	}
}

func TestPostgresMetricsUsesCompletedRollupAfterOneDayRawRetention(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	ctx := context.Background()
	now := time.Date(2026, time.July, 29, 18, 37, 0, 0, time.UTC)
	workspaceID := fmt.Sprintf("metrics-retention-%d", time.Now().UnixNano())
	oldHour := now.Truncate(time.Hour).Add(-30 * time.Hour)
	recentAt := now.Add(-2 * time.Hour)
	rollupBoundary := now.Truncate(time.Hour).Add(-time.Duration(metricsGuaranteedRawCompletedHours) * time.Hour)
	for index, row := range []struct {
		at               time.Time
		status, duration int
		outcome          string
	}{{oldHour.Add(time.Minute), 200, 20, "success"}, {recentAt, 500, 900, "server_error"}} {
		if _, err := pool.Exec(ctx, `INSERT INTO api_request_events(occurred_at,id,workspace_id,api_key_id,method,route_pattern,status_code,duration_ms,outcome) VALUES($1,$2,$3,'key','GET','/v1/retention',$4,$5,$6)`, row.at, fmt.Sprintf("metrics-retention-%d-%s", index, workspaceID), workspaceID, row.status, row.duration, row.outcome); err != nil {
			t.Fatal(err)
		}
	}
	rollups := NewPostgresRollupStore(pool)
	rollups.now = func() time.Time { return now }
	if err := rollups.RecomputeHour(ctx, oldHour); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO api_request_metric_rollup_hours(hour_bucket) SELECT generate_series($1::timestamptz,$2::timestamptz-interval '1 hour',interval '1 hour') ON CONFLICT DO NOTHING`, oldHour, rollupBoundary); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM api_request_events WHERE workspace_id=$1 AND occurred_at < $2`, workspaceID, now.Add(-metricsShortestRawRetention)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_events WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_metric_rollups_hourly WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_metric_rollup_hours WHERE hour_bucket >= $1 AND hour_bucket < $2`, oldHour, rollupBoundary)
	})
	store := NewPostgresMetricsStore(pool)
	store.now = func() time.Time { return now }
	got, freshness, err := store.Overall(ctx, MetricsQuery{From: oldHour, To: now, WorkspaceID: workspaceID})
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalCalls != 2 || got.SuccessCount != 1 || got.ServerErrorCount != 1 || freshness.DataState != MetricsDataApproximate || freshness.MissingRollupHours != 0 {
		t.Fatalf("retained metrics/freshness=%#v/%#v, want old rollup + recent raw", got, freshness)
	}
}

func TestPostgresMetricsHistoricalAlignedToUsesRawPointOrReportsDelayed(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	ctx := context.Background()
	now := time.Date(2026, time.July, 29, 18, 37, 0, 0, time.UTC)
	workspaceID := fmt.Sprintf("metrics-point-%d", time.Now().UnixNano())
	from := now.Truncate(time.Hour).Add(-50 * time.Hour)
	to := from.Add(2 * time.Hour)
	beforeID, pointID := "metrics-before-"+workspaceID, "metrics-point-"+workspaceID
	for _, row := range []struct {
		id string
		at time.Time
	}{{beforeID, from.Add(time.Minute)}, {pointID, to}} {
		if _, err := pool.Exec(ctx, `INSERT INTO api_request_events(occurred_at,id,workspace_id,api_key_id,method,route_pattern,status_code,duration_ms,outcome) VALUES($1,$2,$3,'key','GET','/v1/point',200,10,'success')`, row.at, row.id, workspaceID); err != nil {
			t.Fatal(err)
		}
	}
	rollups := NewPostgresRollupStore(pool)
	rollups.now = func() time.Time { return now }
	for _, hour := range []time.Time{from, from.Add(time.Hour)} {
		if err := rollups.RecomputeHour(ctx, hour); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_events WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_metric_rollups_hourly WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_metric_rollup_hours WHERE hour_bucket >= $1 AND hour_bucket <= $2`, from, to)
	})
	store := NewPostgresMetricsStore(pool)
	store.now = func() time.Time { return now }
	query := MetricsQuery{From: from, To: to, WorkspaceID: workspaceID}
	got, freshness, err := store.Overall(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalCalls != 2 || freshness.DataState != MetricsDataApproximate {
		t.Fatalf("point-present metrics/freshness=%#v/%#v, want inclusive point", got, freshness)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM api_request_events WHERE workspace_id=$1 AND id=$2`, workspaceID, pointID); err != nil {
		t.Fatal(err)
	}
	got, freshness, err = store.Overall(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalCalls != 1 || freshness.DataState != MetricsDataDelayed {
		t.Fatalf("point-expired metrics/freshness=%#v/%#v, want rollup-only partial result + delayed", got, freshness)
	}
}

func TestPostgresAdminWorkspaceRawOnlyRowStaysExactBesideMixedWorkspace(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	ctx := context.Background()
	now := time.Date(2026, time.July, 29, 18, 37, 0, 0, time.UTC)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	mixedWS, rawWS := "metrics-mixed-"+suffix, "metrics-raw-"+suffix
	oldHour := now.Truncate(time.Hour).Add(-30 * time.Hour)
	boundary := now.Truncate(time.Hour).Add(-time.Duration(metricsGuaranteedRawCompletedHours) * time.Hour)
	rows := []struct {
		ws, id   string
		at       time.Time
		route    string
		duration int
	}{{mixedWS, "mixed-old-" + suffix, oldHour.Add(time.Minute), "/v1/mixed", 100}, {mixedWS, "mixed-new-" + suffix, now.Add(-time.Hour), "/v1/mixed", 200}, {rawWS, "raw-one-" + suffix, now.Add(-2 * time.Hour), "/v1/a", 1}, {rawWS, "raw-two-" + suffix, now.Add(-time.Hour), "/v1/b", 2}, {rawWS, "raw-three-" + suffix, now.Add(-30 * time.Minute), "/v1/b", 2}}
	for _, row := range rows {
		if _, err := pool.Exec(ctx, `INSERT INTO api_request_events(occurred_at,id,workspace_id,api_key_id,method,route_pattern,status_code,duration_ms,outcome) VALUES($1,$2,$3,'key','GET',$4,200,$5,'success')`, row.at, row.id, row.ws, row.route, row.duration); err != nil {
			t.Fatal(err)
		}
	}
	rollups := NewPostgresRollupStore(pool)
	rollups.now = func() time.Time { return now }
	if err := rollups.RecomputeHour(ctx, oldHour); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO api_request_metric_rollup_hours(hour_bucket) SELECT generate_series($1::timestamptz,$2::timestamptz-interval '1 hour',interval '1 hour') ON CONFLICT DO NOTHING`, oldHour, boundary); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_events WHERE workspace_id=ANY($1::text[])`, []string{mixedWS, rawWS})
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_metric_rollups_hourly WHERE workspace_id=ANY($1::text[])`, []string{mixedWS, rawWS})
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_metric_rollup_hours WHERE hour_bucket >= $1 AND hour_bucket < $2`, oldHour, boundary)
	})
	store := NewPostgresMetricsStore(pool)
	store.now = func() time.Time { return now }
	workspaces, freshness, err := store.Workspaces(ctx, MetricsQuery{From: oldHour, To: now, Limit: 200, MinCalls: 1})
	if err != nil {
		t.Fatal(err)
	}
	var rawRow, mixedRow *AdminMetricsWorkspaceRow
	for i := range workspaces {
		switch workspaces[i].WorkspaceID {
		case rawWS:
			rawRow = &workspaces[i]
		case mixedWS:
			mixedRow = &workspaces[i]
		}
	}
	if freshness.DataState != MetricsDataApproximate || rawRow == nil || mixedRow == nil {
		t.Fatalf("freshness/raw/mixed=%#v/%#v/%#v", freshness, rawRow, mixedRow)
	}
	if rawRow.P95MS != 2 || rawRow.P99MS != 2 {
		t.Fatalf("raw-only workspace percentiles=%#v, want exact 2/2 despite global mixed range", rawRow)
	}
	limited, limitedFreshness, err := store.Workspaces(ctx, MetricsQuery{From: oldHour, To: now, Sort: "total_calls_desc", Limit: 1, MinCalls: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 1 || limited[0].WorkspaceID != rawWS || limitedFreshness.Approximate || limitedFreshness.DataState != MetricsDataExact {
		t.Fatalf("limited raw-only response/freshness=%#v/%#v, want actual returned provenance exact", limited, limitedFreshness)
	}
}

func TestPostgresMetricsHistoricalRollupBoundaryAndDelayedState(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	workspaceID := "metrics-history-" + suffix
	now := time.Date(2026, time.July, 29, 18, 30, 0, 0, time.UTC)
	historicalHour := now.Truncate(time.Hour).Add(-72 * time.Hour)
	recentAt := now.Add(-time.Hour)
	rollupBoundary := now.Truncate(time.Hour).Add(-time.Duration(metricsGuaranteedRawCompletedHours) * time.Hour)
	if _, err := pool.Exec(ctx, `DELETE FROM api_request_metric_rollup_hours WHERE hour_bucket >= $1 AND hour_bucket < $2`, historicalHour, rollupBoundary); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_events WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_metric_rollups_hourly WHERE workspace_id=$1`, workspaceID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_metric_rollup_hours WHERE hour_bucket >= $1 AND hour_bucket < $2`, historicalHour, rollupBoundary)
	})
	for i, row := range []struct {
		at               time.Time
		status, duration int
		outcome          string
	}{{historicalHour.Add(time.Minute), 200, 20, "success"}, {recentAt, 500, 900, "server_error"}} {
		if _, err := pool.Exec(ctx, `INSERT INTO api_request_events(occurred_at,id,workspace_id,api_key_id,method,route_pattern,status_code,duration_ms,outcome) VALUES($1,$2,$3,'key','GET','/v1/mixed',$4,$5,$6)`, row.at, fmt.Sprintf("metrics-history-%d-%s", i, suffix), workspaceID, row.status, row.duration, row.outcome); err != nil {
			t.Fatal(err)
		}
	}
	rollups := NewPostgresRollupStore(pool)
	rollups.now = func() time.Time { return now }
	if err := rollups.RecomputeHour(ctx, historicalHour); err != nil {
		t.Fatal(err)
	}
	store := NewPostgresMetricsStore(pool)
	store.now = func() time.Time { return now }
	query := MetricsQuery{From: historicalHour, To: now, WorkspaceID: workspaceID, Limit: 50, MinCalls: 1}
	got, freshness, err := store.Overall(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalCalls != 2 || got.SuccessCount != 1 || got.ServerErrorCount != 1 {
		t.Fatalf("mixed=%#v, want no boundary double count", got)
	}
	if freshness.DataState != MetricsDataDelayed || freshness.MissingRollupHours == 0 {
		t.Fatalf("freshness=%#v, want delayed because intervening hours were not recomputed", freshness)
	}

	if _, err := pool.Exec(ctx, `INSERT INTO api_request_metric_rollup_hours(hour_bucket) SELECT generate_series($1::timestamptz,$2::timestamptz-interval '1 hour',interval '1 hour') ON CONFLICT DO NOTHING`, ceilHour(query.From), rollupBoundary); err != nil {
		t.Fatal(err)
	}
	got, freshness, err = store.Overall(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalCalls != 2 || freshness.DataState != MetricsDataApproximate || freshness.MissingRollupHours != 0 {
		t.Fatalf("complete mixed=%#v freshness=%#v", got, freshness)
	}
	query.Interval = "hour"
	trend, _, err := store.Trend(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	var recent *MetricsTrendRow
	for i := range trend {
		if trend[i].Bucket.Equal(recentAt.Truncate(time.Hour)) {
			recent = &trend[i]
		}
	}
	if recent == nil || recent.P95MS != 900 {
		t.Fatalf("recent raw trend bucket=%#v, want exact p95 900 despite historical rollups", recent)
	}
}
