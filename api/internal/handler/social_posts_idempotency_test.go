package handler

import (
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/platform"
)

func TestScheduledIdempotencyPayloadHashIgnoresPostOrder(t *testing.T) {
	scheduledAt := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	posts := []platform.PlatformPostInput{
		{AccountID: "sa_threads", Caption: "Launching today", MediaIDs: []string{"media_1"}},
		{AccountID: "sa_linkedin", Caption: "Launching today", MediaURLs: []string{"https://cdn.example.com/a.jpg"}},
	}
	reordered := []platform.PlatformPostInput{posts[1], posts[0]}

	first, err := scheduledIdempotencyPayloadHash(posts, scheduledAt)
	if err != nil {
		t.Fatalf("hash posts: %v", err)
	}
	second, err := scheduledIdempotencyPayloadHash(reordered, scheduledAt)
	if err != nil {
		t.Fatalf("hash reordered posts: %v", err)
	}
	if first != second {
		t.Fatalf("expected same hash for reordered posts, got %q and %q", first, second)
	}
}

func TestScheduledIdempotencyPayloadHashIncludesScheduledAt(t *testing.T) {
	posts := []platform.PlatformPostInput{{AccountID: "sa_threads", Caption: "Launching today"}}

	first, err := scheduledIdempotencyPayloadHash(posts, time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("hash first time: %v", err)
	}
	second, err := scheduledIdempotencyPayloadHash(posts, time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("hash second time: %v", err)
	}
	if first == second {
		t.Fatal("expected different hash when scheduled_at changes")
	}
}

func TestScheduledIdempotencyPayloadHashIncludesCaption(t *testing.T) {
	scheduledAt := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)

	first, err := scheduledIdempotencyPayloadHash([]platform.PlatformPostInput{{AccountID: "sa_threads", Caption: "Launching today"}}, scheduledAt)
	if err != nil {
		t.Fatalf("hash first caption: %v", err)
	}
	second, err := scheduledIdempotencyPayloadHash([]platform.PlatformPostInput{{AccountID: "sa_threads", Caption: "Launching tomorrow"}}, scheduledAt)
	if err != nil {
		t.Fatalf("hash second caption: %v", err)
	}
	if first == second {
		t.Fatal("expected different hash when payload changes")
	}
}
