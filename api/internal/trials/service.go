package trials

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrWorkspaceNotFound          = errors.New("workspace not found")
	ErrGrantNotFound              = errors.New("trial grant not found")
	ErrOpenGrantExists            = errors.New("workspace already has an open trial grant")
	ErrPaidPlanMismatch           = errors.New("paid trial plan must match the current plan")
	ErrIneligibleSubscription     = errors.New("subscription is not eligible for a trial grant")
	ErrRevokeConflict             = errors.New("trial grant can no longer be revoked")
	ErrConcurrentTransition       = errors.New("trial grant changed concurrently")
	ErrBillingModeUnavailable     = errors.New("Stripe billing mode is unavailable")
	ErrUnrelatedSchedule          = errors.New("subscription already has an unrelated Stripe schedule")
	ErrCheckoutExpiryUnconfirmed  = errors.New("Checkout Session expiry could not be confirmed")
	ErrTrialCheckoutNotApplicable = errors.New("no matching trial grant applies to checkout")
	ErrCheckoutCompletionPending  = errors.New("Checkout completion is awaiting webhook projection")
	ErrCheckoutStateConflict      = errors.New("trial checkout state changed concurrently")
	ErrCheckoutCustomerRequired   = errors.New("Stripe customer is required to start trial checkout")
	ErrWebhookNotApplicable       = errors.New("trial webhook is not applicable to this environment")
	ErrWebhookStateConflict       = errors.New("trial webhook does not match the recorded grant")
)

type OpenGrantConflictError struct {
	Current Grant
}

func (e *OpenGrantConflictError) Error() string { return ErrOpenGrantExists.Error() }
func (e *OpenGrantConflictError) Unwrap() error { return ErrOpenGrantExists }

type ConflictSummary struct {
	ID               string     `json:"id"`
	Kind             Kind       `json:"kind"`
	PlanID           string     `json:"plan_id"`
	DurationDays     int32      `json:"duration_days"`
	Status           Status     `json:"status"`
	GrantedAt        time.Time  `json:"granted_at"`
	ScheduledStartAt *time.Time `json:"scheduled_start_at,omitempty"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	EndsAt           *time.Time `json:"ends_at,omitempty"`
}

func NewConflictSummary(grant Grant) ConflictSummary {
	return ConflictSummary{ID: grant.ID, Kind: grant.Kind, PlanID: grant.PlanID, DurationDays: grant.DurationDays, Status: grant.Status, GrantedAt: grant.GrantedAt, ScheduledStartAt: grant.ScheduledStartAt, StartedAt: grant.StartedAt, EndsAt: grant.EndsAt}
}

type Grant struct {
	ID                      string     `json:"id"`
	WorkspaceID             string     `json:"workspace_id"`
	Kind                    Kind       `json:"kind"`
	PlanID                  string     `json:"plan_id"`
	DurationDays            int32      `json:"duration_days"`
	Status                  Status     `json:"status"`
	ActorUserID             string     `json:"granted_by_user_id"`
	StripeMode              string     `json:"stripe_mode,omitempty"`
	StripeCustomerID        string     `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID    string     `json:"stripe_subscription_id,omitempty"`
	StripeScheduleID        string     `json:"stripe_schedule_id,omitempty"`
	StripeCheckoutSessionID string     `json:"stripe_checkout_session_id,omitempty"`
	CheckoutAttempt         int32      `json:"-"`
	GrantedAt               time.Time  `json:"granted_at"`
	ScheduledStartAt        *time.Time `json:"scheduled_start_at,omitempty"`
	StartedAt               *time.Time `json:"started_at,omitempty"`
	EndsAt                  *time.Time `json:"ends_at,omitempty"`
	ActivatedAt             *time.Time `json:"activated_at,omitempty"`
	CanceledAt              *time.Time `json:"canceled_at,omitempty"`
	RevokedAt               *time.Time `json:"revoked_at,omitempty"`
	SupersededAt            *time.Time `json:"superseded_at,omitempty"`
	CompletedAt             *time.Time `json:"completed_at,omitempty"`
	SupersededByPlanID      string     `json:"superseded_by_plan_id,omitempty"`
	FailureCode             string     `json:"failure_code,omitempty"`
	FailureMessage          string     `json:"failure_message,omitempty"`
	PreviousStatus          Status     `json:"-"`
}

type SubscriptionRecord struct {
	PlanID               string
	Status               string
	StripeCustomerID     string
	StripeSubscriptionID string
	CancelAtPeriodEnd    bool
}

type BillingSnapshot struct {
	WorkspaceID  string
	OwnerUserID  string
	Subscription SubscriptionRecord
}

type BillingMode struct {
	Name    string
	PriceID string
}

type ModeResolver interface {
	Resolve(context.Context, string, string) (BillingMode, error)
}

type GrantStore interface {
	GetBilling(context.Context, string) (BillingSnapshot, error)
	GetOpenGrant(context.Context, string) (Grant, error)
	GetGrant(context.Context, string) (Grant, error)
	CreateGrant(context.Context, CreateGrantInput) (Grant, error)
	MarkScheduled(context.Context, ScheduledUpdate) (Grant, error)
	MarkFailed(context.Context, FailureUpdate) (Grant, error)
	RecordProvisioningSchedule(context.Context, ProvisioningScheduleUpdate) (Grant, error)
	MarkRevoked(context.Context, string, Status, time.Time) (Grant, error)
	ClaimCheckout(context.Context, string, string, string, string) (Grant, error)
	RecordCheckoutSession(context.Context, string, string, int32) (Grant, error)
	ReleaseCheckout(context.Context, string, string, time.Time) (Grant, error)
	ReopenUnrecordedCheckout(context.Context, string, int32) (Grant, error)
	GetGrantByCheckoutSession(context.Context, string) (Grant, error)
	GetGrantBySubscription(context.Context, string) (Grant, error)
	GetGrantBySchedule(context.Context, string) (Grant, error)
	MarkActive(context.Context, ActiveUpdate) (Grant, error)
	RecordRenewalCancellation(context.Context, string, time.Time) (Grant, error)
	MarkCanceled(context.Context, string, Status, time.Time) (Grant, error)
	MarkCompleted(context.Context, string, Status, time.Time) (Grant, error)
	MarkSuperseded(context.Context, string, Status, string, time.Time) (Grant, error)
}

type CreateGrantInput struct {
	WorkspaceID          string
	Kind                 Kind
	PlanID               string
	DurationDays         int32
	Status               Status
	ActorUserID          string
	StripeMode           string
	StripeCustomerID     string
	StripeSubscriptionID string
	GrantedAt            time.Time
}

type ScheduledUpdate struct {
	ID                   string
	ExpectedStatus       Status
	StripeCustomerID     string
	StripeSubscriptionID string
	StripeScheduleID     string
	ScheduledStartAt     time.Time
	EndsAt               time.Time
}

type FailureUpdate struct {
	ID             string
	ExpectedStatus Status
	Code           string
	Message        string
}

type ProvisioningScheduleUpdate struct {
	ID               string
	StripeScheduleID string
	FailureCode      string
	FailureMessage   string
}

