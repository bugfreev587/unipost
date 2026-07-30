package observabilityreads

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestDurationHistogramUsesTwelveDeterministicBuckets(t *testing.T) {
	boundaries := DurationHistogramUpperBoundsMS()
	if len(boundaries) != 12 {
		t.Fatalf("bucket count = %d, want 12", len(boundaries))
	}
	want := []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, math.Inf(1)}
	if !reflect.DeepEqual(boundaries[:], want) {
		t.Fatalf("bucket boundaries = %v, want %v", boundaries, want)
	}

	tests := []struct {
		duration int64
		bucket   int
	}{
		{duration: 0, bucket: 0},
		{duration: 5, bucket: 0},
		{duration: 6, bucket: 1},
		{duration: 10, bucket: 1},
		{duration: 11, bucket: 2},
		{duration: 25, bucket: 2},
		{duration: 26, bucket: 3},
		{duration: 50, bucket: 3},
		{duration: 51, bucket: 4},
		{duration: 100, bucket: 4},
		{duration: 101, bucket: 5},
		{duration: 250, bucket: 5},
		{duration: 251, bucket: 6},
		{duration: 500, bucket: 6},
		{duration: 501, bucket: 7},
		{duration: 1000, bucket: 7},
		{duration: 1001, bucket: 8},
		{duration: 2500, bucket: 8},
		{duration: 2501, bucket: 9},
		{duration: 5000, bucket: 9},
		{duration: 5001, bucket: 10},
		{duration: 10000, bucket: 10},
		{duration: 10001, bucket: 11},
		{duration: math.MaxInt64, bucket: 11},
	}
	for _, test := range tests {
		got, err := DurationHistogramBucket(test.duration)
		if err != nil || got != test.bucket {
			t.Fatalf("DurationHistogramBucket(%d) = %d, %v; want %d, nil", test.duration, got, err, test.bucket)
		}
	}
	if _, err := DurationHistogramBucket(-1); err == nil {
		t.Fatal("DurationHistogramBucket(-1) error = nil, want nonnegative-duration rejection")
	}
}

func TestApproximateDurationPercentilesReturnDeterministicBucketUpperBounds(t *testing.T) {
	histogram := []int64{1, 1, 2, 0, 1, 2, 1, 1, 0, 0, 0, 1}
	for _, test := range []struct {
		percentile float64
		want       int64
	}{
		{percentile: 0.50, want: 100},
		{percentile: 0.95, want: 12345},
		{percentile: 0.99, want: 12345},
	} {
		got, err := ApproximateDurationPercentile(histogram, 12345, test.percentile)
		if err != nil || got != test.want {
			t.Fatalf("ApproximateDurationPercentile(p=%v) = %d, %v; want %d, nil", test.percentile, got, err, test.want)
		}
	}
}

