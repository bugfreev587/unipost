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

	usageID, err := recorder.ReservePublishingObject(context.Background(), reservation)
	if err != nil {
		t.Fatalf("ReservePublishingObject returned error: %v", err)
	}
	if usageID != "usage_1" {
		t.Fatalf("usage ID = %q, want usage_1", usageID)
	}
	if len(queries.reservations) != 1 {
		t.Fatalf("reservations = %d, want 1", len(queries.reservations))
	}
	got := queries.reservations[0]
	if got.ObjectKey != reservation.ObjectKey || got.WorkspaceID != reservation.WorkspaceID || got.PostID != reservation.PostID || got.ContentType != reservation.ContentType || got.SizeBytes != reservation.SizeBytes {
		t.Fatalf("reservation params = %#v", got)
	}

	if err := recorder.AbandonPublishingObject(context.Background(), usageID); err != nil {
		t.Fatalf("AbandonPublishingObject returned error: %v", err)
	}
	if len(queries.abandoned) != 1 || queries.abandoned[0] != usageID {
		t.Fatalf("abandoned params = %#v", queries.abandoned)
	}
}

func TestPublishingObjectLifecycleRecorderPropagatesReservationConflict(t *testing.T) {
	t.Parallel()

	want := errors.New("object is being deleted")
	recorder := publishingObjectLifecycleRecorder{queries: &fakePublishingObjectLifecycleQueries{reserveErr: want}}
	_, err := recorder.ReservePublishingObject(context.Background(), storage.PublishingObjectReservation{
		ObjectKey: "pull/abc.jpg", WorkspaceID: "workspace_1", PostID: "post_1", ContentType: "image/jpeg", SizeBytes: 42,
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

type fakePublishingObjectLifecycleQueries struct {
	reservations []db.ReservePublishingPullObjectUsageParams
	abandoned    []string
	reserveErr   error
	abandonErr   error
}

func (q *fakePublishingObjectLifecycleQueries) ReservePublishingPullObjectUsage(_ context.Context, arg db.ReservePublishingPullObjectUsageParams) (string, error) {
	q.reservations = append(q.reservations, arg)
	return "usage_1", q.reserveErr
}

func (q *fakePublishingObjectLifecycleQueries) AbandonPublishingPullObjectUsage(_ context.Context, usageID string) error {
	q.abandoned = append(q.abandoned, usageID)
	return q.abandonErr
}
