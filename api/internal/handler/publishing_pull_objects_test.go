package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

func TestPublishingObjectLifecycleRecorderPersistsAndAbandonsReservation(t *testing.T) {
	t.Parallel()

	queries := &fakePublishingObjectLifecycleQueries{}
	recorder := publishingObjectLifecycleRecorder{queries: queries}
	reservation := storage.PublishingObjectReservation{
		ObjectKey:   "pull/abc.jpg",
		WorkspaceID: "workspace_1",
		PostID:      "post_1",
		ContentType: "image/jpeg",
		SizeBytes:   42,
	}

	if err := recorder.ReservePublishingObject(context.Background(), reservation); err != nil {
		t.Fatalf("ReservePublishingObject returned error: %v", err)
	}
	if len(queries.reservations) != 1 {
		t.Fatalf("reservations = %d, want 1", len(queries.reservations))
	}
	got := queries.reservations[0]
	if got.ObjectKey != reservation.ObjectKey || got.WorkspaceID != reservation.WorkspaceID || got.PostID != reservation.PostID || got.ContentType != reservation.ContentType || got.SizeBytes != reservation.SizeBytes {
		t.Fatalf("reservation params = %#v", got)
	}

	if err := recorder.AbandonPublishingObject(context.Background(), reservation); err != nil {
		t.Fatalf("AbandonPublishingObject returned error: %v", err)
	}
	if len(queries.abandoned) != 1 || queries.abandoned[0].ObjectKey != reservation.ObjectKey || queries.abandoned[0].PostID != reservation.PostID {
		t.Fatalf("abandoned params = %#v", queries.abandoned)
	}
}

func TestPublishingObjectLifecycleRecorderPropagatesReservationConflict(t *testing.T) {
	t.Parallel()

	want := errors.New("object is being deleted")
	recorder := publishingObjectLifecycleRecorder{queries: &fakePublishingObjectLifecycleQueries{reserveErr: want}}
	err := recorder.ReservePublishingObject(context.Background(), storage.PublishingObjectReservation{
		ObjectKey: "pull/abc.jpg", WorkspaceID: "workspace_1", PostID: "post_1", ContentType: "image/jpeg", SizeBytes: 42,
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

type fakePublishingObjectLifecycleQueries struct {
	reservations []db.ReservePublishingPullObjectUsageParams
	abandoned    []db.AbandonPublishingPullObjectUsageParams
	reserveErr   error
	abandonErr   error
}

func (q *fakePublishingObjectLifecycleQueries) ReservePublishingPullObjectUsage(_ context.Context, arg db.ReservePublishingPullObjectUsageParams) (string, error) {
	q.reservations = append(q.reservations, arg)
	return arg.ObjectKey, q.reserveErr
}

func (q *fakePublishingObjectLifecycleQueries) AbandonPublishingPullObjectUsage(_ context.Context, arg db.AbandonPublishingPullObjectUsageParams) error {
	q.abandoned = append(q.abandoned, arg)
	return q.abandonErr
}
