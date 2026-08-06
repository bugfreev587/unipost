package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	appcrypto "github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
	"github.com/xiaoboyu/unipost-api/internal/integrationlogs"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/postfailuredebug"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
	"github.com/xiaoboyu/unipost-api/internal/quota"
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

func TestSchedulerDelegatesClaimAndEnqueueToOneHandlerTransaction(t *testing.T) {
	source, err := os.ReadFile("../worker/scheduler.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if strings.Contains(text, ".ClaimScheduledPost(") {
		t.Fatal("scheduler worker must not commit the scheduled claim before enqueue")
	}
	if !strings.Contains(text, ".ClaimAndEnqueueScheduledPost(") {
		t.Fatal("scheduler worker must delegate claim and enqueue to the handler transaction")
	}
}

func TestDraftClaimAndRestrictedEnqueueShareOneTransaction(t *testing.T) {
	source, err := os.ReadFile("social_posts_drafts.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "func (h *SocialPostHandler) PublishDraft")
	end := strings.Index(text[start:], "func (h *SocialPostHandler) rollbackToDraft")
	if start < 0 || end < 0 {
		t.Fatal("PublishDraft boundaries not found")
	}
	fn := text[start : start+end]
	tx := strings.Index(fn, ".WithTransaction(")
	claim := strings.Index(fn, ".ClaimDraftForPublish(")
	enqueue := strings.Index(fn, ".enqueueExistingPostDeliveriesInTransaction(")
	if tx < 0 || claim < 0 || enqueue < 0 || tx > claim || claim > enqueue {
		t.Fatalf("draft transaction/claim/enqueue order = %d/%d/%d", tx, claim, enqueue)
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

type panickingPostFailureDebugSink struct{}

func (panickingPostFailureDebugSink) Enqueue(postfailuredebug.Detail) {
	panic("forced observability sink failure")
}

func TestPostFailureDebugSinkPanicIsContained(t *testing.T) {
	h := (&SocialPostHandler{}).SetPostFailureDebugSink(panickingPostFailureDebugSink{})
	didReturn := false
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("observability panic escaped into publishing: %v", recovered)
			}
		}()
		h.recordPostFailureDebug("result-1", "workspace-1", "safe diagnostic")
		didReturn = true
	}()
	if !didReturn {
		t.Fatal("publishing caller did not continue after observability panic")
	}
}

func TestPublishingDiagnosticsStayOutsideBusinessWritesAndTransactions(t *testing.T) {
	immediateSource, err := os.ReadFile("social_posts.go")
	if err != nil {
		t.Fatal(err)
	}
	immediate := string(immediateSource)
	createAt := strings.Index(immediate, "h.queries.CreateSocialPostResult")
	createEnd := strings.Index(immediate[createAt:], "if dbErr != nil")
	if createAt < 0 || createEnd < 0 {
		t.Fatal("immediate result persistence boundaries not found")
	}
	createBlock := immediate[createAt : createAt+createEnd]
	if !strings.Contains(createBlock, "DebugCurl:       pgtype.Text{}") {
		t.Fatal("immediate business insert must always receive an empty legacy debug value")
	}
	if strings.Contains(createBlock, "oc.debugCurl") {
		t.Fatal("immediate business insert still depends on diagnostic text")
	}

	queueSource, err := os.ReadFile("social_post_queue.go")
	if err != nil {
		t.Fatal(err)
	}
	queue := string(queueSource)
	start := strings.Index(queue, "func (h *SocialPostHandler) handleJobDispatchFailure")
	endOffset := strings.Index(queue[start:], "func inferDispatchFailureStage")
	if start < 0 || endOffset < 0 {
		t.Fatal("queued failure handler boundaries not found")
	}
	fn := queue[start : start+endOffset]
	resultUpdateAt := strings.Index(fn, "resultUpdate := db.UpdateSocialPostResultAfterRetryParams")
	transactionAt := strings.Index(fn, "h.queries.WithTransaction")
	debugEnqueueAt := strings.Index(fn, "h.recordPostFailureDebug")
	failureAppliedAt := strings.Index(fn, "if !failureApplied")
	if resultUpdateAt < 0 || transactionAt < 0 || failureAppliedAt < 0 || debugEnqueueAt < failureAppliedAt {
		t.Fatal("queued diagnostic must be enqueued only after the successful business transaction")
	}
	resultUpdateBlock := fn[resultUpdateAt:transactionAt]
	if !strings.Contains(resultUpdateBlock, "DebugCurl:       pgtype.Text{}") || strings.Contains(resultUpdateBlock, "oc.debugCurl") {
		t.Fatal("queued business transaction still depends on diagnostic text")
	}
	if strings.Contains(fn, `"debug_curl": oc.debugCurl`) {
		t.Fatal("queued integration log still duplicates the full diagnostic body")
	}
}

func TestMarkDeliveryJobSucceededParamsUsesCapturedFinishedAt(t *testing.T) {
	finishedAt := time.Date(2026, 7, 10, 18, 30, 0, 125000000, time.UTC)
	job := db.PostDeliveryJob{
		ID:            "job_1",
		LeaseOwner:    pgtype.Text{String: "worker_1", Valid: true},
		LastAttemptAt: pgtype.Timestamptz{Time: finishedAt.Add(-time.Minute), Valid: true},
	}

	params := markDeliveryJobSucceededParams(job, finishedAt)
	if !params.FinishedAt.Valid || !params.FinishedAt.Time.Equal(finishedAt) {
		t.Fatalf("finished_at = %#v, want %s", params.FinishedAt, finishedAt)
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

	resultGuard := strings.Index(fn, `if providerTerminalOrAccepted(res, job.Platform)`)
	platformInput := strings.Index(fn, "platformPostInputAtIndex")
	policyRecheck := strings.Index(fn, "evaluateDeliveryPublishingRestriction")
	markStarted := strings.Index(fn, "MarkPostDeliveryJobPlatformStarted")
	dispatch := strings.Index(fn, "h.publishOneContext(")
	for name, idx := range map[string]int{
		"result duplicate guard":     resultGuard,
		"platform input preparation": platformInput,
		"publishing policy recheck":  policyRecheck,
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
	if policyRecheck < platformInput || policyRecheck > markStarted {
		t.Fatal("publishing policy must be rechecked after input preparation and before platform_started_at")
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

type typedFailureStageTestError struct {
	message     string
	stage       string
	errorCode   string
	isRetriable bool
	provider    map[string]any
}

func (e typedFailureStageTestError) Error() string        { return e.message }
func (e typedFailureStageTestError) FailureStage() string { return e.stage }
func (e typedFailureStageTestError) FailureContractFields() map[string]any {
	return map[string]any{
		"error_code":    e.errorCode,
		"failure_stage": e.stage,
		"is_retriable":  e.isRetriable,
	}
}
func (e typedFailureStageTestError) ProviderErrorFields() map[string]any { return e.provider }

func TestInferDispatchFailureStagePrefersTypedStage(t *testing.T) {
	err := typedFailureStageTestError{message: "create_tweet failed", stage: "destination_preflight"}
	if got := inferDispatchFailureStage(err); got != "destination_preflight" {
		t.Fatalf("stage = %q, want destination_preflight", got)
	}
}

func TestInferDispatchFailureStageFindsWrappedTypedStage(t *testing.T) {
	err := fmt.Errorf("publish failed: %w", typedFailureStageTestError{message: "provider rejected request", stage: "destination_preflight"})
	if got := inferDispatchFailureStage(err); got != "destination_preflight" {
		t.Fatalf("stage = %q, want destination_preflight from wrapped carrier", got)
	}
}

func TestInferDispatchFailureStageKeepsLegacyFallback(t *testing.T) {
	if got := inferDispatchFailureStage(errors.New("upload_media_append failed")); got != "upload_media_append" {
		t.Fatalf("stage = %q, want upload_media_append", got)
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

func terminalPostEventOutboxScanRow(id, postID, workspaceID, status, version, event string) scanRow {
	return scanRow{values: []any{
		id,
		int64(1),
		postID,
		workspaceID,
		status,
		version,
		event,
		[]byte(`{"id":"` + postID + `"}`),
		pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}}
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
		post.QuotaHoldReason,
		post.QuotaHoldAt,
		post.QuotaHoldOriginalScheduledAt,
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

func (f *retryAfterPublishedDB) Begin(context.Context) (pgx.Tx, error) {
	return &queueDBTXTestTx{db: f}, nil
}

func (f *retryAfterPublishedDB) Exec(_ context.Context, query string, _ ...interface{}) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(query, "-- name: LockPostDeliveryJobFinalization"):
		return pgconn.CommandTag{}, nil
	case strings.Contains(query, "-- name: UpdateSocialPostStatus"),
		strings.Contains(query, "-- name: UpdateSocialPostErrorMetadata"),
		strings.Contains(query, "-- name: DeleteMediaPostUsagesForPost"),
		strings.Contains(query, "-- name: CreateTerminalPostEventDelivery"):
		return pgconn.NewCommandTag("UPDATE 1"), nil
	default:
		f.reachedPublishPath = true
		return pgconn.CommandTag{}, errors.New("unexpected Exec")
	}
}

func (f *retryAfterPublishedDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	switch {
	case strings.Contains(query, "-- name: ListSocialPostResultsByPost"):
		return &queueRows{values: [][]any{socialPostResultScanRow(f.result).values}}, nil
	case strings.Contains(query, "-- name: ListPostDeliveryJobsByPost"):
		return &queueRows{values: [][]any{postDeliveryJobScanRow(f.job).values}}, nil
	default:
		f.reachedPublishPath = true
		return nil, errors.New("unexpected Query")
	}
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
		finishedAt := args[0].(pgtype.Timestamptz)
		f.markedSucceededID = args[1].(string)
		succeeded := f.job
		succeeded.State = "succeeded"
		succeeded.FinishedAt = finishedAt
		f.job = succeeded
		return postDeliveryJobScanRow(succeeded)
	case strings.Contains(query, "-- name: GetSocialPostProjectionVersion"):
		return scanRow{values: []any{"published-version"}}
	case strings.Contains(query, "-- name: ElectWorkspaceFirstPostPublished"):
		return scanRow{values: []any{f.job.WorkspaceID}}
	case strings.Contains(query, "-- name: CreatePostStatusTransitionEvent"):
		return terminalPostEventOutboxScanRow("post-event-1", f.job.PostID, f.job.WorkspaceID, "published", "published-version", "post.published")
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
	h := &SocialPostHandler{queries: db.New(dbtx), bus: events.NoopBus{}}

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

type staleLoadFailureOwnerDB struct {
	resultUpdateCalls int
	parentLockCalls   int
	result            db.SocialPostResult
}

func (f *staleLoadFailureOwnerDB) Begin(context.Context) (pgx.Tx, error) {
	return &queueDBTXTestTx{db: f}, nil
}

func (f *staleLoadFailureOwnerDB) Exec(_ context.Context, query string, _ ...interface{}) (pgconn.CommandTag, error) {
	if strings.Contains(query, "-- name: LockPostDeliveryJobFinalization") {
		f.parentLockCalls++
		return pgconn.CommandTag{}, nil
	}
	return pgconn.CommandTag{}, fmt.Errorf("unexpected Exec: %s", query)
}

func (f *staleLoadFailureOwnerDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query")
}

func (f *staleLoadFailureOwnerDB) QueryRow(_ context.Context, query string, _ ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetSocialPostResultByIDAndPost"):
		return socialPostResultScanRow(f.result)
	case strings.Contains(query, "-- name: MarkPostDeliveryJobFailed"):
		return scanRow{err: pgx.ErrNoRows}
	case strings.Contains(query, "-- name: UpdateSocialPostResultAfterRetry"):
		f.resultUpdateCalls++
		return scanRow{err: errors.New("stale owner must not update the shared result")}
	default:
		return scanRow{err: fmt.Errorf("unexpected QueryRow: %s", query)}
	}
}

func TestFinalizeJobLoadFailureStaleOwnerDoesNotMutateOrRefresh(t *testing.T) {
	dbtx := &staleLoadFailureOwnerDB{}
	job := baseDeliveryJob()
	job.LeaseOwner = pgtype.Text{String: "stale_owner", Valid: true}
	job.LastAttemptAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	post := db.SocialPost{ID: job.PostID, WorkspaceID: job.WorkspaceID, Status: "publishing"}
	result := db.SocialPostResult{
		ID: job.SocialPostResultID, PostID: job.PostID,
		SocialAccountID: job.SocialAccountID, Status: "processing",
	}
	dbtx.result = result
	h := NewSocialPostHandler(db.New(dbtx), nil, nil, events.NoopBus{}, nil, nil, nil)

	if err := h.finalizeJobLoadFailure(context.Background(), job, result, post, errors.New("invalid platform input")); err != nil {
		t.Fatalf("finalizeJobLoadFailure stale owner: %v", err)
	}
	if dbtx.resultUpdateCalls != 0 {
		t.Fatalf("stale owner result updates = %d, want 0", dbtx.resultUpdateCalls)
	}
	if dbtx.parentLockCalls != 1 {
		t.Fatalf("stale owner terminal parent finalization locks = %d, want 1", dbtx.parentLockCalls)
	}
}

// publishTokenDB captures SetSocialPostResultPublishToken persistence.
type publishTokenDB struct {
	setID    string
	setToken string
	setJobID string
}

func (f *publishTokenDB) Exec(_ context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	if strings.Contains(query, "-- name: SetSocialPostResultPublishToken") {
		f.setToken = args[0].(pgtype.Text).String
		f.setID = args[1].(string)
		f.setJobID = args[2].(string)
		return pgconn.NewCommandTag("UPDATE 1"), nil
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
	job := baseDeliveryJob()
	job.ID = "job_token_1"

	h.attachPublishTokenResume(context.Background(), &pp, res, job)

	if got := pp.PlatformOptions[platform.OptResumePublishToken]; got != "creation_123" {
		t.Fatalf("resume token = %v, want creation_123", got)
	}
	fn, ok := pp.PlatformOptions[platform.OptOnPublishToken].(func(string) error)
	if !ok {
		t.Fatal("persist callback not injected into opts")
	}
	if err := fn("new_token_456"); err != nil {
		t.Fatalf("persist callback: %v", err)
	}
	if fake.setID != "res_1" || fake.setToken != "new_token_456" || fake.setJobID != job.ID {
		t.Fatalf("persist callback stored id=%q token=%q job=%q, want res_1/new_token_456/%s",
			fake.setID, fake.setToken, fake.setJobID, job.ID)
	}
}

func TestAttachPublishTokenResumeOmitsResumeWhenNoPriorToken(t *testing.T) {
	h := &SocialPostHandler{queries: db.New(&publishTokenDB{})}
	pp := platform.PlatformPostInput{AccountID: "acct_ig"}
	res := db.SocialPostResult{ID: "res_1"} // no prior token

	h.attachPublishTokenResume(context.Background(), &pp, res, baseDeliveryJob())

	if _, present := pp.PlatformOptions[platform.OptResumePublishToken]; present {
		t.Fatal("resume token must not be injected when no prior token exists")
	}
	if _, ok := pp.PlatformOptions[platform.OptOnPublishToken].(func(string) error); !ok {
		t.Fatal("persist callback should still be injected on a first attempt")
	}
}

func TestPersistedPublishTokenBypassesNewPublishingRestriction(t *testing.T) {
	started := db.SocialPostResult{
		ID:           "res_started",
		Status:       "processing",
		PublishToken: pgtype.Text{String: "publish_123", Valid: true},
	}
	if shouldEvaluateDeliveryPublishingRestriction(started) {
		t.Fatal("already-started delivery must resume from its persisted token instead of being finalized as restricted")
	}

	notStarted := db.SocialPostResult{ID: "res_new", Status: "pending"}
	if !shouldEvaluateDeliveryPublishingRestriction(notStarted) {
		t.Fatal("delivery without a persisted platform token must still recheck the publishing restriction")
	}
}

type restrictedFinalizeLeaseDB struct {
	job                 db.PostDeliveryJob
	result              db.SocialPostResult
	planID              string
	lostLease           bool
	convergedPublished  bool
	resultUpdateCalls   int
	failureDetailCalls  int
	failureHistoryCalls int
	parentRefreshCalls  int
	retentionCalls      int
	retentionUpserts    []db.UpsertMediaPostUsageParams
	afterFinalize       func(context.Context)
	afterListResults    func(context.Context)
	listResultsErr      error
}

func (f *restrictedFinalizeLeaseDB) Begin(context.Context) (pgx.Tx, error) {
	snapshot := *f
	snapshot.retentionUpserts = append([]db.UpsertMediaPostUsageParams(nil), f.retentionUpserts...)
	return &restrictedFinalizeLeaseTx{store: f, snapshot: snapshot}, nil
}

type restrictedFinalizeLeaseTx struct {
	store    *restrictedFinalizeLeaseDB
	snapshot restrictedFinalizeLeaseDB
	done     bool
}

func (f *restrictedFinalizeLeaseTx) Begin(context.Context) (pgx.Tx, error) { return f, nil }
func (f *restrictedFinalizeLeaseTx) Commit(context.Context) error {
	f.done = true
	return nil
}
func (f *restrictedFinalizeLeaseTx) Rollback(context.Context) error {
	if !f.done {
		*f.store = f.snapshot
		f.done = true
	}
	return nil
}
func (*restrictedFinalizeLeaseTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("unexpected copy")
}
func (*restrictedFinalizeLeaseTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}
func (*restrictedFinalizeLeaseTx) LargeObjects() pgx.LargeObjects { return pgx.LargeObjects{} }
func (*restrictedFinalizeLeaseTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("unexpected prepare")
}
func (f *restrictedFinalizeLeaseTx) Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	return f.store.Exec(ctx, query, args...)
}
func (f *restrictedFinalizeLeaseTx) Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	return f.store.Query(ctx, query, args...)
}
func (f *restrictedFinalizeLeaseTx) QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	return f.store.QueryRow(ctx, query, args...)
}
func (*restrictedFinalizeLeaseTx) Conn() *pgx.Conn { return nil }

func (f *restrictedFinalizeLeaseDB) Exec(_ context.Context, query string, _ ...interface{}) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(query, "-- name: LockPostDeliveryJobFinalization"):
		return pgconn.CommandTag{}, nil
	case strings.Contains(query, "-- name: UpdateSocialPostResultFailureDetails"):
		f.failureDetailCalls++
		return pgconn.CommandTag{}, nil
	case strings.Contains(query, "-- name: DeleteMediaPostUsagesForPost"):
		f.retentionCalls++
		return pgconn.CommandTag{}, nil
	default:
		return pgconn.CommandTag{}, errors.New("unexpected Exec: " + query)
	}
}

func (f *restrictedFinalizeLeaseDB) Query(ctx context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	switch {
	case strings.Contains(query, "-- name: ListSocialPostResultsByPost"):
		f.parentRefreshCalls++
		if f.listResultsErr != nil {
			return nil, f.listResultsErr
		}
		rows := &queueRows{values: [][]any{socialPostResultScanRow(f.result).values}}
		if f.afterListResults != nil {
			f.afterListResults(ctx)
		}
		return rows, nil
	case strings.Contains(query, "-- name: ListPostDeliveryJobsByPost"):
		f.parentRefreshCalls++
		return &queueRows{}, nil
	default:
		return nil, errors.New("unexpected Query: " + query)
	}
}

func (f *restrictedFinalizeLeaseDB) QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: FinalizeRestrictedPostDeliveryJob"):
		if f.lostLease {
			return scanRow{err: pgx.ErrNoRows}
		}
		if f.convergedPublished {
			updated := f.job
			updated.State = "succeeded"
			updated.FailureStage = pgtype.Text{}
			updated.ErrorCode = pgtype.Text{}
			updated.PlatformErrorCode = pgtype.Text{}
			updated.LastError = pgtype.Text{}
			updated.NextRunAt = pgtype.Timestamptz{}
			f.job = updated
			if f.afterFinalize != nil {
				f.afterFinalize(ctx)
			}
			row := postDeliveryJobScanRow(updated)
			row.values = append(row.values, "publishing", "publishing", false)
			return row
		}
		f.resultUpdateCalls++
		updated := f.job
		updated.State = "dead"
		updated.FailureStage = args[4].(pgtype.Text)
		updated.ErrorCode = args[5].(pgtype.Text)
		updated.PlatformErrorCode = pgtype.Text{}
		updated.LastError = pgtype.Text{String: args[6].(string), Valid: true}
		updated.NextRunAt = pgtype.Timestamptz{}
		f.job = updated
		f.result.Status = "failed"
		f.result.ExternalID = pgtype.Text{}
		f.result.ErrorMessage = pgtype.Text{String: args[6].(string), Valid: true}
		f.result.PublishedAt = pgtype.Timestamptz{}
		f.result.Url = pgtype.Text{}
		f.result.DebugCurl = pgtype.Text{}
		f.result.PublishToken = pgtype.Text{}
		f.result.ErrorCode = args[5].(pgtype.Text)
		f.result.FailureStage = args[4].(pgtype.Text)
		f.result.PlatformErrorCode = pgtype.Text{}
		f.result.IsRetriable = pgtype.Bool{Bool: false, Valid: true}
		f.result.NextAction = args[7].(pgtype.Text)
		f.result.ErrorSource = args[8].(pgtype.Text)
		f.result.ErrorTemporality = args[9].(pgtype.Text)
		f.result.ProviderError = nil
		f.result.XCreditsCounted = 0
		f.result.XCreditOperation = pgtype.Text{}
		f.result.XCreditCatalogVersion = pgtype.Text{}
		f.result.XCreditBillingMode = pgtype.Text{}
		for _, mediaID := range args[3].([]string) {
			f.retentionUpserts = append(f.retentionUpserts, db.UpsertMediaPostUsageParams{
				MediaID:         mediaID,
				WorkspaceID:     f.job.WorkspaceID,
				PostStatus:      "failed",
				CleanupAfterAt:  args[10].(pgtype.Timestamptz),
				RetentionReason: "publishing_restriction",
				PostID:          f.job.PostID,
			})
		}
		if f.afterFinalize != nil {
			f.afterFinalize(ctx)
		}
		row := postDeliveryJobScanRow(updated)
		row.values = append(row.values, "publishing", "failed", false)
		return row
	case strings.Contains(query, "-- name: UpdateSocialPostResultAfterRetry"):
		f.resultUpdateCalls++
		f.result.Status = args[1].(string)
		f.result.ExternalID = args[2].(pgtype.Text)
		f.result.ErrorMessage = args[3].(pgtype.Text)
		f.result.PublishedAt = args[4].(pgtype.Timestamptz)
		f.result.Url = args[5].(pgtype.Text)
		f.result.DebugCurl = args[6].(pgtype.Text)
		return socialPostResultScanRow(f.result)
	case strings.Contains(query, "-- name: MarkPostDeliveryJobFailed"):
		if f.lostLease {
			return scanRow{err: pgx.ErrNoRows}
		}
		updated := f.job
		updated.State = "dead"
		return postDeliveryJobScanRow(updated)
	case strings.Contains(query, "-- name: CreatePostFailure"):
		f.failureHistoryCalls++
		return scanRow{err: errors.New("post failure scan not needed")}
	case strings.Contains(query, "-- name: GetSocialPostByID"):
		f.parentRefreshCalls++
		return scanRow{err: pgx.ErrNoRows}
	case strings.Contains(query, "-- name: GetSubscriptionByWorkspace"):
		return subscriptionScanRow(f.planID)
	case strings.Contains(query, "-- name: UpsertMediaPostUsage"):
		f.retentionUpserts = append(f.retentionUpserts, db.UpsertMediaPostUsageParams{
			MediaID:         args[0].(string),
			WorkspaceID:     args[1].(string),
			PostStatus:      args[2].(string),
			CleanupAfterAt:  args[3].(pgtype.Timestamptz),
			RetentionReason: args[4].(string),
			PostID:          args[5].(string),
		})
		return scanRow{values: []any{true}}
	default:
		return scanRow{err: errors.New("unexpected QueryRow: " + query)}
	}
}

