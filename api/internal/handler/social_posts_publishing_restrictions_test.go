package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

type fakePostRestrictionEvaluator struct {
	decisions map[string]publishingrestrictions.Decision
	err       error
	errOnCall int
	calls     []string
}

func TestRetryResultRechecksRestrictionBeforeEnqueue(t *testing.T) {
	source, err := os.ReadFile("social_post_retry.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "func (h *SocialPostHandler) RetryResult")
	end := strings.Index(text[start:], "func (h *SocialPostHandler) refreshParentPostStatus")
	if start < 0 || end < 0 {
		t.Fatal("RetryResult boundaries not found")
	}
	fn := text[start : start+end]
	policy := strings.Index(fn, "evaluateRetryPublishingRestriction")
	enqueue := strings.Index(fn, "EnqueueRetryForResult")
	if policy < 0 || enqueue < 0 || policy > enqueue {
		t.Fatalf("policy recheck must precede retry enqueue: policy=%d enqueue=%d", policy, enqueue)
	}
}

func TestBothRetryHandlersMapPolicyReadFailuresToRetryableServiceUnavailable(t *testing.T) {
	for _, file := range []string{"social_post_retry.go", "social_post_queue.go"} {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(source)
		if !strings.Contains(text, "retryPolicyUnavailableError") ||
			!strings.Contains(text, "http.StatusServiceUnavailable") ||
			!strings.Contains(text, `"POLICY_UNAVAILABLE"`) {
			t.Fatalf("%s must map policy read failures to 503 POLICY_UNAVAILABLE", file)
		}
	}
}

func TestRetryQueueConflictRecognizesConcurrentActiveJobUniqueViolation(t *testing.T) {
	activeRetry := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "post_delivery_jobs_one_active_per_result_idx",
	}
	if !isQueueConflict(activeRetry) {
		t.Fatal("concurrent active retry must map to QUEUE_JOB_ACTIVE")
	}
	if isQueueConflict(&pgconn.PgError{Code: "23505", ConstraintName: "unrelated_unique_idx"}) {
		t.Fatal("unrelated unique violations must not map to QUEUE_JOB_ACTIVE")
	}
}

func TestMissingRetryAccountUsesStableActionError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeRetrySocialAccountUnavailable(rec)
	if rec.Code != http.StatusConflict ||
		!strings.Contains(rec.Body.String(), `"code":"SOCIAL_ACCOUNT_NOT_AVAILABLE"`) ||
		strings.Contains(rec.Body.String(), "POLICY_UNAVAILABLE") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func (f *fakePostRestrictionEvaluator) Evaluate(_ context.Context, _ string, platformName string) (publishingrestrictions.Decision, error) {
	f.calls = append(f.calls, platformName)
	if f.err != nil && (f.errOnCall == 0 || len(f.calls) == f.errOnCall) {
		return publishingrestrictions.Decision{}, f.err
	}
	return f.decisions[platformName], nil
}

