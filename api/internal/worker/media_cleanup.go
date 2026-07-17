package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

// MediaCleanupWorker owns both business-state retention cleanup and the
// abandoned pending-upload sweep. Terminal retention runs daily; pending
// uploads older than seven days are checked hourly.
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
	MarkStaleMediaCleanupRunsFailed(context.Context, db.MarkStaleMediaCleanupRunsFailedParams) (int64, error)
	CreateMediaCleanupRun(context.Context, db.CreateMediaCleanupRunParams) (db.MediaCleanupRun, error)
	CompleteMediaCleanupRun(context.Context, db.CompleteMediaCleanupRunParams) (db.MediaCleanupRun, error)
	ClaimMediaDueForRetentionCleanup(context.Context, int32) ([]db.Media, error)
	ClaimAbandonedMedia(context.Context) ([]db.Media, error)
	ReleaseAbandonedMediaClaim(context.Context, string) error
	HardDeleteMedia(context.Context, string) error
}

type mediaCleanupStorage interface {
	Delete(context.Context, string) error
}

// mediaCleanupInterval is how often the worker checks for due rows.
// Retention is measured in days by plan, so daily cleanup is enough
// and avoids unnecessary DB/R2 churn.
const mediaCleanupInterval = 24 * time.Hour
const mediaAbandonedCleanupInterval = time.Hour

const mediaCleanupBatchSize = 500
const mediaCleanupWorkerName = "media_cleanup"
const mediaCleanupRunCompleted = "completed"
const mediaCleanupRunCompletedWithErrors = "completed_with_errors"
const mediaCleanupRunFailed = "failed"

const mediaCleanupStaleRunningAfter = 2 * mediaCleanupInterval

func (w *MediaCleanupWorker) Start(ctx context.Context) {
	if w.storage == nil {
		slog.Info("media cleanup worker: storage not configured, worker disabled")
		return
	}

	retentionTicker := time.NewTicker(mediaCleanupInterval)
	defer retentionTicker.Stop()
	abandonedTicker := time.NewTicker(mediaAbandonedCleanupInterval)
	defer abandonedTicker.Stop()

	slog.Info("media cleanup worker started", "interval", mediaCleanupInterval)

	// Run once on startup so a freshly-deployed instance doesn't sit
	// idle for the first interval before processing the backlog.
	w.sweepAbandoned(ctx)
	w.sweepDue(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("media cleanup worker stopped")
			return
		case <-retentionTicker.C:
			w.sweepDue(ctx)
		case <-abandonedTicker.C:
			w.sweepAbandoned(ctx)
		}
	}
}

// sweepAbandoned removes pending Media whose upload reservation has been
// unused for seven days. Storage deletion happens first so a transient R2
// failure keeps the database row available for the next hourly retry.
func (w *MediaCleanupWorker) sweepAbandoned(ctx context.Context) {
	rows, err := w.queries.ClaimAbandonedMedia(ctx)
	if err != nil {
		slog.Error("media cleanup: failed to list abandoned uploads", "error", err)
		return
	}
	if len(rows) == 0 {
		return
	}

	slog.Info("media cleanup: cleaning abandoned uploads", "count", len(rows))
	for _, row := range rows {
		if err := w.storage.Delete(ctx, row.StorageKey); err != nil {
			slog.Warn("media cleanup: abandoned object delete failed", "media_id", row.ID, "error", err)
			if releaseErr := w.queries.ReleaseAbandonedMediaClaim(ctx, row.ID); releaseErr != nil {
				slog.Warn("media cleanup: abandoned claim release failed", "media_id", row.ID, "error", releaseErr)
			}
			continue
		}
		if err := w.queries.HardDeleteMedia(ctx, row.ID); err != nil {
			slog.Warn("media cleanup: abandoned row delete failed", "media_id", row.ID, "error", err)
		}
	}
}

