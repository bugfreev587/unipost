package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
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

func TestQueuedPolicyPreflightAbortsWhenLaterTargetReadFails(t *testing.T) {
	evaluator := &fakePostRestrictionEvaluator{err: errors.New("policy database unavailable"), errOnCall: 2}
	h := &SocialPostHandler{publishingRestrictions: evaluator}
	parsed := []platform.PlatformPostInput{{AccountID: "tk_1"}, {AccountID: "ig_1"}}
	accounts := map[string]db.SocialAccount{
		"tk_1": {ID: "tk_1", Platform: "tiktok", Status: "active"},
		"ig_1": {ID: "ig_1", Platform: "instagram", Status: "active"},
	}
	evaluations, err := h.evaluateQueuedDeliveryTargets(
		context.Background(),
		"ws_1",
		parsed,
		accounts,
		map[string]platform.ValidateAccount{},
	)
	if err == nil || evaluations != nil {
		t.Fatalf("later policy read must abort the whole preflight: evaluations=%+v err=%v", evaluations, err)
	}
	if strings.Join(evaluator.calls, ",") != "tiktok,instagram" {
		t.Fatalf("policy calls=%v", evaluator.calls)
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
