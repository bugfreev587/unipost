package xcredits

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	period       WorkspacePeriod
	events       map[string]UsageEvent
	used         int64
	reserveCalls int
	lastReserve  StoreReserveRequest
}

func newFakeStore(planID string, start, end time.Time) *fakeStore {
	return &fakeStore{
		period: WorkspacePeriod{PlanID: planID, Start: start, End: end},
		events: make(map[string]UsageEvent),
	}
}

func (s *fakeStore) ResolveWorkspacePeriod(context.Context, string, time.Time) (WorkspacePeriod, error) {
	return s.period, nil
}

func (s *fakeStore) Reserve(_ context.Context, req StoreReserveRequest) (UsageEvent, error) {
	s.reserveCalls++
	s.lastReserve = req
	if existing, ok := s.events[req.IdempotencyKey]; ok {
		existing.Duplicate = true
		return existing, nil
	}
	if s.used+req.WeightedUnits > req.WeightedUnitsLimit {
		return UsageEvent{}, ErrMonthlyLimitExceeded
	}
	event := UsageEvent{
		ID:             "xue_" + req.IdempotencyKey,
		Status:         UsageStatusProvisional,
		OperationKey:   req.OperationKey,
		CatalogVersion: req.CatalogVersion,
		WeightedUnits:  req.WeightedUnits,
	}
	s.events[req.IdempotencyKey] = event
	s.used += req.WeightedUnits
	return event, nil
}

func (s *fakeStore) Finalize(_ context.Context, eventID string, finalUnits int64) error {
	for key, event := range s.events {
		if event.ID != eventID || event.Status != UsageStatusProvisional {
			continue
		}
		s.used -= event.WeightedUnits - finalUnits
		event.WeightedUnits = finalUnits
		event.Status = UsageStatusFinalized
		s.events[key] = event
		return nil
	}
	return nil
}

func (s *fakeStore) Reverse(_ context.Context, eventID string) error {
	for key, event := range s.events {
		if event.ID != eventID || event.Status != UsageStatusProvisional {
			continue
		}
		s.used -= event.WeightedUnits
		event.Status = UsageStatusReversed
		s.events[key] = event
		return nil
	}
	return nil
}

func (s *fakeStore) Snapshot(_ context.Context, _ string, _ time.Time) (Snapshot, error) {
	allowance := int64(4000)
	remaining := allowance - s.used
	return Snapshot{
		PlanID:           s.period.PlanID,
		PeriodStart:      s.period.Start,
		PeriodEnd:        s.period.End,
		MonthlyAllowance: &allowance,
		MonthlyUsed:      s.used,
		MonthlyRemaining: &remaining,
		CatalogVersion:   CatalogVersion,
	}, nil
}

func TestServiceReserveCreatesOneProvisionalEvent(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := newFakeStore("basic", now.Add(-time.Hour), now.Add(30*24*time.Hour))
	service := NewService(store)

	event, err := service.Reserve(context.Background(), ReserveRequest{
		WorkspaceID:     "ws_1",
		SocialAccountID: "sa_1",
		AppMode:         "unipost_managed_app",
		ConnectionType:  "managed",
		OperationKey:    "post.create",
		Source:          "publish",
		IdempotencyKey:  "post_1:sa_1:main",
		RequestedUnits:  15,
		Now:             now,
	})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if event.Status != UsageStatusProvisional || event.WeightedUnits != 15 {
		t.Fatalf("event = %+v", event)
	}
	if store.used != 15 {
		t.Fatalf("used = %d, want 15", store.used)
	}
}

func TestServiceReserveIsIdempotent(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := newFakeStore("basic", now, now.Add(30*24*time.Hour))
	service := NewService(store)
	req := ReserveRequest{
		WorkspaceID: "ws_1", AppMode: "unipost_managed_app", ConnectionType: "managed",
		OperationKey: "post.create", Source: "publish",
		IdempotencyKey: "same", RequestedUnits: 15, Now: now,
	}

	if _, err := service.Reserve(context.Background(), req); err != nil {
		t.Fatalf("first Reserve: %v", err)
	}
	event, err := service.Reserve(context.Background(), req)
	if err != nil {
		t.Fatalf("second Reserve: %v", err)
	}
	if !event.Duplicate {
		t.Fatal("duplicate reservation was not identified")
	}
	if store.used != 15 {
		t.Fatalf("used = %d, want 15", store.used)
	}
}

func TestServiceReserveBlocksBeforeMonthlyOverspend(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := newFakeStore("basic", now, now.Add(30*24*time.Hour))
	store.used = 3990
	service := NewService(store)

	_, err := service.Reserve(context.Background(), ReserveRequest{
		WorkspaceID: "ws_1", AppMode: "unipost_managed_app", ConnectionType: "managed",
		OperationKey: "post.create", Source: "publish",
		IdempotencyKey: "over", RequestedUnits: 15, Now: now,
	})
	if !errors.Is(err, ErrMonthlyLimitExceeded) {
		t.Fatalf("Reserve error = %v, want ErrMonthlyLimitExceeded", err)
	}
	if _, ok := store.events["over"]; ok {
		t.Fatal("limit failure must not leave an event")
	}
}

