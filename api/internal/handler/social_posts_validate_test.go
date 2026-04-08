package handler

import (
	"testing"
)

// TestParsePublishRequest_Legacy exercises the legacy expansion path:
// caption + account_ids becomes one PlatformPostInput per account.
func TestParsePublishRequest_Legacy(t *testing.T) {
	body := publishRequestBody{
		Caption:    "shipped",
		AccountIDs: []string{"a", "b", "c"},
		MediaURLs:  []string{"https://x/y.jpg"},
	}
	pr, status, msg := parsePublishRequest(body)
	if status != 0 {
		t.Fatalf("expected ok, got %d: %s", status, msg)
	}
	if len(pr.Posts) != 3 {
		t.Fatalf("expected 3 posts, got %d", len(pr.Posts))
	}
	for i, p := range pr.Posts {
		if p.Caption != "shipped" {
			t.Errorf("post %d caption mismatch: %q", i, p.Caption)
		}
		if len(p.MediaURLs) != 1 {
			t.Errorf("post %d should inherit media", i)
		}
	}
}

// TestParsePublishRequest_New exercises the new platform_posts shape.
func TestParsePublishRequest_New(t *testing.T) {
	body := publishRequestBody{
		PlatformPosts: []platformPostBody{
			{AccountID: "a", Caption: "tweet"},
			{AccountID: "b", Caption: "linkedin", MediaURLs: []string{"https://x/y.jpg"}},
		},
	}
	pr, status, _ := parsePublishRequest(body)
	if status != 0 {
		t.Fatalf("expected ok, got %d", status)
	}
	if len(pr.Posts) != 2 {
		t.Fatalf("expected 2, got %d", len(pr.Posts))
	}
	if pr.Posts[0].Caption != "tweet" || pr.Posts[1].Caption != "linkedin" {
		t.Errorf("captions mismatched: %#v", pr.Posts)
	}
}

// TestParsePublishRequest_MutuallyExclusive — passing BOTH shapes
// should return a structured error rather than silently picking one.
func TestParsePublishRequest_MutuallyExclusive(t *testing.T) {
	body := publishRequestBody{
		Caption:    "x",
		AccountIDs: []string{"a"},
		PlatformPosts: []platformPostBody{
			{AccountID: "b", Caption: "y"},
		},
	}
	_, status, msg := parsePublishRequest(body)
	if status == 0 {
		t.Fatal("expected error")
	}
	if msg == "" {
		t.Error("expected error message")
	}
}

// TestParsePublishRequest_Empty — passing neither shape should error.
func TestParsePublishRequest_Empty(t *testing.T) {
	_, status, _ := parsePublishRequest(publishRequestBody{})
	if status == 0 {
		t.Fatal("expected error")
	}
}

// TestParsePublishRequest_PerPostScheduledForbidden — per the v1
// contract in §3.1, only the top-level scheduled_at is allowed.
func TestParsePublishRequest_PerPostScheduledForbidden(t *testing.T) {
	when := "2026-04-08T10:00:00Z"
	body := publishRequestBody{
		PlatformPosts: []platformPostBody{
			{AccountID: "a", Caption: "x", ScheduledAt: &when},
		},
	}
	_, status, msg := parsePublishRequest(body)
	if status == 0 {
		t.Fatal("expected error")
	}
	if msg == "" {
		t.Error("expected message")
	}
}

// TestParsePublishRequest_TopLevelScheduledOK — same field is allowed
// at the top level.
func TestParsePublishRequest_TopLevelScheduledOK(t *testing.T) {
	when := "2026-04-08T10:00:00Z"
	body := publishRequestBody{
		ScheduledAt: &when,
		PlatformPosts: []platformPostBody{
			{AccountID: "a", Caption: "x"},
		},
	}
	pr, status, _ := parsePublishRequest(body)
	if status != 0 {
		t.Fatalf("expected ok, got %d", status)
	}
	if pr.ScheduledAt == nil {
		t.Error("expected scheduled_at to be parsed")
	}
}

// TestParsePublishRequest_InvalidScheduled — non-RFC3339 should error.
func TestParsePublishRequest_InvalidScheduled(t *testing.T) {
	when := "next tuesday"
	body := publishRequestBody{
		ScheduledAt: &when,
		AccountIDs:  []string{"a"},
		Caption:     "x",
	}
	_, status, _ := parsePublishRequest(body)
	if status == 0 {
		t.Fatal("expected error")
	}
}

// TestParsePublishRequest_PassesIdempotencyKey — the key should
// survive the parse so the publish path can use it for collision
// detection.
func TestParsePublishRequest_PassesIdempotencyKey(t *testing.T) {
	body := publishRequestBody{
		AccountIDs:     []string{"a"},
		Caption:        "x",
		IdempotencyKey: "abc123",
	}
	pr, status, _ := parsePublishRequest(body)
	if status != 0 {
		t.Fatalf("expected ok, got %d", status)
	}
	if pr.IdempotencyKey != "abc123" {
		t.Errorf("expected idempotency_key to survive parse, got %q", pr.IdempotencyKey)
	}
}
