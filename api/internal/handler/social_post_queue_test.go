package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
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
	"github.com/xiaoboyu/unipost-api/internal/platform"
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

func TestPostDeliveryJobResponseDerivesDeliveryPhase(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	created := pgtype.Timestamptz{Time: now.Add(-10 * time.Minute), Valid: true}
	firstClaimed := pgtype.Timestamptz{Time: now.Add(-8 * time.Minute), Valid: true}
	lastAttempt := pgtype.Timestamptz{Time: now.Add(-7 * time.Minute), Valid: true}
	platformStarted := pgtype.Timestamptz{Time: now.Add(-6 * time.Minute), Valid: true}
	finished := pgtype.Timestamptz{Time: now.Add(-4 * time.Minute), Valid: true}
	futureRetry := pgtype.Timestamptz{Time: now.Add(5 * time.Minute), Valid: true}
	dueRetry := pgtype.Timestamptz{Time: now.Add(-1 * time.Minute), Valid: true}

	base := db.PostDeliveryJob{
		ID:                 "job_1",
		PostID:             "post_1",
		SocialPostResultID: "result_1",
		SocialAccountID:    "acct_1",
		WorkspaceID:        "ws_1",
		Platform:           "twitter",
		Kind:               "dispatch",
		State:              "pending",
		CreatedAt:          created,
		UpdatedAt:          created,
	}

	tests := []struct {
		name   string
		mutate func(*db.PostDeliveryJob)
		want   string
	}{
		{name: "pending dispatch without claim is queued", want: "queued"},
		{
			name: "pending retry scheduled in future is waiting retry",
			mutate: func(job *db.PostDeliveryJob) {
				job.Kind = "retry"
				job.NextRunAt = futureRetry
			},
			want: "waiting_retry",
		},
		{
			name: "pending retry due now is queued retry",
			mutate: func(job *db.PostDeliveryJob) {
				job.Kind = "retry"
				job.NextRunAt = dueRetry
			},
			want: "queued_retry",
		},
		{
			name: "running before platform start is reserved",
			mutate: func(job *db.PostDeliveryJob) {
				job.State = "running"
				job.FirstClaimedAt = firstClaimed
				job.LastAttemptAt = lastAttempt
			},
			want: "reserved",
		},
		{
			name: "running after platform start is dispatching",
			mutate: func(job *db.PostDeliveryJob) {
				job.State = "running"
				job.FirstClaimedAt = firstClaimed
				job.LastAttemptAt = lastAttempt
				job.PlatformStartedAt = platformStarted
			},
			want: "dispatching",
		},
		{
			name: "retrying after platform start is retrying",
			mutate: func(job *db.PostDeliveryJob) {
				job.Kind = "retry"
				job.State = "retrying"
				job.FirstClaimedAt = firstClaimed
				job.LastAttemptAt = lastAttempt
				job.PlatformStartedAt = platformStarted
			},
			want: "retrying",
		},
		{
			name: "succeeded is published",
			mutate: func(job *db.PostDeliveryJob) {
				job.State = "succeeded"
				job.FinishedAt = finished
			},
			want: "published",
		},
		{
			name: "dead is failed",
			mutate: func(job *db.PostDeliveryJob) {
				job.State = "dead"
				job.FinishedAt = finished
			},
			want: "failed",
		},
		{
			name: "cancelled remains cancelled",
			mutate: func(job *db.PostDeliveryJob) {
				job.State = "cancelled"
				job.FinishedAt = finished
			},
			want: "cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := base
			if tt.mutate != nil {
				tt.mutate(&job)
			}

			resp := postDeliveryJobResponseFromRowAt(job, now)

			if resp.DeliveryPhase != tt.want {
				t.Fatalf("delivery phase = %q, want %q", resp.DeliveryPhase, tt.want)
			}
			if resp.QueuedAt != created.Time {
				t.Fatalf("queued_at = %s, want %s", resp.QueuedAt, created.Time)
			}
		})
	}
}

