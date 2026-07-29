package observabilityreads

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultReadModeRefreshInterval   = time.Second
	DefaultReadModeEvaluationTimeout = 500 * time.Millisecond
)

var ErrReadModeSourceUnavailable = errors.New("observability read mode source is unavailable")

type ReadModeSource interface {
	Evaluate(context.Context) (bool, error)
}

type CachedReadSelectorConfig struct {
	RefreshInterval   time.Duration
	EvaluationTimeout time.Duration
	Logger            *slog.Logger
}

// CachedReadSelector keeps the live-log projection mode in process memory.
// Event delivery reads only the atomic value; the source is evaluated by one
// bounded process-level refresh loop, never per event or per connection.
type CachedReadSelector struct {
	source            ReadModeSource
	logger            *slog.Logger
	refreshInterval   time.Duration
	evaluationTimeout time.Duration
	enabled           atomic.Bool
	started           atomic.Bool
	stop              chan struct{}
	stopOnce          sync.Once
	done              chan struct{}
	cancelMu          sync.Mutex
	cancel            context.CancelFunc
}

func NewCachedReadSelector(source ReadModeSource, config CachedReadSelectorConfig) *CachedReadSelector {
	interval := config.RefreshInterval
	if interval <= 0 {
		interval = DefaultReadModeRefreshInterval
	}
	evaluationTimeout := config.EvaluationTimeout
	if evaluationTimeout <= 0 {
		evaluationTimeout = DefaultReadModeEvaluationTimeout
	}
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &CachedReadSelector{
		source:            source,
		logger:            logger,
		refreshInterval:   interval,
		evaluationTimeout: evaluationTimeout,
		stop:              make(chan struct{}),
		done:              make(chan struct{}),
	}
}

func (c *CachedReadSelector) UseV2(context.Context) bool {
	return c != nil && c.enabled.Load()
}

func (c *CachedReadSelector) Refresh(ctx context.Context) error {
	if c == nil {
		return ErrReadModeSourceUnavailable
	}
	if interfaceIsNil(c.source) {
		c.enabled.Store(false)
		return ErrReadModeSourceUnavailable
	}
	evaluationCtx, cancel := context.WithTimeout(ctx, c.evaluationTimeout)
	defer cancel()
	enabled, err := c.source.Evaluate(evaluationCtx)
	if err == nil {
		err = evaluationCtx.Err()
	}
	if err != nil {
		c.enabled.Store(false)
		c.logger.WarnContext(ctx, "observability live-log mode refresh failed; using legacy projection",
			"event", "observability_live_log_mode_refresh_failed",
			"error", err,
		)
		return err
	}
	c.enabled.Store(enabled)
	return nil
}

// Start launches the single refresh goroutine. Until its first successful
// refresh completes, UseV2 remains false.
func (c *CachedReadSelector) Start(ctx context.Context) {
	if c == nil || !c.started.CompareAndSwap(false, true) {
		return
	}
	workerCtx, cancel := context.WithCancel(ctx)
	c.cancelMu.Lock()
	c.cancel = cancel
	c.cancelMu.Unlock()
	select {
	case <-c.stop:
		cancel()
	default:
	}
	go c.run(workerCtx)
}

func (c *CachedReadSelector) run(ctx context.Context) {
	defer func() {
		c.cancelMu.Lock()
		c.cancel = nil
		c.cancelMu.Unlock()
		close(c.done)
	}()
	select {
	case <-ctx.Done():
		return
	case <-c.stop:
		return
	default:
	}
	_ = c.Refresh(ctx)
	ticker := time.NewTicker(c.refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stop:
			return
		case <-ticker.C:
			_ = c.Refresh(ctx)
		}
	}
}

func (c *CachedReadSelector) Stop() {
	if c == nil {
		return
	}
	c.stopOnce.Do(func() { close(c.stop) })
	c.cancelMu.Lock()
	cancel := c.cancel
	c.cancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (c *CachedReadSelector) Wait(ctx context.Context) error {
	if c == nil || !c.started.Load() {
		return nil
	}
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
