package trials

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v82"
	"github.com/xiaoboyu/unipost-api/internal/billing"
)

type MutationOutcome string

const (
	MutationRejected      MutationOutcome = "rejected"
	MutationIndeterminate MutationOutcome = "indeterminate"
)

type ScheduleMutationError struct {
	Outcome MutationOutcome
	Err     error
}

type CheckoutMutationError struct {
	Outcome MutationOutcome
	Err     error
}

func (e *CheckoutMutationError) Error() string {
	if e == nil || e.Err == nil {
		return "Stripe Checkout mutation failed"
	}
	return e.Err.Error()
}

func (e *CheckoutMutationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *ScheduleMutationError) Error() string {
	if e == nil || e.Err == nil {
		return "Stripe schedule mutation failed"
	}
	return e.Err.Error()
}

func (e *ScheduleMutationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func classifyScheduleCreateError(err error) *ScheduleMutationError {
	outcome := MutationIndeterminate
	var stripeErr *stripe.Error
	if errors.As(err, &stripeErr) && stripeErr.Type != stripe.ErrorTypeIdempotency && stripeErr.HTTPStatusCode >= http.StatusBadRequest && stripeErr.HTTPStatusCode < http.StatusInternalServerError && stripeErr.HTTPStatusCode != http.StatusTooManyRequests {
		outcome = MutationRejected
	}
	return &ScheduleMutationError{Outcome: outcome, Err: err}
}

func classifyCheckoutCreateError(err error) *CheckoutMutationError {
	outcome := MutationIndeterminate
	var stripeErr *stripe.Error
	if errors.As(err, &stripeErr) && stripeErr.Type != stripe.ErrorTypeIdempotency && stripeErr.HTTPStatusCode >= http.StatusBadRequest && stripeErr.HTTPStatusCode < http.StatusInternalServerError && stripeErr.HTTPStatusCode != http.StatusTooManyRequests {
		outcome = MutationRejected
	}
	return &CheckoutMutationError{Outcome: outcome, Err: err}
}

const (
	metadataWorkspaceID = "workspace_id"
	metadataPlanID      = "plan_id"
	metadataTrialGrant  = "trial_grant_id"
	metadataTrialKind   = "trial_kind"
	metadataEnvironment = "unipost_environment"
)

// StripeGateway is the complete Stripe surface used by the trial service.
// Keeping this interface narrow makes service tests deterministic and prevents
// trial plan changes from reaching Stripe's schedule Release endpoint.
type StripeGateway interface {
	CreatePaidTrialSchedule(context.Context, CreatePaidTrialScheduleRequest) (ScheduleSnapshot, error)
	ConfigurePaidTrialSchedule(context.Context, string, CreatePaidTrialScheduleRequest) (ScheduleSnapshot, error)
	CreateTrialCheckout(context.Context, CreateTrialCheckoutRequest) (CheckoutSnapshot, error)
	RetrieveCheckout(context.Context, string, string) (CheckoutSnapshot, error)
	ExpireCheckout(context.Context, ExpireCheckoutRequest) (CheckoutSnapshot, error)
	RetrieveSubscription(context.Context, string, string) (SubscriptionSnapshot, error)
	ChangeFreeTrialPlanNow(context.Context, ChangeFreeTrialPlanRequest) (SubscriptionSnapshot, error)
	ChangeScheduledTrialPlanNow(context.Context, ChangeScheduledTrialPlanRequest) (ScheduleSnapshot, error)
	CancelFreeTrialAtEnd(context.Context, CancelFreeTrialRequest) (SubscriptionSnapshot, error)
	CancelPaidScheduleAtTrialEnd(context.Context, CancelPaidScheduleRequest) (ScheduleSnapshot, error)
	CreatePortal(context.Context, CreatePortalRequest) (string, error)
}

type CreatePaidTrialScheduleRequest struct {
	StripeMode     string
	WorkspaceID    string
	PlanID         string
	TrialGrantID   string
	TrialKind      Kind
	Environment    string
	SubscriptionID string
	PriceID        string
	DurationDays   int32
	CurrentPhase   SchedulePhase
	TrialStartAt   time.Time
	TrialEndAt     time.Time
}

type CreateTrialCheckoutRequest struct {
	StripeMode      string
	WorkspaceID     string
	PlanID          string
	TrialGrantID    string
	TrialKind       Kind
	Environment     string
	CustomerID      string
	PriceID         string
	DurationDays    int32
	CheckoutAttempt int32
	SuccessURL      string
	CancelURL       string
}

type ExpireCheckoutRequest struct {
	StripeMode        string
	TrialGrantID      string
	CheckoutSessionID string
	CheckoutAttempt   int32
}

type ChangeFreeTrialPlanRequest struct {
	StripeMode         string
	WorkspaceID        string
	PlanID             string
	TrialGrantID       string
	TrialKind          Kind
	Environment        string
	SubscriptionID     string
	SubscriptionItemID string
	PriceID            string
}

type ChangeScheduledTrialPlanRequest struct {
	StripeMode   string
	WorkspaceID  string
	PlanID       string
	TrialGrantID string
	TrialKind    Kind
	Environment  string
	ScheduleID   string
	PriceID      string
}

type CancelFreeTrialRequest struct {
	StripeMode     string
	WorkspaceID    string
	PlanID         string
	TrialGrantID   string
	TrialKind      Kind
	Environment    string
	SubscriptionID string
}

type CancelPaidScheduleRequest struct {
	StripeMode     string
	WorkspaceID    string
	PlanID         string
	TrialGrantID   string
	TrialKind      Kind
	Environment    string
	ScheduleID     string
	RetainedPhases []SchedulePhase
}

type CreatePortalRequest struct {
	StripeMode                    string
	CustomerID                    string
	ReturnURL                     string
	RequireTrialSafeConfiguration bool
}

// SchedulePhase is the normalized schedule shape needed to preserve current
// and trial phases during Stripe schedule updates.
type SchedulePhase struct {
	PriceID            string
	StartAt            time.Time
	EndAt              time.Time
	TrialEndAt         time.Time
	BillingCycleAnchor string
	Metadata           map[string]string
}

type CheckoutSnapshot struct {
	StripeMode     string
	ID             string
	Status         string
	PaymentStatus  string
	URL            string
	CustomerID     string
	SubscriptionID string
	ExpiresAt      *time.Time
	Metadata       map[string]string
}

type SubscriptionSnapshot struct {
	StripeMode           string
	ID                   string
	Status               string
	CustomerID           string
	ScheduleID           string
	ItemID               string
	PriceID              string
	TrialStartAt         *time.Time
	TrialEndAt           *time.Time
	CurrentPeriodStartAt *time.Time
	CurrentPeriodEndAt   *time.Time
	CancelAt             *time.Time
	CancelAtPeriodEnd    bool
	CanceledAt           *time.Time
	EndedAt              *time.Time
	Metadata             map[string]string
}

type ScheduleSnapshot struct {
	StripeMode             string
	ID                     string
	Status                 string
	EndBehavior            string
	CustomerID             string
	SubscriptionID         string
	ReleasedSubscriptionID string
	CurrentPhaseStartAt    *time.Time
	CurrentPhaseEndAt      *time.Time
	CanceledAt             *time.Time
	CompletedAt            *time.Time
	ReleasedAt             *time.Time
	Phases                 []SchedulePhase
	Metadata               map[string]string
}

type stripeGateway struct {
	manager       *billing.Manager
	clientForMode func(*billing.Mode) stripeTrialClient
}

type stripeTrialClient interface {
	createCheckout(*stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error)
	retrieveCheckout(string, *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error)
	expireCheckout(string, *stripe.CheckoutSessionExpireParams) (*stripe.CheckoutSession, error)
	retrieveSubscription(string, *stripe.SubscriptionParams) (*stripe.Subscription, error)
	updateSubscription(string, *stripe.SubscriptionParams) (*stripe.Subscription, error)
	createSchedule(*stripe.SubscriptionScheduleParams) (*stripe.SubscriptionSchedule, error)
	updateSchedule(string, *stripe.SubscriptionScheduleParams) (*stripe.SubscriptionSchedule, error)
	createPortal(*stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error)
}

type billingStripeTrialClient struct {
	mode *billing.Mode
}

func (c *billingStripeTrialClient) createCheckout(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
	return c.mode.Client.CheckoutSessions.New(params)
}

func (c *billingStripeTrialClient) retrieveCheckout(id string, params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
	return c.mode.Client.CheckoutSessions.Get(id, params)
}

func (c *billingStripeTrialClient) expireCheckout(id string, params *stripe.CheckoutSessionExpireParams) (*stripe.CheckoutSession, error) {
	return c.mode.Client.CheckoutSessions.Expire(id, params)
}

func (c *billingStripeTrialClient) retrieveSubscription(id string, params *stripe.SubscriptionParams) (*stripe.Subscription, error) {
	return c.mode.Client.Subscriptions.Get(id, params)
}

func (c *billingStripeTrialClient) updateSubscription(id string, params *stripe.SubscriptionParams) (*stripe.Subscription, error) {
	return c.mode.Client.Subscriptions.Update(id, params)
}

func (c *billingStripeTrialClient) createSchedule(params *stripe.SubscriptionScheduleParams) (*stripe.SubscriptionSchedule, error) {
	return c.mode.Client.SubscriptionSchedules.New(params)
}

func (c *billingStripeTrialClient) updateSchedule(id string, params *stripe.SubscriptionScheduleParams) (*stripe.SubscriptionSchedule, error) {
	return c.mode.Client.SubscriptionSchedules.Update(id, params)
}

func (c *billingStripeTrialClient) createPortal(params *stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
	return c.mode.Client.BillingPortalSessions.New(params)
}

var _ StripeGateway = (*stripeGateway)(nil)

func NewStripeGateway(manager *billing.Manager) StripeGateway {
	return &stripeGateway{
		manager: manager,
		clientForMode: func(mode *billing.Mode) stripeTrialClient {
			return &billingStripeTrialClient{mode: mode}
		},
	}
}

func (g *stripeGateway) CreatePaidTrialSchedule(ctx context.Context, req CreatePaidTrialScheduleRequest) (ScheduleSnapshot, error) {
	createParams, updateParams, err := buildPaidTrialScheduleParams(req)
	if err != nil {
		return ScheduleSnapshot{}, err
	}
	mode, err := g.mode(req.StripeMode)
	if err != nil {
		return ScheduleSnapshot{}, err
	}
	client, err := g.client(mode)
	if err != nil {
		return ScheduleSnapshot{}, err
	}
	createParams.Context = ctx
	created, err := client.createSchedule(createParams)
	if err != nil {
		return ScheduleSnapshot{}, fmt.Errorf("create Stripe trial schedule: %w", classifyScheduleCreateError(err))
	}
	if err := validateScheduleResponse(created, ""); err != nil {
		return ScheduleSnapshot{}, fmt.Errorf("create Stripe trial schedule: %w", &ScheduleMutationError{Outcome: MutationIndeterminate, Err: err})
	}

	// Stripe forbids phases when creating from_subscription, so this operation
	// necessarily uses create+update. Both calls have stable sub-operation keys.
	// If update fails, return the created schedule snapshot and ID so a retry can
	// reconcile the partial success before repeating the idempotent update.
	updateParams.Context = ctx
	configured, err := client.updateSchedule(created.ID, updateParams)
	if err != nil {
		return scheduleSnapshot(req.StripeMode, created), fmt.Errorf("configure created Stripe trial schedule %s: %w", created.ID, &ScheduleMutationError{Outcome: MutationIndeterminate, Err: err})
	}
	if err := validateScheduleResponse(configured, created.ID); err != nil {
		return scheduleSnapshot(req.StripeMode, created), fmt.Errorf("configure created Stripe trial schedule %s: %w", created.ID, &ScheduleMutationError{Outcome: MutationIndeterminate, Err: err})
	}
	return scheduleSnapshot(req.StripeMode, configured), nil
}

func (g *stripeGateway) ConfigurePaidTrialSchedule(ctx context.Context, scheduleID string, req CreatePaidTrialScheduleRequest) (ScheduleSnapshot, error) {
	if err := requireID("subscription schedule", scheduleID); err != nil {
		return ScheduleSnapshot{}, err
	}
	_, updateParams, err := buildPaidTrialScheduleParams(req)
	if err != nil {
		return ScheduleSnapshot{}, err
	}
	mode, err := g.mode(req.StripeMode)
	if err != nil {
		return ScheduleSnapshot{}, err
	}
	client, err := g.client(mode)
	if err != nil {
		return ScheduleSnapshot{}, err
	}
	updateParams.Context = ctx
	configured, err := client.updateSchedule(scheduleID, updateParams)
	if err != nil {
		return ScheduleSnapshot{StripeMode: req.StripeMode, ID: scheduleID}, fmt.Errorf("configure existing Stripe trial schedule %s: %w", scheduleID, &ScheduleMutationError{Outcome: MutationIndeterminate, Err: err})
	}
	if err := validateScheduleResponse(configured, scheduleID); err != nil {
		return ScheduleSnapshot{StripeMode: req.StripeMode, ID: scheduleID}, fmt.Errorf("configure existing Stripe trial schedule %s: %w", scheduleID, &ScheduleMutationError{Outcome: MutationIndeterminate, Err: err})
	}
	return scheduleSnapshot(req.StripeMode, configured), nil
}

func (g *stripeGateway) CreateTrialCheckout(ctx context.Context, req CreateTrialCheckoutRequest) (CheckoutSnapshot, error) {
	params, err := buildTrialCheckoutParams(req)
	if err != nil {
		return CheckoutSnapshot{}, &CheckoutMutationError{Outcome: MutationRejected, Err: err}
	}
	mode, err := g.mode(req.StripeMode)
	if err != nil {
		return CheckoutSnapshot{}, &CheckoutMutationError{Outcome: MutationRejected, Err: err}
	}
	client, err := g.client(mode)
	if err != nil {
		return CheckoutSnapshot{}, &CheckoutMutationError{Outcome: MutationRejected, Err: err}
	}
	params.Context = ctx
	session, err := client.createCheckout(params)
	if err != nil {
		return CheckoutSnapshot{}, fmt.Errorf("create Stripe trial checkout: %w", classifyCheckoutCreateError(err))
	}
	if err := validateCheckoutResponse(session, "", true); err != nil {
		return CheckoutSnapshot{}, fmt.Errorf("create Stripe trial checkout: %w", &CheckoutMutationError{Outcome: MutationIndeterminate, Err: err})
	}
	return checkoutSnapshot(req.StripeMode, session), nil
}

func (g *stripeGateway) RetrieveCheckout(ctx context.Context, modeName, sessionID string) (CheckoutSnapshot, error) {
	if err := validateLookup("checkout session", modeName, sessionID); err != nil {
		return CheckoutSnapshot{}, err
	}
	mode, err := g.mode(modeName)
	if err != nil {
		return CheckoutSnapshot{}, err
	}
	client, err := g.client(mode)
	if err != nil {
		return CheckoutSnapshot{}, err
	}
	params := &stripe.CheckoutSessionParams{}
	params.Context = ctx
	session, err := client.retrieveCheckout(sessionID, params)
	if err != nil {
		return CheckoutSnapshot{}, fmt.Errorf("retrieve Stripe checkout %s: %w", sessionID, err)
	}
	if err := validateCheckoutResponse(session, sessionID, false); err != nil {
		return CheckoutSnapshot{}, fmt.Errorf("retrieve Stripe checkout %s: %w", sessionID, err)
	}
	return checkoutSnapshot(modeName, session), nil
}

func (g *stripeGateway) ExpireCheckout(ctx context.Context, req ExpireCheckoutRequest) (CheckoutSnapshot, error) {
	params, err := buildExpireCheckoutParams(req)
	if err != nil {
		return CheckoutSnapshot{}, err
	}
	mode, err := g.mode(req.StripeMode)
	if err != nil {
		return CheckoutSnapshot{}, err
	}
	client, err := g.client(mode)
	if err != nil {
		return CheckoutSnapshot{}, err
	}
	params.Context = ctx
	session, err := client.expireCheckout(req.CheckoutSessionID, params)
	if err != nil {
		return CheckoutSnapshot{}, fmt.Errorf("expire Stripe checkout %s: %w", req.CheckoutSessionID, err)
	}
	if err := validateCheckoutResponse(session, req.CheckoutSessionID, false); err != nil {
		return CheckoutSnapshot{}, fmt.Errorf("expire Stripe checkout %s: %w", req.CheckoutSessionID, err)
	}
	return checkoutSnapshot(req.StripeMode, session), nil
}

func (g *stripeGateway) RetrieveSubscription(ctx context.Context, modeName, subscriptionID string) (SubscriptionSnapshot, error) {
	if err := validateLookup("subscription", modeName, subscriptionID); err != nil {
		return SubscriptionSnapshot{}, err
	}
	mode, err := g.mode(modeName)
	if err != nil {
		return SubscriptionSnapshot{}, err
	}
	client, err := g.client(mode)
	if err != nil {
		return SubscriptionSnapshot{}, err
	}
	params := &stripe.SubscriptionParams{}
	params.Context = ctx
	subscription, err := client.retrieveSubscription(subscriptionID, params)
	if err != nil {
		return SubscriptionSnapshot{}, fmt.Errorf("retrieve Stripe subscription %s: %w", subscriptionID, err)
	}
	if err := validateSubscriptionResponse(subscription, subscriptionID); err != nil {
		return SubscriptionSnapshot{}, fmt.Errorf("retrieve Stripe subscription %s: %w", subscriptionID, err)
	}
	return subscriptionSnapshot(modeName, subscription), nil
}

func (g *stripeGateway) ChangeFreeTrialPlanNow(ctx context.Context, req ChangeFreeTrialPlanRequest) (SubscriptionSnapshot, error) {
	params, err := buildChangeFreeTrialPlanParams(req)
	if err != nil {
		return SubscriptionSnapshot{}, err
	}
	mode, err := g.mode(req.StripeMode)
	if err != nil {
		return SubscriptionSnapshot{}, err
	}
	client, err := g.client(mode)
	if err != nil {
		return SubscriptionSnapshot{}, err
	}
	params.Context = ctx
	subscription, err := client.updateSubscription(req.SubscriptionID, params)
	if err != nil {
		return SubscriptionSnapshot{}, fmt.Errorf("change Stripe free-trial plan: %w", err)
	}
	if err := validateSubscriptionResponse(subscription, req.SubscriptionID); err != nil {
		return SubscriptionSnapshot{}, fmt.Errorf("change Stripe free-trial plan: %w", err)
	}
	return subscriptionSnapshot(req.StripeMode, subscription), nil
}

func (g *stripeGateway) ChangeScheduledTrialPlanNow(ctx context.Context, req ChangeScheduledTrialPlanRequest) (ScheduleSnapshot, error) {
	mode, err := g.mode(req.StripeMode)
	if err != nil {
		return ScheduleSnapshot{}, err
	}
	client, err := g.client(mode)
	if err != nil {
		return ScheduleSnapshot{}, err
	}
	return changeScheduledTrialPlanNow(ctx, client, req)
}

type schedulePlanChanger interface {
	updateSchedule(string, *stripe.SubscriptionScheduleParams) (*stripe.SubscriptionSchedule, error)
}

func changeScheduledTrialPlanNow(ctx context.Context, client schedulePlanChanger, req ChangeScheduledTrialPlanRequest) (ScheduleSnapshot, error) {
	params, err := buildChangeScheduledTrialPlanParams(req)
	if err != nil {
		return ScheduleSnapshot{}, err
	}
	params.Context = ctx
	schedule, err := client.updateSchedule(req.ScheduleID, params)
	if err != nil {
		return ScheduleSnapshot{}, fmt.Errorf("change Stripe scheduled-trial plan: %w", err)
	}
	if err := validateScheduleResponse(schedule, req.ScheduleID); err != nil {
		return ScheduleSnapshot{}, fmt.Errorf("change Stripe scheduled-trial plan: %w", err)
	}
	return scheduleSnapshot(req.StripeMode, schedule), nil
}

func (g *stripeGateway) CancelFreeTrialAtEnd(ctx context.Context, req CancelFreeTrialRequest) (SubscriptionSnapshot, error) {
	params, err := buildCancelFreeTrialParams(req)
	if err != nil {
		return SubscriptionSnapshot{}, err
	}
	mode, err := g.mode(req.StripeMode)
	if err != nil {
		return SubscriptionSnapshot{}, err
	}
	client, err := g.client(mode)
	if err != nil {
		return SubscriptionSnapshot{}, err
	}
	params.Context = ctx
	subscription, err := client.updateSubscription(req.SubscriptionID, params)
	if err != nil {
		return SubscriptionSnapshot{}, fmt.Errorf("cancel Stripe free-trial renewal: %w", err)
	}
	if err := validateSubscriptionResponse(subscription, req.SubscriptionID); err != nil {
		return SubscriptionSnapshot{}, fmt.Errorf("cancel Stripe free-trial renewal: %w", err)
	}
	return subscriptionSnapshot(req.StripeMode, subscription), nil
}

func (g *stripeGateway) CancelPaidScheduleAtTrialEnd(ctx context.Context, req CancelPaidScheduleRequest) (ScheduleSnapshot, error) {
	params, err := buildCancelPaidScheduleParams(req)
	if err != nil {
		return ScheduleSnapshot{}, err
	}
	mode, err := g.mode(req.StripeMode)
	if err != nil {
		return ScheduleSnapshot{}, err
	}
	client, err := g.client(mode)
	if err != nil {
		return ScheduleSnapshot{}, err
	}
	params.Context = ctx
	schedule, err := client.updateSchedule(req.ScheduleID, params)
	if err != nil {
		return ScheduleSnapshot{}, fmt.Errorf("cancel Stripe paid trial schedule renewal: %w", err)
	}
	if err := validateScheduleResponse(schedule, req.ScheduleID); err != nil {
		return ScheduleSnapshot{}, fmt.Errorf("cancel Stripe paid trial schedule renewal: %w", err)
	}
	return scheduleSnapshot(req.StripeMode, schedule), nil
}

func (g *stripeGateway) CreatePortal(ctx context.Context, req CreatePortalRequest) (string, error) {
	mode, err := g.mode(req.StripeMode)
	if err != nil {
		return "", err
	}
	params, err := buildPortalParams(req, mode.TrialPortalConfigurationID())
	if err != nil {
		return "", err
	}
	client, err := g.client(mode)
	if err != nil {
		return "", err
	}
	params.Context = ctx
	session, err := client.createPortal(params)
	if err != nil {
		return "", fmt.Errorf("create Stripe billing portal: %w", err)
	}
	if session == nil || strings.TrimSpace(session.URL) == "" {
		return "", fmt.Errorf("create Stripe billing portal: Stripe returned no URL")
	}
	return session.URL, nil
}

func (g *stripeGateway) mode(name string) (*billing.Mode, error) {
	if g == nil || g.manager == nil {
		return nil, fmt.Errorf("Stripe trial gateway is not configured")
	}
	mode := g.manager.ByName(name)
	if mode == nil {
		return nil, fmt.Errorf("Stripe mode %q is unavailable", name)
	}
	return mode, nil
}

func (g *stripeGateway) client(mode *billing.Mode) (stripeTrialClient, error) {
	if g == nil || g.clientForMode == nil {
		return nil, fmt.Errorf("Stripe trial gateway client is not configured")
	}
	if mode == nil {
		return nil, fmt.Errorf("Stripe mode is unavailable")
	}
	client := g.clientForMode(mode)
	if client == nil {
		return nil, fmt.Errorf("Stripe client for mode %q is unavailable", mode.Name)
	}
	if production, ok := client.(*billingStripeTrialClient); ok && (production.mode == nil || production.mode.Client == nil) {
		return nil, fmt.Errorf("Stripe client for mode %q is unavailable", mode.Name)
	}
	return client, nil
}

func buildTrialCheckoutParams(req CreateTrialCheckoutRequest) (*stripe.CheckoutSessionParams, error) {
	if err := validateTrialRequestIdentity(req.StripeMode, req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment); err != nil {
		return nil, err
	}
	if req.TrialKind != KindFreeToPaid {
		return nil, fmt.Errorf("trial checkout requires trial kind %q", KindFreeToPaid)
	}
	if err := requireID("customer", req.CustomerID); err != nil {
		return nil, err
	}
	if err := requireID("price", req.PriceID); err != nil {
		return nil, err
	}
	if req.DurationDays < 1 || req.DurationDays > 730 {
		return nil, fmt.Errorf("trial checkout duration must be between 1 and 730 days")
	}
	if req.CheckoutAttempt < 1 {
		return nil, fmt.Errorf("trial checkout attempt must be positive")
	}
	if err := validateReturnURL("checkout success", req.SuccessURL); err != nil {
		return nil, err
	}
	if err := validateReturnURL("checkout cancel", req.CancelURL); err != nil {
		return nil, err
	}
	metadata := trialMetadata(req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment)
	params := &stripe.CheckoutSessionParams{
		Mode:                    stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		PaymentMethodCollection: stripe.String(string(stripe.CheckoutSessionPaymentMethodCollectionAlways)),
		SuccessURL:              stripe.String(req.SuccessURL),
		CancelURL:               stripe.String(req.CancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{{
			Price:    stripe.String(req.PriceID),
			Quantity: stripe.Int64(1),
		}},
		Metadata: metadata,
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			TrialPeriodDays: stripe.Int64(int64(req.DurationDays)),
			Metadata:        cloneMetadata(metadata),
		},
	}
	params.Customer = stripe.String(req.CustomerID)
	params.SetIdempotencyKey(fmt.Sprintf("%s:%d", trialOperationKey(req.TrialGrantID, "checkout"), req.CheckoutAttempt))
	return params, nil
}

func buildExpireCheckoutParams(req ExpireCheckoutRequest) (*stripe.CheckoutSessionExpireParams, error) {
	if err := validateStripeMode(req.StripeMode); err != nil {
		return nil, err
	}
	if err := requireTrialGrantID(req.TrialGrantID); err != nil {
		return nil, err
	}
	if err := requireID("checkout session", req.CheckoutSessionID); err != nil {
		return nil, err
	}
	if req.CheckoutAttempt < 1 {
		return nil, fmt.Errorf("checkout attempt is required")
	}
	params := &stripe.CheckoutSessionExpireParams{}
	params.SetIdempotencyKey(fmt.Sprintf("%s:%d", trialOperationKey(req.TrialGrantID, "expire_checkout"), req.CheckoutAttempt))
	return params, nil
}

func buildPaidTrialScheduleParams(req CreatePaidTrialScheduleRequest) (*stripe.SubscriptionScheduleParams, *stripe.SubscriptionScheduleParams, error) {
	if err := validatePaidTrialScheduleRequest(req); err != nil {
		return nil, nil, err
	}
	metadata := trialMetadata(req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment)
	createParams := &stripe.SubscriptionScheduleParams{
		FromSubscription: stripe.String(req.SubscriptionID),
		Metadata:         cloneMetadata(metadata),
	}
	createParams.SetIdempotencyKey(trialOperationKey(req.TrialGrantID, "schedule") + ":create")

	current := req.CurrentPhase
	trial := SchedulePhase{
		PriceID:    req.PriceID,
		StartAt:    req.TrialStartAt,
		EndAt:      req.TrialEndAt,
		TrialEndAt: req.TrialEndAt,
	}
	resumedPaid := SchedulePhase{
		PriceID:            req.PriceID,
		StartAt:            req.TrialEndAt,
		BillingCycleAnchor: string(stripe.SubscriptionSchedulePhaseBillingCycleAnchorPhaseStart),
	}
	updateParams := &stripe.SubscriptionScheduleParams{
		EndBehavior:       stripe.String(string(stripe.SubscriptionScheduleEndBehaviorRelease)),
		ProrationBehavior: stripe.String("none"),
		Metadata:          cloneMetadata(metadata),
		Phases: []*stripe.SubscriptionSchedulePhaseParams{
			buildSchedulePhaseParams(current, metadata),
			buildSchedulePhaseParams(trial, metadata),
			buildSchedulePhaseParams(resumedPaid, metadata),
		},
	}
	updateParams.SetIdempotencyKey(trialOperationKey(req.TrialGrantID, "schedule") + ":update")
	return createParams, updateParams, nil
}

func buildChangeFreeTrialPlanParams(req ChangeFreeTrialPlanRequest) (*stripe.SubscriptionParams, error) {
	if err := validateTrialRequestIdentity(req.StripeMode, req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment); err != nil {
		return nil, err
	}
	if req.TrialKind != KindFreeToPaid {
		return nil, fmt.Errorf("free trial plan change requires trial kind %q", KindFreeToPaid)
	}
	if err := requireID("subscription", req.SubscriptionID); err != nil {
		return nil, err
	}
	if err := requireID("subscription item", req.SubscriptionItemID); err != nil {
		return nil, err
	}
	if err := requireID("price", req.PriceID); err != nil {
		return nil, err
	}
	params := &stripe.SubscriptionParams{
		Items: []*stripe.SubscriptionItemsParams{{
			ID:    stripe.String(req.SubscriptionItemID),
			Price: stripe.String(req.PriceID),
		}},
		TrialEndNow:           stripe.Bool(true),
		BillingCycleAnchorNow: stripe.Bool(true),
		ProrationBehavior:     stripe.String("none"),
		PaymentBehavior:       stripe.String("error_if_incomplete"),
		Metadata:              trialMetadata(req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment),
	}
	params.SetIdempotencyKey(trialOperationKey(req.TrialGrantID, "change_plan"))
	return params, nil
}

func buildChangeScheduledTrialPlanParams(req ChangeScheduledTrialPlanRequest) (*stripe.SubscriptionScheduleParams, error) {
	if err := validateTrialRequestIdentity(req.StripeMode, req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment); err != nil {
		return nil, err
	}
	if req.TrialKind != KindPaidSamePlan {
		return nil, fmt.Errorf("scheduled trial plan change requires trial kind %q", KindPaidSamePlan)
	}
	if err := requireID("schedule", req.ScheduleID); err != nil {
		return nil, err
	}
	if err := requireID("price", req.PriceID); err != nil {
		return nil, err
	}
	metadata := trialMetadata(req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment)
	params := &stripe.SubscriptionScheduleParams{
		EndBehavior:       stripe.String(string(stripe.SubscriptionScheduleEndBehaviorRelease)),
		ProrationBehavior: stripe.String("none"),
		Metadata:          cloneMetadata(metadata),
		Phases: []*stripe.SubscriptionSchedulePhaseParams{{
			StartDateNow:       stripe.Bool(true),
			BillingCycleAnchor: stripe.String(string(stripe.SubscriptionSchedulePhaseBillingCycleAnchorPhaseStart)),
			ProrationBehavior:  stripe.String(string(stripe.SubscriptionSchedulePhaseProrationBehaviorNone)),
			Items: []*stripe.SubscriptionSchedulePhaseItemParams{{
				Price: stripe.String(req.PriceID),
			}},
			Metadata: cloneMetadata(metadata),
		}},
	}
	params.SetIdempotencyKey(trialOperationKey(req.TrialGrantID, "change_plan"))
	return params, nil
}

func buildCancelFreeTrialParams(req CancelFreeTrialRequest) (*stripe.SubscriptionParams, error) {
	if err := validateTrialRequestIdentity(req.StripeMode, req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment); err != nil {
		return nil, err
	}
	if req.TrialKind != KindFreeToPaid {
		return nil, fmt.Errorf("free trial cancellation requires trial kind %q", KindFreeToPaid)
	}
	if err := requireID("subscription", req.SubscriptionID); err != nil {
		return nil, err
	}
	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(true),
		Metadata:          trialMetadata(req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment),
	}
	params.SetIdempotencyKey(trialOperationKey(req.TrialGrantID, "cancel_renewal"))
	return params, nil
}

