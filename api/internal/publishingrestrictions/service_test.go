package publishingrestrictions

import (
	"context"
	"errors"
	"testing"
)

type fakeStore struct {
	restriction      Restriction
	restrictions     []Restriction
	admin            []Restriction
	planID           string
	planErr          error
	restrictionErr   error
	transition       TransitionResult
	transitionErr    error
	recipients       []RecipientSnapshot
	campaign         Campaign
	restrictionCalls int
	previewCalls     int
	createCalls      int
	retryCalls       int
}

func (s *fakeStore) ListRestrictions(context.Context) ([]Restriction, error) {
	return s.restrictions, nil
}

func (s *fakeStore) ListAdminRestrictions(context.Context) ([]Restriction, error) {
	return s.admin, nil
}

func (s *fakeStore) RestrictionForPlatform(context.Context, string) (Restriction, error) {
	s.restrictionCalls++
	return s.restriction, s.restrictionErr
}

func (s *fakeStore) WorkspacePlanID(context.Context, string) (string, error) {
	return s.planID, s.planErr
}

func (s *fakeStore) SetEnabled(context.Context, TransitionRequest) (TransitionResult, error) {
	return s.transition, s.transitionErr
}

func (s *fakeStore) PreviewCampaignRecipients(context.Context, Restriction, CampaignType) ([]RecipientSnapshot, error) {
	s.previewCalls++
	return s.recipients, nil
}

func (s *fakeStore) CreateCampaign(context.Context, Restriction, CampaignType, CampaignCopy, int, string) (Campaign, error) {
	s.createCalls++
	return s.campaign, nil
}

func (s *fakeStore) ListCampaigns(context.Context, string) ([]Campaign, error) { return nil, nil }

func (s *fakeStore) RetryFailedCampaign(context.Context, string, string) (Campaign, error) {
	s.retryCalls++
	return s.campaign, nil
}

type fakePolicyOnlyStore struct{ inner *fakeStore }

func (s *fakePolicyOnlyStore) ListRestrictions(ctx context.Context) ([]Restriction, error) {
	return s.inner.ListRestrictions(ctx)
}

func (s *fakePolicyOnlyStore) ListAdminRestrictions(ctx context.Context) ([]Restriction, error) {
	return s.inner.ListAdminRestrictions(ctx)
}

func (s *fakePolicyOnlyStore) RestrictionForPlatform(ctx context.Context, platform string) (Restriction, error) {
	return s.inner.RestrictionForPlatform(ctx, platform)
}

func (s *fakePolicyOnlyStore) WorkspacePlanID(ctx context.Context, workspaceID string) (string, error) {
	return s.inner.WorkspacePlanID(ctx, workspaceID)
}

func (s *fakePolicyOnlyStore) SetEnabled(ctx context.Context, request TransitionRequest) (TransitionResult, error) {
	return s.inner.SetEnabled(ctx, request)
}

