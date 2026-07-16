package worker

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
)

type fakeXInboundOutboxStore struct {
	rows       []db.XInboundCapNotification
	enqueued   []string
	retryIDs   []string
	lastErrors []string
}

func TestXInboundNotificationOutboxWorkerIsWiredInAPIProcess(t *testing.T) {
	source, err := os.ReadFile("../../cmd/api/main.go")
	if err != nil {
		t.Fatal(err)
	}
	mainSource := string(source)
	for _, want := range []string{
		"NewXInboundNotificationOutboxWorker(queries, notificationDispatcher)",
		"go xInboundNotificationWorker.Start(workerCtx)",
	} {
		if !strings.Contains(mainSource, want) {
			t.Fatalf("API process missing X inbound outbox wiring %q", want)
		}
	}
}

func (s *fakeXInboundOutboxStore) ClaimPendingXInboundNotifications(context.Context, int32) ([]db.XInboundCapNotification, error) {
	rows := append([]db.XInboundCapNotification(nil), s.rows...)
	s.rows = nil
	return rows, nil
}

func (s *fakeXInboundOutboxStore) MarkXInboundNotificationEnqueued(_ context.Context, id string) error {
	s.enqueued = append(s.enqueued, id)
	return nil
}

func (s *fakeXInboundOutboxStore) RetryXInboundNotification(_ context.Context, arg db.RetryXInboundNotificationParams) error {
	s.retryIDs = append(s.retryIDs, arg.ID)
	s.lastErrors = append(s.lastErrors, arg.LastError.String)
	return nil
}

type flakyXInboundEnqueuer struct {
	failures int
	eventIDs []string
	payloads [][]byte
}

func (e *flakyXInboundEnqueuer) EnqueueXInboundNotification(_ context.Context, _ string, _ string, eventID string, payload []byte) error {
	e.eventIDs = append(e.eventIDs, eventID)
	e.payloads = append(e.payloads, append([]byte(nil), payload...))
	if e.failures > 0 {
		e.failures--
		return errors.New("notification database temporarily unavailable")
	}
	return nil
}

func TestXInboundNotificationOutboxRetriesUntilDurablyEnqueued(t *testing.T) {
	row := db.XInboundCapNotification{
		ID:          "xin_1",
		WorkspaceID: "ws_1",
		Threshold:   80,
		EventType:   events.EventBillingXInbound80pct,
		Payload:     []byte(`{"workspace_id":"ws_1","inbound_daily_usage":320,"inbound_daily_limit":400,"reset_at":"2026-07-17T00:00:00Z"}`),
	}
	store := &fakeXInboundOutboxStore{rows: []db.XInboundCapNotification{row}}
	enqueuer := &flakyXInboundEnqueuer{failures: 1}
	worker := NewXInboundNotificationOutboxWorker(store, enqueuer)

	worker.tick(context.Background())
	if len(store.enqueued) != 0 || len(store.retryIDs) != 1 {
		t.Fatalf("after failure enqueued=%v retries=%v", store.enqueued, store.retryIDs)
	}
	store.rows = []db.XInboundCapNotification{row}
	worker.tick(context.Background())

	if len(store.enqueued) != 1 || store.enqueued[0] != row.ID {
		t.Fatalf("enqueued = %v", store.enqueued)
	}
	if len(enqueuer.eventIDs) != 2 || enqueuer.eventIDs[0] != row.ID || enqueuer.eventIDs[1] != row.ID {
		t.Fatalf("stable event ids = %v", enqueuer.eventIDs)
	}
	for _, payload := range enqueuer.payloads {
		if strings.Contains(strings.ToLower(string(payload)), "body") || strings.Contains(string(payload), "private") {
			t.Fatalf("outbox payload leaked content: %s", payload)
		}
	}
	if len(store.lastErrors) != 1 || store.lastErrors[0] == "" {
		t.Fatalf("retry errors = %v", store.lastErrors)
	}
}

func TestXInboundNotificationOutboxMapsThresholdToCuratedEvent(t *testing.T) {
	for _, tc := range []struct {
		threshold int16
		want      string
	}{
		{80, events.EventBillingXInbound80pct},
		{100, events.EventBillingXInboundCapReached},
	} {
		if got := xInboundEventForThreshold(tc.threshold); got != tc.want {
			t.Fatalf("event for %d = %q, want %q", tc.threshold, got, tc.want)
		}
	}
	arg := db.RetryXInboundNotificationParams{
		ID:            "xin_1",
		NextAttemptAt: pgtype.Timestamptz{Time: xInboundOutboxRetryAt(time.Unix(0, 0), 3), Valid: true},
	}
	if !arg.NextAttemptAt.Time.After(time.Unix(0, 0)) {
		t.Fatalf("retry time = %s", arg.NextAttemptAt.Time)
	}
}
