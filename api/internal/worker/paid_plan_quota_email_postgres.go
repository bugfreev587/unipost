package worker

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type PostgresPaidQuotaDeliveryStore struct {
	queries *db.Queries
}

func NewPostgresPaidQuotaDeliveryStore(queries *db.Queries) *PostgresPaidQuotaDeliveryStore {
	return &PostgresPaidQuotaDeliveryStore{queries: queries}
}

func (s *PostgresPaidQuotaDeliveryStore) Claim(ctx context.Context, limit int) ([]PaidQuotaDelivery, error) {
	rows, err := s.queries.ClaimPaidPlanQuotaNotifications(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]PaidQuotaDelivery, 0, len(rows))
	for _, row := range rows {
		workspace, err := s.queries.GetWorkspace(ctx, row.WorkspaceID)
		if err != nil {
			return nil, err
		}
		plan, err := s.queries.GetPlan(ctx, row.PlanID)
		if err != nil {
			return nil, err
		}
		ownerName := ""
		if row.UserID.Valid {
			if owner, err := s.queries.GetUser(ctx, row.UserID.String); err == nil && owner.Name.Valid {
				ownerName = owner.Name.String
			}
		}
		out = append(out, PaidQuotaDelivery{
			ID:               row.ID,
			WorkspaceID:      row.WorkspaceID,
			WorkspaceName:    workspace.Name,
			UserID:           textOrEmpty(row.UserID),
			OwnerEmail:       textOrEmpty(row.Email),
			OwnerName:        ownerName,
			PlanID:           row.PlanID,
			PlanName:         plan.Name,
			Period:           row.Period,
			ThresholdPercent: int(row.ThresholdPercent),
			Severity:         row.Severity,
			EventKey:         row.EventKey,
			TransactionalID:  textOrEmpty(row.TransactionalID),
			IdempotencyKey:   row.IdempotencyKey,
			CompletedUsage:   int(row.CompletedUsage),
			ScheduledUsage:   int(row.ScheduledUsage),
			QuotaHoldUsage:   int(row.QuotaHoldUsage),
			EffectiveUsage:   int(row.EffectiveUsage),
			PostLimit:        int(row.PostLimit),
			AttemptCount:     int(row.AttemptCount),
		})
	}
	return out, nil
}

func (s *PostgresPaidQuotaDeliveryStore) MarkSent(ctx context.Context, id string) error {
	return s.queries.MarkPaidPlanQuotaNotificationSent(ctx, id)
}

func (s *PostgresPaidQuotaDeliveryStore) MarkRetry(ctx context.Context, retry PaidQuotaRetry) error {
	return s.queries.MarkPaidPlanQuotaNotificationRetryWait(ctx, db.MarkPaidPlanQuotaNotificationRetryWaitParams{
		NextAttemptAt: pgtype.Timestamptz{Time: retry.NextAttemptAt, Valid: true},
		LastError:     pgtype.Text{String: retry.LastError, Valid: retry.LastError != ""},
		ID:            retry.ID,
	})
}

func (s *PostgresPaidQuotaDeliveryStore) MarkFailed(ctx context.Context, id, lastError string) error {
	return s.queries.MarkPaidPlanQuotaNotificationFailed(ctx, db.MarkPaidPlanQuotaNotificationFailedParams{
		LastError: pgtype.Text{String: lastError, Valid: lastError != ""},
		ID:        id,
	})
}

func (s *PostgresPaidQuotaDeliveryStore) MarkPreferenceDisabled(ctx context.Context, id string) error {
	return s.queries.MarkPaidPlanQuotaNotificationPreferenceDisabled(ctx, id)
}

func (s *PostgresPaidQuotaDeliveryStore) ReconciliationWorkspaces(ctx context.Context) ([]string, error) {
	return s.queries.ListPaidQuotaReconciliationWorkspaces(ctx)
}

func textOrEmpty(value pgtype.Text) string {
	if value.Valid {
		return value.String
	}
	return ""
}