type ActiveUpdate struct {
	ID                   string
	ExpectedStatus       Status
	StripeCustomerID     string
	StripeSubscriptionID string
	StartedAt            time.Time
	EndsAt               time.Time
	ActivatedAt          time.Time
}

type WebhookSubscriptionRequest struct {
	Snapshot   SubscriptionSnapshot
	PlanID     string
	OccurredAt time.Time
}

type WebhookCheckoutRequest struct {
	StripeMode string
	SessionID  string
	Metadata   map[string]string
	OccurredAt time.Time
}

type WebhookScheduleRequest struct {
	EventType  string
	Snapshot   ScheduleSnapshot
	OccurredAt time.Time
}

type WebhookReconcileResult struct {
	Managed         bool
	RenewalCanceled bool
	DoNotProject    bool
	Grant           Grant
}

type GrantRequest struct {
	WorkspaceID  string
	PlanID       string
	DurationDays int32
	ActorUserID  string
}

type RevokeRequest struct {
	WorkspaceID string
	GrantID     string
	ActorUserID string
}

type CheckoutRequest struct {
	WorkspaceID string
	PlanID      string
	StripeMode  string
	CustomerID  string
	PriceID     string
	SuccessURL  string
	CancelURL   string
}

type CheckoutResult struct {
	SessionID    string            `json:"checkout_session_id"`
	URL          string            `json:"checkout_url"`
	TrialGrantID string            `json:"trial_grant_id,omitempty"`
	TrialDays    int32             `json:"trial_days,omitempty"`
	Metadata     map[string]string `json:"-"`
	Resumed      bool              `json:"-"`
}

type Service struct {
	store       GrantStore
	stripe      StripeGateway
	modes       ModeResolver
	environment string
	now         func() time.Time
}

func NewService(store GrantStore, stripe StripeGateway, modes ModeResolver, environment string, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, stripe: stripe, modes: modes, environment: strings.TrimSpace(environment), now: now}
}

func (s *Service) PrepareCheckout(ctx context.Context, req CheckoutRequest) (CheckoutResult, error) {
	if err := validateCheckoutRequest(req); err != nil {
		return CheckoutResult{}, err
	}
	if s == nil || s.store == nil || s.stripe == nil || s.modes == nil || s.environment == "" {
		return CheckoutResult{}, ErrBillingModeUnavailable
	}
	grant, err := s.store.GetOpenGrant(ctx, req.WorkspaceID)
	if errors.Is(err, ErrGrantNotFound) {
		return CheckoutResult{}, ErrTrialCheckoutNotApplicable
	}
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("load trial grant for Checkout: %w", err)
	}
	if grant.Kind != KindFreeToPaid {
		return CheckoutResult{}, ErrTrialCheckoutNotApplicable
	}
	if grant.PlanID != req.PlanID {
		if grant.Status == StatusCheckoutPending {
			return CheckoutResult{}, s.abandonCheckoutForOrdinaryPlan(ctx, grant, req)
		}
		return CheckoutResult{}, ErrTrialCheckoutNotApplicable
	}
	if grant.Status == StatusScheduled || grant.Status == StatusActive {
		return CheckoutResult{}, ErrCheckoutCompletionPending
	}
	if grant.Status != StatusPendingActivation && grant.Status != StatusCheckoutPending {
		return CheckoutResult{}, ErrTrialCheckoutNotApplicable
	}
	if grant.StripeMode == "" || grant.StripeMode != req.StripeMode {
		return CheckoutResult{}, ErrBillingModeUnavailable
	}
	billingState, err := s.store.GetBilling(ctx, req.WorkspaceID)
	if err != nil || billingState.OwnerUserID == "" {
		return CheckoutResult{}, ErrWorkspaceNotFound
	}
	mode, err := s.modes.Resolve(ctx, billingState.OwnerUserID, req.PlanID)
	if err != nil || mode.Name != grant.StripeMode || mode.Name != req.StripeMode || mode.PriceID != req.PriceID {
		return CheckoutResult{}, ErrBillingModeUnavailable
	}
	return s.prepareCheckoutGrant(ctx, grant, req, true)
}

func (s *Service) abandonCheckoutForOrdinaryPlan(ctx context.Context, grant Grant, req CheckoutRequest) error {
	if grant.StripeMode == "" || grant.StripeMode != req.StripeMode || grant.StripeCheckoutSessionID == "" || grant.CheckoutAttempt < 1 {
		return ErrCheckoutStateConflict
	}
	checkout, err := s.stripe.RetrieveCheckout(ctx, grant.StripeMode, grant.StripeCheckoutSessionID)
	if err != nil {
		return fmt.Errorf("%w: retrieve trial Checkout before plan change: %v", ErrCheckoutExpiryUnconfirmed, err)
	}
	if err := validateCheckoutForAbandonment(checkout, grant, s.environment); err != nil {
		return err
	}
	switch checkout.Status {
	case "complete":
		return ErrCheckoutCompletionPending
	case "expired":
		return s.releaseCheckoutForOrdinaryPlan(ctx, grant, checkout.ID)
	case "open":
		expired, expireErr := s.stripe.ExpireCheckout(ctx, ExpireCheckoutRequest{StripeMode: grant.StripeMode, TrialGrantID: grant.ID, CheckoutSessionID: checkout.ID, CheckoutAttempt: grant.CheckoutAttempt})
		if expireErr == nil {
			if err := validateCheckoutForAbandonment(expired, grant, s.environment); err != nil || expired.Status != "expired" {
				if err != nil {
					return err
				}
				return ErrCheckoutExpiryUnconfirmed
			}
			return s.releaseCheckoutForOrdinaryPlan(ctx, grant, expired.ID)
		}
		confirmed, retrieveErr := s.stripe.RetrieveCheckout(ctx, grant.StripeMode, grant.StripeCheckoutSessionID)
		if retrieveErr != nil {
			return fmt.Errorf("%w: expire error: %v; retrieve error: %v", ErrCheckoutExpiryUnconfirmed, expireErr, retrieveErr)
		}
		if err := validateCheckoutForAbandonment(confirmed, grant, s.environment); err != nil {
			return err
		}
		switch confirmed.Status {
		case "expired":
			return s.releaseCheckoutForOrdinaryPlan(ctx, grant, confirmed.ID)
		case "complete":
			return ErrCheckoutCompletionPending
		default:
			return fmt.Errorf("%w: Checkout remains %s", ErrCheckoutExpiryUnconfirmed, confirmed.Status)
		}
	default:
		return ErrCheckoutStateConflict
	}
}

func validateCheckoutForAbandonment(checkout CheckoutSnapshot, grant Grant, environment string) error {
	if checkout.ID != grant.StripeCheckoutSessionID || checkout.StripeMode != grant.StripeMode || !checkoutCustomerMatches(checkout, grant) || !checkoutMetadataMatches(checkout.Metadata, grant, environment) {
		return ErrCheckoutStateConflict
	}
	return nil
}