func buildCancelPaidScheduleParams(req CancelPaidScheduleRequest) (*stripe.SubscriptionScheduleParams, error) {
	if err := validateTrialRequestIdentity(req.StripeMode, req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment); err != nil {
		return nil, err
	}
	if req.TrialKind != KindPaidSamePlan {
		return nil, fmt.Errorf("paid trial cancellation requires trial kind %q", KindPaidSamePlan)
	}
	if err := requireID("schedule", req.ScheduleID); err != nil {
		return nil, err
	}
	if len(req.RetainedPhases) == 0 {
		return nil, fmt.Errorf("paid trial cancellation requires retained phases")
	}
	for index, phase := range req.RetainedPhases {
		if err := validateSchedulePhase(fmt.Sprintf("retained %d", index), phase, true); err != nil {
			return nil, err
		}
		if index > 0 && !req.RetainedPhases[index-1].EndAt.Equal(phase.StartAt) {
			return nil, fmt.Errorf("retained schedule phases must be contiguous")
		}
	}
	metadata := trialMetadata(req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment)
	phases := make([]*stripe.SubscriptionSchedulePhaseParams, 0, len(req.RetainedPhases))
	for _, phase := range req.RetainedPhases {
		phases = append(phases, buildSchedulePhaseParams(phase, metadata))
	}
	params := &stripe.SubscriptionScheduleParams{
		EndBehavior: stripe.String(string(stripe.SubscriptionScheduleEndBehaviorCancel)),
		Metadata:    cloneMetadata(metadata),
		Phases:      phases,
	}
	params.SetIdempotencyKey(trialOperationKey(req.TrialGrantID, "cancel_renewal"))
	return params, nil
}

