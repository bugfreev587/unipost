package socialconnectioncutover

import (
	"context"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

const cutoverAdvisoryLockName = "unipost:social-connection-profile-bindings-cutover"

type PostgresRolloutStore struct {
	pool *pgxpool.Pool
	mu   sync.Mutex
	conn *pgxpool.Conn
}

func NewPostgresRolloutStore(pool *pgxpool.Pool) *PostgresRolloutStore {
	return &PostgresRolloutStore{pool: pool}
}

func (s *PostgresRolloutStore) Acquire(ctx context.Context) (func(context.Context) error, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("Postgres rollout pool is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		return nil, errors.New("social connection cutover lock is already held")
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1, 0))`, cutoverAdvisoryLockName); err != nil {
		conn.Release()
		return nil, err
	}
	s.conn = conn
	return func(releaseCtx context.Context) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.conn == nil {
			return nil
		}
		_, unlockErr := s.conn.Exec(releaseCtx, `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, cutoverAdvisoryLockName)
		s.conn.Release()
		s.conn = nil
		return unlockErr
	}, nil
}

func (s *PostgresRolloutStore) Phase(ctx context.Context) (string, error) {
	conn, err := s.lockedConnection()
	if err != nil {
		return "", err
	}
	var phase string
	err = conn.QueryRow(ctx, `SELECT phase FROM social_connection_rollout_state WHERE id = 'global'`).Scan(&phase)
	return phase, err
}

func (s *PostgresRolloutStore) SetPhase(ctx context.Context, phase string) error {
	conn, err := s.lockedConnection()
	if err != nil {
		return err
	}
	command, err := conn.Exec(ctx, `
		UPDATE social_connection_rollout_state
		SET phase = $1,
		    cutover_backend_pid = CASE WHEN $1 = 'cutting_over' THEN cutover_backend_pid ELSE NULL END,
		    updated_at = NOW()
		WHERE id = 'global'
	`, phase)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return errors.New("global social connection rollout state is missing")
	}
	return nil
}

func (s *PostgresRolloutStore) ActiveProviderLeases(ctx context.Context) (int64, error) {
	conn, err := s.lockedConnection()
	if err != nil {
		return 0, err
	}
	var count int64
	err = conn.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM post_delivery_jobs
		WHERE state IN ('running', 'retrying')
	`).Scan(&count)
	return count, err
}

func (s *PostgresRolloutStore) lockedConnection() (*pgxpool.Conn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s == nil || s.conn == nil {
		return nil, errors.New("social connection cutover lock is not held")
	}
	return s.conn, nil
}
