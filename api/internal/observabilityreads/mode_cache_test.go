package observabilityreads

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/featureflags"
)

type controllableReadModeSource struct {
	mu      sync.Mutex
	enabled bool
	err     error
	calls   int
	called  chan struct{}
}

type blockingReadModeSource struct {
	mu      sync.Mutex
	block   bool
	calls   int
	entered chan struct{}
}

func (s *blockingReadModeSource) Evaluate(ctx context.Context) (bool, error) {
	s.mu.Lock()
	s.calls++
	block := s.block
	s.mu.Unlock()
	if !block {
		return true, nil
	}
	select {
	case s.entered <- struct{}{}:
	default:
	}
	<-ctx.Done()
	// Simulate a source that unblocks on cancellation but forgets to return
	// ctx.Err. The cache must still inspect its own evaluation context.
	return true, nil
}

func (s *blockingReadModeSource) setBlocking(block bool) {
	s.mu.Lock()
	s.block = block
	s.mu.Unlock()
}

func (s *blockingReadModeSource) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *controllableReadModeSource) Evaluate(context.Context) (bool, error) {
	s.mu.Lock()
	s.calls++
	enabled, err := s.enabled, s.err
	s.mu.Unlock()
	if s.called != nil {
		select {
		case s.called <- struct{}{}:
		default:
		}
	}
	return enabled, err
}

func (s *controllableReadModeSource) set(enabled bool, err error) {
	s.mu.Lock()
	s.enabled, s.err = enabled, err
	s.mu.Unlock()
}

func (s *controllableReadModeSource) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func TestCachedReadSelectorStartsLegacyAndRefreshFailureFailsClosed(t *testing.T) {
	source := &controllableReadModeSource{enabled: true}
	cache := NewCachedReadSelector(source, CachedReadSelectorConfig{})

	if cache.UseV2(context.Background()) {
		t.Fatal("cache selected v2 before its first successful refresh")
	}
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !cache.UseV2(context.Background()) {
		t.Fatal("successful ON refresh did not select v2")
	}

	source.set(true, errors.New("flag store unavailable"))
	if err := cache.Refresh(context.Background()); err == nil {
		t.Fatal("refresh error = nil, want source error")
	}
	if cache.UseV2(context.Background()) {
		t.Fatal("refresh error kept stale v2 mode; want fail-closed legacy")
	}
	callsBeforeReads := source.callCount()
	for range 100 {
		_ = cache.UseV2(context.Background())
	}
	if got := source.callCount(); got != callsBeforeReads {
		t.Fatalf("source calls after 100 cached reads = %d, want %d", got, callsBeforeReads)
	}
}

func TestCachedReadSelectorPeriodicLifecycleStopsWithoutFurtherRefreshes(t *testing.T) {
	source := &controllableReadModeSource{enabled: true, called: make(chan struct{}, 8)}
	cache := NewCachedReadSelector(source, CachedReadSelectorConfig{RefreshInterval: time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cache.Start(ctx)

	waitForModeRefresh(t, source.called)
	waitForCachedMode(t, cache, true)
	source.set(false, nil)
	waitForModeRefresh(t, source.called)
	waitForCachedMode(t, cache, false)

	cache.Stop()
	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	if err := cache.Wait(waitCtx); err != nil {
		t.Fatal(err)
	}
	callsAfterStop := source.callCount()
	if err := cache.Wait(waitCtx); err != nil {
		t.Fatal(err)
	}
	if got := source.callCount(); got != callsAfterStop {
		t.Fatalf("refresh calls after Wait = %d, want %d", got, callsAfterStop)
	}
}

func TestCachedReadSelectorDefaultsBoundWorstCaseFallbackBelowTwoSeconds(t *testing.T) {
	cache := NewCachedReadSelector(&controllableReadModeSource{}, CachedReadSelectorConfig{})
	if cache.refreshInterval != time.Second {
		t.Fatalf("refresh interval = %v, want 1s", cache.refreshInterval)
	}
	if cache.evaluationTimeout != 500*time.Millisecond {
		t.Fatalf("evaluation timeout = %v, want 500ms", cache.evaluationTimeout)
	}
	if cache.refreshInterval+cache.evaluationTimeout >= 2*time.Second {
		t.Fatalf("worst-case fallback = %v, want less than 2s", cache.refreshInterval+cache.evaluationTimeout)
	}
}

func TestCachedReadSelectorRefreshTimeoutReplacesStaleOnWithLegacy(t *testing.T) {
	source := &blockingReadModeSource{entered: make(chan struct{}, 1)}
	cache := NewCachedReadSelector(source, CachedReadSelectorConfig{EvaluationTimeout: 20 * time.Millisecond})
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !cache.UseV2(context.Background()) {
		t.Fatal("seed refresh did not select v2")
	}
	source.setBlocking(true)

	result := make(chan error, 1)
	go func() { result <- cache.Refresh(context.Background()) }()
	waitForModeRefresh(t, source.entered)
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("refresh error = %v, want context deadline exceeded", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("blocked refresh exceeded its evaluation timeout")
	}
	if cache.UseV2(context.Background()) {
		t.Fatal("timed-out refresh kept stale v2 mode")
	}
	if got := source.callCount(); got != 2 {
		t.Fatalf("source calls = %d, want seed + one timed-out refresh", got)
	}
}

func TestCachedReadSelectorStopCancelsInFlightRefreshAndWaitsCleanly(t *testing.T) {
	source := &blockingReadModeSource{entered: make(chan struct{}, 1)}
	cache := NewCachedReadSelector(source, CachedReadSelectorConfig{
		RefreshInterval:   time.Hour,
		EvaluationTimeout: time.Hour,
	})
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	source.setBlocking(true)
	cache.Start(context.Background())
	waitForModeRefresh(t, source.entered)

	cache.Stop()
	waitCtx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	if err := cache.Wait(waitCtx); err != nil {
		t.Fatalf("Wait after Stop = %v", err)
	}
	if cache.UseV2(context.Background()) {
		t.Fatal("Stop left stale v2 mode after canceling in-flight refresh")
	}
	if got := source.callCount(); got != 2 {
		t.Fatalf("source calls = %d, want seed + one canceled refresh", got)
	}
}

func TestCachedReadSelectorActivationLockRefreshDoesNotQueryStore(t *testing.T) {
	store := &activationLockedStore{}
	source := NewReadSelector(featureflags.NewEvaluator(store, nil), slog.Default())
	cache := NewCachedReadSelector(source, CachedReadSelectorConfig{})

	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cache.UseV2(context.Background()) {
		t.Fatal("activation-locked cache selected v2")
	}
	if store.globalCalls != 0 {
		t.Fatalf("GlobalEnabled calls = %d, want 0", store.globalCalls)
	}
}

func waitForModeRefresh(t *testing.T, called <-chan struct{}) {
	t.Helper()
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for mode refresh")
	}
}

func waitForCachedMode(t *testing.T, cache *CachedReadSelector, want bool) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if cache.UseV2(context.Background()) == want {
			return
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatalf("timed out waiting for cached mode %v", want)
		}
	}
}