// sweepDue atomically claims media rows past their retention gates, then
// deletes each R2 object before hard-deleting the row. A claimed row remains
// soft-deleted after an R2 error so the next tick can safely retry it.
func (w *MediaCleanupWorker) sweepDue(ctx context.Context) {
	startedAt := time.Now().UTC()
	if _, err := w.queries.MarkStaleMediaCleanupRunsFailed(ctx, db.MarkStaleMediaCleanupRunsFailedParams{
		WorkerName: mediaCleanupWorkerName,
		StartedAt:  pgtype.Timestamptz{Time: startedAt.Add(-mediaCleanupStaleRunningAfter), Valid: true},
	}); err != nil {
		slog.Warn("media cleanup: stale run recovery failed", "error", err)
	}

	run, err := w.queries.CreateMediaCleanupRun(ctx, db.CreateMediaCleanupRunParams{
		WorkerName: mediaCleanupWorkerName,
		StartedAt:  pgtype.Timestamptz{Time: startedAt, Valid: true},
		NextRunAt:  pgtype.Timestamptz{Time: startedAt.Add(mediaCleanupInterval), Valid: true},
	})
	if err != nil {
		if isUniqueViolation(err) {
			slog.Info("media cleanup: another active run exists, skipping")
			return
		}
		slog.Error("media cleanup: failed to create run", "error", err)
		return
	}

	stats := mediaCleanupRunStats{}
	status := mediaCleanupRunCompleted
	for {
		rows, err := w.queries.ClaimMediaDueForRetentionCleanup(ctx, mediaCleanupBatchSize)
		if err != nil {
			slog.Error("media cleanup: failed to claim due rows", "error", err)
			status = mediaCleanupRunFailed
			stats.addError("claim due rows failed", err)
			break
		}
		if len(rows) == 0 {
			break
		}
		slog.Info("media cleanup: processing due rows", "count", len(rows))
		stats.scanned += int32(len(rows))

		deletedCount := 0
		for _, row := range rows {
			// R2 delete first; if R2 already lost the object the storage
			// layer treats a missing object as success, so we always advance
			// the row after successful API calls.
			if err := w.storage.Delete(ctx, row.StorageKey); err != nil {
				stats.recordFailure(row, "r2 delete failed", err)
				slog.Warn("media cleanup: r2 delete failed",
					"media_id", row.ID,
					"size_bytes", row.SizeBytes,
					"error", err)
				continue
			}
			if pullKey := storage.PullObjectKeyForSource(row.StorageKey); pullKey != "" {
				if err := w.storage.Delete(ctx, pullKey); err != nil {
					stats.recordFailure(row, "r2 pull-copy delete failed", err)
					slog.Warn("media cleanup: r2 pull-copy delete failed",
						"media_id", row.ID,
						"pull_key", pullKey,
						"error", err)
					continue
				}
			}
			if err := w.queries.HardDeleteMedia(ctx, row.ID); err != nil {
				stats.recordFailure(row, "db delete failed", err)
				slog.Warn("media cleanup: db delete failed",
					"media_id", row.ID,
					"error", err)
				continue
			}
			stats.deletedObjects++
			stats.deletedBytes += row.SizeBytes
			slog.Info("media cleanup: deleted",
				"media_id", row.ID,
				"size_bytes", row.SizeBytes,
				"content_type", row.ContentType)
			deletedCount++
		}
		if len(rows) < mediaCleanupBatchSize || deletedCount == 0 {
			break
		}
	}
	if stats.failedObjects > 0 && status == mediaCleanupRunCompleted {
		status = mediaCleanupRunCompletedWithErrors
	}
	w.completeRun(ctx, run.ID, status, stats)
}

func (w *MediaCleanupWorker) completeRun(ctx context.Context, runID, status string, stats mediaCleanupRunStats) {
	if _, err := w.queries.CompleteMediaCleanupRun(ctx, db.CompleteMediaCleanupRunParams{
		ID:             runID,
		Status:         status,
		FinishedAt:     pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		ScannedObjects: stats.scanned,
		DeletedObjects: stats.deletedObjects,
		DeletedBytes:   stats.deletedBytes,
		FailedObjects:  stats.failedObjects,
		FailedBytes:    stats.failedBytes,
		ErrorSummary:   textOrNull(stats.summary()),
	}); err != nil {
		slog.Error("media cleanup: failed to complete run", "run_id", runID, "error", err)
	}
}

type mediaCleanupRunStats struct {
	scanned        int32
	deletedObjects int32
	deletedBytes   int64
	failedObjects  int32
	failedBytes    int64
	errors         []string
}

func (s *mediaCleanupRunStats) recordFailure(row db.Media, label string, err error) {
	s.failedObjects++
	s.failedBytes += row.SizeBytes
	s.addError(label, err)
}

func (s *mediaCleanupRunStats) addError(label string, err error) {
	if len(s.errors) >= 3 {
		return
	}
	s.errors = append(s.errors, fmt.Sprintf("%s: %v", label, err))
}

func (s mediaCleanupRunStats) summary() string {
	return strings.Join(s.errors, "; ")
}

func textOrNull(value string) pgtype.Text {
	if strings.TrimSpace(value) == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	message := err.Error()
	return strings.Contains(message, "SQLSTATE 23505") ||
		strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "media_cleanup_runs_one_running_idx")
}
