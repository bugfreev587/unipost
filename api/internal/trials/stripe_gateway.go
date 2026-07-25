package trials

import (
	"context"
	"fmt"
	"time"

	"github.com/stripe/stripe-go/v82"
	"github.com/xiaoboyu/unipost-api/internal/billing"
)

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
	CreateTrialCheckout(context.Context, CreateTrialCheckoutRequest) (CheckoutSnapshot, error)
	RetrieveCheckout(context.Context, string, string) (CheckoutSnapshot, error)
	ExpireCheckout(context.Context, string, string) (CheckoutSnapshot, error)
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
	StripeMode    string
	WorkspaceID   string
	PlanID        string
	TrialGrantID  string
	TrialKind     Kind
	Environment   string
	CustomerID    string
	CustomerEmail string
	PriceID       string
	DurationDays  int32
	SuccessURL    string
	CancelURL     string
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
	manager *billing.Manager
}

var _ StripeGateway = (*stripeGateway)(nil)

func NewStripeGateway(manager *billing.Manager) StripeGateway {
	return &stripeGateway{manager: manager}
}

func (g *stripeGateway) CreatePaidTrialSchedule(ctx context.Context, req CreatePaidTrialScheduleRequest) (ScheduleSnapshot, error) {
	mode, err := g.mode(req.StripeMode)
	if err != nil {
		return ScheduleSnapshot{}, err
	}
	createParams, updateParams, err := buildPaidTrialScheduleParams(req)
	if err != nil {
		return ScheduleSnapshot{}, err
	}
	createParams.Context = ctx
	created, err := mode.Client.SubscriptionSchedules.New(createParams)
	if err != nil {
		return ScheduleSnapshot{}, fmt.Errorf("create Stripe trial schedule: %w", err)
	}
	if created == nil || created.ID == "" {
		return ScheduleSnapshot{}, fmt.Errorf("create Stripe trial schedule: Stripe returned no schedule ID")
	}

	// Stripe forbids phases when creating from_subscription, so this operation
	// necessarily uses create+update. Both calls have stable sub-operation keys.
	// If update fails, return the created schedule snapshot and ID so a retry can
	// reconcile the partial success before repeating the idempotent update.
	updateParams.Context = ctx
	configured, err := mode.Client.SubscriptionSchedules.Update(created.ID, updateParams)
	if err != nil {
		return scheduleSnapshot(req.StripeMode, created), fmt.Errorf("configure created Stripe trial schedule %s: %w", created.ID, err)
	}
	return scheduleSnapshot(req.StripeMode, configured), nil
}

func (g *stripeGateway) CreateTrialCheckout(ctx context.Context, req CreateTrialCheckoutRequest) (CheckoutSnapshot, error) {
	mode, err := g.mode(req.StripeMode)
	if err != nil {
		return CheckoutSnapshot{}, err
	}
	params, err := buildTrialCheckoutParams(req)
	if err != nil {
		return CheckoutSnapshot{}, err
	}
	params.Context = ctx
	session, err := mode.Client.CheckoutSessions.New(params)
	if err != nil {
		return CheckoutSnapshot{}, fmt.Errorf("create Stripe trial checkout: %w", err)
	}
	return checkoutSnapshot(req.StripeMode, session), nil
}

func (g *stripeGateway) RetrieveCheckout(ctx context.Context, modeName, sessionID string) (CheckoutSnapshot, error) {
	mode, err := g.mode(modeName)
	if err != nil {
		return CheckoutSnapshot{}, err
	}
	params := &stripe.CheckoutSessionParams{}
	params.Context = ctx
	session, err := mode.Client.CheckoutSessions.Get(sessionID, params)
	if err != nil {
		return CheckoutSnapshot{}, fmt.Errorf("retrieve Stripe checkout %s: %w", sessionID, err)
	}
	return checkoutSnapshot(modeName, session), nil
}