func TestServiceReserveUsesWorkspaceContractAllowance(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	allowance := int64(90000)
	store := newFakeStore("enterprise", now, now.Add(30*24*time.Hour))
	store.period.MonthlyAllowance = &allowance
	service := NewService(store)

	event, err := service.Reserve(context.Background(), ReserveRequest{
		WorkspaceID: "ws_enterprise", AppMode: "unipost_managed_app", ConnectionType: "managed",
		OperationKey: "post.create", Source: "publish",
		IdempotencyKey: "enterprise-post", RequestedUnits: 15, Now: now,
	})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if event.WeightedUnits != 15 || store.used != 15 {
		t.Fatalf("event=%+v used=%d", event, store.used)
	}
}

func TestServiceFinalizeAndReverseAreIdempotent(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := newFakeStore("basic", now, now.Add(30*24*time.Hour))
	service := NewService(store)

	finalized, err := service.Reserve(context.Background(), ReserveRequest{
		WorkspaceID: "ws_1", AppMode: "unipost_managed_app", ConnectionType: "managed",
		OperationKey: "post.create_url", Source: "publish",
		IdempotencyKey: "finalize", RequestedUnits: 200, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Finalize(context.Background(), finalized.ID, 15); err != nil {
		t.Fatal(err)
	}
	if err := service.Finalize(context.Background(), finalized.ID, 15); err != nil {
		t.Fatal(err)
	}
	if store.used != 15 {
		t.Fatalf("used after finalize = %d, want 15", store.used)
	}

	reversed, err := service.Reserve(context.Background(), ReserveRequest{
		WorkspaceID: "ws_1", AppMode: "unipost_managed_app", ConnectionType: "managed",
		OperationKey: "post.create", Source: "publish",
		IdempotencyKey: "reverse", RequestedUnits: 15, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Reverse(context.Background(), reversed.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.Reverse(context.Background(), reversed.ID); err != nil {
		t.Fatal(err)
	}
	if store.used != 15 {
		t.Fatalf("used after reverse = %d, want 15", store.used)
	}
}

func TestServiceWorkspaceAppBypassesStore(t *testing.T) {
	store := newFakeStore("basic", time.Now(), time.Now().Add(time.Hour))
	service := NewService(store)

	event, err := service.Reserve(context.Background(), ReserveRequest{
		WorkspaceID: "ws_1", AppMode: "workspace_x_app", ConnectionType: "byo",
		OperationKey: "post.create", IdempotencyKey: "byo", RequestedUnits: 15,
	})
	if err != nil {
		t.Fatal(err)
	}
	if event.WeightedUnits != 0 || event.Status != UsageStatusBypassed {
		t.Fatalf("event = %+v", event)
	}
	if store.reserveCalls != 0 {
		t.Fatalf("reserve calls = %d, want 0", store.reserveCalls)
	}
}

func TestXUsageUsesPersistedAppModeRegardlessOfConnectionType(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name           string
		appMode        string
		connectionType string
		wantStatus     string
		wantCalls      int
	}{
		{
			name:           "UniPost app meters a native BYO-owned account",
			appMode:        "unipost_managed_app",
			connectionType: "byo",
			wantStatus:     UsageStatusProvisional,
			wantCalls:      1,
		},
		{
			name:           "workspace app bypasses a Hosted Connect managed account",
			appMode:        "workspace_x_app",
			connectionType: "managed",
			wantStatus:     UsageStatusBypassed,
			wantCalls:      0,
		},
		{
			name:           "ambiguous legacy app bypasses credits",
			appMode:        "legacy_unknown",
			connectionType: "byo",
			wantStatus:     UsageStatusBypassed,
			wantCalls:      0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore("basic", now, now.Add(30*24*time.Hour))
			service := NewService(store)
			event, err := service.Reserve(context.Background(), ReserveRequest{
				WorkspaceID:     "ws_1",
				SocialAccountID: "sa_1",
				AppMode:         tt.appMode,
				ConnectionType:  tt.connectionType,
				OperationKey:    "post.create",
				IdempotencyKey:  "mode-test",
				RequestedUnits:  15,
				Now:             now,
			})
			if err != nil {
				t.Fatal(err)
			}
			if event.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", event.Status, tt.wantStatus)
			}
			if store.reserveCalls != tt.wantCalls {
				t.Fatalf("reserve calls = %d, want %d", store.reserveCalls, tt.wantCalls)
			}
			if tt.wantCalls == 1 && store.lastReserve.AppMode != tt.appMode {
				t.Fatalf("persisted app mode = %q, want %q", store.lastReserve.AppMode, tt.appMode)
			}
		})
	}
}

func TestXUsageBlankPersistedAppModeUsesLegacyBypass(t *testing.T) {
	store := newFakeStore("basic", time.Now(), time.Now().Add(time.Hour))
	service := NewService(store)
	event, err := service.Reserve(context.Background(), ReserveRequest{
		WorkspaceID:    "ws_1",
		AppMode:        "",
		OperationKey:   "post.create",
		IdempotencyKey: "legacy-null",
	})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if event.Status != UsageStatusBypassed || store.reserveCalls != 0 {
		t.Fatalf("event=%+v reserve calls=%d, want legacy bypass", event, store.reserveCalls)
	}
}

func TestXUsageRejectsInvalidPersistedAppMode(t *testing.T) {
	for _, appMode := range []string{"managed", "garbage"} {
		t.Run(appMode, func(t *testing.T) {
			store := newFakeStore("basic", time.Now(), time.Now().Add(time.Hour))
			service := NewService(store)
			if _, err := service.Reserve(context.Background(), ReserveRequest{
				WorkspaceID:    "ws_1",
				AppMode:        appMode,
				OperationKey:   "post.create",
				IdempotencyKey: "invalid-mode",
			}); err == nil {
				t.Fatalf("Reserve app_mode=%q error = nil, want validation error", appMode)
			}
			if store.reserveCalls != 0 {
				t.Fatalf("reserve calls = %d, want 0", store.reserveCalls)
			}
		})
	}
}

func TestCalendarMonthPeriodFallback(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.FixedZone("PDT", -7*60*60))
	start, end := CalendarMonthPeriod(now)
	if want := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC); !start.Equal(want) {
		t.Fatalf("start = %s, want %s", start, want)
	}
	if want := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC); !end.Equal(want) {
		t.Fatalf("end = %s, want %s", end, want)
	}
}

