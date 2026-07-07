package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/integrationlogs"
)

func TestRetryDeliveryJobNowMarksDeprecated(t *testing.T) {
	h := &SocialPostHandler{}
	req := httptest.NewRequest(http.MethodPost, "/v1/post-delivery-jobs//retry-now", nil)
	rr := httptest.NewRecorder()

	h.RetryDeliveryJobNow(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when job id is missing", rr.Code)
	}
	if got := rr.Header().Get("Deprecation"); got != "true" {
		t.Fatalf("Deprecation header = %q, want true", got)
	}
	if got := rr.Header().Get("Sunset"); got == "" {
		t.Fatal("expected Sunset header on legacy retry-now alias")
	}
	if got := rr.Header().Get("Link"); got == "" {
		t.Fatal("expected Link header pointing to canonical retry route")
	}
}

func TestWorkerPublishingEventSourceIsWorker(t *testing.T) {
	event := workerPublishingEvent(integrationlogs.Event{
		Action: integrationlogs.ActionPostPublishPlatformFailed,
	})

	if event.Source != integrationlogs.SourceWorker {
		t.Fatalf("source = %q, want %q", event.Source, integrationlogs.SourceWorker)
	}
}

func TestResolvePublishingEventSourcePreservesExplicitSource(t *testing.T) {
	got := resolvePublishingEventSource(context.Background(), integrationlogs.Event{
		Source: integrationlogs.SourceWorker,
	})

	if got != integrationlogs.SourceWorker {
		t.Fatalf("source = %q, want %q", got, integrationlogs.SourceWorker)
	}
}

func TestResolvePublishingEventSourceUsesAPIWhenAPIKeyPresent(t *testing.T) {
	ctx := context.WithValue(context.Background(), auth.APIKeyIDKey, "api_key_123")

	got := resolvePublishingEventSource(ctx, integrationlogs.Event{})

	if got != integrationlogs.SourceAPI {
		t.Fatalf("source = %q, want %q", got, integrationlogs.SourceAPI)
	}
}

func TestResolvePublishingEventSourceDefaultsToDashboard(t *testing.T) {
	got := resolvePublishingEventSource(context.Background(), integrationlogs.Event{})

	if got != integrationlogs.SourceDashboard {
		t.Fatalf("source = %q, want %q", got, integrationlogs.SourceDashboard)
	}
}

func TestPostFailureShouldMarkReconnectRequired(t *testing.T) {
	arg := db.CreatePostFailureParams{
		ErrorCode:       "account_reconnect_required",
		SocialAccountID: pgtype.Text{String: "acc_threads", Valid: true},
	}
	if !postFailureShouldMarkReconnectRequired(arg) {
		t.Fatal("expected account_reconnect_required with account id to mark reconnect required")
	}

	arg.ErrorCode = "missing_permission"
	if postFailureShouldMarkReconnectRequired(arg) {
		t.Fatal("missing_permission should not mark reconnect required")
	}

	arg.ErrorCode = "account_reconnect_required"
	arg.SocialAccountID = pgtype.Text{}
	if postFailureShouldMarkReconnectRequired(arg) {
		t.Fatal("missing account id should not mark reconnect required")
	}
}

func TestSanitizeDeliveryErrorTextRemovesNULAndInvalidUTF8(t *testing.T) {
	got := sanitizeDeliveryErrorText("tiktok upload failed:\x00" + string([]byte{0xff, 0xfe}) + "done")

	if strings.Contains(got, "\x00") {
		t.Fatalf("sanitized error still contains NUL: %q", got)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("sanitized error is not valid UTF-8: %q", got)
	}
	if !strings.Contains(got, "tiktok upload failed:done") {
		t.Fatalf("sanitized error = %q, want stable surrounding text", got)
	}
}

