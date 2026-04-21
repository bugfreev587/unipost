package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/handler"
)

// SchedulerWorker claims due scheduled posts and hands them off to the
// shared delivery-job enqueue path.
type SchedulerWorker struct {
	queries      *db.Queries
	postHandler  *handler.SocialPostHandler
}

func NewSchedulerWorker(queries *db.Queries, postHandler *handler.SocialPostHandler) *SchedulerWorker {
	return &SchedulerWorker{queries: queries, postHandler: postHandler}
}

func (w *SchedulerWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	slog.Info("scheduler worker started")
	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler worker stopped")
			return
		case <-ticker.C:
			w.enqueueDue(ctx)
		}
	}
}

func (w *SchedulerWorker) enqueueDue(ctx context.Context) {
	posts, err := w.queries.GetDueScheduledPosts(ctx)
	if err != nil {
		slog.Error("scheduler: failed to get due posts", "error", err)
		return
	}
	for _, post := range posts {
		claimed, err := w.queries.ClaimScheduledPost(ctx, post.ID)
		if err != nil {
			continue
		}
		go func(post db.SocialPost) {
			if err := w.postHandler.EnqueueScheduledPost(ctx, post); err != nil {
				slog.Error("scheduler: failed to enqueue scheduled post", "post_id", post.ID, "error", err)
			}
		}(claimed)
	}
}
