package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

// MediaCleanupWorker hard-deletes media rows that the media_post_usages
// ledger says are past their plan/status retention window, plus the
// matching R2 objects.
//
// Distinct from the abandoned-pending sweep folded into
// AnalyticsRefreshWorker: that one runs hourly and only touches
// status='pending' rows older than 7 days. This one runs every
// day and only touches rows that have a terminal post usage whose
// cleanup_after_at is set. They never overlap because pending rows are
// not attached to terminal post usage rows.
//
// A nil storage Client makes Start a no-op so a server without R2
// (or a test env) doesn't trip on the missing client.
type MediaCleanupWorker struct {
	queries mediaCleanupQueries
	storage mediaCleanupStorage
}

func NewMediaCleanupWorker(queries *db.Queries, store *storage.Client) *MediaCleanupWorker {
	var cleanupStore mediaCleanupStorage
	if store != nil {
		cleanupStore = store
	}
	return &MediaCleanupWorker{queries: queries, storage: cleanupStore}
}

type mediaCleanupQueries interface {
	ListMediaDueForRetentionCleanup(context.Context, int32) ([]db.Media, error)
	HardDeleteMedia(context.Context, string) error
}

type mediaCleanupStorage interface {
	Delete(context.Context, string) error
}

// mediaCleanupInterval is how often the worker checks for due rows.
// Retention is measured in days by plan, so daily cleanup is enough
// and avoids unnecessary DB/R2 churn.
const mediaCleanupInterval = 24 * time.Hour

const mediaCleanupBatchSize = 500

func (w *MediaCleanupWorker) Start(ctx context.Context) {
	if w.storage == nil {
		slog.Info("media cleanup worker: storage not configured, worker disabled")
		return
	}

	ticker := time.NewTicker(mediaCleanupInterval)
	defer ticker.Stop()

	slog.Info("media cleanup worker started", "interval", mediaCleanupInterval)

	// Run once on startup so a freshly-deployed instance doesn't sit
	// idle for the first interval before processing the backlog.
	w.sweepDue(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("media cleanup worker stopped")
			return
		case <-ticker.C:
			w.sweepDue(ctx)
		}
	}
}

// sweepDue lists media rows past their cleanup_after_at and deletes
// each one's R2 object before hard-deleting the row. R2 delete
// errors leave the row in place so the next tick can retry.
func (w *MediaCleanupWorker) sweepDue(ctx context.Context) {
	for {
		rows, err := w.queries.ListMediaDueForRetentionCleanup(ctx, mediaCleanupBatchSize)
		if err != nil {
			slog.Error("media cleanup: failed to list due rows", "error", err)
			return
		}
		if len(rows) == 0 {
			return
		}
		slog.Info("media cleanup: processing due rows", "count", len(rows))

		deletedCount := 0
		for _, row := range rows {
			// R2 delete first; if R2 already lost the object the storage
			// layer treats a missing object as success, so we always advance
			// the row after successful API calls.
			if err := w.storage.Delete(ctx, row.StorageKey); err != nil {
				slog.Warn("media cleanup: r2 delete failed",
					"media_id", row.ID,
					"size_bytes", row.SizeBytes,
					"error", err)
				continue
			}
			if pullKey := storage.PullObjectKeyForSource(row.StorageKey); pullKey != "" {
				if err := w.storage.Delete(ctx, pullKey); err != nil {
					slog.Warn("media cleanup: r2 pull-copy delete failed",
						"media_id", row.ID,
						"pull_key", pullKey,
						"error", err)
					continue
				}
			}
			if err := w.queries.HardDeleteMedia(ctx, row.ID); err != nil {
				slog.Warn("media cleanup: db delete failed",
					"media_id", row.ID,
					"error", err)
				continue
			}
			slog.Info("media cleanup: deleted",
				"media_id", row.ID,
				"size_bytes", row.SizeBytes,
				"content_type", row.ContentType)
			deletedCount++
		}
		if len(rows) < mediaCleanupBatchSize || deletedCount == 0 {
			return
		}
	}
}
