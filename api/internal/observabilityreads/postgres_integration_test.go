//go:build integration

package observabilityreads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/testdbguard"
)

const observabilityReadsIntegrationDatabaseEnv = "REQUEST_EVENTS_TEST_DATABASE_URL"

func openObservabilityReadsIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv(observabilityReadsIntegrationDatabaseEnv))
	if databaseURL == "" {
		t.Fatalf("%s is required", observabilityReadsIntegrationDatabaseEnv)
	}
	pool, _, err := testdbguard.OpenValidated(
		observabilityReadsIntegrationDatabaseEnv,
		databaseURL,
		func(validatedURL string) (*pgxpool.Pool, error) {
			return pgxpool.New(context.Background(), validatedURL)
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestPostgresLogReadsWorkspaceSafetyMetadataAndCursorContinuity(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	store := NewPostgresLogStore(pool)
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	workspaceA := "observability-reads-a-" + suffix
	workspaceB := "observability-reads-b-" + suffix
	userA := "observability-user-a-" + suffix
	userB := "observability-user-b-" + suffix
	base := time.Date(2026, time.July, 29, 17, 0, 0, 0, time.UTC)
	requestIDs := []string{
		fmt.Sprintf("%020d_%s", base.UnixMicro(), "request-a3-"+suffix),
		fmt.Sprintf("%020d_%s", base.UnixMicro(), "request-a2-"+suffix),
		fmt.Sprintf("%020d_%s", base.Add(-time.Second).UnixMicro(), "request-b-"+suffix),
	}

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email) VALUES ($1, $1 || '@example.test'), ($2, $2 || '@example.test')`, userA, userB); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, user_id, name) VALUES ($1, $2, 'A'), ($3, $4, 'B')`, workspaceA, userA, workspaceB, userB); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_events WHERE id = ANY($1::TEXT[])`, requestIDs)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = ANY($1::TEXT[])`, []string{userA, userB})
	})

	for _, row := range []struct {
		id, workspace, outcome string
		at                     time.Time
		status                 int
		hasDetail              bool
	}{
		{id: requestIDs[0], workspace: workspaceA, at: base, outcome: "success", status: 200},
		{id: requestIDs[1], workspace: workspaceA, at: base, outcome: "server_error", status: 500, hasDetail: true},
		{id: requestIDs[2], workspace: workspaceB, at: base.Add(-time.Second), outcome: "success", status: 200},
	} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO api_request_events (
				occurred_at, id, workspace_id, api_key_id, request_id, method,
				route_pattern, status_code, duration_ms, outcome, has_error_detail
			) VALUES ($1, $2, $3, 'key', $2, 'GET', '/v1/test', $4, 12, $5, $6)
		`, row.at, row.id, row.workspace, row.status, row.outcome, row.hasDetail); err != nil {
			t.Fatal(err)
		}
	}

	integrationIDs := make([]int64, 25)
	for index := range integrationIDs {
		if err := pool.QueryRow(ctx, `
			INSERT INTO integration_logs (
				workspace_id, ts, level, status, category, action, source, message,
				metadata, request_payload, response_payload
			) VALUES ($1, $2, 'error', 'error', 'oauth', 'oauth.failed', 'oauth', 'safe', '{"safe":true}', '{"secret":"[REDACTED]"}', '{"ok":false}')
			RETURNING id
		`, workspaceA, base).Scan(&integrationIDs[index]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO integration_logs (workspace_id, ts, level, status, category, action, source, message)
		VALUES ($1, $2, 'info', 'success', 'api_request', 'duplicate', 'api', 'must be excluded')
	`, workspaceA, base.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	filters := LogFilters{From: base.Add(-time.Hour), To: base.Add(time.Hour)}
	var cursor *LogCursor
	var ids []string
	for {
		page, err := store.ListWorkspace(ctx, workspaceA, filters, cursor, 7)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Logs) > 7 {
			t.Fatalf("page returned %d rows for limit 7", len(page.Logs))
		}
		for _, item := range page.Logs {
			if item.WorkspaceID != workspaceA {
				t.Fatalf("cross-workspace row returned: %#v", item)
			}
			if item.Category == "api_request" && item.SourceKind == LogSourceIntegration {
				t.Fatalf("duplicate legacy api_request row returned: %#v", item)
			}
			ids = append(ids, item.ID)
		}
		if page.NextCursor == nil {
			break
		}
		cursor = page.NextCursor
	}
	want := []string{
		"request:" + requestIDs[0],
		"request:" + requestIDs[1],
	}
	for index := len(integrationIDs) - 1; index >= 0; index-- {
		want = append(want, fmt.Sprintf("integration:%d", integrationIDs[index]))
	}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("paged IDs = %v, want %v", ids, want)
	}
	compatibilityFilters := filters
	compatibilityFilters.Status = "error"
	compatibilityFilters.Action = "api.request.failed"
	compatibilityPage, err := store.ListWorkspace(ctx, workspaceA, compatibilityFilters, nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := projectionIDs(compatibilityPage.Logs); !reflect.DeepEqual(got, []string{"request:" + requestIDs[1]}) {
		t.Fatalf("legacy-compatible canonical status/action IDs = %v, want failed request", got)
	}

	adminFilters := filters
	adminFilters.OwnerEmail = suffix
	adminPage, err := store.ListAdmin(ctx, adminFilters, nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	wantAdmin := append(append([]string(nil), want...), "request:"+requestIDs[2])
	if got := projectionIDs(adminPage.Logs); !reflect.DeepEqual(got, wantAdmin) {
		t.Fatalf("admin global IDs = %v, want %v", got, wantAdmin)
	}
	generalSearchFilters := filters
	generalSearchFilters.Query = userA + "@example.test"
	generalSearchPage, err := store.ListAdmin(ctx, generalSearchFilters, nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	if got := projectionIDs(generalSearchPage.Logs); !reflect.DeepEqual(got, want) {
		t.Fatalf("owner-email-only q search IDs = %v, want %v", got, want)
	}
	adminDetail, err := store.GetAdmin(ctx, "request:"+requestIDs[1])
	if err != nil || adminDetail.WorkspaceID != workspaceA || adminDetail.DetailStatus != "expired" {
		t.Fatalf("admin detail = %#v err=%v", adminDetail, err)
	}
	successDetail, err := store.GetWorkspace(ctx, workspaceA, "request:"+requestIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	if successDetail.DetailStatus != "" || successDetail.RequestPayload != nil || successDetail.ResponsePayload != nil {
		t.Fatalf("canonical success unexpectedly loaded detail: %#v", successDetail)
	}
}

func TestPostgresWorkspaceSeekIndexIsValidAndUsed(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	ctx := context.Background()
	var valid bool
	var definition string
	if err := pool.QueryRow(ctx, `
		SELECT i.indisvalid, pg_get_indexdef(i.indexrelid)
		FROM pg_index i
		JOIN pg_class c ON c.oid = i.indexrelid
		WHERE c.relname = 'idx_integration_logs_workspace_ts_id'
	`).Scan(&valid, &definition); err != nil {
		t.Fatal(err)
	}
	if !valid || !strings.Contains(strings.ToLower(definition), "(workspace_id, ts desc, id desc)") {
		t.Fatalf("workspace seek index valid=%v definition=%q", valid, definition)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SET LOCAL enable_seqscan = off`); err != nil {
		t.Fatal(err)
	}
	rows, err := tx.Query(ctx, `
		EXPLAIN (COSTS OFF)
		SELECT id
		FROM integration_logs
		WHERE workspace_id = 'index-plan-workspace'
		  AND ts >= '2026-07-29T00:00:00Z'
		  AND ts <= '2026-07-30T00:00:00Z'
		  AND (ts < '2026-07-30T00:00:00Z' OR (ts = '2026-07-30T00:00:00Z' AND id < 1000))
		ORDER BY ts DESC, id DESC
		LIMIT 8
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(line)
		plan.WriteByte('\n')
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan.String(), "idx_integration_logs_workspace_ts_id") {
		t.Fatalf("workspace seek EXPLAIN did not use index:\n%s", plan.String())
	}
}

func TestPostgresLogDetailIsolationExpiredAndCompatibility(t *testing.T) {
	pool := openObservabilityReadsIntegrationPool(t)
	store := NewPostgresLogStore(pool)
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	workspaceA := "observability-detail-a-" + suffix
	workspaceB := "observability-detail-b-" + suffix
	userA := "observability-detail-user-a-" + suffix
	userB := "observability-detail-user-b-" + suffix
	at := time.Date(2026, time.July, 29, 18, 0, 0, 0, time.UTC)
	requestID := fmt.Sprintf("%020d_%s", at.UnixMicro(), "expired-"+suffix)

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email) VALUES ($1, $1 || '@example.test'), ($2, $2 || '@example.test')`, userA, userB); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, user_id) VALUES ($1, $2), ($3, $4)`, workspaceA, userA, workspaceB, userB); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_events WHERE id = $1`, requestID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = ANY($1::TEXT[])`, []string{userA, userB})
	})
	if _, err := pool.Exec(ctx, `
		INSERT INTO api_request_events (
			occurred_at, id, workspace_id, api_key_id, method, route_pattern,
			status_code, duration_ms, outcome, error_code, has_error_detail
		) VALUES ($1, $2, $3, 'key', 'POST', '/v1/test', 500, 42, 'server_error', 'internal_error', TRUE)
	`, at, requestID, workspaceA); err != nil {
		t.Fatal(err)
	}

	detail, err := store.GetWorkspace(ctx, workspaceA, "request:"+requestID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.DetailStatus != "expired" || detail.RequestPayload != nil || detail.ResponsePayload != nil {
		t.Fatalf("expired canonical detail = %#v", detail)
	}
	for name, workspace := range map[string]string{"cross workspace": workspaceB, "unknown workspace": "missing-" + suffix} {
		t.Run(name, func(t *testing.T) {
			_, err := store.GetWorkspace(ctx, workspace, "request:"+requestID)
			if !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("GetWorkspace() error = %v, want pgx.ErrNoRows", err)
			}
		})
	}
	if _, err := store.GetWorkspace(ctx, workspaceA, "request:"+fmt.Sprintf("%020d_missing", at.UnixMicro())); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("unknown detail error = %v, want pgx.ErrNoRows", err)
	}

	var integrationID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO integration_logs (
			workspace_id, ts, level, status, category, action, source, message,
			request_payload, response_payload
		) VALUES ($1, $2, 'error', 'error', 'webhook', 'delivery.failed', 'webhook', 'failed', '{"token":"[REDACTED]"}', '{"error":"safe"}')
		RETURNING id
	`, workspaceA, at).Scan(&integrationID); err != nil {
		t.Fatal(err)
	}
	integrationDetail, err := store.GetWorkspace(ctx, workspaceA, fmt.Sprintf("integration:%d", integrationID))
	if err != nil {
		t.Fatal(err)
	}
	var requestPayload, responsePayload map[string]string
	requestErr := json.Unmarshal(integrationDetail.RequestPayload, &requestPayload)
	responseErr := json.Unmarshal(integrationDetail.ResponsePayload, &responsePayload)
	if requestErr != nil || responseErr != nil || requestPayload["token"] != "[REDACTED]" || responsePayload["error"] != "safe" {
		t.Fatalf("integration payload compatibility lost: %#v", integrationDetail)
	}
	legacyIntegrationDetail, err := store.GetWorkspace(ctx, workspaceA, fmt.Sprintf("%d", integrationID))
	if err != nil || legacyIntegrationDetail.ID != fmt.Sprintf("integration:%d", integrationID) {
		t.Fatalf("legacy numeric workspace detail = %#v err=%v", legacyIntegrationDetail, err)
	}
	legacyAdminDetail, err := store.GetAdmin(ctx, fmt.Sprintf("%d", integrationID))
	if err != nil || legacyAdminDetail.ID != fmt.Sprintf("integration:%d", integrationID) {
		t.Fatalf("legacy numeric admin detail = %#v err=%v", legacyAdminDetail, err)
	}
	for _, invalidID := range []string{"0", "-1", "+1", "01", "9223372036854775808"} {
		if _, err := store.GetWorkspace(ctx, workspaceA, invalidID); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("GetWorkspace(%q) error = %v, want pgx.ErrNoRows", invalidID, err)
		}
		if _, err := store.GetAdmin(ctx, invalidID); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("GetAdmin(%q) error = %v, want pgx.ErrNoRows", invalidID, err)
		}
	}
	if _, err := store.GetWorkspace(ctx, workspaceB, fmt.Sprintf("integration:%d", integrationID)); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cross-workspace integration detail error = %v, want pgx.ErrNoRows", err)
	}
}
