package worker

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestTerminalPostEventOutboxWorkerIsWiredInAPIProcess(t *testing.T) {
	source, err := os.ReadFile("../../cmd/api/main.go")
	if err != nil {
		t.Fatal(err)
	}
	mainSource := string(source)
	for _, want := range []string{
		"NewTerminalPostEventOutboxWorker(",
		"go terminalPostEventWorker.Start(workerCtx)",
	} {
		if !strings.Contains(mainSource, want) {
			t.Fatalf("API process missing terminal post event outbox wiring %q", want)
		}
	}
}

type fakeTerminalPostEventOutboxStore struct {
	rows       []db.ClaimPendingTerminalPostEventDeliveriesRow
	enqueued   []string
	retried    []string
	lastErrors []string
}

func (s *fakeTerminalPostEventOutboxStore) ClaimPendingTerminalPostEventDeliveries(context.Context, int32) ([]db.ClaimPendingTerminalPostEventDeliveriesRow, error) {
	rows := append([]db.ClaimPendingTerminalPostEventDeliveriesRow(nil), s.rows...)
	s.rows = nil
	return rows, nil
}

func (s *fakeTerminalPostEventOutboxStore) MarkTerminalPostEventDeliveryEnqueued(_ context.Context, arg db.MarkTerminalPostEventDeliveryEnqueuedParams) (int64, error) {
	s.enqueued = append(s.enqueued, arg.EventID+":"+arg.Sink)
	return 1, nil
}

func (s *fakeTerminalPostEventOutboxStore) RetryTerminalPostEventDelivery(_ context.Context, arg db.RetryTerminalPostEventDeliveryParams) (int64, error) {
	s.retried = append(s.retried, arg.EventID+":"+arg.Sink)
	s.lastErrors = append(s.lastErrors, arg.LastError.String)
	return 1, nil
}

type flakyDurableEventBus struct {
	failures int
	eventIDs []string
}

func (b *flakyDurableEventBus) Publish(context.Context, string, string, any) {}

func (b *flakyDurableEventBus) PublishDurable(_ context.Context, eventID, _, _ string, _ []byte) error {
	b.eventIDs = append(b.eventIDs, eventID)
	if b.failures > 0 {
		b.failures--
		return errors.New("delivery ledger temporarily unavailable")
	}
	return nil
}

func TestTerminalPostEventOutboxRetriesWithStableEventID(t *testing.T) {
	row := db.ClaimPendingTerminalPostEventDeliveriesRow{
		EventID:     "evt_1",
		Sink:        "webhook",
		WorkspaceID: "ws_1",
		PostID:      "post_1",
		EventType:   "post.failed",
		Payload:     []byte(`{"id":"post_1","status":"failed"}`),
		Attempts:    1,
	}
	store := &fakeTerminalPostEventOutboxStore{rows: []db.ClaimPendingTerminalPostEventDeliveriesRow{row}}
	bus := &flakyDurableEventBus{failures: 1}
	worker := NewTerminalPostEventOutboxWorker(store, bus, bus, bus)

	worker.tick(context.Background())
	if len(store.enqueued) != 0 || len(store.retried) != 1 {
		t.Fatalf("after failure enqueued=%v retried=%v", store.enqueued, store.retried)
	}
	store.rows = []db.ClaimPendingTerminalPostEventDeliveriesRow{row}
	worker.tick(context.Background())

	if len(store.enqueued) != 1 || store.enqueued[0] != row.EventID+":"+row.Sink {
		t.Fatalf("enqueued = %v", store.enqueued)
	}
	if len(bus.eventIDs) != 2 || bus.eventIDs[0] != row.EventID || bus.eventIDs[1] != row.EventID {
		t.Fatalf("stable event ids = %v", bus.eventIDs)
	}
	if len(store.lastErrors) != 1 || store.lastErrors[0] == "" {
		t.Fatalf("retry errors = %v", store.lastErrors)
	}
	if got := terminalPostEventRetryAt(worker.now(), 2); !got.After(worker.now()) {
		t.Fatalf("retry time = %v", got)
	}
}
