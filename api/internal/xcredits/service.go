package xcredits

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/events"
	"github.com/xiaoboyu/unipost-api/internal/xinbox"
)

const (
	UsageStatusProvisional = "provisional"
	UsageStatusFinalized   = "finalized"
	UsageStatusReversed    = "reversed"
	UsageStatusBypassed    = "bypassed"
)

var (
	ErrMonthlyLimitExceeded                   = errors.New("x_monthly_usage_limit_exceeded: managed X usage has reached this billing period's allowance")
	ErrAllowanceNotConfigured                 = errors.New("x_monthly_usage_limit_not_configured: contact UniPost to configure the workspace X allowance")
	ErrInboundDailyCapExceeded                = errors.New("x_inbound_daily_cap_exceeded: managed X inbound usage has reached today's workspace cap")
	ErrInboundCapExceedsMonthlyRemaining      = errors.New("x_inbound_cap_exceeds_monthly_remaining: inbound daily limit cannot exceed the remaining monthly allowance")
	ErrInboundExposureAcknowledgementRequired = errors.New("x_inbound_exposure_acknowledgement_required: raising the inbound daily limit requires explicit exposure acknowledgement")
)

const (
	InboundDecisionAccepted                   = "accepted"
	InboundDecisionSuppressedDailyCap         = "suppressed_daily_cap"
	InboundDecisionSuppressedMonthlyAllowance = "suppressed_monthly_allowance"

	PauseReasonDailySafetyBuffer = "daily_safety_buffer"
	PauseReasonDailyCap          = "daily_cap"
	PauseReasonMonthlyAllowance  = "monthly_allowance"
)

type WorkspacePeriod struct {
	PlanID            string
	Start             time.Time
	End               time.Time
	MonthlyAllowance  *int64
	InboundDailyLimit *int64
}

type ReserveRequest struct {
	WorkspaceID     string
	SocialAccountID string
	AppMode         string
	ConnectionType  string
	OperationKey    string
	Source          string
	IdempotencyKey  string
	RequestedUnits  int64
	Now             time.Time
}

type StoreReserveRequest struct {
	WorkspaceID        string
	SocialAccountID    string
	AppMode            string
	OperationKey       string
	CatalogVersion     string
	Source             string
	IdempotencyKey     string
	WeightedUnits      int64
	WeightedUnitsLimit int64
	PeriodStart        time.Time
	PeriodEnd          time.Time
}

type UsageEvent struct {
	ID             string `json:"id,omitempty"`
	Status         string `json:"status"`
	OperationKey   string `json:"operation_key,omitempty"`
	CatalogVersion string `json:"catalog_version,omitempty"`
	WeightedUnits  int64  `json:"weighted_units"`
	Duplicate      bool   `json:"duplicate,omitempty"`
}

type Snapshot struct {
	PlanID             string    `json:"plan_id"`
	PeriodStart        time.Time `json:"billing_period_start"`
	PeriodEnd          time.Time `json:"billing_period_end"`
	MonthlyAllowance   *int64    `json:"monthly_allowance"`
	MonthlyUsed        int64     `json:"monthly_used"`
	MonthlyRemaining   *int64    `json:"monthly_remaining"`
	InboundDailyUsed   int64     `json:"inbound_daily_usage"`
	InboundDailyLimit  *int64    `json:"inbound_daily_limit"`
	InboundAccepted    int64     `json:"inbound_events_accepted"`
	InboundSuppressed  int64     `json:"inbound_events_suppressed"`
	InboundResetAt     time.Time `json:"inbound_daily_reset_at"`
	InboundPercent     float64   `json:"inbound_daily_percent"`
	PausePaidSources   bool      `json:"pause_paid_sources"`
	InboundPauseReason string    `json:"inbound_pause_reason,omitempty"`
	CatalogVersion     string    `json:"catalog_version"`
}

type InboundRequest struct {
	WorkspaceID          string
	SocialAccountID      string
	AppMode              string
	OperationKey         string
	Source               string
	UpstreamResourceType string
	UpstreamResourceID   string
	RequestedUnits       int64
	Now                  time.Time
}

type StoreInboundRequest struct {
	WorkspaceID          string
	SocialAccountID      string
	AppMode              string
	OperationKey         string
	CatalogVersion       string
	Source               string
	UpstreamResourceType string
	UpstreamResourceID   string
	WeightedUnits        int64
	MonthlyAllowance     int64
	InboundDailyLimit    int64
	PeriodStart          time.Time
	PeriodEnd            time.Time
	UTCDate              time.Time
}

