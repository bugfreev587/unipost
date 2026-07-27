package billing

import (
	"context"
	"errors"
	"testing"

	"github.com/stripe/stripe-go/v82"
)

type recordingPlanChangeStripeClient struct {
	subscription    *stripe.Subscription
	subscriptionErr error
	portal          *stripe.BillingPortalSession
	portalErr       error
	getCalls        int
	portalCalls     int
	gotSubscription string
	gotGetParams    *stripe.SubscriptionParams
	gotPortalParams *stripe.BillingPortalSessionParams
}

func (c *recordingPlanChangeStripeClient) GetSubscription(id string, params *stripe.SubscriptionParams) (*stripe.Subscription, error) {
	c.getCalls++
	c.gotSubscription = id
	c.gotGetParams = params
	return c.subscription, c.subscriptionErr
}

func (c *recordingPlanChangeStripeClient) CreatePortalSession(params *stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
	c.portalCalls++
	c.gotPortalParams = params
	return c.portal, c.portalErr
}

func planChangeTestMode() *Mode {
	return &Mode{
		Name:                            "sandbox",
		priceIDs:                        map[string]string{"basic": "price_sandbox_basic", "growth": "price_sandbox_growth", "team": "price_sandbox_team"},
		planChangePortalConfigurationID: "bpc_sandbox_plan_change",
	}
}

func planChangeTestSubscription() *stripe.Subscription {
	return &stripe.Subscription{
		ID:       "sub_sandbox_1",
		Status:   stripe.SubscriptionStatusActive,
		Livemode: false,
		Customer: &stripe.Customer{ID: "cus_sandbox_1"},
		Metadata: map[string]string{
			"workspace_id":        "ws_1",
			"mode":                "sandbox",
			"unipost_environment": "staging",
		},
		Items: &stripe.SubscriptionItemList{Data: []*stripe.SubscriptionItem{{
			ID:       "si_1",
			Price:    &stripe.Price{ID: "price_sandbox_basic"},
			Quantity: 1,
		}}},
	}
}

func planChangeTestRequest() PlanChangeRequest {
	return PlanChangeRequest{
		StripeMode:     "sandbox",
		WorkspaceID:    "ws_1",
		CustomerID:     "cus_sandbox_1",
		SubscriptionID: "sub_sandbox_1",
		CurrentPlanID:  "basic",
		TargetPlanID:   "growth",
		ReturnURL:      "https://staging-app.unipost.dev/settings/billing",
	}
}

func newPlanChangeTestService(mode *Mode, client *recordingPlanChangeStripeClient) *PlanChangeService {
	return newPlanChangeService(
		&Manager{Sandbox: mode},
		"staging",
		func(got *Mode) planChangeStripeClient {
			if got != mode {
				panic("unexpected Stripe mode")
			}
			return client
		},
	)
}

