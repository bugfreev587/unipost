package worker

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestXInboxOperationsReconciliationEmitsBoundedMetrics(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := &xInboxReconciliationFakeStore{snapshot: XInboxOperationsSnapshot{
		ProvisionalUsageEvents:      7,
		StaleProvisionalUsageEvents: 2,
		ReversedUsageEvents:         3,
		SuppressedDailyCapEvents:    5,
		SuppressedAllowanceEvents:   4,
		CapacityScopes: []XInboxCapacityScope{
			{AppScope: "managed", AppMode: "unipost_managed_app", ResourceType: "filtered_stream_rules", Used: 71},
			{AppScope: "managed", AppMode: "unipost_managed_app", ResourceType: "activity_subscriptions", Used: 86},
		},
		Notification80Claims:          8,
		Notification80Enqueued:        7,
		Notification100Claims:         4,
		Notification100Enqueued:       3,
		StaleNotificationClaims:       1,
		PausedSources:                 6,
		PauseMaxAgeSeconds:            125,
		RestorePendingSources:         2,
		RestorePendingMaxAgeSeconds:   44,
		StaleDeliveryResources:        3,
		PendingCleanupIntents:         9,
		OverdueCleanupIntents:         2,
		StaleCleanupLeases:            1,
		OldestCleanupIntentAgeSeconds: 300,
	}}
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	worker := NewXInboxOperationsReconciliationWorker(store, logger, XInboxOperationsReconciliationConfig{
		ManagedFilteredStreamRuleCapacity:   100,
		ManagedActivitySubscriptionCapacity: 100,
		Now:                                 func() time.Time { return now },
	})

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	logs := output.String()
	for _, want := range []string{
		`"event":"x_inbox_operations_snapshot"`,
		`"evidence_day_start":"2026-07-15T00:00:00Z"`,
		`"evidence_day_end":"2026-07-16T00:00:00Z"`,
		`"provisional_usage_events":7`,
		`"stale_provisional_usage_events":2`,
		`"reversed_usage_events":3`,
		`"suppressed_daily_cap_events":5`,
		`"suppressed_allowance_events":4`,
		`"notification_80_claims":8`,
		`"notification_100_claims":4`,
		`"pause_max_age_seconds":125`,
		`"restore_pending_max_age_seconds":44`,
		`"stale_delivery_resources":3`,
		`"overdue_cleanup_intents":2`,
		`"event":"x_inbox_capacity_alert"`,
		`"resource":"filtered_stream_rules"`,
		`"threshold_percent":70`,
		`"resource":"activity_subscriptions"`,
		`"threshold_percent":85`,
		`"event":"x_inbox_reconciliation_alert"`,
	} {
		if !strings.Contains(logs, want) {
			t.Errorf("logs missing %s\n%s", want, logs)
		}
	}
	if store.now != now {
		t.Fatalf("snapshot time = %s, want %s", store.now, now)
	}
	if want := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC); store.dayStart != want {
		t.Fatalf("snapshot dayStart = %s, want %s", store.dayStart, want)
	}
	if want := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC); store.dayEnd != want {
		t.Fatalf("snapshot dayEnd = %s, want %s", store.dayEnd, want)
	}
}

func TestXInboxCapacityReconciliationIsAppScoped(t *testing.T) {
	t.Parallel()

	workspaceScope := XInboxCapacityScopeKey("workspace_x_app", "workspace-client-id-sensitive")
	store := &xInboxReconciliationFakeStore{snapshot: XInboxOperationsSnapshot{
		CapacityScopes: []XInboxCapacityScope{
			{AppScope: "managed", AppMode: "unipost_managed_app", ResourceType: "filtered_stream_rules", Used: 60},
			{AppScope: workspaceScope, AppMode: "workspace_x_app", ResourceType: "filtered_stream_rules", Used: 9},
			{AppScope: "workspace_second", AppMode: "workspace_x_app", ResourceType: "filtered_stream_rules", Used: 10},
		},
	}}
	var output bytes.Buffer
	worker := NewXInboxOperationsReconciliationWorker(
		store,
		slog.New(slog.NewJSONHandler(&output, nil)),
		XInboxOperationsReconciliationConfig{
			ManagedFilteredStreamRuleCapacity: 100,
			WorkspaceAppCapacities: map[string]XInboxAppResourceCapacities{
				workspaceScope:     {FilteredStreamRules: 10},
				"workspace_second": {FilteredStreamRules: 10},
			},
		},
	)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	logs := output.String()
	if strings.Contains(logs, "workspace-client-id-sensitive") {
		t.Fatalf("logs exposed raw workspace app identity: %s", logs)
	}
	for _, want := range []string{
		`"app_scope":"` + workspaceScope + `"`,
		`"threshold_percent":85`,
		`"app_scope":"workspace_second"`,
		`"threshold_percent":95`,
	} {
		if !strings.Contains(logs, want) {
			t.Errorf("app-scoped capacity logs missing %s\n%s", want, logs)
		}
	}
	if strings.Contains(logs, `"used":79`) {
		t.Fatalf("capacity incorrectly aggregated across applications: %s", logs)
	}
}

