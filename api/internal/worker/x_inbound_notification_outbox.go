package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
)

type xInboundNotificationOutboxStore interface {
	ClaimPendingXInboundNotifications(context.Context, int32) ([]db.XInboundCapNotification, error)
	MarkXInboundNotificationEnqueued(context.Context, string) error
	RetryXInboundNotification(context.Context, db.RetryXInboundNotificationParams) error
}

type xInboundNotificationEnqueuer interface {
	EnqueueXInboundNotification(context.Context, string, string, string, []byte) error
}

type XInboundNotificationOutboxWorker struct {
	store    xInboundNotificationOutboxStore
	enqueuer xInboundNotificationEnqueuer
	now      func() time.Time
}

func NewXInboundNotificationOutboxWorker(
	store xInboundNotificationOutboxStore,
	enqueuer xInboundNotificationEnqueuer,
) *XInboundNotificationOutboxWorker {
	return &XInboundNotificationOutboxWorker{
		store:    store,
		enqueuer: enqueuer,
		now:      time.Now,
	}
}

func (w *XInboundNotificationOutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
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

func (w *XInboundNotificationOutboxWorker) tick(ctx context.Context) {
	if w == nil || w.store == nil || w.enqueuer == nil {
		return
	}
	rows, err := w.store.ClaimPendingXInboundNotifications(ctx, 50)
	if err != nil {
		slog.Warn("X inbound notification outbox: claim failed", "error", err)
		return
	}
	for _, row := range rows {
		event := row.EventType
		if event == "" {
			event = xInboundEventForThreshold(row.Threshold)
		}
		if err := w.enqueuer.EnqueueXInboundNotification(
			ctx,
			row.WorkspaceID,
			event,
			row.ID,
			row.Payload,
		); err != nil {
			retryAt := xInboundOutboxRetryAt(w.now().UTC(), row.Attempts)
			if retryErr := w.store.RetryXInboundNotification(ctx, db.RetryXInboundNotificationParams{
				ID:            row.ID,
				NextAttemptAt: pgtype.Timestamptz{Time: retryAt, Valid: true},
				LastError:     pgtype.Text{String: truncate(err.Error(), 500), Valid: true},
			}); retryErr != nil {
				slog.Error("X inbound notification outbox: retry scheduling failed", "id", row.ID, "error", retryErr)
			}
			continue
		}
		if err := w.store.MarkXInboundNotificationEnqueued(ctx, row.ID); err != nil {
			slog.Error("X inbound notification outbox: mark enqueued failed", "id", row.ID, "error", err)
		}
	}
}

func xInboundEventForThreshold(threshold int16) string {
	if threshold == 100 {
		return events.EventBillingXInboundCapReached
	}
	return events.EventBillingXInbound80pct
}

func xInboundOutboxRetryAt(now time.Time, attempts int32) time.Time {
	delay := time.Duration(attempts) * 30 * time.Second
	if delay < 30*time.Second {
		delay = 30 * time.Second
	}
	if delay > 15*time.Minute {
		delay = 15 * time.Minute
	}
	return now.Add(delay)
}
