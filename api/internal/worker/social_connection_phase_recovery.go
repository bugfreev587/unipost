package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/socialconnectioncutover"
)

// SocialConnectionPhaseRecoveryWorker restores the Expand rollout phase after a
// cutover process is killed mid-run.
//
// Without it, a SIGKILL or pod eviction during cutover leaves the phase at
// 'draining' or 'cutting_over', which stops delivery claims for every workspace
// and — in 'cutting_over' — also rejects connect and reconnect writes. Nothing
// expires the phase on its own, so recovery would need a human with database
// access to notice and run UPDATE by hand.
//
// The check is cheap (one advisory-lock probe, then at most one guarded UPDATE)
// and safe to run in every process: a live cutover holds the advisory lock, so
// concurrent recovery attempts all decline.
type SocialConnectionPhaseRecoveryWorker struct {
	pool     *pgxpool.Pool
	interval time.Duration
	grace    time.Duration
}

func NewSocialConnectionPhaseRecoveryWorker(pool *pgxpool.Pool) *SocialConnectionPhaseRecoveryWorker {
	return &SocialConnectionPhaseRecoveryWorker{
		pool:     pool,
		interval: 30 * time.Second,
		grace:    socialconnectioncutover.AbandonedPhaseGrace,
	}
}

func (w *SocialConnectionPhaseRecoveryWorker) Start(ctx context.Context) {
	if w == nil || w.pool == nil {
		return
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *SocialConnectionPhaseRecoveryWorker) runOnce(ctx context.Context) {
	recovered, err := socialconnectioncutover.RecoverAbandonedCutoverPhase(ctx, w.pool, w.grace)
	if err != nil {
		slog.Error("social connection rollout phase recovery failed", "error", err)
		return
	}
	if recovered {
		// Loud on purpose. This means a cutover attempt died partway through,
		// so an operator has an interrupted run to investigate even though
		// publishing has already resumed.
		slog.Warn("restored social connection rollout phase to expand after an interrupted cutover; " +
			"delivery claims have resumed — review social_connection_cutover_runs before retrying")
	}
}
