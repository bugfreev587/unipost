package xaccountreads

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/xcredits"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Lookup(
	ctx context.Context,
	workspaceID string,
	keyHash string,
	now time.Time,
) (Receipt, error) {
	var receipt Receipt
	var response []byte
	var encryptedRequest []byte
	var completedAt pgtype.Timestamptz
	err := s.pool.QueryRow(ctx, `
		SELECT operation_id, workspace_id, social_account_id, external_user_id,
		       exposure_id, endpoint, idempotency_key_hash, request_fingerprint,
		       encrypted_request, requested_resources, operation_key, status,
		       response_json, COALESCE(failure_class, ''), attempt_count,
		       next_attempt_at, completed_at, expires_at
		FROM x_read_receipts
		WHERE workspace_id = $1 AND idempotency_key_hash = $2
	`, workspaceID, keyHash).Scan(
		&receipt.OperationID, &receipt.WorkspaceID, &receipt.AccountID, &receipt.ExternalUserID,
		&receipt.ExposureID, &receipt.Endpoint, &receipt.IdempotencyKeyHash,
		&receipt.RequestFingerprint, &encryptedRequest, &receipt.RequestedResources,
		&receipt.OperationKey, &receipt.Status, &response, &receipt.FailureClass,
		&receipt.AttemptCount, &receipt.NextAttemptAt, &completedAt, &receipt.ExpiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Receipt{}, ErrReceiptNotFound
	}
	if err != nil {
		return Receipt{}, err
	}
	if !receipt.ExpiresAt.After(now.UTC()) {
		_, _ = s.pool.Exec(ctx, `
			DELETE FROM x_read_receipts
			WHERE operation_id = $1 AND expires_at <= $2
		`, receipt.OperationID, now.UTC())
		return Receipt{}, ErrReceiptNotFound
	}
	receipt.ResponseJSON = response
	receipt.EncryptedRequest = string(encryptedRequest)
	if completedAt.Valid {
		value := completedAt.Time.UTC()
		receipt.CompletedAt = &value
	}
	return receipt, nil
}

func (s *PostgresStore) AdmissionMutation(input NewReceipt) xcredits.ExposureMutation {
	return func(ctx context.Context, tx pgx.Tx, exposure xcredits.ExposureReservation) error {
		receipt := input.WithExposure(exposure.ID)
		_, err := tx.Exec(ctx, `
			INSERT INTO x_read_receipts (
				operation_id, workspace_id, social_account_id, external_user_id,
				exposure_id, endpoint, idempotency_key_hash, request_fingerprint,
				encrypted_request, requested_resources, operation_key, status,
				next_attempt_at, expires_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::BYTEA, $10, $11, $12, $13, $14)
		`, receipt.OperationID, receipt.WorkspaceID, receipt.AccountID, receipt.ExternalUserID,
			receipt.ExposureID, receipt.Endpoint, receipt.IdempotencyKeyHash,
			receipt.RequestFingerprint, []byte(receipt.EncryptedRequest), receipt.RequestedResources,
			receipt.OperationKey, receipt.Status, receipt.NextAttemptAt, receipt.ExpiresAt,
		)
		return err
	}
}

func (s *PostgresStore) MarkSucceeded(
	ctx context.Context,
	operationID string,
	response []byte,
	now time.Time,
) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE x_read_receipts
		SET status = 'succeeded', response_json = $2::JSONB, completed_at = $3,
		    execution_owner = NULL, execution_lease_expires_at = NULL,
		    failure_class = NULL, updated_at = $3
		WHERE operation_id = $1
		  AND status IN ('executing', 'outcome_unknown', 'settlement_pending', 'succeeded')
	`, operationID, response, now.UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("X account-read success receipt transition was not persisted")
	}
	return nil
}

func (s *PostgresStore) MarkFailed(
	ctx context.Context,
	operationID string,
	class string,
	now time.Time,
) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE x_read_receipts
		SET status = 'failed', failure_class = LEFT($2, 100), completed_at = $3,
		    execution_owner = NULL, execution_lease_expires_at = NULL, updated_at = $3
		WHERE operation_id = $1 AND status IN ('executing', 'outcome_unknown', 'settlement_pending')
	`, operationID, class, now.UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("X account-read failure receipt transition was not persisted")
	}
	return nil
}

