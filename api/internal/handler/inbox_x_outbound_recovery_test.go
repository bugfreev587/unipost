package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestDetachedXInboxCompletionSurvivesClientDisconnect(t *testing.T) {
	parent, parentCancel := context.WithCancel(context.Background())
	parentCancel()
	ctx, cancel := detachedXInboxCompletionContext(parent)
	defer cancel()
	if err := ctx.Err(); err != nil {
		t.Fatalf("detached context inherited client cancellation: %v", err)
	}
}

func TestXInboxOutboundRecoveryHealsTransientCompletionFailureWithoutResend(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := &fakeXInboxOutboundRecoveryStore{
		rows: []db.XInboxOutboundRequest{{
			ID:               "request-1",
			Status:           "remote_succeeded",
			RemoteExternalID: pgtype.Text{String: "tweet-2", Valid: true},
		}},
		completeErrs: []error{errors.New("database unavailable"), nil},
	}
	service := newXInboxOutboundRecoveryService(store, func() time.Time { return now })
	first, err := service.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("first ProcessOnce: %v", err)
	}
	if first.Deferred != 1 || store.completeCalls != 1 || store.deferCalls != 1 {
		t.Fatalf("first stats/store = %+v %+v", first, store)
	}
	second, err := service.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("second ProcessOnce: %v", err)
	}
	if second.Completed != 1 || store.completeCalls != 2 {
		t.Fatalf("second stats/store = %+v %+v", second, store)
	}
	if store.upstreamCalls != 0 {
		t.Fatalf("recovery attempted upstream resend: %d", store.upstreamCalls)
	}
}

func TestXInboxOutboundUnknownOutcomeBecomesManualReconciliationAfterDeadline(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := &fakeXInboxOutboundRecoveryStore{
		rows: []db.XInboxOutboundRequest{{
			ID:                     "request-1",
			Status:                 "outcome_unknown",
			ReconciliationDeadline: pgtype.Timestamptz{Time: now.Add(-time.Second), Valid: true},
		}},
	}
	stats, err := newXInboxOutboundRecoveryService(store, func() time.Time { return now }).
		ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("ProcessOnce: %v", err)
	}
	if stats.NeedsReconciliation != 1 || store.manualCalls != 1 || store.upstreamCalls != 0 {
		t.Fatalf("stats/store = %+v %+v", stats, store)
	}
}

type fakeXInboxOutboundRecoveryStore struct {
	rows          []db.XInboxOutboundRequest
	completeErrs  []error
	completeCalls int
	deferCalls    int
	manualCalls   int
	upstreamCalls int
}

func (f *fakeXInboxOutboundRecoveryStore) ListRecoverable(
	context.Context,
	int32,
) ([]db.XInboxOutboundRequest, error) {
	return f.rows, nil
}

func (f *fakeXInboxOutboundRecoveryStore) CompleteKnown(context.Context, string) error {
	f.completeCalls++
	if len(f.completeErrs) == 0 {
		return nil
	}
	err := f.completeErrs[0]
	f.completeErrs = f.completeErrs[1:]
	return err
}

func (f *fakeXInboxOutboundRecoveryStore) Defer(
	context.Context,
	string,
	time.Time,
	string,
) error {
	f.deferCalls++
	return nil
}

func (f *fakeXInboxOutboundRecoveryStore) MarkNeedsReconciliation(
	context.Context,
	string,
	string,
) error {
	f.manualCalls++
	return nil
}