func TestApproximateDurationPercentileRejectsInvalidHistograms(t *testing.T) {
	tests := []struct {
		name      string
		histogram []int64
		maximum   int64
		p         float64
	}{
		{name: "wrong bucket count", histogram: make([]int64, 11), maximum: 1, p: .5},
		{name: "negative bucket", histogram: []int64{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -1}, maximum: 1, p: .5},
		{name: "empty", histogram: make([]int64, 12), maximum: 0, p: .5},
		{name: "negative maximum", histogram: []int64{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, maximum: -1, p: .5},
		{name: "zero percentile", histogram: []int64{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, maximum: 1, p: 0},
		{name: "percentile over one", histogram: []int64{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, maximum: 1, p: 1.01},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ApproximateDurationPercentile(test.histogram, test.maximum, test.p); err == nil {
				t.Fatal("error = nil, want invalid-input rejection")
			}
		})
	}
}

func TestApproximateDurationPercentileRejectsCountOverflowAndInconsistentFinalBucket(t *testing.T) {
	tests := []struct {
		name      string
		histogram []int64
		maximum   int64
	}{
		{name: "count overflow", histogram: []int64{math.MaxInt64, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, maximum: 10},
		{name: "unbounded bucket below boundary", histogram: []int64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, maximum: 10000},
		{name: "maximum above boundary without unbounded count", histogram: []int64{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, maximum: 10001},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ApproximateDurationPercentile(test.histogram, test.maximum, .5); err == nil {
				t.Fatal("error = nil, want structurally invalid histogram rejection")
			}
		})
	}
}

func TestApproximateDurationPercentileUsesExactNearestRankAboveFloatPrecision(t *testing.T) {
	const lowerBucketCount int64 = 1 << 52
	histogram := []int64{lowerBucketCount, lowerBucketCount + 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	got, err := ApproximateDurationPercentile(histogram, 10, .5)
	if err != nil {
		t.Fatal(err)
	}
	if got != 10 {
		t.Fatalf("p50 for total 2^53+1 = %dms, want 10ms because exact nearest rank falls in second bucket", got)
	}
}

type fakeRollupDB struct {
	tx       *fakeRollupTx
	beginErr error
}

func (d *fakeRollupDB) Begin(context.Context) (pgx.Tx, error) {
	return d.tx, d.beginErr
}

type fakeRollupTx struct {
	pgx.Tx
	execCount           int
	execSQL             []string
	execArgs            [][]any
	failExecAt          int
	cancelOnExecFailure context.CancelFunc
	commitErr           error
	committed           bool
	rollbackCount       int
	rollbackContextErr  error
	rollbackHasDeadline bool
	rollbackRemaining   time.Duration
}

func (tx *fakeRollupTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tx.execCount++
	tx.execSQL = append(tx.execSQL, sql)
	tx.execArgs = append(tx.execArgs, append([]any(nil), args...))
	if tx.execCount == tx.failExecAt {
		if tx.cancelOnExecFailure != nil {
			tx.cancelOnExecFailure()
		}
		return pgconn.CommandTag{}, errors.New("statement failed")
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func TestPostgresRollupStoreNormalizesInputToContainingUTCHour(t *testing.T) {
	tx := &fakeRollupTx{}
	store := NewPostgresRollupStore(&fakeRollupDB{tx: tx})
	store.now = func() time.Time { return time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC) }
	input := time.Date(2026, time.July, 29, 3, 37, 0, 0, time.FixedZone("minus-seven", -7*60*60))
	wantHour := time.Date(2026, time.July, 29, 10, 0, 0, 0, time.UTC)

	if err := store.RecomputeHour(context.Background(), input); err != nil {
		t.Fatalf("RecomputeHour(%s) error = %v, want normalization to %s", input, err, wantHour)
	}
	if !tx.committed || len(tx.execArgs) != 4 {
		t.Fatalf("committed=%v exec calls=%d, want true/4", tx.committed, len(tx.execArgs))
	}
	if got := tx.execArgs[1][0].(time.Time); !got.Equal(wantHour) {
		t.Fatalf("delete hour = %s, want %s", got, wantHour)
	}
	if gotStart, gotEnd := tx.execArgs[2][0].(time.Time), tx.execArgs[2][1].(time.Time); !gotStart.Equal(wantHour) || !gotEnd.Equal(wantHour.Add(time.Hour)) {
		t.Fatalf("insert range = [%s,%s), want [%s,%s)", gotStart, gotEnd, wantHour, wantHour.Add(time.Hour))
	}
}

func (tx *fakeRollupTx) Commit(context.Context) error {
	tx.committed = true
	return tx.commitErr
}

func (tx *fakeRollupTx) Rollback(ctx context.Context) error {
	tx.rollbackCount++
	tx.rollbackContextErr = ctx.Err()
	deadline, ok := ctx.Deadline()
	tx.rollbackHasDeadline = ok
	if ok {
		tx.rollbackRemaining = time.Until(deadline)
	}
	return nil
}

func TestPostgresRollupStoreTakesTransactionScopedObservabilityHourLockFirst(t *testing.T) {
	tx := &fakeRollupTx{}
	store := NewPostgresRollupStore(&fakeRollupDB{tx: tx})
	store.now = func() time.Time { return time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC) }
	hour := time.Date(2026, time.July, 29, 10, 37, 0, 0, time.UTC)

	if err := store.RecomputeHour(context.Background(), hour); err != nil {
		t.Fatal(err)
	}
	if len(tx.execSQL) == 0 || !strings.Contains(strings.ToLower(tx.execSQL[0]), "pg_advisory_xact_lock") {
		t.Fatalf("first transaction statement = %q, want transaction-scoped advisory lock", firstString(tx.execSQL))
	}
	if len(tx.execArgs[0]) != 2 || tx.execArgs[0][0] != int32(0x4f425356) || tx.execArgs[0][1] != int32(hour.UTC().Truncate(time.Hour).Unix()/3600) {
		t.Fatalf("advisory lock args = %#v, want observability namespace and normalized UTC hour", tx.execArgs[0])
	}
}

func TestPostgresRollupStoreMarksZeroEventHourCompleteInSameTransaction(t *testing.T) {
	tx := &fakeRollupTx{}
	store := NewPostgresRollupStore(&fakeRollupDB{tx: tx})
	store.now = func() time.Time { return time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC) }
	hour := time.Date(2026, time.July, 29, 10, 0, 0, 0, time.UTC)

	if err := store.RecomputeHour(context.Background(), hour); err != nil {
		t.Fatal(err)
	}
	if len(tx.execSQL) != 4 {
		t.Fatalf("transaction statements = %d, want lock/delete/insert/complete", len(tx.execSQL))
	}
	if !strings.Contains(strings.ToLower(tx.execSQL[3]), "api_request_metric_rollup_hours") {
		t.Fatalf("final transaction statement = %q, want completion marker", tx.execSQL[3])
	}
	if len(tx.execArgs[3]) != 1 || !tx.execArgs[3][0].(time.Time).Equal(hour) {
		t.Fatalf("completion marker args = %#v, want normalized hour", tx.execArgs[3])
	}
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func TestPostgresRollupStoreRollsBackStatementFailureWithLiveContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tx := &fakeRollupTx{failExecAt: 3, cancelOnExecFailure: cancel}
	store := NewPostgresRollupStore(&fakeRollupDB{tx: tx})
	store.now = func() time.Time { return time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC) }

	err := store.RecomputeHour(ctx, time.Date(2026, time.July, 29, 10, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("insert failure error = nil")
	}
	if tx.committed || tx.rollbackCount != 1 {
		t.Fatalf("committed=%v rollback count=%d, want false/1", tx.committed, tx.rollbackCount)
	}
	if tx.rollbackContextErr != nil {
		t.Fatalf("rollback context error = %v, want live rollback context", tx.rollbackContextErr)
	}
	if !tx.rollbackHasDeadline || tx.rollbackRemaining <= 0 || tx.rollbackRemaining > 5*time.Second {
		t.Fatalf("rollback deadline present=%v remaining=%s, want independent timeout in (0,5s]", tx.rollbackHasDeadline, tx.rollbackRemaining)
	}
}

func TestPostgresRollupStoreRollsBackCommitFailure(t *testing.T) {
	tx := &fakeRollupTx{commitErr: errors.New("commit failed")}
	store := NewPostgresRollupStore(&fakeRollupDB{tx: tx})
	store.now = func() time.Time { return time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC) }

	err := store.RecomputeHour(context.Background(), time.Date(2026, time.July, 29, 10, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("commit failure error = nil")
	}
	if !tx.committed || tx.rollbackCount != 1 {
		t.Fatalf("commit attempted=%v rollback count=%d, want true/1", tx.committed, tx.rollbackCount)
	}
}

func TestRecentCompletedHoursReturnsExactlyFortyEightUTCCompletedHours(t *testing.T) {
	now := time.Date(2026, time.July, 29, 12, 34, 56, 0, time.FixedZone("offset", -7*60*60))
	hours := RecentCompletedHours(now, 48)
	if len(hours) != 48 {
		t.Fatalf("hour count = %d, want 48", len(hours))
	}
	wantFirst := time.Date(2026, time.July, 27, 19, 0, 0, 0, time.UTC)
	wantLast := time.Date(2026, time.July, 29, 18, 0, 0, 0, time.UTC)
	if !hours[0].Equal(wantFirst) || !hours[len(hours)-1].Equal(wantLast) {
		t.Fatalf("range = [%s,%s], want [%s,%s]", hours[0], hours[len(hours)-1], wantFirst, wantLast)
	}
	for index, hour := range hours {
		if hour.Location() != time.UTC || !hour.Equal(hour.Truncate(time.Hour)) {
			t.Fatalf("hour[%d] = %s, want UTC hour boundary", index, hour)
		}
		if !hour.Before(now.UTC().Truncate(time.Hour)) {
			t.Fatalf("hour[%d] = %s includes open/future hour %s", index, hour, now.UTC().Truncate(time.Hour))
		}
		if index > 0 && !hour.Equal(hours[index-1].Add(time.Hour)) {
			t.Fatalf("hour[%d] = %s, want %s", index, hour, hours[index-1].Add(time.Hour))
		}
	}
}

type recordingRollupStore struct {
	mu          sync.Mutex
	hours       []time.Time
	failAt      map[time.Time]error
	called      chan time.Time
	blockUntil  <-chan struct{}
	callStarted chan struct{}
}

func (s *recordingRollupStore) RecomputeHour(ctx context.Context, hour time.Time) error {
	if s.callStarted != nil {
		select {
		case s.callStarted <- struct{}{}:
		default:
		}
	}
	if s.blockUntil != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.blockUntil:
		}
	}
	s.mu.Lock()
	s.hours = append(s.hours, hour)
	err := s.failAt[hour]
	s.mu.Unlock()
	if s.called != nil {
		s.called <- hour
	}
	return err
}