func TestImmediatePersistenceUsesAdmissionPolicySnapshot(t *testing.T) {
	h, dbtx, evaluator, parsed, accounts := newPolicySnapshotHarness(t, "publishing")
	blockedTargets, err := h.evaluatePublishingRestrictions(context.Background(), "ws_1", parsed.Posts, accounts)
	if err != nil {
		t.Fatalf("admission policy snapshot: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/social-posts", nil)
	if _, err := h.executeImmediatePost(req, "ws_1", parsed, accounts, blockedTargets); err != nil {
		t.Fatalf("executeImmediatePost: %v", err)
	}
	assertMixedPolicySnapshotPersistence(t, dbtx, evaluator)
}

func TestBulkPersistenceUsesAdmissionPolicySnapshot(t *testing.T) {
	h, dbtx, evaluator, parsed, accounts := newPolicySnapshotHarness(t, "publishing")
	body := publishRequestBody{PlatformPosts: []platformPostBody{
		{AccountID: parsed.Posts[0].AccountID, Caption: parsed.Posts[0].Caption, MediaURLs: parsed.Posts[0].MediaURLs, PlatformOptions: parsed.Posts[0].PlatformOptions},
		{AccountID: parsed.Posts[1].AccountID, Caption: parsed.Posts[1].Caption, MediaURLs: parsed.Posts[1].MediaURLs},
	}}
	req := httptest.NewRequest(http.MethodPost, "/v1/social-posts/bulk", nil)
	entry, _ := h.processBulkOne(req, "ws_1", body, accounts, quota.FreePlanHardBlockGate{}, 0)
	if entry.Error != nil {
		t.Fatalf("processBulkOne: %+v", entry.Error)
	}
	assertMixedPolicySnapshotPersistence(t, dbtx, evaluator)
}

func TestPublishFromDraftPersistenceUsesAdmissionPolicySnapshot(t *testing.T) {
	h, dbtx, evaluator, parsed, accounts := newPolicySnapshotHarness(t, "publishing")
	blockedTargets, err := h.evaluatePublishingRestrictions(context.Background(), "ws_1", parsed.Posts, accounts)
	if err != nil {
		t.Fatalf("draft policy snapshot: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/social-posts/post_1/publish", nil)
	rr := httptest.NewRecorder()
	h.publishExistingPost(rr, req, "ws_1", dbtx.post, parsed, accounts, blockedTargets)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("publishExistingPost status=%d body=%s", rr.Code, rr.Body.String())
	}
	assertMixedPolicySnapshotPersistence(t, dbtx, evaluator)
}

func TestScheduledEnqueueUsesExecutionPolicySnapshot(t *testing.T) {
	h, dbtx, evaluator, _, _ := newPolicySnapshotHarness(t, "scheduled")
	if err := h.EnqueueScheduledPost(context.Background(), dbtx.post); err != nil {
		t.Fatalf("EnqueueScheduledPost: %v", err)
	}
	assertMixedPolicySnapshotPersistence(t, dbtx, evaluator)
}

func TestDisconnectedAccountFailureTakesPrecedenceOverRestrictionSnapshot(t *testing.T) {
	h, dbtx, _, parsed, accounts := newPolicySnapshotHarness(t, "publishing")
	disconnected := dbtx.accounts["tk_1"]
	disconnected.DisconnectedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	dbtx.accounts["tk_1"] = disconnected
	parsed.Posts[0].MediaIDs = []string{"media_tk"}
	parsed.Posts[1].MediaIDs = []string{"media_ig"}
	metadata, err := platform.EncodePostMetadata(parsed.Posts)
	if err != nil {
		t.Fatal(err)
	}
	dbtx.post.Metadata = metadata
	blockedTargets := map[string]publishingrestrictions.Decision{
		"tk_1": {Restricted: true, Platform: "tiktok", CycleID: "cycle_1"},
	}

	_, err = h.enqueueExistingPostDeliveries(context.Background(), dbtx.post, parsed.Posts, accounts, blockedTargets)
	if err != nil {
		t.Fatalf("enqueueExistingPostDeliveries: %v", err)
	}
	if len(dbtx.failureDetails) == 0 {
		t.Fatal("missing persisted result failure details")
	}
	details := dbtx.failureDetails[len(dbtx.failureDetails)-1]
	if details.ErrorCode.String != "account_reconnect_required" || details.FailureStage.String != "dispatch_prepare" || details.NextAction.String != "reconnect_account" {
		t.Fatalf("disconnected failure details=%+v", details)
	}
	if len(dbtx.failures) != 1 || dbtx.failures[0].RestrictionCycleID.Valid {
		t.Fatalf("failures=%+v, want account failure without restriction cycle", dbtx.failures)
	}
	if len(dbtx.jobs) != 1 || dbtx.jobs[0].SocialAccountID != "ig_1" {
		t.Fatalf("jobs=%+v, want one allowed instagram job", dbtx.jobs)
	}
	for _, reason := range dbtx.retentionReasons {
		if reason == "publishing_restriction" {
			t.Fatalf("restriction retention applied to disconnected account: %v", dbtx.retentionReasons)
		}
	}
}

func TestCreatePassesAdmissionPolicySnapshotToPersistence(t *testing.T) {
	h, dbtx, evaluator, _, _ := newPolicySnapshotHarness(t, "publishing")
	h.quota = quota.NewChecker(h.queries)
	req := httptest.NewRequest(http.MethodPost, "/v1/social-posts", strings.NewReader(`{
		"platform_posts":[
			{"account_id":"tk_1","caption":"blocked","media_urls":["https://cdn.example.com/video.mp4"],"platform_options":{"privacy_level":"PUBLIC_TO_EVERYONE"}},
			{"account_id":"ig_1","caption":"allowed","media_urls":["https://cdn.example.com/image.jpg"]}
		]
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("Create status=%d body=%s", rr.Code, rr.Body.String())
	}
	assertMixedPolicySnapshotPersistence(t, dbtx, evaluator)
}

func TestPublishDraftPassesAdmissionPolicySnapshotToPersistence(t *testing.T) {
	h, dbtx, evaluator, _, _ := newPolicySnapshotHarness(t, "draft")
	h.quota = quota.NewChecker(h.queries)
	req := httptest.NewRequest(http.MethodPost, "/v1/social-posts/post_1/publish", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = withChiParam(req, "id", "post_1")
	rr := httptest.NewRecorder()

	h.PublishDraft(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("PublishDraft status=%d body=%s", rr.Code, rr.Body.String())
	}
	assertMixedPolicySnapshotPersistence(t, dbtx, evaluator)
}

func TestPublishDraftCountsOnlyAllowedTargetsAtQuotaBoundary(t *testing.T) {
	h, dbtx, evaluator, _, _ := newPolicySnapshotHarness(t, "draft")
	dbtx.quotaEnabled = true
	dbtx.quotaUsage = 99
	dbtx.quotaLimit = 100
	h.quota = quota.NewChecker(h.queries)
	req := httptest.NewRequest(http.MethodPost, "/v1/social-posts/post_1/publish", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = withChiParam(req, "id", "post_1")
	rr := httptest.NewRecorder()

	h.PublishDraft(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("PublishDraft status=%d body=%s", rr.Code, rr.Body.String())
	}
	assertMixedPolicySnapshotPersistence(t, dbtx, evaluator)
}

func newPolicySnapshotHarness(t *testing.T, status string) (*SocialPostHandler, *policySnapshotDB, *fakePostRestrictionEvaluator, parsedRequest, map[string]platform.ValidateAccount) {
	t.Helper()
	posts := []platform.PlatformPostInput{
		{AccountID: "tk_1", Caption: "blocked", MediaURLs: []string{"https://cdn.example.com/video.mp4"}, PlatformOptions: map[string]any{"privacy_level": "PUBLIC_TO_EVERYONE"}},
		{AccountID: "ig_1", Caption: "allowed", MediaURLs: []string{"https://cdn.example.com/image.jpg"}},
	}
	metadata, err := platform.EncodePostMetadata(posts)
	if err != nil {
		t.Fatal(err)
	}
	dbtx := &policySnapshotDB{
		accounts: map[string]db.SocialAccount{
			"tk_1": {ID: "tk_1", ProfileID: "profile_1", Platform: "tiktok", Status: "active"},
			"ig_1": {ID: "ig_1", ProfileID: "profile_1", Platform: "instagram", Status: "active"},
		},
		post: db.SocialPost{
			ID:          "post_1",
			WorkspaceID: "ws_1",
			Status:      status,
			Metadata:    metadata,
			CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
			ProfileIds:  []string{"profile_1"},
		},
	}
	evaluator := &fakePostRestrictionEvaluator{
		decisions: map[string]publishingrestrictions.Decision{
			"tiktok": {Restricted: true, Platform: "tiktok", PlanID: "free", CycleID: "cycle_1", Code: publishingrestrictions.NormalizedCode},
		},
		err:       errors.New("unexpected second policy read"),
		errOnCall: 3,
	}
	h := NewSocialPostHandler(db.New(dbtx), nil, nil, nil, nil, nil, nil).SetPublishingRestrictions(evaluator)
	accounts := map[string]platform.ValidateAccount{
		"tk_1": {Platform: "tiktok"},
		"ig_1": {Platform: "instagram"},
	}
	return h, dbtx, evaluator, parsedRequest{Posts: posts}, accounts
}

func assertMixedPolicySnapshotPersistence(t *testing.T, dbtx *policySnapshotDB, evaluator *fakePostRestrictionEvaluator) {
	t.Helper()
	if got := strings.Join(evaluator.calls, ","); got != "tiktok,instagram" {
		t.Fatalf("policy calls=%q, want one tiktok/instagram snapshot", got)
	}
	if len(dbtx.results) != 2 {
		t.Fatalf("results=%+v, want two", dbtx.results)
	}
	if dbtx.results[0].SocialAccountID != "tk_1" || dbtx.results[0].Status != "failed" ||
		!dbtx.results[0].ErrorMessage.Valid || dbtx.results[0].ErrorMessage.String != publishingrestrictions.UserMessage {
		t.Fatalf("restricted tiktok result=%+v", dbtx.results[0])
	}
	if len(dbtx.jobs) != 1 || dbtx.jobs[0].SocialAccountID != "ig_1" || dbtx.jobs[0].Platform != "instagram" {
		t.Fatalf("jobs=%+v, want one instagram job", dbtx.jobs)
	}
	if dbtx.updatedStatus != "publishing" {
		t.Fatalf("parent status=%q, want publishing with an allowed job", dbtx.updatedStatus)
	}
	if len(dbtx.failures) != 1 {
		t.Fatalf("failures=%+v, want one restriction failure", dbtx.failures)
	}
	failure := dbtx.failures[0]
	if failure.ErrorCode != publishingrestrictions.NormalizedCode || failure.FailureStage != publishingrestrictions.FailureStage ||
		!failure.RestrictionCycleID.Valid || failure.RestrictionCycleID.String != "cycle_1" {
		t.Fatalf("restriction failure=%+v", failure)
	}
}

type policySnapshotDB struct {
	accounts         map[string]db.SocialAccount
	post             db.SocialPost
	results          []db.SocialPostResult
	jobs             []db.PostDeliveryJob
	failures         []db.CreatePostFailureParams
	failureDetails   []db.UpdateSocialPostResultFailureDetailsParams
	retentionReasons []string
	updatedStatus    string
	quotaEnabled     bool
	quotaUsage       int32
	quotaLimit       int32
}

func (f *policySnapshotDB) Exec(_ context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(query, "-- name: UpdateSocialPostStatus"):
		f.updatedStatus = args[1].(string)
		f.post.Status = f.updatedStatus
		return pgconn.CommandTag{}, nil
	case strings.Contains(query, "-- name: UpdateSocialPostResultFailureDetails"):
		f.failureDetails = append(f.failureDetails, db.UpdateSocialPostResultFailureDetailsParams{
			ID:                args[0].(string),
			ErrorCode:         args[1].(pgtype.Text),
			FailureStage:      args[2].(pgtype.Text),
			PlatformErrorCode: args[3].(pgtype.Text),
			IsRetriable:       args[4].(pgtype.Bool),
			NextAction:        args[5].(pgtype.Text),
			ErrorSource:       args[6].(pgtype.Text),
			ErrorTemporality:  args[7].(pgtype.Text),
			ProviderError:     args[8].([]byte),
		})
		return pgconn.CommandTag{}, nil
	case strings.Contains(query, "-- name: DeleteMediaPostUsagesForPost"),
		strings.Contains(query, "-- name: UpdateSocialPostErrorMetadata"),
		strings.Contains(query, "-- name: MarkSocialAccountReconnectRequired"):
		return pgconn.CommandTag{}, nil
	default:
		return pgconn.CommandTag{}, fmt.Errorf("unexpected Exec: %s", query)
	}
}

func (f *policySnapshotDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	switch {
	case strings.Contains(query, "-- name: GetDistinctProfileIDsForAccounts"):
		return &policySnapshotRows{}, nil
	case strings.Contains(query, "-- name: ListSocialAccountsByWorkspace"):
		return &policySnapshotRows{values: [][]any{
			policySnapshotSocialAccountValues(f.accounts["tk_1"]),
			policySnapshotSocialAccountValues(f.accounts["ig_1"]),
		}}, nil
	}
	return nil, fmt.Errorf("unexpected Query: %s", query)
}

func (f *policySnapshotDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: CreateSocialPost :one"):
		f.post.Caption = args[1].(pgtype.Text)
		f.post.MediaUrls = args[2].([]string)
		f.post.Status = args[3].(string)
		f.post.Metadata = args[4].([]byte)
		return socialPostScanRow(f.post)
	case strings.Contains(query, "-- name: ClaimDraftForPublish"):
		f.post.Status = "publishing"
		return socialPostScanRow(f.post)
	case strings.Contains(query, "-- name: GetSocialAccountByIDAndWorkspace"):
		account, ok := f.accounts[args[0].(string)]
		if !ok {
			return scanRow{err: pgx.ErrNoRows}
		}
		return policySnapshotSocialAccountRow(account)
	case strings.Contains(query, "-- name: CreateSocialPostResult"):
		result := db.SocialPostResult{
			ID:              fmt.Sprintf("result_%d", len(f.results)+1),
			PostID:          args[0].(string),
			SocialAccountID: args[1].(string),
			Caption:         args[2].(string),
			Status:          args[3].(string),
			ExternalID:      args[4].(pgtype.Text),
			ErrorMessage:    args[5].(pgtype.Text),
			PublishedAt:     args[6].(pgtype.Timestamptz),
			Url:             args[7].(pgtype.Text),
			DebugCurl:       args[8].(pgtype.Text),
			FbMediaType:     args[9].(pgtype.Text),
		}
		f.results = append(f.results, result)
		return socialPostResultScanRow(result)
	case strings.Contains(query, "-- name: CreatePostDeliveryJob"):
		job := db.PostDeliveryJob{
			ID:                 fmt.Sprintf("job_%d", len(f.jobs)+1),
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
			CreatedAt:          pgtype.Timestamptz{Time: time.Now(), Valid: true},
		}
		f.jobs = append(f.jobs, job)
		return postDeliveryJobScanRow(job)
	case strings.Contains(query, "-- name: GetPostPublishingRestrictionMediaRetention"):
		return scanRow{values: []any{pgtype.Timestamptz{}}}
	case strings.Contains(query, "-- name: UpsertMediaPostUsage"):
		f.retentionReasons = append(f.retentionReasons, args[4].(string))
		return scanRow{values: []any{true}}
	case strings.Contains(query, "-- name: CreatePostFailure"):
		failure := db.CreatePostFailureParams{
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
		f.failures = append(f.failures, failure)
		return policySnapshotPostFailureRow(failure)
	case strings.Contains(query, "-- name: GetSubscriptionByWorkspace"):
		if !f.quotaEnabled {
			return scanRow{err: pgx.ErrNoRows}
		}
		return scanRow{values: []any{
			"sub_1", "free", pgtype.Text{}, pgtype.Text{}, "active",
			pgtype.Timestamptz{}, pgtype.Timestamptz{}, pgtype.Bool{},
			pgtype.Timestamptz{}, pgtype.Timestamptz{}, false, "ws_1",
		}}
	case strings.Contains(query, "-- name: GetPlan"):
		if !f.quotaEnabled {
			return scanRow{err: pgx.ErrNoRows}
		}
		return scanRow{values: []any{
			"free", "Free", int32(0), f.quotaLimit, pgtype.Text{},
			pgtype.Timestamptz{}, false, false, false, false,
			pgtype.Int4{}, pgtype.Int4{},
		}}
	case strings.Contains(query, "-- name: GetUsage"):
		if !f.quotaEnabled {
			return scanRow{err: pgx.ErrNoRows}
		}
		return scanRow{values: []any{
			"usage_1", args[1].(string), f.quotaUsage,
			pgtype.Timestamptz{}, pgtype.Timestamptz{}, "ws_1",
		}}
	case strings.Contains(query, "-- name: CountScheduledQuotaUnitsByWorkspaceAndPeriod"):
		return scanRow{values: []any{int32(0)}}
	default:
		return scanRow{err: fmt.Errorf("unexpected QueryRow: %s", query)}
	}
}

func policySnapshotSocialAccountRow(account db.SocialAccount) scanRow {
	return scanRow{values: policySnapshotSocialAccountValues(account)}
}

func policySnapshotSocialAccountValues(account db.SocialAccount) []any {
	return []any{
		account.ID, account.ProfileID, account.Platform, account.AccessToken, account.RefreshToken,
		account.TokenExpiresAt, account.ExternalAccountID, account.AccountName, account.AccountAvatarUrl,
		account.ConnectedAt, account.DisconnectedAt, account.Metadata, account.Scope, account.Status,
		account.ConnectionType, account.ConnectSessionID, account.ExternalUserID, account.ExternalUserEmail,
		account.LastRefreshedAt, account.XAppMode,
	}
}

func policySnapshotPostFailureRow(failure db.CreatePostFailureParams) scanRow {
	return scanRow{values: []any{
		"failure_1", failure.PostID, failure.SocialPostResultID, failure.WorkspaceID,
		failure.SocialAccountID, failure.Platform, failure.FailureStage, failure.ErrorCode,
		failure.PlatformErrorCode, failure.Message, failure.RawError, failure.IsRetriable,
		pgtype.Timestamptz{Time: time.Now(), Valid: true}, failure.ErrorSource,
		failure.ErrorTemporality, failure.ProviderError, failure.RestrictionCycleID,
	}}
}

type policySnapshotRows struct {
	values [][]any
	index  int
}

func (*policySnapshotRows) Close()                                       {}
func (*policySnapshotRows) Err() error                                   { return nil }
func (*policySnapshotRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (*policySnapshotRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *policySnapshotRows) Next() bool {
	if r.index >= len(r.values) {
		return false
	}
	r.index++
	return true
}
func (r *policySnapshotRows) Scan(dest ...any) error {
	if r.index == 0 || r.index > len(r.values) {
		return errors.New("no current row")
	}
	return scanRow{values: r.values[r.index-1]}.Scan(dest...)
}
func (r *policySnapshotRows) Values() ([]any, error) {
	if r.index == 0 || r.index > len(r.values) {
		return nil, errors.New("no current row")
	}
	return r.values[r.index-1], nil
}
func (*policySnapshotRows) RawValues() [][]byte { return nil }
func (*policySnapshotRows) Conn() *pgx.Conn     { return nil }

func TestEvaluatePublishingRestrictionsUsesTrustedAccountPlatforms(t *testing.T) {
	evaluator := &fakePostRestrictionEvaluator{decisions: map[string]publishingrestrictions.Decision{
		"tiktok": {Restricted: true, Platform: "tiktok", PlanID: "free", Code: publishingrestrictions.NormalizedCode},
	}}
	h := &SocialPostHandler{publishingRestrictions: evaluator}
	posts := []platform.PlatformPostInput{{AccountID: "tk_1"}, {AccountID: "ig_1"}}
	accounts := map[string]platform.ValidateAccount{
		"tk_1": {Platform: "tiktok"},
		"ig_1": {Platform: "instagram"},
	}
	blocked, err := h.evaluatePublishingRestrictions(context.Background(), "ws_1", posts, accounts)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocked) != 1 || !blocked["tk_1"].Restricted {
		t.Fatalf("blocked=%+v", blocked)
	}
	if strings.Join(evaluator.calls, ",") != "tiktok,instagram" {
		t.Fatalf("calls=%v", evaluator.calls)
	}
}

func TestEvaluatePublishingRestrictionsPropagatesReadFailure(t *testing.T) {
	h := &SocialPostHandler{publishingRestrictions: &fakePostRestrictionEvaluator{err: errors.New("database down")}}
	_, err := h.evaluatePublishingRestrictions(context.Background(), "ws_1", []platform.PlatformPostInput{{AccountID: "tk_1"}}, map[string]platform.ValidateAccount{"tk_1": {Platform: "tiktok"}})
	if err == nil {
		t.Fatal("expected policy read error")
	}
}

func TestQueuedPolicyPreflightPrecedesEveryResultAndJobWrite(t *testing.T) {
	source, err := os.ReadFile("social_post_queue.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "func (h *SocialPostHandler) enqueueParsedPostDeliveries")
	end := strings.Index(text[start:], "func (h *SocialPostHandler) queueImmediatePost")
	if start < 0 || end < 0 {
		t.Fatal("enqueueParsedPostDeliveries boundaries not found")
	}
	fn := text[start : start+end]
	preflight := strings.Index(fn, "evaluateQueuedDeliveryTargets")
	firstResultWrite := strings.Index(fn, "CreateSocialPostResult")
	firstJobWrite := strings.Index(fn, "CreatePostDeliveryJob")
	if preflight < 0 || firstResultWrite < 0 || firstJobWrite < 0 ||
		preflight > firstResultWrite || preflight > firstJobWrite {
		t.Fatalf("all target policies must be read before persistence: preflight=%d result=%d job=%d", preflight, firstResultWrite, firstJobWrite)
	}
	if strings.Count(fn, "publishingRestrictions.Evaluate") != 0 {
		t.Fatal("persistence loop must not perform policy reads")
	}
}

func TestQueuedPolicyEvaluationUsesProvidedSnapshot(t *testing.T) {
	parsed := []platform.PlatformPostInput{{AccountID: "tk_1"}, {AccountID: "ig_1"}}
	accounts := map[string]db.SocialAccount{
		"tk_1": {ID: "tk_1", Platform: "tiktok", Status: "active"},
		"ig_1": {ID: "ig_1", Platform: "instagram", Status: "active"},
	}
	evaluations := evaluateQueuedDeliveryTargets(
		parsed,
		accounts,
		map[string]platform.ValidateAccount{},
		map[string]publishingrestrictions.Decision{
			"tk_1": {Restricted: true, Platform: "tiktok"},
		},
	)
	if len(evaluations) != 2 || !evaluations[0].policyDecision.Restricted || evaluations[0].validationErr == nil {
		t.Fatalf("evaluations=%+v", evaluations)
	}
	if evaluations[1].policyDecision.Restricted || evaluations[1].validationErr != nil {
		t.Fatalf("allowed evaluation=%+v", evaluations[1])
	}
}

func TestFullyRestrictedDecisionHandlesMultipleInputsForOneAccount(t *testing.T) {
	decision := publishingrestrictions.Decision{Restricted: true, Platform: "tiktok", PlanID: "free"}
	posts := []platform.PlatformPostInput{
		{AccountID: "tk_1", ThreadPosition: 1},
		{AccountID: "tk_1", ThreadPosition: 2},
	}

	got, blocked := fullyRestrictedDecision(posts, map[string]publishingrestrictions.Decision{"tk_1": decision})
	if !blocked || got.Platform != "tiktok" {
		t.Fatalf("decision=%+v blocked=%v", got, blocked)
	}

	if _, blocked := fullyRestrictedDecision(
		append(posts, platform.PlatformPostInput{AccountID: "ig_1"}),
		map[string]publishingrestrictions.Decision{"tk_1": decision},
	); blocked {
		t.Fatal("mixed-platform inputs must not be treated as fully restricted")
	}
}

func TestAllowedPublishingTargetsExcludeRestrictedAccounts(t *testing.T) {
	posts := []platform.PlatformPostInput{
		{AccountID: "tk_1"},
		{AccountID: "ig_1"},
		{AccountID: "tk_1", ThreadPosition: 2},
	}
	allowed := allowedPublishingTargets(posts, map[string]publishingrestrictions.Decision{
		"tk_1": {Restricted: true},
	})
	if len(allowed) != 1 || allowed[0].AccountID != "ig_1" {
		t.Fatalf("allowed=%+v", allowed)
	}
}

func TestBulkAndScheduledQuotaPartitionPolicyBeforeQuota(t *testing.T) {
	bulkSource, err := os.ReadFile("social_posts_bulk.go")
	if err != nil {
		t.Fatal(err)
	}
	bulk := string(bulkSource)
	bulkStart := strings.Index(bulk, "func (h *SocialPostHandler) processBulkOne")
	if bulkStart < 0 {
		t.Fatal("processBulkOne missing")
	}
	bulkFn := bulk[bulkStart:]
	if policy, quota := strings.Index(bulkFn, "evaluatePublishingRestrictions"), strings.Index(bulkFn, "quotaGate.Blocked"); policy < 0 || quota < 0 || policy > quota {
		t.Fatalf("bulk policy must precede quota: policy=%d quota=%d", policy, quota)
	}

	queueSource, err := os.ReadFile("social_post_queue.go")
	if err != nil {
		t.Fatal(err)
	}
	queue := string(queueSource)
	start := strings.Index(queue, "func (h *SocialPostHandler) EnqueueScheduledPost")
	end := strings.Index(queue[start:], "func (h *SocialPostHandler) failScheduledPostForQuota")
	if start < 0 || end < 0 {
		t.Fatal("scheduled enqueue boundaries missing")
	}
	fn := queue[start : start+end]
	if policy, quota := strings.Index(fn, "evaluatePublishingRestrictions"), strings.Index(fn, "checkFreePlanPostQuota"); policy < 0 || quota < 0 || policy > quota {
		t.Fatalf("scheduled policy must precede quota: policy=%d quota=%d", policy, quota)
	}
}

func TestWritePublishingRestrictionErrorExactContract(t *testing.T) {
	rec := httptest.NewRecorder()
	writePublishingRestrictionError(rec, publishingrestrictions.Decision{Restricted: true, Platform: "tiktok", PlanID: "free", CycleID: "internal_cycle"})
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, fragment := range []string{
		`"code":"PLAN_PLATFORM_PUBLISHING_RESTRICTED"`,
		`"normalized_code":"plan_platform_publishing_restricted"`,
		`"next_action":"upgrade_or_wait_then_retry"`,
		`"is_retriable":false`,
		publishingrestrictions.UserMessage,
	} {
		if !strings.Contains(body, fragment) {
			t.Errorf("body missing %q: %s", fragment, body)
		}
	}
	if strings.Contains(body, "internal_cycle") || strings.Contains(body, "cycle_id") {
		t.Fatalf("cycle leaked: %s", body)
	}
}

func TestPublishingRestrictionFailurePersistsOnlySafeInternalCorrelation(t *testing.T) {
	failure := publishingRestrictionFailure("post_1", "result_1", "ws_1", "sa_1", "tiktok", "cycle_1")
	if failure.ProviderError != nil {
		t.Fatalf("provider_error=%s, want SQL NULL because no provider was called", failure.ProviderError)
	}
	if failure.RestrictionCycleID.String != "cycle_1" || !failure.RestrictionCycleID.Valid {
		t.Fatalf("restriction cycle=%+v", failure.RestrictionCycleID)
	}

	source, err := os.ReadFile("../db/queries/social_post_results.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "publish_token = CASE") || !strings.Contains(string(source), "plan_platform_publishing_restricted") {
		t.Fatalf("policy failure update must clear stale publish tokens:\n%s", source)
	}
}
