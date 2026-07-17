package xcredits

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type fakeInboundStore struct {
	*fakeStore
	mu          sync.Mutex
	receipts    map[string]InboundAdmission
	dailyUsed   int64
	accepted    int64
	suppressed  int64
	customLimit *int64
	lastInbound StoreInboundRequest
}

func newFakeInboundStore(planID string, now time.Time) *fakeInboundStore {
	return &fakeInboundStore{
		fakeStore: newFakeStore(planID, now, now.AddDate(0, 1, 0)),
		receipts:  make(map[string]InboundAdmission),
	}
}

func (s *fakeInboundStore) AdmitInbound(_ context.Context, req StoreInboundRequest) (InboundAdmission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastInbound = req

	key := strings.Join([]string{
		req.WorkspaceID,
		req.SocialAccountID,
		req.UpstreamResourceType,
		req.UpstreamResourceID,
	}, ":")
	if existing, ok := s.receipts[key]; ok {
		existing.Duplicate = true
		return existing, nil
	}

	limit := req.InboundDailyLimit
	if s.customLimit != nil {
		limit = *s.customLimit
	}
	admission := InboundAdmission{
		Decision:          InboundDecisionAccepted,
		WeightedUnits:     req.WeightedUnits,
		InboundDailyLimit: limit,
		ResetAt:           req.UTCDate.AddDate(0, 0, 1),
	}
	switch {
	case s.dailyUsed+req.WeightedUnits > limit:
		admission.Decision = InboundDecisionSuppressedDailyCap
		s.suppressed++
		admission.Claimed100Percent = true
	case req.AccountingEnabled && s.used+req.WeightedUnits > req.MonthlyAllowance:
		admission.Decision = InboundDecisionSuppressedMonthlyAllowance
		s.suppressed++
	default:
		s.dailyUsed += req.WeightedUnits
		if req.AccountingEnabled {
			s.used += req.WeightedUnits
		}
		s.accepted++
		if limit > 0 && s.dailyUsed*100 >= limit*80 {
			admission.Claimed80Percent = true
		}
		if limit > 0 && s.dailyUsed >= limit {
			admission.Claimed100Percent = true
		}
	}
	admission.InboundDailyUsed = s.dailyUsed
	admission.EventsAccepted = s.accepted
	admission.EventsSuppressed = s.suppressed
	admission.MonthlyUsed = s.used
	admission.MonthlyRemaining = req.MonthlyAllowance - s.used
	admission.PausePaidSources = remainingWithinSafetyBuffer(s.dailyUsed, limit)
	if admission.Decision == InboundDecisionSuppressedMonthlyAllowance {
		admission.PausePaidSources = true
		admission.PauseReason = PauseReasonMonthlyAllowance
	} else if admission.PausePaidSources {
		admission.PauseReason = PauseReasonDailySafetyBuffer
	}
	s.receipts[key] = admission
	return admission, nil
}

func (s *fakeInboundStore) AdmitInboundWithMutation(
	ctx context.Context,
	req StoreInboundRequest,
	mutation InboundMutation,
) (InboundAdmission, error) {
	s.mu.Lock()
	receiptsBefore := make(map[string]InboundAdmission, len(s.receipts))
	for key, receipt := range s.receipts {
		receiptsBefore[key] = receipt
	}
	usedBefore := s.used
	dailyBefore := s.dailyUsed
	acceptedBefore := s.accepted
	suppressedBefore := s.suppressed
	s.mu.Unlock()

	admission, err := s.AdmitInbound(ctx, req)
	if err != nil {
		return InboundAdmission{}, err
	}
	if admission.Decision == InboundDecisionAccepted && mutation != nil {
		if err := mutation(ctx, nil); err != nil {
			s.mu.Lock()
			s.receipts = receiptsBefore
			s.used = usedBefore
			s.dailyUsed = dailyBefore
			s.accepted = acceptedBefore
			s.suppressed = suppressedBefore
			s.mu.Unlock()
			return InboundAdmission{}, err
		}
	}
	return admission, nil
}

func (s *fakeInboundStore) RunInboundMutation(ctx context.Context, mutation InboundMutation) error {
	if mutation == nil {
		return nil
	}
	return mutation(ctx, nil)
}