type restrictedFinalizeIntegrationLogStore struct {
	calls  int
	params []db.InsertIntegrationLogParams
}

func (s *restrictedFinalizeIntegrationLogStore) InsertIntegrationLog(_ context.Context, params db.InsertIntegrationLogParams) (db.IntegrationLog, error) {
	s.calls++
	s.params = append(s.params, params)
	return db.IntegrationLog{}, nil
}

func runIntegrationLogger(t *testing.T, store *restrictedFinalizeIntegrationLogStore) (*integrationlogs.Logger, func()) {
	t.Helper()
	logger := integrationlogs.NewLogger(store, nil)
	loggerContext, stopLogger := context.WithCancel(context.Background())
	loggerStopped := make(chan struct{})
	go func() {
		logger.Start(loggerContext)
		close(loggerStopped)
	}()
	var once sync.Once
	return logger, func() {
		once.Do(func() {
			stopLogger()
			<-loggerStopped
		})
	}
}

func queuedLogMetadata(t *testing.T, store *restrictedFinalizeIntegrationLogStore) map[string]any {
	t.Helper()
	for _, params := range store.params {
		if params.Action != integrationlogs.ActionPostPublishQueued {
			continue
		}
		var metadata map[string]any
		if err := json.Unmarshal(params.Metadata, &metadata); err != nil {
			t.Fatalf("decode queued log metadata: %v", err)
		}
		return metadata
	}
	t.Fatal("queued publishing integration log not found")
	return nil
}

