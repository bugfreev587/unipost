package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stripe/stripe-go/v82"
	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/billing"
	"github.com/xiaoboyu/unipost-api/internal/db"
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

func TestGetBillingTrialIncludesSafeProjectionForEveryVisibleState(t *testing.T) {
	for _, status := range []trials.Status{
		trials.StatusPendingActivation,
		trials.StatusCheckoutPending,
		trials.StatusScheduled,
		trials.StatusActive,
	} {
		t.Run(string(status), func(t *testing.T) {
			start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
			end := start.Add(30 * 24 * time.Hour)
			projection := &trials.TrialProjection{
				ID: "grant_1", Kind: trials.KindFreeToPaid, PlanID: "growth", DurationDays: 30,
				Status: status, ScheduledStartAt: &start, StartedAt: &start, EndsAt: &end,
				PostTrialPriceCents: 4900, CancelAtPeriodEnd: true,
				ChangingPlanForfeitsTrial: status == trials.StatusScheduled || status == trials.StatusActive,
			}
			service := &fakeBillingTrialCheckoutService{currentProjection: projection}
			h := (&BillingHandler{}).SetTrialService(service)

			got, err := h.getCurrentTrialProjection(t.Context(), "ws_1")
			if err != nil || !reflect.DeepEqual(got, projection) {
				t.Fatalf("projection=%#v error=%v", got, err)
			}
			encoded, err := json.Marshal(billingResponse{Trial: got})
			if err != nil {
				t.Fatal(err)
			}
			body := string(encoded)
			for _, want := range []string{`"trial"`, `"post_trial_price_cents":4900`, `"cancel_at_period_end":true`, `"changing_plan_forfeits_trial"`} {
				if !strings.Contains(body, want) {
					t.Fatalf("response missing %q: %s", want, body)
				}
			}
		})
	}
}

