package trials

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrWorkspaceNotFound      = errors.New("workspace not found")
	ErrGrantNotFound          = errors.New("trial grant not found")
	ErrOpenGrantExists        = errors.New("workspace already has an open trial grant")
	ErrPaidPlanMismatch       = errors.New("paid trial plan must match the current plan")
	ErrIneligibleSubscription = errors.New("subscription is not eligible for a trial grant")
	ErrRevokeConflict         = errors.New("trial grant can no longer be revoked")
	ErrConcurrentTransition   = errors.New("trial grant changed concurrently")
	ErrBillingModeUnavailable = errors.New("Stripe billing mode is unavailable")
)

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

	if _, err := s.store.GetOpenGrant(ctx, req.WorkspaceID); err == nil {
		return Grant{}, ErrOpenGrantExists
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
		mode, err := s.modes.Resolve(ctx, billingState.OwnerUserID, req.PlanID)
		if err != nil || mode.Name == "" || mode.PriceID == "" {
			return Grant{}, fmt.Errorf("%w: %v", ErrBillingModeUnavailable, err)
		}
		return s.store.CreateGrant(ctx, CreateGrantInput{
			WorkspaceID: req.WorkspaceID, Kind: KindFreeToPaid, PlanID: req.PlanID,
			DurationDays: req.DurationDays, Status: StatusPendingActivation,
			ActorUserID: req.ActorUserID, StripeMode: mode.Name, GrantedAt: now,
		})
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
	remote, err := s.stripe.RetrieveSubscription(ctx, mode.Name, billingState.Subscription.StripeSubscriptionID)
	if err != nil {
		return Grant{}, fmt.Errorf("retrieve Stripe subscription: %w", err)
	}
	if !eligiblePaidSubscription(remote, billingState.Subscription, mode.PriceID) {
		return Grant{}, ErrIneligibleSubscription
	}

	grant, err := s.store.CreateGrant(ctx, CreateGrantInput{
		WorkspaceID: req.WorkspaceID, Kind: KindPaidSamePlan, PlanID: req.PlanID,
		DurationDays: req.DurationDays, Status: StatusProvisioning, ActorUserID: req.ActorUserID,
		StripeMode: mode.Name, StripeCustomerID: remote.CustomerID,
		StripeSubscriptionID: remote.ID, GrantedAt: now,
	})
	if err != nil {
		return Grant{}, fmt.Errorf("create provisioning trial grant: %w", err)
	}

	periodStart := remote.CurrentPeriodStartAt.UTC()
	periodEnd := remote.CurrentPeriodEndAt.UTC()
	trialEnd := periodEnd.AddDate(0, 0, int(req.DurationDays))
	schedule, stripeErr := s.stripe.CreatePaidTrialSchedule(ctx, CreatePaidTrialScheduleRequest{
		StripeMode: mode.Name, WorkspaceID: req.WorkspaceID, PlanID: req.PlanID,
		TrialGrantID: grant.ID, TrialKind: KindPaidSamePlan, Environment: s.environment,
		SubscriptionID: remote.ID, PriceID: mode.PriceID, DurationDays: req.DurationDays,
		CurrentPhase: SchedulePhase{PriceID: mode.PriceID, StartAt: periodStart, EndAt: periodEnd},
		TrialStartAt: periodEnd, TrialEndAt: trialEnd,
	})
	if stripeErr != nil {
		// A non-empty schedule means Stripe attached a Schedule but its
		// configuration was not confirmed. Keep the grant open so no second
		// grant can be issued while metadata/webhook reconciliation catches up.
		if schedule.ID != "" {
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
		return Grant{}, fmt.Errorf("record scheduled trial: %w", err)
	}
	return scheduled, nil
}

func eligiblePaidSubscription(remote SubscriptionSnapshot, local SubscriptionRecord, priceID string) bool {
	return remote.ID == local.StripeSubscriptionID &&
		remote.Status == "active" &&
		remote.PriceID == priceID &&
		remote.ScheduleID == "" &&
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
		expired, err := s.stripe.ExpireCheckout(ctx, ExpireCheckoutRequest{
			StripeMode: grant.StripeMode, TrialGrantID: grant.ID,
			CheckoutSessionID: grant.StripeCheckoutSessionID,
		})
		if err != nil || expired.ID != grant.StripeCheckoutSessionID || expired.Status != "expired" {
			return Grant{}, ErrRevokeConflict
		}
		return s.revokeCAS(ctx, grant, now)
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