func TestPlanChangeServiceCreatesVerifiedSubscriptionUpdateConfirmation(t *testing.T) {
	mode := planChangeTestMode()
	client := &recordingPlanChangeStripeClient{
		subscription: planChangeTestSubscription(),
		portal:       &stripe.BillingPortalSession{ID: "bps_1", URL: "https://billing.stripe.test/session/bps_1"},
	}
	service := newPlanChangeTestService(mode, client)

	result, err := service.CreateSession(t.Context(), planChangeTestRequest())
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if result.URL != "https://billing.stripe.test/session/bps_1" {
		t.Fatalf("CreateSession() URL = %q", result.URL)
	}
	if client.getCalls != 1 || client.portalCalls != 1 {
		t.Fatalf("Stripe calls: get=%d portal=%d, want 1 each", client.getCalls, client.portalCalls)
	}
	if client.gotSubscription != "sub_sandbox_1" {
		t.Fatalf("retrieved subscription = %q", client.gotSubscription)
	}
	if client.gotGetParams == nil || client.gotGetParams.Context == nil {
		t.Fatal("subscription retrieval must carry request context")
	}

	params := client.gotPortalParams
	if params == nil {
		t.Fatal("missing Portal Session params")
	}
	if got := stripe.StringValue(params.Configuration); got != "bpc_sandbox_plan_change" {
		t.Fatalf("configuration = %q", got)
	}
	if got := stripe.StringValue(params.Customer); got != "cus_sandbox_1" {
		t.Fatalf("customer = %q", got)
	}
	if got := stripe.StringValue(params.ReturnURL); got != planChangeTestRequest().ReturnURL {
		t.Fatalf("return_url = %q", got)
	}
	if params.Context == nil {
		t.Fatal("Portal Session must carry request context")
	}
	flow := params.FlowData
	if flow == nil || stripe.StringValue(flow.Type) != string(stripe.BillingPortalSessionFlowTypeSubscriptionUpdateConfirm) {
		t.Fatalf("flow = %#v", flow)
	}
	if flow.AfterCompletion == nil || stripe.StringValue(flow.AfterCompletion.Type) != string(stripe.BillingPortalSessionFlowAfterCompletionTypeRedirect) {
		t.Fatalf("after_completion = %#v", flow.AfterCompletion)
	}
	if flow.AfterCompletion.Redirect == nil || stripe.StringValue(flow.AfterCompletion.Redirect.ReturnURL) != planChangeTestRequest().ReturnURL {
		t.Fatalf("redirect = %#v", flow.AfterCompletion.Redirect)
	}
	confirm := flow.SubscriptionUpdateConfirm
	if confirm == nil || stripe.StringValue(confirm.Subscription) != "sub_sandbox_1" || len(confirm.Items) != 1 {
		t.Fatalf("subscription_update_confirm = %#v", confirm)
	}
	item := confirm.Items[0]
	if stripe.StringValue(item.ID) != "si_1" || stripe.StringValue(item.Price) != "price_sandbox_growth" || stripe.Int64Value(item.Quantity) != 1 {
		t.Fatalf("update item = %#v", item)
	}
}

