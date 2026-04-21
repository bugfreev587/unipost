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
	jobs, err := w.queries.ClaimPostDispatchJobs(ctx, 20)
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
	jobs, err := w.queries.ClaimPostRetryJobs(ctx, 20)
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
