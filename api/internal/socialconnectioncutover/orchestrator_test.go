package socialconnectioncutover

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type rolloutStoreStub struct {
	phase         string
	leases        []int64
	calls         *[]string
	setPhaseError map[string]error
}

func (s *rolloutStoreStub) Acquire(context.Context) (func(context.Context) error, error) {
	*s.calls = append(*s.calls, "lock")
	return func(context.Context) error { *s.calls = append(*s.calls, "unlock"); return nil }, nil
}
func (s *rolloutStoreStub) Phase(context.Context) (string, error) { return s.phase, nil }
func (s *rolloutStoreStub) SetPhase(_ context.Context, phase string) error {
	*s.calls = append(*s.calls, "phase:"+phase)
	if err := s.setPhaseError[phase]; err != nil {
		return err
	}
	s.phase = phase
	return nil
}
func (s *rolloutStoreStub) ActiveProviderLeases(context.Context) (int64, error) {
	value := int64(0)
	if len(s.leases) > 0 {
		value = s.leases[0]
		s.leases = s.leases[1:]
	}
	*s.calls = append(*s.calls, "leases")
	return value, nil
}

type proofStub struct{ calls *[]string }

func (p proofStub) Verify(context.Context) error { *p.calls = append(*p.calls, "proof"); return nil }

type preflightStub struct{ calls *[]string }

func (p preflightStub) Run(context.Context, string) (Report, error) {
	*p.calls = append(*p.calls, "preflight")
	return Report{Counts: Counts{Accounts: 4}}, nil
}

func TestOrchestratorDrainsProvesBacksUpAndReconcilesInOrder(t *testing.T) {
	calls := make([]string, 0)
	store := &rolloutStoreStub{phase: "expand", leases: []int64{1, 0}, calls: &calls}
	orchestrator := Orchestrator{
		Store:     store,
		Proof:     proofStub{calls: &calls},
		Preflight: preflightStub{calls: &calls},
		Backup: func(_ context.Context, report Report, operation func(context.Context) error) error {
			calls = append(calls, "backup")
			if report.Counts.Accounts != 4 {
				t.Fatalf("backup report = %+v", report)
			}
			return operation(context.Background())
		},
		Reconcile:    func(context.Context, Report) error { calls = append(calls, "reconcile"); return nil },
		PollInterval: time.Millisecond,
		DrainTimeout: time.Second,
		Sleep:        func(context.Context, time.Duration) error { calls = append(calls, "sleep"); return nil },
	}

	if _, err := orchestrator.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"lock", "phase:draining", "leases", "sleep", "leases", "proof", "preflight",
		"phase:cutting_over", "backup", "reconcile", "phase:cutover", "unlock",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}

func TestOrchestratorRestoresExpandWhenReconciliationFails(t *testing.T) {
	calls := make([]string, 0)
	store := &rolloutStoreStub{phase: "expand", calls: &calls}
	wantErr := errors.New("reconciliation failed")
	orchestrator := Orchestrator{
		Store: store, Proof: proofStub{calls: &calls}, Preflight: preflightStub{calls: &calls},
		Backup: func(ctx context.Context, _ Report, operation func(context.Context) error) error {
			return operation(ctx)
		},
		Reconcile:    func(context.Context, Report) error { return wantErr },
		PollInterval: time.Millisecond, DrainTimeout: time.Second,
	}
	if _, err := orchestrator.Run(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("Run error = %v, want %v", err, wantErr)
	}
	if store.phase != "expand" {
		t.Fatalf("phase = %q, want expand", store.phase)
	}
}

func TestOrchestratorReturnsCompletedCutoverWithoutRepeatingBackup(t *testing.T) {
	calls := make([]string, 0)
	store := &rolloutStoreStub{phase: "cutover", calls: &calls}
	orchestrator := Orchestrator{Store: store}
	if _, err := orchestrator.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"lock", "unlock"}) {
		t.Fatalf("completed cutover calls = %v", calls)
	}
}

func TestOrchestratorDoesNotRestoreExpandAfterCommittedReconciliation(t *testing.T) {
	calls := make([]string, 0)
	finalizeErr := errors.New("confirmation failed")
	store := &rolloutStoreStub{
		phase:         "expand",
		calls:         &calls,
		setPhaseError: map[string]error{"cutover": finalizeErr},
	}
	orchestrator := Orchestrator{
		Store: store, Proof: proofStub{calls: &calls}, Preflight: preflightStub{calls: &calls},
		Backup: func(ctx context.Context, _ Report, operation func(context.Context) error) error {
			return operation(ctx)
		},
		Reconcile:    func(context.Context, Report) error { return nil },
		PollInterval: time.Millisecond, DrainTimeout: time.Second,
	}

	if _, err := orchestrator.Run(context.Background()); !errors.Is(err, finalizeErr) {
		t.Fatalf("Run error = %v, want %v", err, finalizeErr)
	}
	for _, call := range calls {
		if call == "phase:expand" {
			t.Fatalf("committed reconciliation was compensated to Expand: calls=%v", calls)
		}
	}
}
