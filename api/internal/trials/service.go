package trials

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrWorkspaceNotFound         = errors.New("workspace not found")
	ErrGrantNotFound             = errors.New("trial grant not found")
	ErrOpenGrantExists           = errors.New("workspace already has an open trial grant")
	ErrPaidPlanMismatch          = errors.New("paid trial plan must match the current plan")
	ErrIneligibleSubscription    = errors.New("subscription is not eligible for a trial grant")
	ErrRevokeConflict            = errors.New("trial grant can no longer be revoked")
	ErrConcurrentTransition      = errors.New("trial grant changed concurrently")
	ErrBillingModeUnavailable    = errors.New("Stripe billing mode is unavailable")
	ErrUnrelatedSchedule         = errors.New("subscription already has an unrelated Stripe schedule")
	ErrCheckoutExpiryUnconfirmed = errors.New("Checkout Session expiry could not be confirmed")
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
