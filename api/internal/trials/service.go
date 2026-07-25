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
	if grant.Kind != KindFreeToPaid || grant.PlanID != req.PlanID {
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
			CheckoutSessionID: grant.StripeCheckoutSessionID,
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