func TestPostDeliveryJobResponseCalculatesWaitDurations(t *testing.T) {
	created := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	firstClaimed := created.Add(30 * time.Second)
	lastAttempt := created.Add(2 * time.Minute)
	platformStarted := created.Add(3 * time.Minute)
	finished := created.Add(5 * time.Minute)
	job := db.PostDeliveryJob{
		ID:                 "job_1",
		PostID:             "post_1",
		SocialPostResultID: "result_1",
		SocialAccountID:    "acct_1",
		WorkspaceID:        "ws_1",
		Platform:           "twitter",
		Kind:               "dispatch",
		State:              "succeeded",
		CreatedAt:          pgtype.Timestamptz{Time: created, Valid: true},
		UpdatedAt:          pgtype.Timestamptz{Time: finished, Valid: true},
		FirstClaimedAt:     pgtype.Timestamptz{Time: firstClaimed, Valid: true},
		LastAttemptAt:      pgtype.Timestamptz{Time: lastAttempt, Valid: true},
		PlatformStartedAt:  pgtype.Timestamptz{Time: platformStarted, Valid: true},
		FinishedAt:         pgtype.Timestamptz{Time: finished, Valid: true},
	}

	resp := postDeliveryJobResponseFromRowAt(job, finished)

	if resp.QueueWaitMS == nil || *resp.QueueWaitMS != int64(lastAttempt.Sub(created)/time.Millisecond) {
		t.Fatalf("queue_wait_ms = %v, want %d", resp.QueueWaitMS, int64(lastAttempt.Sub(created)/time.Millisecond))
	}
	if resp.WorkerWaitMS == nil || *resp.WorkerWaitMS != int64(platformStarted.Sub(lastAttempt)/time.Millisecond) {
		t.Fatalf("worker_wait_ms = %v, want %d", resp.WorkerWaitMS, int64(platformStarted.Sub(lastAttempt)/time.Millisecond))
	}
	if resp.PlatformDurationMS == nil || *resp.PlatformDurationMS != int64(finished.Sub(platformStarted)/time.Millisecond) {
		t.Fatalf("platform_duration_ms = %v, want %d", resp.PlatformDurationMS, int64(finished.Sub(platformStarted)/time.Millisecond))
	}
}

