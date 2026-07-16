package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
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
	ProvisionalUsageEvents             int64
	StaleProvisionalUsageEvents        int64
	ReversedUsageEvents                int64
	SuppressedDailyCapEvents           int64
	SuppressedAllowanceEvents          int64
	CapacityScopes                     []XInboxCapacityScope
	Notification80Claims               int64
	Notification80Enqueued             int64
	Notification100Claims              int64
	Notification100Enqueued            int64
	StaleNotificationClaims            int64
	PausedSources                      int64
	PauseMaxAgeSeconds                 int64
	RestorePendingSources              int64
	RestorePendingMaxAgeSeconds        int64
	StaleDeliveryResources             int64
	PendingCleanupIntents              int64
	OverdueCleanupIntents              int64
	StaleCleanupLeases                 int64
	OldestCleanupIntentAgeSeconds      int64
	OutboundOutcomeUnknown             int64
	OutboundNeedsReconciliation        int64
	OutboundRemoteSucceeded            int64
	StaleOutboundOperations            int64
	BYOUnmeteredUncertainWrites        int64
	BackfillConfirmationsCreated       int64
	BackfillConfirmationsCompleted     int64
	BackfillConfirmationsFailed        int64
	BackfillConfirmationsExpired       int64
	StaleBackfillConfirmations         int64
	BackfillEstimatedCredits           int64
	ExposureReserved                   int64
	ExposureReadStarted                int64
	ExposureFinalizePending            int64
	ExposureReleasePending             int64
	ExposureNeedsReconciliation        int64
	StaleExposureReservations          int64
	DedupObservedResources             int64
	DeduplicatedResources              int64
	WebhookItemsObserved               int64
	WebhookAverageLatencyMilliseconds  int64
	WebhookMaxLatencyMilliseconds      int64
	BackfillItemsObserved              int64
	BackfillAverageLatencyMilliseconds int64
	BackfillMaxLatencyMilliseconds     int64
	OutboundRequests                   int64
	OutboundCompleted                  int64
	CustomerDemandWorkspaces           int64
	FinalizedUsageEvents               int64
	FinalizedWeightedUnits             int64
	UsageMetrics                       []XInboxUsageMetric
}

type XInboxCapacityScope struct {
	AppScope     string
	AppMode      string
	ResourceType string
	Used         int64
}

type XInboxUsageMetric struct {
	OperationKey   string
	CatalogVersion string
	Status         string
	Events         int64
	WeightedUnits  int64
}

type XInboxAppResourceCapacities struct {
	FilteredStreamRules   int64 `json:"filtered_stream_rules"`
	ActivitySubscriptions int64 `json:"activity_subscriptions"`
}

type XInboxDailyCost struct {
	Available                        bool
	ProviderCostMicros               int64
	ExpectedFinalizedUsageCostMicros int64
}

type XInboxDailyCostInput interface {
	DailyCost(context.Context, time.Time) (XInboxDailyCost, error)
}

type XInboxOperationsReconciliationStore interface {
	Snapshot(context.Context, time.Time) (XInboxOperationsSnapshot, error)
}

type XInboxOperationsReconciliationConfig struct {
	Interval                            time.Duration
	MetricsWindow                       time.Duration
	StaleAfter                          time.Duration
	ManagedFilteredStreamRuleCapacity   int64
	ManagedActivitySubscriptionCapacity int64
	WorkspaceAppCapacities              map[string]XInboxAppResourceCapacities
	DailyCostInput                      XInboxDailyCostInput
	Now                                 func() time.Time
}

