package requesteventpartitions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureDatabaseReady(ctx context.Context, databaseURL string) error {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("parse partition database URL: %w", err)
	}
	config.MaxConns = 1
	config.MinConns = 0
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("open partition database: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping partition database: %w", err)
	}

	store := NewPostgresStore(pool)
	now := time.Now().UTC()
	weeks, err := PlanWeeks(now, MaintenanceWeeksAhead)
	if err != nil {
		return fmt.Errorf("plan request-event partitions: %w", err)
	}
	if err := store.Ensure(ctx, weeks); err != nil {
		return fmt.Errorf("ensure request-event partitions: %w", err)
	}
	inspection, err := store.Inspect(ctx, now, MinimumCoverageDays)
	if err != nil {
		return fmt.Errorf("inspect request-event partitions: %w", err)
	}
	if !inspection.Ready {
		return fmt.Errorf("request-event partitions are not ready: %s", strings.Join(inspection.Reasons, ","))
	}
	return nil
}
