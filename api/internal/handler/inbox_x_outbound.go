package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/inboxaccess"
	"github.com/xiaoboyu/unipost-api/internal/xcredits"
)

const (
	xInboxOutboundCompletionTimeout = 10 * time.Second
	xInboxOutcomeUnknownTimeout     = 30 * time.Minute
)

var errXInboxStateTransitionConflict = errors.New("X Inbox state transition conflict")
var errXInboxOutboundOutsideScope = errors.New("X Inbox outbound request is unavailable for this Inbox scope")

func detachedXInboxCompletionContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), xInboxOutboundCompletionTimeout)
}

func (h *InboxHandler) recordXInboxRemoteSuccess(
	ctx context.Context,
	requestID string,
	result xInboxSendResult,
) error {
	updated, err := h.queries.RecordXInboxOutboundRemoteSuccess(ctx, db.RecordXInboxOutboundRemoteSuccessParams{
		UsageEventID:         result.UsageEventID,
		OperationKey:         result.Operation,
		ReservedUnits:        result.XCreditsCounted,
		RemoteExternalID:     pgtype.Text{String: result.ExternalID, Valid: result.ExternalID != ""},
		RemoteConversationID: result.ConversationID,
		RemoteUrl:            result.URL,
		ID:                   requestID,
	})
	if err != nil {
		return err
	}
	if updated != 1 {
		return errXInboxStateTransitionConflict
	}
	return nil
}

func retryXInboxStatePersistence(ctx context.Context, persist func() error) error {
	delay := 50 * time.Millisecond
	for {
		err := persist()
		if err == nil || errors.Is(err, errXInboxStateTransitionConflict) {
			return err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(err, ctx.Err())
		case <-timer.C:
		}
		if delay < 500*time.Millisecond {
			delay *= 2
		}
	}
}

func (h *InboxHandler) completeKnownXInboxOutbound(
	ctx context.Context,
	requestID string,
) (db.InboxItem, xInboxSendResult, error) {
	if h == nil || h.pool == nil || h.encryptor == nil {
		return db.InboxItem{}, xInboxSendResult{}, errors.New("X Inbox outbound completion is not configured")
	}
	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.InboxItem{}, xInboxSendResult{}, err
	}
	return h.completeKnownXInboxOutboundWithTx(ctx, requestID, tx)
}

