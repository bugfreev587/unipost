package paidquotaemail

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

type PostgresStore struct {
	queries *db.Queries
	checker *quota.Checker
}

func NewPostgresStore(queries *db.Queries, checker *quota.Checker) *PostgresStore {
	return &PostgresStore{queries: queries, checker: checker}
}

func (s *PostgresStore) Snapshot(ctx context.Context, workspaceID, period string) (Snapshot, error) {
	if period == "" {
		period = time.Now().UTC().Format("2006-01")
	}
	monthly, err := s.checker.MonthlySnapshotForPeriod(ctx, workspaceID, period)
	if err != nil {
		return Snapshot{}, err
	}
	workspace, err := s.queries.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return Snapshot{}, err
	}
	owner, err := s.queries.GetUser(ctx, workspace.UserID)
	if err != nil {
		return Snapshot{}, err
	}
	plan, err := s.queries.GetPlan(ctx, monthly.PlanID)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		MonthlySnapshot: monthly,
		WorkspaceName:   workspace.Name,
		UserID:          owner.ID,
		OwnerEmail:      owner.Email,
		OwnerName:       textValue(owner.Name),
		PlanName:        plan.Name,
	}, nil
}

func (s *PostgresStore) Decisions(ctx context.Context, workspaceID, period string) (map[int]string, error) {
	rows, err := s.queries.ListPaidPlanQuotaNotificationDecisions(ctx, db.ListPaidPlanQuotaNotificationDecisionsParams{
		WorkspaceID: workspaceID,
		Period:      period,
	})
	if err != nil {
		return nil, err
	}
	out := make(map[int]string, len(rows))
	for _, row := range rows {
		out[int(row.ThresholdPercent)] = row.Status
	}
	return out, nil
}

func (s *PostgresStore) CreateDecision(ctx context.Context, decision Decision) (bool, error) {
	_, err := s.queries.InsertPaidPlanQuotaNotificationDecision(ctx, db.InsertPaidPlanQuotaNotificationDecisionParams{
		WorkspaceID:      decision.WorkspaceID,
		UserID:           nullableText(decision.UserID),
		Email:            nullableText(decision.Email),
		PlanID:           decision.PlanID,
		Period:           decision.Period,
		ThresholdPercent: int32(decision.ThresholdPercent),
		Severity:         decision.Severity,
		EventKey:         decision.EventKey,
		Status:           decision.Status,
		TransactionalID:  nullableText(decision.TransactionalID),
		IdempotencyKey:   decision.IdempotencyKey,
		CompletedUsage:   int32(decision.CompletedUsage),
		ScheduledUsage:   int32(decision.ScheduledUsage),
		QuotaHoldUsage:   int32(decision.QuotaHoldUsage),
		EffectiveUsage:   int32(decision.EffectiveUsage),
		PostLimit:        int32(decision.PostLimit),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *PostgresStore) MarkLowerPendingSuperseded(ctx context.Context, workspaceID, period string, threshold int) error {
	return s.queries.MarkLowerPaidPlanQuotaNotificationsSuperseded(ctx, db.MarkLowerPaidPlanQuotaNotificationsSupersededParams{
		WorkspaceID:      workspaceID,
		Period:           period,
		ThresholdPercent: int32(threshold),
	})
}

func (s *PostgresStore) EnsureFollowUp(ctx context.Context, snapshot Snapshot, decision Decision) error {
	_, err := s.queries.InsertPaidQuotaFollowUp(ctx, db.InsertPaidQuotaFollowUpParams{
		WorkspaceID:    snapshot.WorkspaceID,
		OwnerUserID:    nullableText(snapshot.UserID),
		PlanID:         snapshot.PlanID,
		Period:         snapshot.Period,
		NotificationID: pgtype.Text{},
		CompletedUsage: int32(snapshot.Completed),
		ScheduledUsage: int32(snapshot.Scheduled),
		QuotaHoldUsage: int32(snapshot.QuotaHold),
		EffectiveUsage: int32(snapshot.EffectiveUsage()),
		PostLimit:      int32(snapshot.Limit),
	})
	return err
}

func (s *PostgresStore) ResolveFollowUpsBelowLimit(ctx context.Context, workspaceID, period string) error {
	return s.queries.ResolvePaidQuotaFollowUpsBelowLimit(ctx, db.ResolvePaidQuotaFollowUpsBelowLimitParams{
		WorkspaceID: workspaceID,
		Period:      period,
	})
}

func nullableText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func textValue(value pgtype.Text) string {
	if value.Valid {
		return value.String
	}
	return ""
}
