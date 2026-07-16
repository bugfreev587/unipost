package worker

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestXInboxOperationsReconciliationEmitsBoundedMetrics(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := &xInboxReconciliationFakeStore{snapshot: XInboxOperationsSnapshot{
		ProvisionalUsageEvents:        7,
		StaleProvisionalUsageEvents:   2,
		ReversedUsageEvents:           3,
		SuppressedDailyCapEvents:      5,
		SuppressedAllowanceEvents:     4,
		FilteredStreamRules:           71,
		ActivitySubscriptions:         86,
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
		FilteredStreamRuleCapacity:   100,
		ActivitySubscriptionCapacity: 100,
		Now:                          func() time.Time { return now },
	})

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	logs := output.String()
	for _, want := range []string{
		`"event":"x_inbox_operations_snapshot"`,
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
}

func (s *xInboxReconciliationFakeStore) Snapshot(_ context.Context, now time.Time) (XInboxOperationsSnapshot, error) {
	s.now = now
	return s.snapshot, s.err
}
