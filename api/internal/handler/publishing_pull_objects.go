package handler

import (
	"context"
	"errors"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

type publishingObjectLifecycleQueries interface {
	ReservePublishingPullObjectUsage(context.Context, db.ReservePublishingPullObjectUsageParams) (string, error)
	AbandonPublishingPullObjectUsage(context.Context, db.AbandonPublishingPullObjectUsageParams) error
}

type publishingObjectLifecycleRecorder struct {
	queries publishingObjectLifecycleQueries
}

func (r publishingObjectLifecycleRecorder) ReservePublishingObject(ctx context.Context, reservation storage.PublishingObjectReservation) error {
	if r.queries == nil {
		return errors.New("publishing pull object lifecycle database is not configured")
	}
	_, err := r.queries.ReservePublishingPullObjectUsage(ctx, db.ReservePublishingPullObjectUsageParams{
		WorkspaceID: reservation.WorkspaceID,
		PostID:      reservation.PostID,
		ObjectKey:   reservation.ObjectKey,
		ContentType: reservation.ContentType,
		SizeBytes:   reservation.SizeBytes,
	})
	return err
}

func (r publishingObjectLifecycleRecorder) AbandonPublishingObject(ctx context.Context, reservation storage.PublishingObjectReservation) error {
	if r.queries == nil {
		return errors.New("publishing pull object lifecycle database is not configured")
	}
	return r.queries.AbandonPublishingPullObjectUsage(ctx, db.AbandonPublishingPullObjectUsageParams{
		ObjectKey: reservation.ObjectKey,
		PostID:    reservation.PostID,
	})
}