func (h *InboxHandler) completeKnownXInboxOutboundWithTx(
	ctx context.Context,
	requestID string,
	tx pgx.Tx,
) (db.InboxItem, xInboxSendResult, error) {
	defer tx.Rollback(ctx)
	scope, ok := inboxaccess.FromContext(ctx)
	if !ok || !validInboxAccessScope(scope) {
		return db.InboxItem{}, xInboxSendResult{}, errXInboxOutboundOutsideScope
	}
	workspaceScope := scope.WorkspaceWide()
	externalUserID := scope.ExternalUserID
	queries := db.New(tx)
	outbound, err := queries.GetXInboxOutboundRequestByIDForUpdate(ctx, requestID)
	if err != nil {
		return db.InboxItem{}, xInboxSendResult{}, err
	}
	if outbound.WorkspaceID != scope.WorkspaceID {
		return db.InboxItem{}, xInboxSendResult{}, errXInboxOutboundOutsideScope
	}
	if outbound.Status == "completed" || outbound.Status == "succeeded" {
		if !outbound.ResponseInboxItemID.Valid {
			return db.InboxItem{}, xInboxSendResult{}, errors.New("completed X Inbox outbound request is missing response item")
		}
		item, loadErr := queries.GetInboxItem(ctx, db.GetInboxItemParams{
			ID:             outbound.ResponseInboxItemID.String,
			WorkspaceID:    outbound.WorkspaceID,
			WorkspaceScope: workspaceScope,
			ExternalUserID: externalUserID,
		})
		if loadErr != nil {
			return db.InboxItem{}, xInboxSendResult{}, loadErr
		}
		return item, xInboxResultFromOutbound(outbound), tx.Commit(ctx)
	}
	if outbound.Status != "remote_succeeded" || !outbound.RemoteExternalID.Valid {
		return db.InboxItem{}, xInboxSendResult{}, fmt.Errorf(
			"X Inbox outbound request %s is not ready for completion", requestID,
		)
	}
	if !outbound.EncryptedPayload.Valid {
		return db.InboxItem{}, xInboxSendResult{}, errors.New("X Inbox outbound request is missing encrypted payload")
	}
	text, err := h.encryptor.Decrypt(outbound.EncryptedPayload.String)
	if err != nil {
		return db.InboxItem{}, xInboxSendResult{}, fmt.Errorf("decrypt X Inbox outbound payload: %w", err)
	}
	target, err := queries.GetInboxItem(ctx, db.GetInboxItemParams{
		ID:             outbound.InboxItemID,
		WorkspaceID:    outbound.WorkspaceID,
		WorkspaceScope: workspaceScope,
		ExternalUserID: externalUserID,
	})
	if err != nil {
		return db.InboxItem{}, xInboxSendResult{}, err
	}
	account, err := queries.GetSocialAccountByIDAndWorkspace(ctx, db.GetSocialAccountByIDAndWorkspaceParams{
		ID: outbound.SocialAccountID, WorkspaceID: outbound.WorkspaceID,
	})
	if err != nil {
		return db.InboxItem{}, xInboxSendResult{}, err
	}
	result := xInboxResultFromOutbound(outbound)
	result.BillingMode = account.XAppMode.String
	parentID := target.ParentExternalID
	threadKey := target.ThreadKey
	if target.Source == "x_reply" {
		parentID = pgtype.Text{String: target.ExternalID, Valid: true}
	} else if result.ConversationID != "" {
		parentID = pgtype.Text{String: result.ConversationID, Valid: true}
		threadKey = result.ConversationID
	} else if !parentID.Valid {
		parentID = pgtype.Text{String: target.ExternalID, Valid: true}
	}
	metadata, _ := json.Marshal(map[string]any{
		"idempotency_key":          outbound.IdempotencyKey,
		"reply_to_inbox_item_id":   target.ID,
		"conversation_id":          result.ConversationID,
		"permalink":                result.URL,
		"x_credits_counted":        result.XCreditsCounted,
		"x_credit_operation":       result.Operation,
		"x_credit_catalog_version": result.CatalogVersion,
		"x_credit_billing_mode":    result.BillingMode,
	})
	replyItem, err := queries.UpsertInboxItem(ctx, db.UpsertInboxItemParams{
		SocialAccountID:  target.SocialAccountID,
		WorkspaceID:      outbound.WorkspaceID,
		Source:           target.Source,
		ExternalID:       result.ExternalID,
		ParentExternalID: parentID,
		AuthorName:       pgtype.Text{String: account.AccountName.String, Valid: account.AccountName.Valid},
		AuthorID: pgtype.Text{
			String: firstNonEmptyString(account.ExternalUserID.String, account.ExternalAccountID),
			Valid:  true,
		},
		Body:         pgtype.Text{String: text, Valid: true},
		IsOwn:        true,
		ReceivedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		Metadata:     metadata,
		ThreadKey:    threadKey,
		ThreadStatus: target.ThreadStatus,
		AssignedTo:   target.AssignedTo,
		LinkedPostID: target.LinkedPostID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		replyItem, err = queries.GetInboxItemByExternalID(ctx, db.GetInboxItemByExternalIDParams{
			SocialAccountID: target.SocialAccountID,
			ExternalID:      result.ExternalID,
		})
	}
	if err != nil {
		return db.InboxItem{}, xInboxSendResult{}, err
	}
	if outbound.UsageEventID.Valid {
		if err := finalizeXUsageInTx(ctx, tx, outbound.UsageEventID.String, outbound.ReservedUnits); err != nil {
			return db.InboxItem{}, xInboxSendResult{}, err
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE x_inbox_outbound_requests
		SET status = 'completed',
		    response_inbox_item_id = $2,
		    last_error = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND status = 'remote_succeeded'
	`, requestID, replyItem.ID); err != nil {
		return db.InboxItem{}, xInboxSendResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.InboxItem{}, xInboxSendResult{}, err
	}
	return replyItem, result, nil
}

func finalizeXUsageInTx(ctx context.Context, tx pgx.Tx, eventID string, finalUnits int64) error {
	var workspaceID, status string
	var start, end time.Time
	var currentUnits int64
	err := tx.QueryRow(ctx, `
		SELECT workspace_id, period_start, period_end, weighted_units, status
		FROM x_usage_events
		WHERE id = $1
		FOR UPDATE
	`, eventID).Scan(&workspaceID, &start, &end, &currentUnits, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if status == xcredits.UsageStatusFinalized {
		return nil
	}
	if status != xcredits.UsageStatusProvisional {
		return fmt.Errorf("cannot finalize X usage event in status %s", status)
	}
	if finalUnits > currentUnits {
		return errors.New("final X usage cannot exceed provisional usage")
	}
	if delta := currentUnits - finalUnits; delta > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE x_usage_periods
			SET weighted_units_used = weighted_units_used - $4, updated_at = NOW()
			WHERE workspace_id = $1 AND period_start = $2 AND period_end = $3
		`, workspaceID, start, end, delta); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `
		UPDATE x_usage_events
		SET status = 'finalized', weighted_units = $2, updated_at = NOW()
		WHERE id = $1
	`, eventID, finalUnits)
	return err
}

func xInboxResultFromOutbound(outbound db.XInboxOutboundRequest) xInboxSendResult {
	return xInboxSendResult{
		ExternalID:      outbound.RemoteExternalID.String,
		ConversationID:  outbound.RemoteConversationID.String,
		URL:             outbound.RemoteUrl.String,
		XCreditsCounted: outbound.ReservedUnits,
		Operation:       outbound.OperationKey.String,
		CatalogVersion:  xcredits.CatalogVersion,
		UsageEventID:    outbound.UsageEventID.String,
	}
}
