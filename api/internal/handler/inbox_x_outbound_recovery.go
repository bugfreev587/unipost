package handler

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type XInboxOutboundReconcileStats struct {
	Scanned             int `json:"scanned"`
	Completed           int `json:"completed"`
	Deferred            int `json:"deferred"`
	NeedsReconciliation int `json:"needs_reconciliation"`
}

type xInboxOutboundRecoveryStore interface {
	ListRecoverable(context.Context, int32) ([]db.XInboxOutboundRequest, error)
	CompleteKnown(context.Context, string) error
	Defer(context.Context, string, time.Time, string) error
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
