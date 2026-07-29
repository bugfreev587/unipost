package observabilityreads

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type panicTypedNilMetricsBeginner struct{}

func (*panicTypedNilMetricsBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	panic("typed-nil metrics database must not be called")
}

type rollbackDeadlineMetricsTx struct {
	pgx.Tx
	rollbackCalled      bool
	rollbackHasDeadline bool
	rollbackDeadline    time.Time
}

func (tx *rollbackDeadlineMetricsTx) Rollback(ctx context.Context) error {
	tx.rollbackCalled = true
	tx.rollbackDeadline, tx.rollbackHasDeadline = ctx.Deadline()
	return nil
}

type rollbackDeadlineMetricsDB struct{ tx *rollbackDeadlineMetricsTx }

func (db rollbackDeadlineMetricsDB) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return db.tx, nil
}

func TestMetricsSnapshotRollbackUsesBoundedCleanupContextAfterCancelledLoad(t *testing.T) {
	tx := &rollbackDeadlineMetricsTx{}
	store := NewPostgresMetricsStore(rollbackDeadlineMetricsDB{tx: tx})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := withMetricsSnapshot(ctx, store, func(metricsDB, time.Time) (int, MetricsFreshness, error) {
		return 0, MetricsFreshness{}, errors.New("load failed")
	})
	if err == nil {
		t.Fatal("snapshot load error was discarded")
	}
	if !tx.rollbackCalled || !tx.rollbackHasDeadline {
		t.Fatalf("rollback called/deadline = %v/%v, want true/true", tx.rollbackCalled, tx.rollbackHasDeadline)
	}
	remaining := time.Until(tx.rollbackDeadline)
	if remaining <= 0 || remaining > rollupRollbackTimeout {
		t.Fatalf("rollback deadline remaining = %v, want within (0,%v]", remaining, rollupRollbackTimeout)
	}
}

func TestPostgresMetricsStoreTypedNilDatabaseFailsClosed(t *testing.T) {
	var database *panicTypedNilMetricsBeginner
	store := NewPostgresMetricsStore(database)
	now := time.Date(2026, 7, 29, 18, 0, 0, 0, time.UTC)
	if _, _, err := store.Overall(context.Background(), MetricsQuery{From: now.Add(-time.Hour), To: now}); err == nil {
		t.Fatal("typed-nil metrics database was accepted")
	}
}

func TestMetricsSourcePlanUsesRawForRecentRange(t *testing.T) {
	now := time.Date(2026, 7, 29, 18, 37, 0, 0, time.UTC)
	plan := BuildMetricsSourcePlan(
		now.Add(-24*time.Hour),
		now,
		now,
	)

	if len(plan.RollupRanges) != 0 {
		t.Fatalf("rollup ranges = %#v, want none", plan.RollupRanges)
	}
	if len(plan.RawRanges) != 1 || !plan.RawRanges[0].From.Equal(now.Add(-24*time.Hour)) || !plan.RawRanges[0].To.Equal(now) {
		t.Fatalf("raw ranges = %#v, want entire recent range", plan.RawRanges)
	}
	if plan.State != MetricsDataExact {
		t.Fatalf("state = %q, want %q", plan.State, MetricsDataExact)
	}
}

func TestMetricsSourcePlanUsesRollupsBeyondShortestRawRetention(t *testing.T) {
	now := time.Date(2026, 7, 29, 18, 37, 0, 0, time.UTC)
	from := now.Add(-36 * time.Hour)
	plan := BuildMetricsSourcePlan(from, now, now)
	wantRollupTo := now.Truncate(time.Hour).Add(-23 * time.Hour)
	if len(plan.RollupRanges) != 1 || !plan.RollupRanges[0].To.Equal(wantRollupTo) {
		t.Fatalf("rollup ranges = %#v, want completed hours through %s", plan.RollupRanges, wantRollupTo)
	}
	for _, rawRange := range plan.RawRanges {
		if rawRange.From.Before(wantRollupTo) && !rawRange.To.Equal(plan.RollupRanges[0].From) {
			t.Fatalf("unsafe raw range beyond guaranteed retention = %#v", rawRange)
		}
	}
}

