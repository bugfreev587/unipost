//go:build integration

package requestevents

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/testdbguard"
)

const requestEventsIntegrationDatabaseEnv = "REQUEST_EVENTS_TEST_DATABASE_URL"

func openRequestEventsIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv(requestEventsIntegrationDatabaseEnv))
	if databaseURL == "" {
		t.Fatalf("%s is required", requestEventsIntegrationDatabaseEnv)
	}
	pool, _, err := testdbguard.OpenValidated(
		requestEventsIntegrationDatabaseEnv,
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

func TestPostgresStorePartitionRoutingAtomicityAndCascade(t *testing.T) {
	pool := openRequestEventsIntegrationPool(t)
	store := NewPostgresStore(pool)
	ctx := context.Background()
	ids := []string{"integration-success-w31", "integration-success-w32", "integration-failure", "integration-rollback"}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM api_request_events WHERE id = ANY($1::TEXT[])`, ids)
	})

	week31 := validEvent(ids[0], httpStatusOK)
	week31.WorkspaceID = "workspace-a"
	week31.OccurredAt = time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	week32 := validEvent(ids[1], httpStatusCreated)
	week32.WorkspaceID = "workspace-b"
	week32.OccurredAt = time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	if err := store.InsertSuccessBatch(ctx, []Event{week31, week32}); err != nil {
		t.Fatal(err)
	}

	for _, assertion := range []struct {
		id   string
		want string
	}{
		{ids[0], "api_request_events_2026w31"},
		{ids[1], "api_request_events_2026w32"},
	} {
		var partition string
		if err := pool.QueryRow(ctx, `SELECT tableoid::regclass::TEXT FROM api_request_events WHERE id = $1`, assertion.id).Scan(&partition); err != nil {
			t.Fatal(err)
		}
		if partition != assertion.want {
			t.Fatalf("event %s partition = %q, want %q", assertion.id, partition, assertion.want)
		}
	}
	var firstCreatedAt, lastCreatedAt time.Time
	if err := pool.QueryRow(ctx, `
		SELECT MIN(created_at), MAX(created_at)
		FROM api_request_events
		WHERE id = ANY($1::TEXT[])
	`, ids[:2]).Scan(&firstCreatedAt, &lastCreatedAt); err != nil {
		t.Fatalf("read controlled parity window: %v", err)
	}
	parity, err := NewPostgresParityStore(pool).CompareWindow(ctx, firstCreatedAt.Add(-time.Millisecond), lastCreatedAt.Add(time.Millisecond))
	if err != nil {
		t.Fatalf("execute parity query: %v", err)
	}
	if parity.LegacyCount != 0 || parity.CanonicalCount != 2 || parity.MismatchedGroups != 2 || parity.AbsoluteDelta != 2 {
		t.Fatalf("parity query result = %#v, want two canonical-only dimensions", parity)
	}
	controlledParity, err := NewPostgresParityStore(pool).CompareControlledCohort(
		ctx,
		week31.WorkspaceID,
		week31.APIKeyID,
		week31.Method,
		week31.RoutePattern,
		week31.StatusCode,
	)
	if err != nil {
		t.Fatalf("execute controlled cohort parity query: %v", err)
	}
	if controlledParity.LegacyCount != 0 || controlledParity.CanonicalCount != 1 || controlledParity.MismatchedGroups != 1 || controlledParity.AbsoluteDelta != 1 {
		t.Fatalf("controlled cohort parity = %#v, want one canonical-only event", controlledParity)
	}

	failure := validEvent(ids[2], httpStatusInternalServerError)
	failure.WorkspaceID = "workspace-a"
	failure.OccurredAt = week31.OccurredAt.Add(time.Hour)
	failure.HasErrorDetail = true
	detail := &Detail{
		EventID:                failure.ID,
		OccurredAt:             failure.OccurredAt,
		WorkspaceID:            failure.WorkspaceID,
		RequestHeaders:         map[string]string{"Content-Type": "application/json"},
		ResponseHeaders:        map[string]string{"Content-Type": "application/json"},
		RequestPayload:         `{"safe":true}`,
		ResponsePayload:        `{"code":"internal_error"}`,
		RequestOriginalBytes:   13,
		RequestStoredBytes:     13,
		ResponseOriginalBytes:  25,
		ResponseStoredBytes:    25,
		RequestContentType:     "application/json",
		ResponseContentType:    "application/json",
		RedactionPolicyVersion: 1,
	}
	if err := store.InsertFailure(ctx, failure, detail); err != nil {
		t.Fatal(err)
	}
	var eventCount, detailCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM api_request_events WHERE workspace_id = 'workspace-a'`).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 2 {
		t.Fatalf("workspace-scoped event count = %d, want 2", eventCount)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO api_request_error_details (
			occurred_at, event_id, workspace_id, redaction_policy_version
		) VALUES ($1, $2, 'workspace-cross-tenant', 1)`, week31.OccurredAt, week31.ID); err == nil {
		t.Fatal("database accepted detail whose workspace differs from its parent event")
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO api_request_error_details (
			occurred_at, event_id, workspace_id, request_content_type, redaction_policy_version
		) VALUES ($1, $2, $3, $4, 1)`, week32.OccurredAt, week32.ID, week32.WorkspaceID, strings.Repeat("a", 72*1024)); err == nil {
		t.Fatal("database accepted detail whose full serialized size exceeds 70 KiB")
	}
	largeHeader := `{"safe":"` + strings.Repeat("h", 5*1024) + `"}`
	largePayload := strings.Repeat("p", 32*1024)
	if _, err := pool.Exec(ctx, `
		INSERT INTO api_request_error_details (
			occurred_at, event_id, workspace_id,
			request_headers, response_headers,
			request_payload, response_payload,
			request_stored_bytes, response_stored_bytes,
			redaction_policy_version
		) VALUES ($1, $2, $3, $4::JSONB, $4::JSONB, $5, $5, $6, $6, 1)
	`, week32.OccurredAt, week32.ID, week32.WorkspaceID, largeHeader, largePayload, len(largePayload)); err == nil {
		t.Fatal("database accepted individually valid fields whose combined serialized size exceeds 70 KiB")
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM api_request_error_details WHERE event_id = $1`, failure.ID).Scan(&detailCount); err != nil {
		t.Fatal(err)
	}
	if detailCount != 1 {
		t.Fatalf("failure detail count = %d, want 1", detailCount)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM api_request_events WHERE occurred_at = $1 AND id = $2`, failure.OccurredAt, failure.ID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM api_request_error_details WHERE event_id = $1`, failure.ID).Scan(&detailCount); err != nil {
		t.Fatal(err)
	}
	if detailCount != 0 {
		t.Fatalf("detail count after parent delete = %d, want 0", detailCount)
	}

	rollbackEvent := validEvent(ids[3], httpStatusInternalServerError)
	rollbackEvent.WorkspaceID = "workspace-a"
	rollbackEvent.OccurredAt = week31.OccurredAt.Add(2 * time.Hour)
	rollbackEvent.HasErrorDetail = true
	invalidDetail := &Detail{
		EventID:                rollbackEvent.ID,
		OccurredAt:             rollbackEvent.OccurredAt,
		WorkspaceID:            rollbackEvent.WorkspaceID,
		RequestStoredBytes:     1,
		RedactionPolicyVersion: 1,
	}
	if err := store.InsertFailure(ctx, rollbackEvent, invalidDetail); err == nil {
		t.Fatal("invalid detail must fail atomically")
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM api_request_events WHERE id = $1`, rollbackEvent.ID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 0 {
		t.Fatalf("rolled-back failure event count = %d, want 0", eventCount)
	}
}