type XInboxOperationsReconciliationWorker struct {
	store                               XInboxOperationsReconciliationStore
	logger                              *slog.Logger
	interval                            time.Duration
	managedFilteredStreamRuleCapacity   int64
	managedActivitySubscriptionCapacity int64
	workspaceAppCapacities              map[string]XInboxAppResourceCapacities
	dailyCostInput                      XInboxDailyCostInput
	now                                 func() time.Time
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
		store:                               store,
		logger:                              logger,
		interval:                            config.Interval,
		managedFilteredStreamRuleCapacity:   config.ManagedFilteredStreamRuleCapacity,
		managedActivitySubscriptionCapacity: config.ManagedActivitySubscriptionCapacity,
		workspaceAppCapacities:              config.WorkspaceAppCapacities,
		dailyCostInput:                      config.DailyCostInput,
		now:                                 config.Now,
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
		"oldest_cleanup_intent_age_seconds", snapshot.OldestCleanupIntentAgeSeconds,
		"outbound_outcome_unknown", snapshot.OutboundOutcomeUnknown,
		"outbound_needs_reconciliation", snapshot.OutboundNeedsReconciliation,
		"outbound_remote_succeeded", snapshot.OutboundRemoteSucceeded,
		"stale_outbound_operations", snapshot.StaleOutboundOperations,
		"byo_unmetered_uncertain_writes", snapshot.BYOUnmeteredUncertainWrites,
		"backfill_confirmations_created", snapshot.BackfillConfirmationsCreated,
		"backfill_confirmations_completed", snapshot.BackfillConfirmationsCompleted,
		"backfill_confirmations_failed", snapshot.BackfillConfirmationsFailed,
		"backfill_confirmations_expired", snapshot.BackfillConfirmationsExpired,
		"stale_backfill_confirmations", snapshot.StaleBackfillConfirmations,
		"backfill_estimated_credits", snapshot.BackfillEstimatedCredits,
		"exposure_reserved", snapshot.ExposureReserved,
		"exposure_read_started", snapshot.ExposureReadStarted,
		"exposure_finalize_pending", snapshot.ExposureFinalizePending,
		"exposure_release_pending", snapshot.ExposureReleasePending,
		"exposure_needs_reconciliation", snapshot.ExposureNeedsReconciliation,
		"stale_exposure_reservations", snapshot.StaleExposureReservations,
		"dedup_observed_resources", snapshot.DedupObservedResources,
		"deduplicated_resources", snapshot.DeduplicatedResources,
		"dedup_rate_basis_points", ratioBasisPoints(snapshot.DeduplicatedResources, snapshot.DedupObservedResources),
		"webhook_items_observed", snapshot.WebhookItemsObserved,
		"webhook_average_latency_milliseconds", snapshot.WebhookAverageLatencyMilliseconds,
		"webhook_max_latency_milliseconds", snapshot.WebhookMaxLatencyMilliseconds,
		"backfill_items_observed", snapshot.BackfillItemsObserved,
		"backfill_average_latency_milliseconds", snapshot.BackfillAverageLatencyMilliseconds,
		"backfill_max_latency_milliseconds", snapshot.BackfillMaxLatencyMilliseconds,
		"outbound_requests", snapshot.OutboundRequests,
		"outbound_completed", snapshot.OutboundCompleted,
		"outbound_success_rate_basis_points", ratioBasisPoints(snapshot.OutboundCompleted, snapshot.OutboundRequests),
		"customer_demand_workspaces", snapshot.CustomerDemandWorkspaces,
		"finalized_usage_events", snapshot.FinalizedUsageEvents,
		"finalized_weighted_units", snapshot.FinalizedWeightedUnits)

	for _, capacity := range snapshot.CapacityScopes {
		w.logCapacityMetric(capacity)
	}
	for _, usage := range snapshot.UsageMetrics {
		w.logger.Info("X Inbox operation and catalog usage",
			"event", "x_inbox_usage_metric",
			"operation_key", safeMetricLabel(usage.OperationKey),
			"catalog_version", safeMetricLabel(usage.CatalogVersion),
			"status", safeMetricLabel(usage.Status),
			"events", usage.Events,
			"weighted_units", usage.WeightedUnits)
	}
	w.logDailyCost(ctx, now, snapshot)
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

