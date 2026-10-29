package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/platformcontrols"
)

type fixedPlatformControlEvaluator struct {
	decision platformcontrols.Decision
	err      error
	calls    int
	platform string
	op       platformcontrols.Operation
}

func (e *fixedPlatformControlEvaluator) Evaluate(_ context.Context, platform string, operation platformcontrols.Operation) (platformcontrols.Decision, error) {
	e.calls++
	e.platform = platform
	e.op = operation
	return e.decision, e.err
}

func fullyDisabledDecision(platform string, operation platformcontrols.Operation) platformcontrols.Decision {
	return platformcontrols.Decision{
		Platform:   platform,
		State:      platformcontrols.StateFullyDisabled,
		Operation:  operation,
		Blocked:    true,
		ReasonCode: "security_incident",
		Message:    platform + " is temporarily unavailable during a security review.",
	}
}

var errOperationalControlUnavailable = errors.New("operational control unavailable")

func TestSocialAccountReadFullyDisabledReturnsBeforeProviderWork(t *testing.T) {
	evaluator := &fixedPlatformControlEvaluator{decision: fullyDisabledDecision("tiktok", platformcontrols.OperationRead)}
	h := (&SocialAccountHandler{}).SetPlatformOperationalControls(evaluator)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/tk_1/metrics", nil)

	if h.platformReadAllowed(rec, req, "tiktok") {
		t.Fatal("fully disabled platform was allowed")
	}
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), platformcontrols.APICode) {
		t.Fatalf("response=(%d, %q)", rec.Code, rec.Body.String())
	}
	if evaluator.calls != 1 || evaluator.op != platformcontrols.OperationRead {
		t.Fatalf("evaluation=%+v", evaluator)
	}
}

func TestSocialAccountReadControlFailureFailsClosed(t *testing.T) {
	h := (&SocialAccountHandler{}).SetPlatformOperationalControls(&fixedPlatformControlEvaluator{err: errOperationalControlUnavailable})
	rec := httptest.NewRecorder()
	if h.platformReadAllowed(rec, httptest.NewRequest(http.MethodGet, "/", nil), "tiktok") {
		t.Fatal("control-store failure was allowed")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
}

type fakePlatformControlsAdminService struct {
	fixedPlatformControlEvaluator
	controls  []platformcontrols.Control
	listErr   error
	setResult platformcontrols.TransitionResult
	setErr    error
	request   platformcontrols.TransitionRequest
}

func (s *fakePlatformControlsAdminService) List(context.Context) ([]platformcontrols.Control, error) {
	return s.controls, s.listErr
}

func (s *fakePlatformControlsAdminService) SetState(_ context.Context, request platformcontrols.TransitionRequest) (platformcontrols.TransitionResult, error) {
	s.request = request
	return s.setResult, s.setErr
}

func TestPlatformOperationalControlsAdminList(t *testing.T) {
	service := &fakePlatformControlsAdminService{controls: []platformcontrols.Control{{
		Platform: "tiktok", State: platformcontrols.StateFullyDisabled, Version: 2,
	}}}
	h := NewPlatformOperationalControlsHandler(service)
	rec := httptest.NewRecorder()
	h.AdminList(rec, httptest.NewRequest(http.MethodGet, "/v1/admin/platform-operational-controls", nil))

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"fully_disabled"`) {
		t.Fatalf("response=(%d, %q)", rec.Code, rec.Body.String())
	}
}

func TestPlatformOperationalControlsAdminSetCapturesAuditMetadata(t *testing.T) {
	service := &fakePlatformControlsAdminService{setResult: platformcontrols.TransitionResult{
		Control: platformcontrols.Control{Platform: "tiktok", State: platformcontrols.StateFullyDisabled, Version: 2},
		Changed: true,
	}}
	h := NewPlatformOperationalControlsHandler(service)
	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/platform-operational-controls/tiktok", strings.NewReader(`{
		"state":"fully_disabled",
		"expected_version":1,
		"reason_code":"security_incident",
		"user_message":"TikTok is temporarily unavailable."
	}`))
	req = withChiParam(req, "platform", "tiktok")
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_1"))
	req.Header.Set("User-Agent", "incident-console")
	rec := httptest.NewRecorder()
	h.AdminSet(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("response=(%d, %q)", rec.Code, rec.Body.String())
	}
	if service.request.Platform != "tiktok" || service.request.State != platformcontrols.StateFullyDisabled || service.request.ExpectedVersion != 1 || service.request.ActorUserID != "user_1" || service.request.ActorUserAgent != "incident-console" {
		encoded, _ := json.Marshal(service.request)
		t.Fatalf("request=%s", encoded)
	}
}

func TestPlatformOperationalControlsAdminSetVersionConflict(t *testing.T) {
	service := &fakePlatformControlsAdminService{setErr: &platformcontrols.VersionConflictError{Current: platformcontrols.Control{
		Platform: "tiktok", State: platformcontrols.StateActive, Version: 3,
	}}}
	h := NewPlatformOperationalControlsHandler(service)
	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/platform-operational-controls/tiktok", strings.NewReader(`{"state":"fully_disabled","expected_version":1}`))
	req = withChiParam(req, "platform", "tiktok")
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_1"))
	rec := httptest.NewRecorder()
	h.AdminSet(rec, req)

	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"version":3`) {
		t.Fatalf("response=(%d, %q)", rec.Code, rec.Body.String())
	}
}