func (s *recordingRollupStore) snapshot() []time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Time(nil), s.hours...)
}

type manualRollupTicker struct {
	ch      chan time.Time
	stopped chan struct{}
}

func (t *manualRollupTicker) C() <-chan time.Time { return t.ch }
func (t *manualRollupTicker) Stop() {
	select {
	case <-t.stopped:
	default:
		close(t.stopped)
	}
}

type fixedRollupClock struct {
	mu              sync.Mutex
	now             time.Time
	ticker          *manualRollupTicker
	tickerDurations []time.Duration
}

type sequenceRollupClock struct {
	mu     sync.Mutex
	values []time.Time
	index  int
}

func (c *sequenceRollupClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.index >= len(c.values) {
		panic("sequence rollup clock exhausted")
	}
	value := c.values[c.index]
	c.index++
	return value
}

func (*sequenceRollupClock) NewTicker(time.Duration) rollupTicker {
	panic("sequence clock ticker is not used")
}

func (c *fixedRollupClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fixedRollupClock) NewTicker(duration time.Duration) rollupTicker {
	c.mu.Lock()
	c.tickerDurations = append(c.tickerDurations, duration)
	c.mu.Unlock()
	return c.ticker
}

func (c *fixedRollupClock) setNow(now time.Time) {
	c.mu.Lock()
	c.now = now
	c.mu.Unlock()
}

