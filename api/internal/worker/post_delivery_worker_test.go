package worker

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/handler"
)

type dispatchWorkerDB struct {
	jobs    []db.PostDeliveryJob
	started chan string
	release chan struct{}
}

func (f *dispatchWorkerDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected Exec")
}

func (f *dispatchWorkerDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	switch {
	case strings.Contains(query, "-- name: ListStaleActivePostDeliveryJobs"):
		return &postDeliveryJobRows{}, nil
	case strings.Contains(query, "-- name: ClaimPostDispatchJobs"):
		return &postDeliveryJobRows{jobs: f.jobs}, nil
	default:
		return nil, errors.New("unexpected Query: " + query)
	}
}

func (f *dispatchWorkerDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetPostDeliveryJobByIDAndWorkspace"):
		id := args[0].(string)
		for _, job := range f.jobs {
			if job.ID == id {
				return postDeliveryJobRow{job: job}
			}
		}
		return errScanRow{err: errors.New("job not found")}
	case strings.Contains(query, "-- name: GetSocialPostByID"):
		return blockingPostRow{
			postID:  args[0].(string),
			started: f.started,
			release: f.release,
		}
	default:
		return errScanRow{err: errors.New("unexpected QueryRow: " + query)}
	}
}

type postDeliveryJobRows struct {
	jobs []db.PostDeliveryJob
	idx  int
}

func (r *postDeliveryJobRows) Close()                                       {}
func (r *postDeliveryJobRows) Err() error                                   { return nil }
func (r *postDeliveryJobRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *postDeliveryJobRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *postDeliveryJobRows) Values() ([]interface{}, error)               { return nil, errors.New("unused") }
func (r *postDeliveryJobRows) RawValues() [][]byte                          { return nil }
func (r *postDeliveryJobRows) Conn() *pgx.Conn                              { return nil }

func (r *postDeliveryJobRows) Next() bool {
	if r.idx >= len(r.jobs) {
		return false
	}
	r.idx++
	return true
}

func (r *postDeliveryJobRows) Scan(dest ...interface{}) error {
	if r.idx == 0 || r.idx > len(r.jobs) {
		return errors.New("scan without row")
	}
	return scanPostDeliveryJob(dest, r.jobs[r.idx-1])
}

type postDeliveryJobRow struct {
	job db.PostDeliveryJob
}

func (r postDeliveryJobRow) Scan(dest ...interface{}) error {
	return scanPostDeliveryJob(dest, r.job)
}

type blockingPostRow struct {
	postID  string
	started chan string
	release chan struct{}
}

func (r blockingPostRow) Scan(...interface{}) error {
	r.started <- r.postID
	<-r.release
	return errors.New("stop after publish path started")
}

type errScanRow struct {
	err error
}

func (r errScanRow) Scan(...interface{}) error { return r.err }

func scanPostDeliveryJob(dest []interface{}, job db.PostDeliveryJob) error {
	values := []interface{}{
		job.ID,
		job.PostID,
		job.SocialPostResultID,
		job.WorkspaceID,
		job.SocialAccountID,
		job.Platform,
		job.PostInputIndex,
		job.Kind,
		job.State,
		job.Attempts,
		job.MaxAttempts,
		job.FailureStage,
		job.ErrorCode,
		job.PlatformErrorCode,
		job.LastError,
		job.NextRunAt,
		job.LastAttemptAt,
		job.CreatedAt,
		job.UpdatedAt,
		job.FinishedAt,
		job.DismissedAt,
		job.LeaseExpiresAt,
		job.LeaseOwner,
		job.FirstClaimedAt,
		job.PlatformStartedAt,
	}
	if len(dest) != len(values) {
		return errors.New("unexpected post_delivery_jobs scan shape")
	}
	for i := range dest {
		target := reflect.ValueOf(dest[i])
		if target.Kind() != reflect.Ptr || target.IsNil() {
			return errors.New("scan target is not a pointer")
		}
		target.Elem().Set(reflect.ValueOf(values[i]))
	}
	return nil
}

