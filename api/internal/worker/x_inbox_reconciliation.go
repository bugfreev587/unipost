package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultXInboxOperationsReconciliationInterval = time.Minute
	defaultXInboxOperationsMetricsWindow          = 24 * time.Hour
	defaultXInboxOperationsStaleAfter             = 10 * time.Minute
)

// XInboxOperationsSnapshot intentionally contains only aggregate counters and
// durations. Provider payloads, customer identifiers, credentials, and tokens
// must never cross this operations boundary.
type XInboxOperationsSnapshot struct {
	ProvisionalUsageEvents        int64
	StaleProvisionalUsageEvents   int64
	ReversedUsageEvents           int64
	SuppressedDailyCapEvents      int64
	SuppressedAllowanceEvents     int64
	FilteredStreamRules           int64
	ActivitySubscriptions         int64
	Notification80Claims          int64
	Notification80Enqueued        int64
	Notification100Claims         int64
	Notification100Enqueued       int64
	StaleNotificationClaims       int64
	PausedSources                 int64
	PauseMaxAgeSeconds            int64
	RestorePendingSources         int64
	RestorePendingMaxAgeSeconds   int64
	StaleDeliveryResources        int64
	PendingCleanupIntents         int64
	OverdueCleanupIntents         int64
	StaleCleanupLeases            int64
	OldestCleanupIntentAgeSeconds int64
}

type XInboxOperationsReconciliationStore interface {
	Snapshot(context.Context, time.Time) (XInboxOperationsSnapshot, error)
}

type XInboxOperationsReconciliationConfig struct {
	Interval                     time.Duration
	MetricsWindow                time.Duration
	StaleAfter                   time.Duration
	FilteredStreamRuleCapacity   int64
	ActivitySubscriptionCapacity int64
	Now                          func() time.Time
}

type XInboxOperationsReconciliationWorker struct {
	store                        XInboxOperationsReconciliationStore
	logger                       *slog.Logger
	interval                     time.Duration
	filteredStreamRuleCapacity   int64
	activitySubscriptionCapacity int64
	now                          func() time.Time
}

func NewXInboxOperationsReconciliationWorker(
	store XInboxOperationsReconciliationStore,
	logger *slog.Logger,
	config XInboxOperationsReconciliationConfig,
) *XInboxOperationsReconciliationWorker {
	if logger == nil {
		logger = slog.Default()
	}
	if config.Interval <= 0 {
		config.Interval = defaultXInboxOperationsReconciliationInterval
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	return &XInboxOperationsReconciliationWorker{
		store:                        store,
		logger:                       logger,
		interval:                     config.Interval,
		filteredStreamRuleCapacity:   config.FilteredStreamRuleCapacity,
		activitySubscriptionCapacity: config.ActivitySubscriptionCapacity,
		now:                          config.Now,
	}
}

func NewPostgresXInboxOperationsReconciliationWorker(
	pool *pgxpool.Pool,
	logger *slog.Logger,
	config XInboxOperationsReconciliationConfig,
) *XInboxOperationsReconciliationWorker {
	if config.MetricsWindow <= 0 {
		config.MetricsWindow = defaultXInboxOperationsMetricsWindow
	}
	if config.StaleAfter <= 0 {
		config.StaleAfter = defaultXInboxOperationsStaleAfter
	}
	return NewXInboxOperationsReconciliationWorker(
		&postgresXInboxOperationsReconciliationStore{
			pool:          pool,
			metricsWindow: config.MetricsWindow,
			staleAfter:    config.StaleAfter,
		},
		logger,
		config,
	)
}

func (w *XInboxOperationsReconciliationWorker) Start(ctx context.Context) {
	if w == nil || w.store == nil {
		return
	}
	w.runAndReport(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runAndReport(ctx)
		}
	}
}

func (w *XInboxOperationsReconciliationWorker) runAndReport(ctx context.Context) {
	// RunOnce emits a sanitized operational error itself. The returned error
	// must not be attached to logs because a driver/provider error can include
	// credentials or customer content.
	_ = w.RunOnce(ctx)
}

