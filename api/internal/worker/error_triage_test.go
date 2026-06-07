package worker

import (
	"context"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/errortriage"
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

func TestRunScheduledUsesPreviousPTDayWindow(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeErrorTriageRunner{}
	worker := &ErrorTriageWorker{service: runner}

	worker.runScheduled(context.Background(), time.Date(2026, 6, 7, 12, 30, 0, 0, loc))

	if len(runner.options) != 1 {
		t.Fatalf("scheduled runs = %d, want 1", len(runner.options))
	}
	opts := runner.options[0]
	if opts.RunType != errortriage.RunTypeScheduled {
		t.Fatalf("run type = %q, want scheduled", opts.RunType)
	}
	if got, want := opts.WindowStart.In(loc).Format(time.RFC3339), "2026-06-06T00:00:00-07:00"; got != want {
		t.Fatalf("window start = %s, want %s", got, want)
	}
	if got, want := opts.WindowEnd.In(loc).Format(time.RFC3339), "2026-06-07T00:00:00-07:00"; got != want {
		t.Fatalf("window end = %s, want %s", got, want)
	}
}

type fakeErrorTriageRunner struct {
	options []errortriage.RunOptions
}

func (r *fakeErrorTriageRunner) Run(ctx context.Context, opts errortriage.RunOptions) (errortriage.RunRecord, error) {
	r.options = append(r.options, opts)
	return errortriage.RunRecord{ID: "run_1", RunType: opts.RunType, Status: errortriage.RunStatusCompleted, WindowStart: opts.WindowStart, WindowEnd: opts.WindowEnd}, nil
}
