package platform

import (
	"context"
	"testing"
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

func TestDispatchMetadataFromNilContextIsAbsent(t *testing.T) {
	if _, ok := DispatchMetadataFromContext(nil); ok {
		t.Fatal("nil context unexpectedly returned metadata")
	}
}
