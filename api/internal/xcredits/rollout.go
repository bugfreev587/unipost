package xcredits

import (
	"context"
	"errors"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/featureflags"
	"github.com/xiaoboyu/unipost-api/internal/xinbox"
)

var ErrFeatureNotAvailable = errors.New("X Credits billing is not available")

type RolloutEvaluator interface {
	ForWorkspace(context.Context, string, string) (bool, error)
}

// RolloutService keeps every X usage call site on one service while separating
// customer accounting from internal cost-safety enforcement.
type RolloutService struct {
	base      *Service
	evaluator RolloutEvaluator
}

func NewRolloutService(base *Service, evaluator RolloutEvaluator) *RolloutService {
	return &RolloutService{base: base, evaluator: evaluator}
}

func (s *RolloutService) EnabledForWorkspace(ctx context.Context, workspaceID string) (bool, error) {
	if s == nil || s.evaluator == nil {
		return false, errors.New("X Credits rollout evaluator is not configured")
	}
	return s.evaluator.ForWorkspace(ctx, workspaceID, featureflags.XCreditsBillingV1)
}

func (s *RolloutService) Reserve(ctx context.Context, req ReserveRequest) (UsageEvent, error) {
	mode, err := xinbox.NormalizePersistedAppMode(req.AppMode)
	if err != nil || mode != xinbox.AppModeUniPostManaged {
		return s.base.Reserve(ctx, req)
	}
	enabled, err := s.EnabledForWorkspace(ctx, req.WorkspaceID)
	if err != nil {
		return UsageEvent{}, err
	}
	if !enabled {
		return UsageEvent{Status: UsageStatusBypassed}, nil
	}
	return s.base.Reserve(ctx, req)
}

func (s *RolloutService) Finalize(ctx context.Context, eventID string, finalUnits int64) error {
	return s.base.Finalize(ctx, eventID, finalUnits)
}

func (s *RolloutService) Reverse(ctx context.Context, eventID string) error {
	return s.base.Reverse(ctx, eventID)
}

func (s *RolloutService) ReverseByIdempotencyKey(ctx context.Context, workspaceID, key string) error {
	return s.base.ReverseByIdempotencyKey(ctx, workspaceID, key)
}

func (s *RolloutService) Snapshot(ctx context.Context, workspaceID string, now time.Time) (Snapshot, error) {
	snapshot, err := s.base.Snapshot(ctx, workspaceID, now)
	if err != nil {
		return Snapshot{}, err
	}
	enabled, err := s.EnabledForWorkspace(ctx, workspaceID)
	if err != nil {
		return Snapshot{}, err
	}
	if enabled {
		return snapshot, nil
	}
	snapshot.MonthlyAllowance = nil
	snapshot.MonthlyUsed = 0
	snapshot.MonthlyRemaining = nil
	snapshot.PausePaidSources = false
	snapshot.InboundPauseReason = ""
	if snapshot.InboundDailyLimit != nil {
		switch {
		case snapshot.InboundDailyUsed >= *snapshot.InboundDailyLimit:
			snapshot.PausePaidSources = true
			snapshot.InboundPauseReason = PauseReasonDailyCap
		case remainingWithinSafetyBuffer(snapshot.InboundDailyUsed, *snapshot.InboundDailyLimit):
			snapshot.PausePaidSources = true
			snapshot.InboundPauseReason = PauseReasonDailySafetyBuffer
		}
	}
	return snapshot, nil
}

func (s *RolloutService) AdmitInbound(ctx context.Context, req InboundRequest) (InboundAdmission, error) {
	return s.admitInbound(ctx, req, nil)
}

func (s *RolloutService) AdmitInboundWithMutation(
	ctx context.Context,
	req InboundRequest,
	mutation InboundMutation,
) (InboundAdmission, error) {
	return s.admitInbound(ctx, req, mutation)
}

func (s *RolloutService) admitInbound(
	ctx context.Context,
	req InboundRequest,
	mutation InboundMutation,
) (InboundAdmission, error) {
	mode, err := xinbox.NormalizePersistedAppMode(req.AppMode)
	if err != nil || mode != xinbox.AppModeUniPostManaged {
		return s.base.admitInbound(ctx, req, mutation, true)
	}
	enabled, err := s.EnabledForWorkspace(ctx, req.WorkspaceID)
	if err != nil {
		return InboundAdmission{}, err
	}
	return s.base.admitInbound(ctx, req, mutation, enabled)
}

func (s *RolloutService) ReserveExposure(
	ctx context.Context,
	req ExposureReservationRequest,
) (ExposureReservation, error) {
	mode, err := xinbox.NormalizePersistedAppMode(req.AppMode)
	if err != nil || mode != xinbox.AppModeUniPostManaged {
		return s.base.ReserveExposure(ctx, req)
	}
	enabled, err := s.EnabledForWorkspace(ctx, req.WorkspaceID)
	if err != nil {
		return ExposureReservation{}, err
	}
	return s.base.reserveExposure(ctx, req, enabled)
}

func (s *RolloutService) MarkExposureReadStarted(ctx context.Context, id string) error {
	return s.base.MarkExposureReadStarted(ctx, id)
}

func (s *RolloutService) MarkExposureFinalizePending(ctx context.Context, id string, units int64, message string) error {
	return s.base.MarkExposureFinalizePending(ctx, id, units, message)
}

func (s *RolloutService) FinalizeExposure(ctx context.Context, id string, units int64) error {
	return s.base.FinalizeExposure(ctx, id, units)
}

func (s *RolloutService) ReleaseExposure(ctx context.Context, id string) error {
	return s.base.ReleaseExposure(ctx, id)
}

func (s *RolloutService) MarkExposureReleasePending(ctx context.Context, id, message string) error {
	return s.base.MarkExposureReleasePending(ctx, id, message)
}

func (s *RolloutService) MarkExposureNeedsReconciliation(ctx context.Context, id, message string) error {
	return s.base.MarkExposureNeedsReconciliation(ctx, id, message)
}

func (s *RolloutService) ReconcilePendingExposures(
	ctx context.Context,
	limit int,
	now time.Time,
) (ExposureReleaseReconcileStats, error) {
	return s.base.ReconcilePendingExposures(ctx, limit, now)
}

func (s *RolloutService) UpdateInboundCap(
	ctx context.Context,
	req UpdateInboundCapRequest,
) (InboundCapSetting, error) {
	enabled, err := s.EnabledForWorkspace(ctx, req.WorkspaceID)
	if err != nil {
		return InboundCapSetting{}, err
	}
	if !enabled {
		return InboundCapSetting{}, ErrFeatureNotAvailable
	}
	return s.base.UpdateInboundCap(ctx, req)
}
