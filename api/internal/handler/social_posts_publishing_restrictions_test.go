package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
)

type fakePostRestrictionEvaluator struct {
	decisions map[string]publishingrestrictions.Decision
	err       error
	calls     []string
}

func (f *fakePostRestrictionEvaluator) Evaluate(_ context.Context, _ string, platformName string) (publishingrestrictions.Decision, error) {
	f.calls = append(f.calls, platformName)
	if f.err != nil {
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
