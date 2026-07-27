package billing

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/stripe/stripe-go/v82"
)

var (
	ErrInvalidPlanChangeRequest           = errors.New("invalid plan change request")
	ErrPlanChangeConfigurationUnavailable = errors.New("plan change configuration unavailable")
	ErrPlanChangeBindingConflict          = errors.New("plan change subscription binding conflict")
	ErrPlanChangeUpstream                 = errors.New("plan change provider failure")
)

type PlanChangeRequest struct {
	StripeMode     string
	WorkspaceID    string
	CustomerID     string
	SubscriptionID string
	CurrentPlanID  string
	TargetPlanID   string
	ReturnURL      string
}

type PlanChangeResult struct {
	URL string
}

type planChangeStripeClient interface {
	GetSubscription(string, *stripe.SubscriptionParams) (*stripe.Subscription, error)
	CreatePortalSession(*stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error)
}

type modePlanChangeStripeClient struct {
	mode *Mode
}

func (c *modePlanChangeStripeClient) GetSubscription(id string, params *stripe.SubscriptionParams) (*stripe.Subscription, error) {
	return c.mode.Client.Subscriptions.Get(id, params)
}

func (c *modePlanChangeStripeClient) CreatePortalSession(params *stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
	return c.mode.Client.BillingPortalSessions.New(params)
}

type PlanChangeService struct {
	manager       *Manager
	environment   string
	clientForMode func(*Mode) planChangeStripeClient
}

func NewPlanChangeService(manager *Manager, environment string) *PlanChangeService {
	return newPlanChangeService(manager, environment, func(mode *Mode) planChangeStripeClient {
		if mode == nil || mode.Client == nil || mode.Client.Subscriptions == nil || mode.Client.BillingPortalSessions == nil {
			return nil
		}
		return &modePlanChangeStripeClient{mode: mode}
	})
}

func newPlanChangeService(manager *Manager, environment string, clientForMode func(*Mode) planChangeStripeClient) *PlanChangeService {
	return &PlanChangeService{
		manager:       manager,
		environment:   strings.TrimSpace(environment),
		clientForMode: clientForMode,
	}
}

func (s *PlanChangeService) CreateSession(ctx context.Context, req PlanChangeRequest) (PlanChangeResult, error) {
	req = normalizePlanChangeRequest(req)
	if err := validatePlanChangeRequest(req); err != nil {
		return PlanChangeResult{}, err
	}

	mode := s.manager.ByName(req.StripeMode)
	if mode == nil {
		return PlanChangeResult{}, ErrPlanChangeConfigurationUnavailable
	}
	configurationID := strings.TrimSpace(mode.PlanChangePortalConfigurationID())
	if configurationID == "" {
		return PlanChangeResult{}, ErrPlanChangeConfigurationUnavailable
	}
	currentPriceID := strings.TrimSpace(mode.PriceID(req.CurrentPlanID))
	targetPriceID := strings.TrimSpace(mode.PriceID(req.TargetPlanID))
	if currentPriceID == "" || targetPriceID == "" || currentPriceID == targetPriceID {
		return PlanChangeResult{}, ErrInvalidPlanChangeRequest
	}
	if s.clientForMode == nil {
		return PlanChangeResult{}, ErrPlanChangeConfigurationUnavailable
	}
	client := s.clientForMode(mode)
	if client == nil {
		return PlanChangeResult{}, ErrPlanChangeConfigurationUnavailable
	}

	getParams := &stripe.SubscriptionParams{}
	getParams.Context = ctx
	subscription, err := client.GetSubscription(req.SubscriptionID, getParams)
	if err != nil {
		return PlanChangeResult{}, fmt.Errorf("%w: retrieve subscription", ErrPlanChangeUpstream)
	}
	item, err := validatePlanChangeSubscription(subscription, mode, s.environment, req, currentPriceID)
	if err != nil {
		return PlanChangeResult{}, err
	}

	params := &stripe.BillingPortalSessionParams{
		Configuration: stripe.String(configurationID),
		Customer:      stripe.String(req.CustomerID),
		ReturnURL:     stripe.String(req.ReturnURL),
		FlowData: &stripe.BillingPortalSessionFlowDataParams{
			Type: stripe.String(string(stripe.BillingPortalSessionFlowTypeSubscriptionUpdateConfirm)),
			AfterCompletion: &stripe.BillingPortalSessionFlowDataAfterCompletionParams{
				Type: stripe.String(string(stripe.BillingPortalSessionFlowAfterCompletionTypeRedirect)),
				Redirect: &stripe.BillingPortalSessionFlowDataAfterCompletionRedirectParams{
					ReturnURL: stripe.String(req.ReturnURL),
				},
			},
			SubscriptionUpdateConfirm: &stripe.BillingPortalSessionFlowDataSubscriptionUpdateConfirmParams{
				Subscription: stripe.String(req.SubscriptionID),
				Items: []*stripe.BillingPortalSessionFlowDataSubscriptionUpdateConfirmItemParams{{
					ID:       stripe.String(item.ID),
					Price:    stripe.String(targetPriceID),
					Quantity: stripe.Int64(1),
				}},
			},
		},
	}
	params.Context = ctx
	portal, err := client.CreatePortalSession(params)
	if err != nil {
		return PlanChangeResult{}, fmt.Errorf("%w: create Portal Session", ErrPlanChangeUpstream)
	}
	if portal == nil || strings.TrimSpace(portal.ID) == "" || !isHTTPSURL(portal.URL) {
		return PlanChangeResult{}, fmt.Errorf("%w: incomplete Portal Session", ErrPlanChangeUpstream)
	}
	return PlanChangeResult{URL: strings.TrimSpace(portal.URL)}, nil
}

