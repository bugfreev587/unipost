package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v82"
	"github.com/xiaoboyu/unipost-api/internal/quota"
	"github.com/xiaoboyu/unipost-api/internal/runtimeenv"
	"github.com/xiaoboyu/unipost-api/internal/trials"
)

func TestCheckoutMetadataIncludesRuntimeEnvironment(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, " staging ")

	got := stripeCheckoutMetadata("ws_staging", "basic", "sandbox")

	want := map[string]string{
		"workspace_id":        "ws_staging",
		"plan_id":             "basic",
		"mode":                "sandbox",
		"unipost_environment": "staging",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("metadata = %#v, want %#v", got, want)
	}
}

func TestCheckoutSubscriptionDataReusesRoutingMetadata(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	metadata := stripeCheckoutMetadata("ws_staging", "basic", "sandbox")

	data := stripeCheckoutSubscriptionData(metadata)

	if !reflect.DeepEqual(data.Metadata, metadata) {
		t.Fatalf("subscription metadata = %#v, want %#v", data.Metadata, metadata)
	}
}

func TestStripeCustomerParamsUseWorkspaceModeIdempotency(t *testing.T) {
	params := stripeCustomerParams("ws_1", "owner_1", "Workspace One", "owner@example.com", "sandbox")
	if got := stripe.StringValue(params.IdempotencyKey); got != "billing:sandbox:workspace:ws_1:customer" {
		t.Fatalf("idempotency key=%q", got)
	}
	if params.Params.Metadata["workspace_id"] != "ws_1" || params.Params.Metadata["user_id"] != "owner_1" || params.Params.Metadata["mode"] != "sandbox" {
		t.Fatalf("metadata=%#v", params.Params.Metadata)
	}
}

func TestStripeCheckoutURLFailsClosedOnEmptyResponse(t *testing.T) {
	for _, session := range []*stripe.CheckoutSession{nil, {}, {ID: "cs_1"}} {
		if _, err := stripeCheckoutURL(session); err == nil {
			t.Fatalf("session=%#v accepted", session)
		}
	}
	if got, err := stripeCheckoutURL(&stripe.CheckoutSession{ID: "cs_1", URL: "https://checkout.stripe.test/cs_1"}); err != nil || got == "" {
		t.Fatalf("url=%q error=%v", got, err)
	}
}

func TestResolveTrialStripeCustomerValidatesCandidateBeforeReuse(t *testing.T) {
	for _, test := range []struct {
		name           string
		candidate      *stripe.Customer
		retrieveErr    error
		wantID         string
		wantCreateCall int
		wantErr        bool
	}{
		{name: "matching workspace", candidate: &stripe.Customer{ID: "cus_existing", Metadata: map[string]string{"workspace_id": "ws_1"}}, wantID: "cus_existing"},
		{name: "legacy empty metadata", candidate: &stripe.Customer{ID: "cus_existing"}, wantID: "cus_existing"},
		{name: "resource missing", retrieveErr: &stripe.Error{Code: stripe.ErrorCodeResourceMissing, HTTPStatusCode: http.StatusNotFound}, wantID: "cus_new", wantCreateCall: 1},
		{name: "deleted", candidate: &stripe.Customer{ID: "cus_existing", Deleted: true}, wantID: "cus_new", wantCreateCall: 1},
		{name: "workspace mismatch", candidate: &stripe.Customer{ID: "cus_existing", Metadata: map[string]string{"workspace_id": "ws_other"}}, wantID: "cus_new", wantCreateCall: 1},
		{name: "returned ID mismatch", candidate: &stripe.Customer{ID: "cus_other", Metadata: map[string]string{"workspace_id": "ws_1"}}, wantID: "cus_new", wantCreateCall: 1},
		{name: "transient lookup", retrieveErr: context.DeadlineExceeded, wantErr: true},
		{name: "Stripe 500 lookup", retrieveErr: &stripe.Error{HTTPStatusCode: http.StatusInternalServerError, Type: stripe.ErrorTypeAPI}, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &fakeStripeCustomerService{
				retrieveResult: test.candidate, retrieveErr: test.retrieveErr,
				createResult: &stripe.Customer{ID: "cus_new"},
			}
			got, err := resolveTrialStripeCustomer(t.Context(), client, "cus_existing", "ws_1", "owner_1", "Workspace", "owner@example.com", "live")
			if test.wantErr {
				if err == nil || client.createCalls != 0 {
					t.Fatalf("id=%q error=%v createCalls=%d", got, err, client.createCalls)
				}
				return
			}
			if err != nil || got != test.wantID || client.createCalls != test.wantCreateCall {
				t.Fatalf("id=%q error=%v createCalls=%d", got, err, client.createCalls)
			}
		})
	}
}