func TestRecoverStaleDeliveryJobsCancelsJobWhenParentPostIsDeleted(t *testing.T) {
	staleAt := time.Now().Add(-10 * time.Minute)
	job := db.PostDeliveryJob{
		ID:                 "job_stale_deleted",
		PostID:             "post_deleted",
		SocialPostResultID: "result_deleted",
		WorkspaceID:        "ws_1",
		SocialAccountID:    "acct_tiktok",
		Platform:           "tiktok",
		PostInputIndex:     0,
		Kind:               "dispatch",
		State:              "running",
		Attempts:           1,
		MaxAttempts:        5,
		LastAttemptAt:      pgtype.Timestamptz{Time: staleAt, Valid: true},
	}
	dbtx := &staleDeletedPostQueueDB{staleJob: job}
	h := &SocialPostHandler{queries: db.New(dbtx)}

	if err := h.RecoverStaleDeliveryJobs(context.Background(), 5*time.Minute); err != nil {
		t.Fatalf("RecoverStaleDeliveryJobs returned error: %v", err)
	}

	if dbtx.markedJobID != job.ID {
		t.Fatalf("marked job id = %q, want %q", dbtx.markedJobID, job.ID)
	}
	if dbtx.markedState != "cancelled" {
		t.Fatalf("marked state = %q, want cancelled", dbtx.markedState)
	}
	if dbtx.markedFailureStage != "post_deleted" {
		t.Fatalf("failure stage = %q, want post_deleted", dbtx.markedFailureStage)
	}
	if dbtx.createdRetryJob {
		t.Fatal("deleted post recovery must not create a retry job")
	}
}

func TestDeletePostCancelsActiveDeliveryJobs(t *testing.T) {
	dbtx := &deletePostQueueDB{post: db.SocialPost{
		ID:          "post_delete",
		WorkspaceID: "ws_1",
		Status:      "publishing",
		CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}}
	h := &SocialPostHandler{queries: db.New(dbtx)}
	req := httptest.NewRequest(http.MethodDelete, "/v1/posts/post_delete", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = withChiParam(req, "id", "post_delete")
	rr := httptest.NewRecorder()

	h.Delete(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if !dbtx.cancelActiveCalled {
		t.Fatal("delete should cancel active delivery jobs for the post")
	}
}

func TestDeletePostSucceedsWhenActiveDeliveryCancelFailsAfterSoftDelete(t *testing.T) {
	dbtx := &deletePostQueueDB{
		post: db.SocialPost{
			ID:          "post_delete",
			WorkspaceID: "ws_1",
			Status:      "publishing",
			CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
		},
		cancelActiveErr: errors.New("database temporarily unavailable"),
	}
	h := &SocialPostHandler{queries: db.New(dbtx)}
	req := httptest.NewRequest(http.MethodDelete, "/v1/posts/post_delete", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = withChiParam(req, "id", "post_delete")
	rr := httptest.NewRecorder()

	h.Delete(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 after soft delete succeeded; body=%s", rr.Code, rr.Body.String())
	}
	if !dbtx.cancelActiveCalled {
		t.Fatal("delete should still attempt to cancel active delivery jobs")
	}
}

func TestInlineRefreshFailureShouldAbortPublish(t *testing.T) {
	if !inlineRefreshFailureShouldAbortPublish(errors.New(`refresh failed (400): {"error":{"message":"Error validating access token: Session has expired","type":"OAuthException","code":190}}`)) {
		t.Fatal("expected expired Meta OAuth refresh failure to abort publish")
	}

	if inlineRefreshFailureShouldAbortPublish(errors.New(`refresh failed (500): {"error":{"message":"temporarily unavailable"}}`)) {
		t.Fatal("temporary refresh failures should not abort publish")
	}

	if inlineRefreshFailureShouldAbortPublish(nil) {
		t.Fatal("nil refresh error should not abort publish")
	}
}

type deletePostQueueDB struct {
	post               db.SocialPost
	cancelActiveCalled bool
	cancelActiveErr    error
}

func (f *deletePostQueueDB) Exec(_ context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(query, "-- name: CancelActivePostDeliveryJobsByPost"):
		if got := args[0].(string); got != f.post.ID {
			return pgconn.CommandTag{}, errors.New("cancel active called with wrong post id")
		}
		if got := args[1].(string); got != f.post.WorkspaceID {
			return pgconn.CommandTag{}, errors.New("cancel active called with wrong workspace id")
		}
		f.cancelActiveCalled = true
		return pgconn.CommandTag{}, f.cancelActiveErr
	default:
		return pgconn.CommandTag{}, errors.New("unexpected Exec: " + query)
	}
}

func (f *deletePostQueueDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query")
}

func (f *deletePostQueueDB) QueryRow(_ context.Context, query string, _ ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: SoftDeleteSocialPost"):
		return socialPostScanRow(f.post)
	default:
		return scanRow{err: errors.New("unexpected QueryRow: " + query)}
	}
}

type staleDeletedPostQueueDB struct {
	staleJob db.PostDeliveryJob

	markedJobID        string
	markedState        string
	markedFailureStage string
	createdRetryJob    bool
}

func (f *staleDeletedPostQueueDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected Exec")
}

func (f *staleDeletedPostQueueDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	switch {
	case strings.Contains(query, "-- name: ListStaleActivePostDeliveryJobs"):
		return &queueRows{values: [][]any{postDeliveryJobValues(f.staleJob)}}, nil
	default:
		return nil, errors.New("unexpected Query: " + query)
	}
}

func (f *staleDeletedPostQueueDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetSocialPostByID"):
		return scanRow{err: pgx.ErrNoRows}
	case strings.Contains(query, "-- name: MarkPostDeliveryJobFailed"):
		f.markedJobID = args[0].(string)
		f.markedState = args[1].(string)
		f.markedFailureStage = args[2].(pgtype.Text).String
		updated := f.staleJob
		updated.State = f.markedState
		updated.FailureStage = args[2].(pgtype.Text)
		updated.ErrorCode = args[3].(pgtype.Text)
		updated.PlatformErrorCode = args[4].(pgtype.Text)
		updated.LastError = args[5].(pgtype.Text)
		updated.NextRunAt = args[6].(pgtype.Timestamptz)
		return postDeliveryJobScanRow(updated)
	case strings.Contains(query, "-- name: CreatePostDeliveryJob"):
		f.createdRetryJob = true
		return scanRow{err: errors.New("deleted post recovery should not create retry jobs")}
	default:
		return scanRow{err: errors.New("unexpected QueryRow: " + query)}
	}
}

