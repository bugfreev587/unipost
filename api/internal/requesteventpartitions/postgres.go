package requesteventpartitions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

type PostgresStore struct {
	db Beginner
}

func NewPostgresStore(db Beginner) *PostgresStore {
	return &PostgresStore{db: db}
}

type DefaultPartitionOccupiedError struct {
	Week       Week
	EventRows  int64
	DetailRows int64
}

func (e *DefaultPartitionOccupiedError) Error() string {
	return fmt.Sprintf(
		"default partitions contain rows for [%s, %s): events=%d details=%d",
		e.Week.Start.Format(time.RFC3339),
		e.Week.End.Format(time.RFC3339),
		e.EventRows,
		e.DetailRows,
	)
}

func (s *PostgresStore) Ensure(ctx context.Context, weeks []Week) error {
	if s == nil || s.db == nil {
		return errors.New("partition store is not configured")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin partition ensure: %w", err)
	}
	defer tx.Rollback(context.Background())

	if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout = '5s'`); err != nil {
		return fmt.Errorf("set partition lock timeout: %w", err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL statement_timeout = '30s'`); err != nil {
		return fmt.Errorf("set partition statement timeout: %w", err)
	}
	if _, err := tx.Exec(
		ctx,
		`SELECT pg_advisory_xact_lock($1::INTEGER, $2::INTEGER)`,
		PartitionLockNamespace,
		PartitionLockKey,
	); err != nil {
		return fmt.Errorf("lock request-event partition maintenance: %w", err)
	}

	for _, week := range weeks {
		if err := ensureWeek(ctx, tx, week); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit partition ensure: %w", err)
	}
	return nil
}

func ensureWeek(ctx context.Context, tx pgx.Tx, week Week) error {
	canonical, err := weekForStart(week.Start)
	if err != nil {
		return fmt.Errorf("validate request-event partition week: %w", err)
	}
	if !week.Start.Equal(canonical.Start) || !week.End.Equal(canonical.End) ||
		week.EventTable != canonical.EventTable || week.DetailTable != canonical.DetailTable {
		return fmt.Errorf("request-event partition week does not match canonical ISO week: %#v", week)
	}

	var manifestEnd time.Time
	var manifestEvent, manifestDetail string
	err = tx.QueryRow(ctx, `
		SELECT week_end, event_partition, detail_partition
		FROM api_request_partition_manifest
		WHERE week_start = $1
	`, week.Start).Scan(&manifestEnd, &manifestEvent, &manifestDetail)
	if err == nil {
		if !manifestEnd.Equal(week.End) || manifestEvent != week.EventTable || manifestDetail != week.DetailTable {
			return fmt.Errorf(
				"request-event partition manifest mismatch for %s: end=%s event=%q detail=%q",
				week.Start.Format(time.RFC3339),
				manifestEnd.Format(time.RFC3339),
				manifestEvent,
				manifestDetail,
			)
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("read request-event partition manifest for %s: %w", week.Start.Format(time.RFC3339), err)
	}

	var eventRows, detailRows int64
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM api_request_events_default
		WHERE occurred_at >= $1 AND occurred_at < $2
	`, week.Start, week.End).Scan(&eventRows); err != nil {
		return fmt.Errorf("count request-event default rows for %s: %w", week.Start.Format(time.RFC3339), err)
	}
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM api_request_error_details_default
		WHERE occurred_at >= $1 AND occurred_at < $2
	`, week.Start, week.End).Scan(&detailRows); err != nil {
		return fmt.Errorf("count request-error default rows for %s: %w", week.Start.Format(time.RFC3339), err)
	}
	if eventRows != 0 || detailRows != 0 {
		return &DefaultPartitionOccupiedError{
			Week:       week,
			EventRows:  eventRows,
			DetailRows: detailRows,
		}
	}

	eventTable := pgx.Identifier{week.EventTable}.Sanitize()
	detailTable := pgx.Identifier{week.DetailTable}.Sanitize()
	start := week.Start.UTC().Format(time.RFC3339)
	end := week.End.UTC().Format(time.RFC3339)
	if _, err := tx.Exec(ctx, fmt.Sprintf(
		"CREATE TABLE %s PARTITION OF api_request_events FOR VALUES FROM ('%s') TO ('%s')",
		eventTable,
		start,
		end,
	)); err != nil {
		return fmt.Errorf("create request-event partition %s: %w", week.EventTable, err)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(
		"CREATE TABLE %s PARTITION OF api_request_error_details FOR VALUES FROM ('%s') TO ('%s')",
		detailTable,
		start,
		end,
	)); err != nil {
		return fmt.Errorf("create request-error partition %s: %w", week.DetailTable, err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO api_request_partition_manifest (
			week_start, week_end, event_partition, detail_partition
		) VALUES ($1, $2, $3, $4)
	`, week.Start, week.End, week.EventTable, week.DetailTable); err != nil {
		return fmt.Errorf("insert request-event partition manifest for %s: %w", week.Start.Format(time.RFC3339), err)
	}
	return nil
}
