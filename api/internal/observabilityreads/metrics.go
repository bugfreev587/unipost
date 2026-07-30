package observabilityreads

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	// Free is the shortest planned physical retention tier. A request wholly
	// inside this duration can safely use raw events. For longer requests the
	// handoff is hour-aligned one hour inside the retention boundary, so every
	// raw-backed completed hour is guaranteed to remain physically available.
	metricsShortestRawRetention        = 24 * time.Hour
	metricsGuaranteedRawCompletedHours = 23
)

type MetricsDataState string

const (
	MetricsDataExact       MetricsDataState = "exact"
	MetricsDataApproximate MetricsDataState = "approximate"
	MetricsDataDelayed     MetricsDataState = "delayed"
)

type MetricsRange struct {
	From time.Time
	To   time.Time
}

type MetricsSourcePlan struct {
	RawRanges                  []MetricsRange
	HistoricalPartialRawRanges []MetricsRange
	RollupRanges               []MetricsRange
	State                      MetricsDataState
	UsesApproximatePercentiles bool
	MissingRollupHours         int
}

func BuildMetricsSourcePlan(from, to, now time.Time) MetricsSourcePlan {
	from = from.UTC()
	to = to.UTC()
	if from.After(to) {
		return MetricsSourcePlan{State: MetricsDataExact}
	}
	guaranteedRawCutoff := now.UTC().Add(-metricsShortestRawRetention)
	if from.Equal(to) {
		plan := MetricsSourcePlan{RawRanges: []MetricsRange{{From: from, To: to}}, State: MetricsDataExact}
		if from.Before(guaranteedRawCutoff) {
			plan.HistoricalPartialRawRanges = append(plan.HistoricalPartialRawRanges, MetricsRange{From: from, To: to})
		}
		return plan
	}
	if !from.Before(guaranteedRawCutoff) {
		return MetricsSourcePlan{RawRanges: []MetricsRange{{From: from, To: to}}, State: MetricsDataExact}
	}
	recentBoundary := now.UTC().Truncate(time.Hour).Add(-time.Duration(metricsGuaranteedRawCompletedHours) * time.Hour)
	rollupFrom := ceilHour(from)
	rollupTo := recentBoundary
	if floor := to.Truncate(time.Hour); floor.Before(rollupTo) {
		rollupTo = floor
	}
	plan := MetricsSourcePlan{State: MetricsDataExact}
	if rollupFrom.Before(rollupTo) {
		plan.RollupRanges = []MetricsRange{{From: rollupFrom, To: rollupTo}}
		plan.State = MetricsDataApproximate
		plan.UsesApproximatePercentiles = true
		if from.Before(rollupFrom) {
			plan.RawRanges = append(plan.RawRanges, MetricsRange{From: from, To: minTime(to, rollupFrom)})
		}
		if rollupTo.Before(to) {
			plan.RawRanges = append(plan.RawRanges, MetricsRange{From: maxTime(from, rollupTo), To: to})
		} else if rollupTo.Equal(to) {
			// Legacy ranges include `to`. A completed rollup ending at `to`
			// contains only values before it, so retain an explicit raw point.
			plan.RawRanges = append(plan.RawRanges, MetricsRange{From: to, To: to})
		}
	} else {
		plan.RawRanges = []MetricsRange{{From: from, To: to}}
	}
	for _, rawRange := range plan.RawRanges {
		historicalEnd := minTime(rawRange.To, guaranteedRawCutoff)
		if rawRange.From.Before(historicalEnd) || (rawRange.From.Equal(rawRange.To) && rawRange.From.Before(guaranteedRawCutoff)) {
			plan.HistoricalPartialRawRanges = append(plan.HistoricalPartialRawRanges, MetricsRange{From: rawRange.From, To: historicalEnd})
		}
	}
	return plan
}

func (p *MetricsSourcePlan) MarkHistoricalPartialRawEvidence(available bool) {
	if len(p.HistoricalPartialRawRanges) > 0 && !available {
		p.State = MetricsDataDelayed
	}
}

func (p *MetricsSourcePlan) MarkRollupCompletion(completedHours int) {
	requested := 0
	for _, item := range p.RollupRanges {
		requested += int(item.To.Sub(item.From) / time.Hour)
	}
	if completedHours < 0 {
		completedHours = 0
	}
	if completedHours > requested {
		completedHours = requested
	}
	p.MissingRollupHours = requested - completedHours
	if p.MissingRollupHours > 0 {
		p.State = MetricsDataDelayed
	}
}

