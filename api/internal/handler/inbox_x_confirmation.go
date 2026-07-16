package handler

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type xBackfillAccountSnapshot struct {
	ID      string `json:"id"`
	AppMode string `json:"app_mode"`
}

type xBackfillConfirmationOperation struct {
	ID                 string
	WorkspaceID        string
	Accounts           []xBackfillAccountSnapshot
	AccountFingerprint string
	Request            xBackfillRequest
	EstimatedXCredits  int64
	Nonce              string
	Status             string
	Result             json.RawMessage
	ExpiresAt          time.Time
	ExecutionOwner     string
	ExecutionLease     time.Time
	StartedByThisCall  bool
}

const xBackfillExecutionLease = 30 * time.Minute

func xBackfillAccountSnapshots(accounts []db.SocialAccount) []xBackfillAccountSnapshot {
	snapshots := make([]xBackfillAccountSnapshot, 0, len(accounts))
	for _, account := range accounts {
		snapshots = append(snapshots, xBackfillAccountSnapshot{
			ID: account.ID, AppMode: account.XAppMode.String,
		})
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].ID < snapshots[j].ID })
	return snapshots
}

func xBackfillAccountFingerprint(accounts []xBackfillAccountSnapshot) string {
	raw, _ := json.Marshal(accounts)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func signXBackfillOperationToken(secret []byte, operationID, nonce string) (string, error) {
	if len(secret) == 0 || strings.TrimSpace(operationID) == "" || strings.TrimSpace(nonce) == "" {
		return "", errors.New("X backfill confirmation signing secret is not configured")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(operationID + "\x00" + nonce))
	return operationID + "." + nonce + "." +
		base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func verifyXBackfillOperationToken(secret []byte, token string) (string, string, error) {
	parts := strings.Split(token, ".")
	if len(secret) == 0 || len(parts) != 3 ||
		strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", errors.New("invalid X backfill confirmation token")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", "", errors.New("invalid X backfill confirmation token")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0] + "\x00" + parts[1]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return "", "", errors.New("invalid X backfill confirmation token")
	}
	return parts[0], parts[1], nil
}