func TestQueueImmediatePostLogsEveryMixedDeliveryResult(t *testing.T) {
	h, _, _, parsed, accounts := newPolicySnapshotHarness(t, "publishing")
	logStore := &restrictedFinalizeIntegrationLogStore{}
	logger, stopLogger := runIntegrationLogger(t, logStore)
	defer stopLogger()
	h.ilog = logger

	response, err := h.queueImmediatePost(context.Background(), "ws_1", parsed, accounts, map[string]publishingrestrictions.Decision{
		"tk_1": {Restricted: true, Platform: "tiktok", CycleID: "cycle_1"},
	})
	if err != nil {
		t.Fatalf("queueImmediatePost: %v", err)
	}
	if len(response.Results) != 2 || response.ActiveJobCount != 1 {
		t.Fatalf("response results/jobs = %d/%d, want 2/1", len(response.Results), response.ActiveJobCount)
	}
	stopLogger()
	metadata := queuedLogMetadata(t, logStore)
	if metadata["result_count"] != float64(2) || metadata["queued_jobs"] != float64(1) {
		t.Fatalf("queued metadata = %#v, want result_count=2 queued_jobs=1", metadata)
	}
}

func TestQueueImmediatePostLogsEveryLocallyFailedResult(t *testing.T) {
	h, _, _, parsed, accounts := newPolicySnapshotHarness(t, "publishing")
	accounts["tk_1"] = platform.ValidateAccount{Platform: "tiktok", Disconnected: true}
	accounts["ig_1"] = platform.ValidateAccount{Platform: "instagram", Disconnected: true}
	logStore := &restrictedFinalizeIntegrationLogStore{}
	logger, stopLogger := runIntegrationLogger(t, logStore)
	defer stopLogger()
	h.ilog = logger

	response, err := h.queueImmediatePost(context.Background(), "ws_1", parsed, accounts, nil)
	if err != nil {
		t.Fatalf("queueImmediatePost: %v", err)
	}
	if len(response.Results) != 2 || response.ActiveJobCount != 0 {
		t.Fatalf("response results/jobs = %d/%d, want 2/0", len(response.Results), response.ActiveJobCount)
	}
	stopLogger()
	metadata := queuedLogMetadata(t, logStore)
	if metadata["result_count"] != float64(2) || metadata["queued_jobs"] != float64(0) {
		t.Fatalf("queued metadata = %#v, want result_count=2 queued_jobs=0", metadata)
	}
}

func TestPinterestDispatchObservabilityUsesAllowlistedSafeFields(t *testing.T) {
	logStore := &restrictedFinalizeIntegrationLogStore{}
	logger, stopLogger := runIntegrationLogger(t, logStore)
	h := NewSocialPostHandler(nil, nil, nil, nil, nil, nil, logger)
	post := db.SocialPost{ID: "post_pin", WorkspaceID: "ws_pin"}
	result := db.SocialPostResult{ID: "result_pin", SocialAccountID: "account_pin"}
	job := db.PostDeliveryJob{ID: "job_pin", Platform: "pinterest"}

	h.logPlatformDispatchEvents(context.Background(), post, result, job, []platform.DispatchEvent{
		{
			Name:             integrationlogs.ActionPinterestDestinationPreflightFailed,
			Status:           "failed",
			Environment:      "production",
			BoardFingerprint: "b4f9206f2d2c145a",
			HTTPStatus:       http.StatusForbidden,
			ProviderCode:     "29",
			ErrorCode:        "target_not_found",
			Reason:           "board_not_writable",
			FailureStage:     "destination_preflight",
			Retriable:        false,
			Duration:         42 * time.Millisecond,
		},
		{
			Name:             integrationlogs.ActionPinterestMediaPreflightFailed,
			Status:           "failed",
			Environment:      "production",
			BoardFingerprint: "b4f9206f2d2c145a",
			ErrorCode:        "media_error",
			Reason:           "customer_media_unreachable",
			FailureStage:     "media_preflight",
			CustomerInput:    true,
			Duration:         17 * time.Millisecond,
		},
		{
			Name:             integrationlogs.ActionPinterestMediaStaged,
			Status:           "succeeded",
			Environment:      "production",
			BoardFingerprint: "b4f9206f2d2c145a",
			FailureStage:     "media_preflight",
			MediaType:        "image/jpeg",
			MediaSizeBytes:   4096,
			Duration:         23 * time.Millisecond,
		},
		{Name: "not_allowlisted", Status: "failed", Reason: "access_token=secret https://api.pinterest.com/v5/boards/123"},
	})
	stopLogger()

	if len(logStore.params) != 3 {
		t.Fatalf("integration log count = %d, want 3 allowlisted events", len(logStore.params))
	}
	got := logStore.params[0]
	if got.Action != integrationlogs.ActionPinterestDestinationPreflightFailed || got.Message != integrationlogs.ActionPinterestDestinationPreflightFailed {
		t.Fatalf("action/message = %q/%q, want stable preflight failure action", got.Action, got.Message)
	}
	if !got.RemoteStatusCode.Valid || got.RemoteStatusCode.Int32 != http.StatusForbidden {
		t.Fatalf("remote status = %#v, want 403", got.RemoteStatusCode)
	}
	var metadata map[string]any
	if err := json.Unmarshal(got.Metadata, &metadata); err != nil {
		t.Fatalf("decode event metadata: %v", err)
	}
	for key, want := range map[string]any{
		"job_id":                "job_pin",
		"result_id":             "result_pin",
		"board_fingerprint":     "b4f9206f2d2c145a",
		"pinterest_environment": "production",
		"provider_code":         "29",
		"normalized_reason":     "board_not_writable",
		"failure_stage":         "destination_preflight",
		"retriable":             false,
	} {
		if metadata[key] != want {
			t.Errorf("metadata[%q] = %#v, want %#v", key, metadata[key], want)
		}
	}
	encoded := string(got.Metadata)
	for _, forbidden := range []string{"access_token", "secret", "api.pinterest.com", "/boards/123"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("safe event metadata contains %q: %s", forbidden, encoded)
		}
	}

	mediaFailure := logStore.params[1]
	if mediaFailure.Action != integrationlogs.ActionPinterestMediaPreflightFailed || mediaFailure.ErrorCode.String != "media_error" {
		t.Fatalf("media failure action/error = %q/%#v", mediaFailure.Action, mediaFailure.ErrorCode)
	}
	var mediaFailureMetadata map[string]any
	if err := json.Unmarshal(mediaFailure.Metadata, &mediaFailureMetadata); err != nil {
		t.Fatalf("decode media failure metadata: %v", err)
	}
	if mediaFailureMetadata["normalized_reason"] != "customer_media_unreachable" || mediaFailureMetadata["customer_input"] != true {
		t.Fatalf("media failure metadata = %#v", mediaFailureMetadata)
	}

	mediaStaged := logStore.params[2]
	var mediaStagedMetadata map[string]any
	if err := json.Unmarshal(mediaStaged.Metadata, &mediaStagedMetadata); err != nil {
		t.Fatalf("decode media staged metadata: %v", err)
	}
	if mediaStagedMetadata["media_type"] != "image/jpeg" || mediaStagedMetadata["media_size_bytes"] != float64(4096) {
		t.Fatalf("media staged metadata = %#v", mediaStagedMetadata)
	}
	if mediaStaged.WorkspaceID != "ws_pin" || mediaStaged.PostID.String != "post_pin" || mediaStaged.SocialAccountID.String != "account_pin" {
		t.Fatalf("media staged identifiers = workspace:%q post:%#v account:%#v", mediaStaged.WorkspaceID, mediaStaged.PostID, mediaStaged.SocialAccountID)
	}
}

func TestFinalizeRestrictedDeliveryJobLostLeasePreservesNewerSuccess(t *testing.T) {
	publishedAt := time.Date(2026, 7, 26, 19, 45, 0, 0, time.UTC)
	job := baseDeliveryJob()
	job.LeaseOwner = pgtype.Text{String: "worker_stale", Valid: true}
	job.LastAttemptAt = pgtype.Timestamptz{Time: publishedAt.Add(-time.Minute), Valid: true}
	newerSuccess := db.SocialPostResult{
		ID:              job.SocialPostResultID,
		PostID:          job.PostID,
		SocialAccountID: job.SocialAccountID,
		Status:          "published",
		ExternalID:      pgtype.Text{String: "platform_post_newer", Valid: true},
		PublishedAt:     pgtype.Timestamptz{Time: publishedAt, Valid: true},
		Url:             pgtype.Text{String: "https://social.example.com/platform_post_newer", Valid: true},
	}
	dbtx := &restrictedFinalizeLeaseDB{job: job, result: newerSuccess, lostLease: true}
	logStore := &restrictedFinalizeIntegrationLogStore{}
	logger := integrationlogs.NewLogger(logStore, nil)
	loggerContext, stopLogger := context.WithCancel(context.Background())
	loggerStopped := make(chan struct{})
	go func() {
		logger.Start(loggerContext)
		close(loggerStopped)
	}()
	h := NewSocialPostHandler(db.New(dbtx), nil, nil, nil, nil, nil, logger)
	staleResult := newerSuccess
	staleResult.Status = "pending"
	staleResult.ExternalID = pgtype.Text{}
	staleResult.PublishedAt = pgtype.Timestamptz{}
	staleResult.Url = pgtype.Text{}
	post := db.SocialPost{ID: job.PostID, WorkspaceID: job.WorkspaceID, Status: "published"}

	err := h.finalizeRestrictedDeliveryJob(context.Background(), job, staleResult, post, publishingrestrictions.Decision{
		Restricted: true,
		Platform:   job.Platform,
		CycleID:    "cycle_newer_success",
	})
	if err != nil {
		t.Fatalf("finalizeRestrictedDeliveryJob: %v", err)
	}
	stopLogger()
	<-loggerStopped

	if !reflect.DeepEqual(dbtx.result, newerSuccess) {
		t.Fatalf("durable result = %#v, want newer published success %#v", dbtx.result, newerSuccess)
	}
	if dbtx.resultUpdateCalls != 0 {
		t.Fatalf("result update calls = %d, want 0 after lease loss", dbtx.resultUpdateCalls)
	}
	if dbtx.failureDetailCalls != 0 || dbtx.failureHistoryCalls != 0 {
		t.Fatalf("failure detail/history calls = %d/%d, want 0/0 after lease loss", dbtx.failureDetailCalls, dbtx.failureHistoryCalls)
	}
	if dbtx.parentRefreshCalls != 0 || dbtx.retentionCalls != 0 {
		t.Fatalf("parent refresh/retention calls = %d/%d, want 0/0 after lease loss", dbtx.parentRefreshCalls, dbtx.retentionCalls)
	}
	if logStore.calls != 0 {
		t.Fatalf("integration log calls = %d, want 0 after lease loss", logStore.calls)
	}
}