type queueRows struct {
	values [][]any
	index  int
}

func (r *queueRows) Close()                                       {}
func (r *queueRows) Err() error                                   { return nil }
func (r *queueRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *queueRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *queueRows) Next() bool {
	if r.index >= len(r.values) {
		return false
	}
	r.index++
	return true
}
func (r *queueRows) Scan(dest ...any) error {
	if r.index == 0 || r.index > len(r.values) {
		return errors.New("Scan called without current row")
	}
	return scanRow{values: r.values[r.index-1]}.Scan(dest...)
}
func (r *queueRows) Values() ([]any, error) { return r.values[r.index-1], nil }
func (r *queueRows) RawValues() [][]byte    { return nil }
func (r *queueRows) Conn() *pgx.Conn        { return nil }

func postDeliveryJobScanRow(job db.PostDeliveryJob) scanRow {
	return scanRow{values: postDeliveryJobValues(job)}
}

func postDeliveryJobValues(job db.PostDeliveryJob) []any {
	return []any{
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
	}
}

func socialPostScanRow(post db.SocialPost) scanRow {
	return scanRow{values: []any{
		post.ID,
		post.Caption,
		post.MediaUrls,
		post.Status,
		post.ScheduledAt,
		post.PublishedAt,
		post.CreatedAt,
		post.Metadata,
		post.IdempotencyKey,
		post.WorkspaceID,
		post.ArchivedAt,
		post.DeletedAt,
		post.Source,
		post.ProfileIds,
	}}
}

// --- Double-publish hotfix regression tests ---
//
// Root cause: a claimed delivery job can wait minutes in the worker's serial
// processing queue. Meanwhile the stale-recovery sweep marks it failed and
// queues a retry. Without a guard, the original worker still publishes when it
// finally runs, and the retry publishes too -> the same scheduled post is
// delivered to the platform multiple times.

func baseDeliveryJob() db.PostDeliveryJob {
	return db.PostDeliveryJob{
		ID:                 "job_1",
		PostID:             "post_1",
		SocialPostResultID: "result_1",
		WorkspaceID:        "ws_1",
		SocialAccountID:    "acct_ig",
		Platform:           "instagram",
		Kind:               "dispatch",
		State:              "running",
		Attempts:           1,
		MaxAttempts:        5,
	}
}

// guardedDeliveryDB answers the pre-publish state read and treats any other
// query as "advanced past the guard toward publishing".
type guardedDeliveryDB struct {
	current            db.PostDeliveryJob
	getJobCalls        int
	reachedPublishPath bool
}

func (f *guardedDeliveryDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	f.reachedPublishPath = true
	return pgconn.CommandTag{}, errors.New("unexpected Exec")
}

func (f *guardedDeliveryDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	f.reachedPublishPath = true
	return nil, errors.New("unexpected Query")
}

