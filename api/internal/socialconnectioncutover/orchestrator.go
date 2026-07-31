package socialconnectioncutover

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type RolloutStore interface {
	Acquire(context.Context) (func(context.Context) error, error)
	Phase(context.Context) (string, error)
	SetPhase(context.Context, string) error
	ActiveProviderLeases(context.Context) (int64, error)
}

type DeploymentVerifier interface {
	Verify(context.Context) error
}

type PreflightRunner interface {
	Run(context.Context, string) (Report, error)
}

type BackupOperation func(context.Context, Report, func(context.Context) error) error
type ReconcileOperation func(context.Context, Report) error
type SleepOperation func(context.Context, time.Duration) error

type Orchestrator struct {
	Store        RolloutStore
	Proof        DeploymentVerifier
	Preflight    PreflightRunner
	Backup       BackupOperation
	Reconcile    ReconcileOperation
	PollInterval time.Duration
	DrainTimeout time.Duration
	Sleep        SleepOperation
}

func (o Orchestrator) Run(ctx context.Context) (report Report, runErr error) {
	if o.Store == nil {
		return Report{}, errors.New("social connection rollout store is required")
	}
	release, err := o.Store.Acquire(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("acquire social connection cutover lock: %w", err)
	}
	defer func() {
		if releaseErr := release(context.Background()); runErr == nil && releaseErr != nil {
			runErr = fmt.Errorf("release social connection cutover lock: %w", releaseErr)
		}
	}()

	phase, err := o.Store.Phase(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("read social connection rollout phase: %w", err)
	}
	if phase == "cutover" {
		return Report{Mode: "cutover"}, nil
	}
	if phase != "expand" {
		return Report{}, fmt.Errorf("social connection rollout phase %q cannot start cutover", phase)
	}
	if o.Proof == nil || o.Preflight == nil || o.Backup == nil || o.Reconcile == nil {
		return Report{}, errors.New("cutover proof, preflight, backup, and reconciliation are required")
	}

	phaseChanged := false
	completed := false
	defer func() {
		if !phaseChanged || completed {
			return
		}
		compensationCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := o.Store.SetPhase(compensationCtx, "expand"); err != nil && runErr == nil {
			runErr = fmt.Errorf("restore Expand phase after cutover failure: %w", err)
		}
	}()

	if err := o.Store.SetPhase(ctx, "draining"); err != nil {
		return Report{}, fmt.Errorf("enter social connection delivery drain: %w", err)
	}
	phaseChanged = true

	pollInterval := o.PollInterval
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	drainTimeout := o.DrainTimeout
	if drainTimeout <= 0 {
		drainTimeout = 5 * time.Minute
	}
	sleep := o.Sleep
	if sleep == nil {
		sleep = sleepWithContext
	}
	deadline := time.Now().Add(drainTimeout)
	for {
		active, err := o.Store.ActiveProviderLeases(ctx)
		if err != nil {
			return Report{}, fmt.Errorf("count active provider leases: %w", err)
		}
		if active == 0 {
			break
		}
		if !time.Now().Before(deadline) {
			return Report{}, fmt.Errorf("delivery drain timed out with %d active provider leases", active)
		}
		if err := sleep(ctx, pollInterval); err != nil {
			return Report{}, fmt.Errorf("wait for delivery drain: %w", err)
		}
	}

	if err := o.Proof.Verify(ctx); err != nil {
		return Report{}, err
	}
	report, err = o.Preflight.Run(ctx, "cutover")
	if err != nil {
		return Report{}, err
	}
	if report.HasBlockers() {
		return report, fmt.Errorf("social connection preflight has %d blocker(s)", len(report.Blockers))
	}
	if err := o.Store.SetPhase(ctx, "cutting_over"); err != nil {
		return report, fmt.Errorf("enter social connection cutover phase: %w", err)
	}
	if err := o.Backup(ctx, report, func(operationCtx context.Context) error {
		return o.Reconcile(operationCtx, report)
	}); err != nil {
		return report, err
	}
	// Reconciliation commits both the authority changes and the durable cutover
	// phase in one transaction. From this point onward it is unsafe to compensate
	// back to Expand even if the best-effort phase confirmation below fails.
	completed = true
	if err := o.Store.SetPhase(ctx, "cutover"); err != nil {
		return report, fmt.Errorf("finalize social connection rollout phase: %w", err)
	}
	return report, nil
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