func TestFinalizeRestrictedDeliveryJobPublishedResultConvergesWithoutFailureSideEffects(t *testing.T) {
	publishedAt := time.Date(2026, 7, 27, 1, 0, 0, 0, time.UTC)
	job := baseDeliveryJob()
	job.LeaseOwner = pgtype.Text{String: "worker_restricted", Valid: true}
	job.LastAttemptAt = pgtype.Timestamptz{Time: publishedAt.Add(-time.Minute), Valid: true}
	published := db.SocialPostResult{
		ID:              job.SocialPostResultID,
		PostID:          job.PostID,
		SocialAccountID: job.SocialAccountID,
		Status:          "published",
		ExternalID:      pgtype.Text{String: "platform_post_winner", Valid: true},
		PublishedAt:     pgtype.Timestamptz{Time: publishedAt, Valid: true},
		Url:             pgtype.Text{String: "https://social.example.com/platform_post_winner", Valid: true},
	}
	dbtx := &restrictedFinalizeLeaseDB{
		job:                job,
		result:             published,
		convergedPublished: true,
	}
	logStore := &restrictedFinalizeIntegrationLogStore{}
	logger := integrationlogs.NewLogger(logStore, nil)
	loggerContext, stopLogger := context.WithCancel(context.Background())
	loggerStopped := make(chan struct{})
	go func() {
		logger.Start(loggerContext)
		close(loggerStopped)
	}()
	h := NewSocialPostHandler(db.New(dbtx), nil, nil, nil, nil, nil, logger)
	staleResult := published
	staleResult.Status = "processing"
	staleResult.ExternalID = pgtype.Text{}
	staleResult.PublishedAt = pgtype.Timestamptz{}
	staleResult.Url = pgtype.Text{}
	post := mediaRetentionPost(t, "publishing")
	post.ID = job.PostID
	post.WorkspaceID = job.WorkspaceID

	err := h.finalizeRestrictedDeliveryJob(context.Background(), job, staleResult, post, publishingrestrictions.Decision{
		Restricted: true,
		Platform:   job.Platform,
		CycleID:    "cycle_published_winner",
	})
	if err != nil {
		t.Fatalf("finalizeRestrictedDeliveryJob: %v", err)
	}
	stopLogger()
	<-loggerStopped

	if !reflect.DeepEqual(dbtx.result, published) {
		t.Fatalf("published result changed:\n got=%#v\nwant=%#v", dbtx.result, published)
	}
	if dbtx.job.State != "succeeded" {
		t.Fatalf("restricted duplicate job state = %q, want succeeded convergence", dbtx.job.State)
	}
	if dbtx.resultUpdateCalls != 0 || dbtx.failureDetailCalls != 0 || dbtx.failureHistoryCalls != 0 {
		t.Fatalf("published convergence failure writes = result:%d detail:%d history:%d, want zero",
			dbtx.resultUpdateCalls, dbtx.failureDetailCalls, dbtx.failureHistoryCalls)
	}
	if len(dbtx.retentionUpserts) != 0 {
		t.Fatalf("published convergence restriction retention upserts = %d, want zero", len(dbtx.retentionUpserts))
	}
	if dbtx.parentRefreshCalls != 0 {
		t.Fatalf("published convergence parent refresh calls = %d, want 0 after atomic finalization", dbtx.parentRefreshCalls)
	}
	if logStore.calls != 0 {
		t.Fatalf("published convergence integration failure logs = %d, want zero", logStore.calls)
	}
}

func TestFinalizeRestrictedDeliveryJobOwnedLeasePersistsFailureAndSideEffects(t *testing.T) {
	job := baseDeliveryJob()
	job.LeaseOwner = pgtype.Text{String: "worker_owner", Valid: true}
	job.LastAttemptAt = pgtype.Timestamptz{Time: time.Date(2026, 7, 26, 20, 0, 0, 0, time.UTC), Valid: true}
	result := db.SocialPostResult{
		ID:                job.SocialPostResultID,
		PostID:            job.PostID,
		SocialAccountID:   job.SocialAccountID,
		Status:            "pending",
		ExternalID:        pgtype.Text{String: "stale_external", Valid: true},
		PublishedAt:       pgtype.Timestamptz{Time: job.LastAttemptAt.Time.Add(-time.Hour), Valid: true},
		Url:               pgtype.Text{String: "https://social.example.com/stale", Valid: true},
		DebugCurl:         pgtype.Text{String: "curl stale", Valid: true},
		PlatformErrorCode: pgtype.Text{String: "stale_provider_code", Valid: true},
		ProviderError:     []byte(`{"stale":true}`),
		PublishToken:      pgtype.Text{String: "stale_publish_token", Valid: true},
		XCreditsCounted:   17,
		XCreditOperation:  pgtype.Text{String: "stale_operation", Valid: true},
		XCreditCatalogVersion: pgtype.Text{
			String: "stale_catalog",
			Valid:  true,
		},
		XCreditBillingMode: pgtype.Text{String: "stale_billing", Valid: true},
	}
	dbtx := &restrictedFinalizeLeaseDB{job: job, result: result}
	logStore := &restrictedFinalizeIntegrationLogStore{}
	logger := integrationlogs.NewLogger(logStore, nil)
	loggerContext, stopLogger := context.WithCancel(context.Background())
	loggerStopped := make(chan struct{})
	go func() {
		logger.Start(loggerContext)
		close(loggerStopped)
	}()
	h := NewSocialPostHandler(db.New(dbtx), nil, nil, nil, nil, nil, logger)
	post := mediaRetentionPost(t, "publishing")
	post.ID = job.PostID
	post.WorkspaceID = job.WorkspaceID

	err := h.finalizeRestrictedDeliveryJob(context.Background(), job, result, post, publishingrestrictions.Decision{
		Restricted: true,
		Platform:   job.Platform,
		CycleID:    "cycle_owned_lease",
	})
	if err != nil {
		t.Fatalf("finalizeRestrictedDeliveryJob: %v", err)
	}
	stopLogger()
	<-loggerStopped

	got := dbtx.result
	if got.Status != "failed" || got.ExternalID.Valid || got.PublishedAt.Valid || got.Url.Valid || got.DebugCurl.Valid || got.PublishToken.Valid {
		t.Fatalf("restricted result core fields = %#v, want failed with published artifacts cleared", got)
	}
	if !got.ErrorMessage.Valid || got.ErrorMessage.String != publishingrestrictions.UserMessage {
		t.Fatalf("error_message = %#v, want publishing restriction message", got.ErrorMessage)
	}
	if !got.ErrorCode.Valid || got.ErrorCode.String != publishingrestrictions.NormalizedCode ||
		!got.FailureStage.Valid || got.FailureStage.String != publishingrestrictions.FailureStage {
		t.Fatalf("error code/stage = %#v/%#v, want restriction code/stage", got.ErrorCode, got.FailureStage)
	}
	if !got.IsRetriable.Valid || got.IsRetriable.Bool ||
		!got.NextAction.Valid || got.NextAction.String != publishingrestrictions.NextAction ||
		!got.ErrorSource.Valid || got.ErrorSource.String != "unipost" ||
		!got.ErrorTemporality.Valid || got.ErrorTemporality.String != "temporary" {
		t.Fatalf("failure remediation fields = retriable:%#v action:%#v source:%#v temporality:%#v", got.IsRetriable, got.NextAction, got.ErrorSource, got.ErrorTemporality)
	}
	if got.PlatformErrorCode.Valid || got.ProviderError != nil {
		t.Fatalf("provider diagnostics = code:%#v payload:%s, want null", got.PlatformErrorCode, got.ProviderError)
	}
	if got.XCreditsCounted != 0 || got.XCreditOperation.Valid || got.XCreditCatalogVersion.Valid || got.XCreditBillingMode.Valid {
		t.Errorf("X billing metadata = count:%d operation:%#v catalog:%#v billing:%#v, want cleared", got.XCreditsCounted, got.XCreditOperation, got.XCreditCatalogVersion, got.XCreditBillingMode)
	}
	if dbtx.failureDetailCalls != 0 || dbtx.failureHistoryCalls != 1 {
		t.Errorf("failure detail/history calls = %d/%d, want 0/1 on owned lease", dbtx.failureDetailCalls, dbtx.failureHistoryCalls)
	}
	if dbtx.resultUpdateCalls != 1 {
		t.Fatalf("atomic result update calls = %d, want 1 on owned lease", dbtx.resultUpdateCalls)
	}
	if dbtx.parentRefreshCalls != 0 {
		t.Fatalf("parent refresh calls = %d, want 0 after atomic finalization", dbtx.parentRefreshCalls)
	}
	if dbtx.retentionCalls != 0 {
		t.Fatalf("post-commit retention calls = %d, want 0 after atomic restriction retention", dbtx.retentionCalls)
	}
	if len(dbtx.retentionUpserts) != 2 {
		t.Fatalf("retention upserts = %d, want 2 for restricted post media", len(dbtx.retentionUpserts))
	}
	wantBefore := time.Now().Add(60*24*time.Hour - time.Minute)
	wantAfter := time.Now().Add(60*24*time.Hour + time.Minute)
	for _, upsert := range dbtx.retentionUpserts {
		if upsert.PostStatus != "failed" || upsert.RetentionReason != "publishing_restriction" {
			t.Fatalf("restricted retention = %+v, want atomic publishing_restriction retention", upsert)
		}
		if !upsert.CleanupAfterAt.Valid || upsert.CleanupAfterAt.Time.Before(wantBefore) || upsert.CleanupAfterAt.Time.After(wantAfter) {
			t.Fatalf("restricted cleanup_after_at = %#v, want about 60 days", upsert.CleanupAfterAt)
		}
	}
	if logStore.calls != 1 {
		t.Fatalf("integration log calls = %d, want 1 on owned lease", logStore.calls)
	}
}

func TestFinalizeRestrictedDeliveryJobDoesNotOverwriteNewerRetryRetention(t *testing.T) {
	publishedAt := time.Date(2026, 7, 26, 22, 0, 0, 0, time.UTC)
	job := baseDeliveryJob()
	job.LeaseOwner = pgtype.Text{String: "worker_restricted", Valid: true}
	job.LastAttemptAt = pgtype.Timestamptz{Time: publishedAt.Add(-time.Minute), Valid: true}
	staleResult := db.SocialPostResult{
		ID:              job.SocialPostResultID,
		PostID:          job.PostID,
		SocialAccountID: job.SocialAccountID,
		Status:          "processing",
	}
	post := mediaRetentionPost(t, "published")
	post.ID = job.PostID
	post.WorkspaceID = job.WorkspaceID
	newerSuccess := staleResult
	newerSuccess.Status = "published"
	newerSuccess.ExternalID = pgtype.Text{String: "newer_platform_post", Valid: true}
	newerSuccess.PublishedAt = pgtype.Timestamptz{Time: publishedAt, Valid: true}
	newerSuccess.Url = pgtype.Text{String: "https://social.example.com/newer_platform_post", Valid: true}

	dbtx := &restrictedFinalizeLeaseDB{
		job:    job,
		result: staleResult,
		planID: "team",
	}
	queries := db.New(dbtx)
	h := NewSocialPostHandler(queries, nil, quota.NewChecker(queries), nil, nil, nil, nil)
	dbtx.afterFinalize = func(ctx context.Context) {
		// Interleave a newer retry after A's atomic restriction transition but
		// before A resumes post-commit side effects. B publishes successfully
		// and restores the current plan's media retention.
		dbtx.result = newerSuccess
		dbtx.job.State = "succeeded"
		dbtx.job.LeaseOwner = pgtype.Text{String: "worker_retry", Valid: true}
		h.syncPostMediaRetention(ctx, post, post.Status)
	}

	err := h.finalizeRestrictedDeliveryJob(context.Background(), job, staleResult, post, publishingrestrictions.Decision{
		Restricted: true,
		Platform:   job.Platform,
		CycleID:    "cycle_stale_finalizer",
	})
	if err != nil {
		t.Fatalf("finalizeRestrictedDeliveryJob: %v", err)
	}
	if !reflect.DeepEqual(dbtx.result, newerSuccess) {
		t.Fatalf("durable result = %#v, want newer published success %#v", dbtx.result, newerSuccess)
	}
	if len(dbtx.retentionUpserts) < 2 {
		t.Fatalf("retention upserts = %d, want at least 2", len(dbtx.retentionUpserts))
	}
	wantBefore := time.Now().Add(30*24*time.Hour - time.Minute)
	wantAfter := time.Now().Add(30*24*time.Hour + time.Minute)
	for _, upsert := range dbtx.retentionUpserts[len(dbtx.retentionUpserts)-2:] {
		if upsert.PostStatus != "published" || upsert.RetentionReason != "plan_status" {
			t.Fatalf("final retention = %+v, want published current-plan retention", upsert)
		}
		if !upsert.CleanupAfterAt.Valid || upsert.CleanupAfterAt.Time.Before(wantBefore) || upsert.CleanupAfterAt.Time.After(wantAfter) {
			t.Fatalf("final retention deadline = %#v, want team published deadline about 30 days", upsert.CleanupAfterAt)
		}
	}
}