func TestShouldSkipUsageSettlementDoesNotSwallowQueryErrors(t *testing.T) {
	queryErr := errors.New("database connection reset")
	skip, err := shouldSkipUsageSettlement(queryErr, "")
	if skip {
		t.Fatal("skip = true, want false for a database query error")
	}
	if !errors.Is(err, queryErr) {
		t.Fatalf("error = %v, want %v", err, queryErr)
	}
}

func TestPostgresReserveUsesRowSerializationWithoutSerializableFailures(t *testing.T) {
	source, err := os.ReadFile("postgres.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if strings.Contains(text, "pgx.Serializable") {
		t.Fatal("Reserve must not use SERIALIZABLE without a retry loop")
	}
	insert := strings.Index(text, "INSERT INTO x_usage_events")
	increment := strings.Index(text, "SET weighted_units_used = weighted_units_used + $4")
	if insert < 0 || increment < 0 || insert >= increment {
		t.Fatalf("event insert index=%d period increment index=%d", insert, increment)
	}
	if !strings.Contains(text, "ON CONFLICT (workspace_id, idempotency_key) DO NOTHING") {
		t.Fatal("concurrent duplicate reservations must converge on one usage event")
	}
}

type fakeExposureStore struct {
	*fakeStore
	markedID      string
	reconcileCall bool
	reconcileStat ExposureReleaseReconcileStats
}

func (s *fakeExposureStore) ReserveExposure(
	context.Context,
	StoreExposureReservationRequest,
) (ExposureReservation, error) {
	return ExposureReservation{}, nil
}
func (s *fakeExposureStore) MarkExposureReadStarted(context.Context, string) error { return nil }
func (s *fakeExposureStore) MarkExposureFinalizePending(
	context.Context,
	string,
	int64,
	string,
) error {
	return nil
}
func (s *fakeExposureStore) FinalizeExposure(context.Context, string, int64) error { return nil }
func (s *fakeExposureStore) ReleaseExposure(context.Context, string) error         { return nil }
func (s *fakeExposureStore) MarkExposureReleasePending(
	_ context.Context,
	id string,
	_ string,
) error {
	s.markedID = id
	return nil
}
func (s *fakeExposureStore) MarkExposureNeedsReconciliation(
	context.Context,
	string,
	string,
) error {
	return nil
}
func (s *fakeExposureStore) ReconcilePendingExposures(
	context.Context,
	int,
	time.Time,
) (ExposureReleaseReconcileStats, error) {
	s.reconcileCall = true
	return s.reconcileStat, nil
}

func TestExposureReleasePendingIsPersistedAndReconciled(t *testing.T) {
	store := &fakeExposureStore{
		fakeStore:     newFakeStore("basic", time.Now(), time.Now().Add(time.Hour)),
		reconcileStat: ExposureReleaseReconcileStats{Scanned: 1, Released: 1},
	}
	service := NewService(store)
	if err := service.MarkExposureReleasePending(
		context.Background(), "reservation-1", "release failed",
	); err != nil {
		t.Fatalf("MarkExposureReleasePending: %v", err)
	}
	stats, err := service.ReconcilePendingExposures(
		context.Background(), 100, time.Now(),
	)
	if err != nil {
		t.Fatalf("ReconcilePendingExposures: %v", err)
	}
	if store.markedID != "reservation-1" || !store.reconcileCall ||
		stats.Released != 1 {
		t.Fatalf("store/stats = %+v %+v", store, stats)
	}
}