func (g *stripeGateway) ExpireCheckout(ctx context.Context, modeName, sessionID string) (CheckoutSnapshot, error) {
	mode, err := g.mode(modeName)
	if err != nil {
		return CheckoutSnapshot{}, err
	}
	params := &stripe.CheckoutSessionExpireParams{}
	params.Context = ctx
	session, err := mode.Client.CheckoutSessions.Expire(sessionID, params)
	if err != nil {
		return CheckoutSnapshot{}, fmt.Errorf("expire Stripe checkout %s: %w", sessionID, err)
	}
	return checkoutSnapshot(modeName, session), nil
}

func (g *stripeGateway) RetrieveSubscription(ctx context.Context, modeName, subscriptionID string) (SubscriptionSnapshot, error) {
	mode, err := g.mode(modeName)
	if err != nil {
		return SubscriptionSnapshot{}, err
	}
	params := &stripe.SubscriptionParams{}
	params.Context = ctx
	subscription, err := mode.Client.Subscriptions.Get(subscriptionID, params)
	if err != nil {
		return SubscriptionSnapshot{}, fmt.Errorf("retrieve Stripe subscription %s: %w", subscriptionID, err)
	}
	return subscriptionSnapshot(modeName, subscription), nil
}

func (g *stripeGateway) ChangeFreeTrialPlanNow(ctx context.Context, req ChangeFreeTrialPlanRequest) (SubscriptionSnapshot, error) {
	mode, err := g.mode(req.StripeMode)
	if err != nil {
		return SubscriptionSnapshot{}, err
	}
	params, err := buildChangeFreeTrialPlanParams(req)
	if err != nil {
		return SubscriptionSnapshot{}, err
	}
	params.Context = ctx
	subscription, err := mode.Client.Subscriptions.Update(req.SubscriptionID, params)
	if err != nil {
		return SubscriptionSnapshot{}, fmt.Errorf("change Stripe free-trial plan: %w", err)
	}
	return subscriptionSnapshot(req.StripeMode, subscription), nil
}

func (g *stripeGateway) ChangeScheduledTrialPlanNow(ctx context.Context, req ChangeScheduledTrialPlanRequest) (ScheduleSnapshot, error) {
	mode, err := g.mode(req.StripeMode)
	if err != nil {
		return ScheduleSnapshot{}, err
	}
	return changeScheduledTrialPlanNow(ctx, mode.Client.SubscriptionSchedules, req)
}

type schedulePlanChanger interface {
	Update(string, *stripe.SubscriptionScheduleParams) (*stripe.SubscriptionSchedule, error)
}

func changeScheduledTrialPlanNow(ctx context.Context, client schedulePlanChanger, req ChangeScheduledTrialPlanRequest) (ScheduleSnapshot, error) {
	params, err := buildChangeScheduledTrialPlanParams(req)
	if err != nil {
		return ScheduleSnapshot{}, err
	}
	params.Context = ctx
	schedule, err := client.Update(req.ScheduleID, params)
	if err != nil {
		return ScheduleSnapshot{}, fmt.Errorf("change Stripe scheduled-trial plan: %w", err)
	}
	return scheduleSnapshot(req.StripeMode, schedule), nil
}

func (g *stripeGateway) CancelFreeTrialAtEnd(ctx context.Context, req CancelFreeTrialRequest) (SubscriptionSnapshot, error) {
	mode, err := g.mode(req.StripeMode)
	if err != nil {
		return SubscriptionSnapshot{}, err
	}
	params, err := buildCancelFreeTrialParams(req)
	if err != nil {
		return SubscriptionSnapshot{}, err
	}
	params.Context = ctx
	subscription, err := mode.Client.Subscriptions.Update(req.SubscriptionID, params)
	if err != nil {
		return SubscriptionSnapshot{}, fmt.Errorf("cancel Stripe free-trial renewal: %w", err)
	}
	return subscriptionSnapshot(req.StripeMode, subscription), nil
}