func TestFinalizeRestrictedDeliveryJobHasNoPostCommitResultRefreshWindow(t *testing.T) {
	job := baseDeliveryJob()
	job.LeaseOwner = pgtype.Text{String: "worker_restricted", Valid: true}
	job.LastAttemptAt = pgtype.Timestamptz{Time: time.Date(2026, 7, 26, 22, 30, 0, 0, time.UTC), Valid: true}
	staleResult := db.SocialPostResult{
		ID:              job.SocialPostResultID,
		PostID:          job.PostID,
		SocialAccountID: job.SocialAccountID,
		Status:          "processing",
	}
	post := mediaRetentionPost(t, "publishing")
	post.ID = job.PostID
	post.WorkspaceID = job.WorkspaceID

	dbtx := &restrictedFinalizeLeaseDB{
		job: job, result: staleResult,
	}
	queries := db.New(dbtx)
	h := NewSocialPostHandler(queries, nil, quota.NewChecker(queries), nil, nil, nil, nil)
	refreshCalled := false
	dbtx.afterListResults = func(ctx context.Context) {
		refreshCalled = true
	}

	err := h.finalizeRestrictedDeliveryJob(context.Background(), job, staleResult, post, publishingrestrictions.Decision{
		Restricted: true,
		Platform:   job.Platform,
		CycleID:    "cycle_stale_snapshot",
	})
	if err != nil {
		t.Fatalf("finalizeRestrictedDeliveryJob: %v", err)
	}
	if refreshCalled || dbtx.parentRefreshCalls != 0 {
		t.Fatalf("post-commit refresh window remained: callback=%v calls=%d", refreshCalled, dbtx.parentRefreshCalls)
	}
	if dbtx.result.Status != "failed" {
		t.Fatalf("durable result status = %q, want failed", dbtx.result.Status)
	}
	for _, upsert := range dbtx.retentionUpserts {
		if upsert.PostStatus != "failed" || upsert.RetentionReason != "publishing_restriction" {
			t.Fatalf("atomic retention = %+v, want failed/publishing_restriction", upsert)
		}
	}
}

func TestFinalizeRestrictedDeliveryJobDoesNotDependOnPostCommitRefresh(t *testing.T) {
	job := baseDeliveryJob()
	job.LeaseOwner = pgtype.Text{String: "worker_owner", Valid: true}
	job.LastAttemptAt = pgtype.Timestamptz{Time: time.Date(2026, 7, 26, 23, 0, 0, 0, time.UTC), Valid: true}
	result := db.SocialPostResult{
		ID:              job.SocialPostResultID,
		PostID:          job.PostID,
		SocialAccountID: job.SocialAccountID,
		Status:          "processing",
	}
	wantErr := errors.New("list current results unavailable")
	dbtx := &restrictedFinalizeLeaseDB{job: job, result: result, listResultsErr: wantErr}
	h := NewSocialPostHandler(db.New(dbtx), nil, nil, nil, nil, nil, nil)
	post := mediaRetentionPost(t, "publishing")
	post.ID = job.PostID
	post.WorkspaceID = job.WorkspaceID

	err := h.finalizeRestrictedDeliveryJob(context.Background(), job, result, post, publishingrestrictions.Decision{
		Restricted: true,
		Platform:   job.Platform,
		CycleID:    "cycle_refresh_error",
	})
	if err != nil {
		t.Fatalf("finalizeRestrictedDeliveryJob error = %v, want nil after atomic finalization", err)
	}
	if dbtx.result.Status != "failed" {
		t.Fatalf("atomic result status = %q, want failed before refresh error", dbtx.result.Status)
	}
	if len(dbtx.retentionUpserts) != 2 {
		t.Fatalf("atomic retention upserts = %d, want 2 despite refresh error", len(dbtx.retentionUpserts))
	}
	for _, upsert := range dbtx.retentionUpserts {
		if upsert.RetentionReason != "publishing_restriction" || !upsert.CleanupAfterAt.Valid {
			t.Fatalf("atomic retention after refresh error = %+v, want durable publishing_restriction retention", upsert)
		}
	}
	if dbtx.parentRefreshCalls != 0 {
		t.Fatalf("parent refresh calls = %d, want 0 after atomic finalization", dbtx.parentRefreshCalls)
	}
}

func TestFinalizeRestrictedDeliveryJobInvalidMediaMetadataFailsClosed(t *testing.T) {
	job := baseDeliveryJob()
	job.LeaseOwner = pgtype.Text{String: "worker_invalid_metadata", Valid: true}
	job.LastAttemptAt = pgtype.Timestamptz{Time: time.Date(2026, 7, 26, 23, 30, 0, 0, time.UTC), Valid: true}
	result := db.SocialPostResult{
		ID:                    job.SocialPostResultID,
		PostID:                job.PostID,
		SocialAccountID:       job.SocialAccountID,
		Status:                "processing",
		XCreditsCounted:       19,
		XCreditOperation:      pgtype.Text{String: "existing_operation", Valid: true},
		XCreditCatalogVersion: pgtype.Text{String: "existing_catalog", Valid: true},
		XCreditBillingMode:    pgtype.Text{String: "existing_billing", Valid: true},
	}
	originalJob := job
	originalResult := result
	dbtx := &restrictedFinalizeLeaseDB{job: job, result: result}
	logStore := &restrictedFinalizeIntegrationLogStore{}
	logger := integrationlogs.NewLogger(logStore, nil)
	loggerContext, stopLogger := context.WithCancel(context.Background())
	loggerStopped := make(chan struct{})
	go func() {
		logger.Start(loggerContext)
		close(loggerStopped)
	}()
	h := NewSocialPostHandler(db.New(dbtx), nil, nil, nil, nil, nil, logger)
	post := db.SocialPost{
		ID:          job.PostID,
		WorkspaceID: job.WorkspaceID,
		Status:      "publishing",
		Metadata:    []byte(`{"schema_version":2,"platform_posts":[`),
	}

	err := h.finalizeRestrictedDeliveryJob(context.Background(), job, result, post, publishingrestrictions.Decision{
		Restricted: true,
		Platform:   job.Platform,
		CycleID:    "cycle_invalid_metadata",
	})
	stopLogger()
	<-loggerStopped
	if !errors.Is(err, errInvalidPostMediaMetadata) {
		t.Fatalf("finalizeRestrictedDeliveryJob error = %v, want media metadata error", err)
	}
	if !reflect.DeepEqual(dbtx.job, originalJob) {
		t.Fatalf("job changed on metadata failure:\n got=%#v\nwant=%#v", dbtx.job, originalJob)
	}
	if !reflect.DeepEqual(dbtx.result, originalResult) {
		t.Fatalf("result/billing changed on metadata failure:\n got=%#v\nwant=%#v", dbtx.result, originalResult)
	}
	if dbtx.resultUpdateCalls != 0 || dbtx.failureHistoryCalls != 0 || dbtx.parentRefreshCalls != 0 || len(dbtx.retentionUpserts) != 0 {
		t.Fatalf("metadata failure side effects = result:%d history:%d parent:%d retention:%d, want all zero",
			dbtx.resultUpdateCalls, dbtx.failureHistoryCalls, dbtx.parentRefreshCalls, len(dbtx.retentionUpserts))
	}
	if logStore.calls != 0 {
		t.Fatalf("integration log calls = %d, want 0 on metadata failure", logStore.calls)
	}
}

func TestFinalizeRestrictedDeliveryJobAllowsValidMetadataWithoutMedia(t *testing.T) {
	job := baseDeliveryJob()
	job.LeaseOwner = pgtype.Text{String: "worker_no_media", Valid: true}
	job.LastAttemptAt = pgtype.Timestamptz{Time: time.Date(2026, 7, 26, 23, 45, 0, 0, time.UTC), Valid: true}
	result := db.SocialPostResult{
		ID:              job.SocialPostResultID,
		PostID:          job.PostID,
		SocialAccountID: job.SocialAccountID,
		Status:          "processing",
	}
	metadata, err := platform.EncodePostMetadata([]platform.PlatformPostInput{{AccountID: job.SocialAccountID}})
	if err != nil {
		t.Fatal(err)
	}
	dbtx := &restrictedFinalizeLeaseDB{job: job, result: result}
	h := NewSocialPostHandler(db.New(dbtx), nil, nil, nil, nil, nil, nil)
	post := db.SocialPost{ID: job.PostID, WorkspaceID: job.WorkspaceID, Status: "publishing", Metadata: metadata}

	err = h.finalizeRestrictedDeliveryJob(context.Background(), job, result, post, publishingrestrictions.Decision{
		Restricted: true,
		Platform:   job.Platform,
		CycleID:    "cycle_valid_no_media",
	})
	if err != nil {
		t.Fatalf("finalizeRestrictedDeliveryJob: %v", err)
	}
	if dbtx.result.Status != "failed" || dbtx.resultUpdateCalls != 1 {
		t.Fatalf("valid no-media finalization = status:%q updates:%d, want failed/1", dbtx.result.Status, dbtx.resultUpdateCalls)
	}
	if len(dbtx.retentionUpserts) != 0 {
		t.Fatalf("no-media retention upserts = %d, want 0", len(dbtx.retentionUpserts))
	}
}

type unavailableAccountAdapterSpy struct {
	platformName string
	postCalls    int
}

func (s *unavailableAccountAdapterSpy) Platform() string { return s.platformName }
func (s *unavailableAccountAdapterSpy) Connect(context.Context, map[string]string) (*platform.ConnectResult, error) {
	return nil, errors.New("unexpected connect")
}
func (s *unavailableAccountAdapterSpy) Post(context.Context, string, string, []platform.MediaItem, map[string]any) (*platform.PostResult, error) {
	s.postCalls++
	return nil, errors.New("unavailable account reached adapter")
}
func (s *unavailableAccountAdapterSpy) DeletePost(context.Context, string, string) error {
	return errors.New("unexpected delete")
}
func (s *unavailableAccountAdapterSpy) RefreshToken(context.Context, string) (string, string, time.Time, error) {
	return "", "", time.Time{}, errors.New("unexpected refresh")
}

type unavailableAccountDeliveryDB struct {
	job                  db.PostDeliveryJob
	post                 db.SocialPost
	result               db.SocialPostResult
	account              db.SocialAccount
	accountErr           error
	platformStartedCalls int
	failureDetailsCode   string
	recordedFailureCode  string
	recordedFailure      db.CreatePostFailureParams
	retryJobs            []db.PostDeliveryJob
	reconnectMarkedIDs   []string
	// reconnectAttemptStamps records the attempt_started_at guard value passed
	// to each mark call; accountLastConnectedAt simulates a reconnect landing
	// at that time so the fake can reproduce the SQL guard.
	reconnectAttemptStamps []pgtype.Timestamptz
	accountLastConnectedAt pgtype.Timestamptz
}

func (f *unavailableAccountDeliveryDB) Begin(context.Context) (pgx.Tx, error) {
	return &queueDBTXTestTx{db: f}, nil
}

type queueDBTXTestTx struct {
	db db.DBTX
}

func (f *queueDBTXTestTx) Begin(context.Context) (pgx.Tx, error) { return f, nil }
func (*queueDBTXTestTx) Commit(context.Context) error            { return nil }
func (*queueDBTXTestTx) Rollback(context.Context) error          { return nil }
func (*queueDBTXTestTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("unexpected copy")
}
func (*queueDBTXTestTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }
func (*queueDBTXTestTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (*queueDBTXTestTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("unexpected prepare")
}
func (f *queueDBTXTestTx) Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	return f.db.Exec(ctx, query, args...)
}
func (f *queueDBTXTestTx) Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	return f.db.Query(ctx, query, args...)
}
func (f *queueDBTXTestTx) QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	return f.db.QueryRow(ctx, query, args...)
}
func (*queueDBTXTestTx) Conn() *pgx.Conn { return nil }

func (f *unavailableAccountDeliveryDB) Exec(_ context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(query, "-- name: LockPostDeliveryJobFinalization"):
		return pgconn.CommandTag{}, nil
	case strings.Contains(query, "-- name: UpdateSocialPostResultFailureDetails"):
		f.failureDetailsCode = args[1].(pgtype.Text).String
		return pgconn.NewCommandTag("UPDATE 1"), nil
	// More specific name first: strings.Contains matches the unguarded query
	// name as a prefix of the guarded one.
	case strings.Contains(query, "-- name: MarkSocialAccountReconnectRequiredForAttempt"):
		attemptStartedAt := args[1].(pgtype.Timestamptz)
		f.reconnectAttemptStamps = append(f.reconnectAttemptStamps, attemptStartedAt)
		// Mirror the SQL guard: a reconnect after this attempt started
		// supersedes the failure and the UPDATE matches no rows.
		if f.accountLastConnectedAt.Valid && f.accountLastConnectedAt.Time.After(attemptStartedAt.Time) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		f.reconnectMarkedIDs = append(f.reconnectMarkedIDs, args[0].(string))
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(query, "-- name: MarkSocialAccountReconnectRequired"):
		f.reconnectMarkedIDs = append(f.reconnectMarkedIDs, args[0].(string))
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(query, "-- name: UpdateSocialPostStatus"),
		strings.Contains(query, "-- name: UpdateSocialPostErrorMetadata"),
		strings.Contains(query, "-- name: DeleteMediaPostUsagesForPost"),
		strings.Contains(query, "-- name: CreateTerminalPostEventDelivery"):
		return pgconn.NewCommandTag("UPDATE 1"), nil
	default:
		return pgconn.CommandTag{}, fmt.Errorf("unexpected Exec: %s", query)
	}
}

func (f *unavailableAccountDeliveryDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	switch {
	case strings.Contains(query, "-- name: ListSocialPostResultsByPost"):
		return &queueRows{values: [][]any{socialPostResultScanRow(f.result).values}}, nil
	case strings.Contains(query, "-- name: ListPostDeliveryJobsByPost"):
		values := [][]any{postDeliveryJobScanRow(f.job).values}
		for _, retryJob := range f.retryJobs {
			values = append(values, postDeliveryJobScanRow(retryJob).values)
		}
		return &queueRows{values: values}, nil
	default:
		return nil, fmt.Errorf("unexpected Query: %s", query)
	}
}

func (f *unavailableAccountDeliveryDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetPostDeliveryJobByIDAndWorkspace"):
		return postDeliveryJobScanRow(f.job)
	case strings.Contains(query, "-- name: GetSocialPostByID"):
		return socialPostScanRow(f.post)
	case strings.Contains(query, "-- name: GetSocialPostResultByIDAndPost"):
		return socialPostResultScanRow(f.result)
	case strings.Contains(query, "-- name: GetSocialAccountByIDAndWorkspace"):
		if f.accountErr != nil {
			return scanRow{err: f.accountErr}
		}
		return policySnapshotSocialAccountRow(f.account)
	case strings.Contains(query, "-- name: MarkPostDeliveryJobPlatformStarted"):
		f.platformStartedCalls++
		started := f.job
		started.PlatformStartedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return postDeliveryJobScanRow(started)
	case strings.Contains(query, "-- name: UpdateSocialPostResultAfterRetry"):
		f.result.Status = args[1].(string)
		f.result.ErrorMessage = args[3].(pgtype.Text)
		return socialPostResultScanRow(f.result)
	case strings.Contains(query, "-- name: CreatePostFailure"):
		f.recordedFailure = db.CreatePostFailureParams{
			PostID:             args[0].(string),
			SocialPostResultID: args[1].(pgtype.Text),
			WorkspaceID:        args[2].(string),
			SocialAccountID:    args[3].(pgtype.Text),
			Platform:           args[4].(string),
			FailureStage:       args[5].(string),
			ErrorCode:          args[6].(string),
			PlatformErrorCode:  args[7].(pgtype.Text),
			Message:            args[8].(string),
			RawError:           args[9].(pgtype.Text),
			IsRetriable:        args[10].(bool),
			ErrorSource:        args[11].(pgtype.Text),
			ErrorTemporality:   args[12].(pgtype.Text),
			ProviderError:      args[13].([]byte),
			RestrictionCycleID: args[14].(pgtype.Text),
		}
		f.recordedFailureCode = f.recordedFailure.ErrorCode
		return policySnapshotPostFailureRow(f.recordedFailure)
	case strings.Contains(query, "-- name: CreatePostDeliveryJob"):
		retryJob := db.PostDeliveryJob{
			ID:                 fmt.Sprintf("retry_%d", len(f.retryJobs)+1),
			PostID:             args[0].(string),
			SocialPostResultID: args[1].(string),
			WorkspaceID:        args[2].(string),
			SocialAccountID:    args[3].(string),
			Platform:           args[4].(string),
			PostInputIndex:     args[5].(int32),
			Kind:               args[6].(string),
			State:              args[7].(string),
			Attempts:           args[8].(int32),
			MaxAttempts:        args[9].(int32),
			FailureStage:       args[10].(pgtype.Text),
			ErrorCode:          args[11].(pgtype.Text),
			PlatformErrorCode:  args[12].(pgtype.Text),
			LastError:          args[13].(pgtype.Text),
			NextRunAt:          args[14].(pgtype.Timestamptz),
			LastAttemptAt:      args[15].(pgtype.Timestamptz),
			FinishedAt:         args[16].(pgtype.Timestamptz),
			CreatedAt:          pgtype.Timestamptz{Time: time.Now(), Valid: true},
		}
		f.retryJobs = append(f.retryJobs, retryJob)
		return postDeliveryJobScanRow(retryJob)
	case strings.Contains(query, "-- name: MarkPostDeliveryJobFailed"):
		f.job.State = args[0].(string)
		f.job.FailureStage = args[1].(pgtype.Text)
		f.job.ErrorCode = args[2].(pgtype.Text)
		f.job.PlatformErrorCode = args[3].(pgtype.Text)
		f.job.LastError = args[4].(pgtype.Text)
		f.job.NextRunAt = args[5].(pgtype.Timestamptz)
		return postDeliveryJobScanRow(f.job)
	case strings.Contains(query, "-- name: GetSocialPostProjectionVersion"):
		return scanRow{values: []any{"failure-version"}}
	case strings.Contains(query, "-- name: CreatePostStatusTransitionEvent"):
		return terminalPostEventOutboxScanRow("post-event-failure", args[0].(string), args[1].(string), args[2].(string), args[3].(string), args[4].(string))
	default:
		return scanRow{err: fmt.Errorf("unexpected QueryRow: %s", query)}
	}
}

func TestProcessPostDeliveryJobRejectsMissingAccountBeforePlatformStart(t *testing.T) {
	metadata, err := platform.EncodePostMetadata([]platform.PlatformPostInput{{AccountID: "acct_missing", Caption: "do not dispatch"}})
	if err != nil {
		t.Fatal(err)
	}
	job := baseDeliveryJob()
	job.SocialAccountID = "acct_missing"
	job.LeaseOwner = pgtype.Text{String: "worker_1", Valid: true}
	job.LastAttemptAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	dbtx := &unavailableAccountDeliveryDB{
		job: job,
		post: db.SocialPost{
			ID: job.PostID, WorkspaceID: job.WorkspaceID, Status: "publishing", Metadata: metadata,
			CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		},
		result: db.SocialPostResult{
			ID: job.SocialPostResultID, PostID: job.PostID, SocialAccountID: job.SocialAccountID,
			Status: "processing", Caption: "do not dispatch",
		},
		accountErr: pgx.ErrNoRows,
	}
	adapter := &unavailableAccountAdapterSpy{platformName: "instagram"}
	previous, previousErr := platform.Get("instagram")
	platform.Register(adapter)
	defer func() {
		if previousErr == nil {
			platform.Register(previous)
		}
	}()
	logStore := &restrictedFinalizeIntegrationLogStore{}
	logger, stopLogger := runIntegrationLogger(t, logStore)
	defer stopLogger()
	h := NewSocialPostHandler(db.New(dbtx), nil, nil, nil, nil, nil, logger)

	if err := h.ProcessPostDeliveryJob(context.Background(), job); err != nil {
		t.Fatalf("ProcessPostDeliveryJob: %v", err)
	}
	stopLogger()

	if adapter.postCalls != 0 {
		t.Fatalf("adapter post calls = %d, want 0", adapter.postCalls)
	}
	if dbtx.platformStartedCalls != 0 || dbtx.job.PlatformStartedAt.Valid {
		t.Fatalf("platform start writes = %d timestamp=%v, want none", dbtx.platformStartedCalls, dbtx.job.PlatformStartedAt)
	}
	if dbtx.result.Status != "failed" || dbtx.result.ErrorMessage.String != "account not found" || dbtx.recordedFailureCode != "target_not_found" {
		t.Fatalf("terminal result/failure = status:%q message:%q history:%q, want failed/account not found/target_not_found",
			dbtx.result.Status, dbtx.result.ErrorMessage.String, dbtx.recordedFailureCode)
	}
	if dbtx.job.State != "dead" || dbtx.job.ErrorCode.String != "target_not_found" || dbtx.job.NextRunAt.Valid {
		t.Fatalf("job = state:%q code:%q next_run_at:%v, want dead/target_not_found/nonretry", dbtx.job.State, dbtx.job.ErrorCode.String, dbtx.job.NextRunAt)
	}
	for _, params := range logStore.params {
		if params.Action == integrationlogs.ActionPostPublishPlatformStarted {
			t.Fatal("missing account emitted a platform-started integration log")
		}
	}
}

func TestPinterestDestinationFailureRetryDisposition(t *testing.T) {
	tests := []struct {
		name             string
		failure          typedFailureStageTestError
		wantOriginal     string
		wantResult       string
		wantRetryJobs    int
		wantErrorCode    string
		wantNextRunValid bool
	}{
		{
			name: "permanent board failure is terminal after one attempt",
			failure: typedFailureStageTestError{
				message:     "Pinterest board is unavailable or not writable for this account.",
				stage:       "destination_preflight",
				errorCode:   "target_not_found",
				isRetriable: false,
				provider:    map[string]any{"provider": "pinterest", "http_status": 403, "code": "29", "reason": "board_not_writable"},
			},
			wantOriginal:  "dead",
			wantResult:    "failed",
			wantRetryJobs: 0,
			wantErrorCode: "target_not_found",
		},
		{
			name: "temporary board failure schedules retry",
			failure: typedFailureStageTestError{
				message:     "Pinterest board verification is temporarily unavailable.",
				stage:       "destination_preflight",
				errorCode:   "temporary_platform_error",
				isRetriable: true,
				provider:    map[string]any{"provider": "pinterest", "http_status": 503, "code": "503", "reason": "provider_unavailable"},
			},
			wantOriginal:     "failed",
			wantResult:       "processing",
			wantRetryJobs:    1,
			wantErrorCode:    "temporary_platform_error",
			wantNextRunValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := baseDeliveryJob()
			job.Platform = "pinterest"
			job.SocialAccountID = "acct_pin"
			job.LeaseOwner = pgtype.Text{String: "worker_pin", Valid: true}
			job.LastAttemptAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
			post := db.SocialPost{ID: job.PostID, WorkspaceID: job.WorkspaceID, Status: "publishing"}
			result := db.SocialPostResult{
				ID: job.SocialPostResultID, PostID: job.PostID, SocialAccountID: job.SocialAccountID,
				Status: "processing", Caption: "safe caption",
			}
			dbtx := &unavailableAccountDeliveryDB{job: job, post: post, result: result}
			h := NewSocialPostHandler(db.New(dbtx), nil, nil, nil, nil, nil, nil)

			err := h.handleJobDispatchFailure(
				withoutPostMediaRetentionSync(context.Background()),
				post,
				result,
				job,
				publishOneOutcome{platform: "pinterest", err: tt.failure},
			)
			if err != nil {
				t.Fatalf("handleJobDispatchFailure: %v", err)
			}
			if dbtx.job.State != tt.wantOriginal || dbtx.job.Attempts != 1 {
				t.Errorf("original job = state:%q attempts:%d, want %q/1", dbtx.job.State, dbtx.job.Attempts, tt.wantOriginal)
			}
			if dbtx.job.FailureStage.String != "destination_preflight" || dbtx.job.NextRunAt.Valid {
				t.Errorf("original failure stage/next run = %q/%#v, want destination_preflight/empty", dbtx.job.FailureStage.String, dbtx.job.NextRunAt)
			}
			if dbtx.result.Status != tt.wantResult {
				t.Errorf("result status = %q, want %q", dbtx.result.Status, tt.wantResult)
			}
			if len(dbtx.retryJobs) != tt.wantRetryJobs {
				t.Fatalf("retry job count = %d, want %d", len(dbtx.retryJobs), tt.wantRetryJobs)
			}
			if tt.wantRetryJobs == 1 {
				retry := dbtx.retryJobs[0]
				if retry.Kind != "retry" || retry.State != "pending" || retry.Attempts != 0 || retry.FailureStage.String != "destination_preflight" || retry.NextRunAt.Valid != tt.wantNextRunValid {
					t.Errorf("retry job = %#v, want pending retry at destination_preflight with a schedule", retry)
				}
			}
			failure := dbtx.recordedFailure
			if failure.FailureStage != "destination_preflight" || failure.ErrorCode != tt.wantErrorCode || failure.IsRetriable != tt.failure.isRetriable {
				t.Errorf("recorded failure = stage:%q code:%q retriable:%v", failure.FailureStage, failure.ErrorCode, failure.IsRetriable)
			}
			diagnostics := failure.Message + " " + failure.RawError.String + " " + string(failure.ProviderError)
			for _, forbidden := range []string{"access_token", "secret-token", "api.pinterest.com/v5/boards"} {
				if strings.Contains(diagnostics, forbidden) {
					t.Fatalf("failure diagnostics contain %q: %s", forbidden, diagnostics)
				}
			}
		})
	}
}