func buildPortalParams(req CreatePortalRequest, trialConfigurationID string) (*stripe.BillingPortalSessionParams, error) {
	if err := validateStripeMode(req.StripeMode); err != nil {
		return nil, err
	}
	if err := requireID("customer", req.CustomerID); err != nil {
		return nil, err
	}
	if err := validateReturnURL("billing portal return", req.ReturnURL); err != nil {
		return nil, err
	}
	if req.RequireTrialSafeConfiguration && strings.TrimSpace(trialConfigurationID) == "" {
		return nil, fmt.Errorf("trial-safe Stripe Billing Portal configuration is required for mode %q", req.StripeMode)
	}
	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(req.CustomerID),
		ReturnURL: stripe.String(req.ReturnURL),
	}
	if req.RequireTrialSafeConfiguration {
		params.Configuration = stripe.String(trialConfigurationID)
	}
	return params, nil
}

func buildSchedulePhaseParams(phase SchedulePhase, requiredMetadata map[string]string) *stripe.SubscriptionSchedulePhaseParams {
	metadata := cloneMetadata(phase.Metadata)
	for key, value := range requiredMetadata {
		metadata[key] = value
	}
	params := &stripe.SubscriptionSchedulePhaseParams{
		Items:    []*stripe.SubscriptionSchedulePhaseItemParams{{Price: stripe.String(phase.PriceID)}},
		Metadata: metadata,
	}
	if !phase.StartAt.IsZero() {
		params.StartDate = stripe.Int64(phase.StartAt.Unix())
	}
	if !phase.EndAt.IsZero() {
		params.EndDate = stripe.Int64(phase.EndAt.Unix())
	}
	if !phase.TrialEndAt.IsZero() {
		if !phase.EndAt.IsZero() && phase.TrialEndAt.Equal(phase.EndAt) {
			// Stripe requires an explicit trial_end to be before end_date. A
			// full-phase trial is represented by trial=true instead.
			params.Trial = stripe.Bool(true)
		} else {
			params.TrialEnd = stripe.Int64(phase.TrialEndAt.Unix())
		}
	}
	if phase.BillingCycleAnchor != "" {
		params.BillingCycleAnchor = stripe.String(phase.BillingCycleAnchor)
	}
	return params
}

