package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
)

type fakePublishingRestrictionService struct {
	projection  []publishingrestrictions.Decision
	admin       []publishingrestrictions.Restriction
	transition  publishingrestrictions.TransitionResult
	err         error
	campaignErr error
	request     publishingrestrictions.TransitionRequest
}

func (s *fakePublishingRestrictionService) PreviewCampaign(context.Context, string, publishingrestrictions.CampaignType) (publishingrestrictions.CampaignPreview, error) {
	return publishingrestrictions.CampaignPreview{}, s.campaignErr
}

func (s *fakePublishingRestrictionService) ConfirmCampaign(context.Context, publishingrestrictions.ConfirmCampaignRequest) (publishingrestrictions.Campaign, error) {
	return publishingrestrictions.Campaign{}, s.campaignErr
}

func (s *fakePublishingRestrictionService) ListCampaigns(context.Context, string) ([]publishingrestrictions.Campaign, error) {
	return nil, s.campaignErr
}

func (s *fakePublishingRestrictionService) RetryFailedCampaign(context.Context, string, string) (publishingrestrictions.Campaign, error) {
	return publishingrestrictions.Campaign{}, s.campaignErr
}

func (s *fakePublishingRestrictionService) WorkspaceProjection(context.Context, string) ([]publishingrestrictions.Decision, error) {
	return s.projection, s.err
}

func (s *fakePublishingRestrictionService) ListAdmin(context.Context) ([]publishingrestrictions.Restriction, error) {
	return s.admin, s.err
}

func (s *fakePublishingRestrictionService) SetEnabled(_ context.Context, request publishingrestrictions.TransitionRequest) (publishingrestrictions.TransitionResult, error) {
	s.request = request
	return s.transition, s.err
}

func TestPublishingRestrictionsWorkspaceProjection(t *testing.T) {
	service := &fakePublishingRestrictionService{projection: []publishingrestrictions.Decision{{
		Restricted: true, Platform: "tiktok", PlanID: "free", ReasonCode: "platform_capacity_limit",
		Code: publishingrestrictions.NormalizedCode, Message: publishingrestrictions.UserMessage, NextAction: publishingrestrictions.NextAction,
	}}}
	h := NewPublishingRestrictionsHandler(service)
	req := httptest.NewRequest(http.MethodGet, "/v1/publishing-restrictions", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()
	h.WorkspaceProjection(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "cycle_id") || !strings.Contains(rec.Body.String(), publishingrestrictions.UserMessage) {
		t.Fatalf("unsafe or incomplete projection: %s", rec.Body.String())
	}
}

func TestPublishingRestrictionsAdminList(t *testing.T) {
	service := &fakePublishingRestrictionService{admin: []publishingrestrictions.Restriction{{Platform: "tiktok", Enabled: false, Version: 1, RestrictedPlanIDs: []string{"free"}}}}
	rec := httptest.NewRecorder()
	NewPublishingRestrictionsHandler(service).AdminList(rec, httptest.NewRequest(http.MethodGet, "/v1/admin/publishing-restrictions", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPublishingRestrictionsAdminSetUsesExpectedVersion(t *testing.T) {
	service := &fakePublishingRestrictionService{transition: publishingrestrictions.TransitionResult{Restriction: publishingrestrictions.Restriction{Platform: "tiktok", Enabled: true, Version: 2}, Changed: true}}
	h := NewPublishingRestrictionsHandler(service)
	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/publishing-restrictions/tiktok", strings.NewReader(`{"enabled":true,"expected_version":1}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("platform", "tiktok")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, auth.UserIDKey, "user_1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.AdminSet(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !service.request.Enabled || service.request.ExpectedVersion != 1 || service.request.ActorUserID != "user_1" {
		t.Fatalf("request=%+v", service.request)
	}
}

func TestPublishingRestrictionsAdminSetReturnsCurrentOnVersionConflict(t *testing.T) {
	current := publishingrestrictions.Restriction{Platform: "tiktok", Enabled: false, Version: 3}
	service := &fakePublishingRestrictionService{err: &publishingrestrictions.VersionConflictError{Current: current}}
	h := NewPublishingRestrictionsHandler(service)
	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/publishing-restrictions/tiktok", strings.NewReader(`{"enabled":true,"expected_version":2}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("platform", "tiktok")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, auth.UserIDKey, "user_1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.AdminSet(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Error struct {
			Details map[string]json.RawMessage `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if _, ok := envelope.Error.Details["current"]; !ok {
		t.Fatalf("missing current state: %s", rec.Body.String())
	}
}

func TestPublishingRestrictionCampaignActionsMapNotConfigured(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
		run  func(*PublishingRestrictionsHandler, http.ResponseWriter, *http.Request)
	}{
		{name: "preview", path: "/v1/admin/publishing-restrictions/tiktok/email-campaigns/preview", body: `{"campaign_type":"restriction_notice"}`, run: (*PublishingRestrictionsHandler).AdminPreviewCampaign},
		{name: "create", path: "/v1/admin/publishing-restrictions/tiktok/email-campaigns", body: `{"campaign_type":"restriction_notice","preview_token":"token","confirmation":"SEND"}`, run: (*PublishingRestrictionsHandler).AdminCreateCampaign},
		{name: "retry", path: "/v1/admin/publishing-restrictions/tiktok/email-campaigns/campaign_1/retry-failed", run: (*PublishingRestrictionsHandler).AdminRetryFailedCampaign},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakePublishingRestrictionService{campaignErr: fmt.Errorf("delivery unavailable: %w", publishingrestrictions.ErrCampaignNotConfigured)}
			h := NewPublishingRestrictionsHandler(service)
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("platform", "tiktok")
			rctx.URLParams.Add("campaignID", "campaign_1")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			rec := httptest.NewRecorder()

			tt.run(h, rec, req)

			if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"code":"NOT_CONFIGURED"`) {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "Publishing restriction email campaigns are not configured") || strings.Contains(rec.Body.String(), "delivery unavailable") {
				t.Fatalf("response did not use safe not-configured copy: %s", rec.Body.String())
			}
		})
	}
}