func (w *XInboxOperationsReconciliationWorker) logCapacityMetric(scope XInboxCapacityScope) {
	capacity := w.capacityFor(scope)
	w.logger.Info("X Inbox app-scoped delivery capacity",
		"event", "x_inbox_capacity_metric",
		"app_scope", scope.AppScope,
		"app_mode", safeMetricLabel(scope.AppMode),
		"resource", safeMetricLabel(scope.ResourceType),
		"used", scope.Used,
		"capacity", capacity)
	if capacity <= 0 {
		w.logger.Warn("X Inbox app capacity input is missing",
			"event", "x_inbox_reconciliation_alert",
			"kind", "app_capacity_input_missing",
			"app_scope", scope.AppScope,
			"resource", safeMetricLabel(scope.ResourceType))
		return
	}
	level := XInboxCapacityAlertLevel(scope.Used, capacity)
	if level == 0 {
		return
	}
	w.logger.Warn("X Inbox delivery capacity threshold crossed",
		"event", "x_inbox_capacity_alert",
		"app_scope", scope.AppScope,
		"app_mode", safeMetricLabel(scope.AppMode),
		"resource", safeMetricLabel(scope.ResourceType),
		"used", scope.Used,
		"capacity", capacity,
		"threshold_percent", level)
}

func (w *XInboxOperationsReconciliationWorker) capacityFor(scope XInboxCapacityScope) int64 {
	if scope.AppMode == "unipost_managed_app" || scope.AppScope == "managed" {
		if scope.ResourceType == "filtered_stream_rules" {
			return w.managedFilteredStreamRuleCapacity
		}
		if scope.ResourceType == "activity_subscriptions" {
			return w.managedActivitySubscriptionCapacity
		}
		return 0
	}
	capacity := w.workspaceAppCapacities[scope.AppScope]
	if scope.ResourceType == "filtered_stream_rules" {
		return capacity.FilteredStreamRules
	}
	if scope.ResourceType == "activity_subscriptions" {
		return capacity.ActivitySubscriptions
	}
	return 0
}

func (w *XInboxOperationsReconciliationWorker) logDailyCost(
	ctx context.Context,
	now time.Time,
	snapshot XInboxOperationsSnapshot,
) {
	if w.dailyCostInput == nil {
		w.logMissingDailyCost()
		return
	}
	cost, err := w.dailyCostInput.DailyCost(ctx, now)
	if err != nil || !cost.Available {
		w.logMissingDailyCost()
		return
	}
	variance := cost.ProviderCostMicros - cost.ExpectedFinalizedUsageCostMicros
	w.logger.Info("X Inbox daily provider cost variance",
		"event", "x_inbox_cost_variance",
		"provider_cost_micros", cost.ProviderCostMicros,
		"expected_finalized_usage_cost_micros", cost.ExpectedFinalizedUsageCostMicros,
		"variance_micros", variance,
		"variance_basis_points", signedRatioBasisPoints(variance, cost.ExpectedFinalizedUsageCostMicros),
		"finalized_usage_events", snapshot.FinalizedUsageEvents,
		"finalized_weighted_units", snapshot.FinalizedWeightedUnits)
}

func (w *XInboxOperationsReconciliationWorker) logMissingDailyCost() {
	w.logger.Warn("X Inbox daily provider cost input is unavailable",
		"event", "x_inbox_reconciliation_alert",
		"kind", "daily_cost_input_missing")
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
		{kind: "outbound_outcome_unknown", count: snapshot.OutboundOutcomeUnknown},
		{kind: "outbound_needs_reconciliation", count: snapshot.OutboundNeedsReconciliation},
		{kind: "stale_outbound_operation", count: snapshot.StaleOutboundOperations},
		{kind: "byo_unmetered_uncertain_write", count: snapshot.BYOUnmeteredUncertainWrites},
		{kind: "stale_backfill_confirmation", count: snapshot.StaleBackfillConfirmations},
		{kind: "exposure_needs_reconciliation", count: snapshot.ExposureNeedsReconciliation},
		{kind: "stale_exposure_reservation", count: snapshot.StaleExposureReservations},
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

func XInboxCapacityScopeKey(appMode, appIdentity string) string {
	if strings.TrimSpace(appMode) == "unipost_managed_app" {
		return "managed"
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(appIdentity)))
	return "workspace_" + hex.EncodeToString(sum[:8])
}

var opaqueWorkspaceScopePattern = regexp.MustCompile(`^workspace_[0-9a-f]{16}$`)