func (c *fixedRollupClock) tickerDurationSnapshot() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]time.Duration(nil), c.tickerDurations...)
}

func TestRollupWorkerRunsStartupAndHourlyRecomputationWithoutOpenHour(t *testing.T) {
	startupNow := time.Date(2026, time.July, 29, 12, 15, 0, 0, time.UTC)
	ticker := &manualRollupTicker{ch: make(chan time.Time, 1), stopped: make(chan struct{})}
	clock := &fixedRollupClock{now: startupNow, ticker: ticker}
	store := &recordingRollupStore{called: make(chan time.Time, 96)}
	worker := newRollupWorker(store, clock)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		worker.Start(ctx)
		close(done)
	}()

	for range 48 {
		<-store.called
	}
	wantStartup := RecentCompletedHours(startupNow, 48)
	if got := store.snapshot(); !reflect.DeepEqual(got, wantStartup) {
		t.Fatalf("startup hours = %v, want %v", got, wantStartup)
	}

	nextNow := startupNow.Add(time.Hour)
	clock.setNow(nextNow)
	ticker.ch <- nextNow
	for range 48 {
		<-store.called
	}
	wantAll := append(append([]time.Time(nil), wantStartup...), RecentCompletedHours(nextNow, 48)...)
	if got := store.snapshot(); !reflect.DeepEqual(got, wantAll) {
		t.Fatalf("startup + tick hours = %v, want %v", got, wantAll)
	}
	if got := clock.tickerDurationSnapshot(); !reflect.DeepEqual(got, []time.Duration{time.Hour}) {
		t.Fatalf("ticker durations = %v, want [%s]", got, time.Hour)
	}

	cancel()
	<-done
	select {
	case <-ticker.stopped:
	default:
		t.Fatal("worker did not stop its ticker")
	}
}

