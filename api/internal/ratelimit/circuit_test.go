package ratelimit

import (
	"errors"
	"testing"
	"time"
)

func TestBreaker_TripsAfterThreshold(t *testing.T) {
	b := NewBreaker("test")
	b.failureThreshold = 3
	b.failureWindow = time.Second

	for i := 0; i < 3; i++ {
		if err := b.BeforeCall(); err != nil {
			t.Fatalf("call %d: BeforeCall returned err while closed: %v", i, err)
		}
		b.AfterCall(errors.New("boom"))
	}

	// 3rd failure should have opened the breaker.
	if got := b.State(); got != circuitOpen {
		t.Fatalf("State after 3 failures = %v, want circuitOpen", got)
	}
	if err := b.BeforeCall(); err != ErrCircuitOpen {
		t.Fatalf("BeforeCall while open returned %v, want ErrCircuitOpen", err)
	}
}

func TestBreaker_ClosesOnHalfOpenSuccess(t *testing.T) {
	b := NewBreaker("test")
	b.failureThreshold = 1
	b.failureWindow = time.Second
	b.openDuration = 10 * time.Millisecond

	// Trip the breaker.
	_ = b.BeforeCall()
	b.AfterCall(errors.New("boom"))
	if got := b.State(); got != circuitOpen {
		t.Fatalf("State after trip = %v, want circuitOpen", got)
	}

	// Wait out the open window so the next BeforeCall promotes the
	// circuit to half-open and lets the canary through.
	time.Sleep(20 * time.Millisecond)
	if err := b.BeforeCall(); err != nil {
		t.Fatalf("canary BeforeCall returned %v, want nil", err)
	}
	b.AfterCall(nil)

	if got := b.State(); got != circuitClosed {
		t.Fatalf("State after canary success = %v, want circuitClosed", got)
	}
}

func TestBreaker_ReopensOnHalfOpenFailure(t *testing.T) {
	b := NewBreaker("test")
	b.failureThreshold = 1
	b.failureWindow = time.Second
	b.openDuration = 10 * time.Millisecond

	_ = b.BeforeCall()
	b.AfterCall(errors.New("boom"))
	time.Sleep(20 * time.Millisecond)
	if err := b.BeforeCall(); err != nil {
		t.Fatalf("canary BeforeCall returned %v, want nil", err)
	}
	b.AfterCall(errors.New("still down"))

	if got := b.State(); got != circuitOpen {
		t.Fatalf("State after canary failure = %v, want circuitOpen", got)
	}
}

func TestBreaker_FailuresOutsideWindowDoNotAccumulate(t *testing.T) {
	b := NewBreaker("test")
	b.failureThreshold = 3
	b.failureWindow = 50 * time.Millisecond

	// Two failures, then wait past the window, then one more.
	for i := 0; i < 2; i++ {
		_ = b.BeforeCall()
		b.AfterCall(errors.New("boom"))
	}
	time.Sleep(80 * time.Millisecond)
	_ = b.BeforeCall()
	b.AfterCall(errors.New("boom"))

	if got := b.State(); got != circuitClosed {
		t.Fatalf("State after stale failures + 1 fresh = %v, want circuitClosed", got)
	}
}