func ceilHour(value time.Time) time.Time {
	truncated := value.Truncate(time.Hour)
	if value.Equal(truncated) {
		return truncated
	}
	return truncated.Add(time.Hour)
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

type MetricsFreshness struct {
	DataState          MetricsDataState `json:"data_state"`
	Approximate        bool             `json:"percentiles_approximate"`
	MissingRollupHours int              `json:"missing_rollup_hours,omitempty"`
}

type MetricsQuery struct {
	From, To      time.Time
	WorkspaceID   string
	Method        string
	Path          string
	StatusClass   string
	Sort          string
	Interval      string
	Limit         int32
	MinCalls      int32
	ApplyMinCalls bool
}

type MetricsOverall struct {
	TotalCalls           int64   `json:"total_calls"`
	SuccessCount         int64   `json:"success_count"`
	ClientErrorCount     int64   `json:"client_error_count"`
	ServerErrorCount     int64   `json:"server_error_count"`
	RateLimitedCount     int64   `json:"rate_limited_count"`
	ErrorRatePct         float64 `json:"error_rate_pct"`
	ServerFailureRatePct float64 `json:"server_failure_rate_pct"`
	ReliabilityPct       float64 `json:"reliability_pct"`
	P50MS                int64   `json:"p50_ms"`
	P95MS                int64   `json:"p95_ms"`
	P99MS                int64   `json:"p99_ms"`
	AvgMS                int64   `json:"avg_ms"`
}

type MetricsSummaryRow struct {
	Path                 string  `json:"path"`
	Method               string  `json:"method"`
	TotalCalls           int64   `json:"total_calls"`
	SuccessCount         int64   `json:"success_count"`
	ClientErrorCount     int64   `json:"client_error_count"`
	ServerErrorCount     int64   `json:"server_error_count"`
	RateLimitedCount     int64   `json:"rate_limited_count"`
	ErrorRatePct         float64 `json:"error_rate_pct"`
	ServerFailureRatePct float64 `json:"server_failure_rate_pct"`
	P50MS                int64   `json:"p50_ms"`
	P95MS                int64   `json:"p95_ms"`
	P99MS                int64   `json:"p99_ms"`
	AvgMS                int64   `json:"avg_ms"`
}

type MetricsTrendRow struct {
	Bucket           time.Time `json:"bucket"`
	TotalCalls       int64     `json:"total_calls"`
	SuccessCount     int64     `json:"success_count"`
	ErrorCount       int64     `json:"error_count"`
	ClientErrorCount int64     `json:"client_error_count"`
	ServerErrorCount int64     `json:"server_error_count"`
	RateLimitedCount int64     `json:"rate_limited_count"`
	P50MS            int64     `json:"p50_ms"`
	P95MS            int64     `json:"p95_ms"`
	P99MS            int64     `json:"p99_ms"`
	AvgMS            int64     `json:"avg_ms"`
}

type MetricsStatusCodeRow struct {
	StatusCode int32  `json:"status_code"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	TotalCalls int64  `json:"total_calls"`
}

type AdminMetricsWorkspaceRow struct {
	WorkspaceID          string  `json:"workspace_id"`
	WorkspaceName        string  `json:"workspace_name"`
	TotalCalls           int64   `json:"total_calls"`
	RateLimitedCount     int64   `json:"rate_limited_count"`
	ErrorRatePct         float64 `json:"error_rate_pct"`
	ServerFailureRatePct float64 `json:"server_failure_rate_pct"`
	P95MS                int64   `json:"p95_ms"`
	P99MS                int64   `json:"p99_ms"`
	SlowestEndpoint      string  `json:"slowest_endpoint"`
	SlowestEndpointP95MS int64   `json:"slowest_endpoint_p95_ms"`
}

type MetricsReader interface {
	Overall(context.Context, MetricsQuery) (MetricsOverall, MetricsFreshness, error)
	Summary(context.Context, MetricsQuery) ([]MetricsSummaryRow, MetricsFreshness, error)
	Trend(context.Context, MetricsQuery) ([]MetricsTrendRow, MetricsFreshness, error)
	StatusCodes(context.Context, MetricsQuery) ([]MetricsStatusCodeRow, MetricsFreshness, error)
	Workspaces(context.Context, MetricsQuery) ([]AdminMetricsWorkspaceRow, MetricsFreshness, error)
}

func SelectMetricsReader(ctx context.Context, selector ReadPathSelector, reader MetricsReader) MetricsReader {
	if interfaceIsNil(selector) || interfaceIsNil(reader) || !selector.UseV2(ctx) {
		return nil
	}
	return reader
}

type metricAggregate struct {
	RequestCount                       int64
	SuccessCount                       int64
	ClientErrorCount                   int64
	ServerErrorCount                   int64
	RateLimitedCount                   int64
	DurationSumMS                      int64
	DurationMinMS                      int64
	DurationMaxMS                      int64
	DurationHistogram                  [durationHistogramBucketCount]int64
	ExactP50MS, ExactP95MS, ExactP99MS int64
	HasExactPercentiles                bool
	UsedRollup                         bool
}

type statusMetricAggregate struct {
	Count      int64
	UsedRollup bool
}

func (a *statusMetricAggregate) Merge(count int64, rollup bool) error {
	if count < 0 {
		return errors.New("status metric count must be nonnegative")
	}
	merged, err := checkedMetricAdd(a.Count, count)
	if err != nil {
		return fmt.Errorf("merge status metric count: %w", err)
	}
	a.Count = merged
	a.UsedRollup = a.UsedRollup || (rollup && count > 0)
	return nil
}

func (a *metricAggregate) Merge(other metricAggregate) error {
	if err := other.validate(); err != nil {
		return err
	}
	wasEmpty := a.RequestCount == 0
	if other.RequestCount > 0 && (a.RequestCount == 0 || other.DurationMinMS < a.DurationMinMS) {
		a.DurationMinMS = other.DurationMinMS
	}
	if other.DurationMaxMS > a.DurationMaxMS {
		a.DurationMaxMS = other.DurationMaxMS
	}
	var err error
	if a.RequestCount, err = checkedMetricAdd(a.RequestCount, other.RequestCount); err != nil {
		return fmt.Errorf("merge request counts: %w", err)
	}
	for target, source := range map[*int64]int64{
		&a.SuccessCount: other.SuccessCount, &a.ClientErrorCount: other.ClientErrorCount,
		&a.ServerErrorCount: other.ServerErrorCount, &a.RateLimitedCount: other.RateLimitedCount,
		&a.DurationSumMS: other.DurationSumMS,
	} {
		if *target, err = checkedMetricAdd(*target, source); err != nil {
			return fmt.Errorf("merge metrics aggregate: %w", err)
		}
	}
	for i := range a.DurationHistogram {
		if a.DurationHistogram[i], err = checkedMetricAdd(a.DurationHistogram[i], other.DurationHistogram[i]); err != nil {
			return fmt.Errorf("merge duration histogram bucket %d: %w", i, err)
		}
	}
	if wasEmpty && other.HasExactPercentiles {
		a.ExactP50MS, a.ExactP95MS, a.ExactP99MS, a.HasExactPercentiles = other.ExactP50MS, other.ExactP95MS, other.ExactP99MS, true
	} else if other.RequestCount > 0 {
		a.HasExactPercentiles = false
	}
	a.UsedRollup = a.UsedRollup || (other.UsedRollup && other.RequestCount > 0)
	return a.validate()
}

func checkedMetricAdd(left, right int64) (int64, error) {
	if right > 0 && left > math.MaxInt64-right {
		return 0, errors.New("int64 overflow")
	}
	if right < 0 && left < math.MinInt64-right {
		return 0, errors.New("int64 underflow")
	}
	return left + right, nil
}

func (a metricAggregate) validate() error {
	counts := []int64{a.RequestCount, a.SuccessCount, a.ClientErrorCount, a.ServerErrorCount, a.RateLimitedCount, a.DurationSumMS, a.DurationMinMS, a.DurationMaxMS}
	for _, value := range counts {
		if value < 0 {
			return errors.New("metrics aggregate contains a negative count or duration")
		}
	}
	classified, err := checkedMetricAdd(a.SuccessCount, a.ClientErrorCount)
	if err == nil {
		classified, err = checkedMetricAdd(classified, a.ServerErrorCount)
	}
	if err != nil || classified != a.RequestCount || a.RateLimitedCount > a.ClientErrorCount {
		return errors.New("metrics aggregate status counts are inconsistent")
	}
	var histogramTotal int64
	for _, count := range a.DurationHistogram {
		if count < 0 {
			return errors.New("metrics aggregate histogram contains a negative count")
		}
		histogramTotal, err = checkedMetricAdd(histogramTotal, count)
		if err != nil {
			return errors.New("metrics aggregate histogram count overflows int64")
		}
	}
	if histogramTotal != a.RequestCount {
		return errors.New("metrics aggregate histogram total does not match request count")
	}
	if a.RequestCount == 0 {
		return nil
	}
	if a.DurationMinMS > a.DurationMaxMS {
		return errors.New("metrics aggregate duration minimum exceeds maximum")
	}
	if _, err := ApproximateDurationPercentile(a.DurationHistogram[:], a.DurationMaxMS, .50); err != nil {
		return fmt.Errorf("metrics aggregate histogram is invalid: %w", err)
	}
	return nil
}

func (a metricAggregate) Overall() (MetricsOverall, error) {
	if err := a.validate(); err != nil {
		return MetricsOverall{}, err
	}
	result := MetricsOverall{TotalCalls: a.RequestCount, SuccessCount: a.SuccessCount, ClientErrorCount: a.ClientErrorCount, ServerErrorCount: a.ServerErrorCount, RateLimitedCount: a.RateLimitedCount}
	if result.TotalCalls > 0 {
		result.ErrorRatePct = float64(result.ClientErrorCount+result.ServerErrorCount) * 100 / float64(result.TotalCalls)
		result.ServerFailureRatePct = float64(result.ServerErrorCount) * 100 / float64(result.TotalCalls)
		result.ReliabilityPct = float64(result.TotalCalls-result.ServerErrorCount) * 100 / float64(result.TotalCalls)
		result.AvgMS = roundedNonnegativeRatio(a.DurationSumMS, result.TotalCalls)
		if !a.UsedRollup && a.HasExactPercentiles {
			result.P50MS, result.P95MS, result.P99MS = a.ExactP50MS, a.ExactP95MS, a.ExactP99MS
		} else {
			hist := a.DurationHistogram[:]
			var err error
			if result.P50MS, err = ApproximateDurationPercentile(hist, a.DurationMaxMS, .50); err != nil {
				return MetricsOverall{}, err
			}
			if result.P95MS, err = ApproximateDurationPercentile(hist, a.DurationMaxMS, .95); err != nil {
				return MetricsOverall{}, err
			}
			if result.P99MS, err = ApproximateDurationPercentile(hist, a.DurationMaxMS, .99); err != nil {
				return MetricsOverall{}, err
			}
		}
	}
	return result, nil
}

func roundedNonnegativeRatio(numerator, denominator int64) int64 {
	if numerator <= 0 || denominator <= 0 {
		return 0
	}
	quotient, remainder := numerator/denominator, numerator%denominator
	threshold := denominator/2 + denominator%2
	if remainder >= threshold {
		return quotient + 1
	}
	return quotient
}

type metricsDB interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type metricsBeginner interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

type PostgresMetricsStore struct {
	db  metricsBeginner
	now func() time.Time
}

func NewPostgresMetricsStore(db metricsBeginner) *PostgresMetricsStore {
	return &PostgresMetricsStore{db: db, now: time.Now}
}

func withMetricsSnapshot[T any](ctx context.Context, store *PostgresMetricsStore, load func(metricsDB, time.Time) (T, MetricsFreshness, error)) (T, MetricsFreshness, error) {
	var zero T
	if store == nil || interfaceIsNil(store.db) {
		return zero, MetricsFreshness{}, errors.New("metrics store is not configured")
	}
	now := time.Now
	if store.now != nil {
		now = store.now
	}
	pinnedNow := now().UTC()
	tx, err := store.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return zero, MetricsFreshness{}, fmt.Errorf("begin metrics snapshot: %w", err)
	}
	defer func() {
		rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), rollupRollbackTimeout)
		defer rollbackCancel()
		_ = tx.Rollback(rollbackCtx)
	}()
	data, freshness, err := load(tx, pinnedNow)
	if err != nil {
		return zero, freshness, err
	}
	if err := tx.Commit(ctx); err != nil {
		return zero, freshness, fmt.Errorf("commit metrics snapshot: %w", err)
	}
	return data, freshness, nil
}

type metricGroupMode string

const (
	metricGroupOverall        metricGroupMode = "overall"
	metricGroupSummary        metricGroupMode = "summary"
	metricGroupTrendHour      metricGroupMode = "trend_hour"
	metricGroupTrendDay       metricGroupMode = "trend_day"
	metricGroupWorkspaceTotal metricGroupMode = "workspace_total"
	metricGroupWorkspaceRoute metricGroupMode = "workspace_route"
)

type metricGroupKey struct {
	WorkspaceID, WorkspaceName, Path, Method string
	StatusCode                               int
	Bucket                                   time.Time
}

func (s *PostgresMetricsStore) Overall(ctx context.Context, query MetricsQuery) (MetricsOverall, MetricsFreshness, error) {
	return withMetricsSnapshot(ctx, s, func(db metricsDB, now time.Time) (MetricsOverall, MetricsFreshness, error) {
		groups, freshness, err := s.loadAt(ctx, db, query, metricGroupOverall, now)
		if err != nil {
			return MetricsOverall{}, freshness, err
		}
		var total metricAggregate
		for _, value := range groups {
			if err := total.Merge(value); err != nil {
				return MetricsOverall{}, freshness, err
			}
		}
		result, err := total.Overall()
		return result, freshnessForReturnedProvenance(freshness, total.UsedRollup), err
	})
}

func (s *PostgresMetricsStore) Summary(ctx context.Context, query MetricsQuery) ([]MetricsSummaryRow, MetricsFreshness, error) {
	return withMetricsSnapshot(ctx, s, func(db metricsDB, now time.Time) ([]MetricsSummaryRow, MetricsFreshness, error) {
		groups, freshness, err := s.loadAt(ctx, db, query, metricGroupSummary, now)
		if err != nil {
			return nil, freshness, err
		}
		rows := make([]MetricsSummaryRow, 0, len(groups))
		for key, aggregate := range groups {
			metrics, err := aggregate.Overall()
			if err != nil {
				return nil, freshness, err
			}
			if query.ApplyMinCalls && metrics.TotalCalls < int64(max32(query.MinCalls, 1)) {
				continue
			}
			rows = append(rows, summaryRow(key, metrics))
		}
		sort.Slice(rows, func(i, j int) bool { return summaryLess(rows[i], rows[j], query.Sort) })
		rows = limitSlice(rows, query.Limit)
		usedRollup := false
		for _, row := range rows {
			usedRollup = usedRollup || groups[metricGroupKey{Path: row.Path, Method: row.Method}].UsedRollup
		}
		return legacyCompatibleMetricsRows(rows), freshnessForReturnedProvenance(freshness, usedRollup), nil
	})
}

func (s *PostgresMetricsStore) Trend(ctx context.Context, query MetricsQuery) ([]MetricsTrendRow, MetricsFreshness, error) {
	return withMetricsSnapshot(ctx, s, func(db metricsDB, now time.Time) ([]MetricsTrendRow, MetricsFreshness, error) {
		mode := metricGroupTrendHour
		if query.Interval == "day" {
			mode = metricGroupTrendDay
		}
		groups, freshness, err := s.loadAt(ctx, db, query, mode, now)
		if err != nil {
			return nil, freshness, err
		}
		rows := make([]MetricsTrendRow, 0, len(groups))
		for key, aggregate := range groups {
			m, err := aggregate.Overall()
			if err != nil {
				return nil, freshness, err
			}
			rows = append(rows, MetricsTrendRow{Bucket: key.Bucket, TotalCalls: m.TotalCalls, SuccessCount: m.SuccessCount, ErrorCount: m.ClientErrorCount + m.ServerErrorCount, ClientErrorCount: m.ClientErrorCount, ServerErrorCount: m.ServerErrorCount, RateLimitedCount: m.RateLimitedCount, P50MS: m.P50MS, P95MS: m.P95MS, P99MS: m.P99MS, AvgMS: m.AvgMS})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].Bucket.Before(rows[j].Bucket) })
		usedRollup := false
		for _, aggregate := range groups {
			usedRollup = usedRollup || aggregate.UsedRollup
		}
		return legacyCompatibleMetricsRows(rows), freshnessForReturnedProvenance(freshness, usedRollup), nil
	})
}

func (s *PostgresMetricsStore) StatusCodes(ctx context.Context, query MetricsQuery) ([]MetricsStatusCodeRow, MetricsFreshness, error) {
	return withMetricsSnapshot(ctx, s, func(db metricsDB, now time.Time) ([]MetricsStatusCodeRow, MetricsFreshness, error) {
		groups, freshness, err := s.loadStatusAt(ctx, db, query, now)
		if err != nil {
			return nil, freshness, err
		}
		rows := make([]MetricsStatusCodeRow, 0, len(groups))
		for key, a := range groups {
			rows = append(rows, MetricsStatusCodeRow{StatusCode: int32(key.StatusCode), Method: key.Method, Path: key.Path, TotalCalls: a.Count})
		}
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].TotalCalls != rows[j].TotalCalls {
				return rows[i].TotalCalls > rows[j].TotalCalls
			}
			return rows[i].StatusCode < rows[j].StatusCode
		})
		rows = limitSlice(rows, query.Limit)
		usedRollup := false
		for _, row := range rows {
			usedRollup = usedRollup || groups[metricGroupKey{Path: row.Path, Method: row.Method, StatusCode: int(row.StatusCode)}].UsedRollup
		}
		return legacyCompatibleMetricsRows(rows), freshnessForReturnedProvenance(freshness, usedRollup), nil
	})
}

func (s *PostgresMetricsStore) Workspaces(ctx context.Context, query MetricsQuery) ([]AdminMetricsWorkspaceRow, MetricsFreshness, error) {
	return withMetricsSnapshot(ctx, s, func(db metricsDB, now time.Time) ([]AdminMetricsWorkspaceRow, MetricsFreshness, error) {
		plan, err := prepareMetricsSourcePlan(ctx, db, query, now)
		if err != nil {
			return nil, MetricsFreshness{}, err
		}
		totalGroups, _, err := s.loadAtWithPlan(ctx, db, query, metricGroupWorkspaceTotal, plan)
		if err != nil {
			return nil, MetricsFreshness{}, err
		}
		routeGroups, _, err := s.loadAtWithPlan(ctx, db, query, metricGroupWorkspaceRoute, plan)
		if err != nil {
			return nil, MetricsFreshness{}, err
		}
		totals := map[string]metricAggregate{}
		names := map[string]string{}
		endpoints := map[string]struct {
			path       string
			p95        int64
			usedRollup bool
		}{}
		for key, a := range totalGroups {
			totals[key.WorkspaceID] = a
			names[key.WorkspaceID] = key.WorkspaceName
		}
		for key, a := range routeGroups {
			metric, err := a.Overall()
			if err != nil {
				return nil, MetricsFreshness{}, err
			}
			p95 := metric.P95MS
			current := endpoints[key.WorkspaceID]
			if shouldReplaceSlowestEndpoint(current.path, current.p95, key.Path, p95) {
				endpoints[key.WorkspaceID] = struct {
					path       string
					p95        int64
					usedRollup bool
				}{path: key.Path, p95: p95, usedRollup: a.UsedRollup}
			}
		}
		rows := make([]AdminMetricsWorkspaceRow, 0, len(totals))
		for id, a := range totals {
			m, err := a.Overall()
			if err != nil {
				return nil, MetricsFreshness{}, err
			}
			if m.TotalCalls < int64(max32(query.MinCalls, 1)) {
				continue
			}
			ep := endpoints[id]
			rows = append(rows, AdminMetricsWorkspaceRow{WorkspaceID: id, WorkspaceName: names[id], TotalCalls: m.TotalCalls, RateLimitedCount: m.RateLimitedCount, ErrorRatePct: m.ErrorRatePct, ServerFailureRatePct: m.ServerFailureRatePct, P95MS: m.P95MS, P99MS: m.P99MS, SlowestEndpoint: ep.path, SlowestEndpointP95MS: ep.p95})
		}
		sort.Slice(rows, func(i, j int) bool { return workspaceLess(rows[i], rows[j], query.Sort) })
		rows = limitSlice(rows, query.Limit)
		usedRollup := false
		for _, row := range rows {
			usedRollup = usedRollup || totals[row.WorkspaceID].UsedRollup || endpoints[row.WorkspaceID].usedRollup
		}
		return legacyCompatibleMetricsRows(rows), freshnessFromSourcePlan(plan, usedRollup), nil
	})
}

func freshnessForReturnedProvenance(freshness MetricsFreshness, usedRollup bool) MetricsFreshness {
	freshness.Approximate = usedRollup
	if freshness.DataState != MetricsDataDelayed {
		if usedRollup {
			freshness.DataState = MetricsDataApproximate
		} else {
			freshness.DataState = MetricsDataExact
		}
	}
	return freshness
}

func shouldReplaceSlowestEndpoint(currentPath string, currentP95 int64, candidatePath string, candidateP95 int64) bool {
	return candidateP95 > currentP95 || (candidateP95 == currentP95 && (currentPath == "" || candidatePath < currentPath))
}

func summaryRow(key metricGroupKey, metrics MetricsOverall) MetricsSummaryRow {
	return MetricsSummaryRow{Path: key.Path, Method: key.Method, TotalCalls: metrics.TotalCalls, SuccessCount: metrics.SuccessCount, ClientErrorCount: metrics.ClientErrorCount, ServerErrorCount: metrics.ServerErrorCount, RateLimitedCount: metrics.RateLimitedCount, ErrorRatePct: metrics.ErrorRatePct, ServerFailureRatePct: metrics.ServerFailureRatePct, P50MS: metrics.P50MS, P95MS: metrics.P95MS, P99MS: metrics.P99MS, AvgMS: metrics.AvgMS}
}

func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
func limitSlice[T any](rows []T, limit int32) []T {
	if limit > 0 && len(rows) > int(limit) {
		return rows[:limit]
	}
	return rows
}

func legacyCompatibleMetricsRows[T any](rows []T) []T {
	if len(rows) == 0 {
		return nil
	}
	return rows
}

func summaryLess(a, b MetricsSummaryRow, sortKey string) bool {
	switch sortKey {
	case "p95_ms_desc":
		if a.P95MS != b.P95MS {
			return a.P95MS > b.P95MS
		}
	case "p99_ms_desc":
		if a.P99MS != b.P99MS {
			return a.P99MS > b.P99MS
		}
	case "server_errors_desc":
		if a.ServerErrorCount != b.ServerErrorCount {
			return a.ServerErrorCount > b.ServerErrorCount
		}
	case "rate_limited_desc":
		if a.RateLimitedCount != b.RateLimitedCount {
			return a.RateLimitedCount > b.RateLimitedCount
		}
	}
	if a.TotalCalls != b.TotalCalls {
		return a.TotalCalls > b.TotalCalls
	}
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	return a.Method < b.Method
}
func workspaceLess(a, b AdminMetricsWorkspaceRow, sortKey string) bool {
	switch sortKey {
	case "p95_ms_desc":
		if a.P95MS != b.P95MS {
			return a.P95MS > b.P95MS
		}
	case "p99_ms_desc":
		if a.P99MS != b.P99MS {
			return a.P99MS > b.P99MS
		}
	case "rate_limited_desc":
		if a.RateLimitedCount != b.RateLimitedCount {
			return a.RateLimitedCount > b.RateLimitedCount
		}
	}
	if a.TotalCalls != b.TotalCalls {
		return a.TotalCalls > b.TotalCalls
	}
	return a.WorkspaceID < b.WorkspaceID
}

func (s *PostgresMetricsStore) loadAt(ctx context.Context, db metricsDB, q MetricsQuery, mode metricGroupMode, now time.Time) (map[metricGroupKey]metricAggregate, MetricsFreshness, error) {
	plan, err := prepareMetricsSourcePlan(ctx, db, q, now)
	if err != nil {
		return nil, MetricsFreshness{}, err
	}
	groups, usedRollup, err := s.loadAtWithPlan(ctx, db, q, mode, plan)
	if err != nil {
		return nil, MetricsFreshness{}, err
	}
	return groups, freshnessFromSourcePlan(plan, usedRollup), nil
}

func (s *PostgresMetricsStore) loadAtWithPlan(ctx context.Context, db metricsDB, q MetricsQuery, mode metricGroupMode, plan MetricsSourcePlan) (map[metricGroupKey]metricAggregate, bool, error) {
	groups := map[metricGroupKey]metricAggregate{}
	if len(plan.RawRanges) > 0 {
		if err := s.loadRaw(ctx, db, groups, q, mode, plan.RawRanges); err != nil {
			return nil, false, err
		}
	}
	if len(plan.RollupRanges) > 0 {
		if err := s.loadRollup(ctx, db, groups, q, mode, plan.RollupRanges); err != nil {
			return nil, false, err
		}
	}
	usedRollup := false
	for _, aggregate := range groups {
		usedRollup = usedRollup || aggregate.UsedRollup
	}
	return groups, usedRollup, nil
}

func prepareMetricsSourcePlan(ctx context.Context, db metricsDB, q MetricsQuery, now time.Time) (MetricsSourcePlan, error) {
	plan := BuildMetricsSourcePlan(q.From, q.To, now)
	completed := 0
	for _, rr := range plan.RollupRanges {
		var n int
		if err := db.QueryRow(ctx, `SELECT COUNT(*)::INTEGER FROM api_request_metric_rollup_hours WHERE hour_bucket >= $1 AND hour_bucket < $2`, rr.From, rr.To).Scan(&n); err != nil {
			return MetricsSourcePlan{}, fmt.Errorf("load metrics rollup freshness: %w", err)
		}
		completed += n
	}
	plan.MarkRollupCompletion(completed)
	partialRawAvailable := true
	for _, rr := range plan.HistoricalPartialRawRanges {
		var exists bool
		if err := db.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS (
SELECT 1 FROM api_request_events e
WHERE e.occurred_at >= $1 AND e.occurred_at %s $2
  AND ($3::TEXT='' OR e.workspace_id=$3)
  AND ($4::TEXT='' OR e.method=$4)
  AND ($5::TEXT='' OR e.route_pattern=$5)
  AND ($6::TEXT='' OR LEFT(e.status_code::TEXT,1)=LEFT($6,1))
)`, rawEndOperator(rr, q.To)), rr.From, rr.To, q.WorkspaceID, q.Method, q.Path, q.StatusClass).Scan(&exists); err != nil {
			return MetricsSourcePlan{}, fmt.Errorf("load historical partial metrics freshness: %w", err)
		}
		partialRawAvailable = partialRawAvailable && exists
	}
	plan.MarkHistoricalPartialRawEvidence(partialRawAvailable)
	return plan, nil
}