func TestMetricsSourcePlanPreservesRecentAndHistoricalInclusivePointQueries(t *testing.T) {
	now := time.Date(2026, 7, 29, 18, 37, 0, 0, time.UTC)
	recent := now.Add(-time.Hour)
	recentPlan := BuildMetricsSourcePlan(recent, recent, now)
	if len(recentPlan.RawRanges) != 1 || !recentPlan.RawRanges[0].From.Equal(recent) || !recentPlan.RawRanges[0].To.Equal(recent) || recentPlan.State != MetricsDataExact {
		t.Fatalf("recent point plan = %#v, want exact raw point", recentPlan)
	}

	historical := now.Truncate(time.Hour).Add(-48 * time.Hour)
	historicalPlan := BuildMetricsSourcePlan(historical, historical, now)
	if len(historicalPlan.RawRanges) != 1 || len(historicalPlan.HistoricalPartialRawRanges) != 1 {
		t.Fatalf("historical point plan = %#v, want raw point with retention evidence", historicalPlan)
	}
	historicalPlan.MarkHistoricalPartialRawEvidence(false)
	if historicalPlan.State != MetricsDataDelayed {
		t.Fatalf("historical point state = %q, want delayed when point raw expired", historicalPlan.State)
	}
}

func TestMetricsSourcePlanAddsInclusiveHistoricalAlignedToPointTail(t *testing.T) {
	now := time.Date(2026, 7, 29, 18, 37, 0, 0, time.UTC)
	from := now.Truncate(time.Hour).Add(-72 * time.Hour)
	to := now.Truncate(time.Hour).Add(-48 * time.Hour)
	plan := BuildMetricsSourcePlan(from, to, now)
	if len(plan.RollupRanges) != 1 || len(plan.RawRanges) != 1 || !plan.RawRanges[0].From.Equal(to) || !plan.RawRanges[0].To.Equal(to) {
		t.Fatalf("aligned historical endpoint plan = %#v, want rollup range plus inclusive raw point tail", plan)
	}
	if len(plan.HistoricalPartialRawRanges) != 1 {
		t.Fatalf("historical point evidence ranges = %#v, want one", plan.HistoricalPartialRawRanges)
	}
}

func TestMetricsSourcePlanSeparatesHistoricalFullHoursWithoutDoubleCount(t *testing.T) {
	now := time.Date(2026, 7, 29, 18, 37, 0, 0, time.UTC)
	from := time.Date(2026, 7, 25, 10, 15, 0, 0, time.UTC)
	to := now
	plan := BuildMetricsSourcePlan(from, to, now)

	wantRollupFrom := time.Date(2026, 7, 25, 11, 0, 0, 0, time.UTC)
	wantRollupTo := time.Date(2026, 7, 28, 19, 0, 0, 0, time.UTC)
	if len(plan.RollupRanges) != 1 || !plan.RollupRanges[0].From.Equal(wantRollupFrom) || !plan.RollupRanges[0].To.Equal(wantRollupTo) {
		t.Fatalf("rollup ranges = %#v, want [%s,%s)", plan.RollupRanges, wantRollupFrom, wantRollupTo)
	}
	if len(plan.RawRanges) != 2 {
		t.Fatalf("raw ranges = %#v, want historical partial plus recent", plan.RawRanges)
	}
	if !plan.RawRanges[0].From.Equal(from) || !plan.RawRanges[0].To.Equal(wantRollupFrom) {
		t.Fatalf("first raw range = %#v, want [%s,%s)", plan.RawRanges[0], from, wantRollupFrom)
	}
	if !plan.RawRanges[1].From.Equal(wantRollupTo) || !plan.RawRanges[1].To.Equal(to) {
		t.Fatalf("second raw range = %#v, want [%s,%s]", plan.RawRanges[1], wantRollupTo, to)
	}
	if !plan.UsesApproximatePercentiles || plan.State != MetricsDataApproximate {
		t.Fatalf("approximate/state = %v/%q, want true/%q", plan.UsesApproximatePercentiles, plan.State, MetricsDataApproximate)
	}
}

func TestMetricsSourcePlanMarksIncompleteHistoricalRollupsDelayed(t *testing.T) {
	now := time.Date(2026, 7, 29, 18, 37, 0, 0, time.UTC)
	plan := BuildMetricsSourcePlan(now.Add(-72*time.Hour), now, now)
	plan.MarkRollupCompletion(0)

	if plan.State != MetricsDataDelayed {
		t.Fatalf("state = %q, want %q", plan.State, MetricsDataDelayed)
	}
	if plan.MissingRollupHours != 48 {
		t.Fatalf("missing hours = %d, want 48 completed full hours", plan.MissingRollupHours)
	}
}