func newXBackfillExecutionOwner() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (h *InboxHandler) createXBackfillConfirmationOperation(
	ctx context.Context,
	workspaceID string,
	accounts []db.SocialAccount,
	request xBackfillRequest,
	estimate int64,
	now time.Time,
) (xBackfillConfirmationOperation, string, error) {
	snapshots := xBackfillAccountSnapshots(accounts)
	accountJSON, err := json.Marshal(snapshots)
	if err != nil {
		return xBackfillConfirmationOperation{}, "", err
	}
	request.ConfirmationToken = ""
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return xBackfillConfirmationOperation{}, "", err
	}
	nonceBytes := make([]byte, 24)
	if _, err := rand.Read(nonceBytes); err != nil {
		return xBackfillConfirmationOperation{}, "", err
	}
	expiresAt := now.UTC().Add(xBackfillConfirmationTTL)
	operation := xBackfillConfirmationOperation{
		WorkspaceID:        workspaceID,
		Accounts:           snapshots,
		AccountFingerprint: xBackfillAccountFingerprint(snapshots),
		Request:            request,
		EstimatedXCredits:  estimate,
		Nonce:              base64.RawURLEncoding.EncodeToString(nonceBytes),
		Status:             "pending",
		ExpiresAt:          expiresAt,
	}
	err = h.pool.QueryRow(ctx, `
		INSERT INTO x_inbox_backfill_confirmation_operations (
			workspace_id, account_ids, account_fingerprint, request_snapshot,
			estimated_x_credits, nonce, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, workspaceID, accountJSON, operation.AccountFingerprint, requestJSON, estimate,
		operation.Nonce, expiresAt).Scan(&operation.ID)
	if err != nil {
		return xBackfillConfirmationOperation{}, "", err
	}
	token, err := signXBackfillOperationToken(
		h.xBackfillConfirmationSecret, operation.ID, operation.Nonce,
	)
	return operation, token, err
}

func (h *InboxHandler) beginXBackfillConfirmationOperation(
	ctx context.Context,
	workspaceID string,
	token string,
	now time.Time,
) (xBackfillConfirmationOperation, error) {
	operationID, tokenNonce, err := verifyXBackfillOperationToken(h.xBackfillConfirmationSecret, token)
	if err != nil {
		return xBackfillConfirmationOperation{}, err
	}
	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return xBackfillConfirmationOperation{}, err
	}
	defer tx.Rollback(ctx)
	var operation xBackfillConfirmationOperation
	var accountJSON, requestJSON, resultJSON []byte
	var executionOwner pgtype.Text
	var executionLease pgtype.Timestamptz
	err = tx.QueryRow(ctx, `
		SELECT id, workspace_id, account_ids, account_fingerprint, request_snapshot,
		       estimated_x_credits, nonce, status, COALESCE(result, 'null'::JSONB), expires_at,
		       execution_owner, execution_lease_expires_at
		FROM x_inbox_backfill_confirmation_operations
		WHERE id = $1
		FOR UPDATE
	`, operationID).Scan(
		&operation.ID, &operation.WorkspaceID, &accountJSON, &operation.AccountFingerprint,
		&requestJSON, &operation.EstimatedXCredits, &operation.Nonce, &operation.Status,
		&resultJSON, &operation.ExpiresAt, &executionOwner, &executionLease,
	)
	if err != nil {
		return xBackfillConfirmationOperation{}, err
	}
	if operation.WorkspaceID != workspaceID {
		return xBackfillConfirmationOperation{}, errors.New("X backfill confirmation operation belongs to another workspace")
	}
	if !hmac.Equal([]byte(operation.Nonce), []byte(tokenNonce)) {
		return xBackfillConfirmationOperation{}, errors.New("invalid X backfill confirmation token")
	}
	if err := json.Unmarshal(accountJSON, &operation.Accounts); err != nil {
		return xBackfillConfirmationOperation{}, err
	}
	if err := json.Unmarshal(requestJSON, &operation.Request); err != nil {
		return xBackfillConfirmationOperation{}, err
	}
	operation.Result = append([]byte(nil), resultJSON...)
	if executionOwner.Valid {
		operation.ExecutionOwner = executionOwner.String
	}
	if executionLease.Valid {
		operation.ExecutionLease = executionLease.Time.UTC()
	}
	if !operation.ExpiresAt.After(now.UTC()) && operation.Status == "pending" {
		if _, err := tx.Exec(ctx, `
			UPDATE x_inbox_backfill_confirmation_operations
			SET status = 'expired',
			    updated_at = NOW()
			WHERE id = $1
		`, operation.ID); err != nil {
			return xBackfillConfirmationOperation{}, err
		}
		operation.Status = "expired"
	}
	if operation.Status == "running" &&
		!operation.ExecutionLease.IsZero() &&
		!operation.ExecutionLease.After(now.UTC()) {
		tag, err := tx.Exec(ctx, `
			UPDATE x_inbox_backfill_confirmation_operations
			SET status = 'failed',
			    last_error = 'confirmation execution lease expired before result persistence',
			    completed_at = NOW(),
			    updated_at = NOW()
			WHERE id = $1
			  AND status = 'running'
			  AND execution_owner = $2
		`, operation.ID, operation.ExecutionOwner)
		if err != nil {
			return xBackfillConfirmationOperation{}, err
		}
		if tag.RowsAffected() != 1 {
			return xBackfillConfirmationOperation{}, errors.New("X backfill execution lease changed concurrently")
		}
		operation.Status = "failed"
	}
	if operation.Status == "pending" {
		owner, err := newXBackfillExecutionOwner()
		if err != nil {
			return xBackfillConfirmationOperation{}, err
		}
		leaseExpiresAt := now.UTC().Add(xBackfillExecutionLease)
		tag, err := tx.Exec(ctx, `
			UPDATE x_inbox_backfill_confirmation_operations
			SET status = 'running',
			    started_at = NOW(),
			    execution_owner = $2,
			    execution_lease_expires_at = $3,
			    updated_at = NOW()
			WHERE id = $1 AND status = 'pending'
		`, operation.ID, owner, leaseExpiresAt)
		if err != nil {
			return xBackfillConfirmationOperation{}, err
		}
		if tag.RowsAffected() != 1 {
			return xBackfillConfirmationOperation{}, errors.New("X backfill confirmation operation was already consumed")
		}
		operation.Status = "running"
		operation.ExecutionOwner = owner
		operation.ExecutionLease = leaseExpiresAt
		operation.StartedByThisCall = true
	}
	if err := tx.Commit(ctx); err != nil {
		return xBackfillConfirmationOperation{}, err
	}
	return operation, nil
}

func (h *InboxHandler) completeXBackfillConfirmationOperation(
	ctx context.Context,
	id string,
	executionOwner string,
	result any,
	runErr error,
) error {
	if strings.TrimSpace(executionOwner) == "" {
		return errors.New("X backfill execution owner is required")
	}
	raw, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return marshalErr
	}
	status := "completed"
	message := ""
	if runErr != nil {
		status = "failed"
		message = runErr.Error()
	}
	tag, err := h.pool.Exec(ctx, `
		UPDATE x_inbox_backfill_confirmation_operations
		SET status = $2,
		    result = $3,
		    last_error = NULLIF($4, ''),
		    completed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		  AND status = 'running'
		  AND execution_owner = $5
	`, id, status, raw, message, executionOwner)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("X backfill confirmation operation is no longer running")
	}
	return nil
}

func (h *InboxHandler) renewXBackfillExecutionLease(
	ctx context.Context,
	id string,
	executionOwner string,
	now time.Time,
) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(executionOwner) == "" {
		return errors.New("X backfill execution identity is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tag, err := h.pool.Exec(ctx, `
		UPDATE x_inbox_backfill_confirmation_operations
		SET execution_lease_expires_at = $3,
		    updated_at = NOW()
		WHERE id = $1
		  AND status = 'running'
		  AND execution_owner = $2
		  AND execution_lease_expires_at > NOW()
	`, id, executionOwner, now.UTC().Add(xBackfillExecutionLease))
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("X backfill execution lease is no longer valid")
	}
	return nil
}
