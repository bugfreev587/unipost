package handler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
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

func TestRetryXInboxStatePersistenceHealsTransientDatabaseFailure(t *testing.T) {
	attempts := 0
	err := retryXInboxStatePersistence(context.Background(), func() error {
		attempts++
		if attempts < 3 {
			return errors.New("database unavailable")
		}
		return nil
	})
	if err != nil || attempts != 3 {
		t.Fatalf("err/attempts = %v/%d", err, attempts)
	}
}

func TestRetryXInboxStatePersistenceDoesNotRetryTransitionConflict(t *testing.T) {
	attempts := 0
	err := retryXInboxStatePersistence(context.Background(), func() error {
		attempts++
		return errXInboxStateTransitionConflict
	})
	if !errors.Is(err, errXInboxStateTransitionConflict) || attempts != 1 {
		t.Fatalf("err/attempts = %v/%d", err, attempts)
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

func TestXInboxOutboundLegacyPendingClaimBecomesManualReconciliation(t *testing.T) {
	store := &fakeXInboxOutboundRecoveryStore{
		rows: []db.XInboxOutboundRequest{{ID: "legacy-claim", Status: "pending"}},
	}
	stats, err := newXInboxOutboundRecoveryService(store, time.Now).
		ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("ProcessOnce: %v", err)
	}
	if stats.NeedsReconciliation != 1 || store.manualCalls != 1 || store.upstreamCalls != 0 {
		t.Fatalf("stats/store = %+v %+v", stats, store)
	}
}

func TestXInboxOutboundRecoveryReversesStaleModernPendingClaimWithoutCallingX(t *testing.T) {
	store := &fakeXInboxOutboundRecoveryStore{
		rows: []db.XInboxOutboundRequest{{
			ID:               "modern-pending",
			WorkspaceID:      "workspace-1",
			InboxItemID:      "item-1",
			IdempotencyKey:   "client-key",
			Status:           "pending",
			EncryptedPayload: pgtype.Text{String: "ciphertext", Valid: true},
		}},
		claimPending: true,
	}
	stats, err := newXInboxOutboundRecoveryService(store, time.Now).
		ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("ProcessOnce: %v", err)
	}
	if stats.UsageReversed != 1 || store.reversePendingCalls != 1 ||
		store.manualCalls != 0 || store.upstreamCalls != 0 {
		t.Fatalf("stats/store = %+v %+v", stats, store)
	}
}

func TestXInboxOutboundRecoveryClaimFencesConcurrentSendingBeforeUsageReversal(t *testing.T) {
	store := newConcurrentPendingRecoveryStore()
	service := newXInboxOutboundRecoveryService(store, time.Now)
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-store.claimed
		if store.markSending() {
			store.upstreamCalls.Add(1)
		}
	}()
	stats, err := service.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("ProcessOnce: %v", err)
	}
	<-done
	if stats.UsageReversed != 1 || !store.reversed.Load() {
		t.Fatalf("stats/reversed = %+v/%v", stats, store.reversed.Load())
	}
	if store.upstreamCalls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", store.upstreamCalls.Load())
	}
}

func TestXInboxOutboundRecoveryDoesNotReverseWhenLiveSendingWinsCAS(t *testing.T) {
	store := &fakeXInboxOutboundRecoveryStore{
		rows: []db.XInboxOutboundRequest{{
			ID: "modern-pending", Status: "pending",
			EncryptedPayload: pgtype.Text{String: "ciphertext", Valid: true},
		}},
		claimPending: false,
	}
	stats, err := newXInboxOutboundRecoveryService(store, time.Now).
		ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("ProcessOnce: %v", err)
	}
	if stats.UsageReversed != 0 || store.reversePendingCalls != 0 {
		t.Fatalf("stats/store = %+v %+v", stats, store)
	}
}

func TestXInboxOutboundRecoveryDefersModernPendingWhenUsageLookupFails(t *testing.T) {
	store := &fakeXInboxOutboundRecoveryStore{
		rows: []db.XInboxOutboundRequest{{
			ID: "modern-pending", Status: "pending",
			EncryptedPayload: pgtype.Text{String: "ciphertext", Valid: true},
		}},
		reversePendingErr: errors.New("database unavailable"),
		claimPending:      true,
	}
	stats, err := newXInboxOutboundRecoveryService(store, time.Now).
		ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("ProcessOnce: %v", err)
	}
	if stats.Deferred != 1 || store.deferPendingCalls != 1 || store.upstreamCalls != 0 {
		t.Fatalf("stats/store = %+v %+v", stats, store)
	}
}

func TestXInboxOutboundUsageKeyMatchesReservationKey(t *testing.T) {
	row := db.XInboxOutboundRequest{InboxItemID: "item-1", IdempotencyKey: "client-key"}
	if got := xInboxOutboundUsageIdempotencyKey(row); got != "inbox:item-1:client-key" {
		t.Fatalf("usage key = %q", got)
	}
}

