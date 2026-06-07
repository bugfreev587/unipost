package worker

import (
	"testing"
	"time"
)

func TestDurationUntilNextPTMidnightHandlesDSTSpringForward(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 8, 1, 30, 0, 0, loc)
	got, err := durationUntilNextPTMidnight(now)
	if err != nil {
		t.Fatalf("durationUntilNextPTMidnight returned error: %v", err)
	}
	if got != 21*time.Hour+30*time.Minute {
		t.Fatalf("duration = %s, want 21h30m", got)
	}
}

func TestDurationUntilNextPTMidnightHandlesDSTFallBack(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 1, 0, 30, 0, 0, loc)
	got, err := durationUntilNextPTMidnight(now)
	if err != nil {
		t.Fatalf("durationUntilNextPTMidnight returned error: %v", err)
	}
	if got != 24*time.Hour+30*time.Minute {
		t.Fatalf("duration = %s, want 24h30m", got)
	}
}