func freshnessFromSourcePlan(plan MetricsSourcePlan, usedRollup bool) MetricsFreshness {
	state := plan.State
	if state == MetricsDataApproximate && !usedRollup {
		state = MetricsDataExact
	}
	return MetricsFreshness{DataState: state, Approximate: usedRollup, MissingRollupHours: plan.MissingRollupHours}
}

func (s *PostgresMetricsStore) loadStatusAt(ctx context.Context, db metricsDB, q MetricsQuery, now time.Time) (map[metricGroupKey]statusMetricAggregate, MetricsFreshness, error) {
	plan, err := prepareMetricsSourcePlan(ctx, db, q, now)
	if err != nil {
		return nil, MetricsFreshness{}, err
	}
	groups := map[metricGroupKey]statusMetricAggregate{}
	if len(plan.RawRanges) > 0 {
		if err := s.loadRawStatusCounts(ctx, db, groups, q, plan.RawRanges); err != nil {
			return nil, MetricsFreshness{}, err
		}
	}
	if len(plan.RollupRanges) > 0 {
		if err := s.loadRollupStatusCounts(ctx, db, groups, q, plan.RollupRanges); err != nil {
			return nil, MetricsFreshness{}, err
		}
	}
	usedRollup := false
	for _, aggregate := range groups {
		usedRollup = usedRollup || aggregate.UsedRollup
	}
	return groups, freshnessFromSourcePlan(plan, usedRollup), nil
}