func TestProcessPostDeliveryJobMarksPlatformStartedImmediatelyBeforeDispatch(t *testing.T) {
	source, err := os.ReadFile("social_post_queue.go")
	if err != nil {
		t.Fatalf("read social_post_queue.go: %v", err)
	}
	text := string(source)
	start := strings.Index(text, "func (h *SocialPostHandler) ProcessPostDeliveryJob")
	if start < 0 {
		t.Fatal("ProcessPostDeliveryJob not found")
	}
	end := strings.Index(text[start:], "func (h *SocialPostHandler) finalizeJobLoadFailure")
	if end < 0 {
		t.Fatal("finalizeJobLoadFailure boundary not found")
	}
	fn := text[start : start+end]

	resultGuard := strings.Index(fn, `if res.Status == "published"`)
	platformInput := strings.Index(fn, "platformPostInputAtIndex")
	markStarted := strings.Index(fn, "MarkPostDeliveryJobPlatformStarted")
	dispatch := strings.Index(fn, "h.publishOneContext(")
	for name, idx := range map[string]int{
		"result duplicate guard":     resultGuard,
		"platform input preparation": platformInput,
		"platform started marker":    markStarted,
		"publishOneContext dispatch": dispatch,
	} {
		if idx < 0 {
			t.Fatalf("%s not found in ProcessPostDeliveryJob", name)
		}
	}
	if markStarted < resultGuard {
		t.Fatal("platform_started_at must not be written before the result duplicate guard")
	}
	if markStarted < platformInput {
		t.Fatal("platform_started_at must not be written before platform input preparation")
	}
	if markStarted > dispatch {
		t.Fatal("platform_started_at must be written before publishOneContext dispatch")
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

func TestSanitizeDeliveryErrorTextRedactsTokens(t *testing.T) {
	got := sanitizeDeliveryErrorText(`instagram get user id failed: GET https://graph.instagram.com/v21.0/me?fields=id&access_token=secret-query-token Authorization: Bearer secret-bearer-token`)

	if strings.Contains(got, "secret-query-token") || strings.Contains(got, "secret-bearer-token") {
		t.Fatalf("sanitized error leaked token: %q", got)
	}
	for _, want := range []string{
		"access_token=[REDACTED]",
		"Authorization: Bearer [REDACTED]",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("sanitized error = %q, want to contain %q", got, want)
		}
	}
}

func TestSocialAccountDisconnectedForPublishTreatsReconnectRequiredAsDisconnected(t *testing.T) {
	if !socialAccountDisconnectedForPublish(db.SocialAccount{Status: "reconnect_required"}, true) {
		t.Fatal("reconnect_required account should be unavailable for publish")
	}
	if !socialAccountDisconnectedForPublish(db.SocialAccount{Status: " RECONNECT_REQUIRED "}, true) {
		t.Fatal("reconnect_required check should ignore case and surrounding spaces")
	}
	if !socialAccountDisconnectedForPublish(db.SocialAccount{
		Status:         "active",
		DisconnectedAt: pgtype.Timestamptz{Valid: true},
	}, true) {
		t.Fatal("disconnected_at account should remain unavailable for publish")
	}
	if socialAccountDisconnectedForPublish(db.SocialAccount{Status: "active"}, true) {
		t.Fatal("active connected account should be available for publish")
	}
	if socialAccountDisconnectedForPublish(db.SocialAccount{Status: "reconnect_required"}, false) {
		t.Fatal("missing account should stay missing so validation can report account ownership")
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
		f.markedState = args[0].(string)
		f.markedFailureStage = args[1].(pgtype.Text).String
		f.markedJobID = args[6].(string)
		updated := f.staleJob
		updated.State = f.markedState
		updated.FailureStage = args[1].(pgtype.Text)
		updated.ErrorCode = args[2].(pgtype.Text)
		updated.PlatformErrorCode = args[3].(pgtype.Text)
		updated.LastError = args[4].(pgtype.Text)
		updated.NextRunAt = args[5].(pgtype.Timestamptz)
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
		job.LeaseExpiresAt,
		job.LeaseOwner,
		job.FirstClaimedAt,
		job.PlatformStartedAt,
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
	platformStartedID  string
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

func (f *guardedDeliveryDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetPostDeliveryJobByIDAndWorkspace"):
		f.getJobCalls++
		return postDeliveryJobScanRow(f.current)
	case strings.Contains(query, "-- name: MarkPostDeliveryJobPlatformStarted"):
		f.platformStartedID = args[0].(string)
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
	if dbtx.platformStartedID != "" {
		t.Fatalf("stale job wrote platform_started_at for %q", dbtx.platformStartedID)
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

// retryAfterPublishedDB models the residual race: the retry job is still
// "retrying" (so it clears the job-state guard), but a concurrent original
// attempt already published the result while the retry waited.
type retryAfterPublishedDB struct {
	job                db.PostDeliveryJob
	result             db.SocialPostResult
	markedSucceededID  string
	reachedPublishPath bool
}

func (f *retryAfterPublishedDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	f.reachedPublishPath = true
	return pgconn.CommandTag{}, errors.New("unexpected Exec")
}

func (f *retryAfterPublishedDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	f.reachedPublishPath = true
	return nil, errors.New("unexpected Query")
}

func (f *retryAfterPublishedDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetPostDeliveryJobByIDAndWorkspace"):
		return postDeliveryJobScanRow(f.job) // still active -> passes job-state guard
	case strings.Contains(query, "-- name: GetSocialPostByID"):
		return socialPostScanRow(db.SocialPost{
			ID:          f.job.PostID,
			WorkspaceID: f.job.WorkspaceID,
			Status:      "publishing",
			CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
		})
	case strings.Contains(query, "-- name: GetSocialPostResultByIDAndPost"):
		return socialPostResultScanRow(f.result) // already published
	case strings.Contains(query, "-- name: MarkPostDeliveryJobSucceeded"):
		f.markedSucceededID = args[0].(string)
		succeeded := f.job
		succeeded.State = "succeeded"
		return postDeliveryJobScanRow(succeeded)
	default:
		f.reachedPublishPath = true
		return scanRow{err: errors.New("advanced toward publish path: " + query)}
	}
}

func TestProcessPostDeliveryJobSkipsWhenResultAlreadyPublished(t *testing.T) {
	job := baseDeliveryJob()
	job.Kind = "retry"
	job.State = "retrying"
	dbtx := &retryAfterPublishedDB{
		job: job,
		result: db.SocialPostResult{
			ID:              job.SocialPostResultID,
			PostID:          job.PostID,
			SocialAccountID: job.SocialAccountID,
			Status:          "published",
		},
	}
	h := &SocialPostHandler{queries: db.New(dbtx)}

	if err := h.ProcessPostDeliveryJob(context.Background(), job); err != nil {
		t.Fatalf("ProcessPostDeliveryJob returned error: %v", err)
	}
	if dbtx.markedSucceededID != job.ID {
		t.Fatalf("marked-succeeded job id = %q, want %q", dbtx.markedSucceededID, job.ID)
	}
	if dbtx.reachedPublishPath {
		t.Fatal("retry reached the publish path for an already-published result (double-publish risk)")
	}
}

// publishTokenDB captures SetSocialPostResultPublishToken persistence.
type publishTokenDB struct {
	setID    string
	setToken string
}

func (f *publishTokenDB) Exec(_ context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	if strings.Contains(query, "-- name: SetSocialPostResultPublishToken") {
		for _, a := range args {
			switch v := a.(type) {
			case string:
				f.setID = v
			case pgtype.Text:
				f.setToken = v.String
			}
		}
		return pgconn.CommandTag{}, nil
	}
	return pgconn.CommandTag{}, errors.New("unexpected Exec: " + query)
}
func (f *publishTokenDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query")
}
func (f *publishTokenDB) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	return scanRow{err: errors.New("unexpected QueryRow")}
}

func TestAttachPublishTokenResumeInjectsResumeAndPersist(t *testing.T) {
	fake := &publishTokenDB{}
	h := &SocialPostHandler{queries: db.New(fake)}
	pp := platform.PlatformPostInput{AccountID: "acct_ig"}
	res := db.SocialPostResult{ID: "res_1", PublishToken: pgtype.Text{String: "creation_123", Valid: true}}

	h.attachPublishTokenResume(context.Background(), &pp, res)

	if got := pp.PlatformOptions[platform.OptResumePublishToken]; got != "creation_123" {
		t.Fatalf("resume token = %v, want creation_123", got)
	}
	fn, ok := pp.PlatformOptions[platform.OptOnPublishToken].(func(string))
	if !ok {
		t.Fatal("persist callback not injected into opts")
	}
	fn("new_token_456")
	if fake.setID != "res_1" || fake.setToken != "new_token_456" {
		t.Fatalf("persist callback stored id=%q token=%q, want res_1/new_token_456", fake.setID, fake.setToken)
	}
}

func TestAttachPublishTokenResumeOmitsResumeWhenNoPriorToken(t *testing.T) {
	h := &SocialPostHandler{queries: db.New(&publishTokenDB{})}
	pp := platform.PlatformPostInput{AccountID: "acct_ig"}
	res := db.SocialPostResult{ID: "res_1"} // no prior token

	h.attachPublishTokenResume(context.Background(), &pp, res)

	if _, present := pp.PlatformOptions[platform.OptResumePublishToken]; present {
		t.Fatal("resume token must not be injected when no prior token exists")
	}
	if _, ok := pp.PlatformOptions[platform.OptOnPublishToken].(func(string)); !ok {
		t.Fatal("persist callback should still be injected on a first attempt")
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
