package worker

import (
	"context"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/connect"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platformcontrols"
)

type blockedPlatformControlEvaluator struct {
	calls int
	last  string
	op    platformcontrols.Operation
}

func (e *blockedPlatformControlEvaluator) Evaluate(_ context.Context, platformName string, operation platformcontrols.Operation) (platformcontrols.Decision, error) {
	e.calls++
	e.last = platformName
	e.op = operation
	return platformcontrols.Decision{Platform: platformName, State: platformcontrols.StateFullyDisabled, Operation: operation, Blocked: true}, nil
}

// TestNewManagedTokenRefreshWorker — basic constructor sanity.
func TestNewManagedTokenRefreshWorker(t *testing.T) {
	w := NewManagedTokenRefreshWorker(nil, nil, connect.NewRegistry(), nil, "")
	if w == nil {
		t.Fatal("constructor returned nil")
	}
	if w.tickInterval != 5*time.Minute {
		t.Errorf("tickInterval: got %v, want 5m", w.tickInterval)
	}
	if w.bus == nil {
		t.Error("bus must default to NoopBus when nil is passed")
	}
}

func TestManagedTokenRefreshFullyDisabledStopsBeforeTokenOrConnector(t *testing.T) {
	evaluator := &blockedPlatformControlEvaluator{}
	w := NewManagedTokenRefreshWorker(nil, nil, nil, nil, "").SetPlatformOperationalControls(evaluator)
	w.refreshOne(context.Background(), db.SocialAccount{ID: "tk_1", Platform: "tiktok"})
	if evaluator.calls != 1 || evaluator.last != "tiktok" || evaluator.op != platformcontrols.OperationRead {
		t.Fatalf("evaluation=%+v", evaluator)
	}
}

// Note: end-to-end refresh testing — including the
// FOR UPDATE SKIP LOCKED concurrency contract — happens in the
// Sprint 3 PR10 integration test against a real Postgres. Pure
// unit tests can't exercise the locking semantics meaningfully.
