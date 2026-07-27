package publishingrestrictions

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Store interface {
	RestrictionForPlatform(context.Context, string) (Restriction, error)
	ListRestrictions(context.Context) ([]Restriction, error)
	ListAdminRestrictions(context.Context) ([]Restriction, error)
	WorkspacePlanID(context.Context, string) (string, error)
	SetEnabled(context.Context, TransitionRequest) (TransitionResult, error)
}

func (s *Service) WorkspaceProjection(ctx context.Context, workspaceID string) ([]Decision, error) {
	planID, err := s.store.WorkspacePlanID(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, err
	}
	planID = strings.ToLower(strings.TrimSpace(planID))
	if planID == "" {
		planID = "free"
	}
	restrictions, err := s.store.ListRestrictions(ctx)
	if err != nil {
		return nil, err
	}
	decisions := make([]Decision, 0, len(restrictions))
	for _, restriction := range restrictions {
		if !restriction.Enabled || !containsPlan(restriction.RestrictedPlanIDs, planID) {
			continue
		}
		decisions = append(decisions, Decision{
			Restricted: true,
			Platform:   restriction.Platform,
			PlanID:     planID,
			ReasonCode: restriction.ReasonCode,
			CycleID:    restriction.CycleID,
			Version:    restriction.Version,
			Code:       NormalizedCode,
			Message:    UserMessage,
			NextAction: NextAction,
		})
	}
	return decisions, nil
}

func (s *Service) ListAdmin(ctx context.Context) ([]Restriction, error) {
	return s.store.ListAdminRestrictions(ctx)
}

type Service struct {
	store         Store
	campaignStore CampaignStore
	previewSecret []byte
	readiness     CampaignDeliveryReadiness
	now           func() time.Time
}

type CampaignDeliveryReadiness struct {
	PreviewSecret       bool
	AuditedSender       bool
	RestrictionTemplate bool
	RecoveryTemplate    bool
}

func (r CampaignDeliveryReadiness) Ready() bool {
	return r.PreviewSecret && r.AuditedSender && r.RestrictionTemplate && r.RecoveryTemplate
}

type CampaignStore interface {
	PreviewCampaignRecipients(context.Context, Restriction, CampaignType) ([]RecipientSnapshot, error)
	CreateCampaign(context.Context, Restriction, CampaignType, CampaignCopy, int, string) (Campaign, error)
	ListCampaigns(context.Context, string) ([]Campaign, error)
	RetryFailedCampaign(context.Context, string, string) (Campaign, error)
}

var (
	ErrCampaignPrecondition  = errors.New("campaign precondition failed")
	ErrInvalidPreviewToken   = errors.New("invalid or expired campaign preview token")
	ErrCampaignNotConfigured = errors.New("campaign delivery is not configured")
)

func NewService(store Store) *Service {
	campaignStore, _ := store.(CampaignStore)
	return &Service{store: store, campaignStore: campaignStore, now: time.Now}
}

func (s *Service) SetCampaignPreviewSecret(secret string) *Service {
	s.previewSecret = []byte(strings.TrimSpace(secret))
	return s
}

func (s *Service) SetCampaignDeliveryReadiness(readiness CampaignDeliveryReadiness) *Service {
	s.readiness = readiness
	return s
}

func (s *Service) campaignDeliveryReady() bool {
	return s != nil && s.campaignStore != nil && len(s.previewSecret) > 0 && s.readiness.Ready()
}

func (s *Service) Evaluate(ctx context.Context, workspaceID, platform string) (Decision, error) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	restriction, err := s.store.RestrictionForPlatform(ctx, platform)
	if err != nil {
		return Decision{}, err
	}
	planID, err := s.store.WorkspacePlanID(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return Decision{}, err
	}
	planID = strings.ToLower(strings.TrimSpace(planID))
	if planID == "" {
		planID = "free"
	}
	decision := Decision{Platform: platform, PlanID: planID, ReasonCode: restriction.ReasonCode, CycleID: restriction.CycleID, Version: restriction.Version}
	if !restriction.Enabled || !containsPlan(restriction.RestrictedPlanIDs, planID) {
		return decision, nil
	}
	decision.Restricted = true
	decision.Code = NormalizedCode
	decision.Message = UserMessage
	decision.NextAction = NextAction
	return decision, nil
}

func (s *Service) SetEnabled(ctx context.Context, request TransitionRequest) (TransitionResult, error) {
	request.Platform = strings.ToLower(strings.TrimSpace(request.Platform))
	return s.store.SetEnabled(ctx, request)
}

func containsPlan(plans []string, planID string) bool {
	for _, plan := range plans {
		if strings.EqualFold(strings.TrimSpace(plan), planID) {
			return true
		}
	}
	return false
}

type campaignPreviewToken struct {
	Platform           string       `json:"platform"`
	CycleID            string       `json:"cycle_id"`
	CampaignType       CampaignType `json:"campaign_type"`
	RestrictionVersion int64        `json:"restriction_version"`
	RecipientCount     int          `json:"recipient_count"`
	ExpiresAtUnix      int64        `json:"expires_at"`
}

