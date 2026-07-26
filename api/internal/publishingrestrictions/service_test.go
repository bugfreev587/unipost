package publishingrestrictions

import (
	"context"
	"errors"
	"testing"
)

type fakeStore struct {
	restriction    Restriction
	restrictions   []Restriction
	admin          []Restriction
	planID         string
	planErr        error
	restrictionErr error
	transition     TransitionResult
	transitionErr  error
}

func (s *fakeStore) ListRestrictions(context.Context) ([]Restriction, error) {
	return s.restrictions, nil
}

func (s *fakeStore) ListAdminRestrictions(context.Context) ([]Restriction, error) {
	return s.admin, nil
}

func (s *fakeStore) RestrictionForPlatform(context.Context, string) (Restriction, error) {
	return s.restriction, s.restrictionErr
}

func (s *fakeStore) WorkspacePlanID(context.Context, string) (string, error) {
	return s.planID, s.planErr
}

func (s *fakeStore) SetEnabled(context.Context, TransitionRequest) (TransitionResult, error) {
	return s.transition, s.transitionErr
}

func TestEvaluateRestrictsOnlyFreeTikTok(t *testing.T) {
	store := &fakeStore{
		restriction: Restriction{Platform: "tiktok", Enabled: true, RestrictedPlanIDs: []string{"free"}, ReasonCode: "platform_capacity_limit", CycleID: "cycle_1", Version: 4},
		planID:      "free",
	}
	decision, err := NewService(store).Evaluate(context.Background(), "ws_1", "tiktok")
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Restricted || decision.Code != NormalizedCode || decision.Message != UserMessage || decision.NextAction != NextAction {
		t.Fatalf("unexpected decision: %+v", decision)
	}
	if decision.CycleID != "cycle_1" || decision.Version != 4 {
		t.Fatalf("internal policy metadata missing: %+v", decision)
	}
}

func TestEvaluateBypassesPaidAndOtherPlatforms(t *testing.T) {
	tests := []struct {
		name        string
		planID      string
		platform    string
		restriction Restriction
	}{
		{name: "paid", planID: "team", platform: "tiktok", restriction: Restriction{Platform: "tiktok", Enabled: true, RestrictedPlanIDs: []string{"free"}}},
		{name: "other platform", planID: "free", platform: "instagram", restriction: Restriction{Platform: "instagram", Enabled: false, RestrictedPlanIDs: []string{"free"}}},
		{name: "disabled", planID: "free", platform: "tiktok", restriction: Restriction{Platform: "tiktok", Enabled: false, RestrictedPlanIDs: []string{"free"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := NewService(&fakeStore{restriction: tt.restriction, planID: tt.planID}).Evaluate(context.Background(), "ws_1", tt.platform)
			if err != nil {
				t.Fatal(err)
			}
			if decision.Restricted {
				t.Fatalf("unexpected restriction: %+v", decision)
			}
		})
	}
}

func TestEvaluateTreatsMissingSubscriptionAsFree(t *testing.T) {
	decision, err := NewService(&fakeStore{
		restriction: Restriction{Platform: "tiktok", Enabled: true, RestrictedPlanIDs: []string{"free"}},
	}).Evaluate(context.Background(), "ws_1", "tiktok")
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Restricted || decision.PlanID != "free" {
		t.Fatalf("decision=%+v", decision)
	}
}

func TestEvaluatePropagatesPolicyReadFailure(t *testing.T) {
	want := errors.New("database unavailable")
	_, err := NewService(&fakeStore{restrictionErr: want}).Evaluate(context.Background(), "ws_1", "tiktok")
	if !errors.Is(err, want) {
		t.Fatalf("err=%v, want %v", err, want)
	}
}

func TestSetEnabledDelegatesOptimisticTransition(t *testing.T) {
	want := TransitionResult{Restriction: Restriction{Platform: "tiktok", Enabled: true, Version: 3}, Changed: true}
	store := &fakeStore{transition: want}
	got, err := NewService(store).SetEnabled(context.Background(), TransitionRequest{
		Platform: "tiktok", Enabled: true, ExpectedVersion: 2, ActorUserID: "user_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Restriction.Version != 3 || !got.Changed {
		t.Fatalf("result=%+v", got)
	}
}

func TestWorkspaceProjectionReturnsOnlyApplicableRestrictions(t *testing.T) {
	store := &fakeStore{
		planID: "free",
		restrictions: []Restriction{
			{Platform: "tiktok", Enabled: true, RestrictedPlanIDs: []string{"free"}, ReasonCode: "platform_capacity_limit"},
			{Platform: "instagram", Enabled: false, RestrictedPlanIDs: []string{"free"}},
		},
	}
	got, err := NewService(store).WorkspaceProjection(context.Background(), "ws_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Platform != "tiktok" || !got[0].Restricted {
		t.Fatalf("projection=%+v", got)
	}
}

func TestListAdminReturnsAffectedCountsFromStore(t *testing.T) {
	store := &fakeStore{admin: []Restriction{{Platform: "tiktok", AffectedWorkspaces: 26, AffectedAccounts: 35}}}
	got, err := NewService(store).ListAdmin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].AffectedWorkspaces != 26 || got[0].AffectedAccounts != 35 {
		t.Fatalf("admin=%+v", got)
	}
}