func TestPlanChangeServiceFailsClosedBeforePortalCreation(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*Mode, *recordingPlanChangeStripeClient, *PlanChangeRequest)
		wantErr    error
		wantGet    int
		wantPortal int
	}{
		{name: "missing mode", mutate: func(_ *Mode, _ *recordingPlanChangeStripeClient, req *PlanChangeRequest) { req.StripeMode = "live" }, wantErr: ErrPlanChangeConfigurationUnavailable},
		{name: "missing configuration", mutate: func(mode *Mode, _ *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			mode.planChangePortalConfigurationID = ""
		}, wantErr: ErrPlanChangeConfigurationUnavailable},
		{name: "invalid return URL", mutate: func(_ *Mode, _ *recordingPlanChangeStripeClient, req *PlanChangeRequest) {
			req.ReturnURL = "http://staging-app.unipost.dev/settings/billing"
		}, wantErr: ErrInvalidPlanChangeRequest},
		{name: "unknown current price", mutate: func(mode *Mode, _ *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			delete(mode.priceIDs, "basic")
		}, wantErr: ErrInvalidPlanChangeRequest},
		{name: "unknown target price", mutate: func(mode *Mode, _ *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			delete(mode.priceIDs, "growth")
		}, wantErr: ErrInvalidPlanChangeRequest},
		{name: "same target", mutate: func(_ *Mode, _ *recordingPlanChangeStripeClient, req *PlanChangeRequest) { req.TargetPlanID = "basic" }, wantErr: ErrInvalidPlanChangeRequest},
		{name: "retrieval error", mutate: func(_ *Mode, client *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			client.subscriptionErr = errors.New("stripe unavailable")
		}, wantErr: ErrPlanChangeUpstream, wantGet: 1},
		{name: "nil subscription", mutate: func(_ *Mode, client *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			client.subscription = nil
		}, wantErr: ErrPlanChangeBindingConflict, wantGet: 1},
		{name: "wrong subscription", mutate: func(_ *Mode, client *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			client.subscription.ID = "sub_other"
		}, wantErr: ErrPlanChangeBindingConflict, wantGet: 1},
		{name: "wrong customer", mutate: func(_ *Mode, client *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			client.subscription.Customer.ID = "cus_other"
		}, wantErr: ErrPlanChangeBindingConflict, wantGet: 1},
		{name: "wrong workspace metadata", mutate: func(_ *Mode, client *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			client.subscription.Metadata["workspace_id"] = "ws_other"
		}, wantErr: ErrPlanChangeBindingConflict, wantGet: 1},
		{name: "wrong mode metadata", mutate: func(_ *Mode, client *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			client.subscription.Metadata["mode"] = "live"
		}, wantErr: ErrPlanChangeBindingConflict, wantGet: 1},
		{name: "wrong environment metadata", mutate: func(_ *Mode, client *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			client.subscription.Metadata["unipost_environment"] = "production"
		}, wantErr: ErrPlanChangeBindingConflict, wantGet: 1},
		{name: "wrong livemode", mutate: func(_ *Mode, client *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			client.subscription.Livemode = true
		}, wantErr: ErrPlanChangeBindingConflict, wantGet: 1},
		{name: "inactive subscription", mutate: func(_ *Mode, client *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			client.subscription.Status = stripe.SubscriptionStatusCanceled
		}, wantErr: ErrPlanChangeBindingConflict, wantGet: 1},
		{name: "missing item list", mutate: func(_ *Mode, client *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			client.subscription.Items = nil
		}, wantErr: ErrPlanChangeBindingConflict, wantGet: 1},
		{name: "multiple items", mutate: func(_ *Mode, client *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			client.subscription.Items.Data = append(client.subscription.Items.Data, &stripe.SubscriptionItem{ID: "si_2"})
		}, wantErr: ErrPlanChangeBindingConflict, wantGet: 1},
		{name: "missing item id", mutate: func(_ *Mode, client *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			client.subscription.Items.Data[0].ID = ""
		}, wantErr: ErrPlanChangeBindingConflict, wantGet: 1},
		{name: "wrong current item price", mutate: func(_ *Mode, client *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			client.subscription.Items.Data[0].Price.ID = "price_sandbox_team"
		}, wantErr: ErrPlanChangeBindingConflict, wantGet: 1},
		{name: "wrong quantity", mutate: func(_ *Mode, client *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			client.subscription.Items.Data[0].Quantity = 2
		}, wantErr: ErrPlanChangeBindingConflict, wantGet: 1},
		{name: "portal error", mutate: func(_ *Mode, client *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			client.portalErr = errors.New("portal unavailable")
		}, wantErr: ErrPlanChangeUpstream, wantGet: 1, wantPortal: 1},
		{name: "incomplete portal", mutate: func(_ *Mode, client *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			client.portal = &stripe.BillingPortalSession{ID: "bps_1"}
		}, wantErr: ErrPlanChangeUpstream, wantGet: 1, wantPortal: 1},
		{name: "non https portal", mutate: func(_ *Mode, client *recordingPlanChangeStripeClient, _ *PlanChangeRequest) {
			client.portal.URL = "http://billing.stripe.test/bps_1"
		}, wantErr: ErrPlanChangeUpstream, wantGet: 1, wantPortal: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mode := planChangeTestMode()
			client := &recordingPlanChangeStripeClient{
				subscription: planChangeTestSubscription(),
				portal:       &stripe.BillingPortalSession{ID: "bps_1", URL: "https://billing.stripe.test/session/bps_1"},
			}
			req := planChangeTestRequest()
			test.mutate(mode, client, &req)
			service := newPlanChangeTestService(mode, client)

			_, err := service.CreateSession(t.Context(), req)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("CreateSession() error = %v, want %v", err, test.wantErr)
			}
			if client.getCalls != test.wantGet {
				t.Fatalf("subscription retrievals = %d, want %d", client.getCalls, test.wantGet)
			}
			if client.portalCalls != test.wantPortal {
				t.Fatalf("Portal Sessions created = %d, want %d", client.portalCalls, test.wantPortal)
			}
		})
	}
}

func TestPlanChangeServiceAllowsLegacySubscriptionWithoutRoutingMetadata(t *testing.T) {
	mode := planChangeTestMode()
	subscription := planChangeTestSubscription()
	subscription.Metadata = nil
	client := &recordingPlanChangeStripeClient{
		subscription: subscription,
		portal:       &stripe.BillingPortalSession{ID: "bps_legacy", URL: "https://billing.stripe.test/session/bps_legacy"},
	}

	result, err := newPlanChangeTestService(mode, client).CreateSession(context.Background(), planChangeTestRequest())
	if err != nil || result.URL == "" {
		t.Fatalf("CreateSession() result = %#v, error = %v", result, err)
	}
}