func (w *XInboxOperationsReconciliationWorker) RunOnce(ctx context.Context) error {
	if w == nil || w.store == nil {
		return nil
	}
	now := w.now().UTC()
	snapshot, err := w.store.Snapshot(ctx, now)
	if err != nil {
		w.logger.Warn("X Inbox operations snapshot failed",
			"event", "x_inbox_reconciliation_failed",
			"error_class", "snapshot_query_failed")
		return err
	}

	w.logger.Info("X Inbox operations snapshot",
		"event", "x_inbox_operations_snapshot",
		"provisional_usage_events", snapshot.ProvisionalUsageEvents,
		"stale_provisional_usage_events", snapshot.StaleProvisionalUsageEvents,
		"reversed_usage_events", snapshot.ReversedUsageEvents,
		"suppressed_daily_cap_events", snapshot.SuppressedDailyCapEvents,
		"suppressed_allowance_events", snapshot.SuppressedAllowanceEvents,
		"filtered_stream_rules", snapshot.FilteredStreamRules,
		"filtered_stream_rule_capacity", w.filteredStreamRuleCapacity,
		"activity_subscriptions", snapshot.ActivitySubscriptions,
		"activity_subscription_capacity", w.activitySubscriptionCapacity,
		"notification_80_claims", snapshot.Notification80Claims,
		"notification_80_enqueued", snapshot.Notification80Enqueued,
		"notification_100_claims", snapshot.Notification100Claims,
		"notification_100_enqueued", snapshot.Notification100Enqueued,
		"stale_notification_claims", snapshot.StaleNotificationClaims,
		"paused_sources", snapshot.PausedSources,
		"pause_max_age_seconds", snapshot.PauseMaxAgeSeconds,
		"restore_pending_sources", snapshot.RestorePendingSources,
		"restore_pending_max_age_seconds", snapshot.RestorePendingMaxAgeSeconds,
		"stale_delivery_resources", snapshot.StaleDeliveryResources,
		"pending_cleanup_intents", snapshot.PendingCleanupIntents,
		"overdue_cleanup_intents", snapshot.OverdueCleanupIntents,
		"stale_cleanup_leases", snapshot.StaleCleanupLeases,
		"oldest_cleanup_intent_age_seconds", snapshot.OldestCleanupIntentAgeSeconds)

	w.logCapacityAlert("filtered_stream_rules", snapshot.FilteredStreamRules, w.filteredStreamRuleCapacity)
	w.logCapacityAlert("activity_subscriptions", snapshot.ActivitySubscriptions, w.activitySubscriptionCapacity)
	w.logReconciliationAlerts(snapshot)
	return nil
}

// XInboxCapacityAlertLevel returns the highest crossed alert threshold. The
// ceiling calculation avoids both floating-point rounding and int64 overflow.
func XInboxCapacityAlertLevel(used, capacity int64) int {
	if used < 0 || capacity <= 0 {
		return 0
	}
	for _, level := range []int{95, 85, 70} {
		level64 := int64(level)
		required := (capacity/100)*level64 + ((capacity%100)*level64+99)/100
		if used >= required {
			return level
		}
	}
	return 0
}

func (w *XInboxOperationsReconciliationWorker) logCapacityAlert(resource string, used, capacity int64) {
	level := XInboxCapacityAlertLevel(used, capacity)
	if level == 0 {
		return
	}
	w.logger.Warn("X Inbox delivery capacity threshold crossed",
		"event", "x_inbox_capacity_alert",
		"resource", resource,
		"used", used,
		"capacity", capacity,
		"threshold_percent", level)
}

func (w *XInboxOperationsReconciliationWorker) logReconciliationAlerts(snapshot XInboxOperationsSnapshot) {
	alerts := []struct {
		kind  string
		count int64
	}{
		{kind: "stale_provisional_usage", count: snapshot.StaleProvisionalUsageEvents},
		{kind: "cap_suppression", count: snapshot.SuppressedDailyCapEvents + snapshot.SuppressedAllowanceEvents},
		{kind: "stale_notification_claim", count: snapshot.StaleNotificationClaims},
		{kind: "source_paused", count: snapshot.PausedSources},
		{kind: "source_restore_pending", count: snapshot.RestorePendingSources},
		{kind: "stale_delivery_resource", count: snapshot.StaleDeliveryResources},
		{kind: "overdue_cleanup", count: snapshot.OverdueCleanupIntents},
		{kind: "stale_cleanup_lease", count: snapshot.StaleCleanupLeases},
	}
	for _, alert := range alerts {
		if alert.count == 0 {
			continue
		}
		w.logger.Warn("X Inbox reconciliation attention required",
			"event", "x_inbox_reconciliation_alert",
			"kind", alert.kind,
			"count", alert.count)
	}
}

type postgresXInboxOperationsReconciliationStore struct {
	pool          *pgxpool.Pool
	metricsWindow time.Duration
	staleAfter    time.Duration
}