func (s *PostgresMetricsStore) loadRawStatusCounts(ctx context.Context, db metricsDB, dst map[metricGroupKey]statusMetricAggregate, q MetricsQuery, ranges []MetricsRange) error {
	rangeSQL, args := metricRangePredicate("e.occurred_at", ranges, q.To)
	filterStart := len(args) + 1
	args = append(args, q.WorkspaceID, q.Method, q.Path, q.StatusClass)
	rows, err := db.Query(ctx, fmt.Sprintf(`SELECT e.route_pattern, e.method, e.status_code, COUNT(*)::BIGINT
FROM api_request_events e
WHERE %s AND %s
GROUP BY e.route_pattern, e.method, e.status_code`, rangeSQL, metricFilterSQL(filterStart, "e.workspace_id", "e.method", "e.route_pattern", "e.status_code")), args...)
	if err != nil {
		return fmt.Errorf("load raw status metrics: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key metricGroupKey
		var count int64
		if err := rows.Scan(&key.Path, &key.Method, &key.StatusCode, &count); err != nil {
			return err
		}
		aggregate := dst[key]
		if err := aggregate.Merge(count, false); err != nil {
			return err
		}
		dst[key] = aggregate
	}
	return rows.Err()
}

func (s *PostgresMetricsStore) loadRollupStatusCounts(ctx context.Context, db metricsDB, dst map[metricGroupKey]statusMetricAggregate, q MetricsQuery, ranges []MetricsRange) error {
	rangeSQL, args := metricRangePredicate("r.hour_bucket", ranges, time.Time{})
	filterStart := len(args) + 1
	args = append(args, q.WorkspaceID, q.Method, q.Path, q.StatusClass)
	rows, err := db.Query(ctx, fmt.Sprintf(`SELECT r.route_pattern, r.method, r.status_code, COALESCE(SUM(r.request_count),0)::BIGINT
FROM api_request_metric_rollups_hourly r
WHERE %s AND %s
GROUP BY r.route_pattern, r.method, r.status_code`, rangeSQL, metricFilterSQL(filterStart, "r.workspace_id", "r.method", "r.route_pattern", "r.status_code")), args...)
	if err != nil {
		return fmt.Errorf("load rollup status metrics: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key metricGroupKey
		var count int64
		if err := rows.Scan(&key.Path, &key.Method, &key.StatusCode, &count); err != nil {
			return err
		}
		aggregate := dst[key]
		if err := aggregate.Merge(count, true); err != nil {
			return err
		}
		dst[key] = aggregate
	}
	return rows.Err()
}

func utcTruncSQL(unit, column string) string {
	return fmt.Sprintf("(date_trunc('%s', %s AT TIME ZONE 'UTC') AT TIME ZONE 'UTC')", unit, column)
}

func rawGroupExpressions(mode metricGroupMode) (string, string, error) {
	switch mode {
	case metricGroupOverall:
		return "''::TEXT, ''::TEXT, ''::TEXT, ''::TEXT, 0::INTEGER, NULL::TIMESTAMPTZ", "", nil
	case metricGroupSummary:
		return "''::TEXT, ''::TEXT, route_pattern, method, 0::INTEGER, NULL::TIMESTAMPTZ", "route_pattern, method", nil
	case metricGroupTrendHour:
		expr := utcTruncSQL("hour", "e.occurred_at")
		return "''::TEXT, ''::TEXT, ''::TEXT, ''::TEXT, 0::INTEGER, " + expr, expr, nil
	case metricGroupTrendDay:
		expr := utcTruncSQL("day", "e.occurred_at")
		return "''::TEXT, ''::TEXT, ''::TEXT, ''::TEXT, 0::INTEGER, " + expr, expr, nil
	case metricGroupWorkspaceTotal:
		return "e.workspace_id, COALESCE(w.name, ''), '', '', 0::INTEGER, NULL::TIMESTAMPTZ", "e.workspace_id, w.name", nil
	case metricGroupWorkspaceRoute:
		return "e.workspace_id, COALESCE(w.name, ''), e.route_pattern, '', 0::INTEGER, NULL::TIMESTAMPTZ", "e.workspace_id, w.name, e.route_pattern", nil
	default:
		return "", "", fmt.Errorf("unsupported metrics group mode %q", mode)
	}
}

func durationHistogramAliasSQL(column string) string {
	parts := make([]string, 0, durationHistogramBucketCount)
	for i, b := range durationHistogramUpperBoundsMS {
		if math.IsInf(b, 1) {
			parts = append(parts, fmt.Sprintf("COUNT(*) FILTER (WHERE %s > %v)", column, durationHistogramUpperBoundsMS[i-1]))
			continue
		}
		if i == 0 {
			parts = append(parts, fmt.Sprintf("COUNT(*) FILTER (WHERE %s <= %v)", column, b))
			continue
		}
		parts = append(parts, fmt.Sprintf("COUNT(*) FILTER (WHERE %s > %v AND %s <= %v)", column, durationHistogramUpperBoundsMS[i-1], column, b))
	}
	return strings.Join(parts, ",")
}
func metricRangePredicate(column string, ranges []MetricsRange, queryTo time.Time) (string, []any) {
	parts := make([]string, 0, len(ranges))
	args := make([]any, 0, len(ranges)*2)
	for _, rr := range ranges {
		fromIndex := len(args) + 1
		args = append(args, rr.From, rr.To)
		parts = append(parts, fmt.Sprintf("(%s >= $%d AND %s %s $%d)", column, fromIndex, column, rawEndOperator(rr, queryTo), fromIndex+1))
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}

func metricFilterSQL(firstParameter int, workspaceColumn, methodColumn, pathColumn, statusColumn string) string {
	return fmt.Sprintf("($%d::TEXT='' OR %s=$%d) AND ($%d::TEXT='' OR %s=$%d) AND ($%d::TEXT='' OR %s=$%d) AND ($%d::TEXT='' OR LEFT(%s::TEXT,1)=LEFT($%d,1))",
		firstParameter, workspaceColumn, firstParameter,
		firstParameter+1, methodColumn, firstParameter+1,
		firstParameter+2, pathColumn, firstParameter+2,
		firstParameter+3, statusColumn, firstParameter+3)
}

func (s *PostgresMetricsStore) loadRaw(ctx context.Context, db metricsDB, dst map[metricGroupKey]metricAggregate, q MetricsQuery, mode metricGroupMode, ranges []MetricsRange) error {
	selectExpr, groupBy, err := rawGroupExpressions(mode)
	if err != nil {
		return err
	}
	fromClause := "api_request_events e"
	if mode == metricGroupWorkspaceRoute || mode == metricGroupWorkspaceTotal {
		fromClause += " LEFT JOIN workspaces w ON w.id=e.workspace_id"
	}
	groupSQL := ""
	if groupBy != "" {
		groupSQL = " GROUP BY " + groupBy
	}
	rangeSQL, args := metricRangePredicate("e.occurred_at", ranges, q.To)
	filterStart := len(args) + 1
	args = append(args, q.WorkspaceID, q.Method, q.Path, q.StatusClass)
	sql := fmt.Sprintf(`SELECT %s,
COUNT(*)::BIGINT,
COUNT(*) FILTER (WHERE e.status_code < 400)::BIGINT,
COUNT(*) FILTER (WHERE e.status_code >= 400 AND e.status_code < 500)::BIGINT,
COUNT(*) FILTER (WHERE e.status_code >= 500)::BIGINT,
COUNT(*) FILTER (WHERE e.status_code = 429)::BIGINT,
COALESCE(SUM(e.duration_ms),0)::BIGINT,
COALESCE(MIN(e.duration_ms),0)::BIGINT,
COALESCE(MAX(e.duration_ms),0)::BIGINT,
ARRAY[%s]::BIGINT[],
COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY e.duration_ms),0)::BIGINT,
COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY e.duration_ms),0)::BIGINT,
COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY e.duration_ms),0)::BIGINT
FROM %s WHERE %s AND %s%s`, selectExpr, durationHistogramAliasSQL("e.duration_ms"), fromClause, rangeSQL, metricFilterSQL(filterStart, "e.workspace_id", "e.method", "e.route_pattern", "e.status_code"), groupSQL)
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("load raw metrics: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		key, a, err := scanMetricAggregate(rows, true, false)
		if err != nil {
			return err
		}
		existing := dst[key]
		if err := existing.Merge(a); err != nil {
			return fmt.Errorf("merge raw metrics: %w", err)
		}
		dst[key] = existing
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}

func rawEndOperator(rr MetricsRange, queryTo time.Time) string {
	if rr.To.Equal(queryTo.UTC()) {
		return "<="
	}
	return "<"
}

func rollupGroupExpressions(mode metricGroupMode) (string, string, error) {
	switch mode {
	case metricGroupOverall:
		return "''::TEXT, ''::TEXT, ''::TEXT, ''::TEXT, 0::INTEGER, NULL::TIMESTAMPTZ", "", nil
	case metricGroupSummary:
		return "''::TEXT, ''::TEXT, r.route_pattern, r.method, 0::INTEGER, NULL::TIMESTAMPTZ", "r.route_pattern, r.method", nil
	case metricGroupTrendHour:
		expr := utcTruncSQL("hour", "r.hour_bucket")
		return "''::TEXT, ''::TEXT, ''::TEXT, ''::TEXT, 0::INTEGER, " + expr, expr, nil
	case metricGroupTrendDay:
		expr := utcTruncSQL("day", "r.hour_bucket")
		return "''::TEXT, ''::TEXT, ''::TEXT, ''::TEXT, 0::INTEGER, " + expr, expr, nil
	case metricGroupWorkspaceTotal:
		return "r.workspace_id, COALESCE(w.name, ''), '', '', 0::INTEGER, NULL::TIMESTAMPTZ", "r.workspace_id, w.name", nil
	case metricGroupWorkspaceRoute:
		return "r.workspace_id, COALESCE(w.name, ''), r.route_pattern, '', 0::INTEGER, NULL::TIMESTAMPTZ", "r.workspace_id, w.name, r.route_pattern", nil
	default:
		return "", "", fmt.Errorf("unsupported metrics group mode %q", mode)
	}
}

func rollupHistogramSumSQL() string {
	parts := make([]string, durationHistogramBucketCount)
	for index := range parts {
		parts[index] = fmt.Sprintf("COALESCE(SUM(r.duration_histogram[%d]),0)::BIGINT", index+1)
	}
	return strings.Join(parts, ",")
}

func (s *PostgresMetricsStore) loadRollup(ctx context.Context, db metricsDB, dst map[metricGroupKey]metricAggregate, q MetricsQuery, mode metricGroupMode, ranges []MetricsRange) error {
	selectExpr, groupBy, err := rollupGroupExpressions(mode)
	if err != nil {
		return err
	}
	groupSQL := ""
	if groupBy != "" {
		groupSQL = " GROUP BY " + groupBy
	}
	fromClause := "api_request_metric_rollups_hourly r"
	if mode == metricGroupWorkspaceTotal || mode == metricGroupWorkspaceRoute {
		fromClause += " LEFT JOIN workspaces w ON w.id=r.workspace_id"
	}
	rangeSQL, args := metricRangePredicate("r.hour_bucket", ranges, time.Time{})
	filterStart := len(args) + 1
	args = append(args, q.WorkspaceID, q.Method, q.Path, q.StatusClass)
	rows, err := db.Query(ctx, fmt.Sprintf(`SELECT %s,
COALESCE(SUM(r.request_count),0)::BIGINT,
COALESCE(SUM(r.request_count) FILTER (WHERE r.status_code < 400),0)::BIGINT,
COALESCE(SUM(r.request_count) FILTER (WHERE r.status_code >= 400 AND r.status_code < 500),0)::BIGINT,
COALESCE(SUM(r.request_count) FILTER (WHERE r.status_code >= 500),0)::BIGINT,
COALESCE(SUM(r.request_count) FILTER (WHERE r.status_code = 429),0)::BIGINT,
COALESCE(SUM(r.duration_sum_ms),0)::BIGINT,
COALESCE(MIN(r.duration_min_ms),0)::BIGINT,
COALESCE(MAX(r.duration_max_ms),0)::BIGINT,
ARRAY[%s]::BIGINT[], 0::BIGINT, 0::BIGINT, 0::BIGINT
FROM %s
WHERE %s AND %s%s`, selectExpr, rollupHistogramSumSQL(), fromClause, rangeSQL, metricFilterSQL(filterStart, "r.workspace_id", "r.method", "r.route_pattern", "r.status_code"), groupSQL), args...)
	if err != nil {
		return fmt.Errorf("load rollup metrics: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		key, a, err := scanMetricAggregate(rows, false, true)
		if err != nil {
			return err
		}
		existing := dst[key]
		if err := existing.Merge(a); err != nil {
			return fmt.Errorf("merge rollup metrics: %w", err)
		}
		dst[key] = existing
	}
	return rows.Err()
}

func scanMetricAggregate(rows pgx.Rows, exact, rollup bool) (metricGroupKey, metricAggregate, error) {
	var key metricGroupKey
	var bucket *time.Time
	var hist []int64
	var a metricAggregate
	if err := rows.Scan(&key.WorkspaceID, &key.WorkspaceName, &key.Path, &key.Method, &key.StatusCode, &bucket, &a.RequestCount, &a.SuccessCount, &a.ClientErrorCount, &a.ServerErrorCount, &a.RateLimitedCount, &a.DurationSumMS, &a.DurationMinMS, &a.DurationMaxMS, &hist, &a.ExactP50MS, &a.ExactP95MS, &a.ExactP99MS); err != nil {
		return key, a, err
	}
	if bucket != nil {
		key.Bucket = *bucket
	}
	if len(hist) != durationHistogramBucketCount {
		return key, a, errors.New("invalid raw metrics histogram")
	}
	copy(a.DurationHistogram[:], hist)
	a.HasExactPercentiles = exact && a.RequestCount > 0
	a.UsedRollup = rollup && a.RequestCount > 0
	if err := a.validate(); err != nil {
		return key, a, err
	}
	return key, a, nil
}
