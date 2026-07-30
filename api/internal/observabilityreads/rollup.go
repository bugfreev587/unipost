package observabilityreads

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	durationHistogramBucketCount = 12
	rollupRecomputationHours     = 48
	rollupWorkerCadence          = time.Hour
	defaultRollupHourTimeout     = 30 * time.Second
	rollupRollbackTimeout        = 5 * time.Second
	agedOutRollupRetryBudget     = 4

	// 0x4f425356 is ASCII "OBSV". The second advisory-lock key is the UTC
	// Unix hour, keeping this lock namespace exclusive to observability work.
	observabilityRollupLockNamespace int32 = 0x4f425356
)

var durationHistogramUpperBoundsMS = [durationHistogramBucketCount]float64{
	5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, math.Inf(1),
}

// DurationHistogramUpperBoundsMS returns a copy of the fixed millisecond
// boundaries shared by hourly rollup writers and readers. The last bucket is
// unbounded.
func DurationHistogramUpperBoundsMS() [durationHistogramBucketCount]float64 {
	return durationHistogramUpperBoundsMS
}

// DurationHistogramBucket returns the single non-overlapping bucket that
// contains a nonnegative duration.
func DurationHistogramBucket(durationMS int64) (int, error) {
	if durationMS < 0 {
		return 0, errors.New("duration must be nonnegative")
	}
	for index, upperBound := range durationHistogramUpperBoundsMS {
		if float64(durationMS) <= upperBound {
			return index, nil
		}
	}
	return durationHistogramBucketCount - 1, nil
}

// ApproximateDurationPercentile returns the deterministic upper bound of the
// bucket containing the requested nearest-rank percentile. The unbounded final
// bucket uses the observed maximum instead of returning infinity.
func ApproximateDurationPercentile(histogram []int64, durationMaxMS int64, percentile float64) (int64, error) {
	if len(histogram) != durationHistogramBucketCount {
		return 0, fmt.Errorf("duration histogram must contain %d buckets", durationHistogramBucketCount)
	}
	if durationMaxMS < 0 {
		return 0, errors.New("duration maximum must be nonnegative")
	}
	if percentile <= 0 || percentile > 1 || math.IsNaN(percentile) {
		return 0, errors.New("percentile must be greater than zero and at most one")
	}
	var total int64
	for _, count := range histogram {
		if count < 0 {
			return 0, errors.New("duration histogram counts must be nonnegative")
		}
		if count > math.MaxInt64-total {
			return 0, errors.New("duration histogram count total overflows int64")
		}
		total += count
	}
	if total == 0 {
		return 0, errors.New("duration histogram must not be empty")
	}
	highestBucket := 0
	for index, count := range histogram {
		if count > 0 {
			highestBucket = index
		}
	}
	if highestBucket > 0 && durationMaxMS <= int64(durationHistogramUpperBoundsMS[highestBucket-1]) {
		return 0, errors.New("duration maximum is below the highest populated histogram bucket")
	}
	if highestBucket < durationHistogramBucketCount-1 && durationMaxMS > int64(durationHistogramUpperBoundsMS[highestBucket]) {
		return 0, errors.New("duration maximum exceeds the highest populated histogram bucket")
	}
	rank, err := exactNearestRank(total, percentile)
	if err != nil {
		return 0, err
	}
	var cumulative int64
	for index, count := range histogram {
		cumulative += count
		if cumulative < rank {
			continue
		}
		if index == durationHistogramBucketCount-1 {
			return durationMaxMS, nil
		}
		return int64(durationHistogramUpperBoundsMS[index]), nil
	}
	return 0, errors.New("duration histogram rank is unavailable")
}

func exactNearestRank(total int64, percentile float64) (int64, error) {
	decimal := strconv.FormatFloat(percentile, 'f', -1, 64)
	ratio, ok := new(big.Rat).SetString(decimal)
	if !ok {
		return 0, errors.New("percentile decimal representation is invalid")
	}
	numerator := new(big.Int).Mul(big.NewInt(total), ratio.Num())
	quotient := new(big.Int)
	remainder := new(big.Int)
	quotient.QuoRem(numerator, ratio.Denom(), remainder)
	if remainder.Sign() > 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, errors.New("percentile nearest rank overflows int64")
	}
	return quotient.Int64(), nil
}

