package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/handler"
)

type PostDispatchWorker struct {
	queries     *db.Queries
	postHandler *handler.SocialPostHandler
}

const staleDeliveryAttemptTimeout = 5 * time.Minute

// claimBatchLimit is the per-tick claim count. Conservative —
// platforms tolerate parallel publish but the per-account
// serialization in ClaimPostDispatchJobs already throttles real
// fan-out, so 20 is plenty.
const claimBatchLimit = 20

// workspaceConcurrentDispatchCap is the per-workspace cap on
// running+retrying delivery jobs the worker will allow in flight
// at any moment. Phase-2 of the rate-limit PRD: this is the
// worker-domain protection layer that the API-side admission
// controls cannot reach. The number is intentionally tier-blind
// for v1 — Phase 3 promotes it to a per-plan map. 30 is sized to
// cover routine publishing fan-out (a 5-platform post = 5 jobs)
// while still capping a runaway retry storm to a manageable
// number of concurrent platform calls.
const workspaceConcurrentDispatchCap = 30

func NewPostDispatchWorker(queries *db.Queries, postHandler *handler.SocialPostHandler) *PostDispatchWorker {
	return &PostDispatchWorker{queries: queries, postHandler: postHandler}
}

func (w *PostDispatchWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	slog.Info("post dispatch worker started")
	for {
		select {
		case <-ctx.Done():
			slog.Info("post dispatch worker stopped")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *PostDispatchWorker) runOnce(ctx context.Context) {
	if err := w.postHandler.RecoverStaleDeliveryJobs(ctx, staleDeliveryAttemptTimeout); err != nil {
		slog.Error("post dispatch worker: stale recovery failed", "error", err)
	}
	jobs, err := w.queries.ClaimPostDispatchJobs(ctx, db.ClaimPostDispatchJobsParams{
		BatchLimit:             claimBatchLimit,
		WorkspaceConcurrentCap: workspaceConcurrentDispatchCap,
	})
	if err != nil {
		slog.Error("post dispatch worker: claim failed", "error", err)
		return
	}
	for _, job := range jobs {
		if err := w.postHandler.ProcessPostDeliveryJob(ctx, job); err != nil {
			slog.Error("post dispatch worker: process failed", "job_id", job.ID, "error", err)
		}
	}
}

type PostRetryWorker struct {
	queries     *db.Queries
	postHandler *handler.SocialPostHandler
}

func NewPostRetryWorker(queries *db.Queries, postHandler *handler.SocialPostHandler) *PostRetryWorker {
	return &PostRetryWorker{queries: queries, postHandler: postHandler}
}

func (w *PostRetryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	slog.Info("post retry worker started")
	for {
		select {
		case <-ctx.Done():
			slog.Info("post retry worker stopped")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *PostRetryWorker) runOnce(ctx context.Context) {
	if err := w.postHandler.RecoverStaleDeliveryJobs(ctx, staleDeliveryAttemptTimeout); err != nil {
		slog.Error("post retry worker: stale recovery failed", "error", err)
	}
	jobs, err := w.queries.ClaimPostRetryJobs(ctx, db.ClaimPostRetryJobsParams{
		BatchLimit:             claimBatchLimit,
		WorkspaceConcurrentCap: workspaceConcurrentDispatchCap,
	})
	if err != nil {
		slog.Error("post retry worker: claim failed", "error", err)
		return
	}
	for _, job := range jobs {
		if err := w.postHandler.ProcessPostDeliveryJob(ctx, job); err != nil {
			slog.Error("post retry worker: process failed", "job_id", job.ID, "error", err)
		}
	}
}

type PostDeliveryCleanupWorker struct {
	postHandler *handler.SocialPostHandler
}

func NewPostDeliveryCleanupWorker(postHandler *handler.SocialPostHandler) *PostDeliveryCleanupWorker {
	return &PostDeliveryCleanupWorker{postHandler: postHandler}
}

func (w *PostDeliveryCleanupWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	slog.Info("post delivery cleanup worker started")
	for {
		select {
		case <-ctx.Done():
			slog.Info("post delivery cleanup worker stopped")
			return
		case <-ticker.C:
			if err := w.postHandler.CleanupSucceededDeliveryJobs(ctx, 14*24*time.Hour); err != nil {
				slog.Error("post delivery cleanup worker: cleanup failed", "error", err)
			}
		}
	}
}