func (g *stripeGateway) CancelPaidScheduleAtTrialEnd(ctx context.Context, req CancelPaidScheduleRequest) (ScheduleSnapshot, error) {
	mode, err := g.mode(req.StripeMode)
	if err != nil {
		return ScheduleSnapshot{}, err
	}
	params, err := buildCancelPaidScheduleParams(req)
	if err != nil {
		return ScheduleSnapshot{}, err
	}
	params.Context = ctx
	schedule, err := mode.Client.SubscriptionSchedules.Update(req.ScheduleID, params)
	if err != nil {
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
	params.Context = ctx
	session, err := mode.Client.BillingPortalSessions.New(params)
	if err != nil {
		return "", fmt.Errorf("create Stripe billing portal: %w", err)
	}
	if session == nil || session.URL == "" {
		return "", fmt.Errorf("create Stripe billing portal: Stripe returned no URL")
	}
	return session.URL, nil
}

func (g *stripeGateway) mode(name string) (*billing.Mode, error) {
	if g == nil || g.manager == nil {
		return nil, fmt.Errorf("Stripe trial gateway is not configured")
	}
	mode := g.manager.ByName(name)
	if mode == nil || mode.Client == nil {
		return nil, fmt.Errorf("Stripe mode %q is unavailable", name)
	}
	return mode, nil
}

func buildTrialCheckoutParams(req CreateTrialCheckoutRequest) (*stripe.CheckoutSessionParams, error) {
	if err := requireTrialGrantID(req.TrialGrantID); err != nil {
		return nil, err
	}
	if req.DurationDays < 1 || req.DurationDays > 730 {
		return nil, fmt.Errorf("trial checkout duration must be between 1 and 730 days")
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
	if req.CustomerID != "" {
		params.Customer = stripe.String(req.CustomerID)
	} else if req.CustomerEmail != "" {
		params.CustomerEmail = stripe.String(req.CustomerEmail)
	}
	params.SetIdempotencyKey(trialOperationKey(req.TrialGrantID, "checkout"))
	return params, nil
}

func buildPaidTrialScheduleParams(req CreatePaidTrialScheduleRequest) (*stripe.SubscriptionScheduleParams, *stripe.SubscriptionScheduleParams, error) {
	if err := requireTrialGrantID(req.TrialGrantID); err != nil {
		return nil, nil, err
	}
	if req.DurationDays < 1 || req.DurationDays > 730 {
		return nil, nil, fmt.Errorf("paid trial duration must be between 1 and 730 days")
	}
	if req.SubscriptionID == "" || req.PriceID == "" || req.TrialStartAt.IsZero() || req.TrialEndAt.IsZero() || !req.TrialEndAt.After(req.TrialStartAt) {
		return nil, nil, fmt.Errorf("paid trial schedule requires subscription, price, and increasing trial dates")
	}
	metadata := trialMetadata(req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment)
	createParams := &stripe.SubscriptionScheduleParams{FromSubscription: stripe.String(req.SubscriptionID)}
	createParams.SetIdempotencyKey(trialOperationKey(req.TrialGrantID, "schedule") + ":create")

	current := req.CurrentPhase
	if current.PriceID == "" {
		current.PriceID = req.PriceID
	}
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
	if err := requireTrialGrantID(req.TrialGrantID); err != nil {
		return nil, err
	}
	if req.SubscriptionID == "" || req.SubscriptionItemID == "" || req.PriceID == "" {
		return nil, fmt.Errorf("free trial plan change requires subscription, existing item, and price")
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
	if err := requireTrialGrantID(req.TrialGrantID); err != nil {
		return nil, err
	}
	if req.ScheduleID == "" || req.PriceID == "" {
		return nil, fmt.Errorf("scheduled trial plan change requires schedule and price")
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
	if err := requireTrialGrantID(req.TrialGrantID); err != nil {
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
	if err := requireTrialGrantID(req.TrialGrantID); err != nil {
		return nil, err
	}
	if req.ScheduleID == "" || len(req.RetainedPhases) == 0 {
		return nil, fmt.Errorf("paid trial cancellation requires schedule and retained phases")
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
	if req.RequireTrialSafeConfiguration && trialConfigurationID == "" {
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
	if grantID == "" {
		return fmt.Errorf("trial grant ID is required for Stripe idempotency")
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