func TestResolveTrialStripeCustomerCreatesStableCustomerWhenCandidateAbsent(t *testing.T) {
	client := &fakeStripeCustomerService{createResult: &stripe.Customer{ID: "cus_new"}}
	got, err := resolveTrialStripeCustomer(t.Context(), client, "", "ws_1", "owner_1", "Workspace", "owner@example.com", "sandbox")
	if err != nil || got != "cus_new" || client.retrieveCalls != 0 || client.createCalls != 1 {
		t.Fatalf("id=%q error=%v retrieve=%d create=%d", got, err, client.retrieveCalls, client.createCalls)
	}
	if key := stripe.StringValue(client.createParams.IdempotencyKey); key != "billing:sandbox:workspace:ws_1:customer" {
		t.Fatalf("create idempotency key=%q", key)
	}
}

type fakeStripeCustomerService struct {
	retrieveResult *stripe.Customer
	retrieveErr    error
	createResult   *stripe.Customer
	createErr      error
	retrieveCalls  int
	createCalls    int
	createParams   *stripe.CustomerParams
}

func (s *fakeStripeCustomerService) Get(_ string, _ *stripe.CustomerParams) (*stripe.Customer, error) {
	s.retrieveCalls++
	return s.retrieveResult, s.retrieveErr
}

func (s *fakeStripeCustomerService) New(params *stripe.CustomerParams) (*stripe.Customer, error) {
	s.createCalls++
	s.createParams = params
	return s.createResult, s.createErr
}

func TestTrialCheckoutHandlerPassesValidatedRoutingAndReturnsCheckoutURL(t *testing.T) {
	service := &fakeBillingTrialCheckoutService{result: trials.CheckoutResult{
		SessionID: "cs_trial", URL: "https://checkout.stripe.test/cs_trial", TrialGrantID: "grant_1", TrialDays: 30,
	}}
	h := (&BillingHandler{}).SetTrialService(service)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/checkout", nil)
	checkoutReq := trials.CheckoutRequest{
		WorkspaceID: "ws_1", PlanID: "growth", StripeMode: "sandbox", CustomerID: "cus_1",
		PriceID: "price_growth_test", SuccessURL: "https://app.unipost.dev/settings/billing?status=success",
		CancelURL: "https://app.unipost.dev/settings/billing?status=canceled",
	}

	if handled, needsCustomer := h.tryTrialCheckout(recorder, req, checkoutReq); !handled || needsCustomer {
		t.Fatal("trial checkout was not handled")
	}
	if !reflect.DeepEqual(service.req, checkoutReq) {
		t.Fatalf("request = %#v, want %#v", service.req, checkoutReq)
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data["checkout_url"] != service.result.URL {
		t.Fatalf("response = %#v", response.Data)
	}
}

func TestTrialCheckoutHandlerFallsThroughOnlyWhenGrantDoesNotApply(t *testing.T) {
	for _, test := range []struct {
		name         string
		err          error
		wantHandle   bool
		wantCustomer bool
		wantStatus   int
	}{
		{name: "mismatched or absent grant", err: trials.ErrTrialCheckoutNotApplicable, wantHandle: false},
		{name: "matching pending grant", err: trials.ErrCheckoutCustomerRequired, wantHandle: false, wantCustomer: true},
		{name: "completed awaiting webhook", err: trials.ErrCheckoutCompletionPending, wantHandle: true, wantStatus: http.StatusConflict},
		{name: "concurrent state", err: trials.ErrCheckoutStateConflict, wantHandle: true, wantStatus: http.StatusConflict},
		{name: "Stripe unavailable", err: context.DeadlineExceeded, wantHandle: true, wantStatus: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeBillingTrialCheckoutService{err: test.err}
			h := (&BillingHandler{}).SetTrialService(service)
			recorder := httptest.NewRecorder()
			handled, needsCustomer := h.tryTrialCheckout(recorder, httptest.NewRequest(http.MethodPost, "/v1/billing/checkout", nil), trials.CheckoutRequest{})
			if handled != test.wantHandle {
				t.Fatalf("handled = %v, want %v", handled, test.wantHandle)
			}
			if needsCustomer != test.wantCustomer {
				t.Fatalf("needsCustomer = %v, want %v", needsCustomer, test.wantCustomer)
			}
			if service.calls != 1 {
				t.Fatalf("service calls = %d, want 1", service.calls)
			}
			if handled && recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d body=%s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
		})
	}
}

