package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestMediaCleanupWorkerRunsDaily(t *testing.T) {
	if mediaCleanupInterval != 24*time.Hour {
		t.Fatalf("mediaCleanupInterval = %s, want 24h", mediaCleanupInterval)
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
		t.Fatalf("ListMediaDueForRetentionCleanup calls = %d, want 1", queries.listCalls)
	}
	if store.deleteCalls != mediaCleanupBatchSize {
		t.Fatalf("storage Delete calls = %d, want %d", store.deleteCalls, mediaCleanupBatchSize)
	}
	if queries.hardDeleteCalls != 0 {
		t.Fatalf("HardDeleteMedia calls = %d, want 0", queries.hardDeleteCalls)
	}
}

type mediaCleanupFakeQueries struct {
	rows            []db.Media
	listCalls       int
	hardDeleteCalls int
}

func (q *mediaCleanupFakeQueries) ListMediaDueForRetentionCleanup(context.Context, int32) ([]db.Media, error) {
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
