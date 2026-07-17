package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/xcredits"
)

type XInboxExposureRecoveryWorker struct {
	service interface {
		ReconcilePendingExposures(context.Context, int, time.Time) (xcredits.ExposureReleaseReconcileStats, error)
	}
}

func NewXInboxExposureRecoveryWorker(service interface {
	ReconcilePendingExposures(context.Context, int, time.Time) (xcredits.ExposureReleaseReconcileStats, error)
}) *XInboxExposureRecoveryWorker {
	return &XInboxExposureRecoveryWorker{service: service}
}

func (w *XInboxExposureRecoveryWorker) Start(ctx context.Context) {
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

func (w *XInboxExposureRecoveryWorker) runOnce(ctx context.Context) {
	stats, err := w.service.ReconcilePendingExposures(ctx, 100, time.Now().UTC())
	if err != nil {
		slog.Error("X Inbox exposure release recovery failed", "error", err)
		return
	}
	if stats.Released > 0 || stats.Finalized > 0 ||
		stats.NeedsReconciliation > 0 || stats.Deferred > 0 {
		slog.Info("X Inbox exposure release recovery complete",
			"scanned", stats.Scanned,
			"released", stats.Released,
			"finalized", stats.Finalized,
			"needs_reconciliation", stats.NeedsReconciliation,
			"deferred", stats.Deferred)
	}
}
