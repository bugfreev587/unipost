package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
)

type publishingRestrictionsService interface {
	WorkspaceProjection(context.Context, string) ([]publishingrestrictions.Decision, error)
	ListAdmin(context.Context) ([]publishingrestrictions.Restriction, error)
	SetEnabled(context.Context, publishingrestrictions.TransitionRequest) (publishingrestrictions.TransitionResult, error)
}

type PublishingRestrictionsHandler struct {
	service publishingRestrictionsService
}

type publishingRestrictionCampaignService interface {
	PreviewCampaign(context.Context, string, publishingrestrictions.CampaignType) (publishingrestrictions.CampaignPreview, error)
	ConfirmCampaign(context.Context, publishingrestrictions.ConfirmCampaignRequest) (publishingrestrictions.Campaign, error)
	ListCampaigns(context.Context, string) ([]publishingrestrictions.Campaign, error)
	RetryFailedCampaign(context.Context, string, string) (publishingrestrictions.Campaign, error)
}

func NewPublishingRestrictionsHandler(service publishingRestrictionsService) *PublishingRestrictionsHandler {
	return &PublishingRestrictionsHandler{service: service}
}

func (h *PublishingRestrictionsHandler) WorkspaceProjection(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Publishing restrictions are not configured")
		return
	}
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	decisions, err := h.service.WorkspaceProjection(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "POLICY_UNAVAILABLE", "Publishing policy is temporarily unavailable")
		return
	}
	writeSuccess(w, map[string]any{"restrictions": decisions})
}

func (h *PublishingRestrictionsHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Publishing restrictions are not configured")
		return
	}
	restrictions, err := h.service.ListAdmin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load publishing restrictions")
		return
	}
	writeSuccess(w, restrictions)
}

func (h *PublishingRestrictionsHandler) AdminSet(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Publishing restrictions are not configured")
		return
	}
	platform := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "platform")))
	if platform == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "platform is required")
		return
	}
	var body struct {
		Enabled         *bool `json:"enabled"`
		ExpectedVersion int64 `json:"expected_version"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil || body.Enabled == nil || body.ExpectedVersion <= 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "enabled and a positive expected_version are required")
		return
	}
	actor := auth.GetUserID(r.Context())
	if actor == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing admin user")
		return
	}
	requestID := r.Header.Get("X-Request-Id")
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	result, err := h.service.SetEnabled(r.Context(), publishingrestrictions.TransitionRequest{
		Platform: platform, Enabled: *body.Enabled, ExpectedVersion: body.ExpectedVersion,
		ActorUserID: actor, RequestID: requestID, ActorIP: ip, ActorUserAgent: r.UserAgent(),
	})
	var conflict *publishingrestrictions.VersionConflictError
	if errors.As(err, &conflict) {
		writeErrorWithDetails(w, http.StatusConflict, "VERSION_CONFLICT", "Publishing restriction changed; reload and confirm again", ErrorDetails{
			Details: map[string]any{"current": conflict.Current},
		})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update publishing restriction")
		return
	}
	writeSuccess(w, result)
}

func (h *PublishingRestrictionsHandler) campaignService() (publishingRestrictionCampaignService, bool) {
	service, ok := h.service.(publishingRestrictionCampaignService)
	return service, ok
}

func (h *PublishingRestrictionsHandler) AdminPreviewCampaign(w http.ResponseWriter, r *http.Request) {
	service, ok := h.campaignService()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Publishing restriction email campaigns are not configured")
		return
	}
	var body struct {
		CampaignType publishingrestrictions.CampaignType `json:"campaign_type"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "campaign_type is required")
		return
	}
	preview, err := service.PreviewCampaign(r.Context(), strings.ToLower(strings.TrimSpace(chi.URLParam(r, "platform"))), body.CampaignType)
	if errors.Is(err, publishingrestrictions.ErrCampaignNotConfigured) {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Publishing restriction email campaigns are not configured")
		return
	}
	if errors.Is(err, publishingrestrictions.ErrCampaignPrecondition) {
		writeError(w, http.StatusConflict, "CAMPAIGN_NOT_AVAILABLE", "This campaign is not available in the current restriction state")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "CAMPAIGN_PREVIEW_UNAVAILABLE", "Campaign preview is temporarily unavailable")
		return
	}
	writeSuccess(w, preview)
}

func (h *PublishingRestrictionsHandler) AdminCreateCampaign(w http.ResponseWriter, r *http.Request) {
	service, ok := h.campaignService()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Publishing restriction email campaigns are not configured")
		return
	}
	var body struct {
		CampaignType publishingrestrictions.CampaignType `json:"campaign_type"`
		PreviewToken string                              `json:"preview_token"`
		Confirmation string                              `json:"confirmation"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "campaign_type, preview_token, and confirmation are required")
		return
	}
	campaign, err := service.ConfirmCampaign(r.Context(), publishingrestrictions.ConfirmCampaignRequest{
		Platform: strings.ToLower(strings.TrimSpace(chi.URLParam(r, "platform"))), CampaignType: body.CampaignType,
		PreviewToken: body.PreviewToken, Confirmation: body.Confirmation, ActorUserID: auth.GetUserID(r.Context()),
	})
	if errors.Is(err, publishingrestrictions.ErrCampaignNotConfigured) {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Publishing restriction email campaigns are not configured")
		return
	}
	if errors.Is(err, publishingrestrictions.ErrInvalidPreviewToken) {
		writeError(w, http.StatusBadRequest, "INVALID_PREVIEW_TOKEN", "Campaign preview expired or no longer matches")
		return
	}
	if errors.Is(err, publishingrestrictions.ErrCampaignPrecondition) {
		writeError(w, http.StatusConflict, "CAMPAIGN_NOT_AVAILABLE", "This campaign is not available in the current restriction state")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create email campaign")
		return
	}
	writeSuccess(w, campaign)
}

func (h *PublishingRestrictionsHandler) AdminListCampaigns(w http.ResponseWriter, r *http.Request) {
	service, ok := h.campaignService()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Publishing restriction email campaigns are not configured")
		return
	}
	campaigns, err := service.ListCampaigns(r.Context(), strings.ToLower(strings.TrimSpace(chi.URLParam(r, "platform"))))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list email campaigns")
		return
	}
	writeSuccess(w, campaigns)
}

func (h *PublishingRestrictionsHandler) AdminRetryFailedCampaign(w http.ResponseWriter, r *http.Request) {
	service, ok := h.campaignService()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Publishing restriction email campaigns are not configured")
		return
	}
	campaign, err := service.RetryFailedCampaign(r.Context(), strings.ToLower(strings.TrimSpace(chi.URLParam(r, "platform"))), chi.URLParam(r, "campaignID"))
	if errors.Is(err, publishingrestrictions.ErrCampaignNotConfigured) {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED", "Publishing restriction email campaigns are not configured")
		return
	}
	if errors.Is(err, publishingrestrictions.ErrCampaignRetryExpired) {
		writeError(w, http.StatusConflict, "RETRY_WINDOW_EXPIRED", "This campaign is older than the provider idempotency window and cannot be retried safely")
		return
	}
	if errors.Is(err, publishingrestrictions.ErrCampaignPrecondition) {
		writeError(w, http.StatusConflict, "NO_FAILED_RECIPIENTS", "This campaign has no failed recipients to retry")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retry campaign recipients")
		return
	}
	writeSuccess(w, campaign)
}
