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
	Status             string
	Result             json.RawMessage
	ExpiresAt          time.Time
	StartedByThisCall  bool
}

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

func signXBackfillOperationToken(secret []byte, operationID string) (string, error) {
	if len(secret) == 0 || strings.TrimSpace(operationID) == "" {
		return "", errors.New("X backfill confirmation signing secret is not configured")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(operationID))
	return operationID + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func verifyXBackfillOperationToken(secret []byte, token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(secret) == 0 || len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return "", errors.New("invalid X backfill confirmation token")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", errors.New("invalid X backfill confirmation token")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return "", errors.New("invalid X backfill confirmation token")
	}
	return parts[0], nil
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
		base64.RawURLEncoding.EncodeToString(nonceBytes), expiresAt).Scan(&operation.ID)
	if err != nil {
		return xBackfillConfirmationOperation{}, "", err
	}
	token, err := signXBackfillOperationToken(h.xBackfillConfirmationSecret, operation.ID)
	return operation, token, err
}

func (h *InboxHandler) beginXBackfillConfirmationOperation(
	ctx context.Context,
	workspaceID string,
	token string,
	now time.Time,
) (xBackfillConfirmationOperation, error) {
	operationID, err := verifyXBackfillOperationToken(h.xBackfillConfirmationSecret, token)
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
	err = tx.QueryRow(ctx, `
		SELECT id, workspace_id, account_ids, account_fingerprint, request_snapshot,
		       estimated_x_credits, status, COALESCE(result, 'null'::JSONB), expires_at
		FROM x_inbox_backfill_confirmation_operations
		WHERE id = $1
		FOR UPDATE
	`, operationID).Scan(
		&operation.ID, &operation.WorkspaceID, &accountJSON, &operation.AccountFingerprint,
		&requestJSON, &operation.EstimatedXCredits, &operation.Status, &resultJSON, &operation.ExpiresAt,
	)
	if err != nil {
		return xBackfillConfirmationOperation{}, err
	}
	if operation.WorkspaceID != workspaceID {
		return xBackfillConfirmationOperation{}, errors.New("X backfill confirmation operation belongs to another workspace")
	}
	if err := json.Unmarshal(accountJSON, &operation.Accounts); err != nil {
		return xBackfillConfirmationOperation{}, err
	}
	if err := json.Unmarshal(requestJSON, &operation.Request); err != nil {
		return xBackfillConfirmationOperation{}, err
	}
	operation.Result = append([]byte(nil), resultJSON...)
	if !operation.ExpiresAt.After(now.UTC()) &&
		(operation.Status == "pending" || operation.Status == "running") {
		expiredStatus := "expired"
		if operation.Status == "running" {
			expiredStatus = "failed"
		}
		if _, err := tx.Exec(ctx, `
			UPDATE x_inbox_backfill_confirmation_operations
			SET status = $2,
			    last_error = CASE
			      WHEN $2 = 'failed' THEN 'confirmation execution exceeded its deadline'
			      ELSE last_error
			    END,
			    updated_at = NOW()
			WHERE id = $1
		`, operation.ID, expiredStatus); err != nil {
			return xBackfillConfirmationOperation{}, err
		}
		operation.Status = expiredStatus
	}
	if operation.Status == "pending" {
		if _, err := tx.Exec(ctx, `
			UPDATE x_inbox_backfill_confirmation_operations
			SET status = 'running', started_at = NOW(), updated_at = NOW()
			WHERE id = $1 AND status = 'pending'
		`, operation.ID); err != nil {
			return xBackfillConfirmationOperation{}, err
		}
		operation.Status = "running"
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
	result any,
	runErr error,
) error {
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
	_, err := h.pool.Exec(ctx, `
		UPDATE x_inbox_backfill_confirmation_operations
		SET status = $2,
		    result = $3,
		    last_error = NULLIF($4, ''),
		    completed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND status = 'running'
	`, id, status, raw, message)
	return err
}
