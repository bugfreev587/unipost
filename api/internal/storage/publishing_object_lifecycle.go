package storage

import (
	"context"
	"errors"
	"strings"
)

var ErrExternalMediaLifecycle = errors.New("storage: external media lifecycle unavailable")

// PublishingObjectReservation links a content-addressed pull object to the
// post lifecycle that decides when it may be removed. Source URLs are never
// persisted in this record.
type PublishingObjectReservation struct {
	ObjectKey   string
	WorkspaceID string
	PostID      string
	ContentType string
	SizeBytes   int64
}

// PublishingObjectLifecycle is implemented by the publishing database layer.
// Reserve must be atomic: either both the object and its post usage are
// recorded, or neither is. A reservation starts under a durable pending lease;
// Activate removes that lease after upload, while Abandon makes a failed upload
// immediately eligible for the normal publishing-object cleanup worker.
type PublishingObjectLifecycle interface {
	ReservePublishingObject(context.Context, PublishingObjectReservation) (string, error)
	ActivatePublishingObject(context.Context, string) error
	AbandonPublishingObject(context.Context, string) error
}

type PublishingObjectContext struct {
	WorkspaceID string
	PostID      string
	Lifecycle   PublishingObjectLifecycle
}

type publishingObjectLifecycleContextKey struct{}

func WithPublishingObjectLifecycle(ctx context.Context, metadata PublishingObjectContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, publishingObjectLifecycleContextKey{}, metadata)
}

func publishingObjectContextFromContext(ctx context.Context) (PublishingObjectContext, bool) {
	if ctx == nil {
		return PublishingObjectContext{}, false
	}
	metadata, ok := ctx.Value(publishingObjectLifecycleContextKey{}).(PublishingObjectContext)
	if !ok || metadata.Lifecycle == nil || strings.TrimSpace(metadata.WorkspaceID) == "" || strings.TrimSpace(metadata.PostID) == "" {
		return PublishingObjectContext{}, false
	}
	return metadata, true
}