func (s *Service) PreviewCampaign(ctx context.Context, platform string, campaignType CampaignType) (CampaignPreview, error) {
	if !s.campaignDeliveryReady() {
		return CampaignPreview{}, ErrCampaignNotConfigured
	}
	if campaignType != RestrictionNotice && campaignType != RecoveryNotice {
		return CampaignPreview{}, fmt.Errorf("%w: unsupported campaign type", ErrCampaignPrecondition)
	}
	restriction, err := s.store.RestrictionForPlatform(ctx, strings.ToLower(strings.TrimSpace(platform)))
	if err != nil {
		return CampaignPreview{}, err
	}
	if restriction.CycleID == "" || (campaignType == RestrictionNotice && !restriction.Enabled) || (campaignType == RecoveryNotice && restriction.Enabled) {
		return CampaignPreview{}, ErrCampaignPrecondition
	}
	recipients, err := s.campaignStore.PreviewCampaignRecipients(ctx, restriction, campaignType)
	if err != nil {
		return CampaignPreview{}, err
	}
	expiresAt := s.now().UTC().Add(10 * time.Minute)
	payload := campaignPreviewToken{
		Platform: restriction.Platform, CycleID: restriction.CycleID, CampaignType: campaignType,
		RestrictionVersion: restriction.Version, RecipientCount: len(recipients), ExpiresAtUnix: expiresAt.Unix(),
	}
	token, err := s.signCampaignPreview(payload)
	if err != nil {
		return CampaignPreview{}, err
	}
	copy := CampaignCopyFor(campaignType)
	return CampaignPreview{
		Platform: restriction.Platform, CycleID: restriction.CycleID, CampaignType: campaignType,
		RestrictionVersion: restriction.Version, RecipientCount: len(recipients), Subject: copy.Subject,
		Body: copy.Body, PreviewToken: token, ExpiresAt: expiresAt,
	}, nil
}

func (s *Service) ConfirmCampaign(ctx context.Context, request ConfirmCampaignRequest) (Campaign, error) {
	if !s.campaignDeliveryReady() {
		return Campaign{}, ErrCampaignNotConfigured
	}
	if request.Confirmation != "SEND" {
		return Campaign{}, ErrCampaignPrecondition
	}
	payload, err := s.verifyCampaignPreview(request.PreviewToken)
	if err != nil {
		return Campaign{}, err
	}
	platform := strings.ToLower(strings.TrimSpace(request.Platform))
	if payload.Platform != platform || payload.CampaignType != request.CampaignType {
		return Campaign{}, ErrInvalidPreviewToken
	}
	restriction, err := s.store.RestrictionForPlatform(ctx, platform)
	if err != nil {
		return Campaign{}, err
	}
	if restriction.CycleID != payload.CycleID || restriction.Version != payload.RestrictionVersion ||
		(request.CampaignType == RestrictionNotice && !restriction.Enabled) ||
		(request.CampaignType == RecoveryNotice && restriction.Enabled) {
		return Campaign{}, ErrCampaignPrecondition
	}
	return s.campaignStore.CreateCampaign(ctx, restriction, request.CampaignType, CampaignCopyFor(request.CampaignType), payload.RecipientCount, request.ActorUserID)
}

func (s *Service) ListCampaigns(ctx context.Context, platform string) ([]Campaign, error) {
	if s.campaignStore == nil {
		return nil, errors.New("campaigns are not configured")
	}
	return s.campaignStore.ListCampaigns(ctx, strings.ToLower(strings.TrimSpace(platform)))
}

func (s *Service) RetryFailedCampaign(ctx context.Context, platform, campaignID string) (Campaign, error) {
	if !s.campaignDeliveryReady() {
		return Campaign{}, ErrCampaignNotConfigured
	}
	return s.campaignStore.RetryFailedCampaign(ctx, strings.ToLower(strings.TrimSpace(platform)), campaignID)
}

func (s *Service) signCampaignPreview(payload campaignPreviewToken) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, s.previewSecret)
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *Service) verifyCampaignPreview(token string) (campaignPreviewToken, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || len(s.previewSecret) == 0 {
		return campaignPreviewToken{}, ErrInvalidPreviewToken
	}
	mac := hmac.New(sha256.New, s.previewSecret)
	_, _ = mac.Write([]byte(parts[0]))
	want := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(got, want) {
		return campaignPreviewToken{}, ErrInvalidPreviewToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return campaignPreviewToken{}, ErrInvalidPreviewToken
	}
	var payload campaignPreviewToken
	if json.Unmarshal(raw, &payload) != nil || s.now().UTC().Unix() > payload.ExpiresAtUnix {
		return campaignPreviewToken{}, ErrInvalidPreviewToken
	}
	return payload, nil
}
