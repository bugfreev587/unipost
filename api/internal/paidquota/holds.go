package paidquota

import (
	"context"
	"sort"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

type ScheduledParent struct {
	ID          string
	Status      string
	ScheduledAt time.Time
	CreatedAt   time.Time
	Units       int
}

type HoldDecision struct {
	PostID string
	Status string
}

type HoldReconciler interface {
	ReconcileWorkspace(ctx context.Context, workspaceID, reason string, downgradeEffectiveAt time.Time) error
	ReconcileWorkspaceForPlan(ctx context.Context, workspaceID, planID string, limit int, reason string, downgradeEffectiveAt time.Time) error
	ApplyPlanChange(ctx context.Context, workspaceID, planID string, limit int, reason string, downgradeEffectiveAt time.Time, mutation PlanChangeMutation) error
}

type PlanChangeMutation func(*db.Queries) error

type HoldPeriodTransaction interface {
	Snapshot(ctx context.Context) (quota.MonthlySnapshot, error)
	Parents(ctx context.Context) ([]ScheduledParent, error)
	SetHold(ctx context.Context, postID, reason string) error
	ReleaseHold(ctx context.Context, postID string) error
}

type HoldStore interface {
	WithinPeriod(ctx context.Context, workspaceID, period string, fn func(HoldPeriodTransaction) error) error
}

type HoldService struct {
	store HoldStore
	now   func() time.Time
}

func NewHoldService(store HoldStore, now func() time.Time) *HoldService {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &HoldService{store: store, now: now}
}

func (s *HoldService) ReconcileWorkspace(ctx context.Context, workspaceID, reason string, downgradeEffectiveAt time.Time) error {
	return s.reconcileWorkspace(ctx, workspaceID, reason, downgradeEffectiveAt, nil)
}

func (s *HoldService) ReconcileWorkspaceForPlan(
	ctx context.Context,
	workspaceID string,
	planID string,
	limit int,
	reason string,
	downgradeEffectiveAt time.Time,
) error {
	override := &quota.MonthlySnapshot{PlanID: planID, Limit: limit}
	return s.reconcileWorkspace(ctx, workspaceID, reason, downgradeEffectiveAt, override)
}

func (s *HoldService) reconcileWorkspace(
	ctx context.Context,
	workspaceID string,
	reason string,
	downgradeEffectiveAt time.Time,
	override *quota.MonthlySnapshot,
) error {
	now := s.now().UTC()
	end := now.AddDate(0, 0, 90)
	for cursor := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC); !cursor.After(end); cursor = cursor.AddDate(0, 1, 0) {
		if err := s.reconcilePeriod(ctx, workspaceID, cursor.Format("2006-01"), reason, downgradeEffectiveAt, override); err != nil {
			return err
		}
	}
	return nil
}

func (s *HoldService) ReconcilePeriod(ctx context.Context, workspaceID, period, reason string, downgradeEffectiveAt time.Time) error {
	return s.reconcilePeriod(ctx, workspaceID, period, reason, downgradeEffectiveAt, nil)
}

func (s *HoldService) reconcilePeriod(
	ctx context.Context,
	workspaceID string,
	period string,
	reason string,
	downgradeEffectiveAt time.Time,
	override *quota.MonthlySnapshot,
) error {
	if s == nil || s.store == nil {
		return nil
	}
	now := s.now().UTC()
	return s.store.WithinPeriod(ctx, workspaceID, period, func(tx HoldPeriodTransaction) error {
		return s.reconcilePeriodTransaction(ctx, tx, reason, downgradeEffectiveAt, now, override)
	})
}

func (s *HoldService) reconcilePeriodTransaction(
	ctx context.Context,
	tx HoldPeriodTransaction,
	reason string,
	downgradeEffectiveAt time.Time,
	now time.Time,
	override *quota.MonthlySnapshot,
) error {
	snapshot, err := tx.Snapshot(ctx)
	if err != nil {
		return err
	}
	if override != nil {
		snapshot.PlanID = override.PlanID
		snapshot.Limit = override.Limit
	}
	parents, err := tx.Parents(ctx)
	if err != nil {
		return err
	}
	limit := snapshot.Limit
	if limit < 0 {
		limit = int(^uint(0) >> 2)
	}
	decisions := AllocateQuotaHolds(snapshot.Completed, limit, downgradeEffectiveAt, now, parents)
	currentStatus := make(map[string]string, len(parents))
	for _, parent := range parents {
		currentStatus[parent.ID] = parent.Status
	}
	for _, decision := range decisions {
		switch {
		case decision.Status == "quota_hold" && currentStatus[decision.PostID] == "scheduled":
			if err := tx.SetHold(ctx, decision.PostID, reason); err != nil {
				return err
			}
		case decision.Status == "scheduled" && currentStatus[decision.PostID] == "quota_hold":
			if err := tx.ReleaseHold(ctx, decision.PostID); err != nil {
				return err
			}
		}
	}
	return nil
}

func AllocateQuotaHolds(completed, limit int, downgradeEffectiveAt, now time.Time, parents []ScheduledParent) []HoldDecision {
	ordered := append([]ScheduledParent(nil), parents...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if !ordered[i].ScheduledAt.Equal(ordered[j].ScheduledAt) {
			return ordered[i].ScheduledAt.Before(ordered[j].ScheduledAt)
		}
		if !ordered[i].CreatedAt.Equal(ordered[j].CreatedAt) {
			return ordered[i].CreatedAt.Before(ordered[j].CreatedAt)
		}
		return ordered[i].ID < ordered[j].ID
	})

	remaining := limit - completed
	for _, parent := range ordered {
		if parent.Status == "publishing" ||
			(parent.Status == "quota_hold" && !parent.ScheduledAt.After(now)) ||
			(parent.Status == "scheduled" &&
				!downgradeEffectiveAt.IsZero() &&
				parent.ScheduledAt.Before(downgradeEffectiveAt)) {
			remaining -= max(parent.Units, 0)
		}
	}

	decisions := make([]HoldDecision, 0, len(ordered))
	for _, parent := range ordered {
		units := max(parent.Units, 0)
		switch {
		case parent.Status == "publishing":
			decisions = append(decisions, HoldDecision{PostID: parent.ID, Status: "publishing"})
		case parent.Status == "scheduled" &&
			!downgradeEffectiveAt.IsZero() &&
			parent.ScheduledAt.Before(downgradeEffectiveAt):
			decisions = append(decisions, HoldDecision{PostID: parent.ID, Status: "scheduled"})
		case parent.Status == "quota_hold" && !parent.ScheduledAt.After(now):
			decisions = append(decisions, HoldDecision{PostID: parent.ID, Status: "quota_hold"})
		case units <= max(remaining, 0):
			remaining -= units
			decisions = append(decisions, HoldDecision{PostID: parent.ID, Status: "scheduled"})
		default:
			decisions = append(decisions, HoldDecision{PostID: parent.ID, Status: "quota_hold"})
		}
	}
	return decisions
}