func trialMetadata(workspaceID, planID, grantID string, kind Kind, environment string) map[string]string {
	return map[string]string{
		metadataWorkspaceID: workspaceID,
		metadataPlanID:      planID,
		metadataTrialGrant:  grantID,
		metadataTrialKind:   string(kind),
		metadataEnvironment: environment,
	}
}

func trialOperationKey(grantID, operation string) string {
	return "trial:" + grantID + ":" + operation
}

func requireTrialGrantID(grantID string) error {
	if strings.TrimSpace(grantID) == "" {
		return fmt.Errorf("trial grant ID is required for Stripe idempotency")
	}
	return nil
}

func validatePaidTrialScheduleRequest(req CreatePaidTrialScheduleRequest) error {
	if err := validateTrialRequestIdentity(req.StripeMode, req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment); err != nil {
		return err
	}
	if req.TrialKind != KindPaidSamePlan {
		return fmt.Errorf("paid trial schedule requires trial kind %q", KindPaidSamePlan)
	}
	if err := requireID("subscription", req.SubscriptionID); err != nil {
		return err
	}
	if err := requireID("price", req.PriceID); err != nil {
		return err
	}
	if req.DurationDays < 1 || req.DurationDays > 730 {
		return fmt.Errorf("paid trial duration must be between 1 and 730 days")
	}
	if err := validateSchedulePhase("current", req.CurrentPhase, true); err != nil {
		return err
	}
	if req.CurrentPhase.PriceID != req.PriceID {
		return fmt.Errorf("current phase price must match paid trial price")
	}
	if !req.CurrentPhase.TrialEndAt.IsZero() {
		return fmt.Errorf("current paid phase must not already be trialing")
	}
	if !req.CurrentPhase.EndAt.Equal(req.TrialStartAt) {
		return fmt.Errorf("current phase end must equal trial start")
	}
	if err := requireUTCTime("trial start", req.TrialStartAt); err != nil {
		return err
	}
	if err := requireUTCTime("trial end", req.TrialEndAt); err != nil {
		return err
	}
	wantEnd := req.TrialStartAt.UTC().AddDate(0, 0, int(req.DurationDays))
	if !req.TrialEndAt.Equal(wantEnd) {
		return fmt.Errorf("trial end must equal trial start plus %d days", req.DurationDays)
	}
	return nil
}