func TestRollupWorkerCancellationStopsBlockedStartupBatch(t *testing.T) {
	now := time.Date(2026, time.July, 29, 12, 15, 0, 0, time.UTC)
	ticker := &manualRollupTicker{ch: make(chan time.Time), stopped: make(chan struct{})}
	clock := &fixedRollupClock{now: now, ticker: ticker}
	block := make(chan struct{})
	store := &recordingRollupStore{blockUntil: block, callStarted: make(chan struct{}, 1)}
	worker := newRollupWorker(store, clock)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		worker.Start(ctx)
		close(done)
	}()
	<-store.callStarted
	cancel()
	<-done
	if got := len(store.snapshot()); got != 0 {
		t.Fatalf("completed recomputations after cancellation = %d, want 0", got)
	}
}

func TestRollupWorkerRecordsFailuresAndContinuesRemainingHours(t *testing.T) {
	now := time.Date(2026, time.July, 29, 12, 15, 0, 0, time.UTC)
	hours := RecentCompletedHours(now, 48)
	store := &recordingRollupStore{failAt: map[time.Time]error{hours[17]: errors.New("database unavailable")}}
	clock := &fixedRollupClock{now: now, ticker: &manualRollupTicker{ch: make(chan time.Time), stopped: make(chan struct{})}}
	worker := newRollupWorker(store, clock)

	worker.recomputeRecent(context.Background())

	if got := store.snapshot(); !reflect.DeepEqual(got, hours) {
		t.Fatalf("attempted hours = %v, want all hours despite one failure", got)
	}
	stats := worker.Stats()
	if stats.FailureCount != 1 {
		t.Fatalf("failure count = %d, want 1", stats.FailureCount)
	}
	if !stats.LastAttemptAt.Equal(now) || !stats.LastSuccessAt.Equal(now) {
		t.Fatalf("attempt/success = %s/%s, want %s/%s", stats.LastAttemptAt, stats.LastSuccessAt, now, now)
	}
	if !stats.LastCompletedHour.IsZero() {
		t.Fatalf("last contiguous completed hour = %s, want zero while a gap is unresolved", stats.LastCompletedHour)
	}
	if stats.Freshness != 31*time.Hour {
		t.Fatalf("freshness = %s, want 31h based on oldest unresolved hour", stats.Freshness)
	}
	if got := rollupStatsTimeField(t, stats, "OldestUnresolvedHour"); !got.Equal(hours[17]) {
		t.Fatalf("oldest unresolved hour = %s, want %s", got, hours[17])
	}
	if got := rollupStatsTimeField(t, stats, "LatestSuccessfulHour"); !got.Equal(hours[len(hours)-1]) {
		t.Fatalf("latest successful hour = %s, want %s", got, hours[len(hours)-1])
	}
}