func (s *Service) releaseCheckoutForOrdinaryPlan(ctx context.Context, grant Grant, sessionID string) error {
	_, err := s.store.ReleaseCheckout(ctx, grant.ID, sessionID, s.now().UTC().Add(time.Second))
	if err == nil {
		return ErrTrialCheckoutNotApplicable
	}
	if !errors.Is(err, ErrConcurrentTransition) {
		return fmt.Errorf("release expired trial Checkout before plan change: %w", err)
	}
	current, loadErr := s.store.GetGrant(ctx, grant.ID)
	if loadErr != nil {
		return ErrCheckoutStateConflict
	}
	if current.Status == StatusPendingActivation && current.StripeCheckoutSessionID == "" {
		return ErrTrialCheckoutNotApplicable
	}
	if current.Status == StatusActive || current.Status.IsTerminal() {
		return ErrCheckoutCompletionPending
	}
	return ErrCheckoutStateConflict
}

func validateCheckoutRequest(req CheckoutRequest) error {
	if strings.TrimSpace(req.WorkspaceID) == "" || strings.TrimSpace(req.PlanID) == "" {
		return fmt.Errorf("workspace and plan are required")
	}
	if strings.TrimSpace(req.StripeMode) == "" || strings.TrimSpace(req.PriceID) == "" {
		return fmt.Errorf("Stripe mode and price are required")
	}
	if err := ValidateGrantInput(req.PlanID, 1); err != nil {
		return err
	}
	for label, value := range map[string]string{"success": req.SuccessURL, "cancel": req.CancelURL} {
		if !strings.HasPrefix(strings.TrimSpace(value), "https://") {
			return fmt.Errorf("%s URL must use HTTPS", label)
		}
	}
	return nil
}

func (s *Service) prepareCheckoutGrant(ctx context.Context, grant Grant, req CheckoutRequest, allowExpiredReplacement bool) (CheckoutResult, error) {
	switch grant.Status {
	case StatusPendingActivation:
		customerID := strings.TrimSpace(req.CustomerID)
		if customerID == "" {
			return CheckoutResult{}, ErrCheckoutCustomerRequired
		}
		claimed, err := s.store.ClaimCheckout(ctx, grant.ID, req.WorkspaceID, req.PlanID, customerID)
		if err == nil {
			return s.createAndRecordTrialCheckout(ctx, claimed, req)
		}
		if !errors.Is(err, ErrConcurrentTransition) {
			return CheckoutResult{}, fmt.Errorf("claim trial grant for Checkout: %w", err)
		}
		current, loadErr := s.store.GetOpenGrant(ctx, req.WorkspaceID)
		if loadErr != nil || current.ID != grant.ID || current.PlanID != req.PlanID || current.Kind != KindFreeToPaid {
			return CheckoutResult{}, ErrTrialCheckoutNotApplicable
		}
		return s.prepareCheckoutGrant(ctx, current, req, allowExpiredReplacement)

	case StatusCheckoutPending:
		if grant.StripeCheckoutSessionID == "" {
			// A prior request may have reached Stripe without receiving the
			// response. Repeating the same grant-scoped idempotent operation is
			// the only safe way to recover its Session identity.
			return s.createAndRecordTrialCheckout(ctx, grant, req)
		}
		checkout, err := s.stripe.RetrieveCheckout(ctx, grant.StripeMode, grant.StripeCheckoutSessionID)
		if err != nil {
			return CheckoutResult{}, fmt.Errorf("retrieve trial Checkout Session: %w", err)
		}
		if checkout.ID != grant.StripeCheckoutSessionID || checkout.StripeMode != grant.StripeMode || !checkoutCustomerMatches(checkout, grant) {
			return CheckoutResult{}, ErrCheckoutStateConflict
		}
		if !checkoutMetadataMatches(checkout.Metadata, grant, s.environment) {
			return CheckoutResult{}, ErrCheckoutStateConflict
		}
		switch checkout.Status {
		case "open":
			if checkout.URL == "" {
				return CheckoutResult{}, ErrCheckoutStateConflict
			}
			return checkoutResult(grant, checkout, s.environment, true), nil
		case "complete":
			return CheckoutResult{}, ErrCheckoutCompletionPending
		case "expired":
			if !allowExpiredReplacement {
				return CheckoutResult{}, ErrCheckoutStateConflict
			}
			reopened, err := s.store.ReleaseCheckout(ctx, grant.ID, checkout.ID, s.now().UTC().Add(time.Second))
			if err != nil {
				return s.checkoutTransitionRace(ctx, grant, req, err)
			}
			return s.prepareCheckoutGrant(ctx, reopened, req, false)
		default:
			return CheckoutResult{}, ErrCheckoutStateConflict
		}
	default:
		return CheckoutResult{}, ErrTrialCheckoutNotApplicable
	}
}

func (s *Service) createAndRecordTrialCheckout(ctx context.Context, grant Grant, req CheckoutRequest) (CheckoutResult, error) {
	if strings.TrimSpace(grant.StripeCustomerID) == "" {
		return CheckoutResult{}, ErrCheckoutStateConflict
	}
	checkout, err := s.stripe.CreateTrialCheckout(ctx, CreateTrialCheckoutRequest{
		StripeMode: grant.StripeMode, WorkspaceID: grant.WorkspaceID, PlanID: grant.PlanID,
		TrialGrantID: grant.ID, TrialKind: grant.Kind, Environment: s.environment,
		CustomerID: grant.StripeCustomerID, PriceID: req.PriceID, DurationDays: grant.DurationDays,
		CheckoutAttempt: grant.CheckoutAttempt,
		SuccessURL:      req.SuccessURL, CancelURL: req.CancelURL,
	})
	if err != nil {
		var mutationErr *CheckoutMutationError
		if errors.As(err, &mutationErr) && mutationErr.Outcome == MutationRejected {
			if _, reopenErr := s.store.ReopenUnrecordedCheckout(ctx, grant.ID, grant.CheckoutAttempt); reopenErr != nil && !errors.Is(reopenErr, ErrConcurrentTransition) {
				return CheckoutResult{}, fmt.Errorf("create trial Checkout: %v; reopen claim: %w", err, reopenErr)
			}
		}
		return CheckoutResult{}, fmt.Errorf("create trial Checkout: %w", err)
	}
	if checkout.ID == "" || checkout.Status != "open" || checkout.URL == "" || checkout.StripeMode != grant.StripeMode || !checkoutCustomerMatches(checkout, grant) || !checkoutMetadataMatches(checkout.Metadata, grant, s.environment) {
		return CheckoutResult{}, ErrCheckoutStateConflict
	}
	_, err = s.store.RecordCheckoutSession(ctx, grant.ID, checkout.ID, grant.CheckoutAttempt)
	if err == nil {
		return checkoutResult(grant, checkout, s.environment, false), nil
	}
	if !errors.Is(err, ErrConcurrentTransition) {
		return CheckoutResult{}, fmt.Errorf("record trial Checkout Session: %w", err)
	}
	current, loadErr := s.store.GetGrant(ctx, grant.ID)
	if loadErr == nil && current.Status == StatusCheckoutPending && current.CheckoutAttempt == grant.CheckoutAttempt && current.StripeCheckoutSessionID == checkout.ID {
		return checkoutResult(current, checkout, s.environment, true), nil
	}
	if loadErr == nil && current.CheckoutAttempt == grant.CheckoutAttempt && current.StripeCheckoutSessionID == checkout.ID && current.Status != StatusCheckoutPending {
		return CheckoutResult{}, ErrCheckoutCompletionPending
	}
	return CheckoutResult{}, ErrCheckoutStateConflict
}