func validateTrialRequestIdentity(mode, workspaceID, planID, grantID string, kind Kind, environment string) error {
	if err := validateStripeMode(mode); err != nil {
		return err
	}
	if err := requireID("workspace", workspaceID); err != nil {
		return err
	}
	if !isSupportedPlan(planID) {
		return fmt.Errorf("unsupported trial plan %q", planID)
	}
	if err := requireTrialGrantID(grantID); err != nil {
		return err
	}
	if kind != KindFreeToPaid && kind != KindPaidSamePlan {
		return fmt.Errorf("unsupported trial kind %q", kind)
	}
	if strings.TrimSpace(environment) == "" {
		return fmt.Errorf("environment is required")
	}
	return nil
}

func validateStripeMode(mode string) error {
	if mode != "live" && mode != "sandbox" {
		return fmt.Errorf("unsupported Stripe mode %q", mode)
	}
	return nil
}

func isSupportedPlan(planID string) bool {
	switch planID {
	case "api", "basic", "growth", "team":
		return true
	default:
		return false
	}
}

func requireID(label, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s ID is required", label)
	}
	return nil
}

func requireUTCTime(label string, value time.Time) error {
	if value.IsZero() {
		return fmt.Errorf("%s is required", label)
	}
	if value.Location() != time.UTC {
		return fmt.Errorf("%s must be UTC-normalized", label)
	}
	return nil
}

