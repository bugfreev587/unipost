package socialconnectioncutover

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AbandonedPhaseGrace is how long a non-Expand phase must sit untouched before
// it is eligible for recovery. The advisory lock below is the real safety
// check; this only keeps a recovery from racing a cutover that has just
// started, and costs nothing because a real cutover refreshes updated_at as it
// advances through its phases.
const AbandonedPhaseGrace = 2 * time.Minute

// RecoverAbandonedCutoverPhase restores the Expand phase when a cutover process
// died between changing the phase and finishing.
//
// The orchestrator's own compensation runs in-process, so it cannot run when
// the process is killed outright — SIGKILL, an OOM kill, a pod eviction. A
// phase left at 'draining' or 'cutting_over' halts publishing for every
// workspace: the claim trigger refuses new delivery leases, and 'cutting_over'
// additionally rejects writes to bindings whose connection_id is NULL, which
// breaks connect and reconnect. Nothing else expires it.
//
// Safety rests on the cutover advisory lock being session-scoped. A live
// orchestrator holds it on a dedicated connection for its entire run, so
// pg_try_advisory_lock fails here and no recovery happens. When the process
// dies, Postgres drops that connection and releases the lock, which is what
// makes "nobody is running a cutover" decidable rather than a guess.
//
// A committed reconciliation is never undone: cutover.sql commits authority and
// phase = 'cutover' in one transaction, so a completed cutover no longer
// matches the phase filter.
//
// Reports whether a phase was recovered.
func RecoverAbandonedCutoverPhase(
	ctx context.Context,
	pool *pgxpool.Pool,
	grace time.Duration,
) (bool, error) {
	if pool == nil {
		return false, errors.New("social connection rollout pool is required")
	}
	if grace <= 0 {
		grace = AbandonedPhaseGrace
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return false, fmt.Errorf("acquire abandoned cutover phase connection: %w", err)
	}
	defer conn.Release()

	var locked bool
	if err := conn.QueryRow(ctx,
		`SELECT pg_try_advisory_lock(hashtextextended($1, 0))`, cutoverAdvisoryLockName,
	).Scan(&locked); err != nil {
		return false, fmt.Errorf("probe social connection cutover lock: %w", err)
	}
	if !locked {
		// A cutover is running. Its own compensation owns this phase.
		return false, nil
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx),
			`SELECT pg_advisory_unlock(hashtextextended($1, 0))`, cutoverAdvisoryLockName)
	}()

	var recovered string
	err = conn.QueryRow(ctx, `
UPDATE social_connection_rollout_state
SET phase = 'expand',
    cutover_backend_pid = NULL,
    updated_at = NOW()
WHERE id = 'global'
  AND phase IN ('draining', 'cutting_over')
  AND cutover_completed_at IS NULL
  AND updated_at < NOW() - $1::interval
RETURNING phase
`, grace.String()).Scan(&recovered)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("recover abandoned social connection cutover phase: %w", err)
	}
	return true, nil
}