func TestListTrialHistoryReturnsNewestFirstAndRedactsInternals(t *testing.T) {
	newest := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	older := newest.Add(-24 * time.Hour)
	service := &fakeBillingTrialCheckoutService{history: []trials.HistoryProjection{
		{ID: "newest", Kind: trials.KindPaidSamePlan, PlanID: "basic", DurationDays: 60, Status: trials.StatusCompleted, GrantedAt: &newest, TerminalReason: trials.TerminalReasonCompleted},
		{ID: "older", Kind: trials.KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: trials.StatusFailed, GrantedAt: &older, TerminalReason: trials.TerminalReasonUnavailable},
	}}
	h := (&BillingHandler{}).SetTrialService(service)
	req := httptest.NewRequest(http.MethodGet, "/v1/billing/trials", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	recorder := httptest.NewRecorder()

	h.ListTrialHistory(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data []trials.HistoryProjection `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data) != 2 || response.Data[0].ID != "newest" || response.Data[1].ID != "older" {
		t.Fatalf("history=%#v", response.Data)
	}
	if service.historyWorkspaceID != "ws_1" {
		t.Fatalf("workspace=%q", service.historyWorkspaceID)
	}
	for _, forbidden := range []string{"granted_by_user_id", "failure_code", "failure_message", "stripe_customer_id", "stripe_subscription_id", "admin_secret", "internal secret"} {
		if strings.Contains(recorder.Body.String(), forbidden) {
			t.Fatalf("history leaked %q: %s", forbidden, recorder.Body.String())
		}
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
		{name: "managed trial requires plan change", err: trials.ErrTrialPlanChangeRequired, wantHandle: true, wantStatus: http.StatusConflict},
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
	result             trials.CheckoutResult
	err                error
	errs               []error
	req                trials.CheckoutRequest
	calls              int
	changeResult       trials.Grant
	changeErr          error
	changeReq          trials.ChangePlanRequest
	changeCalls        int
	cancelResult       trials.Grant
	cancelErr          error
	cancelReq          trials.CancelRequest
	cancelCalls        int
	portalURL          string
	portalHandled      bool
	portalErr          error
	portalReq          trials.PortalRequest
	portalCalls        int
	currentProjection  *trials.TrialProjection
	currentErr         error
	history            []trials.HistoryProjection
	historyErr         error
	historyWorkspaceID string
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

func (s *fakeBillingTrialCheckoutService) ChangePlan(_ context.Context, req trials.ChangePlanRequest) (trials.Grant, error) {
	s.changeCalls++
	s.changeReq = req
	return s.changeResult, s.changeErr
}

func (s *fakeBillingTrialCheckoutService) CancelRenewal(_ context.Context, req trials.CancelRequest) (trials.Grant, error) {
	s.cancelCalls++
	s.cancelReq = req
	return s.cancelResult, s.cancelErr
}

func (s *fakeBillingTrialCheckoutService) CreateTrialSafePortal(_ context.Context, req trials.PortalRequest) (string, bool, error) {
	s.portalCalls++
	s.portalReq = req
	return s.portalURL, s.portalHandled, s.portalErr
}

func (s *fakeBillingTrialCheckoutService) CurrentTrialProjection(context.Context, string) (*trials.TrialProjection, error) {
	return s.currentProjection, s.currentErr
}

func (s *fakeBillingTrialCheckoutService) ListTrialHistory(_ context.Context, workspaceID string) ([]trials.HistoryProjection, error) {
	s.historyWorkspaceID = workspaceID
	return s.history, s.historyErr
}

func TestChangePlanHandlerDispatchesWorkspaceTargetPlan(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	service := &fakeBillingTrialCheckoutService{changeResult: trials.Grant{ID: "grant_1", Kind: trials.KindFreeToPaid, PlanID: "basic", DurationDays: 30, Status: trials.StatusActive, StartedAt: &start, EndsAt: &end, ActorUserID: "admin_secret", StripeSubscriptionID: "sub_secret", FailureMessage: "internal secret"}}
	h := (&BillingHandler{}).SetTrialService(service)
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/change-plan", bytes.NewBufferString(`{"plan_id":"growth"}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	recorder := httptest.NewRecorder()

	h.ChangePlan(recorder, req)

	if recorder.Code != http.StatusOK || service.changeCalls != 1 || service.changeReq.WorkspaceID != "ws_1" || service.changeReq.TargetPlanID != "growth" {
		t.Fatalf("status=%d calls=%d request=%#v body=%s", recorder.Code, service.changeCalls, service.changeReq, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "admin_secret") || strings.Contains(recorder.Body.String(), "granted_by") {
		t.Fatalf("user mutation response leaked administrator identity: %s", recorder.Body.String())
	}
	for _, forbidden := range []string{"sub_secret", "internal secret", "stripe_subscription", "failure_message"} {
		if strings.Contains(recorder.Body.String(), forbidden) {
			t.Fatalf("user mutation response leaked %q: %s", forbidden, recorder.Body.String())
		}
	}
	for _, want := range []string{`"trial"`, `"billing"`, `"current_plan":"basic"`, `"target_plan":"growth"`, `"status":"change_pending"`, `"change_pending":true`, `"cancel_at_period_end":false`, `"ends_at":"2026-07-31T00:00:00Z"`} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Fatalf("change-plan response missing %s: %s", want, recorder.Body.String())
		}
	}
}

type fakeBillingQueries struct {
	workspace       db.Workspace
	workspaceErr    error
	subscription    db.Subscription
	subscriptionErr error
	user            db.User
	userErr         error
	plans           map[string]db.Plan
	planErr         error
}

func (q *fakeBillingQueries) GetWorkspace(context.Context, string) (db.Workspace, error) {
	return q.workspace, q.workspaceErr
}

func (q *fakeBillingQueries) GetSubscriptionByWorkspace(context.Context, string) (db.Subscription, error) {
	return q.subscription, q.subscriptionErr
}

func (q *fakeBillingQueries) GetUser(context.Context, string) (db.User, error) {
	return q.user, q.userErr
}

func (q *fakeBillingQueries) GetPlan(_ context.Context, planID string) (db.Plan, error) {
	if q.planErr != nil {
		return db.Plan{}, q.planErr
	}
	plan, ok := q.plans[planID]
	if !ok {
		return db.Plan{}, errors.New("plan not found")
	}
	return plan, nil
}

func (q *fakeBillingQueries) ListPlans(context.Context) ([]db.Plan, error) {
	plans := make([]db.Plan, 0, len(q.plans))
	for _, plan := range q.plans {
		plans = append(plans, plan)
	}
	return plans, nil
}

type fakeBillingPlanChangeService struct {
	result billing.PlanChangeResult
	err    error
	req    billing.PlanChangeRequest
	calls  int
}

func (s *fakeBillingPlanChangeService) CreateSession(_ context.Context, req billing.PlanChangeRequest) (billing.PlanChangeResult, error) {
	s.calls++
	s.req = req
	return s.result, s.err
}

func planChangeHandlerFixture(planID string) (*BillingHandler, *fakeBillingQueries, *fakeBillingPlanChangeService) {
	queries := &fakeBillingQueries{
		workspace: db.Workspace{ID: "ws_1", UserID: "owner_1", Name: "Workspace One"},
		subscription: db.Subscription{
			WorkspaceID: "ws_1", PlanID: planID, Status: "active",
			StripeCustomerID:     pgtype.Text{String: "cus_sandbox_1", Valid: true},
			StripeSubscriptionID: pgtype.Text{String: "sub_sandbox_1", Valid: true},
		},
		user:  db.User{ID: "owner_1", Email: "owner@example.com"},
		plans: map[string]db.Plan{"free": {ID: "free"}, "basic": {ID: "basic"}, "growth": {ID: "growth"}, "team": {ID: "team"}},
	}
	mode := &billing.Mode{Name: "live"}
	manager := &billing.Manager{Live: mode}
	service := &fakeBillingPlanChangeService{result: billing.PlanChangeResult{URL: "https://billing.stripe.test/session/bps_1"}}
	handler := NewBillingHandler(queries, nil, manager).SetPlanChangeService(service)
	return handler, queries, service
}

func planChangeHandlerRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/plan-change-session", bytes.NewBufferString(body))
	ctx := auth.SetWorkspaceID(req.Context(), "ws_1")
	ctx = context.WithValue(ctx, auth.UserIDKey, "owner_1")
	return req.WithContext(ctx)
}

