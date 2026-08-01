//go:build integration

package socialconnectioncutover

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func newAbandonedPhaseTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	// A migrated database per test. Migration 138 creates
	// social_connection_rollout_state seeded at 'expand', and each test moves it
	// from there. Sharing one database across tests would let them race on the
	// single 'global' row.
	pool, err := pgxpool.New(context.Background(), newCutoverIntegrationDatabase(t))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func seedPhase(t *testing.T, pool *pgxpool.Pool, phase string, age time.Duration, completed bool) {
	t.Helper()
	completedAt := "NULL"
	if completed {
		completedAt = "NOW()"
	}
	if _, err := pool.Exec(context.Background(), `
INSERT INTO social_connection_rollout_state (id, phase, cutover_completed_at, cutover_backend_pid, updated_at)
VALUES ('global', $1, `+completedAt+`, 4242, NOW() - $2::interval)
ON CONFLICT (id) DO UPDATE SET
  phase = EXCLUDED.phase,
  cutover_completed_at = EXCLUDED.cutover_completed_at,
  cutover_backend_pid = EXCLUDED.cutover_backend_pid,
  updated_at = EXCLUDED.updated_at`, phase, age.String()); err != nil {
		t.Fatalf("seed phase: %v", err)
	}
}

func currentPhase(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var phase string
	if err := pool.QueryRow(context.Background(),
		`SELECT phase FROM social_connection_rollout_state WHERE id = 'global'`).Scan(&phase); err != nil {
		t.Fatalf("read phase: %v", err)
	}
	return phase
}

// A killed cutover leaves the phase blocking delivery claims for every
// workspace with nothing to expire it. Recovery must clear it unattended.
func TestRecoverAbandonedPhaseRestoresExpandForStuckPhases(t *testing.T) {
	pool := newAbandonedPhaseTestPool(t)
	for _, phase := range []string{"draining", "cutting_over"} {
		t.Run(phase, func(t *testing.T) {
			seedPhase(t, pool, phase, 10*time.Minute, false)
			recovered, err := RecoverAbandonedCutoverPhase(context.Background(), pool, time.Minute)
			if err != nil {
				t.Fatalf("recover: %v", err)
			}
			if !recovered {
				t.Fatal("stuck phase was not recovered")
			}
			if got := currentPhase(t, pool); got != "expand" {
				t.Fatalf("phase = %q, want expand", got)
			}
			var pid *int
			if err := pool.QueryRow(context.Background(),
				`SELECT cutover_backend_pid FROM social_connection_rollout_state WHERE id = 'global'`).Scan(&pid); err != nil {
				t.Fatalf("read pid: %v", err)
			}
			if pid != nil {
				t.Fatalf("cutover_backend_pid = %d, want NULL", *pid)
			}
		})
	}
}

// The advisory lock is the entire safety argument. While a real cutover holds
// it, recovery must decline no matter how long the drain has taken — otherwise
// this worker would race a live cutover into an inconsistent phase.
func TestRecoverAbandonedPhaseDeclinesWhileCutoverHoldsLock(t *testing.T) {
	pool := newAbandonedPhaseTestPool(t)
	seedPhase(t, pool, "cutting_over", time.Hour, false)

	ctx := context.Background()
	holder, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire holder: %v", err)
	}
	defer holder.Release()
	if _, err := holder.Exec(ctx,
		`SELECT pg_advisory_lock(hashtextextended($1, 0))`, cutoverAdvisoryLockName); err != nil {
		t.Fatalf("hold cutover lock: %v", err)
	}

	recovered, err := RecoverAbandonedCutoverPhase(ctx, pool, time.Minute)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if recovered {
		t.Fatal("recovery ran while a live cutover held the lock")
	}
	if got := currentPhase(t, pool); got != "cutting_over" {
		t.Fatalf("phase = %q, want cutting_over untouched", got)
	}

	// Releasing the lock is what a dying process does implicitly.
	if _, err := holder.Exec(ctx,
		`SELECT pg_advisory_unlock(hashtextextended($1, 0))`, cutoverAdvisoryLockName); err != nil {
		t.Fatalf("release cutover lock: %v", err)
	}
	recovered, err = RecoverAbandonedCutoverPhase(ctx, pool, time.Minute)
	if err != nil {
		t.Fatalf("recover after release: %v", err)
	}
	if !recovered {
		t.Fatal("phase was not recovered after the lock was released")
	}
}

// cutover.sql commits authority and phase = 'cutover' in one transaction, so a
// committed cutover must never be walked back to Expand.
func TestRecoverAbandonedPhaseNeverUndoesCommittedCutover(t *testing.T) {
	pool := newAbandonedPhaseTestPool(t)
	for name, phase := range map[string]string{
		"completed cutover": "cutover",
		"steady expand":     "expand",
	} {
		t.Run(name, func(t *testing.T) {
			seedPhase(t, pool, phase, time.Hour, phase == "cutover")
			recovered, err := RecoverAbandonedCutoverPhase(context.Background(), pool, time.Minute)
			if err != nil {
				t.Fatalf("recover: %v", err)
			}
			if recovered {
				t.Fatalf("recovery mutated a %s phase", name)
			}
			if got := currentPhase(t, pool); got != phase {
				t.Fatalf("phase = %q, want %q", got, phase)
			}
		})
	}
}

// A cutover that is mid-drain refreshes updated_at as it advances. The grace
// window keeps recovery from firing against a run that just started.
func TestRecoverAbandonedPhaseRespectsGraceWindow(t *testing.T) {
	pool := newAbandonedPhaseTestPool(t)
	seedPhase(t, pool, "draining", 5*time.Second, false)
	recovered, err := RecoverAbandonedCutoverPhase(context.Background(), pool, time.Minute)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if recovered {
		t.Fatal("recovery fired inside the grace window")
	}
	if got := currentPhase(t, pool); got != "draining" {
		t.Fatalf("phase = %q, want draining", got)
	}
}