func TestXInboxOutboundRecoveryRetriesUsageReversalWithoutCallingX(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := &fakeXInboxOutboundRecoveryStore{
		rows: []db.XInboxOutboundRequest{{
			ID:           "request-1",
			Status:       "usage_reversal_pending",
			UsageEventID: pgtype.Text{String: "usage-1", Valid: true},
		}},
		reverseErrs: []error{errors.New("database unavailable"), nil},
	}
	service := newXInboxOutboundRecoveryService(store, func() time.Time { return now })
	first, err := service.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("first ProcessOnce: %v", err)
	}
	if first.Deferred != 1 || store.reverseCalls != 1 || store.deferReversalCalls != 1 {
		t.Fatalf("first stats/store = %+v %+v", first, store)
	}
	second, err := service.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("second ProcessOnce: %v", err)
	}
	if second.UsageReversed != 1 || store.reverseCalls != 2 || store.upstreamCalls != 0 {
		t.Fatalf("second stats/store = %+v %+v", second, store)
	}
}

type fakeXInboxOutboundRecoveryStore struct {
	rows                []db.XInboxOutboundRequest
	completeErrs        []error
	completeCalls       int
	deferCalls          int
	manualCalls         int
	reverseErrs         []error
	reverseCalls        int
	reversePendingCalls int
	reversePendingErr   error
	deferPendingCalls   int
	deferReversalCalls  int
	upstreamCalls       int
	claimPending        bool
	claimPendingCalls   int
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

func (f *fakeXInboxOutboundRecoveryStore) ReverseUsage(
	context.Context,
	db.XInboxOutboundRequest,
) error {
	f.reverseCalls++
	if len(f.reverseErrs) == 0 {
		return nil
	}
	err := f.reverseErrs[0]
	f.reverseErrs = f.reverseErrs[1:]
	return err
}

func (f *fakeXInboxOutboundRecoveryStore) ReversePendingUsage(
	context.Context,
	db.XInboxOutboundRequest,
) error {
	f.reversePendingCalls++
	return f.reversePendingErr
}

func (f *fakeXInboxOutboundRecoveryStore) ClaimPendingRecovery(
	context.Context,
	string,
) (bool, error) {
	f.claimPendingCalls++
	return f.claimPending, nil
}

func (f *fakeXInboxOutboundRecoveryStore) DeferPending(
	context.Context,
	string,
	time.Time,
	string,
) error {
	f.deferPendingCalls++
	return nil
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

func (f *fakeXInboxOutboundRecoveryStore) DeferUsageReversal(
	context.Context,
	string,
	time.Time,
	string,
) error {
	f.deferReversalCalls++
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

type concurrentPendingRecoveryStore struct {
	mu            sync.Mutex
	status        string
	claimed       chan struct{}
	claimOnce     sync.Once
	reversed      atomic.Bool
	upstreamCalls atomic.Int64
}

func newConcurrentPendingRecoveryStore() *concurrentPendingRecoveryStore {
	return &concurrentPendingRecoveryStore{
		status:  "pending",
		claimed: make(chan struct{}),
	}
}

func (s *concurrentPendingRecoveryStore) ListRecoverable(
	context.Context,
	int32,
) ([]db.XInboxOutboundRequest, error) {
	return []db.XInboxOutboundRequest{{
		ID: "request-1", Status: "pending",
		EncryptedPayload: pgtype.Text{String: "ciphertext", Valid: true},
	}}, nil
}

func (s *concurrentPendingRecoveryStore) ClaimPendingRecovery(
	context.Context,
	string,
) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status != "pending" {
		return false, nil
	}
	s.status = "pending_recovery"
	s.claimOnce.Do(func() { close(s.claimed) })
	return true, nil
}

func (s *concurrentPendingRecoveryStore) markSending() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status != "pending" {
		return false
	}
	s.status = "sending"
	return true
}

func (s *concurrentPendingRecoveryStore) ReversePendingUsage(
	context.Context,
	db.XInboxOutboundRequest,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status != "pending_recovery" {
		return errors.New("usage reversal ran without owning pending recovery")
	}
	s.reversed.Store(true)
	return nil
}

func (*concurrentPendingRecoveryStore) CompleteKnown(context.Context, string) error { return nil }
func (*concurrentPendingRecoveryStore) ReverseUsage(context.Context, db.XInboxOutboundRequest) error {
	return nil
}
func (*concurrentPendingRecoveryStore) Defer(context.Context, string, time.Time, string) error {
	return nil
}
func (*concurrentPendingRecoveryStore) DeferPending(context.Context, string, time.Time, string) error {
	return nil
}
func (*concurrentPendingRecoveryStore) DeferUsageReversal(context.Context, string, time.Time, string) error {
	return nil
}
func (*concurrentPendingRecoveryStore) MarkNeedsReconciliation(context.Context, string, string) error {
	return nil
}