func TestTrialCheckoutHandlerPendingActivationUsesOneProbeAndOnePostCustomerCall(t *testing.T) {
	service := &fakeBillingTrialCheckoutService{
		errs:   []error{trials.ErrCheckoutCustomerRequired, nil},
		result: trials.CheckoutResult{URL: "https://checkout.stripe.test/cs_trial"},
	}
	h := (&BillingHandler{}).SetTrialService(service)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/checkout", nil)
	checkoutReq := trials.CheckoutRequest{WorkspaceID: "ws_1", PlanID: "growth", StripeMode: "live", PriceID: "price_growth", SuccessURL: "https://app.example/success", CancelURL: "https://app.example/cancel"}

	handled, needsCustomer := h.tryTrialCheckout(recorder, req, checkoutReq)
	if handled || !needsCustomer {
		t.Fatalf("probe handled=%v needsCustomer=%v", handled, needsCustomer)
	}
	checkoutReq.CustomerID = "cus_1"
	handled, needsCustomer = h.tryTrialCheckout(recorder, req, checkoutReq)
	if !handled || needsCustomer || service.calls != 2 {
		t.Fatalf("retry handled=%v needsCustomer=%v calls=%d", handled, needsCustomer, service.calls)
	}
}

type fakeBillingTrialCheckoutService struct {
	result trials.CheckoutResult
	err    error
	errs   []error
	req    trials.CheckoutRequest
	calls  int
}

func (s *fakeBillingTrialCheckoutService) PrepareCheckout(_ context.Context, req trials.CheckoutRequest) (trials.CheckoutResult, error) {
	s.req = req
	s.calls++
	if len(s.errs) > 0 {
		err := s.errs[0]
		s.errs = s.errs[1:]
		return s.result, err
	}
	return s.result, s.err
}

func TestUsageResponseFromMonthlySnapshot(t *testing.T) {
	snapshot := quota.MonthlySnapshot{
		WorkspaceID: "ws_123",
		PlanID:      "basic",
		Period:      "2026-07",
		Completed:   2488,
		Scheduled:   12,
		QuotaHold:   2,
		Limit:       2500,
	}

	response := usageResponseFromSnapshot(snapshot)

	if response.PostCount != 2488 || response.ScheduledCount != 12 || response.QuotaHoldCount != 2 {
		t.Fatalf("usage counts = %#v", response)
	}
	if response.EffectiveUsage != 2500 || response.Percentage != 99.52 || response.EffectivePercentage != 100 {
		t.Fatalf("usage percentages = %#v", response)
	}
	if response.Warning != "scheduled_quota_reached" || response.SchedulingAllowed {
		t.Fatalf("scheduling state = %#v", response)
	}
	wantReset := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if !response.ResetsAt.Equal(wantReset) {
		t.Fatalf("resets_at = %s, want %s", response.ResetsAt, wantReset)
	}
}

func TestUsageResponseKeepsPaidSchedulingOpenBelow100(t *testing.T) {
	response := usageResponseFromSnapshot(quota.MonthlySnapshot{
		PlanID:    "api",
		Period:    "2026-07",
		Completed: 790,
		Scheduled: 10,
		Limit:     1000,
	})
	if response.Warning != "approaching_limit" || !response.SchedulingAllowed {
		t.Fatalf("response = %#v", response)
	}
}

func TestUsageResponsePausesSchedulingWhileQuotaHoldsExist(t *testing.T) {
	response := usageResponseFromSnapshot(quota.MonthlySnapshot{
		WorkspaceID: "ws_123",
		PlanID:      "basic",
		Period:      "2026-07",
		Completed:   70,
		Scheduled:   10,
		QuotaHold:   5,
		Limit:       100,
	})
	if response.SchedulingAllowed || response.Warning != "scheduled_quota_reached" {
		t.Fatalf("hold response = %#v, want scheduling paused", response)
	}
}

func TestUsageResponseDoesNotApplyPaidCircuitBreakerToExcludedPlans(t *testing.T) {
	for _, planID := range []string{"free", "team", "enterprise"} {
		response := usageResponseFromSnapshot(quota.MonthlySnapshot{
			PlanID:    planID,
			Period:    "2026-07",
			Completed: 200,
			Limit:     100,
		})
		if !response.SchedulingAllowed {
			t.Fatalf("%s scheduling should remain governed by its existing policy", planID)
		}
	}
}
