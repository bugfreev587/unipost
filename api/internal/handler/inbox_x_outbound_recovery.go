package handler

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type XInboxOutboundReconcileStats struct {
	Scanned             int `json:"scanned"`
	Completed           int `json:"completed"`
	UsageReversed       int `json:"usage_reversed"`
	Deferred            int `json:"deferred"`
	NeedsReconciliation int `json:"needs_reconciliation"`
}

type xInboxOutboundRecoveryStore interface {
	ListRecoverable(context.Context, int32) ([]db.XInboxOutboundRequest, error)
	CompleteKnown(context.Context, string) error
	ReverseUsage(context.Context, db.XInboxOutboundRequest) error
	Defer(context.Context, string, time.Time, string) error
	DeferUsageReversal(context.Context, string, time.Time, string) error
	MarkNeedsReconciliation(context.Context, string, string) error
}

type XInboxOutboundRecoveryService struct {
	store xInboxOutboundRecoveryStore
	now   func() time.Time
}

func NewXInboxOutboundRecoveryService(handler *InboxHandler) *XInboxOutboundRecoveryService {
	return &XInboxOutboundRecoveryService{
		store: postgresXInboxOutboundRecoveryStore{handler: handler},
		now:   time.Now,
	}
}

func newXInboxOutboundRecoveryService(
	store xInboxOutboundRecoveryStore,
	now func() time.Time,
) *XInboxOutboundRecoveryService {
	if now == nil {
		now = time.Now
	}
	return &XInboxOutboundRecoveryService{store: store, now: now}
}

func (s *XInboxOutboundRecoveryService) ProcessOnce(
	ctx context.Context,
) (XInboxOutboundReconcileStats, error) {
	stats := XInboxOutboundReconcileStats{}
	if s == nil || s.store == nil {
		return stats, nil
	}
	rows, err := s.store.ListRecoverable(ctx, 100)
	if err != nil {
		return stats, err
	}
	now := s.now().UTC()
	stats.Scanned = len(rows)
	for _, row := range rows {
		if row.Status == "pending" && !row.EncryptedPayload.Valid {
			if err := s.store.MarkNeedsReconciliation(
				ctx,
				row.ID,
				"Legacy X Inbox write claim requires manual reconciliation",
			); err != nil {
				stats.Deferred++
				continue
			}
			stats.NeedsReconciliation++
			continue
		}
		if row.Status == "usage_reversal_pending" {
			if err := s.store.ReverseUsage(ctx, row); err != nil {
				stats.Deferred++
				_ = s.store.DeferUsageReversal(ctx, row.ID, now.Add(time.Minute), err.Error())
				continue
			}
			stats.UsageReversed++
			continue
		}
		if row.Status == "remote_succeeded" && row.RemoteExternalID.Valid {
			if err := s.store.CompleteKnown(ctx, row.ID); err != nil {
				stats.Deferred++
				_ = s.store.Defer(ctx, row.ID, now.Add(time.Minute), err.Error())
				continue
			}
			stats.Completed++
			continue
		}
		if row.ReconciliationDeadline.Valid && !row.ReconciliationDeadline.Time.After(now) {
			if err := s.store.MarkNeedsReconciliation(
				ctx,
				row.ID,
				"X write outcome remained unknown past the automatic reconciliation deadline",
			); err != nil {
				stats.Deferred++
				continue
			}
			stats.NeedsReconciliation++
		}
	}
	return stats, nil
}

type postgresXInboxOutboundRecoveryStore struct {
	handler *InboxHandler
}

func (s postgresXInboxOutboundRecoveryStore) ListRecoverable(
	ctx context.Context,
	limit int32,
) ([]db.XInboxOutboundRequest, error) {
	return s.handler.queries.ListRecoverableXInboxOutboundRequests(ctx, limit)
}

func (s postgresXInboxOutboundRecoveryStore) CompleteKnown(ctx context.Context, id string) error {
	_, _, err := s.handler.completeKnownXInboxOutbound(ctx, id)
	return err
}

func (s postgresXInboxOutboundRecoveryStore) ReverseUsage(
	ctx context.Context,
	row db.XInboxOutboundRequest,
) error {
	if !row.UsageEventID.Valid {
		return errors.New("X Inbox usage reversal is missing its usage event")
	}
	if s.handler == nil || s.handler.xCredits == nil {
		return errors.New("X Inbox credits service is not configured")
	}
	if err := s.handler.xCredits.Reverse(ctx, row.UsageEventID.String); err != nil {
		return err
	}
	deleted, err := s.handler.queries.DeleteXInboxOutboundAfterUsageReversal(ctx, row.ID)
	if err != nil {
		return err
	}
	if deleted != 1 {
		return errors.New("X Inbox usage reversal state was not removed")
	}
	return nil
}

func (s postgresXInboxOutboundRecoveryStore) Defer(
	ctx context.Context,
	id string,
	next time.Time,
	message string,
) error {
	return s.handler.queries.DeferXInboxOutboundCompletion(ctx, db.DeferXInboxOutboundCompletionParams{
		NextAttemptAt: pgtype.Timestamptz{Time: next, Valid: true},
		LastError:     message,
		ID:            id,
	})
}

func (s postgresXInboxOutboundRecoveryStore) DeferUsageReversal(
	ctx context.Context,
	id string,
	next time.Time,
	message string,
) error {
	return s.handler.queries.DeferXInboxUsageReversal(ctx, db.DeferXInboxUsageReversalParams{
		NextAttemptAt: pgtype.Timestamptz{Time: next, Valid: true},
		LastError:     message,
		ID:            id,
	})
}

func (s postgresXInboxOutboundRecoveryStore) MarkNeedsReconciliation(
	ctx context.Context,
	id string,
	message string,
) error {
	return s.handler.queries.MarkXInboxOutboundNeedsReconciliation(
		ctx,
		db.MarkXInboxOutboundNeedsReconciliationParams{LastError: message, ID: id},
	)
}
