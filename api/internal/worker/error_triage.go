package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/errortriage"
)

type ErrorTriageWorker struct {
	service errorTriageRunner
}

type errorTriageRunner interface {
	Run(context.Context, errortriage.RunOptions) (errortriage.RunRecord, error)
}

func NewErrorTriageWorker(service *errortriage.Service) *ErrorTriageWorker {
	return &ErrorTriageWorker{service: service}
}

func (w *ErrorTriageWorker) Start(ctx context.Context) {
	if w == nil || w.service == nil {
		return
	}
	slog.Info("error triage worker started")
	w.runScheduled(ctx, time.Now())
	for {
		delay, err := durationUntilNextPTMidnight(time.Now())
		if err != nil {
			slog.Error("error triage worker: failed to compute PT midnight", "error", err)
			delay = time.Hour
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			slog.Info("error triage worker stopped")
			return
		case <-timer.C:
			w.runScheduled(ctx, time.Now())
		}
	}
}

func (w *ErrorTriageWorker) runScheduled(ctx context.Context, now time.Time) {
	start, end, err := errortriage.PreviousPTDayWindow(now)
	if err != nil {
		slog.Error("error triage worker: failed to compute daily window", "error", err)
		return
	}
	run, err := w.service.Run(ctx, errortriage.RunOptions{
		RunType:     errortriage.RunTypeScheduled,
		WindowStart: start,
		WindowEnd:   end,
	})
	if err != nil {
		slog.Error("error triage worker: scheduled run failed", "window_start", start, "window_end", end, "error", err)
		return
	}
	if run.ID == "" {
		slog.Info("error triage worker: scheduled run skipped", "window_start", start, "window_end", end)
		return
	}
	slog.Info("error triage worker: scheduled run finished", "run_id", run.ID, "window_start", start, "window_end", end, "failures", run.FailuresAnalyzed)
}

func durationUntilNextPTMidnight(now time.Time) (time.Duration, error) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		return 0, err
	}
	ptNow := now.In(loc)
	next := time.Date(ptNow.Year(), ptNow.Month(), ptNow.Day()+1, 0, 0, 0, 0, loc)
	return next.Sub(ptNow), nil
}
