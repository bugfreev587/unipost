package handler

import (
	"strings"
	"testing"
)

// TestValidateReturnURL — accept https/http with a host, reject everything else.
func TestValidateReturnURL(t *testing.T) {
	cases := []struct {
		in   string
		good bool
	}{
		{"https://app.example.com/done", true},
		{"http://localhost:3000/cb", true},
		{"javascript:alert(1)", false},
		{"data:text/html,<script>", false},
		{"file:///etc/passwd", false},
		{"https://", false},     // no host
		{"not a url", false},    // no scheme
		{"ftp://example.com", false},
	}
	for _, c := range cases {
		err := validateReturnURL(c.in)
		if (err == nil) != c.good {
			t.Errorf("validateReturnURL(%q): want good=%v, got err=%v", c.in, c.good, err)
		}
	}
}

// TestRandomBase64URL — produces unique, URL-safe, padding-free strings
// of approximately the right length for the given byte count.
func TestRandomBase64URL(t *testing.T) {
	a, err := randomBase64URL(32)
	if err != nil {
		t.Fatalf("randomBase64URL err: %v", err)
	}
	b, _ := randomBase64URL(32)
	if a == b {
		t.Error("two calls returned the same string — entropy broken")
	}
	if strings.Contains(a, "=") || strings.Contains(a, "+") || strings.Contains(a, "/") {
		t.Errorf("expected URL-safe encoding without padding, got %q", a)
	}
	// 32 bytes → ceil(32*4/3) = 43 base64url chars (no padding).
	if len(a) != 43 {
		t.Errorf("32-byte input should yield 43 chars, got %d (%q)", len(a), a)
	}
	// 64 bytes → 86 chars.
	v, _ := randomBase64URL(64)
	if len(v) != 86 {
		t.Errorf("64-byte input should yield 86 chars, got %d", len(v))
	}
}

// TestBuildHostedURL — shape of the URL handed back to customers.
func TestBuildHostedURL(t *testing.T) {
	h := &ConnectSessionHandler{dashboardURL: "https://app.unipost.dev"}
	got := h.buildHostedURL("twitter", "sess_abc", "state-xyz")
	want := "https://app.unipost.dev/connect/twitter?session=sess_abc&state=state-xyz"
	if got != want {
		t.Errorf("buildHostedURL: got %q, want %q", got, want)
	}

	// Trailing slash on the dashboard URL must not produce a double slash.
	h2 := &ConnectSessionHandler{dashboardURL: "https://app.unipost.dev/"}
	got = h2.buildHostedURL("bluesky", "s", "state")
	if strings.Contains(got, "dev//connect") {
		t.Errorf("trailing slash produced double slash: %q", got)
	}
}

// TestConnectablePlatforms locks the Sprint 3 platform allowlist.
func TestConnectablePlatforms(t *testing.T) {
	for _, p := range []string{"twitter", "linkedin", "bluesky"} {
		if !connectablePlatforms[p] {
			t.Errorf("%s should be connectable in Sprint 3", p)
		}
	}
	for _, p := range []string{"instagram", "tiktok", "youtube", "threads", "facebook"} {
		if connectablePlatforms[p] {
			t.Errorf("%s should NOT be connectable until App Review unlocks it", p)
		}
	}
}