func TestCreatePlanChangeSessionUsesOnlyServerDerivedStripeBinding(t *testing.T) {
	t.Setenv("NEXT_PUBLIC_APP_URL", "https://staging-app.unipost.dev")
	h, _, service := planChangeHandlerFixture("basic")
	recorder := httptest.NewRecorder()

	h.CreatePlanChangeSession(recorder, planChangeHandlerRequest(`{"plan_id":"growth","customer_id":"cus_attacker","subscription_id":"sub_attacker"}`))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	want := billing.PlanChangeRequest{
		StripeMode: "live", WorkspaceID: "ws_1", CustomerID: "cus_sandbox_1", SubscriptionID: "sub_sandbox_1",
		CurrentPlanID: "basic", TargetPlanID: "growth", ReturnURL: "https://staging-app.unipost.dev/settings/billing",
	}
	if service.calls != 1 || !reflect.DeepEqual(service.req, want) {
		t.Fatalf("calls=%d request=%#v want=%#v", service.calls, service.req, want)
	}
	if !strings.Contains(recorder.Body.String(), `"url":"https://billing.stripe.test/session/bps_1"`) {
		t.Fatalf("response=%s", recorder.Body.String())
	}
}

func TestCreatePlanChangeSessionDispatchesOnlyOrdinaryPaidBilling(t *testing.T) {
	for _, test := range []struct {
		name       string
		planID     string
		body       string
		projection *trials.TrialProjection
		wantStatus int
		wantCode   string
	}{
		{name: "missing workspace", planID: "basic", body: `{"plan_id":"growth"}`, wantStatus: http.StatusUnauthorized, wantCode: "UNAUTHORIZED"},
		{name: "invalid body", planID: "basic", body: `{}`, wantStatus: http.StatusUnprocessableEntity, wantCode: "VALIDATION_ERROR"},
		{name: "free uses checkout", planID: "free", body: `{"plan_id":"growth"}`, wantStatus: http.StatusConflict, wantCode: "CHECKOUT_REQUIRED"},
		{name: "scheduled trial uses change plan", planID: "basic", body: `{"plan_id":"growth"}`, projection: &trials.TrialProjection{Status: trials.StatusScheduled}, wantStatus: http.StatusConflict, wantCode: "TRIAL_PLAN_CHANGE_REQUIRED"},
		{name: "active trial uses change plan", planID: "basic", body: `{"plan_id":"growth"}`, projection: &trials.TrialProjection{Status: trials.StatusActive}, wantStatus: http.StatusConflict, wantCode: "TRIAL_PLAN_CHANGE_REQUIRED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			h, _, service := planChangeHandlerFixture(test.planID)
			if test.projection != nil {
				h.SetTrialService(&fakeBillingTrialCheckoutService{currentProjection: test.projection})
			}
			req := planChangeHandlerRequest(test.body)
			if test.name == "missing workspace" {
				req = httptest.NewRequest(http.MethodPost, "/v1/billing/plan-change-session", bytes.NewBufferString(test.body))
			}
			recorder := httptest.NewRecorder()

			h.CreatePlanChangeSession(recorder, req)

			if recorder.Code != test.wantStatus || !strings.Contains(recorder.Body.String(), test.wantCode) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if service.calls != 0 {
				t.Fatalf("service calls=%d, want 0", service.calls)
			}
		})
	}
}

