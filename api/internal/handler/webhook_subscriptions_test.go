package handler

import (
	"strings"
	"testing"
)

// TestGenerateWebhookSecret_Format locks down the whsec_ prefix +
// hex-suffix shape so a future refactor can't accidentally weaken it.
func TestGenerateWebhookSecret_Format(t *testing.T) {
	for i := 0; i < 5; i++ {
		s, err := generateWebhookSecret()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if !strings.HasPrefix(s, "whsec_") {
			t.Errorf("expected whsec_ prefix, got %q", s)
		}
		if len(s) != len("whsec_")+32 {
			t.Errorf("expected 38 chars (whsec_ + 32 hex), got %d: %q", len(s), s)
		}
		// Hex chars only after the prefix.
		body := s[len("whsec_"):]
		for _, c := range body {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("non-hex char %q in body of %q", c, s)
			}
		}
	}
}

// TestGenerateWebhookSecret_Unique sanity-checks that two consecutive
// generations don't collide. crypto/rand makes this near-impossible
// but the test guards against accidentally seeding from time.
func TestGenerateWebhookSecret_Unique(t *testing.T) {
	a, _ := generateWebhookSecret()
	b, _ := generateWebhookSecret()
	if a == b {
		t.Errorf("two generations should not collide: %q == %q", a, b)
	}
}

// TestSecretPreview never leaks more than the first 8 chars + ellipsis.
// 8 chars = "whsec_" (6) + first 2 hex chars of the body, just enough
// to disambiguate visually but useless for forging a signature.
func TestSecretPreview(t *testing.T) {
	full := "whsec_abcdef0123456789abcdef0123456789ab"
	got := secretPreview(full)
	if got != "whsec_ab…" {
		t.Errorf("expected 'whsec_ab…', got %q", got)
	}
	short := "tiny"
	if secretPreview(short) != "tiny" {
		t.Errorf("short secrets should pass through unchanged, got %q", secretPreview(short))
	}
}

// TestEventForStatus locks down the post-status → event-name mapping
// shared between the immediate path and the scheduler. If this map
// drifts, webhook subscribers stop receiving the events they
// expected.
func TestEventForStatus(t *testing.T) {
	cases := map[string]string{
		"published": "post.published",
		"partial":   "post.partial",
		"failed":    "post.failed",
		"unknown":   "post.failed", // safe default
	}
	for status, want := range cases {
		if got := eventForStatus(status); got != want {
			t.Errorf("eventForStatus(%q) = %q, want %q", status, got, want)
		}
	}
}
