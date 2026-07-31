package requesteventpartitions

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

const (
	workerCadence  = 6 * time.Hour
	attemptTimeout = 30 * time.Second

	ErrorClassMaintenanceFailed = "maintenance_failed"
	ErrorClassDefaultOccupied   = "default_partition_occupied"
	ErrorClassHorizonLow        = "explicit_coverage_low"
)

type Maintainer interface {
	Ensure(context.Context, []Week) error
	Inspect(context.Context, time.Time, int) (Inspection, error)
}

type WorkerStats struct {
	LastAttemptAt  time.Time  `json:"last_attempt_at"`
	LastSuccessAt  time.Time  `json:"last_success_at"`
	FailureCount   uint64     `json:"failure_count"`
	LastErrorClass string     `json:"last_error_class,omitempty"`
	LastInspection Inspection `json:"last_inspection"`
}

type workerTicker interface {
	Chan() <-chan time.Time
	Stop()
}

type workerClock interface {
	Now() time.Time
	NewTicker(time.Duration) workerTicker
}

type realWorkerClock struct{}

func (realWorkerClock) Now() time.Time { return time.Now() }
func (realWorkerClock) NewTicker(duration time.Duration) workerTicker {
	return realWorkerTicker{ticker: time.NewTicker(duration)}
}

type realWorkerTicker struct {
	ticker *time.Ticker
}

func (t realWorkerTicker) Chan() <-chan time.Time { return t.ticker.C }
func (t realWorkerTicker) Stop()                  { t.ticker.Stop() }

type Worker struct {
	maintainer Maintainer
	clock      workerClock
	logger     *slog.Logger

	mu    sync.RWMutex
	stats WorkerStats
}

func NewWorker(maintainer Maintainer) *Worker {
	return newWorker(maintainer, realWorkerClock{})
}

func newWorker(maintainer Maintainer, clock workerClock) *Worker {
	return &Worker{
		maintainer: maintainer,
		clock:      clock,
		logger:     slog.Default(),
	}
}

func (w *Worker) Start(ctx context.Context) {
	if w == nil {
		return
	}
	w.runOnce(ctx)
	ticker := w.clock.NewTicker(workerCadence)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.Chan():
			w.runOnce(ctx)
		}
	}
}

func (w *Worker) Stats() WorkerStats {
	if w == nil {
		return WorkerStats{}
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	stats := w.stats
	stats.LastInspection.Reasons = append([]string(nil), w.stats.LastInspection.Reasons...)
	return stats
}

func (w *Worker) runOnce(parent context.Context) {
	attemptedAt := w.clock.Now().UTC()
	ctx, cancel := context.WithTimeout(parent, attemptTimeout)
	defer cancel()

	weeks, err := PlanWeeks(attemptedAt, MaintenanceWeeksAhead)
	if err != nil {
		w.recordFailure(attemptedAt, ErrorClassMaintenanceFailed, nil)
		w.logger.Error("request_event_partition_maintenance_failed", "error", err)
		return
	}
	if w.maintainer == nil {
		err := errors.New("request-event partition maintainer is not configured")
		w.recordFailure(attemptedAt, ErrorClassMaintenanceFailed, nil)
		w.logger.Error("request_event_partition_maintenance_failed", "error", err)
		return
	}

	ensureErr := w.maintainer.Ensure(ctx, weeks)
	inspection, inspectErr := w.maintainer.Inspect(ctx, attemptedAt, MinimumCoverageDays)
	if ensureErr != nil {
		class, message := classifyMaintenanceError(ensureErr)
		var observed *Inspection
		if inspectErr == nil {
			observed = &inspection
		}
		w.recordFailure(attemptedAt, class, observed)
		w.logger.Error(message, "error", ensureErr)
		return
	}
	if inspectErr != nil {
		w.recordFailure(attemptedAt, ErrorClassMaintenanceFailed, nil)
		w.logger.Error("request_event_partition_maintenance_failed", "error", inspectErr)
		return
	}
	if !inspection.Ready {
		class, message := classifyInspection(inspection)
		w.recordFailure(attemptedAt, class, &inspection)
		w.logger.Warn(message, "reasons", inspection.Reasons)
		return
	}

	w.recordSuccess(attemptedAt, inspection)
	w.logger.Info(
		"request_event_partition_maintenance_succeeded",
		"coverage_days", inspection.CoverageDays,
		"partition_pairs", inspection.PartitionPairs,
	)
}

func classifyMaintenanceError(err error) (string, string) {
	var occupied *DefaultPartitionOccupiedError
	if errors.As(err, &occupied) {
		return ErrorClassDefaultOccupied, "request_event_partition_default_occupied"
	}
	return ErrorClassMaintenanceFailed, "request_event_partition_maintenance_failed"
}

func classifyInspection(inspection Inspection) (string, string) {
	for _, reason := range inspection.Reasons {
		if reason == ReasonEventDefaultRows || reason == ReasonDetailDefaultRows {
			return ErrorClassDefaultOccupied, "request_event_partition_default_occupied"
		}
	}
	for _, reason := range inspection.Reasons {
		if reason == ReasonCoverageLow {
			return ErrorClassHorizonLow, "request_event_partition_horizon_low"
		}
	}
	return ErrorClassMaintenanceFailed, "request_event_partition_maintenance_failed"
}

func (w *Worker) recordFailure(attemptedAt time.Time, class string, inspection *Inspection) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stats.LastAttemptAt = attemptedAt
	w.stats.FailureCount++
	w.stats.LastErrorClass = class
	if inspection != nil {
		w.stats.LastInspection = *inspection
		w.stats.LastInspection.Reasons = append([]string(nil), inspection.Reasons...)
	}
}

func (w *Worker) recordSuccess(attemptedAt time.Time, inspection Inspection) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stats.LastAttemptAt = attemptedAt
	w.stats.LastSuccessAt = attemptedAt
	w.stats.LastErrorClass = ""
	w.stats.LastInspection = inspection
	w.stats.LastInspection.Reasons = append([]string(nil), inspection.Reasons...)
}