func dispatchWorkerTestJob(id string) db.PostDeliveryJob {
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	return db.PostDeliveryJob{
		ID:                 id,
		PostID:             "post-" + id,
		SocialPostResultID: "result-" + id,
		WorkspaceID:        "workspace-1",
		SocialAccountID:    "account-" + id,
		Platform:           "twitter",
		PostInputIndex:     0,
		Kind:               "dispatch",
		State:              "running",
		Attempts:           1,
		MaxAttempts:        5,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

func TestPostDispatchWorkerProcessesClaimedJobsConcurrently(t *testing.T) {
	dbtx := &dispatchWorkerDB{
		jobs: []db.PostDeliveryJob{
			dispatchWorkerTestJob("one"),
			dispatchWorkerTestJob("two"),
		},
		started: make(chan string, 2),
		release: make(chan struct{}),
	}
	queries := db.New(dbtx)
	worker := NewPostDispatchWorker(
		queries,
		handler.NewSocialPostHandler(queries, nil, nil, nil, nil, nil, nil),
	)

	done := make(chan struct{})
	go func() {
		worker.runOnce(context.Background())
		close(done)
	}()

	select {
	case <-dbtx.started:
	case <-time.After(time.Second):
		close(dbtx.release)
		t.Fatal("first claimed job never reached the publish path")
	}

	select {
	case <-dbtx.started:
	case <-time.After(250 * time.Millisecond):
		close(dbtx.release)
		<-done
		t.Fatal("second claimed job did not start until the first one finished; claimed batches must run concurrently")
	}

	close(dbtx.release)
	<-done
}

func TestPostDispatchWorkerRunOnceDoesNotWaitForClaimedJobsToFinish(t *testing.T) {
	dbtx := &dispatchWorkerDB{
		jobs: []db.PostDeliveryJob{
			dispatchWorkerTestJob("one"),
		},
		started: make(chan string, 1),
		release: make(chan struct{}),
	}
	queries := db.New(dbtx)
	worker := NewPostDispatchWorkerWithConfig(
		queries,
		handler.NewSocialPostHandler(queries, nil, nil, nil, nil, nil, nil),
		PostDeliveryWorkerConfig{
			ClaimBatchLimit:        20,
			WorkspaceConcurrentCap: 30,
			GlobalConcurrency:      10,
		},
	)

	done := make(chan struct{})
	go func() {
		worker.runOnce(context.Background())
		close(done)
	}()

	select {
	case <-dbtx.started:
	case <-time.After(time.Second):
		close(dbtx.release)
		t.Fatal("claimed job never reached the publish path")
	}

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		close(dbtx.release)
		t.Fatal("runOnce waited for the claimed job to finish before returning")
	}

	close(dbtx.release)
}

type blockingDeliveryProcessor struct {
	started chan string
	release chan string

	mu       sync.Mutex
	blockers map[string]chan struct{}
}

func newBlockingDeliveryProcessor() *blockingDeliveryProcessor {
	return &blockingDeliveryProcessor{
		started:  make(chan string, 8),
		release:  make(chan string, 8),
		blockers: map[string]chan struct{}{},
	}
}

func (p *blockingDeliveryProcessor) ProcessPostDeliveryJob(_ context.Context, job db.PostDeliveryJob) error {
	ch := make(chan struct{})
	p.mu.Lock()
	p.blockers[job.ID] = ch
	p.mu.Unlock()

	p.started <- job.ID
	<-ch
	return nil
}

func (p *blockingDeliveryProcessor) unblock(jobID string) {
	p.mu.Lock()
	ch := p.blockers[jobID]
	p.mu.Unlock()
	if ch != nil {
		close(ch)
	}
}

func TestPostDeliveryExecutorRespectsGlobalConcurrency(t *testing.T) {
	processor := newBlockingDeliveryProcessor()
	executor := newPostDeliveryExecutor(
		db.New(leaseNoopDB{}),
		processor,
		PostDeliveryWorkerConfig{GlobalConcurrency: 1},
		"test worker",
	)
	executor.submitReserved(context.Background(), []db.PostDeliveryJob{
		dispatchWorkerTestJob("one"),
		dispatchWorkerTestJob("two"),
	})

	first := waitStarted(t, processor.started)
	if first != "one" && first != "two" {
		t.Fatalf("first started job = %q, want one or two", first)
	}
	select {
	case got := <-processor.started:
		t.Fatalf("job %q started while the single global slot was full", got)
	case <-time.After(100 * time.Millisecond):
	}

	processor.unblock(first)
	secondWant := "one"
	if first == "one" {
		secondWant = "two"
	}
	if got := waitStarted(t, processor.started); got != secondWant {
		t.Fatalf("second started job = %q, want %q", got, secondWant)
	}
	processor.unblock(secondWant)
}

func TestPostDeliveryExecutorRespectsPlatformConcurrencyWithoutBlockingOtherPlatforms(t *testing.T) {
	processor := newBlockingDeliveryProcessor()
	executor := newPostDeliveryExecutor(
		db.New(leaseNoopDB{}),
		processor,
		PostDeliveryWorkerConfig{
			GlobalConcurrency: 2,
			PlatformConcurrencyCaps: map[string]int{
				"instagram": 1,
				"twitter":   1,
			},
		},
		"test worker",
	)
	igOne := dispatchWorkerTestJob("ig-one")
	igOne.Platform = "instagram"
	igTwo := dispatchWorkerTestJob("ig-two")
	igTwo.Platform = "instagram"
	twitter := dispatchWorkerTestJob("twitter-one")
	twitter.Platform = "twitter"

	executor.submitReserved(context.Background(), []db.PostDeliveryJob{igOne, igTwo, twitter})

	first := waitStarted(t, processor.started)
	second := waitStarted(t, processor.started)
	started := map[string]bool{first: true, second: true}
	var startedIG string
	var waitingIG string
	switch {
	case started["ig-one"] && started["twitter-one"]:
		startedIG = "ig-one"
		waitingIG = "ig-two"
	case started["ig-two"] && started["twitter-one"]:
		startedIG = "ig-two"
		waitingIG = "ig-one"
	default:
		t.Fatalf("started jobs = %v, want one instagram job and twitter-one while the other instagram waits", started)
	}
	select {
	case got := <-processor.started:
		t.Fatalf("job %q started while instagram cap was full", got)
	case <-time.After(100 * time.Millisecond):
	}

	processor.unblock(startedIG)
	if got := waitStarted(t, processor.started); got != waitingIG {
		t.Fatalf("job after releasing instagram cap = %q, want %q", got, waitingIG)
	}
	processor.unblock(waitingIG)
	processor.unblock("twitter-one")
}

func waitStarted(t *testing.T, started <-chan string) string {
	t.Helper()
	select {
	case id := <-started:
		return id
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for job to start")
		return ""
	}
}
