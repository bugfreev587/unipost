package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

// MediaCleanupWorker hard-deletes media rows whose cleanup_after_at
// timestamp has passed, plus the matching R2 objects. The publish
// path tags large media (>= 200 MB) with a deadline 2h after a
// successful adapter.Post; this worker reaps them.
//
// Distinct from the abandoned-pending sweep folded into
// AnalyticsRefreshWorker: that one runs hourly and only touches
// status='pending' rows older than 7 days. This one runs every
// 5 minutes and only touches rows whose cleanup_after_at is set.
// They never overlap because cleanup_after_at is NULL on rows the
// other sweeper considers, and pending rows never have a
// cleanup_after_at set in the first place.
//
// A nil storage Client makes Start a no-op so a server without R2
// (or a test env) doesn't trip on the missing client.
type MediaCleanupWorker struct {
	queries *db.Queries
	storage *storage.Client
}

func NewMediaCleanupWorker(queries *db.Queries, store *storage.Client) *MediaCleanupWorker {
	return &MediaCleanupWorker{queries: queries, storage: store}
}

// mediaCleanupInterval is how often the worker checks for due rows.
// 5 minutes balances responsiveness (a 200 MB video is gone within
// ~5 min of its 2h cleanup window expiring) against DB churn.
const mediaCleanupInterval = 5 * time.Minute

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
	rows, err := w.queries.ListMediaDueForCleanup(ctx)
	if err != nil {
		slog.Error("media cleanup: failed to list due rows", "error", err)
		return
	}
	if len(rows) == 0 {
		return
	}
	slog.Info("media cleanup: processing due rows", "count", len(rows))

	for _, row := range rows {
		// R2 delete first; if R2 already lost the object the storage
		// layer treats a 404 as success, so we always advance the row.
		if err := w.storage.Delete(ctx, row.StorageKey); err != nil {
			slog.Warn("media cleanup: r2 delete failed",
				"media_id", row.ID,
				"size_bytes", row.SizeBytes,
				"error", err)
			continue
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
	}
}
