package quotaemail

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type PostgresStore struct {
	queries *db.Queries
}

func NewPostgresStore(queries *db.Queries) *PostgresStore {
	return &PostgresStore{queries: queries}
}

func (s *PostgresStore) Snapshot(ctx context.Context, workspaceID, period string) (Snapshot, error) {
	if s == nil || s.queries == nil {
		return Snapshot{}, errors.New("quotaemail: store is not configured")
	}
	if period == "" {
		period = time.Now().UTC().Format("2006-01")
	}
	workspace, err := s.queries.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return Snapshot{}, err
	}
	owner, err := s.queries.GetUser(ctx, workspace.UserID)
	if err != nil {
		return Snapshot{}, err
	}
	planID := "free"
	if sub, err := s.queries.GetSubscriptionByWorkspace(ctx, workspaceID); err == nil && sub.PlanID != "" {
		planID = sub.PlanID
	}
	limit := int32(100)
	if plan, err := s.queries.GetPlan(ctx, planID); err == nil {
		limit = plan.PostLimit
	}
	usage := int32(0)
	if row, err := s.queries.GetUsage(ctx, db.GetUsageParams{WorkspaceID: workspaceID, Period: period}); err == nil {
		usage = row.PostCount
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Snapshot{}, err
	}
	reserved := int32(0)
	if planID == "free" {
		if count, err := s.queries.CountScheduledQuotaUnitsByWorkspaceAndPeriod(ctx, db.CountScheduledQuotaUnitsByWorkspaceAndPeriodParams{
			WorkspaceID: workspaceID,
			Period:      period,
		}); err == nil {
			reserved = count
		}
	}
	return Snapshot{
		WorkspaceID:   workspace.ID,
		WorkspaceName: workspace.Name,
		UserID:        owner.ID,
		OwnerEmail:    owner.Email,
		OwnerName:     pgText(owner.Name),
		PlanID:        planID,
		Period:        period,
		Usage:         int(usage),
		Reserved:      int(reserved),
		Limit:         int(limit),
	}, nil
}

func (s *PostgresStore) AttemptedThresholds(ctx context.Context, workspaceID, period string) (map[int]bool, error) {
	rows, err := s.queries.ListFreePlanQuotaReminderAttemptedThresholds(ctx, db.ListFreePlanQuotaReminderAttemptedThresholdsParams{
		WorkspaceID: workspaceID,
		Period:      period,
	})
	if err != nil {
		return nil, err
	}
	out := make(map[int]bool, len(rows))
	for _, threshold := range rows {
		out[int(threshold)] = true
	}
	return out, nil
}

func (s *PostgresStore) CreatePending(ctx context.Context, reminder Reminder) (Reminder, bool, error) {
	row, err := s.queries.CreatePendingFreePlanQuotaReminder(ctx, db.CreatePendingFreePlanQuotaReminderParams{
		WorkspaceID:      reminder.WorkspaceID,
		UserID:           reminder.UserID,
		Email:            reminder.Email,
		Period:           reminder.Period,
		ThresholdPercent: int32(reminder.ThresholdPercent),
		TransactionalID:  reminder.TransactionalID,
		IdempotencyKey:   reminder.IdempotencyKey,
		EffectiveUsage:   int32(reminder.EffectiveUsage),
		CompletedUsage:   int32(reminder.CompletedUsage),
		ReservedUsage:    int32(reminder.ReservedUsage),
		PostLimit:        int32(reminder.PostLimit),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Reminder{}, false, nil
	}
	if err != nil {
		return Reminder{}, false, err
	}
	return reminderFromRow(row), true, nil
}

func (s *PostgresStore) MarkSent(ctx context.Context, id string) error {
	return s.queries.MarkFreePlanQuotaReminderSent(ctx, id)
}

func (s *PostgresStore) MarkFailed(ctx context.Context, id, reason string) error {
	return s.queries.MarkFreePlanQuotaReminderFailed(ctx, db.MarkFreePlanQuotaReminderFailedParams{
		ID:            id,
		FailureReason: pgtype.Text{String: reason, Valid: reason != ""},
	})
}

func reminderFromRow(row db.FreePlanQuotaEmailReminder) Reminder {
	return Reminder{
		ID:               row.ID,
		WorkspaceID:      row.WorkspaceID,
		UserID:           row.UserID,
		Email:            row.Email,
		Period:           row.Period,
		ThresholdPercent: int(row.ThresholdPercent),
		Status:           row.Status,
		TransactionalID:  row.TransactionalID,
		IdempotencyKey:   row.IdempotencyKey,
		EffectiveUsage:   int(row.EffectiveUsage),
		CompletedUsage:   int(row.CompletedUsage),
		ReservedUsage:    int(row.ReservedUsage),
		PostLimit:        int(row.PostLimit),
		CreatedAt:        row.CreatedAt.Time,
	}
}

func pgText(value pgtype.Text) string {
	if value.Valid {
		return value.String
	}
	return ""
}
