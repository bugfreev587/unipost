package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/handler"
)

type PostDispatchWorker struct {
	queries     *db.Queries
	postHandler *handler.SocialPostHandler
	config      PostDeliveryWorkerConfig
	executor    *postDeliveryExecutor
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
	remaining map[string]db.PostDeliveryJob
}

func startLeaseHeartbeat(ctx context.Context, queries *db.Queries, jobs []db.PostDeliveryJob) *leaseHeartbeat {
	hbCtx, cancel := context.WithCancel(ctx)
	hb := &leaseHeartbeat{cancel: cancel, remaining: make(map[string]db.PostDeliveryJob, len(jobs))}
	for _, j := range jobs {
		hb.remaining[j.ID] = j
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
				jobs := make([]db.PostDeliveryJob, 0, len(hb.remaining))
				for _, job := range hb.remaining {
					jobs = append(jobs, job)
				}
				hb.mu.Unlock()
				for _, job := range jobs {
					if err := queries.RenewPostDeliveryJobLease(hbCtx, db.RenewPostDeliveryJobLeaseParams{
						ID:            job.ID,
						LeaseSeconds:  leaseSecondsArg(),
						LeaseOwner:    job.LeaseOwner,
						LastAttemptAt: job.LastAttemptAt,
					}); err != nil {
						slog.Warn("delivery lease renew failed", "job_id", job.ID, "error", err)
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
	return NewPostDispatchWorkerWithConfig(queries, postHandler, DefaultPostDeliveryWorkerConfigFromEnv())
}

func NewPostDispatchWorkerWithConfig(queries *db.Queries, postHandler *handler.SocialPostHandler, config PostDeliveryWorkerConfig) *PostDispatchWorker {
	config = normalizePostDeliveryWorkerConfig(config)
	executor := NewPostDeliveryExecutor(queries, postHandler, config)
	return NewPostDispatchWorkerWithExecutor(queries, postHandler, config, executor)
}

func NewPostDispatchWorkerWithExecutor(queries *db.Queries, postHandler *handler.SocialPostHandler, config PostDeliveryWorkerConfig, executor *postDeliveryExecutor) *PostDispatchWorker {
	config = normalizePostDeliveryWorkerConfig(config)
	return &PostDispatchWorker{
		queries:     queries,
		postHandler: postHandler,
		config:      config,
		executor:    executor,
	}
}

func (w *PostDispatchWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	slog.Info("post dispatch worker started",
		"claim_batch_limit", w.config.ClaimBatchLimit,
		"workspace_concurrent_cap", w.config.WorkspaceConcurrentCap,
		"global_concurrency", w.config.GlobalConcurrency,
		"platform_concurrency_caps", w.config.PlatformConcurrencyCaps)
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
	claimLimit := w.executor.reserveSlots(w.config.ClaimBatchLimit)
	if claimLimit <= 0 {
		return
	}
	jobs, err := w.queries.ClaimPostDispatchJobs(ctx, db.ClaimPostDispatchJobsParams{
		BatchLimit:             claimLimit,
		WorkspaceConcurrentCap: w.config.WorkspaceConcurrentCap,
		LeaseSeconds:           leaseSecondsArg(),
		LeaseOwner:             leaseOwnerArg(),
	})
	if err != nil {
		w.executor.releaseReservedSlots(int(claimLimit))
		slog.Error("post dispatch worker: claim failed", "error", err)
		return
	}
	if len(jobs) == 0 {
		w.executor.releaseReservedSlots(int(claimLimit))
		return
	}
	if unused := int(claimLimit) - len(jobs); unused > 0 {
		w.executor.releaseReservedSlots(unused)
	}
	w.executor.submitReserved(ctx, jobs)
	slog.Info("post dispatch worker: claimed jobs",
		"claim_batch_size", len(jobs),
		"active_worker_slots", w.executor.activeSlots(),
		"reserved_worker_slots", w.executor.reservedSlotsCount(),
		"jobs_waiting_inside_worker", w.executor.waitingSlots(),
		"free_worker_slots", w.executor.freeGlobalSlots(),
		"per_platform_active", w.executor.platformActiveCounts(),
		"per_workspace_active", w.executor.workspaceActiveCounts())
}

type PostRetryWorker struct {
	queries     *db.Queries
	postHandler *handler.SocialPostHandler
	config      PostDeliveryWorkerConfig
	executor    *postDeliveryExecutor
}

func NewPostRetryWorker(queries *db.Queries, postHandler *handler.SocialPostHandler) *PostRetryWorker {
	return NewPostRetryWorkerWithConfig(queries, postHandler, DefaultPostDeliveryWorkerConfigFromEnv())
}

func NewPostRetryWorkerWithConfig(queries *db.Queries, postHandler *handler.SocialPostHandler, config PostDeliveryWorkerConfig) *PostRetryWorker {
	config = normalizePostDeliveryWorkerConfig(config)
	executor := NewPostDeliveryExecutor(queries, postHandler, config)
	return NewPostRetryWorkerWithExecutor(queries, postHandler, config, executor)
}

func NewPostRetryWorkerWithExecutor(queries *db.Queries, postHandler *handler.SocialPostHandler, config PostDeliveryWorkerConfig, executor *postDeliveryExecutor) *PostRetryWorker {
	config = normalizePostDeliveryWorkerConfig(config)
	return &PostRetryWorker{
		queries:     queries,
		postHandler: postHandler,
		config:      config,
		executor:    executor,
	}
}

func (w *PostRetryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	slog.Info("post retry worker started",
		"claim_batch_limit", w.config.ClaimBatchLimit,
		"workspace_concurrent_cap", w.config.WorkspaceConcurrentCap,
		"global_concurrency", w.config.GlobalConcurrency,
		"platform_concurrency_caps", w.config.PlatformConcurrencyCaps)
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
	claimLimit := w.executor.reserveSlots(w.config.ClaimBatchLimit)
	if claimLimit <= 0 {
		return
	}
	jobs, err := w.queries.ClaimPostRetryJobs(ctx, db.ClaimPostRetryJobsParams{
		BatchLimit:             claimLimit,
		WorkspaceConcurrentCap: w.config.WorkspaceConcurrentCap,
		LeaseSeconds:           leaseSecondsArg(),
		LeaseOwner:             leaseOwnerArg(),
	})
	if err != nil {
		w.executor.releaseReservedSlots(int(claimLimit))
		slog.Error("post retry worker: claim failed", "error", err)
		return
	}
	if len(jobs) == 0 {
		w.executor.releaseReservedSlots(int(claimLimit))
		return
	}
	if unused := int(claimLimit) - len(jobs); unused > 0 {
		w.executor.releaseReservedSlots(unused)
	}
	w.executor.submitReserved(ctx, jobs)
	slog.Info("post retry worker: claimed jobs",
		"claim_batch_size", len(jobs),
		"active_worker_slots", w.executor.activeSlots(),
		"reserved_worker_slots", w.executor.reservedSlotsCount(),
		"jobs_waiting_inside_worker", w.executor.waitingSlots(),
		"free_worker_slots", w.executor.freeGlobalSlots(),
		"per_platform_active", w.executor.platformActiveCounts(),
		"per_workspace_active", w.executor.workspaceActiveCounts())
}

type PostDeliveryWorkerConfig struct {
	ClaimBatchLimit         int32
	WorkspaceConcurrentCap  int32
	GlobalConcurrency       int
	PlatformConcurrencyCaps map[string]int
}

func DefaultPostDeliveryWorkerConfigFromEnv() PostDeliveryWorkerConfig {
	return normalizePostDeliveryWorkerConfig(PostDeliveryWorkerConfig{
		ClaimBatchLimit:        int32(envInt("POST_DELIVERY_CLAIM_BATCH_LIMIT", claimBatchLimit)),
		WorkspaceConcurrentCap: int32(envInt("POST_DELIVERY_WORKSPACE_CONCURRENT_CAP", workspaceConcurrentDispatchCap)),
		GlobalConcurrency:      envInt("POST_DELIVERY_GLOBAL_CONCURRENCY", 10),
		PlatformConcurrencyCaps: map[string]int{
			"instagram": envInt("POST_DELIVERY_PLATFORM_CAP_INSTAGRAM", 3),
			"tiktok":    envInt("POST_DELIVERY_PLATFORM_CAP_TIKTOK", 3),
			"twitter":   envInt("POST_DELIVERY_PLATFORM_CAP_TWITTER", 5),
		},
	})
}

func normalizePostDeliveryWorkerConfig(config PostDeliveryWorkerConfig) PostDeliveryWorkerConfig {
	if config.ClaimBatchLimit <= 0 {
		config.ClaimBatchLimit = claimBatchLimit
	}
	if config.WorkspaceConcurrentCap < 0 {
		config.WorkspaceConcurrentCap = 0
	}
	if config.GlobalConcurrency <= 0 {
		config.GlobalConcurrency = 10
	}
	normalizedCaps := map[string]int{}
	for platform, cap := range config.PlatformConcurrencyCaps {
		if cap > 0 {
			normalizedCaps[strings.ToLower(strings.TrimSpace(platform))] = cap
		}
	}
	config.PlatformConcurrencyCaps = normalizedCaps
	return config
}

func envInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		slog.Warn("invalid integer env value; using default", "name", name, "value", raw, "default", fallback)
		return fallback
	}
	return value
}

type postDeliveryJobProcessor interface {
	ProcessPostDeliveryJob(context.Context, db.PostDeliveryJob) error
}

type postDeliveryExecutor struct {
	queries       *db.Queries
	processor     postDeliveryJobProcessor
	config        PostDeliveryWorkerConfig
	workerName    string
	reservedSlots chan struct{}
	globalSlots   chan struct{}
	platformCaps  map[string]chan struct{}

	mu                sync.Mutex
	activeByPlatform  map[string]int
	activeByWorkspace map[string]int
}

func newPostDeliveryExecutor(queries *db.Queries, processor postDeliveryJobProcessor, config PostDeliveryWorkerConfig, workerName string) *postDeliveryExecutor {
	config = normalizePostDeliveryWorkerConfig(config)
	platformCaps := make(map[string]chan struct{}, len(config.PlatformConcurrencyCaps))
	for platform, cap := range config.PlatformConcurrencyCaps {
		platformCaps[platform] = make(chan struct{}, cap)
	}
	return &postDeliveryExecutor{
		queries:           queries,
		processor:         processor,
		config:            config,
		workerName:        workerName,
		reservedSlots:     make(chan struct{}, config.GlobalConcurrency),
		globalSlots:       make(chan struct{}, config.GlobalConcurrency),
		platformCaps:      platformCaps,
		activeByPlatform:  map[string]int{},
		activeByWorkspace: map[string]int{},
	}
}

func NewPostDeliveryExecutor(queries *db.Queries, postHandler *handler.SocialPostHandler, config PostDeliveryWorkerConfig) *postDeliveryExecutor {
	return newPostDeliveryExecutor(queries, postHandler, config, "post delivery worker")
}

func (e *postDeliveryExecutor) submitReserved(ctx context.Context, jobs []db.PostDeliveryJob) {
	for _, job := range jobs {
		job := job
		go e.process(ctx, job)
	}
}

func (e *postDeliveryExecutor) process(ctx context.Context, job db.PostDeliveryJob) {
	defer e.releaseReservedSlots(1)
	hb := startLeaseHeartbeat(ctx, e.queries, []db.PostDeliveryJob{job})
	defer hb.stop()
	defer hb.done(job.ID)

	releasePlatform, ok := e.acquirePlatform(ctx, job.Platform)
	if !ok {
		return
	}
	defer releasePlatform()

	select {
	case e.globalSlots <- struct{}{}:
		defer func() { <-e.globalSlots }()
	case <-ctx.Done():
		return
	}

	e.incrementPlatform(job.Platform)
	defer e.decrementPlatform(job.Platform)
	e.incrementWorkspace(job.WorkspaceID)
	defer e.decrementWorkspace(job.WorkspaceID)

	if err := e.processor.ProcessPostDeliveryJob(ctx, job); err != nil {
		slog.Error(e.workerName+": process failed", "job_id", job.ID, "error", err)
	}
}

func (e *postDeliveryExecutor) acquirePlatform(ctx context.Context, platform string) (func(), bool) {
	sem, ok := e.platformCaps[strings.ToLower(strings.TrimSpace(platform))]
	if !ok {
		return func() {}, true
	}
	select {
	case sem <- struct{}{}:
		return func() { <-sem }, true
	case <-ctx.Done():
		return nil, false
	}
}

func (e *postDeliveryExecutor) reserveSlots(max int32) int32 {
	var reserved int32
	for reserved < max {
		select {
		case e.reservedSlots <- struct{}{}:
			reserved++
		default:
			return reserved
		}
	}
	return reserved
}

func (e *postDeliveryExecutor) releaseReservedSlots(count int) {
	for i := 0; i < count; i++ {
		select {
		case <-e.reservedSlots:
		default:
			return
		}
	}
}

func (e *postDeliveryExecutor) freeGlobalSlots() int {
	return cap(e.reservedSlots) - len(e.reservedSlots)
}

func (e *postDeliveryExecutor) activeSlots() int {
	return len(e.globalSlots)
}

func (e *postDeliveryExecutor) reservedSlotsCount() int {
	return len(e.reservedSlots)
}

func (e *postDeliveryExecutor) waitingSlots() int {
	waiting := len(e.reservedSlots) - len(e.globalSlots)
	if waiting < 0 {
		return 0
	}
	return waiting
}

func (e *postDeliveryExecutor) incrementPlatform(platform string) {
	key := strings.ToLower(strings.TrimSpace(platform))
	e.mu.Lock()
	e.activeByPlatform[key]++
	e.mu.Unlock()
}

func (e *postDeliveryExecutor) incrementWorkspace(workspaceID string) {
	e.mu.Lock()
	e.activeByWorkspace[workspaceID]++
	e.mu.Unlock()
}

func (e *postDeliveryExecutor) decrementPlatform(platform string) {
	key := strings.ToLower(strings.TrimSpace(platform))
	e.mu.Lock()
	if e.activeByPlatform[key] <= 1 {
		delete(e.activeByPlatform, key)
	} else {
		e.activeByPlatform[key]--
	}
	e.mu.Unlock()
}

func (e *postDeliveryExecutor) decrementWorkspace(workspaceID string) {
	e.mu.Lock()
	if e.activeByWorkspace[workspaceID] <= 1 {
		delete(e.activeByWorkspace, workspaceID)
	} else {
		e.activeByWorkspace[workspaceID]--
	}
	e.mu.Unlock()
}

func (e *postDeliveryExecutor) platformActiveCounts() map[string]int {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make(map[string]int, len(e.activeByPlatform))
	for platform, count := range e.activeByPlatform {
		out[platform] = count
	}
	return out
}

func (e *postDeliveryExecutor) workspaceActiveCounts() map[string]int {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make(map[string]int, len(e.activeByWorkspace))
	for workspaceID, count := range e.activeByWorkspace {
		out[workspaceID] = count
	}
	return out
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