func readyCampaignDelivery() CampaignDeliveryReadiness {
	return CampaignDeliveryReadiness{
		PreviewSecret: true, AuditedSender: true, RestrictionTemplate: true, RecoveryTemplate: true,
	}
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

func TestCampaignCopyIsFixedAndExact(t *testing.T) {
	restriction := CampaignCopyFor(RestrictionNotice)
	if restriction.Subject != "Temporary TikTok publishing restriction for Free plans" {
		t.Fatalf("subject=%q", restriction.Subject)
	}
	wantRestrictionBody := "Hi {{first_name}},\n\nWe’re sorry for the disruption.\n\nTikTok applies a daily capacity limit to publishing through UniPost. Because our current capacity has been reached, TikTok publishing is temporarily unavailable for Free-plan users.\n\nYour connected TikTok accounts, saved posts, and other publishing platforms are not affected. Paid plans will continue to have access to TikTok publishing while this restriction is active.\n\nWe have asked TikTok to increase our publishing capacity and will let you know as soon as TikTok publishing becomes available again on the Free plan.\n\nThank you for your patience and understanding.\n\nThe UniPost Team"
	if restriction.Body != wantRestrictionBody {
		t.Fatalf("restriction body changed:\n%s", restriction.Body)
	}

	recovery := CampaignCopyFor(RecoveryNotice)
	if recovery.Subject != "TikTok publishing is available again on the Free plan" {
		t.Fatalf("subject=%q", recovery.Subject)
	}
	wantRecoveryBody := "Hi {{first_name}},\n\nTikTok publishing is now available again on the UniPost Free plan.\n\nYou can create and publish new TikTok posts from your connected accounts. Posts that previously failed because of the temporary capacity restriction will not publish automatically; please review and publish them again when you’re ready.\n\nThank you for your patience while we worked to increase publishing capacity.\n\nThe UniPost Team"
	if recovery.Body != wantRecoveryBody {
		t.Fatalf("recovery body changed:\n%s", recovery.Body)
	}
}

func TestCampaignPreviewAndConfirmationUseSignedShortLivedToken(t *testing.T) {
	store := &fakeStore{
		restriction: Restriction{ID: "restriction_1", Platform: "tiktok", Enabled: true, CycleID: "cycle_1", Version: 8},
		recipients:  []RecipientSnapshot{{CanonicalUserID: "user_1", NormalizedEmail: "owner@example.com"}},
		campaign:    Campaign{ID: "campaign_1", CycleID: "cycle_1", CampaignType: RestrictionNotice},
	}
	service := NewService(store).
		SetCampaignPreviewSecret("test-preview-secret").
		SetCampaignDeliveryReadiness(readyCampaignDelivery())
	preview, err := service.PreviewCampaign(context.Background(), "tiktok", RestrictionNotice)
	if err != nil {
		t.Fatal(err)
	}
	if preview.RecipientCount != 1 || preview.PreviewToken == "" || preview.CycleID != "cycle_1" {
		t.Fatalf("preview=%+v", preview)
	}
	if store.createCalls != 0 {
		t.Fatal("preview must not persist a campaign")
	}
	created, err := service.ConfirmCampaign(context.Background(), ConfirmCampaignRequest{
		Platform: "tiktok", CampaignType: RestrictionNotice, PreviewToken: preview.PreviewToken,
		Confirmation: "SEND", ActorUserID: "admin_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "campaign_1" || store.createCalls != 1 {
		t.Fatalf("created=%+v calls=%d", created, store.createCalls)
	}
}

func TestCampaignActionsRejectEveryMissingDeliveryPrerequisiteBeforeStoreAccess(t *testing.T) {
	tests := []struct {
		name      string
		readiness CampaignDeliveryReadiness
		store     func(*fakeStore) Store
	}{
		{name: "preview secret", readiness: CampaignDeliveryReadiness{AuditedSender: true, RestrictionTemplate: true, RecoveryTemplate: true}, store: func(store *fakeStore) Store { return store }},
		{name: "audited sender", readiness: CampaignDeliveryReadiness{PreviewSecret: true, RestrictionTemplate: true, RecoveryTemplate: true}, store: func(store *fakeStore) Store { return store }},
		{name: "restriction template", readiness: CampaignDeliveryReadiness{PreviewSecret: true, AuditedSender: true, RecoveryTemplate: true}, store: func(store *fakeStore) Store { return store }},
		{name: "recovery template", readiness: CampaignDeliveryReadiness{PreviewSecret: true, AuditedSender: true, RestrictionTemplate: true}, store: func(store *fakeStore) Store { return store }},
		{name: "campaign store", readiness: readyCampaignDelivery(), store: func(store *fakeStore) Store { return &fakePolicyOnlyStore{inner: store} }},
	}
	actions := []struct {
		name string
		run  func(*Service) error
	}{
		{name: "preview", run: func(service *Service) error {
			_, err := service.PreviewCampaign(context.Background(), "tiktok", RestrictionNotice)
			return err
		}},
		{name: "confirm", run: func(service *Service) error {
			_, err := service.ConfirmCampaign(context.Background(), ConfirmCampaignRequest{
				Platform: "tiktok", CampaignType: RestrictionNotice, PreviewToken: "unused", Confirmation: "SEND",
			})
			return err
		}},
		{name: "retry", run: func(service *Service) error {
			_, err := service.RetryFailedCampaign(context.Background(), "tiktok", "campaign_1")
			return err
		}},
	}

	for _, prerequisite := range tests {
		for _, action := range actions {
			t.Run(prerequisite.name+"/"+action.name, func(t *testing.T) {
				store := &fakeStore{restriction: Restriction{Platform: "tiktok", Enabled: true, CycleID: "cycle_1", Version: 1}}
				service := NewService(prerequisite.store(store)).
					SetCampaignPreviewSecret("test-preview-secret").
					SetCampaignDeliveryReadiness(prerequisite.readiness)
				if prerequisite.name == "preview secret" {
					service.SetCampaignPreviewSecret("")
				}

				err := action.run(service)
				if !errors.Is(err, ErrCampaignNotConfigured) {
					t.Fatalf("error = %v, want ErrCampaignNotConfigured", err)
				}
				if store.restrictionCalls != 0 || store.previewCalls != 0 || store.createCalls != 0 || store.retryCalls != 0 {
					t.Fatalf("store calls before readiness rejection: restriction=%d preview=%d create=%d retry=%d",
						store.restrictionCalls, store.previewCalls, store.createCalls, store.retryCalls)
				}
			})
		}
	}
}
