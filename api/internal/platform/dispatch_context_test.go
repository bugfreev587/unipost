package platform

import (
	"context"
	"testing"
	"time"
)

func TestDispatchMetadataRoundTrip(t *testing.T) {
	type unrelatedKey struct{}
	base := context.WithValue(context.Background(), unrelatedKey{}, "kept")
	ctx := WithDispatchMetadata(base, DispatchMetadata{
		SocialAccountID: "sa_pin_1",
		Environment:     "sandbox",
	})

	got, ok := DispatchMetadataFromContext(ctx)
	if !ok {
		t.Fatal("dispatch metadata missing")
	}
	if got.SocialAccountID != "sa_pin_1" || got.Environment != "sandbox" {
		t.Fatalf("dispatch metadata = %#v", got)
	}
	if ctx.Value(unrelatedKey{}) != "kept" {
		t.Fatal("unrelated context value was not preserved")
	}
}

func TestDispatchEventRecorderIsBoundedAndSnapshotsByValue(t *testing.T) {
	recorder := NewDispatchEventRecorder()
	for i := 0; i < 20; i++ {
		recorder.Record(DispatchEvent{Name: "event", Duration: time.Duration(i)})
	}
	events := recorder.Snapshot()
	if len(events) != 16 {
		t.Fatalf("event count = %d, want 16", len(events))
	}
	events[0].Name = "mutated"
	if recorder.Snapshot()[0].Name != "event" {
		t.Fatal("snapshot mutated recorder state")
	}
}

func TestDispatchMetadataFromNilContextIsAbsent(t *testing.T) {
	if _, ok := DispatchMetadataFromContext(nil); ok {
		t.Fatal("nil context unexpectedly returned metadata")
	}
}
