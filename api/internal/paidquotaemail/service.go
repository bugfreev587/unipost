package paidquotaemail

import (
	"context"
	"fmt"
	"strings"

	"github.com/xiaoboyu/unipost-api/internal/emailpolicy"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

const (
	PaidWarningEventKey = "email.quota.paid_plan_warning.v1"
	PaidAlertEventKey   = "email.quota.paid_plan_alert.v1"
)

var thresholds = []int{80, 90, 100, 105, 110, 115, 120}

type Snapshot struct {
	quota.MonthlySnapshot
	WorkspaceName string
	UserID        string
	OwnerEmail    string
	OwnerName     string
	PlanName      string
}

type Decision struct {
	WorkspaceID      string
	UserID           string
	Email            string
	PlanID           string
	Period           string
	ThresholdPercent int
	Severity         string
	EventKey         string
	Status           string
	TransactionalID  string
	IdempotencyKey   string
	CompletedUsage   int
	ScheduledUsage   int
	QuotaHoldUsage   int
	EffectiveUsage   int
	PostLimit        int
}

type Store interface {
	Snapshot(ctx context.Context, workspaceID, period string) (Snapshot, error)
	Decisions(ctx context.Context, workspaceID, period string) (map[int]string, error)
	CreateDecision(ctx context.Context, decision Decision) (bool, error)
	MarkLowerPendingSuperseded(ctx context.Context, workspaceID, period string, threshold int) error
	EnsureFollowUp(ctx context.Context, snapshot Snapshot, decision Decision) error
	ResolveFollowUpsBelowLimit(ctx context.Context, workspaceID, period string) error
}

type Policy interface {
	Prepare(ctx context.Context, request emailpolicy.Request) (emailpolicy.Decision, error)
}

type Service struct {
	store           Store
	policy          Policy
	transactionalID string
}

func NewService(store Store, policy Policy, transactionalID string) *Service {
	return &Service{
		store:           store,
		policy:          policy,
		transactionalID: strings.TrimSpace(transactionalID),
	}
}

func (s *Service) Evaluate(ctx context.Context, workspaceID, period string) error {
	if s == nil || s.store == nil || strings.TrimSpace(workspaceID) == "" {
		return nil
	}
	snapshot, err := s.store.Snapshot(ctx, workspaceID, period)
	if err != nil {
		return err
	}
	if !eligible(snapshot.MonthlySnapshot) {
		return nil
	}

	existing, err := s.store.Decisions(ctx, snapshot.WorkspaceID, snapshot.Period)
	if err != nil {
		return err
	}
	reached := reachedThresholds(snapshot.MonthlySnapshot)
	highestMissing := 0
	for _, threshold := range reached {
		if _, decided := existing[threshold]; !decided {
			highestMissing = threshold
		}
	}

	var selected Decision
	if highestMissing > 0 {
		if err := s.store.MarkLowerPendingSuperseded(ctx, snapshot.WorkspaceID, snapshot.Period, highestMissing); err != nil {
			return err
		}
	}
	for _, threshold := range reached {
		if threshold >= highestMissing || highestMissing == 0 {
			continue
		}
		if _, decided := existing[threshold]; decided {
			continue
		}
		decision := decisionFor(snapshot, threshold, "skipped_superseded", s.transactionalID)
		if _, err := s.store.CreateDecision(ctx, decision); err != nil {
			return err
		}
	}
	if highestMissing > 0 {
		selected = decisionFor(snapshot, highestMissing, "pending", s.transactionalID)
		switch {
		case strings.TrimSpace(snapshot.OwnerEmail) == "":
			selected.Status = "skipped_missing_recipient"
		case highestMissing < 100 && s.policy != nil:
			policyDecision, err := s.policy.Prepare(ctx, emailpolicy.Request{
				EventKey: selected.EventKey,
				UserID:   snapshot.UserID,
				Email:    snapshot.OwnerEmail,
			})
			if err != nil {
				return err
			}
			if !policyDecision.ShouldSend && policyDecision.SkipReason == emailpolicy.SkipReasonPreferenceDisabled {
				selected.Status = "skipped_preference_disabled"
			}
		case highestMissing >= 100 && s.policy != nil:
			if _, err := s.policy.Prepare(ctx, emailpolicy.Request{
				EventKey: selected.EventKey,
				UserID:   snapshot.UserID,
				Email:    snapshot.OwnerEmail,
			}); err != nil {
				return err
			}
		}
		if _, err := s.store.CreateDecision(ctx, selected); err != nil {
			return err
		}
	}

	if snapshot.Reached(120) {
		if selected.ThresholdPercent != 120 {
			selected = decisionFor(snapshot, 120, "pending", s.transactionalID)
		}
		if err := s.store.EnsureFollowUp(ctx, snapshot, selected); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ResolveFollowUpsBelowLimit(ctx context.Context, workspaceID, period string) error {
	if s == nil || s.store == nil || strings.TrimSpace(workspaceID) == "" {
		return nil
	}
	snapshot, err := s.store.Snapshot(ctx, workspaceID, period)
	if err != nil {
		return err
	}
	if snapshot.Limit >= 0 && snapshot.EffectiveUsage() >= snapshot.Limit {
		return nil
	}
	return s.store.ResolveFollowUpsBelowLimit(ctx, snapshot.WorkspaceID, snapshot.Period)
}

func eligible(snapshot quota.MonthlySnapshot) bool {
	if snapshot.Limit <= 0 {
		return false
	}
	switch snapshot.PlanID {
	case "api", "basic", "growth":
		return true
	default:
		return false
	}
}

func reachedThresholds(snapshot quota.MonthlySnapshot) []int {
	out := make([]int, 0, len(thresholds))
	for _, threshold := range thresholds {
		if snapshot.Reached(threshold) {
			out = append(out, threshold)
		}
	}
	return out
}

func decisionFor(snapshot Snapshot, threshold int, status, transactionalID string) Decision {
	eventKey := PaidWarningEventKey
	severity := "warning"
	if threshold >= 100 {
		eventKey = PaidAlertEventKey
		severity = "alert"
	}
	if threshold >= 120 {
		severity = "critical_alert"
	}
	return Decision{
		WorkspaceID:      snapshot.WorkspaceID,
		UserID:           snapshot.UserID,
		Email:            snapshot.OwnerEmail,
		PlanID:           snapshot.PlanID,
		Period:           snapshot.Period,
		ThresholdPercent: threshold,
		Severity:         severity,
		EventKey:         eventKey,
		Status:           status,
		TransactionalID:  transactionalID,
		IdempotencyKey: fmt.Sprintf(
			"paid_plan_quota:%s:%s:%d",
			snapshot.WorkspaceID,
			snapshot.Period,
			threshold,
		),
		CompletedUsage: snapshot.Completed,
		ScheduledUsage: snapshot.Scheduled,
		QuotaHoldUsage: snapshot.QuotaHold,
		EffectiveUsage: snapshot.EffectiveUsage(),
		PostLimit:      snapshot.Limit,
	}
}