func TestMetricsSourcePlanMarksMissingHistoricalPartialRawDelayed(t *testing.T) {
	now := time.Date(2026, 7, 29, 18, 37, 0, 0, time.UTC)
	plan := BuildMetricsSourcePlan(time.Date(2026, 7, 25, 10, 15, 0, 0, time.UTC), now, now)
	if len(plan.HistoricalPartialRawRanges) != 1 {
		t.Fatalf("historical partial ranges = %#v, want one", plan.HistoricalPartialRawRanges)
	}
	plan.MarkHistoricalPartialRawEvidence(false)
	if plan.State != MetricsDataDelayed {
		t.Fatalf("state = %q, want delayed when old partial-hour raw is unavailable", plan.State)
	}
}

func TestMetricsSourcePlanTracksHistoricalTailPartialWithoutDuplicatingRawRange(t *testing.T) {
	now := time.Date(2026, 7, 29, 18, 37, 0, 0, time.UTC)
	from := time.Date(2026, 7, 25, 10, 5, 0, 0, time.UTC)
	to := from.Add(10 * time.Minute)
	plan := BuildMetricsSourcePlan(from, to, now)
	if len(plan.RawRanges) != 1 || !plan.RawRanges[0].From.Equal(from) || !plan.RawRanges[0].To.Equal(to) {
		t.Fatalf("raw ranges = %#v, want one exact historical partial range", plan.RawRanges)
	}
	if len(plan.HistoricalPartialRawRanges) != 1 {
		t.Fatalf("historical partial ranges = %#v, want trailing partial tracked", plan.HistoricalPartialRawRanges)
	}
	plan.MarkHistoricalPartialRawEvidence(false)
	if plan.State != MetricsDataDelayed {
		t.Fatalf("state = %q, want delayed", plan.State)
	}
}

func TestMetricsSourcePlanKeepsContiguousHistoricalRawSpanExactWhenNoFullHourExists(t *testing.T) {
	now := time.Date(2026, 7, 29, 18, 37, 0, 0, time.UTC)
	from := time.Date(2026, 7, 25, 1, 10, 0, 0, time.UTC)
	to := time.Date(2026, 7, 25, 2, 20, 0, 0, time.UTC)
	plan := BuildMetricsSourcePlan(from, to, now)
	if len(plan.RollupRanges) != 0 || len(plan.RawRanges) != 1 {
		t.Fatalf("rollup/raw ranges = %#v/%#v, want zero/one contiguous raw query", plan.RollupRanges, plan.RawRanges)
	}
	plan.MarkHistoricalPartialRawEvidence(true)
	if plan.State != MetricsDataExact || plan.UsesApproximatePercentiles {
		t.Fatalf("state/approximate = %q/%v, want exact/false", plan.State, plan.UsesApproximatePercentiles)
	}
}

func TestAdminWorkspaceSlowestEndpointTieUsesLexicographicallySmallerPath(t *testing.T) {
	currentPath, currentP95 := "/v1/z", int64(500)
	if !shouldReplaceSlowestEndpoint(currentPath, currentP95, "/v1/a", 500) {
		t.Fatal("equal-p95 lexicographically smaller endpoint must win")
	}
	if shouldReplaceSlowestEndpoint("/v1/a", 500, "/v1/z", 500) {
		t.Fatal("equal-p95 lexicographically larger endpoint must not replace")
	}
}

func TestMetricsAggregateRawExactAndMixedHistogramApproximate(t *testing.T) {
	raw := metricAggregate{
		RequestCount:        4,
		SuccessCount:        3,
		ServerErrorCount:    1,
		DurationSumMS:       1040,
		DurationMinMS:       10,
		DurationMaxMS:       900,
		DurationHistogram:   [durationHistogramBucketCount]int64{0, 1, 1, 0, 0, 1, 0, 1},
		ExactP50MS:          25,
		ExactP95MS:          900,
		ExactP99MS:          900,
		HasExactPercentiles: true,
	}
	got, err := raw.Overall()
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalCalls != 4 || got.SuccessCount != 3 || got.ServerErrorCount != 1 || got.P50MS != 25 || got.P95MS != 900 || got.ErrorRatePct != 25 {
		t.Fatalf("exact overall = %#v", got)
	}

	rollup := metricAggregate{
		RequestCount:      2,
		ClientErrorCount:  2,
		RateLimitedCount:  2,
		DurationSumMS:     500,
		DurationMinMS:     250,
		DurationMaxMS:     250,
		DurationHistogram: [durationHistogramBucketCount]int64{0, 0, 0, 0, 0, 2},
		UsedRollup:        true,
	}
	if err := raw.Merge(rollup); err != nil {
		t.Fatal(err)
	}
	got, err = raw.Overall()
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalCalls != 6 || got.ClientErrorCount != 2 || got.RateLimitedCount != 2 {
		t.Fatalf("mixed counts = %#v", got)
	}
	if got.P50MS != 250 || got.P95MS != 1000 {
		t.Fatalf("mixed percentiles = p50 %d p95 %d, want bucket bounds 250/1000", got.P50MS, got.P95MS)
	}
}