func (s *Service) checkoutTransitionRace(ctx context.Context, grant Grant, req CheckoutRequest, transitionErr error) (CheckoutResult, error) {
	if !errors.Is(transitionErr, ErrConcurrentTransition) {
		return CheckoutResult{}, fmt.Errorf("release expired trial Checkout: %w", transitionErr)
	}
	current, err := s.store.GetGrant(ctx, grant.ID)
	if err == nil && current.Status == StatusPendingActivation && current.StripeCheckoutSessionID == "" && current.PlanID == req.PlanID {
		return s.prepareCheckoutGrant(ctx, current, req, false)
	}
	if err == nil && current.Status == StatusCheckoutPending && current.PlanID == req.PlanID {
		return CheckoutResult{}, ErrCheckoutStateConflict
	}
	if err == nil && current.Status != StatusPendingActivation {
		return CheckoutResult{}, ErrCheckoutCompletionPending
	}
	return CheckoutResult{}, ErrCheckoutStateConflict
}

func checkoutMetadataMatches(metadata map[string]string, grant Grant, environment string) bool {
	want := trialMetadata(grant.WorkspaceID, grant.PlanID, grant.ID, grant.Kind, environment)
	for key, value := range want {
		if metadata[key] != value {
			return false
		}
	}
	return true
}

func checkoutCustomerMatches(checkout CheckoutSnapshot, grant Grant) bool {
	return strings.TrimSpace(checkout.CustomerID) != "" && checkout.CustomerID == grant.StripeCustomerID
}

func checkoutResult(grant Grant, checkout CheckoutSnapshot, environment string, resumed bool) CheckoutResult {
	return CheckoutResult{
		SessionID: checkout.ID, URL: checkout.URL, TrialGrantID: grant.ID,
		TrialDays: grant.DurationDays, Metadata: trialMetadata(grant.WorkspaceID, grant.PlanID, grant.ID, grant.Kind, environment),
		Resumed: resumed,
	}
}

func (s *Service) Grant(ctx context.Context, req GrantRequest) (Grant, error) {
	if err := ValidateGrantInput(req.PlanID, req.DurationDays); err != nil {
		return Grant{}, err
	}
	if strings.TrimSpace(req.WorkspaceID) == "" || strings.TrimSpace(req.ActorUserID) == "" {
		return Grant{}, ErrWorkspaceNotFound
	}
	if s == nil || s.store == nil || s.stripe == nil || s.modes == nil || s.environment == "" {
		return Grant{}, ErrBillingModeUnavailable
	}

	var retry *Grant
	if current, err := s.store.GetOpenGrant(ctx, req.WorkspaceID); err == nil {
		if current.Status != StatusProvisioning || current.Kind != KindPaidSamePlan || current.WorkspaceID != req.WorkspaceID || current.PlanID != req.PlanID || current.DurationDays != req.DurationDays {
			return Grant{}, &OpenGrantConflictError{Current: current}
		}
		retry = &current
	} else if !errors.Is(err, ErrGrantNotFound) {
		return Grant{}, fmt.Errorf("load open trial grant: %w", err)
	}

	billingState, err := s.store.GetBilling(ctx, req.WorkspaceID)
	if err != nil {
		return Grant{}, err
	}
	if billingState.WorkspaceID == "" || billingState.OwnerUserID == "" {
		return Grant{}, ErrWorkspaceNotFound
	}

	now := s.now().UTC()
	if billingState.Subscription.PlanID == "" || billingState.Subscription.PlanID == "free" {
		if retry != nil {
			return Grant{}, &OpenGrantConflictError{Current: *retry}
		}
		mode, err := s.modes.Resolve(ctx, billingState.OwnerUserID, req.PlanID)
		if err != nil || mode.Name == "" || mode.PriceID == "" {
			return Grant{}, fmt.Errorf("%w: %v", ErrBillingModeUnavailable, err)
		}
		grant, err := s.store.CreateGrant(ctx, CreateGrantInput{
			WorkspaceID: req.WorkspaceID, Kind: KindFreeToPaid, PlanID: req.PlanID,
			DurationDays: req.DurationDays, Status: StatusPendingActivation,
			ActorUserID: req.ActorUserID, StripeMode: mode.Name, GrantedAt: now,
		})
		return s.withCreateConflict(ctx, req.WorkspaceID, grant, err)
	}

	if req.PlanID != billingState.Subscription.PlanID {
		return Grant{}, ErrPaidPlanMismatch
	}
	if billingState.Subscription.Status != "active" || billingState.Subscription.StripeSubscriptionID == "" || billingState.Subscription.CancelAtPeriodEnd {
		return Grant{}, ErrIneligibleSubscription
	}

	mode, err := s.modes.Resolve(ctx, billingState.OwnerUserID, req.PlanID)
	if err != nil || mode.Name == "" || mode.PriceID == "" {
		return Grant{}, fmt.Errorf("%w: %v", ErrBillingModeUnavailable, err)
	}
	if retry != nil && (retry.StripeMode != mode.Name || retry.StripeSubscriptionID != billingState.Subscription.StripeSubscriptionID) {
		return Grant{}, &OpenGrantConflictError{Current: *retry}
	}
	remote, err := s.stripe.RetrieveSubscription(ctx, mode.Name, billingState.Subscription.StripeSubscriptionID)
	if err != nil {
		return Grant{}, fmt.Errorf("retrieve Stripe subscription: %w", err)
	}
	if !eligiblePaidSubscription(remote, billingState.Subscription, mode.PriceID) {
		return Grant{}, ErrIneligibleSubscription
	}
	expectedScheduleID := ""
	if retry != nil {
		expectedScheduleID = retry.StripeScheduleID
	}
	if remote.ScheduleID != expectedScheduleID && (retry == nil || expectedScheduleID != "") {
		return Grant{}, ErrUnrelatedSchedule
	}

	var grant Grant
	if retry != nil {
		grant = *retry
	} else {
		grant, err = s.store.CreateGrant(ctx, CreateGrantInput{
			WorkspaceID: req.WorkspaceID, Kind: KindPaidSamePlan, PlanID: req.PlanID,
			DurationDays: req.DurationDays, Status: StatusProvisioning, ActorUserID: req.ActorUserID,
			StripeMode: mode.Name, StripeCustomerID: remote.CustomerID,
			StripeSubscriptionID: remote.ID, GrantedAt: now,
		})
		if err != nil {
			if errors.Is(err, ErrOpenGrantExists) {
				if current, loadErr := s.store.GetOpenGrant(ctx, req.WorkspaceID); loadErr == nil {
					return Grant{}, &OpenGrantConflictError{Current: current}
				}
			}
			return Grant{}, fmt.Errorf("create provisioning trial grant: %w", err)
		}
	}

	periodStart := remote.CurrentPeriodStartAt.UTC()
	periodEnd := remote.CurrentPeriodEndAt.UTC()
	trialEnd := periodEnd.AddDate(0, 0, int(req.DurationDays))
	scheduleReq := CreatePaidTrialScheduleRequest{
		StripeMode: mode.Name, WorkspaceID: req.WorkspaceID, PlanID: req.PlanID,
		TrialGrantID: grant.ID, TrialKind: KindPaidSamePlan, Environment: s.environment,
		SubscriptionID: remote.ID, PriceID: mode.PriceID, DurationDays: req.DurationDays,
		CurrentPhase: SchedulePhase{PriceID: mode.PriceID, StartAt: periodStart, EndAt: periodEnd},
		TrialStartAt: periodEnd, TrialEndAt: trialEnd,
	}
	var schedule ScheduleSnapshot
	var stripeErr error
	if grant.StripeScheduleID != "" {
		schedule, stripeErr = s.stripe.ConfigurePaidTrialSchedule(ctx, grant.StripeScheduleID, scheduleReq)
	} else {
		schedule, stripeErr = s.stripe.CreatePaidTrialSchedule(ctx, scheduleReq)
	}
	if stripeErr != nil {
		var mutationErr *ScheduleMutationError
		confirmedRejected := errors.As(stripeErr, &mutationErr) && mutationErr.Outcome == MutationRejected
		// Any returned ID or outcome-unknown error may have created/changed
		// remote state. Keep the grant open and persist only safe diagnostics.
		if schedule.ID != "" || !confirmedRejected {
			if grant.StripeScheduleID != "" && schedule.ID != "" && grant.StripeScheduleID != schedule.ID {
				return Grant{}, ErrUnrelatedSchedule
			}
			_, recordErr := s.store.RecordProvisioningSchedule(ctx, ProvisioningScheduleUpdate{
				ID: grant.ID, StripeScheduleID: schedule.ID,
				FailureCode:    "stripe_schedule_reconciliation_required",
				FailureMessage: "Stripe schedule exists but trial configuration requires reconciliation",
			})
			if recordErr != nil {
				return Grant{}, fmt.Errorf("schedule Stripe trial requires reconciliation for %s: %v; persist schedule: %w", schedule.ID, stripeErr, recordErr)
			}
			return Grant{}, fmt.Errorf("schedule Stripe trial requires reconciliation for %s: %w", schedule.ID, stripeErr)
		}
		_, markErr := s.store.MarkFailed(ctx, FailureUpdate{
			ID: grant.ID, ExpectedStatus: StatusProvisioning,
			Code: "stripe_schedule_failed", Message: "Stripe could not schedule the trial",
		})
		if markErr != nil {
			return Grant{}, fmt.Errorf("schedule Stripe trial: %v; mark grant failed: %w", stripeErr, markErr)
		}
		return Grant{}, fmt.Errorf("schedule Stripe trial: %w", stripeErr)
	}

	scheduled, err := s.store.MarkScheduled(ctx, ScheduledUpdate{
		ID: grant.ID, ExpectedStatus: StatusProvisioning,
		StripeCustomerID: remote.CustomerID, StripeSubscriptionID: remote.ID,
		StripeScheduleID: schedule.ID, ScheduledStartAt: periodEnd, EndsAt: trialEnd,
	})
	if err != nil {
		if errors.Is(err, ErrConcurrentTransition) {
			if current, loadErr := s.store.GetGrant(ctx, grant.ID); loadErr == nil && current.StripeScheduleID == schedule.ID && (current.Status == StatusScheduled || current.Status == StatusActive) {
				return current, nil
			}
		}
		_, recordErr := s.store.RecordProvisioningSchedule(ctx, ProvisioningScheduleUpdate{
			ID: grant.ID, StripeScheduleID: schedule.ID,
			FailureCode:    "stripe_schedule_projection_required",
			FailureMessage: "Stripe trial schedule succeeded but local projection requires reconciliation",
		})
		if recordErr != nil && !errors.Is(recordErr, ErrConcurrentTransition) {
			return Grant{}, fmt.Errorf("record scheduled trial: %v; preserve schedule identity: %w", err, recordErr)
		}
		return Grant{}, fmt.Errorf("record scheduled trial: %w", err)
	}
	return scheduled, nil
}