func (s *fakeInboundStore) UpdateInboundCap(_ context.Context, req StoreUpdateInboundCapRequest) (InboundCapSetting, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current := req.DefaultInboundDailyLimit
	if s.customLimit != nil {
		current = *s.customLimit
	}
	if err := validateInboundCapIncrease(current, req.InboundDailyLimit, req.AcknowledgedExposure); err != nil {
		return InboundCapSetting{}, err
	}
	s.customLimit = &req.InboundDailyLimit
	return InboundCapSetting{
		InboundDailyLimit:    req.InboundDailyLimit,
		UpdatedBy:            req.UpdatedBy,
		AcknowledgedExposure: req.AcknowledgedExposure,
		UpdatedAt:            req.Now,
	}, nil
}

func (s *fakeInboundStore) Snapshot(_ context.Context, _ string, _ time.Time) (Snapshot, error) {
	allowance := int64(4000)
	remaining := allowance - s.used
	limit := int64(400)
	if s.customLimit != nil {
		limit = *s.customLimit
	}
	return Snapshot{
		PlanID:            s.period.PlanID,
		PeriodStart:       s.period.Start,
		PeriodEnd:         s.period.End,
		MonthlyAllowance:  &allowance,
		MonthlyUsed:       s.used,
		MonthlyRemaining:  &remaining,
		InboundDailyUsed:  s.dailyUsed,
		InboundDailyLimit: &limit,
		CatalogVersion:    CatalogVersion,
	}, nil
}

func TestInboundAdmissionConcurrentDuplicateChargesExactlyOnce(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := newFakeInboundStore("basic", now)
	service := NewService(store)
	req := InboundRequest{
		WorkspaceID:          "ws_1",
		SocialAccountID:      "sa_1",
		AppMode:              "unipost_managed_app",
		OperationKey:         "dm.received",
		Source:               "webhook",
		UpstreamResourceType: "dm_event",
		UpstreamResourceID:   "dm_123",
		Now:                  now,
	}

	const workers = 32
	var wg sync.WaitGroup
	results := make(chan InboundAdmission, workers)
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := service.AdmitInbound(context.Background(), req)
			results <- got
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("AdmitInbound: %v", err)
		}
	}
	duplicates := 0
	for result := range results {
		if result.Decision != InboundDecisionAccepted {
			t.Fatalf("decision = %q, want accepted", result.Decision)
		}
		if result.Duplicate {
			duplicates++
		}
	}
	if duplicates != workers-1 {
		t.Fatalf("duplicates = %d, want %d", duplicates, workers-1)
	}
	if store.used != 10 || store.dailyUsed != 10 || store.accepted != 1 || store.suppressed != 0 {
		t.Fatalf("used=%d daily=%d accepted=%d suppressed=%d", store.used, store.dailyUsed, store.accepted, store.suppressed)
	}
}

