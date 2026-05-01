package quota

import (
	"sync"
	"testing"
)

// TestCapForPlatform_KnownAndDefault — the cap table covers every
// platform we ship publishing for. Unknown platforms fall back to
// PerPlatformDailyCapDefault rather than 0 (which would mean "block
// every dispatch") or unbounded (which would mean "no safety belt
// for the new platform").
func TestCapForPlatform_KnownAndDefault(t *testing.T) {
	cases := map[string]int{
		"twitter":   20,
		"instagram": 100,
		"facebook":  100,
		"threads":   250,
		"bluesky":   50,
		"linkedin":  50,
		"tiktok":    50,
		"youtube":   50,
		"pinterest": 50,
	}
	for plat, want := range cases {
		if got := CapForPlatform(plat); got != want {
			t.Errorf("CapForPlatform(%q) = %d, want %d", plat, got, want)
		}
	}
	if got := CapForPlatform("mastodon"); got != PerPlatformDailyCapDefault {
		t.Errorf("CapForPlatform(unknown) = %d, want %d", got, PerPlatformDailyCapDefault)
	}
	if got := CapForPlatform(""); got != PerPlatformDailyCapDefault {
		t.Errorf("CapForPlatform(empty) = %d, want %d", got, PerPlatformDailyCapDefault)
	}
}

// TestPerPlatformDailyTracker_NilReceiver_AlwaysAllows — Allow on a
// nil tracker is the test/legacy fallback path. Critical regression
// guard: any change that flips this would crash the test suite for
// every handler test that doesn't bother to wire a tracker.
func TestPerPlatformDailyTracker_NilReceiver_AlwaysAllows(t *testing.T) {
	var tr *PerPlatformDailyTracker
	for i := 0; i < 1000; i++ {
		if !tr.Allow("acct-1", "twitter") {
			t.Fatalf("nil tracker denied at iter %d", i)
		}
	}
}

// TestPerPlatformDailyTracker_DecrementsAndExhausts — with a snapshot
// of 3 remaining for one account, the first 3 dispatches succeed and
// the 4th is denied. Mirrors per-account-tracker's contract so the
// publish loop's "tracker.Allow → return error" branch behaves the
// same shape across the two safety belts.
func TestPerPlatformDailyTracker_DecrementsAndExhausts(t *testing.T) {
	tr := &PerPlatformDailyTracker{
		remaining: map[string]int{"acct-1": 3},
		cap:       map[string]int{"acct-1": 3},
	}
	for i := 0; i < 3; i++ {
		if !tr.Allow("acct-1", "twitter") {
			t.Errorf("dispatch %d should have been allowed", i)
		}
	}
	if tr.Allow("acct-1", "twitter") {
		t.Error("4th dispatch should have been denied")
	}
	if tr.Allow("acct-1", "twitter") {
		t.Error("5th dispatch should still be denied")
	}
}

// TestPerPlatformDailyTracker_PerAccountIndependent — exhausting
// account A's daily budget must not affect account B even when both
// are on the same platform. Catches a class of bug where a shared
// counter or wrong key would couple the two together.
func TestPerPlatformDailyTracker_PerAccountIndependent(t *testing.T) {
	tr := &PerPlatformDailyTracker{
		remaining: map[string]int{"a": 2, "b": 2},
		cap:       map[string]int{"a": 2, "b": 2},
	}
	tr.Allow("a", "twitter")
	tr.Allow("a", "twitter")
	if tr.Allow("a", "twitter") {
		t.Error("a should be exhausted")
	}
	if !tr.Allow("b", "twitter") {
		t.Error("b should still have budget")
	}
}

// TestPerPlatformDailyTracker_AlreadyOverCap — if today's count was
// already above the cap when the snapshot loaded (e.g. between two
// requests on different replicas), the remaining clamps to zero
// rather than going negative, and the next Allow denies. Mirrors
// PerAccountTracker's clamp semantics so the two safety belts behave
// identically when the DB count outpaces the in-memory budget.
func TestPerPlatformDailyTracker_AlreadyOverCap(t *testing.T) {
	tr := &PerPlatformDailyTracker{
		remaining: map[string]int{"a": 0}, // simulate clamp from over-cap
		cap:       map[string]int{"a": 20},
	}
	if tr.Allow("a", "twitter") {
		t.Error("should deny when remaining is already 0")
	}
}

// TestPerPlatformDailyTracker_UnknownAccountFreshBudget — an account
// that wasn't in the snapshot (defensive corner case) gets a fresh
// budget sized to the named platform rather than crashing or being
// silently rejected.
func TestPerPlatformDailyTracker_UnknownAccountFreshBudget(t *testing.T) {
	tr := &PerPlatformDailyTracker{
		remaining: map[string]int{},
		cap:       map[string]int{},
	}
	// twitter cap is 20, so this account should accept exactly 20
	// dispatches before exhausting.
	allowed := 0
	for i := 0; i < 25; i++ {
		if tr.Allow("surprise", "twitter") {
			allowed++
		}
	}
	if allowed != 20 {
		t.Errorf("fresh-budget account should accept exactly 20 twitter dispatches, got %d", allowed)
	}
}

// TestPerPlatformDailyTracker_ConcurrentAllow — N goroutines hammer
// Allow concurrently against a single account. The total number of
// "true" returns must equal the cap exactly (no over-publish under
// contention). This is the test that proves the mutex actually works
// — race-detector runs (`go test -race`) catch the alternative.
func TestPerPlatformDailyTracker_ConcurrentAllow(t *testing.T) {
	cap := 50
	tr := &PerPlatformDailyTracker{
		remaining: map[string]int{"a": cap},
		cap:       map[string]int{"a": cap},
	}
	var allowed int64
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tr.Allow("a", "linkedin") {
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