func validateSchedulePhase(label string, phase SchedulePhase, requireEnd bool) error {
	if err := requireID(label+" phase price", phase.PriceID); err != nil {
		return err
	}
	if err := requireUTCTime(label+" phase start", phase.StartAt); err != nil {
		return err
	}
	if requireEnd || !phase.EndAt.IsZero() {
		if err := requireUTCTime(label+" phase end", phase.EndAt); err != nil {
			return err
		}
		if !phase.EndAt.After(phase.StartAt) {
			return fmt.Errorf("%s phase end must be after start", label)
		}
	}
	if !phase.TrialEndAt.IsZero() {
		if err := requireUTCTime(label+" phase trial end", phase.TrialEndAt); err != nil {
			return err
		}
		if phase.TrialEndAt.Before(phase.StartAt) || (!phase.EndAt.IsZero() && phase.TrialEndAt.After(phase.EndAt)) {
			return fmt.Errorf("%s phase trial end must be within phase boundaries", label)
		}
	}
	if phase.BillingCycleAnchor != "" && phase.BillingCycleAnchor != "automatic" && phase.BillingCycleAnchor != "phase_start" {
		return fmt.Errorf("%s phase has unsupported billing cycle anchor %q", label, phase.BillingCycleAnchor)
	}
	return nil
}

