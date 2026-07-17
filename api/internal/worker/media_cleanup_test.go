package worker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestMediaCleanupWorkerRunsDaily(t *testing.T) {
	if mediaCleanupInterval != 24*time.Hour {
		t.Fatalf("mediaCleanupInterval = %s, want 24h", mediaCleanupInterval)
	}
}

func TestMediaCleanupWorkerOwnsHourlyAbandonedUploadSweep(t *testing.T) {
	if mediaAbandonedCleanupInterval != time.Hour {
		t.Fatalf("mediaAbandonedCleanupInterval = %s, want 1h", mediaAbandonedCleanupInterval)
	}
	queries := &mediaCleanupFakeQueries{abandonedRows: []db.Media{{
		ID:         "media_abandoned",
		StorageKey: "media/abandoned.gif",
	}}}
	store := &mediaCleanupFakeStorage{}
	worker := &MediaCleanupWorker{queries: queries, storage: store}

	worker.sweepAbandoned(context.Background())

	if queries.listAbandonedCalls != 1 {
		t.Fatalf("ClaimAbandonedMedia calls = %d, want 1", queries.listAbandonedCalls)
	}
	if store.deleteCalls != 1 || queries.hardDeleteCalls != 1 {
		t.Fatalf("storage/hard delete calls = %d/%d, want 1/1", store.deleteCalls, queries.hardDeleteCalls)
	}
}

func TestMediaCleanupWorkerKeepsAbandonedRowWhenObjectDeleteFails(t *testing.T) {
	queries := &mediaCleanupFakeQueries{abandonedRows: []db.Media{{
		ID:         "media_abandoned",
		StorageKey: "media/abandoned.gif",
	}}}
	store := &mediaCleanupFakeStorage{err: errors.New("r2 unavailable")}
	worker := &MediaCleanupWorker{queries: queries, storage: store}

	worker.sweepAbandoned(context.Background())

	if queries.hardDeleteCalls != 0 {
		t.Fatalf("hard delete calls = %d, want 0", queries.hardDeleteCalls)
	}
	if queries.releaseAbandonedCalls != 1 {
		t.Fatalf("release abandoned calls = %d, want 1", queries.releaseAbandonedCalls)
	}
}

func TestMediaCleanupWorkerStopsWhenFullBatchMakesNoProgress(t *testing.T) {
	rows := make([]db.Media, mediaCleanupBatchSize)
	for i := range rows {
		rows[i] = db.Media{
			ID:         "media_no_progress",
			StorageKey: "media/no-progress",
		}
	}
	queries := &mediaCleanupFakeQueries{rows: rows}
	store := &mediaCleanupFakeStorage{err: errors.New("r2 unavailable")}
	worker := &MediaCleanupWorker{queries: queries, storage: store}

	worker.sweepDue(context.Background())

	if queries.listCalls != 1 {
		t.Fatalf("ClaimMediaDueForRetentionCleanup calls = %d, want 1", queries.listCalls)
	}
	if store.deleteCalls != mediaCleanupBatchSize {
		t.Fatalf("storage Delete calls = %d, want %d", store.deleteCalls, mediaCleanupBatchSize)
	}
	if queries.hardDeleteCalls != 0 {
		t.Fatalf("HardDeleteMedia calls = %d, want 0", queries.hardDeleteCalls)
	}
}

func TestMediaCleanupWorkerRecordsCompletedRunWhenNoRowsDue(t *testing.T) {
	queries := &mediaCleanupFakeQueries{}
	store := &mediaCleanupFakeStorage{}
	worker := &MediaCleanupWorker{queries: queries, storage: store}

	worker.sweepDue(context.Background())

	if queries.staleCalls != 1 {
		t.Fatalf("stale recovery calls = %d, want 1", queries.staleCalls)
	}
	if len(queries.createRuns) != 1 {
		t.Fatalf("created runs = %d, want 1", len(queries.createRuns))
	}
	if len(queries.completeRuns) != 1 {
		t.Fatalf("completed runs = %d, want 1", len(queries.completeRuns))
	}
	got := queries.completeRuns[0]
	if got.Status != mediaCleanupRunCompleted || got.ScannedObjects != 0 || got.DeletedObjects != 0 || got.FailedObjects != 0 {
		t.Fatalf("completed run = %#v, want completed zero-count run", got)
	}
}

func TestMediaCleanupWorkerRecordsDeletedObjectTotals(t *testing.T) {
	queries := &mediaCleanupFakeQueries{rows: []db.Media{{
		ID:          "media_1",
		StorageKey:  "media/media_1.mp4",
		SizeBytes:   7_500_000,
		ContentType: "video/mp4",
	}}}
	store := &mediaCleanupFakeStorage{}
	worker := &MediaCleanupWorker{queries: queries, storage: store}

	worker.sweepDue(context.Background())

	if queries.hardDeleteCalls != 1 {
		t.Fatalf("HardDeleteMedia calls = %d, want 1", queries.hardDeleteCalls)
	}
	if len(queries.completeRuns) != 1 {
		t.Fatalf("completed runs = %d, want 1", len(queries.completeRuns))
	}
	got := queries.completeRuns[0]
	if got.Status != mediaCleanupRunCompleted || got.ScannedObjects != 1 || got.DeletedObjects != 1 || got.DeletedBytes != 7_500_000 || got.FailedObjects != 0 {
		t.Fatalf("completed run = %#v, want one deleted object and bytes", got)
	}
}