type InboundAdmission struct {
	Decision          string    `json:"decision"`
	Duplicate         bool      `json:"duplicate,omitempty"`
	Bypassed          bool      `json:"bypassed,omitempty"`
	WeightedUnits     int64     `json:"weighted_units"`
	MonthlyUsed       int64     `json:"monthly_used"`
	MonthlyRemaining  int64     `json:"monthly_remaining"`
	InboundDailyUsed  int64     `json:"inbound_daily_usage"`
	InboundDailyLimit int64     `json:"inbound_daily_limit"`
	EventsAccepted    int64     `json:"events_accepted"`
	EventsSuppressed  int64     `json:"events_suppressed"`
	PausePaidSources  bool      `json:"pause_paid_sources"`
	PauseReason       string    `json:"pause_reason,omitempty"`
	ResetAt           time.Time `json:"reset_at"`
	Claimed80Percent  bool      `json:"-"`
	Claimed100Percent bool      `json:"-"`
}

type inboundReceiptSnapshot struct {
	Decision              string
	WeightedUnits         int64
	PeriodStart           time.Time
	PeriodEnd             time.Time
	MonthlyUsedAfter      int64
	MonthlyRemainingAfter int64
	InboundDailyUsedAfter int64
	InboundDailyLimit     int64
	EventsAcceptedAfter   int64
	EventsSuppressedAfter int64
	PausePaidSources      bool
	PauseReason           string
	ResetAt               time.Time
}

func admissionFromReceipt(receipt inboundReceiptSnapshot) InboundAdmission {
	return InboundAdmission{
		Decision:          receipt.Decision,
		Duplicate:         true,
		WeightedUnits:     receipt.WeightedUnits,
		MonthlyUsed:       receipt.MonthlyUsedAfter,
		MonthlyRemaining:  receipt.MonthlyRemainingAfter,
		InboundDailyUsed:  receipt.InboundDailyUsedAfter,
		InboundDailyLimit: receipt.InboundDailyLimit,
		EventsAccepted:    receipt.EventsAcceptedAfter,
		EventsSuppressed:  receipt.EventsSuppressedAfter,
		PausePaidSources:  receipt.PausePaidSources,
		PauseReason:       receipt.PauseReason,
		ResetAt:           receipt.ResetAt,
	}
}

type UpdateInboundCapRequest struct {
	WorkspaceID          string
	InboundDailyLimit    int64
	UpdatedBy            string
	AcknowledgedExposure bool
	Now                  time.Time
}

type StoreUpdateInboundCapRequest struct {
	UpdateInboundCapRequest
	MonthlyAllowance int64
	PeriodStart      time.Time
	PeriodEnd        time.Time
}