func validateReturnURL(label, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return fmt.Errorf("%s URL must be absolute", label)
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme == "http" && parsed.Hostname() == "localhost" {
		return nil
	}
	return fmt.Errorf("%s URL must use https", label)
}

func validateLookup(label, mode, id string) error {
	if err := validateStripeMode(mode); err != nil {
		return err
	}
	return requireID(label, id)
}

func validateCheckoutResponse(session *stripe.CheckoutSession, expectedID string, requireURL bool) error {
	if session == nil || strings.TrimSpace(session.ID) == "" {
		return fmt.Errorf("Stripe returned no checkout session ID")
	}
	if expectedID != "" && session.ID != expectedID {
		return fmt.Errorf("Stripe returned checkout session %q, expected %q", session.ID, expectedID)
	}
	if requireURL && strings.TrimSpace(session.URL) == "" {
		return fmt.Errorf("Stripe returned no checkout URL")
	}
	return nil
}

func validateSubscriptionResponse(subscription *stripe.Subscription, expectedID string) error {
	if subscription == nil || strings.TrimSpace(subscription.ID) == "" {
		return fmt.Errorf("Stripe returned no subscription ID")
	}
	if subscription.ID != expectedID {
		return fmt.Errorf("Stripe returned subscription %q, expected %q", subscription.ID, expectedID)
	}
	return nil
}

