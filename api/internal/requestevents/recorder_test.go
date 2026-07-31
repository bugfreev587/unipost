package requestevents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type recordingStore struct {
	mu             sync.Mutex
	successBatches [][]Event
	failures       []storedFailure
	successErr     error
	failureErr     error
	wrote          chan struct{}
}

type storedFailure struct {
	event  Event
	detail Detail
}

func (s *recordingStore) InsertSuccessBatch(_ context.Context, events []Event) error {
	s.mu.Lock()
	s.successBatches = append(s.successBatches, append([]Event(nil), events...))
	err := s.successErr
	s.mu.Unlock()
	signalRecorderWrite(s.wrote)
	return err
}

func (s *recordingStore) InsertFailure(_ context.Context, event Event, detail *Detail) error {
	s.mu.Lock()
	if detail != nil {
		s.failures = append(s.failures, storedFailure{event: event, detail: *detail})
	} else {
		s.failures = append(s.failures, storedFailure{event: event})
	}
	err := s.failureErr
	s.mu.Unlock()
	signalRecorderWrite(s.wrote)
	return err
}

func signalRecorderWrite(ch chan struct{}) {
	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (s *recordingStore) snapshot() ([][]Event, []storedFailure) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]Event(nil), s.successBatches...), append([]storedFailure(nil), s.failures...)
}