func (s *postgresXInboxOperationsReconciliationStore) Snapshot(
	ctx context.Context,
	now time.Time,
) (XInboxOperationsSnapshot, error) {
	if s == nil || s.pool == nil {
		return XInboxOperationsSnapshot{}, errors.New("X Inbox reconciliation database is unavailable")
	}
	windowStart := now.Add(-s.metricsWindow)
	staleBefore := now.Add(-s.staleAfter)
	row := s.pool.QueryRow(ctx, `
WITH usage_metrics AS (
  SELECT
    COUNT(*) FILTER (WHERE status = 'provisional')::BIGINT AS provisional,
    COUNT(*) FILTER (WHERE status = 'provisional' AND created_at < $3)::BIGINT AS stale_provisional,
    COUNT(*) FILTER (WHERE status = 'reversed' AND updated_at >= $2)::BIGINT AS reversed
  FROM x_usage_events
), receipt_metrics AS (
  SELECT
    COUNT(*) FILTER (WHERE decision = 'suppressed_daily_cap')::BIGINT AS suppressed_cap,
    COUNT(*) FILTER (WHERE decision = 'suppressed_monthly_allowance')::BIGINT AS suppressed_allowance
  FROM x_inbound_event_receipts
  WHERE created_at >= $2
), delivery_metrics AS (
  SELECT
    COUNT(*) FILTER (WHERE filtered_stream_rule_id IS NOT NULL)::BIGINT AS stream_rules,
    COUNT(*) FILTER (WHERE activity_dm_subscription_id IS NOT NULL)::BIGINT AS activity_subscriptions,
    COUNT(*) FILTER (WHERE delivery_status IN ('paused_cap', 'paused_allowance', 'paused_plan'))::BIGINT AS paused_sources,
	    COALESCE(EXTRACT(EPOCH FROM ($1 - (MIN(updated_at) FILTER (
	      WHERE delivery_status IN ('paused_cap', 'paused_allowance', 'paused_plan')
	    )))), 0)::BIGINT AS pause_max_age_seconds,
	    COUNT(*) FILTER (WHERE delivery_status = 'pending')::BIGINT AS restore_pending_sources,
	    COALESCE(EXTRACT(EPOCH FROM ($1 - (MIN(updated_at) FILTER (
	      WHERE delivery_status = 'pending'
	    )))), 0)::BIGINT AS restore_pending_max_age_seconds,
    COUNT(*) FILTER (
      WHERE delivery_status IN ('pending', 'error') AND updated_at < $3
         OR delivery_status = 'active' AND COALESCE(last_synced_at, updated_at) < $3
    )::BIGINT AS stale_resources
  FROM x_inbox_delivery_resources
), notification_metrics AS (
  SELECT
    COUNT(*) FILTER (WHERE threshold = 80)::BIGINT AS claims_80,
    COUNT(*) FILTER (WHERE threshold = 80 AND status = 'enqueued')::BIGINT AS enqueued_80,
    COUNT(*) FILTER (WHERE threshold = 100)::BIGINT AS claims_100,
    COUNT(*) FILTER (WHERE threshold = 100 AND status = 'enqueued')::BIGINT AS enqueued_100,
    COUNT(*) FILTER (WHERE status = 'processing' AND lease_expires_at < $1)::BIGINT AS stale_claims
  FROM x_inbound_cap_notifications
  WHERE utc_date = ($1 AT TIME ZONE 'UTC')::DATE
), cleanup_metrics AS (
  SELECT
    COUNT(*)::BIGINT AS pending,
    COUNT(*) FILTER (WHERE next_attempt_at <= $1)::BIGINT AS overdue,
    COUNT(*) FILTER (WHERE lease_until < $1)::BIGINT AS stale_leases,
    COALESCE(EXTRACT(EPOCH FROM ($1 - MIN(created_at))), 0)::BIGINT AS oldest_age_seconds
  FROM x_inbox_delivery_cleanup_intents
)
SELECT
  usage_metrics.provisional,
  usage_metrics.stale_provisional,
  usage_metrics.reversed,
  receipt_metrics.suppressed_cap,
  receipt_metrics.suppressed_allowance,
  delivery_metrics.stream_rules,
  delivery_metrics.activity_subscriptions,
  notification_metrics.claims_80,
  notification_metrics.enqueued_80,
  notification_metrics.claims_100,
  notification_metrics.enqueued_100,
  notification_metrics.stale_claims,
  delivery_metrics.paused_sources,
  delivery_metrics.pause_max_age_seconds,
  delivery_metrics.restore_pending_sources,
  delivery_metrics.restore_pending_max_age_seconds,
  delivery_metrics.stale_resources,
  cleanup_metrics.pending,
  cleanup_metrics.overdue,
  cleanup_metrics.stale_leases,
  cleanup_metrics.oldest_age_seconds
FROM usage_metrics, receipt_metrics, delivery_metrics, notification_metrics, cleanup_metrics`,
		now, windowStart, staleBefore)

	var snapshot XInboxOperationsSnapshot
	err := row.Scan(
		&snapshot.ProvisionalUsageEvents,
		&snapshot.StaleProvisionalUsageEvents,
		&snapshot.ReversedUsageEvents,
		&snapshot.SuppressedDailyCapEvents,
		&snapshot.SuppressedAllowanceEvents,
		&snapshot.FilteredStreamRules,
		&snapshot.ActivitySubscriptions,
		&snapshot.Notification80Claims,
		&snapshot.Notification80Enqueued,
		&snapshot.Notification100Claims,
		&snapshot.Notification100Enqueued,
		&snapshot.StaleNotificationClaims,
		&snapshot.PausedSources,
		&snapshot.PauseMaxAgeSeconds,
		&snapshot.RestorePendingSources,
		&snapshot.RestorePendingMaxAgeSeconds,
		&snapshot.StaleDeliveryResources,
		&snapshot.PendingCleanupIntents,
		&snapshot.OverdueCleanupIntents,
		&snapshot.StaleCleanupLeases,
		&snapshot.OldestCleanupIntentAgeSeconds,
	)
	return snapshot, err
}
