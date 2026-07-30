package requesteventpartitions

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
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

type PartitionDriftError struct {
	Week   Week
	Reason string
}

func (e *PartitionDriftError) Error() string {
	return fmt.Sprintf(
		"request-event partition drift for [%s, %s): %s",
		e.Week.Start.Format(time.RFC3339),
		e.Week.End.Format(time.RFC3339),
		e.Reason,
	)
}

type partitionRelationState struct {
	Exists       bool
	IsPartition  bool
	Attached     bool
	BoundMatches bool
}

func (s partitionRelationState) valid() bool {
	return s.Exists && s.IsPartition && s.Attached && s.BoundMatches
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
	if _, err := tx.Exec(ctx, `SET LOCAL TIME ZONE 'UTC'`); err != nil {
		return fmt.Errorf("set partition time zone: %w", err)
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
	manifestExists := err == nil
	if manifestExists {
		if !manifestEnd.Equal(week.End) || manifestEvent != week.EventTable || manifestDetail != week.DetailTable {
			return fmt.Errorf(
				"request-event partition manifest mismatch for %s: end=%s event=%q detail=%q",
				week.Start.Format(time.RFC3339),
				manifestEnd.Format(time.RFC3339),
				manifestEvent,
				manifestDetail,
			)
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("read request-event partition manifest for %s: %w", week.Start.Format(time.RFC3339), err)
	}

	eventState, err := inspectPartitionRelation(
		ctx,
		tx,
		week.EventTable,
		"api_request_events",
		week.Start,
		week.End,
	)
	if err != nil {
		return fmt.Errorf("inspect request-event partition %s: %w", week.EventTable, err)
	}
	detailState, err := inspectPartitionRelation(
		ctx,
		tx,
		week.DetailTable,
		"api_request_error_details",
		week.Start,
		week.End,
	)
	if err != nil {
		return fmt.Errorf("inspect request-error partition %s: %w", week.DetailTable, err)
	}

	if manifestExists {
		if eventState.valid() && detailState.valid() {
			return nil
		}
		return &PartitionDriftError{
			Week: week,
			Reason: fmt.Sprintf(
				"manifest exists but physical pair is invalid: event=%+v detail=%+v",
				eventState,
				detailState,
			),
		}
	}

	if eventState.valid() && detailState.valid() {
		return insertManifestRecord(ctx, tx, week)
	}
	if eventState.Exists || detailState.Exists {
		return &PartitionDriftError{
			Week: week,
			Reason: fmt.Sprintf(
				"manifest is missing and physical pair is incomplete or invalid: event=%+v detail=%+v",
				eventState,
				detailState,
			),
		}
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
	return insertManifestRecord(ctx, tx, week)
}

func inspectPartitionRelation(
	ctx context.Context,
	tx pgx.Tx,
	childName string,
	parentName string,
	start time.Time,
	end time.Time,
) (partitionRelationState, error) {
	var state partitionRelationState
	err := tx.QueryRow(ctx, `
		SELECT
			child.oid IS NOT NULL,
			COALESCE(child.relispartition, FALSE),
			COALESCE(EXISTS (
				SELECT 1
				FROM pg_inherits AS inheritance
				JOIN pg_class AS parent ON parent.oid = inheritance.inhparent
				JOIN pg_namespace AS parent_namespace ON parent_namespace.oid = parent.relnamespace
				WHERE inheritance.inhrelid = child.oid
				  AND parent_namespace.nspname = current_schema()
				  AND parent.relname = $2
			), FALSE),
			COALESCE(
				pg_get_expr(child.relpartbound, child.oid, TRUE) =
					format('FOR VALUES FROM (%L) TO (%L)', $3::TIMESTAMPTZ, $4::TIMESTAMPTZ),
				FALSE
			)
		FROM (SELECT 1) AS singleton
		LEFT JOIN pg_class AS child
			JOIN pg_namespace AS child_namespace ON child_namespace.oid = child.relnamespace
				AND child_namespace.nspname = current_schema()
			ON child.relname = $1
	`, childName, parentName, start, end).Scan(
		&state.Exists,
		&state.IsPartition,
		&state.Attached,
		&state.BoundMatches,
	)
	return state, err
}

func insertManifestRecord(ctx context.Context, tx pgx.Tx, week Week) error {
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
	if _, err := tx.Exec(ctx, `SET LOCAL TIME ZONE 'UTC'`); err != nil {
		return Inspection{}, fmt.Errorf("set partition inspection time zone: %w", err)
	}

	inspection := Inspection{
		InspectedAt: now.UTC(),
		Reasons:     make([]string, 0, 4),
	}
	mismatchedManifest, err := inspectManifestCatalog(ctx, tx, &inspection)
	if err != nil {
		return Inspection{}, err
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

	if inspection.CoverageDays < minimumCoverageDays {
		inspection.Reasons = append(inspection.Reasons, ReasonCoverageLow)
	}
	if inspection.EventDefaultRows > 0 {
		inspection.Reasons = append(inspection.Reasons, ReasonEventDefaultRows)
	}
	if inspection.DetailDefaultRows > 0 {
		inspection.Reasons = append(inspection.Reasons, ReasonDetailDefaultRows)
	}
	if mismatchedManifest {
		inspection.Reasons = append(inspection.Reasons, ReasonPartitionMismatch)
	}
	sort.Strings(inspection.Reasons)
	inspection.Ready = len(inspection.Reasons) == 0

	if err := tx.Commit(ctx); err != nil {
		return Inspection{}, fmt.Errorf("commit partition inspection: %w", err)
	}
	return inspection, nil
}

func inspectManifestCatalog(ctx context.Context, tx pgx.Tx, inspection *Inspection) (bool, error) {
	rows, err := tx.Query(ctx, `
		SELECT week_start, week_end, event_partition, detail_partition
		FROM api_request_partition_manifest
		ORDER BY week_start
	`)
	if err != nil {
		return false, fmt.Errorf("inspect request-event partition manifest: %w", err)
	}
	defer rows.Close()

	mismatched := false
	manifestWeeks := make([]Week, 0)
	for rows.Next() {
		var week Week
		if err := rows.Scan(&week.Start, &week.End, &week.EventTable, &week.DetailTable); err != nil {
			return false, fmt.Errorf("scan request-event partition manifest: %w", err)
		}
		inspection.PartitionPairs++
		if week.End.After(inspection.LatestExplicitEnd) {
			inspection.LatestExplicitEnd = week.End.UTC()
		}
		manifestWeeks = append(manifestWeeks, week)
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate request-event partition manifest: %w", err)
	}
	rows.Close()

	for _, week := range manifestWeeks {
		canonical, canonicalErr := weekForStart(week.Start)
		if canonicalErr != nil ||
			week.End.Sub(week.Start) != 7*24*time.Hour ||
			week.EventTable != canonical.EventTable ||
			week.DetailTable != canonical.DetailTable {
			mismatched = true
			continue
		}
		eventState, err := inspectPartitionRelation(
			ctx,
			tx,
			week.EventTable,
			"api_request_events",
			week.Start,
			week.End,
		)
		if err != nil {
			return false, fmt.Errorf("inspect request-event partition %s: %w", week.EventTable, err)
		}
		detailState, err := inspectPartitionRelation(
			ctx,
			tx,
			week.DetailTable,
			"api_request_error_details",
			week.Start,
			week.End,
		)
		if err != nil {
			return false, fmt.Errorf("inspect request-error partition %s: %w", week.DetailTable, err)
		}
		if !eventState.valid() || !detailState.valid() {
			mismatched = true
		}
	}

	var orphanEventPartitions, orphanDetailPartitions int64
	if err := tx.QueryRow(ctx, `
		SELECT
			(
				SELECT COUNT(*)
				FROM pg_inherits AS inheritance
				JOIN pg_class AS child ON child.oid = inheritance.inhrelid
				JOIN pg_namespace AS child_namespace ON child_namespace.oid = child.relnamespace
				JOIN pg_class AS parent ON parent.oid = inheritance.inhparent
				JOIN pg_namespace AS parent_namespace ON parent_namespace.oid = parent.relnamespace
				WHERE child_namespace.nspname = current_schema()
				  AND parent_namespace.nspname = current_schema()
				  AND parent.relname = 'api_request_events'
				  AND child.relname <> 'api_request_events_default'
				  AND NOT EXISTS (
					SELECT 1 FROM api_request_partition_manifest AS manifest
					WHERE manifest.event_partition = child.relname
				  )
			),
			(
				SELECT COUNT(*)
				FROM pg_inherits AS inheritance
				JOIN pg_class AS child ON child.oid = inheritance.inhrelid
				JOIN pg_namespace AS child_namespace ON child_namespace.oid = child.relnamespace
				JOIN pg_class AS parent ON parent.oid = inheritance.inhparent
				JOIN pg_namespace AS parent_namespace ON parent_namespace.oid = parent.relnamespace
				WHERE child_namespace.nspname = current_schema()
				  AND parent_namespace.nspname = current_schema()
				  AND parent.relname = 'api_request_error_details'
				  AND child.relname <> 'api_request_error_details_default'
				  AND NOT EXISTS (
					SELECT 1 FROM api_request_partition_manifest AS manifest
					WHERE manifest.detail_partition = child.relname
				  )
			)
	`).Scan(&orphanEventPartitions, &orphanDetailPartitions); err != nil {
		return false, fmt.Errorf("inspect orphan request-event partitions: %w", err)
	}

	return mismatched || orphanEventPartitions > 0 || orphanDetailPartitions > 0, nil
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