func TestInboundAdmissionReplayOnNextUTCDateChargesExactlyOnce(t *testing.T) {
	now := time.Date(2026, 7, 16, 23, 59, 0, 0, time.UTC)
	store := newFakeInboundStore("basic", now)
	service := NewService(store)
	request := InboundRequest{
		WorkspaceID: "ws_1", SocialAccountID: "sa_1", AppMode: "unipost_managed_app",
		OperationKey: "dm.received", Source: "webhook",
		UpstreamResourceType: "x_dm", UpstreamResourceID: "dm_same",
		Now: now,
	}
	first, err := service.AdmitInbound(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.Now = now.Add(2 * time.Minute)
	second, err := service.AdmitInbound(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Duplicate || !second.Duplicate {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
	if store.used != 10 || store.accepted != 1 {
		t.Fatalf("used=%d accepted=%d, replay charged twice", store.used, store.accepted)
	}
}

func TestAtomicInboundMutationRollsBackChargeAndReceiptOnInsertFailure(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := newFakeInboundStore("basic", now)
	service := NewService(store)
	request := InboundRequest{
		WorkspaceID: "ws_1", SocialAccountID: "sa_1", AppMode: "unipost_managed_app",
		OperationKey: "dm.received", Source: "webhook",
		UpstreamResourceType: "x_dm", UpstreamResourceID: "dm_atomic", Now: now,
	}
	insertErr := errors.New("inbox insert failed")
	if _, err := service.AdmitInboundWithMutation(
		context.Background(),
		request,
		func(context.Context, pgx.Tx) error { return insertErr },
	); !errors.Is(err, insertErr) {
		t.Fatalf("error = %v, want insert failure", err)
	}
	if store.used != 0 || store.dailyUsed != 0 || store.accepted != 0 || len(store.receipts) != 0 {
		t.Fatalf(
			"failed insert leaked charge/receipt: used=%d daily=%d accepted=%d receipts=%d",
			store.used, store.dailyUsed, store.accepted, len(store.receipts),
		)
	}

	mutations := 0
	first, err := service.AdmitInboundWithMutation(
		context.Background(),
		request,
		func(context.Context, pgx.Tx) error {
			mutations++
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	request.Now = request.Now.Add(24 * time.Hour)
	second, err := service.AdmitInboundWithMutation(
		context.Background(),
		request,
		func(context.Context, pgx.Tx) error {
			mutations++
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.Duplicate || !second.Duplicate || mutations != 2 {
		t.Fatalf("first=%+v second=%+v mutations=%d", first, second, mutations)
	}
	if store.used != 10 || store.accepted != 1 {
		t.Fatalf("used=%d accepted=%d, duplicate mutation charged twice", store.used, store.accepted)
	}
}

func TestDuplicateInboundAdmissionReconstructsOriginalSnapshotAcrossBillingPeriodChange(t *testing.T) {
	originalStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	originalEnd := originalStart.AddDate(0, 1, 0)
	receipt := inboundReceiptSnapshot{
		Decision:              InboundDecisionAccepted,
		WeightedUnits:         10,
		PeriodStart:           originalStart,
		PeriodEnd:             originalEnd,
		MonthlyUsedAfter:      3990,
		MonthlyRemainingAfter: 10,
		InboundDailyUsedAfter: 320,
		InboundDailyLimit:     400,
		EventsAcceptedAfter:   32,
		EventsSuppressedAfter: 3,
		PausePaidSources:      true,
		PauseReason:           PauseReasonDailySafetyBuffer,
		ResetAt:               time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC),
	}

	got := admissionFromReceipt(receipt)

	if !got.Duplicate || got.Decision != InboundDecisionAccepted ||
		got.MonthlyUsed != 3990 || got.MonthlyRemaining != 10 ||
		got.InboundDailyUsed != 320 || got.InboundDailyLimit != 400 ||
		got.EventsAccepted != 32 || got.EventsSuppressed != 3 ||
		!got.PausePaidSources || got.PauseReason != PauseReasonDailySafetyBuffer ||
		!got.ResetAt.Equal(receipt.ResetAt) {
		t.Fatalf("duplicate admission = %+v", got)
	}
	if !receipt.PeriodStart.Equal(originalStart) || !receipt.PeriodEnd.Equal(originalEnd) {
		t.Fatalf("receipt period changed: %+v", receipt)
	}
}

func TestInboundAdmissionReturnsDistinctCapAndMonthlyDecisions(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)

	t.Run("daily cap", func(t *testing.T) {
		store := newFakeInboundStore("basic", now)
		store.dailyUsed = 395
		got, err := NewService(store).AdmitInbound(context.Background(), InboundRequest{
			WorkspaceID: "ws_daily", SocialAccountID: "sa_1", AppMode: "unipost_managed_app",
			OperationKey: "dm.received", Source: "activity",
			UpstreamResourceType: "dm_event", UpstreamResourceID: "dm_over", Now: now,
		})
		if !errors.Is(err, ErrInboundDailyCapExceeded) {
			t.Fatalf("error = %v, want ErrInboundDailyCapExceeded", err)
		}
		if got.Decision != InboundDecisionSuppressedDailyCap || store.suppressed != 1 || store.used != 0 {
			t.Fatalf("admission=%+v store used=%d suppressed=%d", got, store.used, store.suppressed)
		}
	})

	t.Run("monthly allowance", func(t *testing.T) {
		store := newFakeInboundStore("basic", now)
		store.used = 3995
		got, err := NewService(store).AdmitInbound(context.Background(), InboundRequest{
			WorkspaceID: "ws_monthly", SocialAccountID: "sa_1", AppMode: "unipost_managed_app",
			OperationKey: "dm.received", Source: "activity",
			UpstreamResourceType: "dm_event", UpstreamResourceID: "dm_monthly", Now: now,
		})
		if !errors.Is(err, ErrMonthlyLimitExceeded) {
			t.Fatalf("error = %v, want ErrMonthlyLimitExceeded", err)
		}
		if got.Decision != InboundDecisionSuppressedMonthlyAllowance || store.suppressed != 1 || store.dailyUsed != 0 {
			t.Fatalf("admission=%+v daily=%d suppressed=%d", got, store.dailyUsed, store.suppressed)
		}
	})
}

func TestInboundAdmissionRequestAndStoreContractContainNoPrivateBody(t *testing.T) {
	for _, typ := range []reflect.Type{
		reflect.TypeOf(InboundRequest{}),
		reflect.TypeOf(StoreInboundRequest{}),
	} {
		for i := 0; i < typ.NumField(); i++ {
			name := strings.ToLower(typ.Field(i).Name)
			for _, forbidden := range []string{"body", "content", "message", "text", "payload"} {
				if strings.Contains(name, forbidden) {
					t.Fatalf("%s contains private-content field %q", typ.Name(), typ.Field(i).Name)
				}
			}
		}
	}
}

func TestInboundNotificationContainsOnlySafeMetricsAndManagementURL(t *testing.T) {
	payload := InboundNotification{
		WorkspaceID:       "ws_1",
		InboundDailyUsed:  320,
		InboundDailyLimit: 400,
		ResetAt:           time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC),
		CapManagementURL:  "https://dev-app.unipost.dev/settings/billing#x-inbound-cap",
	}
	if payload.InboundDailyUsed != 320 || payload.InboundDailyLimit != 400 || payload.ResetAt.IsZero() || payload.CapManagementURL == "" {
		t.Fatalf("payload = %+v", payload)
	}
	for _, private := range []string{"dm_80", "dm_event", "sa_1"} {
		if strings.Contains(payload.String(), private) {
			t.Fatalf("notification leaked private/upstream identifier %q: %s", private, payload.String())
		}
	}
}

type racingInboundCapStore struct {
	*fakeInboundStore
	snapshotBarrier sync.WaitGroup
	lowered         chan struct{}
}

func newRacingInboundCapStore(now time.Time) *racingInboundCapStore {
	store := &racingInboundCapStore{
		fakeInboundStore: newFakeInboundStore("basic", now),
		lowered:          make(chan struct{}),
	}
	store.snapshotBarrier.Add(2)
	return store
}

func (s *racingInboundCapStore) Snapshot(ctx context.Context, workspaceID string, now time.Time) (Snapshot, error) {
	s.snapshotBarrier.Done()
	s.snapshotBarrier.Wait()
	allowance := int64(4000)
	remaining := int64(4000)
	stale := int64(400)
	return Snapshot{
		PlanID:            "basic",
		PeriodStart:       now,
		PeriodEnd:         now.AddDate(0, 1, 0),
		MonthlyAllowance:  &allowance,
		MonthlyRemaining:  &remaining,
		InboundDailyLimit: &stale,
	}, nil
}

func (s *racingInboundCapStore) UpdateInboundCap(_ context.Context, req StoreUpdateInboundCapRequest) (InboundCapSetting, error) {
	if req.InboundDailyLimit == 300 {
		<-s.lowered
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current := int64(400)
	if s.customLimit != nil {
		current = *s.customLimit
	}
	if err := validateInboundCapIncrease(current, req.InboundDailyLimit, req.AcknowledgedExposure); err != nil {
		return InboundCapSetting{}, err
	}
	s.customLimit = &req.InboundDailyLimit
	if req.InboundDailyLimit == 100 {
		close(s.lowered)
	}
	return InboundCapSetting{InboundDailyLimit: req.InboundDailyLimit}, nil
}

func TestConcurrentCapUpdateRechecksAcknowledgementAgainstLockedCurrentLimit(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := newRacingInboundCapStore(now)
	service := NewService(store)
	errs := make(chan error, 2)

	go func() {
		_, err := service.UpdateInboundCap(context.Background(), UpdateInboundCapRequest{
			WorkspaceID: "ws_1", InboundDailyLimit: 100, UpdatedBy: "owner", Now: now,
		})
		errs <- err
	}()
	go func() {
		_, err := service.UpdateInboundCap(context.Background(), UpdateInboundCapRequest{
			WorkspaceID: "ws_1", InboundDailyLimit: 300, UpdatedBy: "admin", Now: now,
		})
		errs <- err
	}()

	var sawSuccess, sawAckRequired bool
	for i := 0; i < 2; i++ {
		err := <-errs
		switch {
		case err == nil:
			sawSuccess = true
		case errors.Is(err, ErrInboundExposureAcknowledgementRequired):
			sawAckRequired = true
		default:
			t.Fatalf("unexpected update error: %v", err)
		}
	}
	if !sawSuccess || !sawAckRequired {
		t.Fatalf("success=%v acknowledgement_required=%v", sawSuccess, sawAckRequired)
	}
}

func TestInboundSafetyBufferUsesMaxTwentyOrTenPercent(t *testing.T) {
	tests := []struct {
		used, limit int64
		want        bool
	}{
		{used: 79, limit: 100, want: false},
		{used: 80, limit: 100, want: true},
		{used: 359, limit: 400, want: false},
		{used: 360, limit: 400, want: true},
		{used: 2699, limit: 3000, want: false},
		{used: 2700, limit: 3000, want: true},
	}
	for _, tc := range tests {
		if got := remainingWithinSafetyBuffer(tc.used, tc.limit); got != tc.want {
			t.Fatalf("remainingWithinSafetyBuffer(%d, %d) = %v, want %v", tc.used, tc.limit, got, tc.want)
		}
	}
}

func TestUpdateInboundCapValidatesRemainingAllowanceAndExposure(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := newFakeInboundStore("basic", now)
	store.used = 1000
	service := NewService(store)

	if _, err := service.UpdateInboundCap(context.Background(), UpdateInboundCapRequest{
		WorkspaceID: "ws_1", InboundDailyLimit: 3001, UpdatedBy: "user_1", AcknowledgedExposure: true, Now: now,
	}); !errors.Is(err, ErrInboundCapExceedsMonthlyRemaining) {
		t.Fatalf("error = %v, want ErrInboundCapExceedsMonthlyRemaining", err)
	}
	if _, err := service.UpdateInboundCap(context.Background(), UpdateInboundCapRequest{
		WorkspaceID: "ws_1", InboundDailyLimit: 500, UpdatedBy: "user_1", Now: now,
	}); !errors.Is(err, ErrInboundExposureAcknowledgementRequired) {
		t.Fatalf("error = %v, want ErrInboundExposureAcknowledgementRequired", err)
	}
	got, err := service.UpdateInboundCap(context.Background(), UpdateInboundCapRequest{
		WorkspaceID: "ws_1", InboundDailyLimit: 500, UpdatedBy: "user_1", AcknowledgedExposure: true, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.InboundDailyLimit != 500 || got.UpdatedBy != "user_1" || !got.AcknowledgedExposure {
		t.Fatalf("setting = %+v", got)
	}
}

func TestPostgresInboundAdmissionContractIsAtomicAndBodyFree(t *testing.T) {
	source, err := os.ReadFile("postgres.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"INSERT INTO x_inbound_event_receipts",
		"ON CONFLICT (workspace_id, social_account_id, upstream_resource_type, upstream_resource_id) DO NOTHING",
		"AdmitInboundWithMutation",
		"RunInboundMutation",
		"admissionFromReceipt",
		"monthly_used_after",
		"monthly_remaining_after",
		"FOR UPDATE",
		"UPDATE x_inbound_daily_usage",
		"UPDATE x_usage_periods",
		"INSERT INTO x_inbound_cap_notifications",
		"payload",
		"status",
		"tx.Commit",
		"if req.AccountingEnabled",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("postgres admission missing %q", want)
		}
	}
	if !strings.Contains(text, "if req.AccountingEnabled {") {
		t.Fatal("safety-only inbound path must guard customer monthly accounting")
	}
	duplicateStart := strings.Index(text, "func loadDuplicateInboundAdmission")
	duplicateEnd := strings.Index(text, "func claimInboundThreshold")
	if duplicateStart < 0 || duplicateEnd <= duplicateStart {
		t.Fatal("duplicate admission function boundaries not found")
	}
	duplicateSource := text[duplicateStart:duplicateEnd]
	if strings.Contains(duplicateSource, "FROM x_usage_periods") {
		t.Fatal("duplicate replay must reconstruct from its receipt, not the caller's current billing period")
	}
	if strings.Contains(duplicateSource, "utc_date =") {
		t.Fatal("duplicate replay identity must remain stable across UTC dates")
	}
	updateStart := strings.Index(text, "func (s *PostgresStore) UpdateInboundCap")
	snapshotStart := strings.Index(text, "func (s *PostgresStore) Snapshot")
	if updateStart < 0 || snapshotStart <= updateStart {
		t.Fatal("cap update function boundaries not found")
	}
	updateSource := text[updateStart:snapshotStart]
	for _, want := range []string{"FROM x_inbound_cap_settings", "FOR UPDATE", "validateInboundCapIncrease"} {
		if !strings.Contains(updateSource, want) {
			t.Fatalf("atomic cap update missing %q", want)
		}
	}
	for _, forbidden := range []string{"dm_body", "message_body", "raw_payload"} {
		if strings.Contains(strings.ToLower(text), forbidden) {
			t.Fatalf("postgres admission contains private body field %q", forbidden)
		}
	}
}
