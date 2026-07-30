package requestevents

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultNormalQueueSize  = 2048
	defaultFailureQueueSize = 256
	defaultBatchSize        = 100
	defaultFlushInterval    = 250 * time.Millisecond
	defaultWriteTimeout     = 2 * time.Second
	defaultDirectFallback   = 750 * time.Millisecond
	defaultDrainTimeout     = 3 * time.Second
	defaultFallbackWorkers  = 4
)

type Store interface {
	InsertSuccessBatch(context.Context, []Event) error
	InsertFailure(context.Context, Event, *Detail) error
}

type Config struct {
	NormalQueueSize     int
	FailureQueueSize    int
	BatchSize           int
	FlushInterval       time.Duration
	WriteTimeout        time.Duration
	DirectFallbackLimit time.Duration
	DrainTimeout        time.Duration
	FallbackWorkers     int
	CallbackTimeout     time.Duration
	OnPersistedBatch    func(context.Context, []Event)
}

type Stats struct {
	NormalQueueDepth               int    `json:"normal_queue_depth"`
	FailureQueueDepth              int    `json:"failure_queue_depth"`
	SuccessAccepted                uint64 `json:"success_accepted"`
	SuccessWritten                 uint64 `json:"success_written"`
	SuccessDropped                 uint64 `json:"success_dropped"`
	SuccessWriteFailures           uint64 `json:"success_write_failures"`
	SuccessBatchWrites             uint64 `json:"success_batch_writes"`
	ConfiguredBatchSize            int    `json:"configured_batch_size"`
	LastSuccessBatchSize           int64  `json:"last_success_batch_size"`
	MaxSuccessBatchSize            int64  `json:"max_success_batch_size"`
	LastSuccessBatchLatencyMicros  int64  `json:"last_success_batch_latency_micros"`
	MaxSuccessBatchLatencyMicros   int64  `json:"max_success_batch_latency_micros"`
	FailureAccepted                uint64 `json:"failure_accepted"`
	FailureWritten                 uint64 `json:"failure_written"`
	FailureWriteFailures           uint64 `json:"failure_write_failures"`
	FailureInsertWrites            uint64 `json:"failure_insert_writes"`
	LastFailureInsertLatencyMicros int64  `json:"last_failure_insert_latency_micros"`
	MaxFailureInsertLatencyMicros  int64  `json:"max_failure_insert_latency_micros"`
	FailureFallbackAttempts        uint64 `json:"failure_fallback_attempts"`
	Abandoned                      uint64 `json:"abandoned"`
}

type failureItem struct {
	event  Event
	detail *Detail
}

type Recorder struct {
	store                Store
	normalQueue          chan Event
	failureQueue         chan failureItem
	fallbackSlots        chan struct{}
	batchSize            int
	flushInterval        time.Duration
	writeTimeout         time.Duration
	directFallbackLimit  time.Duration
	drainTimeout         time.Duration
	stop                 chan struct{}
	done                 chan struct{}
	admissionMu          sync.RWMutex
	accepting            bool
	started              atomic.Bool
	fallbackWG           sync.WaitGroup
	successAccepted      atomic.Uint64
	successWritten       atomic.Uint64
	successDropped       atomic.Uint64
	successWriteFailures atomic.Uint64
	successBatchWrites   atomic.Uint64
	lastSuccessBatchSize atomic.Int64
	maxSuccessBatchSize  atomic.Int64
	lastSuccessLatencyUS atomic.Int64
	maxSuccessLatencyUS  atomic.Int64
	failureAccepted      atomic.Uint64
	failureWritten       atomic.Uint64
	failureWriteFailures atomic.Uint64
	failureInsertWrites  atomic.Uint64
	lastFailureLatencyUS atomic.Int64
	maxFailureLatencyUS  atomic.Int64
	failureFallbacks     atomic.Uint64
	abandoned            atomic.Uint64
	callbackTimeout      time.Duration
	onPersistedBatch     func(context.Context, []Event)
}

func NewRecorder(store Store, config Config) *Recorder {
	normalQueueSize := positiveOr(config.NormalQueueSize, defaultNormalQueueSize)
	failureQueueSize := positiveOr(config.FailureQueueSize, defaultFailureQueueSize)
	batchSize := positiveOr(config.BatchSize, defaultBatchSize)
	fallbackWorkers := positiveOr(config.FallbackWorkers, defaultFallbackWorkers)
	return &Recorder{
		store:               store,
		normalQueue:         make(chan Event, normalQueueSize),
		failureQueue:        make(chan failureItem, failureQueueSize),
		fallbackSlots:       make(chan struct{}, fallbackWorkers),
		batchSize:           batchSize,
		flushInterval:       durationOr(config.FlushInterval, defaultFlushInterval),
		writeTimeout:        durationOr(config.WriteTimeout, defaultWriteTimeout),
		directFallbackLimit: durationOr(config.DirectFallbackLimit, defaultDirectFallback),
		drainTimeout:        durationOr(config.DrainTimeout, defaultDrainTimeout),
		stop:                make(chan struct{}),
		done:                make(chan struct{}),
		accepting:           true,
		callbackTimeout:     durationOr(config.CallbackTimeout, 250*time.Millisecond),
		onPersistedBatch:    config.OnPersistedBatch,
	}
}

