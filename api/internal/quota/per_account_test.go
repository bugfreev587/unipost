package quota

import (
	"sync"
	"testing"
)

// TestPerAccountTracker_NilLimit_AlwaysAllows — when the project
// has no per_account_monthly_limit configured, the tracker is the
// "unlimited" zero value and Allow always returns true. Critical
// regression guard: any change that flips this behavior would
// break every project that hasn't opted in to the cap.
func TestPerAccountTracker_NilLimit_AlwaysAllows(t *testing.T) {
	tr := &PerAccountTracker{} // limit nil → unlimited
	for i := 0; i < 1000; i++ {
		if !tr.Allow("acct-1") {
			t.Fatalf("unlimited tracker denied at iter %d", i)
		}
	}
}

// TestPerAccountTracker_DecrementsAndExhausts — with a cap of 3,
// the first 3 dispatches succeed and the 4th is denied. Verifies
// the basic count-down behavior that publishOne relies on.
func TestPerAccountTracker_DecrementsAndExhausts(t *testing.T) {
	cap := 3
	tr := &PerAccountTracker{
		limit:     &cap,
		remaining: map[string]int{"acct-1": 3},
	}
	for i := 0; i < 3; i++ {
		if !tr.Allow("acct-1") {
			t.Errorf("dispatch %d should have been allowed", i)
		}
	}
	if tr.Allow("acct-1") {
		t.Error("4th dispatch should have been denied")
	}
	if tr.Allow("acct-1") {
		t.Error("5th dispatch should still be denied")
	}
}

// TestPerAccountTracker_PerAccountIndependent — exhausting account
// A's budget must not affect account B. Catches a class of bug
// where a shared counter or wrong key would couple the two.
func TestPerAccountTracker_PerAccountIndependent(t *testing.T) {
	cap := 2
	tr := &PerAccountTracker{
		limit:     &cap,
		remaining: map[string]int{"a": 2, "b": 2},
	}
	tr.Allow("a")
	tr.Allow("a")
	if tr.Allow("a") {
		t.Error("a should be exhausted")
	}
	if !tr.Allow("b") {
		t.Error("b should still have budget")
	}
}

// TestPerAccountTracker_AlreadyOverCap — if the snapshot loaded
// a count higher than the cap (e.g. the cap was lowered after the
// month was already partly used), the remaining clamps to zero
// rather than going negative, and the next Allow denies.
func TestPerAccountTracker_AlreadyOverCap(t *testing.T) {
	cap := 5
	tr := &PerAccountTracker{
		limit:     &cap,
		remaining: map[string]int{"a": 0}, // simulate clamp from over-cap
	}
	if tr.Allow("a") {
		t.Error("should deny when remaining is already 0")
	}
}

// TestPerAccountTracker_UnknownAccountFreshBudget — an account
// that wasn't in the snapshot (defensive corner case) gets a
// fresh budget rather than crashing or being silently rejected.
// Reasoning is documented at the Allow() ok==false branch.
func TestPerAccountTracker_UnknownAccountFreshBudget(t *testing.T) {
	cap := 2
	tr := &PerAccountTracker{
		limit:     &cap,
		remaining: map[string]int{},
	}
	if !tr.Allow("surprise-account") {
		t.Error("first dispatch on unsnapshot account should be allowed")
	}
	if !tr.Allow("surprise-account") {
		t.Error("second dispatch should be allowed (cap is 2)")
	}
	if tr.Allow("surprise-account") {
		t.Error("third dispatch should exhaust the fresh budget")
	}
}

// TestPerAccountTracker_ConcurrentAllow — N goroutines hammer
// Allow concurrently. The total number of "true" returns must
// equal the cap exactly (no over-publish under contention). This
// is the test that proves the mutex actually works — race-detector
// runs (`go test -race`) catch the alternative.
func TestPerAccountTracker_ConcurrentAllow(t *testing.T) {
	cap := 50
	tr := &PerAccountTracker{
		limit:     &cap,
		remaining: map[string]int{"a": 50},
	}
	var allowed int64
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tr.Allow("a") {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed != int64(cap) {
		t.Errorf("expected exactly %d allows under contention, got %d", cap, allowed)
	}
}

// TestPerAccountTracker_ZeroCap — a cap of 0 is a valid emergency
// lockout. Every Allow must deny; this is the "this account is
// frozen for the rest of the month" case the PRD calls out.
func TestPerAccountTracker_ZeroCap(t *testing.T) {
	cap := 0
	tr := &PerAccountTracker{
		limit:     &cap,
		remaining: map[string]int{"a": 0},
	}
	if tr.Allow("a") {
		t.Error("zero cap must deny")
	}
}