func TestRollupWorkerLaterRunRepairsUnresolvedGap(t *testing.T) {
	now := time.Date(2026, time.July, 29, 12, 15, 0, 0, time.UTC)
	hours := RecentCompletedHours(now, 48)
	store := &recordingRollupStore{failAt: map[time.Time]error{hours[17]: errors.New("temporary failure")}}
	clock := &fixedRollupClock{now: now, ticker: &manualRollupTicker{ch: make(chan time.Time), stopped: make(chan struct{})}}
	worker := newRollupWorker(store, clock)
	worker.logger = nil

	worker.recomputeRecent(context.Background())
	store.mu.Lock()
	store.failAt = nil
	store.mu.Unlock()
	worker.recomputeRecent(context.Background())

	stats := worker.Stats()
	if stats.FailureCount != 1 || stats.Freshness != 0 {
		t.Fatalf("failure count/freshness after repair = %d/%s, want 1/0s", stats.FailureCount, stats.Freshness)
	}
	if !stats.LastCompletedHour.Equal(hours[len(hours)-1]) {
		t.Fatalf("last completed hour after repair = %s, want %s", stats.LastCompletedHour, hours[len(hours)-1])
	}
	if got := rollupStatsTimeField(t, stats, "OldestUnresolvedHour"); !got.IsZero() {
		t.Fatalf("oldest unresolved hour after repair = %s, want zero", got)
	}
}

func TestRollupWorkerRetriesUnresolvedHourAfterItAgesOutOfRecentWindow(t *testing.T) {
	firstNow := time.Date(2026, time.July, 29, 12, 15, 0, 0, time.UTC)
	firstWindow := RecentCompletedHours(firstNow, 48)
	agedOutHour := firstWindow[0]
	store := &recordingRollupStore{failAt: map[time.Time]error{agedOutHour: errors.New("temporary failure")}}
	clock := &fixedRollupClock{now: firstNow, ticker: &manualRollupTicker{ch: make(chan time.Time), stopped: make(chan struct{})}}
	worker := newRollupWorker(store, clock)
	worker.logger = nil

	worker.recomputeRecent(context.Background())
	store.mu.Lock()
	store.failAt = nil
	store.mu.Unlock()
	secondNow := firstNow.Add(time.Hour)
	clock.setNow(secondNow)
	worker.recomputeRecent(context.Background())

	allAttempts := store.snapshot()
	secondAttempts := allAttempts[len(firstWindow):]
	wantSecond := append(RecentCompletedHours(secondNow, 48), agedOutHour)
	if !reflect.DeepEqual(secondAttempts, wantSecond) {
		t.Fatalf("second-run hours = %v, want oldest unresolved plus current window %v", secondAttempts, wantSecond)
	}
	stats := worker.Stats()
	if !stats.OldestUnresolvedHour.IsZero() || stats.Freshness != 0 {
		t.Fatalf("oldest unresolved/freshness after aged-out repair = %s/%s, want zero/zero", stats.OldestUnresolvedHour, stats.Freshness)
	}
	if want := RecentCompletedHours(secondNow, 48)[47]; !stats.LastCompletedHour.Equal(want) {
		t.Fatalf("last completed hour after aged-out repair = %s, want %s", stats.LastCompletedHour, want)
	}
}

type alwaysFailRollupStore struct {
	mu    sync.Mutex
	hours []time.Time
}

func (s *alwaysFailRollupStore) RecomputeHour(_ context.Context, hour time.Time) error {
	s.mu.Lock()
	s.hours = append(s.hours, hour)
	s.mu.Unlock()
	return errors.New("sustained failure")
}

func (s *alwaysFailRollupStore) attemptsFrom(index int) []time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Time(nil), s.hours[index:]...)
}

func (s *alwaysFailRollupStore) attemptCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.hours)
}

