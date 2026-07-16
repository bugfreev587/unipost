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
		WorkspaceID: "ws_1", ConnectionType: "managed",
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
		WorkspaceID: "ws_1", ConnectionType: "managed",
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
		WorkspaceID: "ws_enterprise", ConnectionType: "managed",
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
		WorkspaceID: "ws_1", ConnectionType: "managed",
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
		WorkspaceID: "ws_1", ConnectionType: "managed",
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

func TestServiceBYOBypassesStore(t *testing.T) {
	store := newFakeStore("basic", time.Now(), time.Now().Add(time.Hour))
	service := NewService(store)

	event, err := service.Reserve(context.Background(), ReserveRequest{
		WorkspaceID: "ws_1", ConnectionType: "byo",
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