func validateScheduleResponse(schedule *stripe.SubscriptionSchedule, expectedID string) error {
	if schedule == nil || strings.TrimSpace(schedule.ID) == "" {
		return fmt.Errorf("Stripe returned no schedule ID")
	}
	if expectedID != "" && schedule.ID != expectedID {
		return fmt.Errorf("Stripe returned schedule %q, expected %q", schedule.ID, expectedID)
	}
	return nil
}

func checkoutSnapshot(modeName string, session *stripe.CheckoutSession) CheckoutSnapshot {
	if session == nil {
		return CheckoutSnapshot{StripeMode: modeName}
	}
	return CheckoutSnapshot{
		StripeMode:     modeName,
		ID:             session.ID,
		Status:         string(session.Status),
		PaymentStatus:  string(session.PaymentStatus),
		URL:            session.URL,
		CustomerID:     stripeObjectID(session.Customer),
		SubscriptionID: stripeObjectID(session.Subscription),
		ExpiresAt:      unixTime(session.ExpiresAt),
		Metadata:       cloneMetadata(session.Metadata),
	}
}

func subscriptionSnapshot(modeName string, subscription *stripe.Subscription) SubscriptionSnapshot {
	if subscription == nil {
		return SubscriptionSnapshot{StripeMode: modeName}
	}
	snapshot := SubscriptionSnapshot{
		StripeMode:        modeName,
		ID:                subscription.ID,
		Status:            string(subscription.Status),
		CustomerID:        stripeObjectID(subscription.Customer),
		ScheduleID:        stripeObjectID(subscription.Schedule),
		TrialStartAt:      unixTime(subscription.TrialStart),
		TrialEndAt:        unixTime(subscription.TrialEnd),
		CancelAt:          unixTime(subscription.CancelAt),
		CancelAtPeriodEnd: subscription.CancelAtPeriodEnd,
		CanceledAt:        unixTime(subscription.CanceledAt),
		EndedAt:           unixTime(subscription.EndedAt),
		Metadata:          cloneMetadata(subscription.Metadata),
	}
	if subscription.Items != nil && len(subscription.Items.Data) > 0 && subscription.Items.Data[0] != nil {
		item := subscription.Items.Data[0]
		snapshot.ItemID = item.ID
		snapshot.PriceID = stripeObjectID(item.Price)
		snapshot.CurrentPeriodStartAt = unixTime(item.CurrentPeriodStart)
		snapshot.CurrentPeriodEndAt = unixTime(item.CurrentPeriodEnd)
	}
	return snapshot
}

func scheduleSnapshot(modeName string, schedule *stripe.SubscriptionSchedule) ScheduleSnapshot {
	if schedule == nil {
		return ScheduleSnapshot{StripeMode: modeName}
	}
	snapshot := ScheduleSnapshot{
		StripeMode:             modeName,
		ID:                     schedule.ID,
		Status:                 string(schedule.Status),
		EndBehavior:            string(schedule.EndBehavior),
		CustomerID:             stripeObjectID(schedule.Customer),
		SubscriptionID:         stripeObjectID(schedule.Subscription),
		ReleasedSubscriptionID: stripeObjectID(schedule.ReleasedSubscription),
		CanceledAt:             unixTime(schedule.CanceledAt),
		CompletedAt:            unixTime(schedule.CompletedAt),
		ReleasedAt:             unixTime(schedule.ReleasedAt),
		Metadata:               cloneMetadata(schedule.Metadata),
		Phases:                 make([]SchedulePhase, 0, len(schedule.Phases)),
	}
	if schedule.CurrentPhase != nil {
		snapshot.CurrentPhaseStartAt = unixTime(schedule.CurrentPhase.StartDate)
		snapshot.CurrentPhaseEndAt = unixTime(schedule.CurrentPhase.EndDate)
	}
	for _, phase := range schedule.Phases {
		if phase == nil {
			continue
		}
		normalized := SchedulePhase{
			StartAt:            unixTimeValue(phase.StartDate),
			EndAt:              unixTimeValue(phase.EndDate),
			TrialEndAt:         unixTimeValue(phase.TrialEnd),
			BillingCycleAnchor: string(phase.BillingCycleAnchor),
			Metadata:           cloneMetadata(phase.Metadata),
		}
		if len(phase.Items) > 0 && phase.Items[0] != nil {
			normalized.PriceID = stripeObjectID(phase.Items[0].Price)
		}
		snapshot.Phases = append(snapshot.Phases, normalized)
	}
	return snapshot
}

func stripeObjectID(value any) string {
	switch typed := value.(type) {
	case *stripe.Customer:
		if typed != nil {
			return typed.ID
		}
	case *stripe.Subscription:
		if typed != nil {
			return typed.ID
		}
	case *stripe.SubscriptionSchedule:
		if typed != nil {
			return typed.ID
		}
	case *stripe.Price:
		if typed != nil {
			return typed.ID
		}
	}
	return ""
}

func unixTime(seconds int64) *time.Time {
	if seconds == 0 {
		return nil
	}
	value := time.Unix(seconds, 0).UTC()
	return &value
}

func unixTimeValue(seconds int64) time.Time {
	if seconds == 0 {
		return time.Time{}
	}
	return time.Unix(seconds, 0).UTC()
}

func cloneMetadata(source map[string]string) map[string]string {
	clone := make(map[string]string, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}
