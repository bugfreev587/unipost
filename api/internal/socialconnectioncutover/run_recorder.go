package socialconnectioncutover

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresCutoverRunRecorder struct {
	pool           *pgxpool.Pool
	applicationSHA string
	environmentID  string
}

func NewPostgresCutoverRunRecorder(pool *pgxpool.Pool, applicationSHA, environmentID string) *PostgresCutoverRunRecorder {
	return &PostgresCutoverRunRecorder{pool: pool, applicationSHA: strings.TrimSpace(applicationSHA), environmentID: strings.TrimSpace(environmentID)}
}

func (r *PostgresCutoverRunRecorder) Start(ctx context.Context, phase string) (string, error) {
	if r == nil || r.pool == nil || r.applicationSHA == "" || r.environmentID == "" {
		return "", errors.New("cutover run recorder identity is required")
	}
	var id string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO social_connection_cutover_runs (
		  application_sha, environment_id, phase_before, status
		) VALUES ($1, $2, $3, 'started')
		RETURNING id
	`, r.applicationSHA, r.environmentID, phase).Scan(&id)
	return id, err
}

func (r *PostgresCutoverRunRecorder) Finish(ctx context.Context, id, status string, report Report) error {
	body, err := json.Marshal(report)
	if err != nil {
		return err
	}
	command, err := r.pool.Exec(ctx, `
		UPDATE social_connection_cutover_runs
		SET status = $2, report = $3, completed_at = NOW()
		WHERE id = $1 AND status = 'started'
	`, id, status, body)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return errors.New("cutover run record is missing or already completed")
	}
	return nil
}