func normalizePlanChangeRequest(req PlanChangeRequest) PlanChangeRequest {
	req.StripeMode = strings.TrimSpace(req.StripeMode)
	req.WorkspaceID = strings.TrimSpace(req.WorkspaceID)
	req.CustomerID = strings.TrimSpace(req.CustomerID)
	req.SubscriptionID = strings.TrimSpace(req.SubscriptionID)
	req.CurrentPlanID = strings.TrimSpace(req.CurrentPlanID)
	req.TargetPlanID = strings.TrimSpace(req.TargetPlanID)
	req.ReturnURL = strings.TrimSpace(req.ReturnURL)
	return req
}

func validatePlanChangeRequest(req PlanChangeRequest) error {
	if req.StripeMode == "" || req.WorkspaceID == "" || req.CustomerID == "" || req.SubscriptionID == "" || req.CurrentPlanID == "" || req.TargetPlanID == "" || req.CurrentPlanID == req.TargetPlanID || !isHTTPSURL(req.ReturnURL) {
		return ErrInvalidPlanChangeRequest
	}
	return nil
}

func validatePlanChangeSubscription(subscription *stripe.Subscription, mode *Mode, environment string, req PlanChangeRequest, currentPriceID string) (*stripe.SubscriptionItem, error) {
	if subscription == nil || strings.TrimSpace(subscription.ID) != req.SubscriptionID || subscription.Status != stripe.SubscriptionStatusActive {
		return nil, ErrPlanChangeBindingConflict
	}
	if subscription.Customer == nil || strings.TrimSpace(subscription.Customer.ID) != req.CustomerID {
		return nil, ErrPlanChangeBindingConflict
	}
	if mode.Name == "live" {
		if !subscription.Livemode {
			return nil, ErrPlanChangeBindingConflict
		}
	} else if mode.Name == "sandbox" {
		if subscription.Livemode {
			return nil, ErrPlanChangeBindingConflict
		}
	} else {
		return nil, ErrPlanChangeBindingConflict
	}
	if metadataConflicts(subscription.Metadata, "workspace_id", req.WorkspaceID) ||
		metadataConflicts(subscription.Metadata, "mode", mode.Name) ||
		metadataConflicts(subscription.Metadata, "unipost_environment", environment) {
		return nil, ErrPlanChangeBindingConflict
	}
	if subscription.Items == nil || len(subscription.Items.Data) != 1 || subscription.Items.Data[0] == nil {
		return nil, ErrPlanChangeBindingConflict
	}
	item := subscription.Items.Data[0]
	if strings.TrimSpace(item.ID) == "" || item.Quantity != 1 || item.Price == nil || strings.TrimSpace(item.Price.ID) != currentPriceID {
		return nil, ErrPlanChangeBindingConflict
	}
	return item, nil
}

func metadataConflicts(metadata map[string]string, key, expected string) bool {
	actual := strings.TrimSpace(metadata[key])
	return actual != "" && actual != strings.TrimSpace(expected)
}

func isHTTPSURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}
