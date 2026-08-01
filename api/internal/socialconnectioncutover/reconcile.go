package socialconnectioncutover

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed sql/cutover.sql
var cutoverSQL string

var exactCommitSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

func ValidCommitSHA(value string) bool {
	return exactCommitSHA.MatchString(strings.TrimSpace(value))
}

type Reconciler struct {
	Pool          *pgxpool.Pool
	CommitSHA     string
	EnvironmentID string
}

type durableStateCounts struct {
	ReservationUnits int64
	OperationRows    int64
	InboxRows        int64
	ResultRows       int64
}

func (r *Reconciler) Run(ctx context.Context, _ Report) error {
	if r == nil || r.Pool == nil {
		return errors.New("cutover reconciliation database pool is required")
	}
	sha := strings.TrimSpace(r.CommitSHA)
	if !exactCommitSHA.MatchString(sha) {
		return errors.New("cutover reconciliation requires an exact lowercase 40-character commit SHA")
	}
	environmentID := strings.TrimSpace(r.EnvironmentID)
	if environmentID == "" {
		return errors.New("cutover reconciliation environment ID is required")
	}

	tx, err := r.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin social connection cutover reconciliation: %w", err)
	}
	defer tx.Rollback(context.Background())

	before, err := readDurableStateCounts(ctx, tx)
	if err != nil {
		return fmt.Errorf("read pre-cutover durable state counts: %w", err)
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('unipost.cutover_sha', $1, TRUE)`, sha); err != nil {
		return fmt.Errorf("bind cutover commit identity: %w", err)
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('unipost.cutover_environment_id', $1, TRUE)`, environmentID); err != nil {
		return fmt.Errorf("bind cutover environment identity: %w", err)
	}
	if _, err := tx.Exec(ctx, cutoverSQL); err != nil {
		return fmt.Errorf("reconcile social connection authority: %w", err)
	}
	after, err := readDurableStateCounts(ctx, tx)
	if err != nil {
		return fmt.Errorf("read post-cutover durable state counts: %w", err)
	}
	if before != after {
		return fmt.Errorf("cutover durable-state conservation failed: before=%+v after=%+v", before, after)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit social connection authority reconciliation: %w", err)
	}
	return nil
}

func readDurableStateCounts(ctx context.Context, tx pgx.Tx) (durableStateCounts, error) {
	var counts durableStateCounts
	err := tx.QueryRow(ctx, `
		SELECT
		  COALESCE((SELECT SUM(reserved_count) FROM physical_daily_publish_reservations), 0),
		  (SELECT COUNT(*) FROM physical_daily_publish_operations),
		  (SELECT COUNT(*) FROM inbox_items),
		  (SELECT COUNT(*) FROM social_post_results)
	`).Scan(&counts.ReservationUnits, &counts.OperationRows, &counts.InboxRows, &counts.ResultRows)
	return counts, err
}
