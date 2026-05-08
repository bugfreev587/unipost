package integrationlogs

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

const (
	defaultQueueSize   = 2048
	defaultWriteTimout = 5 * time.Second
)

type queuedEvent struct {
	event Event
}

type Logger struct {
	queries    *db.Queries
	afterWrite func(context.Context, db.IntegrationLog)
	queue      chan queuedEvent
	dropped    atomic.Uint64
	failures   atomic.Uint64
}

func NewLogger(queries *db.Queries, afterWrite func(context.Context, db.IntegrationLog)) *Logger {
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
	opCtx, cancel := context.WithTimeout(context.Background(), defaultWriteTimout)
	defer cancel()

	params := Normalize(e)
	row, err := l.queries.InsertIntegrationLog(opCtx, params)
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
		return
	}
	if l.afterWrite != nil {
		l.afterWrite(opCtx, row)
	}
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
