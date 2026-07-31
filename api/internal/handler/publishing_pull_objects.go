package handler

import (
	"context"
	"errors"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

type publishingObjectLifecycleQueries interface {
	ReservePublishingPullObjectUsage(context.Context, db.ReservePublishingPullObjectUsageParams) (string, error)
	ActivatePublishingPullObjectUsage(context.Context, string) (string, error)
	AbandonPublishingPullObjectUsage(context.Context, string) error
}

type publishingObjectLifecycleRecorder struct {
	queries publishingObjectLifecycleQueries
}

func (r publishingObjectLifecycleRecorder) ReservePublishingObject(ctx context.Context, reservation storage.PublishingObjectReservation) (string, error) {
	if r.queries == nil {
		return "", errors.New("publishing pull object lifecycle database is not configured")
	}
	return r.queries.ReservePublishingPullObjectUsage(ctx, db.ReservePublishingPullObjectUsageParams{
		WorkspaceID: reservation.WorkspaceID,
		PostID:      reservation.PostID,
		ObjectKey:   reservation.ObjectKey,
		ContentType: reservation.ContentType,
		SizeBytes:   reservation.SizeBytes,
	})
}

func (r publishingObjectLifecycleRecorder) ActivatePublishingObject(ctx context.Context, usageID string) error {
	if r.queries == nil {
		return errors.New("publishing pull object lifecycle database is not configured")
	}
	_, err := r.queries.ActivatePublishingPullObjectUsage(ctx, usageID)
	return err
}

func (r publishingObjectLifecycleRecorder) AbandonPublishingObject(ctx context.Context, usageID string) error {
	if r.queries == nil {
		return errors.New("publishing pull object lifecycle database is not configured")
	}
	return r.queries.AbandonPublishingPullObjectUsage(ctx, usageID)
}