func (r *Recorder) Enqueue(event Event, detail *Detail) {
	if r == nil || r.store == nil || event.ID == "" || event.WorkspaceID == "" || event.APIKeyID == "" || event.OccurredAt.IsZero() {
		return
	}
	r.admissionMu.RLock()
	defer r.admissionMu.RUnlock()
	if !r.accepting {
		r.abandoned.Add(1)
		return
	}

	if event.StatusCode >= 400 {
		r.failureAccepted.Add(1)
		item := failureItem{event: event, detail: cloneDetail(detail)}
		select {
		case r.failureQueue <- item:
		default:
			r.enqueueDirectFallback(item)
		}
		return
	}

	r.successAccepted.Add(1)
	select {
	case r.normalQueue <- event:
	default:
		dropped := r.successDropped.Add(1)
		if dropped == 1 || dropped%1000 == 0 {
			slog.Warn("api_request_event_success_dropped",
				"workspace_id", event.WorkspaceID,
				"queue_depth", len(r.normalQueue),
				"total_dropped", dropped,
			)
		}
	}
}

func (r *Recorder) Start(ctx context.Context) {
	if r == nil || r.store == nil || !r.started.CompareAndSwap(false, true) {
		return
	}
	defer func() {
		r.fallbackWG.Wait()
		close(r.done)
	}()
	ticker := time.NewTicker(r.flushInterval)
	defer ticker.Stop()
	pending := make([]Event, 0, r.batchSize)
	ctxDone := ctx.Done()
	failureBurst := 0

	for {
		// Prefer at most eight failure writes in a row, then service a queued
		// success event so sustained failures cannot starve success batches.
		if failureBurst < 8 {
			select {
			case item := <-r.failureQueue:
				r.writeFailure(item, r.writeTimeout)
				failureBurst++
				continue
			default:
			}
		} else {
			if len(pending) > 0 {
				r.writeSuccessBatch(pending, r.writeTimeout)
				pending = pending[:0]
				failureBurst = 0
				continue
			}
			select {
			case event := <-r.normalQueue:
				pending = append(pending, event)
				failureBurst = 0
				// Flush immediately at the fairness boundary. Waiting for a full
				// batch or ticker here would let continuous failures postpone the
				// success write for hundreds more failure inserts.
				r.writeSuccessBatch(pending, r.writeTimeout)
				pending = pending[:0]
				continue
			default:
				failureBurst = 0
			}
		}

		select {
		case <-ctxDone:
			r.Stop()
			ctxDone = nil
		case <-r.stop:
			r.drain(pending)
			return
		case item := <-r.failureQueue:
			r.writeFailure(item, r.writeTimeout)
			failureBurst++
		case event := <-r.normalQueue:
			pending = append(pending, event)
			failureBurst = 0
			if len(pending) >= r.batchSize {
				r.writeSuccessBatch(pending, r.writeTimeout)
				pending = pending[:0]
			}
		case <-ticker.C:
			if len(pending) > 0 {
				r.writeSuccessBatch(pending, r.writeTimeout)
				pending = pending[:0]
			}
		}
	}
}

func (r *Recorder) Stop() {
	if r == nil {
		return
	}
	r.admissionMu.Lock()
	defer r.admissionMu.Unlock()
	if !r.accepting {
		return
	}
	r.accepting = false
	close(r.stop)
}

func (r *Recorder) Wait(ctx context.Context) error {
	if r == nil || !r.started.Load() {
		return nil
	}
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Recorder) Stats() Stats {
	if r == nil {
		return Stats{}
	}
	return Stats{
		NormalQueueDepth:               len(r.normalQueue),
		FailureQueueDepth:              len(r.failureQueue),
		SuccessAccepted:                r.successAccepted.Load(),
		SuccessWritten:                 r.successWritten.Load(),
		SuccessDropped:                 r.successDropped.Load(),
		SuccessWriteFailures:           r.successWriteFailures.Load(),
		SuccessBatchWrites:             r.successBatchWrites.Load(),
		ConfiguredBatchSize:            r.batchSize,
		LastSuccessBatchSize:           r.lastSuccessBatchSize.Load(),
		MaxSuccessBatchSize:            r.maxSuccessBatchSize.Load(),
		LastSuccessBatchLatencyMicros:  r.lastSuccessLatencyUS.Load(),
		MaxSuccessBatchLatencyMicros:   r.maxSuccessLatencyUS.Load(),
		FailureAccepted:                r.failureAccepted.Load(),
		FailureWritten:                 r.failureWritten.Load(),
		FailureWriteFailures:           r.failureWriteFailures.Load(),
		FailureInsertWrites:            r.failureInsertWrites.Load(),
		LastFailureInsertLatencyMicros: r.lastFailureLatencyUS.Load(),
		MaxFailureInsertLatencyMicros:  r.maxFailureLatencyUS.Load(),
		FailureFallbackAttempts:        r.failureFallbacks.Load(),
		Abandoned:                      r.abandoned.Load(),
	}
}

