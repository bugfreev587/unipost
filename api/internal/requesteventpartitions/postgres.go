package requesteventpartitions

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	ReasonCoverageLow       = "explicit_coverage_below_14_days"
	ReasonEventDefaultRows  = "event_default_partition_occupied"
	ReasonDetailDefaultRows = "detail_default_partition_occupied"
	ReasonPartitionMismatch = "partition_manifest_mismatch"
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

type Inspection struct {
	InspectedAt         time.Time `json:"inspected_at"`
	LatestExplicitEnd   time.Time `json:"latest_explicit_end"`
	CoverageDays        int       `json:"coverage_days"`
	PartitionPairs      int64     `json:"partition_pairs"`
	EventDefaultRows    int64     `json:"event_default_rows"`
	DetailDefaultRows   int64     `json:"detail_default_rows"`
	EventEstimatedRows  int64     `json:"event_estimated_rows"`
	DetailEstimatedRows int64     `json:"detail_estimated_rows"`
	EventTotalBytes     int64     `json:"event_total_bytes"`
	DetailTotalBytes    int64     `json:"detail_total_bytes"`
	Ready               bool      `json:"ready"`
	Reasons             []string  `json:"reasons"`
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

func (s *PostgresStore) Inspect(
	ctx context.Context,
	now time.Time,
	minimumCoverageDays int,
) (Inspection, error) {
	if s == nil || s.db == nil {
		return Inspection{}, errors.New("partition store is not configured")
	}
	if minimumCoverageDays < 0 {
		return Inspection{}, errors.New("minimum coverage days must be nonnegative")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Inspection{}, fmt.Errorf("begin partition inspection: %w", err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `SET TRANSACTION READ ONLY`); err != nil {
		return Inspection{}, fmt.Errorf("set partition inspection read only: %w", err)
	}

	inspection := Inspection{
		InspectedAt: now.UTC(),
		Reasons:     make([]string, 0, 4),
	}
	var latestEnd pgtype.Timestamptz
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*), MAX(week_end)
		FROM api_request_partition_manifest
	`).Scan(&inspection.PartitionPairs, &latestEnd); err != nil {
		return Inspection{}, fmt.Errorf("inspect request-event partition manifest: %w", err)
	}
	if latestEnd.Valid {
		inspection.LatestExplicitEnd = latestEnd.Time.UTC()
	}
	inspection.CoverageDays = ExplicitCoverageDays(inspection.InspectedAt, inspection.LatestExplicitEnd)

	if err := tx.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM api_request_events_default),
			(SELECT COUNT(*) FROM api_request_error_details_default)
	`).Scan(&inspection.EventDefaultRows, &inspection.DetailDefaultRows); err != nil {
		return Inspection{}, fmt.Errorf("inspect request-event default partitions: %w", err)
	}
	if err := inspectPartitionTree(
		ctx,
		tx,
		"api_request_events",
		&inspection.EventEstimatedRows,
		&inspection.EventTotalBytes,
	); err != nil {
		return Inspection{}, err
	}
	if err := inspectPartitionTree(
		ctx,
		tx,
		"api_request_error_details",
		&inspection.DetailEstimatedRows,
		&inspection.DetailTotalBytes,
	); err != nil {
		return Inspection{}, err
	}

	var mismatchedManifestRows int64
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM api_request_partition_manifest AS manifest
		WHERE manifest.week_end <> manifest.week_start + INTERVAL '7 days'
		   OR manifest.event_partition <>
		      'api_request_events_' || to_char(manifest.week_start AT TIME ZONE 'UTC', 'IYYY"w"IW')
		   OR manifest.detail_partition <>
		      'api_request_error_details_' || to_char(manifest.week_start AT TIME ZONE 'UTC', 'IYYY"w"IW')
		   OR NOT EXISTS (
				SELECT 1
				FROM pg_inherits AS inheritance
				JOIN pg_class AS child ON child.oid = inheritance.inhrelid
				JOIN pg_namespace AS child_namespace ON child_namespace.oid = child.relnamespace
				JOIN pg_class AS parent ON parent.oid = inheritance.inhparent
				JOIN pg_namespace AS parent_namespace ON parent_namespace.oid = parent.relnamespace
				WHERE child_namespace.nspname = current_schema()
				  AND parent_namespace.nspname = current_schema()
				  AND child.relname = manifest.event_partition
				  AND parent.relname = 'api_request_events'
		   )
		   OR NOT EXISTS (
				SELECT 1
				FROM pg_inherits AS inheritance
				JOIN pg_class AS child ON child.oid = inheritance.inhrelid
				JOIN pg_namespace AS child_namespace ON child_namespace.oid = child.relnamespace
				JOIN pg_class AS parent ON parent.oid = inheritance.inhparent
				JOIN pg_namespace AS parent_namespace ON parent_namespace.oid = parent.relnamespace
				WHERE child_namespace.nspname = current_schema()
				  AND parent_namespace.nspname = current_schema()
				  AND child.relname = manifest.detail_partition
				  AND parent.relname = 'api_request_error_details'
		   )
	`).Scan(&mismatchedManifestRows); err != nil {
		return Inspection{}, fmt.Errorf("inspect request-event partition attachments: %w", err)
	}

	if inspection.CoverageDays < minimumCoverageDays {
		inspection.Reasons = append(inspection.Reasons, ReasonCoverageLow)
	}
	if inspection.EventDefaultRows > 0 {
		inspection.Reasons = append(inspection.Reasons, ReasonEventDefaultRows)
	}
	if inspection.DetailDefaultRows > 0 {
		inspection.Reasons = append(inspection.Reasons, ReasonDetailDefaultRows)
	}
	if mismatchedManifestRows > 0 {
		inspection.Reasons = append(inspection.Reasons, ReasonPartitionMismatch)
	}
	sort.Strings(inspection.Reasons)
	inspection.Ready = len(inspection.Reasons) == 0

	if err := tx.Commit(ctx); err != nil {
		return Inspection{}, fmt.Errorf("commit partition inspection: %w", err)
	}
	return inspection, nil
}

func inspectPartitionTree(
	ctx context.Context,
	tx pgx.Tx,
	parent string,
	estimatedRows *int64,
	totalBytes *int64,
) error {
	if err := tx.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(COALESCE(stats.n_live_tup, 0)), 0)::BIGINT,
			COALESCE(SUM(pg_total_relation_size(tree.relid)), 0)::BIGINT
		FROM pg_partition_tree($1::REGCLASS) AS tree
		LEFT JOIN pg_stat_user_tables AS stats ON stats.relid = tree.relid
		WHERE tree.isleaf
	`, parent).Scan(estimatedRows, totalBytes); err != nil {
		return fmt.Errorf("inspect partition tree %s: %w", parent, err)
	}
	return nil
}