// A provider that rejects an unexpired token at dispatch is invisible to the
// token refresh worker, which only marks accounts whose refresh fails. Without
// this marking the account stays status='active' while every delivery keeps
// failing, so the customer sees next_action=reconnect_account on the result but
// never gets a reconnect prompt on the account. Platform-agnostic: the dispatch
// failure path is shared by every adapter.
func TestDispatchFailureMarksAccountReconnectRequired(t *testing.T) {
	tests := []struct {
		name       string
		platform   string
		failure    typedFailureStageTestError
		wantMarked bool
	}{
		{
			name:     "pinterest token rejected at dispatch",
			platform: "pinterest",
			failure: typedFailureStageTestError{
				message:     "Pinterest rejected this connection. Reconnect the account, then try again.",
				stage:       "dispatch",
				errorCode:   "auth_token_invalid",
				isRetriable: false,
				provider:    map[string]any{"provider": "pinterest", "http_status": 401, "code": "3", "reason": "token_invalid"},
			},
			wantMarked: true,
		},
		{
			name:     "instagram reconnect required at dispatch",
			platform: "instagram",
			failure: typedFailureStageTestError{
				message:     "Instagram requires this account to be reconnected.",
				stage:       "dispatch",
				errorCode:   "account_reconnect_required",
				isRetriable: false,
				provider:    map[string]any{"provider": "meta", "http_status": 401, "code": "190"},
			},
			wantMarked: true,
		},
		{
			name:     "tiktok token invalid at dispatch",
			platform: "tiktok",
			failure: typedFailureStageTestError{
				message:     "TikTok rejected the stored credentials.",
				stage:       "dispatch",
				errorCode:   "auth_token_invalid",
				isRetriable: false,
				provider:    map[string]any{"provider": "tiktok", "http_status": 401},
			},
			wantMarked: true,
		},
		{
			name:     "destination failure leaves the account alone",
			platform: "pinterest",
			failure: typedFailureStageTestError{
				message:     "The selected Pinterest board is unavailable for this connected account.",
				stage:       "destination_preflight",
				errorCode:   "target_not_found",
				isRetriable: false,
				provider:    map[string]any{"provider": "pinterest", "http_status": 404, "code": "40", "reason": "board_not_found"},
			},
			wantMarked: false,
		},
		{
			name:     "media failure leaves the account alone",
			platform: "pinterest",
			failure: typedFailureStageTestError{
				message:     "Pinterest could not use this media.",
				stage:       "media_preflight",
				errorCode:   "media_error",
				isRetriable: false,
				provider:    map[string]any{"provider": "pinterest", "reason": "customer_media_unreachable"},
			},
			wantMarked: false,
		},
		{
			name:     "transient failure leaves the account alone",
			platform: "pinterest",
			failure: typedFailureStageTestError{
				message:     "Pinterest is temporarily unavailable. Try again later.",
				stage:       "dispatch",
				errorCode:   "temporary_platform_error",
				isRetriable: true,
				provider:    map[string]any{"provider": "pinterest", "http_status": 503},
			},
			wantMarked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := baseDeliveryJob()
			job.Platform = tt.platform
			job.SocialAccountID = "acct_reconnect"
			job.LeaseOwner = pgtype.Text{String: "worker_reconnect", Valid: true}
			job.LastAttemptAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
			post := db.SocialPost{ID: job.PostID, WorkspaceID: job.WorkspaceID, Status: "publishing"}
			result := db.SocialPostResult{
				ID: job.SocialPostResultID, PostID: job.PostID, SocialAccountID: job.SocialAccountID,
				Status: "processing", Caption: "reconnect marking",
			}
			dbtx := &unavailableAccountDeliveryDB{job: job, post: post, result: result}
			h := NewSocialPostHandler(db.New(dbtx), nil, nil, nil, nil, nil, nil)

			if err := h.handleJobDispatchFailure(
				withoutPostMediaRetentionSync(context.Background()),
				post,
				result,
				job,
				publishOneOutcome{platform: tt.platform, err: tt.failure},
			); err != nil {
				t.Fatalf("handleJobDispatchFailure: %v", err)
			}

			if tt.wantMarked {
				if len(dbtx.reconnectMarkedIDs) != 1 {
					t.Fatalf("reconnect marks = %v, want exactly one for %s", dbtx.reconnectMarkedIDs, tt.failure.errorCode)
				}
				if dbtx.reconnectMarkedIDs[0] != job.SocialAccountID {
					t.Errorf("marked account = %q, want %q", dbtx.reconnectMarkedIDs[0], job.SocialAccountID)
				}
				return
			}
			if len(dbtx.reconnectMarkedIDs) != 0 {
				t.Fatalf("reconnect marks = %v, want none for %s", dbtx.reconnectMarkedIDs, tt.failure.errorCode)
			}
		})
	}
}

// A delivery job dispatches with the token it loaded when the job was claimed.
// If the customer reconnects while that request is still in flight, the repair
// must win: the stale auth failure that comes back afterwards must not flip the
// freshly reconnected account back to reconnect_required and block it again.
func TestDispatchFailureDoesNotUndoConcurrentReconnect(t *testing.T) {
	attemptStartedAt := time.Now().Add(-2 * time.Minute)

	tests := []struct {
		name              string
		lastConnectedAt   pgtype.Timestamptz
		wantMarked        bool
		wantMarkAttempted bool
	}{
		{
			name:              "reconnect during dispatch supersedes the failure",
			lastConnectedAt:   pgtype.Timestamptz{Time: attemptStartedAt.Add(time.Minute), Valid: true},
			wantMarked:        false,
			wantMarkAttempted: true,
		},
		{
			name:              "reconnect before the attempt does not shield a real failure",
			lastConnectedAt:   pgtype.Timestamptz{Time: attemptStartedAt.Add(-time.Hour), Valid: true},
			wantMarked:        true,
			wantMarkAttempted: true,
		},
		{
			name:              "account never reconnected still marks",
			lastConnectedAt:   pgtype.Timestamptz{},
			wantMarked:        true,
			wantMarkAttempted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := baseDeliveryJob()
			job.Platform = "pinterest"
			job.SocialAccountID = "acct_race"
			job.LeaseOwner = pgtype.Text{String: "worker_race", Valid: true}
			job.LastAttemptAt = pgtype.Timestamptz{Time: attemptStartedAt, Valid: true}
			post := db.SocialPost{ID: job.PostID, WorkspaceID: job.WorkspaceID, Status: "publishing"}
			result := db.SocialPostResult{
				ID: job.SocialPostResultID, PostID: job.PostID, SocialAccountID: job.SocialAccountID,
				Status: "processing", Caption: "reconnect race",
			}
			dbtx := &unavailableAccountDeliveryDB{
				job: job, post: post, result: result,
				accountLastConnectedAt: tt.lastConnectedAt,
			}
			h := NewSocialPostHandler(db.New(dbtx), nil, nil, nil, nil, nil, nil)

			staleFailure := typedFailureStageTestError{
				message:     "Pinterest rejected this connection. Reconnect the account, then try again.",
				stage:       "dispatch",
				errorCode:   "auth_token_invalid",
				isRetriable: false,
				provider:    map[string]any{"provider": "pinterest", "http_status": 401, "code": "3", "reason": "token_invalid"},
			}
			if err := h.handleJobDispatchFailure(
				withoutPostMediaRetentionSync(context.Background()),
				post, result, job,
				publishOneOutcome{platform: "pinterest", err: staleFailure},
			); err != nil {
				t.Fatalf("handleJobDispatchFailure: %v", err)
			}

			if tt.wantMarkAttempted && len(dbtx.reconnectAttemptStamps) != 1 {
				t.Fatalf("guarded mark calls = %d, want 1", len(dbtx.reconnectAttemptStamps))
			}
			if got := dbtx.reconnectAttemptStamps[0].Time; !got.Equal(attemptStartedAt) {
				t.Errorf("attempt_started_at guard = %s, want the job's last_attempt_at %s", got, attemptStartedAt)
			}
			marked := len(dbtx.reconnectMarkedIDs) > 0
			if marked != tt.wantMarked {
				t.Fatalf("account marked = %v, want %v (last_connected_at=%v)", marked, tt.wantMarked, tt.lastConnectedAt)
			}
		})
	}
}

func TestProcessPostDeliveryJobReturnsAccountLookupErrorWithoutDeadLettering(t *testing.T) {
	metadata, err := platform.EncodePostMetadata([]platform.PlatformPostInput{{AccountID: "acct_ig", Caption: "retry lookup"}})
	if err != nil {
		t.Fatal(err)
	}
	job := baseDeliveryJob()
	job.LeaseOwner = pgtype.Text{String: "worker_1", Valid: true}
	job.LastAttemptAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	lookupErr := errors.New("temporary account lookup failure")
	dbtx := &unavailableAccountDeliveryDB{
		job: job,
		post: db.SocialPost{
			ID: job.PostID, WorkspaceID: job.WorkspaceID, Status: "publishing", Metadata: metadata,
			CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		},
		result: db.SocialPostResult{
			ID: job.SocialPostResultID, PostID: job.PostID, SocialAccountID: job.SocialAccountID,
			Status: "processing", Caption: "retry lookup",
		},
		accountErr: lookupErr,
	}
	adapter := &unavailableAccountAdapterSpy{platformName: "instagram"}
	previous, previousErr := platform.Get("instagram")
	platform.Register(adapter)
	defer func() {
		if previousErr == nil {
			platform.Register(previous)
		}
	}()
	h := NewSocialPostHandler(db.New(dbtx), nil, nil, nil, nil, nil, nil)

	err = h.ProcessPostDeliveryJob(context.Background(), job)
	if !errors.Is(err, lookupErr) {
		t.Fatalf("ProcessPostDeliveryJob error = %v, want %v", err, lookupErr)
	}
	if adapter.postCalls != 0 {
		t.Fatalf("adapter post calls = %d, want 0", adapter.postCalls)
	}
	if dbtx.platformStartedCalls != 0 || dbtx.job.PlatformStartedAt.Valid {
		t.Fatalf("platform start writes = %d timestamp=%v, want none", dbtx.platformStartedCalls, dbtx.job.PlatformStartedAt)
	}
	if dbtx.result.Status != "processing" || dbtx.job.State != "running" {
		t.Fatalf("lookup error mutated terminal state = result:%q job:%q, want processing/running", dbtx.result.Status, dbtx.job.State)
	}
}