func (s *Service) withCreateConflict(ctx context.Context, workspaceID string, grant Grant, err error) (Grant, error) {
	if err == nil {
		return grant, nil
	}
	if errors.Is(err, ErrOpenGrantExists) {
		if current, loadErr := s.store.GetOpenGrant(ctx, workspaceID); loadErr == nil {
			return Grant{}, &OpenGrantConflictError{Current: current}
		}
	}
	return Grant{}, err
}

func eligiblePaidSubscription(remote SubscriptionSnapshot, local SubscriptionRecord, priceID string) bool {
	return remote.ID == local.StripeSubscriptionID &&
		remote.Status == "active" &&
		remote.PriceID == priceID &&
		!remote.CancelAtPeriodEnd && remote.CancelAt == nil &&
		remote.CurrentPeriodStartAt != nil && remote.CurrentPeriodEndAt != nil &&
		remote.CurrentPeriodStartAt.Before(*remote.CurrentPeriodEndAt) &&
		(local.StripeCustomerID == "" || remote.CustomerID == local.StripeCustomerID)
}

func (s *Service) Revoke(ctx context.Context, req RevokeRequest) (Grant, error) {
	if s == nil || s.store == nil || s.stripe == nil || strings.TrimSpace(req.WorkspaceID) == "" || strings.TrimSpace(req.GrantID) == "" {
		return Grant{}, ErrGrantNotFound
	}
	grant, err := s.store.GetGrant(ctx, req.GrantID)
	if err != nil {
		return Grant{}, err
	}
	if grant.WorkspaceID != req.WorkspaceID {
		return Grant{}, ErrGrantNotFound
	}
	now := s.now().UTC()

	switch grant.Status {
	case StatusPendingActivation:
		return s.revokeCAS(ctx, grant, now)
	case StatusCheckoutPending:
		if grant.StripeMode == "" || grant.StripeCheckoutSessionID == "" {
			return Grant{}, ErrRevokeConflict
		}
		expired, expireErr := s.stripe.ExpireCheckout(ctx, ExpireCheckoutRequest{
			StripeMode: grant.StripeMode, TrialGrantID: grant.ID,
			CheckoutSessionID: grant.StripeCheckoutSessionID, CheckoutAttempt: grant.CheckoutAttempt,
		})
		if expireErr != nil || expired.ID != grant.StripeCheckoutSessionID || expired.Status != "expired" {
			confirmed, retrieveErr := s.stripe.RetrieveCheckout(ctx, grant.StripeMode, grant.StripeCheckoutSessionID)
			if retrieveErr != nil {
				if expireErr != nil {
					return Grant{}, fmt.Errorf("%w: expire error: %w; retrieve error: %v", ErrCheckoutExpiryUnconfirmed, expireErr, retrieveErr)
				}
				return Grant{}, fmt.Errorf("%w: unexpected expire response; retrieve error: %v", ErrCheckoutExpiryUnconfirmed, retrieveErr)
			}
			if confirmed.ID != grant.StripeCheckoutSessionID {
				return Grant{}, fmt.Errorf("%w: retrieved a different Checkout Session", ErrCheckoutExpiryUnconfirmed)
			}
			switch confirmed.Status {
			case "expired":
				expired = confirmed
			case "complete":
				return Grant{}, ErrRevokeConflict
			default:
				if expireErr != nil {
					return Grant{}, fmt.Errorf("%w: status %q after expire error: %w", ErrCheckoutExpiryUnconfirmed, confirmed.Status, expireErr)
				}
				return Grant{}, fmt.Errorf("%w: status %q", ErrCheckoutExpiryUnconfirmed, confirmed.Status)
			}
		}
		revoked, err := s.store.MarkRevoked(ctx, grant.ID, StatusCheckoutPending, now)
		if err == nil {
			revoked.PreviousStatus = StatusCheckoutPending
			return revoked, nil
		}
		if !errors.Is(err, ErrConcurrentTransition) {
			return Grant{}, fmt.Errorf("revoke trial grant: %w", err)
		}
		current, loadErr := s.store.GetGrant(ctx, grant.ID)
		if loadErr != nil {
			return Grant{}, fmt.Errorf("reload trial grant after Checkout expiry: %w", loadErr)
		}
		if current.WorkspaceID != req.WorkspaceID || current.Status != StatusPendingActivation || current.StripeCheckoutSessionID != "" {
			return Grant{}, ErrRevokeConflict
		}
		return s.revokeCAS(ctx, current, now)
	default:
		return Grant{}, ErrRevokeConflict
	}
}

