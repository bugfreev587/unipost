package requesteventpartitions

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

type scriptedMaintainer struct {
	mu           sync.Mutex
	ensureCalls  int
	inspectCalls int
	ensure       func(context.Context, []Week) error
	inspect      func(context.Context, time.Time, int) (Inspection, error)
}

func (m *scriptedMaintainer) Ensure(ctx context.Context, weeks []Week) error {
	m.mu.Lock()
	m.ensureCalls++
	fn := m.ensure
	m.mu.Unlock()
	if fn == nil {
		return nil
	}
	return fn(ctx, weeks)
}

func (m *scriptedMaintainer) Inspect(ctx context.Context, now time.Time, minimum int) (Inspection, error) {
	m.mu.Lock()
	m.inspectCalls++
	fn := m.inspect
	m.mu.Unlock()
	if fn == nil {
		return Inspection{InspectedAt: now, CoverageDays: 60, Ready: true, Reasons: []string{}}, nil
	}
	return fn(ctx, now, minimum)
}

func (m *scriptedMaintainer) calls() (int, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ensureCalls, m.inspectCalls
}

type fakeTicker struct {
	channel chan time.Time
	stopped chan struct{}
	once    sync.Once
}

func (t *fakeTicker) Chan() <-chan time.Time { return t.channel }
func (t *fakeTicker) Stop() {
	t.once.Do(func() { close(t.stopped) })
}

type fakeWorkerClock struct {
	mu       sync.Mutex
	now      time.Time
	duration time.Duration
	ticker   *fakeTicker
}

func newFakeWorkerClock(now time.Time) *fakeWorkerClock {
	return &fakeWorkerClock{
		now: now,
		ticker: &fakeTicker{
			channel: make(chan time.Time, 4),
			stopped: make(chan struct{}),
		},
	}
}

func (c *fakeWorkerClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeWorkerClock) SetNow(now time.Time) {
	c.mu.Lock()
	c.now = now
	c.mu.Unlock()
}

func (c *fakeWorkerClock) NewTicker(duration time.Duration) workerTicker {
	c.mu.Lock()
	c.duration = duration
	c.mu.Unlock()
	return c.ticker
}

func (c *fakeWorkerClock) tickerDuration() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.duration
}

func testWorker(maintainer Maintainer, clock workerClock) *Worker {
	worker := newWorker(maintainer, clock)
	worker.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	return worker
}

func waitForEnsureCalls(t *testing.T, maintainer *scriptedMaintainer, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		got, _ := maintainer.calls()
		if got >= want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	got, _ := maintainer.calls()
	t.Fatalf("ensure calls = %d, want at least %d", got, want)
}

func TestWorkerRunsImmediatelyAndEverySixHours(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	clock := newFakeWorkerClock(now)
	maintainer := &scriptedMaintainer{}
	worker := testWorker(maintainer, clock)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Start(ctx)
	}()

	waitForEnsureCalls(t, maintainer, 1)
	if clock.tickerDuration() != 6*time.Hour {
		t.Fatalf("ticker duration = %v, want 6h", clock.tickerDuration())
	}
	clock.SetNow(now.Add(6 * time.Hour))
	clock.ticker.channel <- clock.Now()
	waitForEnsureCalls(t, maintainer, 2)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
}

func TestWorkerBoundsEachAttemptAtThirtySeconds(t *testing.T) {
	clock := newFakeWorkerClock(time.Now().UTC())
	deadlineDelta := time.Duration(0)
	maintainer := &scriptedMaintainer{
		ensure: func(ctx context.Context, _ []Week) error {
			deadline, ok := ctx.Deadline()
			if !ok {
				return errors.New("attempt context has no deadline")
			}
			deadlineDelta = time.Until(deadline)
			return errors.New("stop after observing deadline")
		},
	}
	testWorker(maintainer, clock).runOnce(context.Background())
	if deadlineDelta < 29*time.Second || deadlineDelta > 30*time.Second {
		t.Fatalf("attempt deadline delta = %v, want approximately 30s", deadlineDelta)
	}
}

func TestWorkerRetainsLastSuccessAfterFailure(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	clock := newFakeWorkerClock(now)
	fail := false
	maintainer := &scriptedMaintainer{
		ensure: func(context.Context, []Week) error {
			if fail {
				return errors.New("database unavailable")
			}
			return nil
		},
	}
	worker := testWorker(maintainer, clock)
	worker.runOnce(context.Background())
	first := worker.Stats()
	if !first.LastSuccessAt.Equal(now) {
		t.Fatalf("first success = %v, want %v", first.LastSuccessAt, now)
	}

	fail = true
	clock.SetNow(now.Add(6 * time.Hour))
	worker.runOnce(context.Background())
	second := worker.Stats()
	if !second.LastSuccessAt.Equal(first.LastSuccessAt) {
		t.Fatalf("last success changed after failure: %v -> %v", first.LastSuccessAt, second.LastSuccessAt)
	}
	if second.FailureCount != 1 || second.LastErrorClass != ErrorClassMaintenanceFailed {
		t.Fatalf("failure stats = %#v", second)
	}
}

func TestWorkerStopsOnContextCancellation(t *testing.T) {
	clock := newFakeWorkerClock(time.Now().UTC())
	worker := testWorker(&scriptedMaintainer{}, clock)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Start(ctx)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
	select {
	case <-clock.ticker.stopped:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop its ticker")
	}
}

func TestWorkerClassifiesDefaultOccupancy(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	clock := newFakeWorkerClock(now)
	week, err := weekForStart(time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	maintainer := &scriptedMaintainer{
		ensure: func(context.Context, []Week) error {
			return &DefaultPartitionOccupiedError{Week: week, EventRows: 2, DetailRows: 1}
		},
	}
	worker := testWorker(maintainer, clock)
	worker.runOnce(context.Background())
	stats := worker.Stats()
	if stats.FailureCount != 1 || stats.LastErrorClass != ErrorClassDefaultOccupied {
		t.Fatalf("default occupancy stats = %#v", stats)
	}
}
