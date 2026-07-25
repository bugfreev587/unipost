package integrationlogs

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

const (
	defaultQueueSize    = 2048
	defaultWriteTimeout = 5 * time.Second
)

type integrationLogStore interface {
	InsertIntegrationLog(context.Context, db.InsertIntegrationLogParams) (db.IntegrationLog, error)
}

type queuedEvent struct {
	event Event
}

type Logger struct {
	queries    integrationLogStore
	afterWrite func(context.Context, db.IntegrationLog)
	queue      chan queuedEvent
	dropped    atomic.Uint64
	failures   atomic.Uint64
}

func NewLogger(queries integrationLogStore, afterWrite func(context.Context, db.IntegrationLog)) *Logger {
	return &Logger{
		queries:    queries,
		afterWrite: afterWrite,
		queue:      make(chan queuedEvent, defaultQueueSize),
	}
}

func (l *Logger) Start(ctx context.Context) {
	if l == nil || l.queries == nil {
		return
	}

	slog.Info("integration logger started", "queue_size", cap(l.queue))
	for {
		select {
		case <-ctx.Done():
			drained := l.drainQueue()
			slog.Info("integration logger stopped", "drained", drained)
			return
		case item := <-l.queue:
			l.writeNow(item.event)
		}
	}
}

func (l *Logger) drainQueue() int {
	drained := 0
	for {
		select {
		case item := <-l.queue:
			l.writeNow(item.event)
			drained++
		default:
			return drained
		}
	}
}

func (l *Logger) writeNow(e Event) {
	opCtx, cancel := context.WithTimeout(context.Background(), defaultWriteTimeout)
	defer cancel()

	err := l.insert(opCtx, e)
	if err != nil {
		l.failures.Add(1)
		slog.Warn("integration_log_write_failed",
			"workspace_id", e.WorkspaceID,
			"category", e.Category,
			"action", e.Action,
			"source", e.Source,
			"error", err,
			"total_failures", l.failures.Load(),
		)
	}
}

func (l *Logger) insert(ctx context.Context, e Event) error {
	row, err := l.queries.InsertIntegrationLog(ctx, Normalize(e))
	if err != nil {
		return err
	}
	if l.afterWrite != nil {
		l.afterWrite(ctx, row)
	}
	return nil
}

func (l *Logger) WriteSync(ctx context.Context, e Event) error {
	if l == nil || l.queries == nil || e.WorkspaceID == "" || e.Action == "" {
		return errors.New("integration logger unavailable")
	}

	opCtx, cancel := context.WithTimeout(ctx, defaultWriteTimeout)
	defer cancel()
	if err := l.insert(opCtx, e); err != nil {
		l.failures.Add(1)
		return err
	}
	return nil
}

func (l *Logger) Write(ctx context.Context, e Event) {
	if l == nil || l.queries == nil || e.WorkspaceID == "" || e.Action == "" {
		return
	}

	select {
	case l.queue <- queuedEvent{event: e}:
	default:
		l.dropped.Add(1)
		slog.Warn("integration_log_dropped",
			"workspace_id", e.WorkspaceID,
			"category", e.Category,
			"action", e.Action,
			"source", e.Source,
			"queue_size", cap(l.queue),
			"total_dropped", l.dropped.Load(),
		)
	}
}