func TestRollupWorkerBoundsAgedBacklogAndRotatesRetriesWithoutStarvingCurrentWindow(t *testing.T) {
	const agedRetryBudget = 4
	firstNow := time.Date(2026, time.July, 29, 12, 15, 0, 0, time.UTC)
	firstWindow := RecentCompletedHours(firstNow, 48)
	store := &alwaysFailRollupStore{}
	clock := &fixedRollupClock{now: firstNow, ticker: &manualRollupTicker{ch: make(chan time.Time), stopped: make(chan struct{})}}
	worker := newRollupWorker(store, clock)
	worker.logger = nil

	worker.recomputeRecent(context.Background())
	if got := store.attemptsFrom(0); !reflect.DeepEqual(got, firstWindow) {
		t.Fatalf("healthy-sized initial run = %v, want exact current 48 %v", got, firstWindow)
	}

	secondNow := firstNow.Add(48 * time.Hour)
	currentWindow := RecentCompletedHours(secondNow, 48)
	clock.setNow(secondNow)
	seenAged := make(map[time.Time]bool, len(firstWindow))
	for runIndex := 0; runIndex < len(firstWindow)/agedRetryBudget; runIndex++ {
		start := store.attemptCount()
		worker.recomputeRecent(context.Background())
		run := store.attemptsFrom(start)
		if len(run) != 48+agedRetryBudget {
			t.Fatalf("run %d attempts = %d, want at most 48+%d", runIndex, len(run), agedRetryBudget)
		}
		if !reflect.DeepEqual(run[:48], currentWindow) {
			t.Fatalf("run %d first 48 = %v, want current window %v", runIndex, run[:48], currentWindow)
		}
		for _, hour := range run[48:] {
			seenAged[hour] = true
		}
	}
	for _, hour := range firstWindow {
		if !seenAged[hour] {
			t.Fatalf("aged unresolved hour %s never received a fair retry", hour)
		}
	}
	worker.mu.RLock()
	unresolvedCount := len(worker.unresolvedHours)
	worker.mu.RUnlock()
	if unresolvedCount != 96 {
		t.Fatalf("retained unresolved count = %d, want all 96 failed hours", unresolvedCount)
	}
	stats := worker.Stats()
	if !stats.OldestUnresolvedHour.Equal(firstWindow[0]) || stats.Freshness == 0 {
		t.Fatalf("oldest unresolved/freshness = %s/%s, want retained oldest and nonzero freshness", stats.OldestUnresolvedHour, stats.Freshness)
	}
}

func TestRollupWorkerAllHoursFailKeepsDataUnavailableAndFreshnessStale(t *testing.T) {
	now := time.Date(2026, time.July, 29, 12, 15, 0, 0, time.UTC)
	hours := RecentCompletedHours(now, 48)
	failures := make(map[time.Time]error, len(hours))
	for _, hour := range hours {
		failures[hour] = errors.New("database unavailable")
	}
	store := &recordingRollupStore{failAt: failures}
	clock := &fixedRollupClock{now: now, ticker: &manualRollupTicker{ch: make(chan time.Time), stopped: make(chan struct{})}}
	worker := newRollupWorker(store, clock)
	worker.logger = nil

	worker.recomputeRecent(context.Background())

	stats := worker.Stats()
	if stats.FailureCount != uint64(len(hours)) {
		t.Fatalf("failure count = %d, want %d", stats.FailureCount, len(hours))
	}
	if !stats.LastAttemptAt.Equal(now) {
		t.Fatalf("last attempt = %s, want %s", stats.LastAttemptAt, now)
	}
	if !stats.LastSuccessAt.IsZero() || !stats.LastCompletedHour.IsZero() {
		t.Fatalf("last success/completed = %s/%s, want unavailable zero values", stats.LastSuccessAt, stats.LastCompletedHour)
	}
	if stats.Freshness != 48*time.Hour {
		t.Fatalf("freshness = %s, want at least the unavailable catch-up window %s", stats.Freshness, 48*time.Hour)
	}
	if got := rollupStatsTimeField(t, stats, "OldestUnresolvedHour"); !got.Equal(hours[0]) {
		t.Fatalf("oldest unresolved hour = %s, want %s", got, hours[0])
	}
}

