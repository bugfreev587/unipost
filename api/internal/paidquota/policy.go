package paidquota

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

type PeriodDelta struct {
	Period         string
	ReleasedUnits  int
	RequestedUnits int
}

type Decision struct {
	Allowed        bool
	ProjectedUsage int
}

type AdmissionError struct {
	Snapshot       quota.MonthlySnapshot
	RequestedUnits int
}

type Mutation func(*db.Queries) error

type Coordinator interface {
	Mutate(ctx context.Context, workspaceID string, deltas []PeriodDelta, mutation Mutation) error
}

type transaction interface {
	LockPeriod(ctx context.Context, workspaceID, period string) error
	Snapshot(ctx context.Context, workspaceID, period string) (quota.MonthlySnapshot, error)
	Queries() *db.Queries
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type transactionBeginner interface {
	Begin(ctx context.Context) (transaction, error)
}

type coordinator struct {
	beginner transactionBeginner
}

func newCoordinator(beginner transactionBeginner) Coordinator {
	return &coordinator{beginner: beginner}
}

func (c *coordinator) Mutate(ctx context.Context, workspaceID string, deltas []PeriodDelta, mutation Mutation) error {
	if c == nil || c.beginner == nil {
		return errors.New("paid quota coordinator is not configured")
	}
	tx, err := c.beginner.Begin(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	normalized := normalizePeriodDeltas(deltas)
	for _, delta := range normalized {
		if err := tx.LockPeriod(ctx, workspaceID, delta.Period); err != nil {
			return err
		}
	}
	for _, delta := range normalized {
		snapshot, err := tx.Snapshot(ctx, workspaceID, delta.Period)
		if err != nil {
			return err
		}
		decision := Decide(snapshot, delta.ReleasedUnits, delta.RequestedUnits)
		if !decision.Allowed {
			return NewAdmissionError(snapshot, delta.RequestedUnits)
		}
	}
	if mutation != nil {
		if err := mutation(tx.Queries()); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}

func NewAdmissionError(snapshot quota.MonthlySnapshot, requestedUnits int) *AdmissionError {
	return &AdmissionError{
		Snapshot:       snapshot,
		RequestedUnits: requestedUnits,
	}
}

func (e *AdmissionError) Error() string {
	if e == nil {
		return "paid schedule quota admission rejected"
	}
	return fmt.Sprintf(
		"paid schedule quota admission rejected for workspace %s period %s: effective=%d requested=%d limit=%d",
		e.Snapshot.WorkspaceID,
		e.Snapshot.Period,
		e.Snapshot.EffectiveUsage(),
		e.RequestedUnits,
		e.Snapshot.Limit,
	)
}

func AppliesToPlan(planID string) bool {
	switch strings.ToLower(strings.TrimSpace(planID)) {
	case "api", "basic", "growth":
		return true
	default:
		return false
	}
}

func Decide(snapshot quota.MonthlySnapshot, released, requested int) Decision {
	if released < 0 {
		released = 0
	}
	if requested < 0 {
		requested = 0
	}
	projected := snapshot.EffectiveUsage() - released + requested
	if !AppliesToPlan(snapshot.PlanID) || snapshot.Limit < 0 {
		return Decision{Allowed: true, ProjectedUsage: projected}
	}
	return Decision{
		Allowed:        projected <= snapshot.Limit,
		ProjectedUsage: projected,
	}
}

func normalizePeriodDeltas(deltas []PeriodDelta) []PeriodDelta {
	byPeriod := make(map[string]PeriodDelta, len(deltas))
	for _, delta := range deltas {
		period := strings.TrimSpace(delta.Period)
		if period == "" {
			continue
		}
		current := byPeriod[period]
		current.Period = period
		if delta.ReleasedUnits > 0 {
			current.ReleasedUnits += delta.ReleasedUnits
		}
		if delta.RequestedUnits > 0 {
			current.RequestedUnits += delta.RequestedUnits
		}
		byPeriod[period] = current
	}

	out := make([]PeriodDelta, 0, len(byPeriod))
	for _, delta := range byPeriod {
		out = append(out, delta)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Period < out[j].Period
	})
	return out
}