func (r *Recorder) enqueueDirectFallback(item failureItem) {
	r.failureFallbacks.Add(1)
	select {
	case r.fallbackSlots <- struct{}{}:
		r.fallbackWG.Add(1)
		go func() {
			defer r.fallbackWG.Done()
			defer func() { <-r.fallbackSlots }()
			r.writeFailure(item, r.directFallbackLimit)
		}()
	default:
		failures := r.failureWriteFailures.Add(1)
		slog.Error("api_request_event_failure_unpersisted",
			"event_id", item.event.ID,
			"workspace_id", item.event.WorkspaceID,
			"reason", "fallback_capacity_exhausted",
			"total_failures", failures,
		)
	}
}

func (r *Recorder) writeSuccessBatch(events []Event, timeout time.Duration) {
	if len(events) == 0 {
		return
	}
	batch := append([]Event(nil), events...)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	startedAt := time.Now()
	r.successBatchWrites.Add(1)
	batchSize := int64(len(batch))
	r.lastSuccessBatchSize.Store(batchSize)
	storeMax(&r.maxSuccessBatchSize, batchSize)
	defer func() { r.recordSuccessLatency(time.Since(startedAt)) }()
	if err := r.store.InsertSuccessBatch(ctx, batch); err != nil {
		failures := r.successWriteFailures.Add(uint64(len(batch)))
		slog.Warn("api_request_event_success_batch_failed", "count", len(batch), "error", err, "total_failures", failures)
		return
	}
	r.successWritten.Add(uint64(len(batch)))
	r.notifyPersistedBatch(batch)
}

func (r *Recorder) writeFailure(item failureItem, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	startedAt := time.Now()
	r.failureInsertWrites.Add(1)
	defer func() { r.recordFailureLatency(time.Since(startedAt)) }()
	if err := r.store.InsertFailure(ctx, item.event, item.detail); err != nil {
		failures := r.failureWriteFailures.Add(1)
		slog.Error("api_request_event_failure_write_failed",
			"event_id", item.event.ID,
			"workspace_id", item.event.WorkspaceID,
			"error", err,
			"total_failures", failures,
		)
		return
	}
	r.failureWritten.Add(1)
	r.notifyPersistedBatch([]Event{item.event})
}

func (r *Recorder) notifyPersistedBatch(events []Event) {
	if r.onPersistedBatch == nil || len(events) == 0 {
		return
	}
	batch := append([]Event(nil), events...)
	ctx, cancel := context.WithTimeout(context.Background(), r.callbackTimeout)
	defer cancel()
	r.onPersistedBatch(ctx, batch)
}

func (r *Recorder) recordSuccessLatency(duration time.Duration) {
	micros := duration.Microseconds()
	r.lastSuccessLatencyUS.Store(micros)
	storeMax(&r.maxSuccessLatencyUS, micros)
}

func (r *Recorder) recordFailureLatency(duration time.Duration) {
	micros := duration.Microseconds()
	r.lastFailureLatencyUS.Store(micros)
	storeMax(&r.maxFailureLatencyUS, micros)
}

func storeMax(target *atomic.Int64, value int64) {
	for current := target.Load(); value > current; current = target.Load() {
		if target.CompareAndSwap(current, value) {
			return
		}
	}
}

func (r *Recorder) drain(pending []Event) {
	deadline := time.Now().Add(r.drainTimeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			r.abandoned.Add(uint64(len(pending) + len(r.normalQueue) + len(r.failureQueue)))
			return
		}
		select {
		case item := <-r.failureQueue:
			r.writeFailure(item, minDuration(remaining, r.writeTimeout))
			continue
		default:
		}
		select {
		case event := <-r.normalQueue:
			pending = append(pending, event)
			if len(pending) >= r.batchSize {
				r.writeSuccessBatch(pending, minDuration(remaining, r.writeTimeout))
				pending = pending[:0]
			}
		default:
			if len(pending) > 0 {
				r.writeSuccessBatch(pending, minDuration(remaining, r.writeTimeout))
			}
			return
		}
	}
}

func cloneDetail(detail *Detail) *Detail {
	if detail == nil {
		return nil
	}
	copy := *detail
	copy.RequestHeaders = cloneStringMap(detail.RequestHeaders)
	copy.ResponseHeaders = cloneStringMap(detail.ResponseHeaders)
	copy.QueryKeys = append([]string(nil), detail.QueryKeys...)
	return &copy
}

func cloneStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func positiveOr(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func durationOr(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

func minDuration(left, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}