func (f *guardedDeliveryDB) QueryRow(_ context.Context, query string, _ ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetPostDeliveryJobByIDAndWorkspace"):
		f.getJobCalls++
		return postDeliveryJobScanRow(f.current)
	default:
		f.reachedPublishPath = true
		return scanRow{err: errors.New("advanced past guard: " + query)}
	}
}

func TestProcessPostDeliveryJobSkipsWhenStaleRecoveryAlreadyFailedIt(t *testing.T) {
	claimed := baseDeliveryJob() // worker still holds its in-memory "running" copy
	current := baseDeliveryJob()
	current.State = "failed" // DB now reports stale recovery already re-queued it
	dbtx := &guardedDeliveryDB{current: current}
	h := &SocialPostHandler{queries: db.New(dbtx)}

	if err := h.ProcessPostDeliveryJob(context.Background(), claimed); err != nil {
		t.Fatalf("ProcessPostDeliveryJob returned error: %v", err)
	}
	if dbtx.getJobCalls != 1 {
		t.Fatalf("pre-publish guard read count = %d, want 1", dbtx.getJobCalls)
	}
	if dbtx.reachedPublishPath {
		t.Fatal("worker reached the publish path for a job that is no longer active (double-publish risk)")
	}
}

func TestProcessPostDeliveryJobProceedsWhenStillActive(t *testing.T) {
	claimed := baseDeliveryJob()
	current := baseDeliveryJob() // DB still reports running -> guard must let it through
	dbtx := &guardedDeliveryDB{current: current}
	h := &SocialPostHandler{queries: db.New(dbtx)}

	err := h.ProcessPostDeliveryJob(context.Background(), claimed)
	if err == nil {
		t.Fatal("expected job to advance past the guard and fail on the stub publish path")
	}
	if !dbtx.reachedPublishPath {
		t.Fatal("guard blocked an active job from publishing")
	}
}

// stalePublishedResultDB drives recoverStaleDeliveryJob for a result that a
// prior attempt already published.
type stalePublishedResultDB struct {
	staleJob          db.PostDeliveryJob
	post              db.SocialPost
	result            db.SocialPostResult
	markedSucceededID string
	createdRetryJob   bool
}

func (f *stalePublishedResultDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected Exec")
}

func (f *stalePublishedResultDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	if strings.Contains(query, "-- name: ListStaleActivePostDeliveryJobs") {
		return &queueRows{values: [][]any{postDeliveryJobValues(f.staleJob)}}, nil
	}
	return nil, errors.New("unexpected Query: " + query)
}

func (f *stalePublishedResultDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetSocialPostByID"):
		return socialPostScanRow(f.post)
	case strings.Contains(query, "-- name: GetSocialPostResultByIDAndPost"):
		return socialPostResultScanRow(f.result)
	case strings.Contains(query, "-- name: MarkPostDeliveryJobSucceeded"):
		f.markedSucceededID = args[0].(string)
		succeeded := f.staleJob
		succeeded.State = "succeeded"
		return postDeliveryJobScanRow(succeeded)
	case strings.Contains(query, "-- name: CreatePostDeliveryJob"):
		f.createdRetryJob = true
		return scanRow{err: errors.New("must not create a retry job when result already published")}
	default:
		return scanRow{err: errors.New("unexpected QueryRow: " + query)}
	}
}

func TestRecoverStaleDeliveryJobSkipsRetryWhenResultAlreadyPublished(t *testing.T) {
	job := baseDeliveryJob()
	job.LastAttemptAt = pgtype.Timestamptz{Time: time.Now().Add(-10 * time.Minute), Valid: true}
	dbtx := &stalePublishedResultDB{
		staleJob: job,
		post: db.SocialPost{
			ID:          job.PostID,
			WorkspaceID: job.WorkspaceID,
			Status:      "publishing",
			CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
		},
		result: db.SocialPostResult{
			ID:              job.SocialPostResultID,
			PostID:          job.PostID,
			SocialAccountID: job.SocialAccountID,
			Status:          "published",
		},
	}
	h := &SocialPostHandler{queries: db.New(dbtx)}

	if err := h.RecoverStaleDeliveryJobs(context.Background(), 5*time.Minute); err != nil {
		t.Fatalf("RecoverStaleDeliveryJobs returned error: %v", err)
	}
	if dbtx.markedSucceededID != job.ID {
		t.Fatalf("marked-succeeded job id = %q, want %q", dbtx.markedSucceededID, job.ID)
	}
	if dbtx.createdRetryJob {
		t.Fatal("stale recovery created a retry for an already-published result (double-publish risk)")
	}
}
