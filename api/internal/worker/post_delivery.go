package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/handler"
)

type PostDispatchWorker struct {
	queries     *db.Queries
	postHandler *handler.SocialPostHandler
}

const staleDeliveryAttemptTimeout = 5 * time.Minute

// Lease/heartbeat. A claimed job holds a lease that the owning worker
// renews while it still owns and is working the job (including jobs waiting
// their turn in the serial processing loop). Stale recovery only reaps jobs
// whose lease has actually expired, so a slow-but-alive worker is never
// mistaken for a dead one. leaseRenewInterval must be comfortably below
// deliveryJobLeaseTTL so a single missed tick doesn't drop the lease.
const (
	deliveryJobLeaseTTL = 90 * time.Second
	leaseRenewInterval  = 30 * time.Second
)

// deliveryWorkerOwner identifies this process in lease_owner for debugging
// (which instance holds a job). Not used for correctness — lease expiry is.
var deliveryWorkerOwner = func() string {
	host, _ := os.Hostname()
	return fmt.Sprintf("%s/%d", host, os.Getpid())
}()

func leaseSecondsArg() int32 { return int32(deliveryJobLeaseTTL / time.Second) }

func leaseOwnerArg() pgtype.Text {
	return pgtype.Text{String: deliveryWorkerOwner, Valid: true}
}

// leaseHeartbeat renews the leases of the jobs a worker claimed until each
// is processed (removed via done) or the batch finishes (stop). This keeps
// owned jobs — including those still queued behind others — out of reach of
// stale recovery while the worker is alive.
type leaseHeartbeat struct {
	cancel    context.CancelFunc
	mu        sync.Mutex
	remaining map[string]bool
}

func startLeaseHeartbeat(ctx context.Context, queries *db.Queries, jobs []db.PostDeliveryJob) *leaseHeartbeat {
	hbCtx, cancel := context.WithCancel(ctx)
	hb := &leaseHeartbeat{cancel: cancel, remaining: make(map[string]bool, len(jobs))}
	for _, j := range jobs {
		hb.remaining[j.ID] = true
	}
	go func() {
		ticker := time.NewTicker(leaseRenewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-ticker.C:
				hb.mu.Lock()
				ids := make([]string, 0, len(hb.remaining))
				for id := range hb.remaining {
					ids = append(ids, id)
				}
				hb.mu.Unlock()
				for _, id := range ids {
					if err := queries.RenewPostDeliveryJobLease(hbCtx, db.RenewPostDeliveryJobLeaseParams{
						ID:           id,
						LeaseSeconds: leaseSecondsArg(),
					}); err != nil {
						slog.Warn("delivery lease renew failed", "job_id", id, "error", err)
					}
				}
			}
		}
	}()
	return hb
}

// done marks a job as no longer needing lease renewal (it has been processed).
func (hb *leaseHeartbeat) done(jobID string) {
	hb.mu.Lock()
	delete(hb.remaining, jobID)
	hb.mu.Unlock()
}

// stop ends the heartbeat goroutine.
func (hb *leaseHeartbeat) stop() { hb.cancel() }

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
		LeaseSeconds:           leaseSecondsArg(),
		LeaseOwner:             leaseOwnerArg(),
	})
	if err != nil {
		slog.Error("post dispatch worker: claim failed", "error", err)
		return
	}
	if len(jobs) == 0 {
		return
	}
	processClaimedPostDeliveryJobs(ctx, w.queries, w.postHandler, jobs, "post dispatch worker")
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
		LeaseSeconds:           leaseSecondsArg(),
		LeaseOwner:             leaseOwnerArg(),
	})
	if err != nil {
		slog.Error("post retry worker: claim failed", "error", err)
		return
	}
	if len(jobs) == 0 {
		return
	}
	processClaimedPostDeliveryJobs(ctx, w.queries, w.postHandler, jobs, "post retry worker")
}

func processClaimedPostDeliveryJobs(ctx context.Context, queries *db.Queries, postHandler *handler.SocialPostHandler, jobs []db.PostDeliveryJob, workerName string) {
	hb := startLeaseHeartbeat(ctx, queries, jobs)
	defer hb.stop()
	var wg sync.WaitGroup
	for _, job := range jobs {
		job := job
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer hb.done(job.ID)
			if err := postHandler.ProcessPostDeliveryJob(ctx, job); err != nil {
				slog.Error(workerName+": process failed", "job_id", job.ID, "error", err)
			}
		}()
	}
	wg.Wait()
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
