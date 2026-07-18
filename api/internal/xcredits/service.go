package xcredits

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

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
	CapManagementURL     string
	AccountingEnabled    bool
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
	MonthlyAllowance         int64
	DefaultInboundDailyLimit int64
	PeriodStart              time.Time
	PeriodEnd                time.Time
}

type InboundCapSetting struct {
	InboundDailyLimit    int64     `json:"inbound_daily_limit"`
	UpdatedBy            string    `json:"updated_by"`
	AcknowledgedExposure bool      `json:"acknowledged_exposure"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type ExposureReservationRequest struct {
	WorkspaceID        string
	SocialAccountID    string
	AppMode            string
	OperationKey       string
	IdempotencyKey     string
	RequestedResources int
	MinimumResources   int
	UnitsPerResource   int64
	Now                time.Time
}

type StoreExposureReservationRequest struct {
	ExposureReservationRequest
	MonthlyAllowance  int64
	InboundDailyLimit int64
	PeriodStart       time.Time
	PeriodEnd         time.Time
	UTCDate           time.Time
	AccountingEnabled bool
}

type ExposureReservation struct {
	ID                 string
	RequestedResources int
	ReservedResources  int
	ReservedUnits      int64
	ActualUnits        int64
	Status             string
	Duplicate          bool
	Bypassed           bool
}

type ExposureReleaseReconcileStats struct {
	Scanned             int
	Released            int
	Finalized           int
	NeedsReconciliation int
	Deferred            int
}

type InboundNotification struct {
	WorkspaceID       string    `json:"workspace_id"`
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

type InboundMutation func(context.Context, pgx.Tx) error

type AtomicInboundStore interface {
	AdmitInboundWithMutation(context.Context, StoreInboundRequest, InboundMutation) (InboundAdmission, error)
	RunInboundMutation(context.Context, InboundMutation) error
}

type ExposureStore interface {
	ReserveExposure(context.Context, StoreExposureReservationRequest) (ExposureReservation, error)
	MarkExposureReadStarted(context.Context, string) error
	MarkExposureFinalizePending(context.Context, string, int64, string) error
	FinalizeExposure(context.Context, string, int64) error
	ReleaseExposure(context.Context, string) error
	MarkExposureReleasePending(context.Context, string, string) error
	MarkExposureNeedsReconciliation(context.Context, string, string) error
	ReconcilePendingExposures(context.Context, int, time.Time) (ExposureReleaseReconcileStats, error)
}

type Service struct {
	store      Store
	inbound    InboundStore
	appBaseURL string
}

func NewService(store Store) *Service {
	service := &Service{
		store:      store,
		appBaseURL: "https://app.unipost.dev",
	}
	if inbound, ok := store.(InboundStore); ok {
		service.inbound = inbound
	}
	return service
}

func (s *Service) SetAppBaseURL(appBaseURL string) *Service {
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

func (s *Service) ReverseByIdempotencyKey(
	ctx context.Context,
	workspaceID string,
	idempotencyKey string,
) error {
	store, ok := s.store.(interface {
		ReverseByIdempotencyKey(context.Context, string, string) error
	})
	if !ok {
		return errors.New("X usage recovery store is not configured")
	}
	if workspaceID == "" || idempotencyKey == "" {
		return errors.New("workspace_id and idempotency_key are required")
	}
	return store.ReverseByIdempotencyKey(ctx, workspaceID, idempotencyKey)
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

func (s *Service) ReserveExposure(
	ctx context.Context,
	req ExposureReservationRequest,
) (ExposureReservation, error) {
	return s.reserveExposure(ctx, req, true)
}

func (s *Service) reserveExposure(
	ctx context.Context,
	req ExposureReservationRequest,
	accountingEnabled bool,
) (ExposureReservation, error) {
	mode, err := xinbox.NormalizePersistedAppMode(req.AppMode)
	if err != nil {
		return ExposureReservation{}, err
	}
	if mode != xinbox.AppModeUniPostManaged {
		return ExposureReservation{
			RequestedResources: req.RequestedResources,
			ReservedResources:  req.RequestedResources,
			Status:             "bypassed",
			Bypassed:           true,
		}, nil
	}
	store, ok := s.store.(ExposureStore)
	if !ok {
		return ExposureReservation{}, errors.New("X exposure reservation store is not configured")
	}
	if req.Now.IsZero() {
		req.Now = time.Now().UTC()
	}
	period, err := s.store.ResolveWorkspacePeriod(ctx, req.WorkspaceID, req.Now)
	if err != nil {
		return ExposureReservation{}, err
	}
	allowance, configured := PlanAllowance(period.PlanID)
	if period.MonthlyAllowance != nil {
		allowance, configured = *period.MonthlyAllowance, true
	}
	if !configured {
		return ExposureReservation{}, ErrAllowanceNotConfigured
	}
	dailyLimit, dailyConfigured := InboundDailyLimit(period.PlanID)
	if period.InboundDailyLimit != nil {
		dailyLimit, dailyConfigured = *period.InboundDailyLimit, true
	}
	if !dailyConfigured {
		return ExposureReservation{}, ErrInboundDailyCapExceeded
	}
	utc := req.Now.UTC()
	return store.ReserveExposure(ctx, StoreExposureReservationRequest{
		ExposureReservationRequest: req,
		MonthlyAllowance:           allowance,
		InboundDailyLimit:          dailyLimit,
		PeriodStart:                period.Start,
		PeriodEnd:                  period.End,
		UTCDate:                    time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC),
		AccountingEnabled:          accountingEnabled,
	})
}

func (s *Service) FinalizeExposure(ctx context.Context, id string, actualUnits int64) error {
	store, ok := s.store.(ExposureStore)
	if !ok {
		return errors.New("X exposure reservation store is not configured")
	}
	return store.FinalizeExposure(ctx, id, actualUnits)
}

func (s *Service) MarkExposureReadStarted(ctx context.Context, id string) error {
	store, ok := s.store.(ExposureStore)
	if !ok {
		return errors.New("X exposure reservation store is not configured")
	}
	return store.MarkExposureReadStarted(ctx, id)
}

func (s *Service) MarkExposureFinalizePending(
	ctx context.Context,
	id string,
	actualUnits int64,
	message string,
) error {
	store, ok := s.store.(ExposureStore)
	if !ok {
		return errors.New("X exposure reservation store is not configured")
	}
	return store.MarkExposureFinalizePending(ctx, id, actualUnits, message)
}

func (s *Service) ReleaseExposure(ctx context.Context, id string) error {
	store, ok := s.store.(ExposureStore)
	if !ok {
		return errors.New("X exposure reservation store is not configured")
	}
	return store.ReleaseExposure(ctx, id)
}

func (s *Service) MarkExposureReleasePending(ctx context.Context, id, message string) error {
	store, ok := s.store.(ExposureStore)
	if !ok {
		return errors.New("X exposure reservation store is not configured")
	}
	return store.MarkExposureReleasePending(ctx, id, message)
}

func (s *Service) MarkExposureNeedsReconciliation(ctx context.Context, id, message string) error {
	store, ok := s.store.(ExposureStore)
	if !ok {
		return errors.New("X exposure reservation store is not configured")
	}
	return store.MarkExposureNeedsReconciliation(ctx, id, message)
}

func (s *Service) ReconcilePendingExposures(
	ctx context.Context,
	limit int,
	now time.Time,
) (ExposureReleaseReconcileStats, error) {
	store, ok := s.store.(ExposureStore)
	if !ok {
		return ExposureReleaseReconcileStats{}, errors.New("X exposure reservation store is not configured")
	}
	if limit <= 0 {
		limit = 100
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return store.ReconcilePendingExposures(ctx, limit, now)
}

func (s *Service) AdmitInbound(ctx context.Context, req InboundRequest) (InboundAdmission, error) {
	return s.admitInbound(ctx, req, nil, true)
}

func (s *Service) AdmitInboundWithMutation(
	ctx context.Context,
	req InboundRequest,
	mutation InboundMutation,
) (InboundAdmission, error) {
	if mutation == nil {
		return s.AdmitInbound(ctx, req)
	}
	return s.admitInbound(ctx, req, mutation, true)
}

func (s *Service) admitInbound(
	ctx context.Context,
	req InboundRequest,
	mutation InboundMutation,
	accountingEnabled bool,
) (InboundAdmission, error) {
	appMode, err := xinbox.NormalizePersistedAppMode(req.AppMode)
	if err != nil {
		return InboundAdmission{}, err
	}
	if appMode != xinbox.AppModeUniPostManaged {
		if mutation != nil {
			if s == nil {
				return InboundAdmission{}, errors.New("x inbound atomic store is not configured")
			}
			atomicStore, ok := s.inbound.(AtomicInboundStore)
			if !ok {
				return InboundAdmission{}, errors.New("x inbound atomic store is not configured")
			}
			if err := atomicStore.RunInboundMutation(ctx, mutation); err != nil {
				return InboundAdmission{}, err
			}
		}
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

	storeReq := StoreInboundRequest{
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
		CapManagementURL:     s.appBaseURL + "/settings/billing#x-inbound-cap",
		AccountingEnabled:    accountingEnabled,
	}
	var admission InboundAdmission
	if mutation != nil {
		atomicStore, ok := s.inbound.(AtomicInboundStore)
		if !ok {
			return InboundAdmission{}, errors.New("x inbound atomic store is not configured")
		}
		admission, err = atomicStore.AdmitInboundWithMutation(ctx, storeReq, mutation)
	} else {
		admission, err = s.inbound.AdmitInbound(ctx, storeReq)
	}
	if err != nil {
		return InboundAdmission{}, err
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
	defaultInboundLimit := int64(0)
	if snapshot.InboundDailyLimit != nil {
		defaultInboundLimit = *snapshot.InboundDailyLimit
	}
	return s.inbound.UpdateInboundCap(ctx, StoreUpdateInboundCapRequest{
		UpdateInboundCapRequest:  req,
		MonthlyAllowance:         *snapshot.MonthlyAllowance,
		DefaultInboundDailyLimit: defaultInboundLimit,
		PeriodStart:              snapshot.PeriodStart,
		PeriodEnd:                snapshot.PeriodEnd,
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

func validateInboundCapIncrease(currentLimit, requestedLimit int64, acknowledged bool) error {
	if requestedLimit > currentLimit && !acknowledged {
		return ErrInboundExposureAcknowledgementRequired
	}
	return nil
}

func CalendarMonthPeriod(now time.Time) (time.Time, time.Time) {
	utc := now.UTC()
	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0)
}