// RetrieveSubscription returns Stripe's authoritative Subscription projection
// in the mode whose webhook signature was verified by the caller.
func (s *Service) RetrieveSubscription(ctx context.Context, stripeMode, subscriptionID string) (SubscriptionSnapshot, error) {
	if s == nil || s.stripe == nil || strings.TrimSpace(stripeMode) == "" || strings.TrimSpace(subscriptionID) == "" {
		return SubscriptionSnapshot{}, ErrWebhookStateConflict
	}
	return s.stripe.RetrieveSubscription(ctx, stripeMode, subscriptionID)
}

// ReconcileSubscription applies only monotonic grant transitions. Subscription
// persistence remains the webhook handler's responsibility so legacy billing
// and quota reconciliation continue to share one projector.
func (s *Service) ReconcileSubscription(ctx context.Context, req WebhookSubscriptionRequest) (WebhookReconcileResult, error) {
	if s == nil || s.store == nil {
		return WebhookReconcileResult{}, ErrWebhookStateConflict
	}
	snapshot := req.Snapshot
	if err := s.validateWebhookEnvironment(snapshot.Metadata); err != nil {
		return WebhookReconcileResult{}, err
	}
	grant, managed, err := s.lookupWebhookGrant(ctx, snapshot.Metadata, snapshot.ID, "")
	if err != nil || !managed {
		return WebhookReconcileResult{Managed: managed, Grant: grant}, err
	}
	if err := validateSubscriptionGrantMatch(grant, snapshot, req.PlanID, s.environment); err != nil {
		return WebhookReconcileResult{}, err
	}
	result := WebhookReconcileResult{Managed: true, Grant: grant}
	if grant.Status.IsTerminal() {
		result.DoNotProject = true
		return result, nil
	}
	at := webhookTime(req.OccurredAt, s.now)

	if req.PlanID != grant.PlanID {
		if grant.Status == StatusScheduled || grant.Status == StatusActive {
			updated, updateErr := s.store.MarkSuperseded(ctx, grant.ID, grant.Status, req.PlanID, at)
			if errors.Is(updateErr, ErrConcurrentTransition) {
				return s.reloadWebhookGrant(ctx, grant.ID)
			}
			return WebhookReconcileResult{Managed: true, Grant: updated}, updateErr
		}
		return WebhookReconcileResult{}, ErrWebhookStateConflict
	}

	switch snapshot.Status {
	case "trialing":
		if snapshot.TrialStartAt == nil || snapshot.TrialEndAt == nil || !snapshot.TrialStartAt.Before(*snapshot.TrialEndAt) {
			return WebhookReconcileResult{}, ErrWebhookStateConflict
		}
		if grant.Status == StatusCheckoutPending || grant.Status == StatusScheduled {
			updated, updateErr := s.store.MarkActive(ctx, ActiveUpdate{
				ID: grant.ID, ExpectedStatus: grant.Status,
				StripeCustomerID: snapshot.CustomerID, StripeSubscriptionID: snapshot.ID,
				StartedAt: snapshot.TrialStartAt.UTC(), EndsAt: snapshot.TrialEndAt.UTC(), ActivatedAt: at,
			})
			if errors.Is(updateErr, ErrConcurrentTransition) {
				return s.reloadWebhookGrant(ctx, grant.ID)
			}
			if updateErr != nil {
				return WebhookReconcileResult{}, updateErr
			}
			grant = updated
			result.Grant = updated
		}
		if grant.Status != StatusActive {
			return result, nil
		}
		if grant.StartedAt == nil || grant.EndsAt == nil || !grant.StartedAt.Equal(snapshot.TrialStartAt.UTC()) || !grant.EndsAt.Equal(snapshot.TrialEndAt.UTC()) {
			return WebhookReconcileResult{}, ErrWebhookStateConflict
		}
		if snapshot.CancelAtPeriodEnd {
			result.RenewalCanceled = true
			if grant.CanceledAt == nil {
				updated, updateErr := s.store.RecordRenewalCancellation(ctx, grant.ID, at)
				if errors.Is(updateErr, ErrConcurrentTransition) {
					return s.reloadWebhookGrant(ctx, grant.ID)
				}
				if updateErr != nil {
					return WebhookReconcileResult{}, updateErr
				}
				result.Grant = updated
			}
		}
		return result, nil

	case "active":
		if grant.Status == StatusActive && grant.EndsAt != nil && !at.Before(*grant.EndsAt) {
			updated, updateErr := s.store.MarkCompleted(ctx, grant.ID, StatusActive, at)
			if errors.Is(updateErr, ErrConcurrentTransition) {
				return s.reloadWebhookGrant(ctx, grant.ID)
			}
			return WebhookReconcileResult{Managed: true, Grant: updated}, updateErr
		}
	case "canceled", "unpaid", "incomplete_expired":
		if grant.Status == StatusScheduled || grant.Status == StatusActive {
			updated, updateErr := s.store.MarkCanceled(ctx, grant.ID, grant.Status, at)
			if errors.Is(updateErr, ErrConcurrentTransition) {
				return s.reloadWebhookGrant(ctx, grant.ID)
			}
			return WebhookReconcileResult{Managed: true, Grant: updated}, updateErr
		}
	}
	return result, nil
}

