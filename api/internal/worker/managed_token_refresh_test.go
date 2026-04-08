package worker

import (
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/connect"
)

// TestNewManagedTokenRefreshWorker — basic constructor sanity.
func TestNewManagedTokenRefreshWorker(t *testing.T) {
	w := NewManagedTokenRefreshWorker(nil, nil, connect.NewRegistry(), nil)
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

// Note: end-to-end refresh testing — including the
// FOR UPDATE SKIP LOCKED concurrency contract — happens in the
// Sprint 3 PR10 integration test against a real Postgres. Pure
// unit tests can't exercise the locking semantics meaningfully.
