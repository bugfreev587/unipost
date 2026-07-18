package featureflags

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) List(ctx context.Context) ([]Flag, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT key, enabled, description, updated_by, updated_at
		FROM feature_flags
		ORDER BY CASE key
			WHEN 'x_dms_v1' THEN 1
			WHEN 'x_credits_billing_v1' THEN 2
			ELSE 99
		END
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Flag, 0, len(definitions))
	for rows.Next() {
		var flag Flag
		if err := rows.Scan(&flag.Key, &flag.Enabled, &flag.Description, &flag.UpdatedBy, &flag.UpdatedAt); err != nil {
			return nil, err
		}
		if _, ok := DefinitionFor(flag.Key); ok {
			result = append(result, flag)
		}
	}
	return result, rows.Err()
}

func (s *PostgresStore) Set(ctx context.Context, key string, enabled bool, actor string) (Flag, error) {
	if _, ok := DefinitionFor(key); !ok {
		return Flag{}, fmt.Errorf("%w: %s", ErrUnknownFlag, key)
	}
	if actor == "" {
		return Flag{}, errors.New("feature flag actor is required")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Flag{}, err
	}
	defer tx.Rollback(ctx)

	var previous bool
	if err := tx.QueryRow(ctx, `
		SELECT enabled
		FROM feature_flags
		WHERE key = $1
		FOR UPDATE
	`, key).Scan(&previous); err != nil {
		return Flag{}, err
	}

	var flag Flag
	if err := tx.QueryRow(ctx, `
		UPDATE feature_flags
		SET enabled = $2, updated_by = $3, updated_at = NOW()
		WHERE key = $1
		RETURNING key, enabled, description, updated_by, updated_at
	`, key, enabled, actor).Scan(
		&flag.Key,
		&flag.Enabled,
		&flag.Description,
		&flag.UpdatedBy,
		&flag.UpdatedAt,
	); err != nil {
		return Flag{}, err
	}
	if previous != enabled {
		if _, err := tx.Exec(ctx, `
			INSERT INTO feature_flag_changes (
				flag_key, previous_enabled, enabled, changed_by
			) VALUES ($1, $2, $3, $4)
		`, key, previous, enabled, actor); err != nil {
			return Flag{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Flag{}, err
	}
	return flag, nil
}

func (s *PostgresStore) GlobalEnabled(ctx context.Context, key string) (bool, error) {
	if _, ok := DefinitionFor(key); !ok {
		return false, fmt.Errorf("%w: %s", ErrUnknownFlag, key)
	}
	var enabled bool
	err := s.pool.QueryRow(ctx, `SELECT enabled FROM feature_flags WHERE key = $1`, key).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return enabled, err
}

func (s *PostgresStore) WorkspaceOwner(ctx context.Context, workspaceID string) (string, error) {
	var ownerID string
	err := s.pool.QueryRow(ctx, `SELECT user_id FROM workspaces WHERE id = $1`, workspaceID).Scan(&ownerID)
	return ownerID, err
}