func TestXInboxOperationsReconciliationEmitsDurableOperationAndPromotionMetrics(t *testing.T) {
	t.Parallel()

	store := &xInboxReconciliationFakeStore{snapshot: XInboxOperationsSnapshot{
		OutboundOutcomeUnknown:             2,
		OutboundNeedsReconciliation:        3,
		OutboundRemoteSucceeded:            4,
		StaleOutboundOperations:            5,
		BYOUnmeteredUncertainWrites:        1,
		BackfillConfirmationsCreated:       10,
		BackfillConfirmationsCompleted:     6,
		BackfillConfirmationsFailed:        1,
		BackfillConfirmationsExpired:       2,
		StaleBackfillConfirmations:         1,
		BackfillEstimatedCredits:           700,
		ExposureReserved:                   2,
		ExposureReadStarted:                3,
		ExposureFinalizePending:            4,
		ExposureReleasePending:             5,
		ExposureNeedsReconciliation:        6,
		StaleExposureReservations:          7,
		DedupObservedResources:             100,
		DeduplicatedResources:              20,
		WebhookItemsObserved:               40,
		WebhookAverageLatencyMilliseconds:  350,
		WebhookMaxLatencyMilliseconds:      900,
		BackfillItemsObserved:              50,
		BackfillAverageLatencyMilliseconds: 1_200,
		BackfillMaxLatencyMilliseconds:     2_500,
		OutboundRequests:                   20,
		OutboundCompleted:                  18,
		CustomerDemandWorkspaces:           12,
		FinalizedUsageEvents:               80,
		FinalizedWeightedUnits:             9_000,
		UsageMetrics: []XInboxUsageMetric{
			{OperationKey: "dm.received", CatalogVersion: "x-v1", Status: "finalized", Events: 70, WeightedUnits: 7_000},
		},
	}}
	costInput := &xInboxDailyCostFakeInput{value: XInboxDailyCost{
		Available:                        true,
		ProviderCostMicros:               120_000,
		ExpectedFinalizedUsageCostMicros: 100_000,
	}}
	var output bytes.Buffer
	worker := NewXInboxOperationsReconciliationWorker(
		store,
		slog.New(slog.NewJSONHandler(&output, nil)),
		XInboxOperationsReconciliationConfig{DailyCostInput: costInput},
	)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	logs := output.String()
	for _, want := range []string{
		`"outbound_outcome_unknown":2`,
		`"outbound_needs_reconciliation":3`,
		`"byo_unmetered_uncertain_writes":1`,
		`"stale_backfill_confirmations":1`,
		`"exposure_needs_reconciliation":6`,
		`"stale_exposure_reservations":7`,
		`"dedup_rate_basis_points":2000`,
		`"outbound_success_rate_basis_points":9000`,
		`"customer_demand_workspaces":12`,
		`"event":"x_inbox_usage_metric"`,
		`"operation_key":"dm.received"`,
		`"catalog_version":"x-v1"`,
		`"event":"x_inbox_cost_variance"`,
		`"variance_micros":20000`,
		`"variance_basis_points":2000`,
	} {
		if !strings.Contains(logs, want) {
			t.Errorf("promotion logs missing %s\n%s", want, logs)
		}
	}
}

func TestXInboxOperationsReconciliationAlertsWhenDailyCostInputIsMissing(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	worker := NewXInboxOperationsReconciliationWorker(
		&xInboxReconciliationFakeStore{},
		slog.New(slog.NewJSONHandler(&output, nil)),
		XInboxOperationsReconciliationConfig{},
	)
	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if logs := output.String(); !strings.Contains(logs, `"kind":"daily_cost_input_missing"`) {
		t.Fatalf("missing daily-cost alert not emitted: %s", logs)
	}
}

func TestXInboxWorkspaceAppCapacityConfigUsesOpaqueScopes(t *testing.T) {
	t.Parallel()

	config, err := ParseXInboxWorkspaceAppCapacities(`{
		"workspace_0123456789abcdef": {
			"filtered_stream_rules": 25,
			"activity_subscriptions": 10
		}
	}`)
	if err != nil {
		t.Fatalf("ParseXInboxWorkspaceAppCapacities() error = %v", err)
	}
	if got := config["workspace_0123456789abcdef"].FilteredStreamRules; got != 25 {
		t.Fatalf("filtered stream capacity = %d, want 25", got)
	}
	if _, err := ParseXInboxWorkspaceAppCapacities(`{"raw-client-id":{"filtered_stream_rules":10}}`); err == nil {
		t.Fatal("raw app identity key was accepted")
	}
}