func (s *PostgresStore) MarkOutcomeUnknown(
	ctx context.Context,
	operationID string,
	next time.Time,
) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE x_read_receipts
		SET status = 'outcome_unknown', attempt_count = attempt_count + 1,
		    next_attempt_at = $2, execution_owner = NULL,
		    execution_lease_expires_at = NULL, updated_at = NOW()
		WHERE operation_id = $1 AND status IN ('executing', 'outcome_unknown')
	`, operationID, next.UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("X account-read ambiguous receipt transition was not persisted")
	}
	return nil
}

func (s *PostgresStore) MarkSettlementPending(
	ctx context.Context,
	operationID string,
	response []byte,
	next time.Time,
) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE x_read_receipts
		SET status = 'settlement_pending', response_json = $2::JSONB,
		    attempt_count = attempt_count + 1, next_attempt_at = $3,
		    execution_owner = NULL, execution_lease_expires_at = NULL, updated_at = NOW()
		WHERE operation_id = $1 AND status IN ('executing', 'settlement_pending')
	`, operationID, response, next.UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("X account-read settlement-pending receipt transition was not persisted")
	}
	return nil
}

func (s *PostgresStore) ClaimRecoverable(
	ctx context.Context,
	limit int,
	owner string,
	now time.Time,
) ([]Receipt, error) {
	rows, err := s.pool.Query(ctx, `
		WITH candidates AS (
			SELECT operation_id
			FROM x_read_receipts
			WHERE status IN ('outcome_unknown', 'settlement_pending')
			  AND (next_attempt_at <= $3 OR expires_at <= $3)
			  AND (execution_lease_expires_at IS NULL OR execution_lease_expires_at <= $3)
			ORDER BY expires_at, next_attempt_at
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		), claimed AS (
			UPDATE x_read_receipts r
			SET execution_owner = $2, execution_lease_expires_at = $3 + INTERVAL '2 minutes',
			    attempt_count = r.attempt_count + 1, updated_at = $3
			FROM candidates c
			WHERE r.operation_id = c.operation_id
			RETURNING r.*
		)
		SELECT c.operation_id, c.workspace_id, c.social_account_id, c.external_user_id,
		       c.exposure_id, c.endpoint, c.idempotency_key_hash, c.request_fingerprint,
		       c.encrypted_request, c.requested_resources, c.operation_key, c.status,
		       c.response_json, COALESCE(c.failure_class, ''), c.attempt_count,
		       c.next_attempt_at, c.completed_at, c.expires_at,
		       e.accounting_enabled, COALESCE(e.bypass_reason, ''), e.reserved_units,
		       COALESCE(e.actual_units, 0), e.catalog_version, e.app_mode
		FROM claimed c
		JOIN x_read_exposures e ON e.id = c.exposure_id
	`, limit, owner, now.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Receipt, 0, limit)
	for rows.Next() {
		var receipt Receipt
		var encrypted, response []byte
		var completedAt pgtype.Timestamptz
		if err := rows.Scan(
			&receipt.OperationID, &receipt.WorkspaceID, &receipt.AccountID, &receipt.ExternalUserID,
			&receipt.ExposureID, &receipt.Endpoint, &receipt.IdempotencyKeyHash,
			&receipt.RequestFingerprint, &encrypted, &receipt.RequestedResources,
			&receipt.OperationKey, &receipt.Status, &response, &receipt.FailureClass,
			&receipt.AttemptCount, &receipt.NextAttemptAt, &completedAt, &receipt.ExpiresAt,
			&receipt.AccountingEnabled, &receipt.BypassReason, &receipt.ReservedUnits,
			&receipt.ActualUnits, &receipt.CatalogVersion, &receipt.AppMode,
		); err != nil {
			return nil, err
		}
		receipt.EncryptedRequest = string(encrypted)
		receipt.ResponseJSON = response
		if completedAt.Valid {
			value := completedAt.Time.UTC()
			receipt.CompletedAt = &value
		}
		result = append(result, receipt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
