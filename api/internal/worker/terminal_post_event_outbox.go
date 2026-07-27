package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
)

type terminalPostEventOutboxStore interface {
	ClaimPendingTerminalPostEventDeliveries(context.Context, int32) ([]db.ClaimPendingTerminalPostEventDeliveriesRow, error)
	MarkTerminalPostEventDeliveryEnqueued(context.Context, db.MarkTerminalPostEventDeliveryEnqueuedParams) (int64, error)
	RetryTerminalPostEventDelivery(context.Context, db.RetryTerminalPostEventDeliveryParams) (int64, error)
}

type TerminalPostEventOutboxWorker struct {
	store terminalPostEventOutboxStore
	sinks map[string]events.DurableEventSink
	now   func() time.Time
}

func NewTerminalPostEventOutboxWorker(
	store terminalPostEventOutboxStore,
	webhook events.DurableEventSink,
	notification events.DurableEventSink,
	loops events.DurableEventSink,
) *TerminalPostEventOutboxWorker {
	return &TerminalPostEventOutboxWorker{
		store: store,
		sinks: map[string]events.DurableEventSink{
			"webhook":      webhook,
			"notification": notification,
			"loops":        loops,
		},
		now: time.Now,
	}
}

func (w *TerminalPostEventOutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *TerminalPostEventOutboxWorker) tick(ctx context.Context) {
	if w == nil || w.store == nil {
		return
	}
	rows, err := w.store.ClaimPendingTerminalPostEventDeliveries(ctx, 50)
	if err != nil {
		slog.Warn("terminal post event outbox: claim failed", "error", err)
		return
	}
	for _, row := range rows {
		sink := w.sinks[row.Sink]
		if sink == nil {
			err = fmt.Errorf("terminal post event sink %q is not configured", row.Sink)
		} else if row.EventType == "" {
			err = fmt.Errorf("terminal post event %q has no event type", row.EventID)
		} else {
			err = sink.PublishDurable(ctx, row.EventID, row.WorkspaceID, row.EventType, row.Payload)
		}
		if err != nil {
			_, retryErr := w.store.RetryTerminalPostEventDelivery(ctx, db.RetryTerminalPostEventDeliveryParams{
				EventID:          row.EventID,
				Sink:             row.Sink,
				ExpectedAttempts: row.Attempts,
				NextAttemptAt:    pgtype.Timestamptz{Time: terminalPostEventRetryAt(w.now().UTC(), row.Attempts), Valid: true},
				LastError:        pgtype.Text{String: truncate(err.Error(), 500), Valid: true},
			})
			if retryErr != nil {
				slog.Error("terminal post event outbox: retry scheduling failed", "event_id", row.EventID, "sink", row.Sink, "error", retryErr)
			}
			continue
		}
		if _, err := w.store.MarkTerminalPostEventDeliveryEnqueued(ctx, db.MarkTerminalPostEventDeliveryEnqueuedParams{
			EventID:          row.EventID,
			Sink:             row.Sink,
			ExpectedAttempts: row.Attempts,
		}); err != nil {
			slog.Error("terminal post event outbox: mark enqueued failed", "event_id", row.EventID, "sink", row.Sink, "error", err)
		}
	}
}

func terminalPostEventRetryAt(now time.Time, attempts int32) time.Time {
	delay := time.Duration(attempts) * 15 * time.Second
	if delay < 15*time.Second {
		delay = 15 * time.Second
	}
	if delay > 15*time.Minute {
		delay = 15 * time.Minute
	}
	return now.Add(delay)
}
