package handler

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestIPLimiter — basic burst + window behavior.
func TestIPLimiter(t *testing.T) {
	l := newIPLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !l.Allow("1.1.1.1") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	if l.Allow("1.1.1.1") {
		t.Error("4th attempt should be denied")
	}
	// Different IP gets its own bucket.
	if !l.Allow("2.2.2.2") {
		t.Error("different IP should be allowed")
	}
}

// TestIPLimiter_Window — entries older than the window are dropped.
func TestIPLimiter_Window(t *testing.T) {
	l := newIPLimiter(2, 100*time.Millisecond)
	l.Allow("ip")
	l.Allow("ip")
	if l.Allow("ip") {
		t.Fatal("should be at limit")
	}
	time.Sleep(120 * time.Millisecond)
	if !l.Allow("ip") {
		t.Error("after window, attempt should succeed")
	}
}

// TestClientIP — XFF handling, then RemoteAddr fallback.
func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("POST", "/", nil)
	r.RemoteAddr = "10.0.0.5:1234"
	if got := clientIP(r); got != "10.0.0.5:1234" {
		t.Errorf("no XFF: got %q", got)
	}

	r.Header.Set("X-Forwarded-For", "203.0.113.7")
	if got := clientIP(r); got != "203.0.113.7" {
		t.Errorf("single XFF: got %q", got)
	}

	r.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1, 10.0.0.2")
	if got := clientIP(r); got != "203.0.113.7" {
		t.Errorf("XFF chain: got %q", got)
	}
}

// TestBlueskyTemplate_NoPasswordEcho — sanity check that the form
// template never includes the password field's value attribute, even
// if blueskyTplData were to have one. Locks the credential-handling
// invariant via a string check on the template source.
func TestBlueskyTemplate_NoPasswordEcho(t *testing.T) {
	for _, line := range strings.Split(blueskyResultTplSrc, "\n") {
		if strings.Contains(line, `name="app_password"`) {
			if strings.Contains(line, "value=") {
				t.Errorf("password input should never carry a value attribute: %q", line)
			}
		}
	}
}
