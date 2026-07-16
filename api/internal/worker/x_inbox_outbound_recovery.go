package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/handler"
)

type XInboxOutboundRecoveryWorker struct {
	service *handler.XInboxOutboundRecoveryService
}

func NewXInboxOutboundRecoveryWorker(
	service *handler.XInboxOutboundRecoveryService,
) *XInboxOutboundRecoveryWorker {
	return &XInboxOutboundRecoveryWorker{service: service}
}

func (w *XInboxOutboundRecoveryWorker) Start(ctx context.Context) {
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

func (w *XInboxOutboundRecoveryWorker) runOnce(ctx context.Context) {
	stats, err := w.service.ProcessOnce(ctx)
	if err != nil {
		slog.Error("X Inbox outbound recovery failed", "error", err)
		return
	}
	if stats.Completed > 0 || stats.Deferred > 0 || stats.NeedsReconciliation > 0 {
		slog.Info("X Inbox outbound recovery complete",
			"scanned", stats.Scanned,
			"completed", stats.Completed,
			"deferred", stats.Deferred,
			"needs_reconciliation", stats.NeedsReconciliation)
	}
}