func TestProcessPostDeliveryJobRejectsUnavailableAccountBeforePlatformStart(t *testing.T) {
	metadata, err := platform.EncodePostMetadata([]platform.PlatformPostInput{{AccountID: "acct_ig", Caption: "do not dispatch"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		account db.SocialAccount
	}{
		{
			name:    "reconnect_required",
			account: db.SocialAccount{ID: "acct_ig", ProfileID: "profile_1", Platform: "instagram", Status: "reconnect_required"},
		},
		{
			name: "disconnected_at",
			account: db.SocialAccount{
				ID: "acct_ig", ProfileID: "profile_1", Platform: "instagram", Status: "active",
				DisconnectedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			job := baseDeliveryJob()
			job.LeaseOwner = pgtype.Text{String: "worker_1", Valid: true}
			job.LastAttemptAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
			dbtx := &unavailableAccountDeliveryDB{
				job: job,
				post: db.SocialPost{
					ID: job.PostID, WorkspaceID: job.WorkspaceID, Status: "publishing", Metadata: metadata,
					CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
				},
				result: db.SocialPostResult{
					ID: job.SocialPostResultID, PostID: job.PostID, SocialAccountID: job.SocialAccountID,
					Status: "processing", Caption: "do not dispatch",
				},
				account: test.account,
			}
			adapter := &unavailableAccountAdapterSpy{platformName: "instagram"}
			previous, previousErr := platform.Get("instagram")
			platform.Register(adapter)
			defer func() {
				if previousErr == nil {
					platform.Register(previous)
				}
			}()
			logStore := &restrictedFinalizeIntegrationLogStore{}
			logger, stopLogger := runIntegrationLogger(t, logStore)
			defer stopLogger()
			h := NewSocialPostHandler(db.New(dbtx), nil, nil, nil, nil, nil, logger)

			if err := h.ProcessPostDeliveryJob(context.Background(), job); err != nil {
				t.Fatalf("ProcessPostDeliveryJob: %v", err)
			}
			stopLogger()

			if adapter.postCalls != 0 {
				t.Fatalf("adapter post calls = %d, want 0", adapter.postCalls)
			}
			if dbtx.platformStartedCalls != 0 || dbtx.job.PlatformStartedAt.Valid {
				t.Fatalf("platform start writes = %d timestamp=%v, want none", dbtx.platformStartedCalls, dbtx.job.PlatformStartedAt)
			}
			if dbtx.result.Status != "failed" || dbtx.failureDetailsCode != "account_reconnect_required" || dbtx.recordedFailureCode != "account_reconnect_required" {
				t.Fatalf("terminal result/failure = status:%q details:%q history:%q", dbtx.result.Status, dbtx.failureDetailsCode, dbtx.recordedFailureCode)
			}
			if dbtx.job.State != "dead" || dbtx.job.ErrorCode.String != "account_reconnect_required" {
				t.Fatalf("job = state:%q code:%q, want dead/account_reconnect_required", dbtx.job.State, dbtx.job.ErrorCode.String)
			}
			for _, params := range logStore.params {
				if params.Action == integrationlogs.ActionPostPublishPlatformStarted {
					t.Fatal("unavailable account emitted a platform-started integration log")
				}
			}
		})
	}
}

type persistedTokenRestrictionSpy struct{ calls int }

func (s *persistedTokenRestrictionSpy) Evaluate(context.Context, string, string) (publishingrestrictions.Decision, error) {
	s.calls++
	return publishingrestrictions.Decision{
		Restricted: true,
		Platform:   "tiktok",
		CycleID:    "cycle_restricted",
	}, nil
}

type persistedTokenTikTokAdapterSpy struct {
	resumeToken string
	resumeCalls int
	initCalls   int
	uploadCalls int
}

func (s *persistedTokenTikTokAdapterSpy) Platform() string { return "tiktok" }

func (s *persistedTokenTikTokAdapterSpy) Connect(context.Context, map[string]string) (*platform.ConnectResult, error) {
	return nil, errors.New("unexpected TikTok connect")
}

func (s *persistedTokenTikTokAdapterSpy) Post(_ context.Context, _ string, _ string, _ []platform.MediaItem, opts map[string]any) (*platform.PostResult, error) {
	resume, _ := opts[platform.OptResumePublishToken].(string)
	if resume == "" {
		s.initCalls++
		s.uploadCalls++
		return nil, errors.New("TikTok publish restarted without persisted token")
	}
	s.resumeToken = resume
	s.resumeCalls++
	return &platform.PostResult{ExternalID: resume, Status: "processing"}, nil
}

func (s *persistedTokenTikTokAdapterSpy) DeletePost(context.Context, string, string) error {
	return errors.New("unexpected TikTok delete")
}

func (s *persistedTokenTikTokAdapterSpy) RefreshToken(context.Context, string) (string, string, time.Time, error) {
	return "", "", time.Time{}, errors.New("unexpected TikTok refresh")
}

type persistedTokenDeliveryDB struct {
	job                    db.PostDeliveryJob
	post                   db.SocialPost
	result                 db.SocialPostResult
	account                db.SocialAccount
	restrictedFinalization bool
}

func (f *persistedTokenDeliveryDB) Begin(context.Context) (pgx.Tx, error) {
	return &queueDBTXTestTx{db: f}, nil
}

func (f *persistedTokenDeliveryDB) Exec(_ context.Context, query string, _ ...interface{}) (pgconn.CommandTag, error) {
	if strings.Contains(query, "-- name: LockPostDeliveryJobFinalization") ||
		strings.Contains(query, "-- name: DeleteMediaPostUsagesForPost") ||
		strings.Contains(query, "-- name: UpdateSocialPostStatus") ||
		strings.Contains(query, "-- name: UpdateSocialPostErrorMetadata") {
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}
	if strings.Contains(query, "-- name: UpdateSocialPostResultFailureDetails") ||
		strings.Contains(query, "-- name: CreatePostFailure") {
		f.restrictedFinalization = true
	}
	return pgconn.CommandTag{}, errors.New("optional write not implemented: " + query)
}

func (f *persistedTokenDeliveryDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	switch {
	case strings.Contains(query, "-- name: ListSocialPostResultsByPost"):
		return &queueRows{values: [][]any{socialPostResultScanRow(f.result).values}}, nil
	case strings.Contains(query, "-- name: ListPostDeliveryJobsByPost"):
		return &queueRows{}, nil
	default:
		return nil, errors.New("optional query not implemented: " + query)
	}
}

func (f *persistedTokenDeliveryDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetPostDeliveryJobByIDAndWorkspace"):
		return postDeliveryJobScanRow(f.job)
	case strings.Contains(query, "-- name: GetSocialPostByID"):
		return socialPostScanRow(f.post)
	case strings.Contains(query, "-- name: GetSocialPostResultByIDAndPost"):
		return socialPostResultScanRow(f.result)
	case strings.Contains(query, "-- name: GetSocialAccountByIDAndWorkspace"):
		return scanRow{values: []any{
			f.account.ID, f.account.ProfileID, f.account.Platform, f.account.AccessToken,
			f.account.RefreshToken, f.account.TokenExpiresAt, f.account.ExternalAccountID,
			f.account.AccountName, f.account.AccountAvatarUrl, f.account.ConnectedAt,
			f.account.DisconnectedAt, f.account.Metadata, f.account.Scope, f.account.Status,
			f.account.ConnectionType, f.account.ConnectSessionID, f.account.ExternalUserID,
			f.account.ExternalUserEmail, f.account.LastRefreshedAt, f.account.XAppMode,
		}}
	case strings.Contains(query, "-- name: GetWorkspace"):
		return scanRow{err: pgx.ErrNoRows}
	case strings.Contains(query, "-- name: CountPublishedTodayByAccount"):
		return scanRow{values: []any{int32(0)}}
	case strings.Contains(query, "-- name: MarkPostDeliveryJobPlatformStarted"):
		started := f.job
		started.PlatformStartedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return postDeliveryJobScanRow(started)
	case strings.Contains(query, "-- name: UpdateSocialPostResultAfterRetry"):
		if strings.Contains(query, "-- name: UpdateSocialPostResultFailureDetails") {
			f.restrictedFinalization = true
			return scanRow{err: errors.New("restricted finalization must not run")}
		}
		updated := f.result
		updated.Status = args[1].(string)
		updated.ExternalID = args[2].(pgtype.Text)
		updated.PublishToken = f.result.PublishToken
		f.result = updated
		return socialPostResultScanRow(updated)
	case strings.Contains(query, "-- name: MarkPostDeliveryJobFailed"):
		f.restrictedFinalization = true
		return scanRow{err: errors.New("restricted job finalization must not run")}
	case strings.Contains(query, "-- name: MarkPostDeliveryJobSucceeded"):
		succeeded := f.job
		succeeded.State = "succeeded"
		return postDeliveryJobScanRow(succeeded)
	default:
		return scanRow{err: errors.New("optional QueryRow not implemented: " + query)}
	}
}

func TestProcessPostDeliveryJobResumesPersistedTikTokTokenAcrossNewRestriction(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	encryptedToken, err := encryptor.Encrypt("tiktok-access-token")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := platform.EncodePostMetadata([]platform.PlatformPostInput{{
		AccountID: "acct_tiktok",
		Caption:   "resume this upload",
		MediaURLs: []string{"https://media.example.com/video.mp4"},
	}})
	if err != nil {
		t.Fatal(err)
	}

	job := baseDeliveryJob()
	job.SocialAccountID = "acct_tiktok"
	job.Platform = "tiktok"
	dbtx := &persistedTokenDeliveryDB{
		job: job,
		post: db.SocialPost{
			ID:          job.PostID,
			Caption:     pgtype.Text{String: "resume this upload", Valid: true},
			Status:      "publishing",
			CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
			Metadata:    metadata,
			WorkspaceID: job.WorkspaceID,
		},
		result: db.SocialPostResult{
			ID:              job.SocialPostResultID,
			PostID:          job.PostID,
			SocialAccountID: job.SocialAccountID,
			Status:          "processing",
			Caption:         "resume this upload",
			PublishToken:    pgtype.Text{String: "publish_token_123", Valid: true},
		},
		account: db.SocialAccount{
			ID:                job.SocialAccountID,
			ProfileID:         "profile_1",
			Platform:          "tiktok",
			AccessToken:       encryptedToken,
			ExternalAccountID: "tiktok_user_1",
			Status:            "active",
			ConnectedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
		},
	}
	restrictions := &persistedTokenRestrictionSpy{}
	adapter := &persistedTokenTikTokAdapterSpy{}
	previous, previousErr := platform.Get("tiktok")
	platform.Register(adapter)
	defer func() {
		if previousErr == nil {
			platform.Register(previous)
		} else {
			platform.Register(platform.NewTikTokAdapter())
		}
	}()

	h := NewSocialPostHandler(db.New(dbtx), encryptor, nil, nil, nil, nil, nil).
		SetPublishingRestrictions(restrictions)
	if err := h.ProcessPostDeliveryJob(context.Background(), job); err != nil {
		t.Fatalf("ProcessPostDeliveryJob: %v", err)
	}

	if restrictions.calls != 0 {
		t.Fatalf("publishing restriction evaluations = %d, want 0 after a persisted publish token", restrictions.calls)
	}
	if adapter.resumeCalls != 1 || adapter.resumeToken != "publish_token_123" {
		t.Fatalf("TikTok resume calls=%d token=%q, want one resume with persisted token", adapter.resumeCalls, adapter.resumeToken)
	}
	if adapter.initCalls != 0 || adapter.uploadCalls != 0 {
		t.Fatalf("TikTok init/upload calls = %d/%d, want 0/0 on resume", adapter.initCalls, adapter.uploadCalls)
	}
	if dbtx.restrictedFinalization {
		t.Fatal("persisted-token resume must not be finalized as publishing-restricted")
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

func (f *stalePublishedResultDB) Begin(context.Context) (pgx.Tx, error) {
	return &queueDBTXTestTx{db: f}, nil
}

func (f *stalePublishedResultDB) Exec(_ context.Context, query string, _ ...interface{}) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(query, "-- name: LockPostDeliveryJobFinalization"):
		return pgconn.CommandTag{}, nil
	case strings.Contains(query, "-- name: UpdateSocialPostStatus"),
		strings.Contains(query, "-- name: UpdateSocialPostErrorMetadata"),
		strings.Contains(query, "-- name: DeleteMediaPostUsagesForPost"),
		strings.Contains(query, "-- name: CreateTerminalPostEventDelivery"):
		return pgconn.NewCommandTag("UPDATE 1"), nil
	default:
		return pgconn.CommandTag{}, errors.New("unexpected Exec")
	}
}

func (f *stalePublishedResultDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	if strings.Contains(query, "-- name: ListStaleActivePostDeliveryJobs") {
		return &queueRows{values: [][]any{postDeliveryJobValues(f.staleJob)}}, nil
	}
	if strings.Contains(query, "-- name: ListSocialPostResultsByPost") {
		return &queueRows{values: [][]any{socialPostResultScanRow(f.result).values}}, nil
	}
	if strings.Contains(query, "-- name: ListPostDeliveryJobsByPost") {
		return &queueRows{values: [][]any{postDeliveryJobScanRow(f.staleJob).values}}, nil
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
		finishedAt := args[0].(pgtype.Timestamptz)
		f.markedSucceededID = args[1].(string)
		succeeded := f.staleJob
		succeeded.State = "succeeded"
		succeeded.FinishedAt = finishedAt
		f.staleJob = succeeded
		return postDeliveryJobScanRow(succeeded)
	case strings.Contains(query, "-- name: CreatePostDeliveryJob"):
		f.createdRetryJob = true
		return scanRow{err: errors.New("must not create a retry job when result already published")}
	case strings.Contains(query, "-- name: GetSocialPostProjectionVersion"):
		return scanRow{values: []any{"published-version"}}
	case strings.Contains(query, "-- name: ElectWorkspaceFirstPostPublished"):
		return scanRow{values: []any{f.post.WorkspaceID}}
	case strings.Contains(query, "-- name: CreatePostStatusTransitionEvent"):
		return terminalPostEventOutboxScanRow("post-event-2", f.post.ID, f.post.WorkspaceID, "published", "published-version", "post.published")
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
	h := &SocialPostHandler{queries: db.New(dbtx), bus: events.NoopBus{}}

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