func TestRecorderBatchesSuccessEvents(t *testing.T) {
	store := &recordingStore{wrote: make(chan struct{}, 1)}
	recorder := NewRecorder(store, Config{NormalQueueSize: 4, BatchSize: 2, FlushInterval: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go recorder.Start(ctx)

	recorder.Enqueue(validEvent("event-1", httpStatusOK), nil)
	recorder.Enqueue(validEvent("event-2", httpStatusCreated), nil)
	stats := waitForRecorderStats(t, recorder, "success batch completion", func(stats Stats) bool {
		return stats.SuccessWritten == 2 && stats.SuccessBatchWrites == 1
	})

	batches, failures := store.snapshot()
	if len(batches) != 1 || len(batches[0]) != 2 || len(failures) != 0 {
		t.Fatalf("writes = batches %#v failures %#v", batches, failures)
	}
	if stats.SuccessAccepted != 2 || stats.SuccessWritten != 2 || stats.SuccessDropped != 0 || stats.SuccessBatchWrites != 1 ||
		stats.ConfiguredBatchSize != 2 || stats.LastSuccessBatchSize != 2 || stats.MaxSuccessBatchSize != 2 {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestRecorderWritesFailureAndDetailAtomicallyThroughStoreContract(t *testing.T) {
	store := &recordingStore{wrote: make(chan struct{}, 1)}
	recorder := NewRecorder(store, Config{FailureQueueSize: 2})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go recorder.Start(ctx)

	event := validEvent("event-failure", httpStatusInternalServerError)
	event.HasErrorDetail = true
	detail := &Detail{EventID: event.ID, OccurredAt: event.OccurredAt, WorkspaceID: event.WorkspaceID}
	recorder.Enqueue(event, detail)
	stats := waitForRecorderStats(t, recorder, "failure insert completion", func(stats Stats) bool {
		return stats.FailureWritten == 1 && stats.FailureInsertWrites == 1
	})

	_, failures := store.snapshot()
	if len(failures) != 1 || failures[0].event.ID != event.ID || failures[0].detail.EventID != event.ID {
		t.Fatalf("failure write = %#v", failures)
	}
	if stats.FailureAccepted != 1 || stats.FailureWritten != 1 || stats.FailureWriteFailures != 0 || stats.FailureInsertWrites != 1 {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestRecorderNotifiesOncePerDurableBatch(t *testing.T) {
	store := &recordingStore{wrote: make(chan struct{}, 1)}
	persisted := make(chan []Event, 1)
	recorder := NewRecorder(store, Config{BatchSize: 2, FlushInterval: time.Hour, OnPersistedBatch: func(_ context.Context, events []Event) {
		persisted <- append([]Event(nil), events...)
	}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go recorder.Start(ctx)

	recorder.Enqueue(validEvent("success-1", httpStatusOK), nil)
	recorder.Enqueue(validEvent("success-2", httpStatusOK), nil)

	select {
	case events := <-persisted:
		if len(events) != 2 || events[0].ID != "success-1" || events[1].ID != "success-2" {
			t.Fatalf("persisted batch = %#v", events)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for persisted batch callback")
	}
	stopRecorderAndWait(t, recorder)
	select {
	case events := <-persisted:
		t.Fatalf("received O(batch) duplicate callback: %#v", events)
	default:
	}
}

func TestRecorderDoesNotNotifyWhenPersistenceFails(t *testing.T) {
	store := &recordingStore{successErr: errors.New("down"), failureErr: errors.New("down"), wrote: make(chan struct{}, 2)}
	persisted := make(chan []Event, 2)
	recorder := NewRecorder(store, Config{BatchSize: 1, OnPersistedBatch: func(_ context.Context, events []Event) {
		persisted <- events
	}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go recorder.Start(ctx)

	recorder.Enqueue(validEvent("success", httpStatusOK), nil)
	recorder.Enqueue(validEvent("failure", httpStatusInternalServerError), nil)
	waitForRecorderStats(t, recorder, "failed persistence completion", func(stats Stats) bool {
		return stats.SuccessWriteFailures == 1 && stats.FailureWriteFailures == 1
	})
	stopRecorderAndWait(t, recorder)
	select {
	case events := <-persisted:
		t.Fatalf("notified unpersisted events %#v", events)
	default:
	}
}

type waitForInsertDeadlineStore struct{}

func (*waitForInsertDeadlineStore) InsertSuccessBatch(ctx context.Context, _ []Event) error {
	<-ctx.Done()
	return nil
}
func (*waitForInsertDeadlineStore) InsertFailure(context.Context, Event, *Detail) error { return nil }

func TestRecorderBatchCallbackUsesIndependentBoundedContext(t *testing.T) {
	type callbackState struct {
		err         error
		hasDeadline bool
	}
	callback := make(chan callbackState, 1)
	recorder := NewRecorder(&waitForInsertDeadlineStore{}, Config{
		BatchSize: 1, WriteTimeout: 5 * time.Millisecond, CallbackTimeout: 50 * time.Millisecond,
		OnPersistedBatch: func(ctx context.Context, _ []Event) {
			_, hasDeadline := ctx.Deadline()
			callback <- callbackState{err: ctx.Err(), hasDeadline: hasDeadline}
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go recorder.Start(ctx)
	recorder.Enqueue(validEvent("success", httpStatusOK), nil)

	select {
	case state := <-callback:
		if state.err != nil {
			t.Fatalf("callback inherited canceled insert context: %v", state.err)
		}
		if !state.hasDeadline {
			t.Fatal("callback context is not bounded")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for callback")
	}
}

func TestRecorderBatchCallbackBackpressureIsBoundedByItsContext(t *testing.T) {
	store := &recordingStore{wrote: make(chan struct{}, 2)}
	callbacks := make(chan string, 2)
	recorder := NewRecorder(store, Config{
		BatchSize: 1, CallbackTimeout: 10 * time.Millisecond,
		OnPersistedBatch: func(ctx context.Context, events []Event) {
			<-ctx.Done()
			callbacks <- events[0].ID
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go recorder.Start(ctx)
	recorder.Enqueue(validEvent("success-1", httpStatusOK), nil)
	recorder.Enqueue(validEvent("success-2", httpStatusOK), nil)

	want := map[string]bool{"success-1": true, "success-2": true}
	deadline := time.After(250 * time.Millisecond)
	for len(want) > 0 {
		select {
		case id := <-callbacks:
			delete(want, id)
		case <-deadline:
			t.Fatalf("callback backpressure stalled later batches: missing %v", want)
		}
	}
}

func TestRecorderFailureInsertInvokesOneSingleEventBatchCallback(t *testing.T) {
	store := &recordingStore{wrote: make(chan struct{}, 1)}
	callbacks := make(chan []Event, 1)
	recorder := NewRecorder(store, Config{OnPersistedBatch: func(_ context.Context, events []Event) {
		callbacks <- append([]Event(nil), events...)
	}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go recorder.Start(ctx)
	recorder.Enqueue(validEvent("failure", httpStatusInternalServerError), nil)

	select {
	case events := <-callbacks:
		if len(events) != 1 || events[0].ID != "failure" {
			t.Fatalf("failure callback batch = %#v", events)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for failure callback batch")
	}
}

func TestRecorderStatsHandlerExposesOperationalHealthWithoutPayloads(t *testing.T) {
	recorder := NewRecorder(&recordingStore{}, Config{NormalQueueSize: 1})
	recorder.Enqueue(validEvent("queued", httpStatusOK), nil)
	recorder.Enqueue(validEvent("dropped", httpStatusOK), nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/observability/request-events", nil)
	rec := httptest.NewRecorder()
	StatsHandler(recorder).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data Stats `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.NormalQueueDepth != 1 || body.Data.SuccessDropped != 1 {
		t.Fatalf("stats response = %#v", body.Data)
	}
	if strings.Contains(rec.Body.String(), "workspace-1") || strings.Contains(rec.Body.String(), "queued") {
		t.Fatalf("stats response leaked event data: %s", rec.Body.String())
	}
}

type orderedRecorderStore struct {
	writes chan string
}

func (s *orderedRecorderStore) InsertSuccessBatch(_ context.Context, events []Event) error {
	for _, event := range events {
		s.writes <- "success:" + event.ID
	}
	return nil
}

func (s *orderedRecorderStore) InsertFailure(_ context.Context, event Event, _ *Detail) error {
	s.writes <- "failure:" + event.ID
	return nil
}

func TestRecorderFailurePriorityDoesNotStarveQueuedSuccess(t *testing.T) {
	store := &orderedRecorderStore{writes: make(chan string, 32)}
	recorder := NewRecorder(store, Config{NormalQueueSize: 2, FailureQueueSize: 32, BatchSize: 1})
	for index := 0; index < 16; index++ {
		recorder.Enqueue(validEvent(fmt.Sprintf("failure-%d", index), httpStatusInternalServerError), nil)
	}
	recorder.Enqueue(validEvent("success", httpStatusOK), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go recorder.Start(ctx)

	successPosition := -1
	for index := 0; index < 9; index++ {
		select {
		case write := <-store.writes:
			if strings.HasPrefix(write, "success:") {
				successPosition = index
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for prioritized writes")
		}
	}
	if successPosition < 0 {
		t.Fatal("queued success was starved by sustained failure traffic")
	}
}

func TestRecorderFailurePriorityDeterministicallyFlushesAlreadyPendingSuccess(t *testing.T) {
	store := &orderedRecorderStore{writes: make(chan string, 32)}
	recorder := NewRecorder(store, Config{
		NormalQueueSize:  2,
		FailureQueueSize: 32,
		BatchSize:        100,
		FlushInterval:    time.Hour,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go recorder.Start(ctx)
	waitForRecorderStart(t, recorder)
	recorder.Enqueue(validEvent("pending-success", httpStatusOK), nil)
	deadline := time.Now().Add(time.Second)
	for recorder.Stats().NormalQueueDepth != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if stats := recorder.Stats(); stats.NormalQueueDepth != 0 || stats.SuccessWritten != 0 {
		t.Fatalf("success was not held pending before failure burst: %#v", stats)
	}

	for index := 0; index < 16; index++ {
		recorder.Enqueue(validEvent(fmt.Sprintf("failure-%d", index), httpStatusInternalServerError), nil)
	}
	successPosition := -1
	for index := 0; index < 9; index++ {
		select {
		case write := <-store.writes:
			if strings.HasPrefix(write, "success:") {
				successPosition = index
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for pending success flush")
		}
	}
	if successPosition < 0 {
		t.Fatal("already-pending success was not deterministically flushed after one failure burst")
	}
}

func TestRecorderSuccessQueueSaturationNeverBlocksAndCountsDrop(t *testing.T) {
	recorder := NewRecorder(&recordingStore{}, Config{NormalQueueSize: 1})
	recorder.Enqueue(validEvent("event-1", httpStatusOK), nil)

	done := make(chan struct{})
	go func() {
		recorder.Enqueue(validEvent("event-2", httpStatusOK), nil)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("saturated success queue blocked the customer path")
	}
	stats := recorder.Stats()
	if stats.NormalQueueDepth != 1 || stats.SuccessAccepted != 2 || stats.SuccessDropped != 1 {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestRecorderFailureQueueSaturationUsesDetachedDirectFallback(t *testing.T) {
	store := &recordingStore{wrote: make(chan struct{}, 1)}
	recorder := NewRecorder(store, Config{FailureQueueSize: 1, DirectFallbackLimit: time.Second})
	first := validEvent("event-1", httpStatusInternalServerError)
	first.HasErrorDetail = true
	recorder.Enqueue(first, &Detail{EventID: first.ID, OccurredAt: first.OccurredAt, WorkspaceID: first.WorkspaceID})

	second := validEvent("event-2", httpStatusInternalServerError)
	second.HasErrorDetail = true
	done := make(chan struct{})
	go func() {
		recorder.Enqueue(second, &Detail{EventID: second.ID, OccurredAt: second.OccurredAt, WorkspaceID: second.WorkspaceID})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("failure fallback blocked the customer path")
	}
	stats := waitForRecorderStats(t, recorder, "direct fallback completion", func(stats Stats) bool {
		return stats.FailureWritten == 1 && stats.FailureFallbackAttempts == 1
	})

	_, failures := store.snapshot()
	if len(failures) != 1 || failures[0].event.ID != second.ID {
		t.Fatalf("direct fallback writes = %#v", failures)
	}
	if stats.FailureAccepted != 2 || stats.FailureFallbackAttempts != 1 || stats.FailureWritten != 1 {
		t.Fatalf("stats = %#v", stats)
	}
}

type blockingFailureStore struct {
	started chan struct{}
	once    sync.Once
}

func (s *blockingFailureStore) InsertSuccessBatch(context.Context, []Event) error { return nil }

func (s *blockingFailureStore) InsertFailure(ctx context.Context, _ Event, _ *Detail) error {
	s.once.Do(func() { close(s.started) })
	<-ctx.Done()
	return ctx.Err()
}

func TestRecorderAccountsForFailureWhenFallbackCapacityIsExhausted(t *testing.T) {
	store := &blockingFailureStore{started: make(chan struct{})}
	recorder := NewRecorder(store, Config{
		FailureQueueSize:    1,
		FallbackWorkers:     1,
		DirectFallbackLimit: 100 * time.Millisecond,
	})
	enqueueFailure := func(id string) {
		event := validEvent(id, httpStatusInternalServerError)
		event.HasErrorDetail = true
		recorder.Enqueue(event, &Detail{EventID: id, OccurredAt: event.OccurredAt, WorkspaceID: event.WorkspaceID})
	}
	enqueueFailure("queued")
	enqueueFailure("fallback")
	select {
	case <-store.started:
	case <-time.After(time.Second):
		t.Fatal("direct fallback did not start")
	}

	done := make(chan struct{})
	go func() {
		enqueueFailure("capacity-exhausted")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("exhausted fallback capacity blocked the customer path")
	}
	stats := recorder.Stats()
	if stats.FailureAccepted != 3 || stats.FailureFallbackAttempts != 2 || stats.FailureWriteFailures != 1 {
		t.Fatalf("fallback capacity stats = %#v", stats)
	}
}

func TestRecorderDatabaseFailuresAreContainedAndObservable(t *testing.T) {
	store := &recordingStore{failureErr: errors.New("telemetry unavailable"), wrote: make(chan struct{}, 1)}
	recorder := NewRecorder(store, Config{FailureQueueSize: 1, WriteTimeout: time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go recorder.Start(ctx)
	event := validEvent("event-1", httpStatusInternalServerError)
	event.HasErrorDetail = true
	recorder.Enqueue(event, &Detail{EventID: event.ID, OccurredAt: event.OccurredAt, WorkspaceID: event.WorkspaceID})
	stats := waitForRecorderStats(t, recorder, "failed failure write completion", func(stats Stats) bool {
		return stats.FailureWriteFailures == 1
	})
	if stats.FailureWritten != 0 {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestRecorderStopRejectsLateAdmissionAndDrains(t *testing.T) {
	store := &recordingStore{wrote: make(chan struct{}, 2)}
	recorder := NewRecorder(store, Config{NormalQueueSize: 2, BatchSize: 10, FlushInterval: time.Hour, DrainTimeout: time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	go recorder.Start(ctx)
	waitForRecorderStart(t, recorder)
	recorder.Enqueue(validEvent("event-1", httpStatusOK), nil)
	recorder.Stop()
	recorder.Enqueue(validEvent("event-late", httpStatusOK), nil)

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	if err := recorder.Wait(waitCtx); err != nil {
		t.Fatal(err)
	}
	cancel()
	stats := recorder.Stats()
	if stats.SuccessWritten != 1 || stats.Abandoned != 1 {
		t.Fatalf("shutdown stats = %#v", stats)
	}
}

const (
	httpStatusOK                  = 200
	httpStatusCreated             = 201
	httpStatusInternalServerError = 500
)

func validEvent(id string, status int) Event {
	return Event{
		ID:           id,
		WorkspaceID:  "workspace-1",
		APIKeyID:     "api-key-1",
		OccurredAt:   time.Unix(100, 0).UTC(),
		Method:       "POST",
		RoutePattern: "/v1/test",
		StatusCode:   status,
		DurationMS:   5,
		Outcome:      NormalizeOutcome(status, ""),
	}
}

func waitForRecorderWrite(t *testing.T, wrote <-chan struct{}) {
	t.Helper()
	select {
	case <-wrote:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for recorder write")
	}
}

func waitForRecorderStart(t *testing.T, recorder *Recorder) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !recorder.started.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !recorder.started.Load() {
		t.Fatal("timed out waiting for recorder start")
	}
}

func stopRecorderAndWait(t *testing.T, recorder *Recorder) {
	t.Helper()
	recorder.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := recorder.Wait(ctx); err != nil {
		t.Fatalf("timed out waiting for recorder stop: %v", err)
	}
}

func waitForRecorderStats(t *testing.T, recorder *Recorder, description string, done func(Stats) bool) Stats {
	t.Helper()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		stats := recorder.Stats()
		if done(stats) {
			return stats
		}
		select {
		case <-ticker.C:
		case <-timer.C:
			t.Fatalf("timed out waiting for %s; stats=%#v", description, stats)
		}
	}
}
