package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

const (
	mediaProcessingCoordinatorInterval = 5 * time.Second
	mediaProcessingRecoveryBatch       = 20
)

type mediaProcessingCoordinatorQueries interface {
	PromoteDueMediaProcessingRetriesByKind(context.Context, string) (int64, error)
	RecoverStaleMediaProcessingJobs(context.Context, int32) ([]db.MediaProcessingJob, error)
	ClaimMediaProcessingJobsByKind(context.Context, db.ClaimMediaProcessingJobsByKindParams) ([]db.MediaProcessingJob, error)
	TouchMediaProcessingJobHeartbeat(context.Context, string) (int64, error)
}

type claimedMediaProcessor interface {
	ProcessClaimedJob(context.Context, db.MediaProcessingJob) error
}

type MediaProcessingCoordinator struct {
	queries mediaProcessingCoordinatorQueries
	audio   claimedMediaProcessor
	gif     claimedMediaProcessor

	runMu     sync.Mutex
	preferred string
}

func NewMediaProcessingCoordinator(queries mediaProcessingCoordinatorQueries, audio, gif claimedMediaProcessor) *MediaProcessingCoordinator {
	return &MediaProcessingCoordinator{queries: queries, audio: audio, gif: gif, preferred: mediaAudioOverlayKind}
}

func (c *MediaProcessingCoordinator) Start(ctx context.Context) {
	if c == nil || c.queries == nil || c.audio == nil || c.gif == nil {
		slog.Info("media processing coordinator: dependencies not configured, worker disabled")
		return
	}
	ticker := time.NewTicker(mediaProcessingCoordinatorInterval)
	defer ticker.Stop()
	slog.Info("media processing coordinator started", "interval", mediaProcessingCoordinatorInterval, "concurrency", 1)
	c.RunOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			slog.Info("media processing coordinator stopped")
			return
		case <-ticker.C:
			c.RunOnce(ctx)
		}
	}
}

func (c *MediaProcessingCoordinator) RunOnce(ctx context.Context) {
	if c == nil || c.queries == nil || c.audio == nil || c.gif == nil || !c.runMu.TryLock() {
		return
	}
	defer c.runMu.Unlock()
	for _, kind := range []string{mediaAudioOverlayKind, mediaGIFConversionKind} {
		promoted, err := c.queries.PromoteDueMediaProcessingRetriesByKind(ctx, kind)
		if err != nil {
			slog.Error("media processing coordinator: retry promotion failed", "kind", kind, "error", err)
			return
		}
		if promoted > 0 {
			slog.Info("media processing retries promoted", "kind", kind, "count", promoted)
		}
	}
	if recovered, err := c.queries.RecoverStaleMediaProcessingJobs(ctx, mediaProcessingRecoveryBatch); err != nil {
		slog.Error("media processing coordinator: stale recovery failed", "error", err)
		return
	} else {
		for _, job := range recovered {
			slog.Warn("media processing stale job recovered", "job_id", job.ID, "kind", job.Kind, "status", job.Status, "attempts", job.Attempts, "error_code", job.ErrorCode.String)
		}
	}

	first := c.preferred
	second := otherMediaProcessingKind(first)
	for _, kind := range []string{first, second} {
		jobs, err := c.queries.ClaimMediaProcessingJobsByKind(ctx, db.ClaimMediaProcessingJobsByKindParams{JobKind: kind, BatchLimit: 1})
		if err != nil {
			slog.Error("media processing coordinator: claim failed", "kind", kind, "error", err)
			return
		}
		if len(jobs) == 0 {
			continue
		}
		job := jobs[0]
		c.preferred = otherMediaProcessingKind(kind)
		started := time.Now()
		queueWaitMS := int64(0)
		if job.CreatedAt.Valid {
			queueWaitMS = time.Since(job.CreatedAt.Time).Milliseconds()
		}
		slog.Info("media processing job claimed", "job_id", job.ID, "kind", job.Kind, "attempts", job.Attempts, "queue_wait_ms", queueWaitMS)
		processor := c.audio
		if kind == mediaGIFConversionKind {
			processor = c.gif
		}
		heartbeatCtx, stopHeartbeat := context.WithCancel(ctx)
		go c.heartbeat(heartbeatCtx, job)
		err = processor.ProcessClaimedJob(ctx, job)
		stopHeartbeat()
		if err != nil {
			slog.Error("media processing coordinator: processing failed", "job_id", job.ID, "kind", job.Kind, "attempts", job.Attempts, "duration_ms", time.Since(started).Milliseconds(), "error", err)
		} else {
			slog.Info("media processing coordinator: processing completed", "job_id", job.ID, "kind", job.Kind, "attempts", job.Attempts, "duration_ms", time.Since(started).Milliseconds())
		}
		return
	}
}

func (c *MediaProcessingCoordinator) heartbeat(ctx context.Context, job db.MediaProcessingJob) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rows, err := c.queries.TouchMediaProcessingJobHeartbeat(ctx, job.ID)
			if err != nil {
				slog.Error("media processing heartbeat failed", "job_id", job.ID, "kind", job.Kind, "error", err)
				continue
			}
			if rows == 0 {
				slog.Warn("media processing heartbeat lost job ownership", "job_id", job.ID, "kind", job.Kind)
				return
			}
			slog.Debug("media processing heartbeat refreshed", "job_id", job.ID, "kind", job.Kind)
		}
	}
}

func otherMediaProcessingKind(kind string) string {
	if kind == mediaGIFConversionKind {
		return mediaAudioOverlayKind
	}
	return mediaGIFConversionKind
}