func TestXInboxCapacityReconciliationQueryKeepsApplicationsSeparated(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("x_inbox_reconciliation.go")
	if err != nil {
		t.Fatalf("read reconciliation source: %v", err)
	}
	text := string(source)
	for _, want := range []string{
		"COUNT(DISTINCT resource_id)::BIGINT",
		"GROUP BY app_mode, app_identity, resource_type",
		"FROM x_inbox_delivery_cleanup_intents",
		"XInboxCapacityScopeKey(scope.AppMode, appIdentity)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("app-scoped capacity query missing %q", want)
		}
	}
	if strings.Contains(text, "COUNT(*) FILTER (WHERE filtered_stream_rule_id IS NOT NULL)") {
		t.Fatal("found cross-application filtered-stream aggregation")
	}
}

func TestXInboxPromotionEvidenceUsesOneCompletedSettlementDay(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("x_inbox_reconciliation.go")
	if err != nil {
		t.Fatalf("read reconciliation source: %v", err)
	}
	text := string(source)
	for _, want := range []string{
		"dayEnd := now.Truncate(24 * time.Hour)",
		"dayStart := dayEnd.Add(-24 * time.Hour)",
		"WHERE status IN ('finalized', 'reversed')",
		"AND updated_at >= $2 AND updated_at < $4",
		"AND updated_at >= $1 AND updated_at < $2",
		"WHERE status = 'provisional'",
		"WHERE o.status NOT IN ('completed', 'succeeded')",
		"WHERE status NOT IN ('finalized', 'released')",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("completed-day/current-state query contract missing %q", want)
		}
	}
	if strings.Contains(text, "windowStart") || strings.Contains(text, "MetricsWindow") {
		t.Fatal("rolling reconciliation window remains in completed-day evidence")
	}
}

func TestXInboxReconciliationSnapshotQueryFreshSchema(t *testing.T) {
	databaseURL := os.Getenv("X_INBOX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("X_INBOX_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := &postgresXInboxOperationsReconciliationStore{
		pool:       pool,
		staleAfter: 10 * time.Minute,
	}
	if _, err := store.Snapshot(
		ctx,
		now,
		time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC),
	); err != nil {
		t.Fatalf("Snapshot() on fresh schema: %v", err)
	}
}

func TestXInboxCapacityReconciliationThresholds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		used      int64
		capacity  int64
		wantLevel int
	}{
		{name: "below", used: 69, capacity: 100, wantLevel: 0},
		{name: "seventy", used: 70, capacity: 100, wantLevel: 70},
		{name: "rounding_does_not_alert_early", used: 849, capacity: 1000, wantLevel: 70},
		{name: "eighty_five", used: 85, capacity: 100, wantLevel: 85},
		{name: "ninety_five", used: 95, capacity: 100, wantLevel: 95},
		{name: "over_capacity", used: 130, capacity: 100, wantLevel: 95},
		{name: "unknown_capacity", used: 99, capacity: 0, wantLevel: 0},
		{name: "large_counts_do_not_overflow", used: 100_000_000_000_000_000, capacity: 9_000_000_000_000_000_000, wantLevel: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := XInboxCapacityAlertLevel(test.used, test.capacity); got != test.wantLevel {
				t.Fatalf("XInboxCapacityAlertLevel(%d, %d) = %d, want %d", test.used, test.capacity, got, test.wantLevel)
			}
		})
	}
}

func TestXInboxOperationsReconciliationDoesNotLogProviderContentOrTokens(t *testing.T) {
	t.Parallel()

	const (
		rawBody = "private DM body must not appear"
		token   = "super-secret-provider-token"
	)
	store := &xInboxReconciliationFakeStore{err: errors.New(rawBody + " " + token)}
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	worker := NewXInboxOperationsReconciliationWorker(store, logger, XInboxOperationsReconciliationConfig{})

	if err := worker.RunOnce(context.Background()); err == nil {
		t.Fatal("RunOnce() error = nil, want store error")
	}

	logs := output.String()
	if strings.Contains(logs, rawBody) || strings.Contains(logs, token) {
		t.Fatalf("logs leaked provider content or token: %s", logs)
	}
	if !strings.Contains(logs, `"error_class":"snapshot_query_failed"`) {
		t.Fatalf("logs missing sanitized error class: %s", logs)
	}
}

type xInboxReconciliationFakeStore struct {
	snapshot XInboxOperationsSnapshot
	err      error
	now      time.Time
	dayStart time.Time
	dayEnd   time.Time
}

type xInboxDailyCostFakeInput struct {
	value XInboxDailyCost
	err   error
}

func (i *xInboxDailyCostFakeInput) DailyCost(context.Context, time.Time) (XInboxDailyCost, error) {
	return i.value, i.err
}

func (s *xInboxReconciliationFakeStore) Snapshot(
	_ context.Context,
	now time.Time,
	dayStart time.Time,
	dayEnd time.Time,
) (XInboxOperationsSnapshot, error) {
	s.now = now
	s.dayStart = dayStart
	s.dayEnd = dayEnd
	return s.snapshot, s.err
}