func ParseXInboxWorkspaceAppCapacities(raw string) (map[string]XInboxAppResourceCapacities, error) {
	result := make(map[string]XInboxAppResourceCapacities)
	if strings.TrimSpace(raw) == "" {
		return result, nil
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("decode workspace X app capacity configuration: %w", err)
	}
	for scope, capacity := range result {
		if !opaqueWorkspaceScopePattern.MatchString(scope) {
			return nil, errors.New("workspace X app capacity keys must be opaque workspace scope hashes")
		}
		if capacity.FilteredStreamRules < 0 || capacity.ActivitySubscriptions < 0 {
			return nil, errors.New("workspace X app capacities cannot be negative")
		}
	}
	return result, nil
}

func ratioBasisPoints(numerator, denominator int64) int64 {
	if numerator <= 0 || denominator <= 0 {
		return 0
	}
	return signedRatioBasisPoints(numerator, denominator)
}

func signedRatioBasisPoints(numerator, denominator int64) int64 {
	if denominator <= 0 {
		return 0
	}
	return int64(float64(numerator) * 10_000 / float64(denominator))
}

var safeMetricLabelPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,80}$`)

func safeMetricLabel(value string) string {
	if safeMetricLabelPattern.MatchString(value) {
		return value
	}
	return "unknown"
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
    COUNT(*) FILTER (WHERE status = 'reversed' AND updated_at >= $2)::BIGINT AS reversed,
    COUNT(*) FILTER (WHERE status = 'finalized' AND updated_at >= $2)::BIGINT AS finalized_events,
    COALESCE(SUM(weighted_units) FILTER (WHERE status = 'finalized' AND updated_at >= $2), 0)::BIGINT AS finalized_units
  FROM x_usage_events
), receipt_metrics AS (
  SELECT
    COUNT(*) FILTER (WHERE decision = 'suppressed_daily_cap')::BIGINT AS suppressed_cap,
    COUNT(*) FILTER (WHERE decision = 'suppressed_monthly_allowance')::BIGINT AS suppressed_allowance
  FROM x_inbound_event_receipts
  WHERE created_at >= $2
), delivery_metrics AS (
  SELECT
    COUNT(*) FILTER (WHERE delivery_status IN ('paused_cap', 'paused_allowance', 'paused_plan'))::BIGINT AS paused_sources,
    COALESCE(EXTRACT(EPOCH FROM ($1 - (MIN(updated_at) FILTER (
      WHERE delivery_status IN ('paused_cap', 'paused_allowance', 'paused_plan')
    )))), 0)::BIGINT AS pause_max_age_seconds,
    COUNT(*) FILTER (WHERE delivery_status = 'pending')::BIGINT AS restore_pending_sources,
    COALESCE(EXTRACT(EPOCH FROM ($1 - (MIN(updated_at) FILTER (
      WHERE delivery_status = 'pending'
    )))), 0)::BIGINT AS restore_pending_max_age_seconds,
    COUNT(*) FILTER (
      WHERE (delivery_status IN ('pending', 'error') AND updated_at < $3)
         OR (delivery_status = 'active' AND COALESCE(last_synced_at, updated_at) < $3)
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
), outbound_metrics AS (
  SELECT
    COUNT(*) FILTER (WHERE o.status = 'outcome_unknown')::BIGINT AS outcome_unknown,
    COUNT(*) FILTER (WHERE o.status = 'needs_reconciliation')::BIGINT AS needs_reconciliation,
    COUNT(*) FILTER (WHERE o.status = 'remote_succeeded')::BIGINT AS remote_succeeded,
    COUNT(*) FILTER (
      WHERE o.status NOT IN ('completed', 'succeeded') AND o.updated_at < $3
    )::BIGINT AS stale_operations,
    COUNT(*) FILTER (
      WHERE sa.x_app_mode = 'workspace_x_app'
        AND o.status IN ('outcome_unknown', 'needs_reconciliation')
        AND o.usage_event_id IS NULL
    )::BIGINT AS byo_unmetered_uncertain,
    COUNT(*) FILTER (WHERE o.created_at >= $2)::BIGINT AS requests,
    COUNT(*) FILTER (
      WHERE o.created_at >= $2 AND o.status IN ('completed', 'succeeded')
    )::BIGINT AS completed
  FROM x_inbox_outbound_requests o
  JOIN social_accounts sa ON sa.id = o.social_account_id
), confirmation_metrics AS (
  SELECT
    COUNT(*) FILTER (WHERE created_at >= $2)::BIGINT AS created,
    COUNT(*) FILTER (WHERE created_at >= $2 AND status = 'completed')::BIGINT AS completed,
    COUNT(*) FILTER (WHERE created_at >= $2 AND status = 'failed')::BIGINT AS failed,
    COUNT(*) FILTER (WHERE created_at >= $2 AND status = 'expired')::BIGINT AS expired,
    COUNT(*) FILTER (
      WHERE (status = 'running' AND execution_lease_expires_at < $1)
         OR (status = 'pending' AND expires_at < $1)
    )::BIGINT AS stale,
    COALESCE(SUM(estimated_x_credits) FILTER (WHERE created_at >= $2), 0)::BIGINT AS estimated_credits,
    COALESCE(SUM(
      CASE WHEN result->>'read' ~ '^[0-9]+$' THEN (result->>'read')::BIGINT ELSE 0 END
    ) FILTER (WHERE created_at >= $2 AND status = 'completed'), 0)::BIGINT AS dedup_observed,
    COALESCE(SUM(
      CASE WHEN result->>'duplicates' ~ '^[0-9]+$' THEN (result->>'duplicates')::BIGINT ELSE 0 END
    ) FILTER (WHERE created_at >= $2 AND status = 'completed'), 0)::BIGINT AS deduplicated,
    COUNT(*) FILTER (
      WHERE created_at >= $2 AND status = 'completed'
        AND started_at IS NOT NULL AND completed_at IS NOT NULL
    )::BIGINT AS latency_observed,
    COALESCE(ROUND(AVG(GREATEST(0, EXTRACT(EPOCH FROM (completed_at - started_at)) * 1000)) FILTER (
      WHERE created_at >= $2 AND status = 'completed'
        AND started_at IS NOT NULL AND completed_at IS NOT NULL
    )), 0)::BIGINT AS average_latency_ms,
    COALESCE(ROUND(MAX(GREATEST(0, EXTRACT(EPOCH FROM (completed_at - started_at)) * 1000)) FILTER (
      WHERE created_at >= $2 AND status = 'completed'
        AND started_at IS NOT NULL AND completed_at IS NOT NULL
    )), 0)::BIGINT AS max_latency_ms
  FROM x_inbox_backfill_confirmation_operations
), exposure_metrics AS (
  SELECT
    COUNT(*) FILTER (WHERE status = 'reserved')::BIGINT AS reserved,
    COUNT(*) FILTER (WHERE status = 'read_started')::BIGINT AS read_started,
    COUNT(*) FILTER (WHERE status = 'finalize_pending')::BIGINT AS finalize_pending,
    COUNT(*) FILTER (WHERE status = 'release_pending')::BIGINT AS release_pending,
    COUNT(*) FILTER (WHERE status = 'needs_reconciliation')::BIGINT AS needs_reconciliation,
    COUNT(*) FILTER (
      WHERE status NOT IN ('finalized', 'released')
        AND (updated_at < $3 OR reconciliation_deadline < $1)
    )::BIGINT AS stale
  FROM x_inbox_backfill_exposure_reservations
), latency_metrics AS (
  SELECT
    COUNT(*) FILTER (WHERE metadata->>'backfill' IS DISTINCT FROM 'true')::BIGINT AS webhook_items,
    COALESCE(ROUND(AVG(GREATEST(0, EXTRACT(EPOCH FROM (created_at - received_at)) * 1000)) FILTER (
      WHERE metadata->>'backfill' IS DISTINCT FROM 'true'
    )), 0)::BIGINT AS webhook_average_ms,
    COALESCE(ROUND(MAX(GREATEST(0, EXTRACT(EPOCH FROM (created_at - received_at)) * 1000)) FILTER (
      WHERE metadata->>'backfill' IS DISTINCT FROM 'true'
    )), 0)::BIGINT AS webhook_max_ms
  FROM inbox_items
  WHERE source IN ('x_reply', 'x_dm') AND created_at >= $2
), demand_metrics AS (
  SELECT COUNT(DISTINCT workspace_id)::BIGINT AS workspaces
  FROM (
    SELECT workspace_id FROM x_inbound_event_receipts WHERE created_at >= $2
    UNION
    SELECT workspace_id FROM x_inbox_outbound_requests WHERE created_at >= $2
    UNION
    SELECT workspace_id FROM x_inbox_backfill_confirmation_operations WHERE created_at >= $2
  ) demand
)
SELECT
  usage_metrics.provisional,
  usage_metrics.stale_provisional,
  usage_metrics.reversed,
  receipt_metrics.suppressed_cap,
  receipt_metrics.suppressed_allowance,
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
	cleanup_metrics.oldest_age_seconds,
	outbound_metrics.outcome_unknown,
	outbound_metrics.needs_reconciliation,
	outbound_metrics.remote_succeeded,
	outbound_metrics.stale_operations,
	outbound_metrics.byo_unmetered_uncertain,
	outbound_metrics.requests,
	outbound_metrics.completed,
	confirmation_metrics.created,
	confirmation_metrics.completed,
	confirmation_metrics.failed,
	confirmation_metrics.expired,
	confirmation_metrics.stale,
	confirmation_metrics.estimated_credits,
	confirmation_metrics.dedup_observed,
	confirmation_metrics.deduplicated,
	exposure_metrics.reserved,
	exposure_metrics.read_started,
	exposure_metrics.finalize_pending,
	exposure_metrics.release_pending,
	exposure_metrics.needs_reconciliation,
	exposure_metrics.stale,
	latency_metrics.webhook_items,
	latency_metrics.webhook_average_ms,
	latency_metrics.webhook_max_ms,
	confirmation_metrics.latency_observed,
	confirmation_metrics.average_latency_ms,
	confirmation_metrics.max_latency_ms,
	demand_metrics.workspaces,
	usage_metrics.finalized_events,
	usage_metrics.finalized_units
FROM usage_metrics, receipt_metrics, delivery_metrics, notification_metrics,
     cleanup_metrics, outbound_metrics, confirmation_metrics, exposure_metrics,
     latency_metrics, demand_metrics`,
		now, windowStart, staleBefore)

	var snapshot XInboxOperationsSnapshot
	err := row.Scan(
		&snapshot.ProvisionalUsageEvents,
		&snapshot.StaleProvisionalUsageEvents,
		&snapshot.ReversedUsageEvents,
		&snapshot.SuppressedDailyCapEvents,
		&snapshot.SuppressedAllowanceEvents,
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
		&snapshot.OutboundOutcomeUnknown,
		&snapshot.OutboundNeedsReconciliation,
		&snapshot.OutboundRemoteSucceeded,
		&snapshot.StaleOutboundOperations,
		&snapshot.BYOUnmeteredUncertainWrites,
		&snapshot.OutboundRequests,
		&snapshot.OutboundCompleted,
		&snapshot.BackfillConfirmationsCreated,
		&snapshot.BackfillConfirmationsCompleted,
		&snapshot.BackfillConfirmationsFailed,
		&snapshot.BackfillConfirmationsExpired,
		&snapshot.StaleBackfillConfirmations,
		&snapshot.BackfillEstimatedCredits,
		&snapshot.DedupObservedResources,
		&snapshot.DeduplicatedResources,
		&snapshot.ExposureReserved,
		&snapshot.ExposureReadStarted,
		&snapshot.ExposureFinalizePending,
		&snapshot.ExposureReleasePending,
		&snapshot.ExposureNeedsReconciliation,
		&snapshot.StaleExposureReservations,
		&snapshot.WebhookItemsObserved,
		&snapshot.WebhookAverageLatencyMilliseconds,
		&snapshot.WebhookMaxLatencyMilliseconds,
		&snapshot.BackfillItemsObserved,
		&snapshot.BackfillAverageLatencyMilliseconds,
		&snapshot.BackfillMaxLatencyMilliseconds,
		&snapshot.CustomerDemandWorkspaces,
		&snapshot.FinalizedUsageEvents,
		&snapshot.FinalizedWeightedUnits,
	)
	if err != nil {
		return XInboxOperationsSnapshot{}, err
	}

	usageRows, err := s.pool.Query(ctx, `
SELECT operation_key, catalog_version, status,
       COUNT(*)::BIGINT, COALESCE(SUM(weighted_units), 0)::BIGINT
FROM x_usage_events
WHERE created_at >= $1
GROUP BY operation_key, catalog_version, status
ORDER BY operation_key, catalog_version, status`, windowStart)
	if err != nil {
		return XInboxOperationsSnapshot{}, err
	}
	defer usageRows.Close()
	for usageRows.Next() {
		var metric XInboxUsageMetric
		if err := usageRows.Scan(
			&metric.OperationKey,
			&metric.CatalogVersion,
			&metric.Status,
			&metric.Events,
			&metric.WeightedUnits,
		); err != nil {
			return XInboxOperationsSnapshot{}, err
		}
		snapshot.UsageMetrics = append(snapshot.UsageMetrics, metric)
	}
	if err := usageRows.Err(); err != nil {
		return XInboxOperationsSnapshot{}, err
	}

	capacityRows, err := s.pool.Query(ctx, `
WITH capacity_resources AS (
  SELECT
    sa.x_app_mode AS app_mode,
    CASE
      WHEN sa.x_app_mode = 'unipost_managed_app' THEN 'managed'
      ELSE COALESCE(NULLIF(pc.client_id, ''), 'account:' || sa.id)
    END AS app_identity,
    'filtered_stream_rules'::TEXT AS resource_type,
    r.filtered_stream_rule_id AS resource_id
  FROM x_inbox_delivery_resources r
  JOIN social_accounts sa ON sa.id = r.social_account_id
  JOIN profiles p ON p.id = sa.profile_id
  LEFT JOIN platform_credentials pc
    ON pc.workspace_id = p.workspace_id AND pc.platform = 'twitter'
  WHERE r.filtered_stream_rule_id IS NOT NULL

  UNION

  SELECT
    sa.x_app_mode,
    CASE
      WHEN sa.x_app_mode = 'unipost_managed_app' THEN 'managed'
      ELSE COALESCE(NULLIF(pc.client_id, ''), 'account:' || sa.id)
    END,
    'activity_subscriptions'::TEXT,
    r.activity_dm_subscription_id
  FROM x_inbox_delivery_resources r
  JOIN social_accounts sa ON sa.id = r.social_account_id
  JOIN profiles p ON p.id = sa.profile_id
  LEFT JOIN platform_credentials pc
    ON pc.workspace_id = p.workspace_id AND pc.platform = 'twitter'
  WHERE r.activity_dm_subscription_id IS NOT NULL

  UNION

  SELECT x_app_mode, source_app_identity, 'filtered_stream_rules'::TEXT, filtered_stream_rule_id
  FROM x_inbox_delivery_cleanup_intents
  WHERE filtered_stream_rule_id IS NOT NULL

  UNION

  SELECT x_app_mode, source_app_identity, 'activity_subscriptions'::TEXT, activity_dm_subscription_id
  FROM x_inbox_delivery_cleanup_intents
  WHERE activity_dm_subscription_id IS NOT NULL
)
SELECT app_mode, app_identity, resource_type, COUNT(DISTINCT resource_id)::BIGINT
FROM capacity_resources
GROUP BY app_mode, app_identity, resource_type
ORDER BY app_mode, app_identity, resource_type`)
	if err != nil {
		return XInboxOperationsSnapshot{}, err
	}
	defer capacityRows.Close()
	for capacityRows.Next() {
		var appIdentity string
		var scope XInboxCapacityScope
		if err := capacityRows.Scan(
			&scope.AppMode,
			&appIdentity,
			&scope.ResourceType,
			&scope.Used,
		); err != nil {
			return XInboxOperationsSnapshot{}, err
		}
		scope.AppScope = XInboxCapacityScopeKey(scope.AppMode, appIdentity)
		snapshot.CapacityScopes = append(snapshot.CapacityScopes, scope)
	}
	if err := capacityRows.Err(); err != nil {
		return XInboxOperationsSnapshot{}, err
	}
	return snapshot, nil
}