// RecentCompletedHours returns oldest-to-newest UTC hour starts and never
// includes the current open hour.
func RecentCompletedHours(now time.Time, count int) []time.Time {
	if count <= 0 {
		return nil
	}
	openHour := now.UTC().Truncate(time.Hour)
	first := openHour.Add(-time.Duration(count) * time.Hour)
	hours := make([]time.Time, count)
	for index := range hours {
		hours[index] = first.Add(time.Duration(index) * time.Hour)
	}
	return hours
}

type rollupBeginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

type PostgresRollupStore struct {
	db  rollupBeginner
	now func() time.Time
}

func NewPostgresRollupStore(db rollupBeginner) *PostgresRollupStore {
	return &PostgresRollupStore{db: db, now: time.Now}
}

var recomputeRollupInsertSQL = fmt.Sprintf(`
INSERT INTO api_request_metric_rollups_hourly (
  hour_bucket, workspace_id, route_pattern, method, status_code,
  request_count, error_count, server_error_count,
  validation_error_count, rate_limit_count,
  duration_sum_ms, duration_min_ms, duration_max_ms,
  duration_histogram, updated_at
)
SELECT
  $1::TIMESTAMPTZ,
  workspace_id,
  route_pattern,
  method,
  status_code,
  COUNT(*)::BIGINT,
  COUNT(*) FILTER (WHERE status_code >= 400)::BIGINT,
  COUNT(*) FILTER (WHERE status_code >= 500)::BIGINT,
  COUNT(*) FILTER (WHERE outcome = 'validation_error')::BIGINT,
  COUNT(*) FILTER (WHERE outcome = 'rate_limited')::BIGINT,
  SUM(duration_ms)::BIGINT,
  MIN(duration_ms)::INTEGER,
  MAX(duration_ms)::INTEGER,
  ARRAY[%s]::BIGINT[],
  NOW()
FROM api_request_events
WHERE occurred_at >= $1 AND occurred_at < $2
GROUP BY workspace_id, route_pattern, method, status_code`, durationHistogramSQLBuckets())

func durationHistogramSQLBuckets() string {
	parts := make([]string, 0, durationHistogramBucketCount)
	for index, upperBound := range durationHistogramUpperBoundsMS {
		if math.IsInf(upperBound, 1) {
			lowerBound := strconv.FormatFloat(durationHistogramUpperBoundsMS[index-1], 'f', -1, 64)
			parts = append(parts, "COUNT(*) FILTER (WHERE duration_ms > "+lowerBound+")")
			continue
		}
		upper := strconv.FormatFloat(upperBound, 'f', -1, 64)
		if index == 0 {
			parts = append(parts, "COUNT(*) FILTER (WHERE duration_ms <= "+upper+")")
			continue
		}
		lower := strconv.FormatFloat(durationHistogramUpperBoundsMS[index-1], 'f', -1, 64)
		parts = append(parts, "COUNT(*) FILTER (WHERE duration_ms > "+lower+" AND duration_ms <= "+upper+")")
	}
	return strings.Join(parts, ",\n    ")
}

