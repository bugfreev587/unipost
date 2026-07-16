package xcredits

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/xinbox"
)

const (
	UsageStatusProvisional = "provisional"
	UsageStatusFinalized   = "finalized"
	UsageStatusReversed    = "reversed"
	UsageStatusBypassed    = "bypassed"
)

var (
	ErrMonthlyLimitExceeded   = errors.New("x_monthly_usage_limit_exceeded: managed X usage has reached this billing period's allowance")
	ErrAllowanceNotConfigured = errors.New("x_monthly_usage_limit_not_configured: contact UniPost to configure the workspace X allowance")
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
	PlanID            string    `json:"plan_id"`
	PeriodStart       time.Time `json:"billing_period_start"`
	PeriodEnd         time.Time `json:"billing_period_end"`
	MonthlyAllowance  *int64    `json:"monthly_allowance"`
	MonthlyUsed       int64     `json:"monthly_used"`
	MonthlyRemaining  *int64    `json:"monthly_remaining"`
	InboundDailyUsed  int64     `json:"inbound_daily_usage"`
	InboundDailyLimit *int64    `json:"inbound_daily_limit"`
	CatalogVersion    string    `json:"catalog_version"`
}

type Store interface {
	ResolveWorkspacePeriod(context.Context, string, time.Time) (WorkspacePeriod, error)
	Reserve(context.Context, StoreReserveRequest) (UsageEvent, error)
	Finalize(context.Context, string, int64) error
	Reverse(context.Context, string) error
	Snapshot(context.Context, string, time.Time) (Snapshot, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Reserve(ctx context.Context, req ReserveRequest) (UsageEvent, error) {
	appMode, err := xinbox.ParseAppMode(req.AppMode)
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

func CalendarMonthPeriod(now time.Time) (time.Time, time.Time) {
	utc := now.UTC()
	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0)
}