func TestMediaCleanupWorkerRecordsFailedObjectWithoutDeletingRow(t *testing.T) {
	queries := &mediaCleanupFakeQueries{rows: []db.Media{{
		ID:         "media_1",
		StorageKey: "media/media_1.mp4",
		SizeBytes:  6_400,
	}}}
	store := &mediaCleanupFakeStorage{err: errors.New("r2 unavailable")}
	worker := &MediaCleanupWorker{queries: queries, storage: store}

	worker.sweepDue(context.Background())

	if queries.hardDeleteCalls != 0 {
		t.Fatalf("HardDeleteMedia calls = %d, want 0", queries.hardDeleteCalls)
	}
	if len(queries.completeRuns) != 1 {
		t.Fatalf("completed runs = %d, want 1", len(queries.completeRuns))
	}
	got := queries.completeRuns[0]
	if got.Status != mediaCleanupRunCompletedWithErrors || got.ScannedObjects != 1 || got.DeletedObjects != 0 || got.FailedObjects != 1 || got.FailedBytes != 6_400 {
		t.Fatalf("completed run = %#v, want failed object counted separately", got)
	}
	if !got.ErrorSummary.Valid || !strings.Contains(got.ErrorSummary.String, "r2 delete failed") {
		t.Fatalf("error summary = %#v, want concise r2 delete failure", got.ErrorSummary)
	}
}

func TestMediaCleanupWorkerSkipsSweepWhenRunAlreadyActive(t *testing.T) {
	queries := &mediaCleanupFakeQueries{
		createErr: &pgconn.PgError{Code: "23505", ConstraintName: "media_cleanup_runs_one_running_idx"},
		rows: []db.Media{{
			ID:         "media_1",
			StorageKey: "media/media_1.mp4",
		}},
	}
	store := &mediaCleanupFakeStorage{}
	worker := &MediaCleanupWorker{queries: queries, storage: store}

	worker.sweepDue(context.Background())

	if queries.listCalls != 0 || store.deleteCalls != 0 || queries.hardDeleteCalls != 0 {
		t.Fatalf("list/delete/hardDelete calls = %d/%d/%d, want no sweep when active run exists", queries.listCalls, store.deleteCalls, queries.hardDeleteCalls)
	}
	if len(queries.createRuns) != 0 || len(queries.completeRuns) != 0 {
		t.Fatalf("created/completed runs = %d/%d, want no normal run when active run exists", len(queries.createRuns), len(queries.completeRuns))
	}
}

func TestMediaCleanupWorkerRecoversStaleRunningRowsBeforeSweep(t *testing.T) {
	queries := &mediaCleanupFakeQueries{}
	store := &mediaCleanupFakeStorage{}
	worker := &MediaCleanupWorker{queries: queries, storage: store}

	worker.sweepDue(context.Background())

	if len(queries.staleCutoffs) != 1 {
		t.Fatalf("stale cutoffs = %d, want 1", len(queries.staleCutoffs))
	}
	cutoff := queries.staleCutoffs[0]
	if !cutoff.Valid {
		t.Fatalf("stale cutoff should be valid")
	}
	if time.Since(cutoff.Time) < 47*time.Hour || time.Since(cutoff.Time) > 49*time.Hour {
		t.Fatalf("stale cutoff = %s, want about 48h ago", cutoff.Time)
	}
}

type mediaCleanupFakeQueries struct {
	rows                  []db.Media
	abandonedRows         []db.Media
	listCalls             int
	listAbandonedCalls    int
	releaseAbandonedCalls int
	hardDeleteCalls       int
	staleCalls            int
	staleCutoffs          []pgtype.Timestamptz
	createErr             error
	createRuns            []db.CreateMediaCleanupRunParams
	completeRuns          []db.CompleteMediaCleanupRunParams
}

func (q *mediaCleanupFakeQueries) ClaimAbandonedMedia(context.Context) ([]db.Media, error) {
	q.listAbandonedCalls++
	return q.abandonedRows, nil
}

func (q *mediaCleanupFakeQueries) ReleaseAbandonedMediaClaim(context.Context, string) error {
	q.releaseAbandonedCalls++
	return nil
}

func (q *mediaCleanupFakeQueries) MarkStaleMediaCleanupRunsFailed(_ context.Context, arg db.MarkStaleMediaCleanupRunsFailedParams) (int64, error) {
	q.staleCalls++
	q.staleCutoffs = append(q.staleCutoffs, arg.StartedAt)
	return 0, nil
}

func (q *mediaCleanupFakeQueries) CreateMediaCleanupRun(_ context.Context, arg db.CreateMediaCleanupRunParams) (db.MediaCleanupRun, error) {
	if q.createErr != nil {
		return db.MediaCleanupRun{}, q.createErr
	}
	q.createRuns = append(q.createRuns, arg)
	return db.MediaCleanupRun{ID: "cleanup_run_1"}, nil
}

func (q *mediaCleanupFakeQueries) CompleteMediaCleanupRun(_ context.Context, arg db.CompleteMediaCleanupRunParams) (db.MediaCleanupRun, error) {
	q.completeRuns = append(q.completeRuns, arg)
	return db.MediaCleanupRun{ID: arg.ID, Status: arg.Status}, nil
}

func (q *mediaCleanupFakeQueries) ClaimMediaDueForRetentionCleanup(context.Context, int32) ([]db.Media, error) {
	q.listCalls++
	return q.rows, nil
}

func (q *mediaCleanupFakeQueries) HardDeleteMedia(context.Context, string) error {
	q.hardDeleteCalls++
	return nil
}

type mediaCleanupFakeStorage struct {
	err         error
	deleteCalls int
}

func (s *mediaCleanupFakeStorage) Delete(context.Context, string) error {
	s.deleteCalls++
	return s.err
}