// RecomputeHour transactionally replaces all rollup dimension groups for one
// completed UTC hour. Deleting before inserting clears groups whose raw input
// was removed, while the transaction hides partial replacement from readers.
func (s *PostgresRollupStore) RecomputeHour(ctx context.Context, hour time.Time) error {
	if s == nil || s.db == nil {
		return errors.New("rollup store is not configured")
	}
	hourUTC := hour.UTC().Truncate(time.Hour)
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	if !hourUTC.Before(now().UTC().Truncate(time.Hour)) {
		return errors.New("rollup hour must be completed")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin rollup recomputation: %w", err)
	}
	defer func() {
		rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), rollupRollbackTimeout)
		defer rollbackCancel()
		_ = tx.Rollback(rollbackCtx)
	}()
	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock($1::INTEGER, $2::INTEGER)`,
		observabilityRollupLockNamespace,
		int32(hourUTC.Unix()/int64(time.Hour/time.Second)),
	); err != nil {
		return fmt.Errorf("lock rollup recomputation hour: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM api_request_metric_rollups_hourly WHERE hour_bucket = $1`, hourUTC); err != nil {
		return fmt.Errorf("delete existing rollup hour: %w", err)
	}
	if _, err := tx.Exec(ctx, recomputeRollupInsertSQL, hourUTC, hourUTC.Add(time.Hour)); err != nil {
		return fmt.Errorf("insert recomputed rollup hour: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO api_request_metric_rollup_hours (hour_bucket, completed_at)
VALUES ($1, NOW())
ON CONFLICT (hour_bucket) DO UPDATE SET completed_at = EXCLUDED.completed_at`, hourUTC); err != nil {
		return fmt.Errorf("mark recomputed rollup hour complete: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit rollup recomputation: %w", err)
	}
	return nil
}

type RollupStore interface {
	RecomputeHour(context.Context, time.Time) error
}

type rollupTicker interface {
	C() <-chan time.Time
	Stop()
}

type rollupClock interface {
	Now() time.Time
	NewTicker(time.Duration) rollupTicker
}

type systemRollupClock struct{}

func (systemRollupClock) Now() time.Time { return time.Now() }
func (systemRollupClock) NewTicker(cadence time.Duration) rollupTicker {
	return systemRollupTicker{Ticker: time.NewTicker(cadence)}
}

type systemRollupTicker struct{ *time.Ticker }

func (t systemRollupTicker) C() <-chan time.Time { return t.Ticker.C }

type RollupWorkerStats struct {
	LastAttemptAt time.Time
	LastSuccessAt time.Time
	// LastCompletedHour is populated only when no known failed hour remains;
	// it therefore represents contiguous observed coverage, not merely the
	// latest isolated success.
	LastCompletedHour    time.Time
	LatestSuccessfulHour time.Time
	OldestUnresolvedHour time.Time
	FailureCount         uint64
	Freshness            time.Duration
}

type RollupWorker struct {
	store       RollupStore
	clock       rollupClock
	logger      *slog.Logger
	hourTimeout time.Duration

	mu    sync.RWMutex
	stats RollupWorkerStats
	// unresolvedHours is intentionally process-local. A restart rebuilds
	// coverage from the normal 48-hour startup window; within one process,
	// known completed-hour gaps remain scheduled until they succeed. Values are
	// monotonic attempt sequences used for fair aged-out retry rotation.
	unresolvedHours map[time.Time]uint64
	attemptSequence uint64
}

func NewRollupWorker(store RollupStore) *RollupWorker {
	return newRollupWorker(store, systemRollupClock{})
}

func newRollupWorker(store RollupStore, clock rollupClock) *RollupWorker {
	return &RollupWorker{
		store:           store,
		clock:           clock,
		logger:          slog.Default(),
		hourTimeout:     defaultRollupHourTimeout,
		unresolvedHours: make(map[time.Time]uint64),
	}
}

func (w *RollupWorker) setHourTimeout(timeout time.Duration) {
	if w == nil || timeout <= 0 {
		return
	}
	w.mu.Lock()
	w.hourTimeout = timeout
	w.mu.Unlock()
}

// Start performs the startup catch-up in the caller's goroutine, then repeats
// it hourly. API wiring launches Start asynchronously, so failures and catch-up
// work never delay server construction or readiness.
func (w *RollupWorker) Start(ctx context.Context) {
	if w == nil || w.clock == nil {
		return
	}
	w.recomputeRecent(ctx)
	if ctx.Err() != nil {
		return
	}
	ticker := w.clock.NewTicker(rollupWorkerCadence)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			w.recomputeRecent(ctx)
		}
	}
}

// recomputeRecent continues after an individual hour fails so a single corrupt
// or transiently unavailable slice cannot leave every later hour stale.
func (w *RollupWorker) recomputeRecent(ctx context.Context) {
	if w == nil || w.clock == nil {
		return
	}
	batchNow := w.clock.Now()
	hours := w.recomputationHours(batchNow)
	latestCompletedHour := batchNow.UTC().Truncate(time.Hour).Add(-time.Hour)
	for _, hour := range hours {
		if ctx.Err() != nil {
			return
		}
		attemptAt := w.clock.Now()
		w.mu.Lock()
		w.stats.LastAttemptAt = attemptAt
		hourTimeout := w.hourTimeout
		w.mu.Unlock()

		var err error
		hourCtx, hourCancel := context.WithTimeout(ctx, hourTimeout)
		if w.store == nil {
			err = errors.New("rollup store is not configured")
		} else {
			err = w.store.RecomputeHour(hourCtx, hour)
		}
		hourCancel()
		if err != nil {
			if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
				return
			}
			w.mu.Lock()
			w.stats.FailureCount++
			w.attemptSequence++
			w.unresolvedHours[hour] = w.attemptSequence
			w.updateCoverageStatsLocked(latestCompletedHour)
			w.mu.Unlock()
			if w.logger != nil {
				w.logger.ErrorContext(ctx, "observability hourly rollup recomputation failed",
					"event", "observability_rollup_recompute_failed",
					"hour", hour,
					"error", err,
				)
			}
			continue
		}

		successAt := w.clock.Now()
		w.mu.Lock()
		w.stats.LastSuccessAt = successAt
		if w.stats.LatestSuccessfulHour.IsZero() || hour.After(w.stats.LatestSuccessfulHour) {
			w.stats.LatestSuccessfulHour = hour
		}
		delete(w.unresolvedHours, hour)
		w.updateCoverageStatsLocked(latestCompletedHour)
		w.mu.Unlock()
	}
}

func (w *RollupWorker) recomputationHours(now time.Time) []time.Time {
	openHour := now.UTC().Truncate(time.Hour)
	currentHours := RecentCompletedHours(now, rollupRecomputationHours)
	currentSet := make(map[time.Time]struct{}, len(currentHours))
	for _, hour := range currentHours {
		currentSet[hour] = struct{}{}
	}
	type agedCandidate struct {
		hour                time.Time
		lastAttemptSequence uint64
	}
	aged := make([]agedCandidate, 0, agedOutRollupRetryBudget)
	w.mu.RLock()
	for hour, lastAttemptSequence := range w.unresolvedHours {
		hourUTC := hour.UTC().Truncate(time.Hour)
		if !hour.Equal(hourUTC) || !hourUTC.Before(openHour) {
			continue
		}
		if _, inCurrentWindow := currentSet[hourUTC]; inCurrentWindow {
			continue
		}
		aged = append(aged, agedCandidate{hour: hourUTC, lastAttemptSequence: lastAttemptSequence})
	}
	w.mu.RUnlock()
	sort.Slice(aged, func(left, right int) bool {
		if aged[left].lastAttemptSequence != aged[right].lastAttemptSequence {
			return aged[left].lastAttemptSequence < aged[right].lastAttemptSequence
		}
		return aged[left].hour.Before(aged[right].hour)
	})
	if len(aged) > agedOutRollupRetryBudget {
		aged = aged[:agedOutRollupRetryBudget]
	}
	hours := make([]time.Time, 0, len(currentHours)+len(aged))
	hours = append(hours, currentHours...)
	for _, candidate := range aged {
		hours = append(hours, candidate.hour)
	}
	return hours
}

func (w *RollupWorker) updateCoverageStatsLocked(latestCompletedHour time.Time) {
	var oldestUnresolved time.Time
	for hour := range w.unresolvedHours {
		if oldestUnresolved.IsZero() || hour.Before(oldestUnresolved) {
			oldestUnresolved = hour
		}
	}
	w.stats.OldestUnresolvedHour = oldestUnresolved
	if !oldestUnresolved.IsZero() {
		w.stats.LastCompletedHour = time.Time{}
		freshness := latestCompletedHour.Sub(oldestUnresolved) + time.Hour
		if freshness < time.Hour {
			freshness = time.Hour
		}
		w.stats.Freshness = freshness
		return
	}
	w.stats.LastCompletedHour = w.stats.LatestSuccessfulHour
	if w.stats.LatestSuccessfulHour.IsZero() {
		w.stats.Freshness = time.Duration(rollupRecomputationHours) * time.Hour
	} else if latestCompletedHour.After(w.stats.LatestSuccessfulHour) {
		w.stats.Freshness = latestCompletedHour.Sub(w.stats.LatestSuccessfulHour)
	} else {
		w.stats.Freshness = 0
	}
}

func (w *RollupWorker) Stats() RollupWorkerStats {
	if w == nil {
		return RollupWorkerStats{}
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.stats
}
