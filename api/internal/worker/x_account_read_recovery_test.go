package worker

import (
	"context"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/xaccountreads"
)

type fakeXAccountReadRecovery struct{ calls int }

func (f *fakeXAccountReadRecovery) Reconcile(context.Context, int, time.Time) (xaccountreads.ReconcileStats, error) {
	f.calls++
	return xaccountreads.ReconcileStats{Scanned: 1, Succeeded: 1}, nil
}

func TestXAccountReadRecoveryWorkerRunsSharedReconciler(t *testing.T) {
	service := &fakeXAccountReadRecovery{}
	worker := NewXAccountReadRecoveryWorker(service)
	worker.runOnce(context.Background())
	if service.calls != 1 {
		t.Fatalf("calls=%d", service.calls)
	}
}
