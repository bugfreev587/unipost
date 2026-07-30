package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/xaccountreads"
)

type XAccountReadRecoveryWorker struct {
	service interface {
		Reconcile(context.Context, int, time.Time) (xaccountreads.ReconcileStats, error)
	}
}

func NewXAccountReadRecoveryWorker(service interface {
	Reconcile(context.Context, int, time.Time) (xaccountreads.ReconcileStats, error)
}) *XAccountReadRecoveryWorker {
	return &XAccountReadRecoveryWorker{service: service}
}

func (w *XAccountReadRecoveryWorker) Start(ctx context.Context) {
	if w == nil || w.service == nil {
		return
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	w.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *XAccountReadRecoveryWorker) runOnce(ctx context.Context) {
	stats, err := w.service.Reconcile(ctx, 100, time.Now().UTC())
	if err != nil {
		slog.Error("X account-read recovery failed", "error", err)
		return
	}
	if stats.Scanned > 0 || stats.Cleaned > 0 {
		slog.Info("X account-read recovery complete",
			"scanned", stats.Scanned,
			"succeeded", stats.Succeeded,
			"released", stats.Released,
			"deferred", stats.Deferred,
			"cleaned", stats.Cleaned,
		)
	}
}