func TestMetricsAggregateAverageRoundsLikePostgresIntegerCast(t *testing.T) {
	aggregate := metricAggregate{
		RequestCount: 2, SuccessCount: 2, DurationSumMS: 3,
		DurationMinMS: 1, DurationMaxMS: 2,
		DurationHistogram: [durationHistogramBucketCount]int64{2},
		ExactP50MS:        2, ExactP95MS: 2, ExactP99MS: 2, HasExactPercentiles: true,
	}
	got, err := aggregate.Overall()
	if err != nil {
		t.Fatal(err)
	}
	if got.AvgMS != 2 {
		t.Fatalf("avg_ms = %d, want Postgres-compatible rounded 2", got.AvgMS)
	}
}

func TestMetricsAggregateRejectsCorruptionAndCheckedMergeOverflow(t *testing.T) {
	corrupt := metricAggregate{RequestCount: 1, SuccessCount: 1, DurationMaxMS: 100, DurationHistogram: [durationHistogramBucketCount]int64{1}}
	if _, err := corrupt.Overall(); err == nil {
		t.Fatal("Overall accepted histogram/maximum corruption")
	}

	left := metricAggregate{RequestCount: math.MaxInt64, SuccessCount: math.MaxInt64, DurationMaxMS: 5, DurationHistogram: [durationHistogramBucketCount]int64{math.MaxInt64}}
	right := metricAggregate{RequestCount: 1, SuccessCount: 1, DurationMaxMS: 5, DurationHistogram: [durationHistogramBucketCount]int64{1}}
	if err := left.Merge(right); err == nil {
		t.Fatal("Merge accepted int64 request-count overflow")
	}
}

func TestRawAndRollupDurationGroupingSQLCoverEveryDurationResponseModeInUTC(t *testing.T) {
	tests := []struct {
		mode          metricGroupMode
		wantRawGroup  string
		wantRollGroup string
	}{
		{metricGroupOverall, "", ""},
		{metricGroupSummary, "route_pattern, method", "r.route_pattern, r.method"},
		{metricGroupTrendHour, "AT TIME ZONE 'UTC'", "AT TIME ZONE 'UTC'"},
		{metricGroupTrendDay, "AT TIME ZONE 'UTC'", "AT TIME ZONE 'UTC'"},
		{metricGroupWorkspaceTotal, "e.workspace_id, w.name", "r.workspace_id, w.name"},
		{metricGroupWorkspaceRoute, "e.workspace_id, w.name, e.route_pattern", "r.workspace_id, w.name, r.route_pattern"},
	}
	for _, test := range tests {
		t.Run(string(test.mode), func(t *testing.T) {
			rawSelect, rawGroup, err := rawGroupExpressions(test.mode)
			if err != nil {
				t.Fatal(err)
			}
			rollupSelect, rollupGroup, err := rollupGroupExpressions(test.mode)
			if err != nil {
				t.Fatal(err)
			}
			if test.wantRawGroup != "" && !strings.Contains(rawGroup, test.wantRawGroup) {
				t.Fatalf("raw group = %q, want %q", rawGroup, test.wantRawGroup)
			}
			if test.wantRollGroup != "" && !strings.Contains(rollupGroup, test.wantRollGroup) {
				t.Fatalf("rollup group = %q, want %q", rollupGroup, test.wantRollGroup)
			}
			if test.mode == metricGroupTrendHour || test.mode == metricGroupTrendDay {
				if !strings.Contains(rawSelect, "AT TIME ZONE 'UTC'") || !strings.Contains(rollupSelect, "AT TIME ZONE 'UTC'") {
					t.Fatalf("trend selects are not explicitly UTC: raw=%q rollup=%q", rawSelect, rollupSelect)
				}
			}
		})
	}
}
