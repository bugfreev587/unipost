package xcredits

import (
	"context"
	"testing"
	"time"
)

type rolloutEvaluator bool

func (e rolloutEvaluator) ForWorkspace(context.Context, string, string) (bool, error) {
	return bool(e), nil
}

type rolloutExposureStore struct {
	*fakeStore
	last StoreExposureReservationRequest
}

func (s *rolloutExposureStore) ReserveExposure(_ context.Context, req StoreExposureReservationRequest) (ExposureReservation, error) {
	s.last = req
	return ExposureReservation{ID: "exposure_1", ReservedResources: req.RequestedResources}, nil
}
func (s *rolloutExposureStore) MarkExposureReadStarted(context.Context, string) error { return nil }
func (s *rolloutExposureStore) MarkExposureFinalizePending(context.Context, string, int64, string) error {
	return nil
}
func (s *rolloutExposureStore) FinalizeExposure(context.Context, string, int64) error { return nil }
func (s *rolloutExposureStore) ReleaseExposure(context.Context, string) error         { return nil }
func (s *rolloutExposureStore) MarkExposureReleasePending(context.Context, string, string) error {
	return nil
}
func (s *rolloutExposureStore) MarkExposureNeedsReconciliation(context.Context, string, string) error {
	return nil
}
func (s *rolloutExposureStore) ReconcilePendingExposures(context.Context, int, time.Time) (ExposureReleaseReconcileStats, error) {
	return ExposureReleaseReconcileStats{}, nil
}

func TestRolloutServiceBypassesOutboundCustomerAccountingWhenOff(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	store := newFakeStore("basic", now, now.AddDate(0, 1, 0))
	service := NewRolloutService(NewService(store), rolloutEvaluator(false))

	event, err := service.Reserve(context.Background(), ReserveRequest{
		WorkspaceID: "ws_1", AppMode: "unipost_managed_app",
		OperationKey: "post.create", IdempotencyKey: "publish_1", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if event.Status != UsageStatusBypassed || store.reserveCalls != 0 || store.used != 0 {
		t.Fatalf("event=%+v reserve_calls=%d used=%d", event, store.reserveCalls, store.used)
	}
}

func TestRolloutServiceKeepsFullAccountingForEnabledWorkspace(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	store := newFakeStore("basic", now, now.AddDate(0, 1, 0))
	service := NewRolloutService(NewService(store), rolloutEvaluator(true))

	event, err := service.Reserve(context.Background(), ReserveRequest{
		WorkspaceID: "ws_super", AppMode: "unipost_managed_app",
		OperationKey: "post.create", IdempotencyKey: "publish_1", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if event.Status != UsageStatusProvisional || store.reserveCalls != 1 {
		t.Fatalf("event=%+v reserve_calls=%d", event, store.reserveCalls)
	}
}

func TestRolloutServiceInboundOffUsesSafetyOnlyAccounting(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	store := newFakeInboundStore("basic", now)
	service := NewRolloutService(NewService(store), rolloutEvaluator(false))

	admission, err := service.AdmitInbound(context.Background(), InboundRequest{
		WorkspaceID: "ws_1", SocialAccountID: "sa_1", AppMode: "unipost_managed_app",
		OperationKey: "post.mention.received", Source: "stream",
		UpstreamResourceType: "x_reply", UpstreamResourceID: "reply_1", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if admission.Decision != InboundDecisionAccepted || store.lastInbound.AccountingEnabled {
		t.Fatalf("admission=%+v request=%+v", admission, store.lastInbound)
	}
	if store.used != 0 || store.dailyUsed == 0 {
		t.Fatalf("monthly used=%d daily safety used=%d, want only daily usage", store.used, store.dailyUsed)
	}
}

func TestRolloutSnapshotOffIgnoresMonthlyPauseButKeepsInboundSafetyPause(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	store := newFakeInboundStore("basic", now)
	store.used = 4000
	store.dailyUsed = 390
	service := NewRolloutService(NewService(store), rolloutEvaluator(false))

	snapshot, err := service.Snapshot(context.Background(), "ws_1", now)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.MonthlyAllowance != nil || snapshot.MonthlyRemaining != nil || snapshot.MonthlyUsed != 0 {
		t.Fatalf("monthly customer accounting leaked into disabled snapshot: %+v", snapshot)
	}
	if !snapshot.PausePaidSources || snapshot.InboundPauseReason != PauseReasonDailySafetyBuffer {
		t.Fatalf("snapshot=%+v, want inbound safety pause", snapshot)
	}
}

func TestRolloutExposureOffReservesOnlySafetyCapacity(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	store := &rolloutExposureStore{fakeStore: newFakeStore("basic", now, now.AddDate(0, 1, 0))}
	service := NewRolloutService(NewService(store), rolloutEvaluator(false))

	_, err := service.ReserveExposure(context.Background(), ExposureReservationRequest{
		WorkspaceID: "ws_1", SocialAccountID: "sa_1", AppMode: "unipost_managed_app",
		OperationKey: "post.read", IdempotencyKey: "backfill_1",
		RequestedResources: 5, MinimumResources: 5, UnitsPerResource: 15, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.last.AccountingEnabled {
		t.Fatalf("request=%+v, want safety-only reservation", store.last)
	}
	if store.last.InboundDailyLimit == 0 {
		t.Fatalf("request=%+v, want internal inbound limit", store.last)
	}
}