func (s *Service) ReconcileCheckoutExpired(ctx context.Context, req WebhookCheckoutRequest) (WebhookReconcileResult, error) {
	if s == nil || s.store == nil || strings.TrimSpace(req.SessionID) == "" {
		return WebhookReconcileResult{}, ErrWebhookStateConflict
	}
	if err := s.validateWebhookEnvironment(req.Metadata); err != nil {
		return WebhookReconcileResult{}, err
	}
	grant, managed, err := s.lookupWebhookGrant(ctx, req.Metadata, "", req.SessionID)
	if err != nil || !managed {
		return WebhookReconcileResult{Managed: managed, Grant: grant}, err
	}
	if req.StripeMode != grant.StripeMode || req.SessionID != grant.StripeCheckoutSessionID || !webhookMetadataMatches(req.Metadata, grant, s.environment) {
		return WebhookReconcileResult{}, ErrWebhookStateConflict
	}
	if grant.Status != StatusCheckoutPending {
		return WebhookReconcileResult{Managed: true, Grant: grant}, nil
	}
	released, err := s.store.ReleaseCheckout(ctx, grant.ID, req.SessionID, webhookTime(req.OccurredAt, s.now).Add(time.Second))
	if errors.Is(err, ErrConcurrentTransition) {
		return s.reloadWebhookGrant(ctx, grant.ID)
	}
	return WebhookReconcileResult{Managed: true, Grant: released}, err
}

// ReconcileOrdinaryCheckout consumes a pending offer only after the ordinary
// paid Subscription was authoritatively projected. It never closes an active
// managed Checkout attempt; that must be expired and released first.
func (s *Service) ReconcileOrdinaryCheckout(ctx context.Context, workspaceID, paidPlanID string, occurredAt time.Time) (WebhookReconcileResult, error) {
	if s == nil || s.store == nil || strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(paidPlanID) == "" {
		return WebhookReconcileResult{}, ErrGrantNotFound
	}
	grant, err := s.store.GetOpenGrant(ctx, workspaceID)
	if errors.Is(err, ErrGrantNotFound) {
		return WebhookReconcileResult{}, ErrGrantNotFound
	}
	if err != nil {
		return WebhookReconcileResult{}, err
	}
	if grant.Kind != KindFreeToPaid || grant.PlanID == paidPlanID {
		return WebhookReconcileResult{Managed: true, Grant: grant}, nil
	}
	if grant.Status == StatusCheckoutPending {
		return WebhookReconcileResult{}, ErrCheckoutCompletionPending
	}
	if grant.Status != StatusPendingActivation {
		return WebhookReconcileResult{Managed: true, Grant: grant}, nil
	}
	updated, err := s.store.MarkSuperseded(ctx, grant.ID, StatusPendingActivation, paidPlanID, webhookTime(occurredAt, s.now))
	if errors.Is(err, ErrConcurrentTransition) {
		return s.reloadWebhookGrant(ctx, grant.ID)
	}
	return WebhookReconcileResult{Managed: true, Grant: updated}, err
}

func (s *Service) ReconcileSchedule(ctx context.Context, req WebhookScheduleRequest) (WebhookReconcileResult, error) {
	if s == nil || s.store == nil || strings.TrimSpace(req.Snapshot.ID) == "" {
		return WebhookReconcileResult{}, ErrWebhookStateConflict
	}
	if err := s.validateWebhookEnvironment(req.Snapshot.Metadata); err != nil {
		return WebhookReconcileResult{}, err
	}
	grant, managed, err := s.lookupWebhookScheduleGrant(ctx, req.Snapshot)
	if err != nil || !managed {
		return WebhookReconcileResult{Managed: managed, Grant: grant}, err
	}
	if err := validateScheduleGrantMatch(grant, req.Snapshot, s.environment, req.EventType); err != nil {
		return WebhookReconcileResult{}, err
	}
	if grant.Status.IsTerminal() {
		return WebhookReconcileResult{Managed: true, Grant: grant}, nil
	}
	at := webhookTime(req.OccurredAt, s.now)
	phase, hasPhase := managedTrialPhase(req.Snapshot, grant.ID)

	switch req.EventType {
	case "subscription_schedule.created", "subscription_schedule.updated":
		if grant.Status == StatusProvisioning {
			if !hasPhase || !phase.StartAt.Before(phase.EndAt) || !phase.TrialEndAt.Equal(phase.EndAt) {
				return WebhookReconcileResult{}, ErrWebhookStateConflict
			}
			updated, updateErr := s.store.MarkScheduled(ctx, ScheduledUpdate{ID: grant.ID, ExpectedStatus: StatusProvisioning, StripeCustomerID: req.Snapshot.CustomerID, StripeSubscriptionID: req.Snapshot.SubscriptionID, StripeScheduleID: req.Snapshot.ID, ScheduledStartAt: phase.StartAt.UTC(), EndsAt: phase.EndAt.UTC()})
			if errors.Is(updateErr, ErrConcurrentTransition) {
				return s.reloadWebhookGrant(ctx, grant.ID)
			}
			return WebhookReconcileResult{Managed: true, Grant: updated}, updateErr
		}
	case "subscription_schedule.completed":
		return s.completeScheduleGrant(ctx, grant, req.Snapshot, phase, hasPhase, at)
	case "subscription_schedule.canceled", "subscription_schedule.aborted":
		if grant.Status == StatusScheduled || grant.Status == StatusActive {
			updated, updateErr := s.store.MarkCanceled(ctx, grant.ID, grant.Status, at)
			if errors.Is(updateErr, ErrConcurrentTransition) {
				return s.reloadWebhookGrant(ctx, grant.ID)
			}
			return WebhookReconcileResult{Managed: true, Grant: updated}, updateErr
		}
	case "subscription_schedule.released":
		if grant.Status == StatusScheduled || grant.Status == StatusActive {
			if req.Snapshot.EndBehavior == "release" && grant.EndsAt != nil && !at.Before(*grant.EndsAt) {
				return s.completeScheduleGrant(ctx, grant, req.Snapshot, phase, hasPhase, at)
			}
			nextPlan := strings.TrimSpace(req.Snapshot.Metadata[metadataPlanID])
			updated, updateErr := s.store.MarkSuperseded(ctx, grant.ID, grant.Status, nextPlan, at)
			if errors.Is(updateErr, ErrConcurrentTransition) {
				return s.reloadWebhookGrant(ctx, grant.ID)
			}
			return WebhookReconcileResult{Managed: true, Grant: updated}, updateErr
		}
	}
	return WebhookReconcileResult{Managed: true, Grant: grant}, nil
}

