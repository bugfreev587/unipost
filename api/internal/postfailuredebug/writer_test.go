package postfailuredebug

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

type recordingStore struct {
	mu      sync.Mutex
	details []StoredDetail
	err     error
	wrote   chan struct{}
}

func (s *recordingStore) UpsertPostFailureDebug(_ context.Context, detail StoredDetail) error {
	s.mu.Lock()
	s.details = append(s.details, detail)
	err := s.err
	s.mu.Unlock()
	if s.wrote != nil {
		select {
		case s.wrote <- struct{}{}:
		default:
		}
	}
	return err
}

func (s *recordingStore) latest() StoredDetail {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.details[len(s.details)-1]
}

func TestWriterNormalizesAndBoundsStoredDetail(t *testing.T) {
	store := &recordingStore{wrote: make(chan struct{}, 1)}
	writer := NewWriter(store, Config{QueueSize: 1, WriteTimeout: time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go writer.Start(ctx)

	raw := strings.Repeat("a", MaxDetailBytes-2) + "\x00\xfftail"
	writer.Enqueue(Detail{
		SocialPostResultID: "result-1",
		WorkspaceID:        "workspace-1",
		DebugText:          raw,
		CapturedAt:         time.Unix(123, 0).UTC(),
		RedactionVersion:   1,
	})

	select {
	case <-store.wrote:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for diagnostic write")
	}
	got := store.latest()
	if got.SocialPostResultID != "result-1" || got.WorkspaceID != "workspace-1" {
		t.Fatalf("identity changed: %#v", got)
	}
	if got.OriginalBytes != len(raw) || got.StoredBytes != len(got.DebugText) {
		t.Fatalf("size metadata = original %d stored %d, want %d/%d", got.OriginalBytes, got.StoredBytes, len(raw), len(got.DebugText))
	}
	if !got.Truncated || got.StoredBytes > MaxDetailBytes {
		t.Fatalf("bounded detail = truncated %v stored %d", got.Truncated, got.StoredBytes)
	}
	if !utf8.ValidString(got.DebugText) || strings.ContainsRune(got.DebugText, '\x00') {
		t.Fatal("stored detail must be valid UTF-8 without NUL")
	}
}

func TestWriterQueueSaturationNeverBlocksCaller(t *testing.T) {
	writer := NewWriter(&recordingStore{}, Config{QueueSize: 1})
	writer.Enqueue(Detail{SocialPostResultID: "result-1", WorkspaceID: "workspace-1", DebugText: "first"})

	done := make(chan struct{})
	go func() {
		writer.Enqueue(Detail{SocialPostResultID: "result-2", WorkspaceID: "workspace-1", DebugText: "second"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("saturated diagnostic queue blocked the publishing caller")
	}
	stats := writer.Stats()
	if stats.QueueDepth != 1 || stats.Dropped != 1 {
		t.Fatalf("stats = %#v, want depth=1 dropped=1", stats)
	}
}

func TestWriterDatabaseFailureIsCountedAndContained(t *testing.T) {
	store := &recordingStore{err: errors.New("database unavailable"), wrote: make(chan struct{}, 1)}
	writer := NewWriter(store, Config{QueueSize: 1, WriteTimeout: time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go writer.Start(ctx)
	writer.Enqueue(Detail{SocialPostResultID: "result-1", WorkspaceID: "workspace-1", DebugText: "safe"})

	select {
	case <-store.wrote:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for failed diagnostic write")
	}
	deadline := time.Now().Add(time.Second)
	for writer.Stats().Failures != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if stats := writer.Stats(); stats.Failures != 1 || stats.Writes != 0 {
		t.Fatalf("stats = %#v, want one contained failure and no successful write", stats)
	}
}

func TestWriterRejectsIncompleteIdentityWithoutQueueing(t *testing.T) {
	writer := NewWriter(&recordingStore{}, Config{QueueSize: 1})
	writer.Enqueue(Detail{WorkspaceID: "workspace-1", DebugText: "missing result"})
	writer.Enqueue(Detail{SocialPostResultID: "result-1", DebugText: "missing workspace"})
	writer.Enqueue(Detail{SocialPostResultID: "result-1", WorkspaceID: "workspace-1"})
	if stats := writer.Stats(); stats.QueueDepth != 0 || stats.Dropped != 0 {
		t.Fatalf("invalid details changed queue stats: %#v", stats)
	}
}

type blockingStore struct {
	started chan struct{}
	once    sync.Once
}

func (s *blockingStore) UpsertPostFailureDebug(ctx context.Context, _ StoredDetail) error {
	s.once.Do(func() { close(s.started) })
	<-ctx.Done()
	return ctx.Err()
}

func TestWriterCancellationHasBoundedDrainAndAccountsForUnwrittenDetails(t *testing.T) {
	store := &blockingStore{started: make(chan struct{})}
	writer := NewWriter(store, Config{
		QueueSize:    4,
		WriteTimeout: time.Second,
		DrainTimeout: 25 * time.Millisecond,
	})
	for i := 0; i < 3; i++ {
		writer.Enqueue(Detail{
			SocialPostResultID: "result-" + string(rune('1'+i)),
			WorkspaceID:        "workspace-1",
			DebugText:          "diagnostic",
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	go writer.Start(ctx)
	select {
	case <-store.started:
	case <-time.After(time.Second):
		t.Fatal("writer did not begin the first write")
	}
	cancel()

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer waitCancel()
	if err := writer.Wait(waitCtx); err != nil {
		t.Fatalf("bounded writer shutdown: %v", err)
	}
	stats := writer.Stats()
	if stats.Failures < 1 || stats.Abandoned+stats.Failures != 3 {
		t.Fatalf("shutdown stats = %#v, want all three details explicitly failed or abandoned", stats)
	}
}

func TestWriterWaitBeforeStartReturnsImmediately(t *testing.T) {
	writer := NewWriter(&recordingStore{}, Config{})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := writer.Wait(ctx); err != nil {
		t.Fatalf("wait before start: %v", err)
	}
}

func TestWriterStopRejectsLateEnqueueAndCountsItAsAbandoned(t *testing.T) {
	writer := NewWriter(&recordingStore{}, Config{QueueSize: 1})
	writer.Stop()
	writer.Enqueue(Detail{
		SocialPostResultID: "late-result",
		WorkspaceID:        "workspace-1",
		DebugText:          "late diagnostic",
	})
	stats := writer.Stats()
	if stats.QueueDepth != 0 || stats.Abandoned != 1 {
		t.Fatalf("late enqueue stats = %#v, want queue_depth=0 abandoned=1", stats)
	}
}