type InboundCapSetting struct {
	InboundDailyLimit    int64     `json:"inbound_daily_limit"`
	UpdatedBy            string    `json:"updated_by"`
	AcknowledgedExposure bool      `json:"acknowledged_exposure"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type InboundNotification struct {
	InboundDailyUsed  int64     `json:"inbound_daily_usage"`
	InboundDailyLimit int64     `json:"inbound_daily_limit"`
	ResetAt           time.Time `json:"reset_at"`
	CapManagementURL  string    `json:"cap_management_url"`
}

func (n InboundNotification) String() string {
	body, _ := json.Marshal(n)
	return string(body)
}

type Store interface {
	ResolveWorkspacePeriod(context.Context, string, time.Time) (WorkspacePeriod, error)
	Reserve(context.Context, StoreReserveRequest) (UsageEvent, error)
	Finalize(context.Context, string, int64) error
	Reverse(context.Context, string) error
	Snapshot(context.Context, string, time.Time) (Snapshot, error)
}

type InboundStore interface {
	AdmitInbound(context.Context, StoreInboundRequest) (InboundAdmission, error)
	UpdateInboundCap(context.Context, StoreUpdateInboundCapRequest) (InboundCapSetting, error)
}

type Service struct {
	store      Store
	inbound    InboundStore
	eventBus   events.EventBus
	appBaseURL string
}

func NewService(store Store) *Service {
	service := &Service{store: store}
	if inbound, ok := store.(InboundStore); ok {
		service.inbound = inbound
	}
	return service
}

func (s *Service) SetEventBus(bus events.EventBus, appBaseURL string) *Service {
	s.eventBus = bus
	s.appBaseURL = strings.TrimRight(appBaseURL, "/")
	if s.appBaseURL == "" {
		s.appBaseURL = "https://app.unipost.dev"
	}
	return s
}

func (s *Service) Reserve(ctx context.Context, req ReserveRequest) (UsageEvent, error) {
	appMode, err := xinbox.NormalizePersistedAppMode(req.AppMode)
	if err != nil {
		return UsageEvent{}, err
	}
	if appMode != xinbox.AppModeUniPostManaged {
		return UsageEvent{Status: UsageStatusBypassed}, nil
	}
	if s == nil || s.store == nil {
		return UsageEvent{}, errors.New("x credits service is not configured")
	}
	if req.WorkspaceID == "" || req.IdempotencyKey == "" || req.OperationKey == "" {
		return UsageEvent{}, errors.New("workspace_id, operation_key, and idempotency_key are required")
	}
	if req.RequestedUnits <= 0 {
		req.RequestedUnits = OperationWeight(req.OperationKey)
	}
	if req.RequestedUnits <= 0 {
		return UsageEvent{}, fmt.Errorf("unknown X credit operation %q", req.OperationKey)
	}
	if req.Now.IsZero() {
		req.Now = time.Now().UTC()
	}

	period, err := s.store.ResolveWorkspacePeriod(ctx, req.WorkspaceID, req.Now)
	if err != nil {
		return UsageEvent{}, err
	}
	allowance := int64(0)
	if period.MonthlyAllowance != nil {
		allowance = *period.MonthlyAllowance
	} else {
		var ok bool
		allowance, ok = PlanAllowance(period.PlanID)
		if !ok {
			return UsageEvent{}, ErrAllowanceNotConfigured
		}
	}
	if period.Start.IsZero() || period.End.IsZero() || !period.End.After(period.Start) {
		period.Start, period.End = CalendarMonthPeriod(req.Now)
	}

	return s.store.Reserve(ctx, StoreReserveRequest{
		WorkspaceID:        req.WorkspaceID,
		SocialAccountID:    req.SocialAccountID,
		AppMode:            req.AppMode,
		OperationKey:       req.OperationKey,
		CatalogVersion:     CatalogVersion,
		Source:             req.Source,
		IdempotencyKey:     req.IdempotencyKey,
		WeightedUnits:      req.RequestedUnits,
		WeightedUnitsLimit: allowance,
		PeriodStart:        period.Start,
		PeriodEnd:          period.End,
	})
}

func (s *Service) Finalize(ctx context.Context, eventID string, finalUnits int64) error {
	if s == nil || s.store == nil || eventID == "" {
		return nil
	}
	if finalUnits < 0 {
		return errors.New("final X usage cannot be negative")
	}
	return s.store.Finalize(ctx, eventID, finalUnits)
}

func (s *Service) Reverse(ctx context.Context, eventID string) error {
	if s == nil || s.store == nil || eventID == "" {
		return nil
	}
	return s.store.Reverse(ctx, eventID)
}

func (s *Service) Snapshot(ctx context.Context, workspaceID string, now time.Time) (Snapshot, error) {
	if s == nil || s.store == nil {
		return Snapshot{}, errors.New("x credits service is not configured")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return s.store.Snapshot(ctx, workspaceID, now)
}

func (s *Service) AdmitInbound(ctx context.Context, req InboundRequest) (InboundAdmission, error) {
	appMode, err := xinbox.NormalizePersistedAppMode(req.AppMode)
	if err != nil {
		return InboundAdmission{}, err
	}
	if appMode != xinbox.AppModeUniPostManaged {
		return InboundAdmission{
			Decision: InboundDecisionAccepted,
			Bypassed: true,
		}, nil
	}
	if s == nil || s.store == nil || s.inbound == nil {
		return InboundAdmission{}, errors.New("x inbound credits service is not configured")
	}
	if req.WorkspaceID == "" || req.SocialAccountID == "" || req.OperationKey == "" || req.Source == "" ||
		req.UpstreamResourceType == "" || req.UpstreamResourceID == "" {
		return InboundAdmission{}, errors.New("workspace_id, social_account_id, operation_key, source, upstream_resource_type, and upstream_resource_id are required")
	}
	if req.RequestedUnits <= 0 {
		req.RequestedUnits = OperationWeight(req.OperationKey)
	}
	if req.RequestedUnits <= 0 {
		return InboundAdmission{}, fmt.Errorf("unknown X credit operation %q", req.OperationKey)
	}
	if req.Now.IsZero() {
		req.Now = time.Now().UTC()
	}

	period, err := s.store.ResolveWorkspacePeriod(ctx, req.WorkspaceID, req.Now)
	if err != nil {
		return InboundAdmission{}, err
	}
	monthlyAllowance, err := resolveMonthlyAllowance(period)
	if err != nil {
		return InboundAdmission{}, err
	}
	inboundLimit, err := resolveInboundDailyLimit(period)
	if err != nil {
		return InboundAdmission{}, err
	}
	if period.Start.IsZero() || period.End.IsZero() || !period.End.After(period.Start) {
		period.Start, period.End = CalendarMonthPeriod(req.Now)
	}
	utcDate := req.Now.UTC()
	utcDate = time.Date(utcDate.Year(), utcDate.Month(), utcDate.Day(), 0, 0, 0, 0, time.UTC)

	admission, err := s.inbound.AdmitInbound(ctx, StoreInboundRequest{
		WorkspaceID:          req.WorkspaceID,
		SocialAccountID:      req.SocialAccountID,
		AppMode:              string(appMode),
		OperationKey:         req.OperationKey,
		CatalogVersion:       CatalogVersion,
		Source:               req.Source,
		UpstreamResourceType: req.UpstreamResourceType,
		UpstreamResourceID:   req.UpstreamResourceID,
		WeightedUnits:        req.RequestedUnits,
		MonthlyAllowance:     monthlyAllowance,
		InboundDailyLimit:    inboundLimit,
		PeriodStart:          period.Start,
		PeriodEnd:            period.End,
		UTCDate:              utcDate,
	})
	if err != nil {
		return InboundAdmission{}, err
	}
	if !admission.Duplicate {
		s.publishInboundNotifications(ctx, req.WorkspaceID, admission)
	}
	switch admission.Decision {
	case InboundDecisionSuppressedDailyCap:
		return admission, ErrInboundDailyCapExceeded
	case InboundDecisionSuppressedMonthlyAllowance:
		return admission, ErrMonthlyLimitExceeded
	default:
		return admission, nil
	}
}

func (s *Service) UpdateInboundCap(ctx context.Context, req UpdateInboundCapRequest) (InboundCapSetting, error) {
	if s == nil || s.store == nil || s.inbound == nil {
		return InboundCapSetting{}, errors.New("x inbound credits service is not configured")
	}
	if req.WorkspaceID == "" || req.UpdatedBy == "" {
		return InboundCapSetting{}, errors.New("workspace_id and updated_by are required")
	}
	if req.InboundDailyLimit < 0 {
		return InboundCapSetting{}, errors.New("inbound_daily_limit cannot be negative")
	}
	if req.Now.IsZero() {
		req.Now = time.Now().UTC()
	}
	snapshot, err := s.store.Snapshot(ctx, req.WorkspaceID, req.Now)
	if err != nil {
		return InboundCapSetting{}, err
	}
	if snapshot.MonthlyAllowance == nil || snapshot.MonthlyRemaining == nil {
		return InboundCapSetting{}, ErrAllowanceNotConfigured
	}
	if req.InboundDailyLimit > *snapshot.MonthlyRemaining {
		return InboundCapSetting{}, ErrInboundCapExceedsMonthlyRemaining
	}
	if snapshot.InboundDailyLimit != nil && req.InboundDailyLimit > *snapshot.InboundDailyLimit && !req.AcknowledgedExposure {
		return InboundCapSetting{}, ErrInboundExposureAcknowledgementRequired
	}
	return s.inbound.UpdateInboundCap(ctx, StoreUpdateInboundCapRequest{
		UpdateInboundCapRequest: req,
		MonthlyAllowance:        *snapshot.MonthlyAllowance,
		PeriodStart:             snapshot.PeriodStart,
		PeriodEnd:               snapshot.PeriodEnd,
	})
}

func resolveMonthlyAllowance(period WorkspacePeriod) (int64, error) {
	if period.MonthlyAllowance != nil {
		return *period.MonthlyAllowance, nil
	}
	if allowance, ok := PlanAllowance(period.PlanID); ok {
		return allowance, nil
	}
	return 0, ErrAllowanceNotConfigured
}

func resolveInboundDailyLimit(period WorkspacePeriod) (int64, error) {
	if period.InboundDailyLimit != nil {
		return *period.InboundDailyLimit, nil
	}
	if limit, ok := InboundDailyLimit(period.PlanID); ok {
		return limit, nil
	}
	return 0, ErrAllowanceNotConfigured
}

func remainingWithinSafetyBuffer(used, limit int64) bool {
	if limit <= 0 {
		return true
	}
	buffer := limit / 10
	if buffer < 20 {
		buffer = 20
	}
	remaining := limit - used
	return remaining <= buffer
}

func (s *Service) publishInboundNotifications(ctx context.Context, workspaceID string, admission InboundAdmission) {
	if s.eventBus == nil {
		return
	}
	payload := InboundNotification{
		InboundDailyUsed:  admission.InboundDailyUsed,
		InboundDailyLimit: admission.InboundDailyLimit,
		ResetAt:           admission.ResetAt,
		CapManagementURL:  s.appBaseURL + "/settings/billing#x-inbound-cap",
	}
	if admission.Claimed80Percent {
		s.eventBus.Publish(ctx, workspaceID, events.EventBillingXInbound80pct, payload)
	}
	if admission.Claimed100Percent {
		s.eventBus.Publish(ctx, workspaceID, events.EventBillingXInboundCapReached, payload)
	}
}

func CalendarMonthPeriod(now time.Time) (time.Time, time.Time) {
	utc := now.UTC()
	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0)
}