func (s *Service) completeScheduleGrant(ctx context.Context, grant Grant, snapshot ScheduleSnapshot, phase SchedulePhase, hasPhase bool, at time.Time) (WebhookReconcileResult, error) {
	if grant.Status == StatusScheduled {
		start, end := time.Time{}, time.Time{}
		if hasPhase {
			start, end = phase.StartAt.UTC(), phase.EndAt.UTC()
		} else if grant.ScheduledStartAt != nil && grant.EndsAt != nil {
			start, end = grant.ScheduledStartAt.UTC(), grant.EndsAt.UTC()
		}
		if start.IsZero() || !start.Before(end) {
			return WebhookReconcileResult{}, ErrWebhookStateConflict
		}
		subscriptionID := snapshot.SubscriptionID
		if subscriptionID == "" {
			subscriptionID = snapshot.ReleasedSubscriptionID
		}
		if subscriptionID == "" {
			subscriptionID = grant.StripeSubscriptionID
		}
		activated, updateErr := s.store.MarkActive(ctx, ActiveUpdate{ID: grant.ID, ExpectedStatus: StatusScheduled, StripeCustomerID: snapshot.CustomerID, StripeSubscriptionID: subscriptionID, StartedAt: start, EndsAt: end, ActivatedAt: start})
		if updateErr != nil && !errors.Is(updateErr, ErrConcurrentTransition) {
			return WebhookReconcileResult{}, updateErr
		}
		if updateErr == nil {
			grant = activated
		} else if current, loadErr := s.store.GetGrant(ctx, grant.ID); loadErr == nil {
			grant = current
		}
	}
	if grant.Status == StatusActive {
		updated, updateErr := s.store.MarkCompleted(ctx, grant.ID, StatusActive, at)
		if errors.Is(updateErr, ErrConcurrentTransition) {
			return s.reloadWebhookGrant(ctx, grant.ID)
		}
		return WebhookReconcileResult{Managed: true, Grant: updated}, updateErr
	}
	return WebhookReconcileResult{Managed: true, Grant: grant, DoNotProject: grant.Status.IsTerminal()}, nil
}

func (s *Service) validateWebhookEnvironment(metadata map[string]string) error {
	eventEnvironment := strings.TrimSpace(metadata[metadataEnvironment])
	if eventEnvironment != "" && eventEnvironment != s.environment {
		return ErrWebhookNotApplicable
	}
	return nil
}

func (s *Service) lookupWebhookGrant(ctx context.Context, metadata map[string]string, subscriptionID, checkoutSessionID string) (Grant, bool, error) {
	if id := strings.TrimSpace(metadata[metadataTrialGrant]); id != "" {
		grant, err := s.store.GetGrant(ctx, id)
		if err != nil {
			return Grant{}, true, err
		}
		return grant, true, nil
	}
	var grant Grant
	var err error
	if subscriptionID != "" {
		grant, err = s.store.GetGrantBySubscription(ctx, subscriptionID)
	} else if checkoutSessionID != "" {
		grant, err = s.store.GetGrantByCheckoutSession(ctx, checkoutSessionID)
	} else {
		return Grant{}, false, nil
	}
	if errors.Is(err, ErrGrantNotFound) {
		return Grant{}, false, nil
	}
	return grant, err == nil, err
}

func (s *Service) lookupWebhookScheduleGrant(ctx context.Context, snapshot ScheduleSnapshot) (Grant, bool, error) {
	if id := strings.TrimSpace(snapshot.Metadata[metadataTrialGrant]); id != "" {
		grant, err := s.store.GetGrant(ctx, id)
		return grant, true, err
	}
	grant, err := s.store.GetGrantBySchedule(ctx, snapshot.ID)
	if errors.Is(err, ErrGrantNotFound) {
		return Grant{}, false, nil
	}
	return grant, err == nil, err
}

func (s *Service) reloadWebhookGrant(ctx context.Context, id string) (WebhookReconcileResult, error) {
	grant, err := s.store.GetGrant(ctx, id)
	return WebhookReconcileResult{Managed: err == nil, Grant: grant}, err
}

func validateSubscriptionGrantMatch(grant Grant, snapshot SubscriptionSnapshot, planID, environment string) error {
	if snapshot.StripeMode == "" || snapshot.StripeMode != grant.StripeMode || snapshot.ID == "" || snapshot.CustomerID == "" || planID == "" {
		return ErrWebhookStateConflict
	}
	if grant.StripeCustomerID != "" && snapshot.CustomerID != grant.StripeCustomerID {
		return ErrWebhookStateConflict
	}
	if grant.StripeSubscriptionID != "" && snapshot.ID != grant.StripeSubscriptionID {
		return ErrWebhookStateConflict
	}
	if len(snapshot.Metadata) > 0 && !webhookMetadataMatches(snapshot.Metadata, grant, environment) {
		return ErrWebhookStateConflict
	}
	return nil
}

func validateScheduleGrantMatch(grant Grant, snapshot ScheduleSnapshot, environment, eventType string) error {
	if snapshot.StripeMode == "" || snapshot.StripeMode != grant.StripeMode || snapshot.ID == "" || grant.Kind != KindPaidSamePlan {
		return ErrWebhookStateConflict
	}
	if grant.StripeScheduleID != "" && snapshot.ID != grant.StripeScheduleID {
		return ErrWebhookStateConflict
	}
	if grant.StripeCustomerID != "" && snapshot.CustomerID != grant.StripeCustomerID {
		return ErrWebhookStateConflict
	}
	if grant.StripeSubscriptionID != "" && snapshot.SubscriptionID != "" && snapshot.SubscriptionID != grant.StripeSubscriptionID {
		return ErrWebhookStateConflict
	}
	if len(snapshot.Metadata) > 0 {
		if snapshot.Metadata[metadataWorkspaceID] != grant.WorkspaceID || snapshot.Metadata[metadataTrialGrant] != grant.ID || snapshot.Metadata[metadataTrialKind] != string(grant.Kind) || snapshot.Metadata[metadataEnvironment] != environment {
			return ErrWebhookStateConflict
		}
		if eventType != "subscription_schedule.released" && snapshot.Metadata[metadataPlanID] != grant.PlanID {
			return ErrWebhookStateConflict
		}
	}
	return nil
}

func webhookMetadataMatches(metadata map[string]string, grant Grant, environment string) bool {
	return metadata[metadataWorkspaceID] == grant.WorkspaceID &&
		metadata[metadataPlanID] == grant.PlanID &&
		metadata[metadataTrialGrant] == grant.ID &&
		metadata[metadataTrialKind] == string(grant.Kind) &&
		metadata[metadataEnvironment] == environment
}

func managedTrialPhase(snapshot ScheduleSnapshot, grantID string) (SchedulePhase, bool) {
	for _, phase := range snapshot.Phases {
		if phase.Metadata[metadataTrialGrant] == grantID {
			return phase, true
		}
	}
	return SchedulePhase{}, false
}

func webhookTime(at time.Time, now func() time.Time) time.Time {
	if !at.IsZero() {
		return at.UTC()
	}
	return now().UTC()
}

func (s *Service) revokeCAS(ctx context.Context, grant Grant, now time.Time) (Grant, error) {
	revoked, err := s.store.MarkRevoked(ctx, grant.ID, grant.Status, now)
	if errors.Is(err, ErrConcurrentTransition) {
		return Grant{}, ErrRevokeConflict
	}
	if err != nil {
		return Grant{}, fmt.Errorf("revoke trial grant: %w", err)
	}
	revoked.PreviousStatus = grant.Status
	return revoked, nil
}
