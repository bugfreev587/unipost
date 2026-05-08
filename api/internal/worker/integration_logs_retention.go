package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/integrationlogs"
)

const listWorkspacePlansSQL = `
SELECT w.id, COALESCE(s.plan_id, 'free') AS plan_id
FROM workspaces w
LEFT JOIN subscriptions s ON s.workspace_id = w.id
`

type IntegrationLogRetentionWorker struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewIntegrationLogRetentionWorker(pool *pgxpool.Pool, queries *db.Queries) *IntegrationLogRetentionWorker {
	return &IntegrationLogRetentionWorker{pool: pool, queries: queries}
}

func (w *IntegrationLogRetentionWorker) Start(ctx context.Context) {
	if w == nil || w.pool == nil || w.queries == nil {
		return
	}

	slog.Info("integration log retention worker started")
	w.runOnce(ctx)

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("integration log retention worker stopped")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *IntegrationLogRetentionWorker) runOnce(ctx context.Context) {
	rows, err := w.pool.Query(ctx, listWorkspacePlansSQL)
	if err != nil {
		slog.Warn("integration log retention: list workspaces failed", "error", err)
		return
	}
	defer rows.Close()

	type workspacePlan struct {
		workspaceID string
		planID      string
	}

	var plans []workspacePlan
	for rows.Next() {
		var item workspacePlan
		if scanErr := rows.Scan(&item.workspaceID, &item.planID); scanErr != nil {
			slog.Warn("integration log retention: scan failed", "error", scanErr)
			continue
		}
		plans = append(plans, item)
	}
	if err := rows.Err(); err != nil {
		slog.Warn("integration log retention: rows failed", "error", err)
		return
	}

	now := time.Now().UTC()
	for _, item := range plans {
		retentionDays := integrationlogs.RetentionDaysForPlan(item.planID)
		cutoff := now.AddDate(0, 0, -retentionDays)
		if err := w.queries.DeleteExpiredIntegrationLogsForWorkspace(ctx, db.DeleteExpiredIntegrationLogsForWorkspaceParams{
			WorkspaceID: item.workspaceID,
			Ts:          pgtype.Timestamptz{Time: cutoff, Valid: true},
		}); err != nil {
			slog.Warn("integration log retention: delete failed",
				"workspace_id", item.workspaceID,
				"plan_id", item.planID,
				"error", err,
			)
		}
	}
}
