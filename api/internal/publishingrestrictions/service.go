package publishingrestrictions

import (
	"context"
	"strings"
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
	store Store
}

func NewService(store Store) *Service { return &Service{store: store} }

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