func rollupStatsTimeField(t *testing.T, stats RollupWorkerStats, name string) time.Time {
	t.Helper()
	field := reflect.ValueOf(stats).FieldByName(name)
	if !field.IsValid() || field.Type() != reflect.TypeOf(time.Time{}) {
		t.Fatalf("RollupWorkerStats missing time field %s", name)
	}
	return field.Interface().(time.Time)
}

type deadlineRecordingRollupStore struct {
	mu            sync.Mutex
	hours         []time.Time
	firstTimedOut bool
	called        chan time.Time
}

func (s *deadlineRecordingRollupStore) RecomputeHour(ctx context.Context, hour time.Time) error {
	s.mu.Lock()
	call := len(s.hours)
	s.hours = append(s.hours, hour)
	s.mu.Unlock()
	if call == 0 {
		if _, ok := ctx.Deadline(); !ok {
			return errors.New("per-hour deadline is missing")
		}
		<-ctx.Done()
		s.mu.Lock()
		s.firstTimedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
		s.mu.Unlock()
		if s.called != nil {
			s.called <- hour
		}
		return ctx.Err()
	}
	if s.called != nil {
		s.called <- hour
	}
	return nil
}

func (s *deadlineRecordingRollupStore) snapshot() ([]time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Time(nil), s.hours...), s.firstTimedOut
}

func TestRollupWorkerTimesOutBlockedHourContinuesAndProcessesNextTick(t *testing.T) {
	startupNow := time.Date(2026, time.July, 29, 12, 15, 0, 0, time.UTC)
	ticker := &manualRollupTicker{ch: make(chan time.Time, 1), stopped: make(chan struct{})}
	clock := &fixedRollupClock{now: startupNow, ticker: ticker}
	store := &deadlineRecordingRollupStore{called: make(chan time.Time, 97)}
	worker := newRollupWorker(store, clock)
	configurable, ok := any(worker).(interface{ setHourTimeout(time.Duration) })
	if !ok {
		t.Fatal("rollup worker lacks injectable per-hour timeout")
	}
	configurable.setHourTimeout(5 * time.Millisecond)
	worker.logger = nil
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		worker.Start(ctx)
		close(done)
	}()

	for range 48 {
		<-store.called
	}
	nextNow := startupNow.Add(time.Hour)
	clock.setNow(nextNow)
	ticker.ch <- nextNow
	for range 49 {
		<-store.called
	}
	cancel()
	<-done
	hours, timedOut := store.snapshot()
	if !timedOut || len(hours) != 97 {
		t.Fatalf("first timed out=%v total attempts=%d, want true/97 including aged-out gap repair", timedOut, len(hours))
	}
	if worker.Stats().FailureCount != 1 {
		t.Fatalf("failure count = %d, want 1", worker.Stats().FailureCount)
	}
}

func TestRollupWorkerRecordsActualPerHourAttemptAndSuccessTimes(t *testing.T) {
	batchNow := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	values := []time.Time{batchNow}
	for index := range 48 {
		values = append(values, batchNow.Add(time.Duration(index*2+1)*time.Second))
		values = append(values, batchNow.Add(time.Duration(index*2+2)*time.Second))
	}
	clock := &sequenceRollupClock{values: values}
	worker := newRollupWorker(&recordingRollupStore{}, clock)
	worker.recomputeRecent(context.Background())

	stats := worker.Stats()
	if !stats.LastAttemptAt.Equal(values[len(values)-2]) || !stats.LastSuccessAt.Equal(values[len(values)-1]) {
		t.Fatalf("last attempt/success = %s/%s, want %s/%s", stats.LastAttemptAt, stats.LastSuccessAt, values[len(values)-2], values[len(values)-1])
	}
}