func TestCreatePlanChangeSessionMapsServiceFailuresWithoutLeakingProviderDetails(t *testing.T) {
	for _, test := range []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "configuration", err: billing.ErrPlanChangeConfigurationUnavailable, wantStatus: http.StatusServiceUnavailable, wantCode: "BILLING_PLAN_CHANGE_UNAVAILABLE"},
		{name: "binding", err: billing.ErrPlanChangeBindingConflict, wantStatus: http.StatusConflict, wantCode: "BILLING_PLAN_CHANGE_CONFLICT"},
		{name: "upstream", err: fmt.Errorf("%w: stripe secret", billing.ErrPlanChangeUpstream), wantStatus: http.StatusBadGateway, wantCode: "BILLING_PLAN_CHANGE_UNAVAILABLE"},
	} {
		t.Run(test.name, func(t *testing.T) {
			h, _, service := planChangeHandlerFixture("basic")
			service.err = test.err
			recorder := httptest.NewRecorder()

			h.CreatePlanChangeSession(recorder, planChangeHandlerRequest(`{"plan_id":"growth"}`))

			if recorder.Code != test.wantStatus || !strings.Contains(recorder.Body.String(), test.wantCode) || strings.Contains(recorder.Body.String(), "stripe secret") {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestCreateCheckoutRejectsPaidWorkspaceBeforeStripeMutation(t *testing.T) {
	h, _, service := planChangeHandlerFixture("basic")
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/checkout", bytes.NewBufferString(`{"plan_id":"growth"}`))
	ctx := auth.SetWorkspaceID(req.Context(), "ws_1")
	ctx = context.WithValue(ctx, auth.UserIDKey, "owner_1")
	recorder := httptest.NewRecorder()

	h.CreateCheckout(recorder, req.WithContext(ctx))

	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "PAID_PLAN_CHANGE_REQUIRED") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.calls != 0 {
		t.Fatalf("plan change service calls=%d, want 0", service.calls)
	}
}

func TestCancelTrialRenewalHandlerDispatchesExactGrant(t *testing.T) {
	canceled := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	service := &fakeBillingTrialCheckoutService{cancelResult: trials.Grant{ID: "grant_1", Kind: trials.KindPaidSamePlan, PlanID: "basic", DurationDays: 30, Status: trials.StatusActive, EndsAt: &end, CanceledAt: &canceled, ActorUserID: "admin_secret", StripeScheduleID: "sched_secret", FailureCode: "internal_code"}}
	h := (&BillingHandler{}).SetTrialService(service)
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/trials/grant_1/cancel-renewal", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("trialID", "grant_1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
	recorder := httptest.NewRecorder()

	h.CancelTrialRenewal(recorder, req)

	if recorder.Code != http.StatusOK || service.cancelCalls != 1 || service.cancelReq != (trials.CancelRequest{WorkspaceID: "ws_1", GrantID: "grant_1"}) {
		t.Fatalf("status=%d calls=%d request=%#v body=%s", recorder.Code, service.cancelCalls, service.cancelReq, recorder.Body.String())
	}
	for _, forbidden := range []string{"admin_secret", "sched_secret", "internal_code", "granted_by", "stripe_schedule", "failure_code"} {
		if strings.Contains(recorder.Body.String(), forbidden) {
			t.Fatalf("cancel response leaked %q: %s", forbidden, recorder.Body.String())
		}
	}
	for _, want := range []string{`"trial"`, `"billing"`, `"current_plan":"basic"`, `"target_plan":"basic"`, `"status":"active"`, `"change_pending":false`, `"cancel_at_period_end":true`, `"canceled_at":"2026-07-10T00:00:00Z"`} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Fatalf("cancel response missing %s: %s", want, recorder.Body.String())
		}
	}
}

func TestTrialMutationHandlersFailClosedOnConflictAndMutationError(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "not applicable", err: trials.ErrTrialMutationNotApplicable, want: http.StatusConflict},
		{name: "not found", err: trials.ErrGrantNotFound, want: http.StatusNotFound},
		{name: "Stripe unknown", err: context.DeadlineExceeded, want: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeBillingTrialCheckoutService{cancelErr: test.err}
			h := (&BillingHandler{}).SetTrialService(service)
			req := httptest.NewRequest(http.MethodPost, "/v1/billing/trials/grant_1/cancel-renewal", nil)
			req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
			routeContext := chi.NewRouteContext()
			routeContext.URLParams.Add("trialID", "grant_1")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
			recorder := httptest.NewRecorder()
			h.CancelTrialRenewal(recorder, req)
			if recorder.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", recorder.Code, test.want, recorder.Body.String())
			}
		})
	}
}

func TestTryTrialSafePortalReturnsHandledErrorWithoutDefaultFallback(t *testing.T) {
	for _, serviceHandled := range []bool{false, true} {
		service := &fakeBillingTrialCheckoutService{portalHandled: serviceHandled, portalErr: errors.New("missing trial-safe configuration")}
		h := (&BillingHandler{}).SetTrialService(service)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/billing/portal", nil)
		handled := h.tryTrialSafePortal(recorder, req, trials.PortalRequest{WorkspaceID: "ws_1", StripeMode: "live", CustomerID: "cus_1", ReturnURL: "https://app.example/settings/billing"})
		if !handled || recorder.Code != http.StatusInternalServerError || service.portalCalls != 1 {
			t.Fatalf("service handled=%v handled=%v status=%d calls=%d", serviceHandled, handled, recorder.Code, service.portalCalls)
		}
	}
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
