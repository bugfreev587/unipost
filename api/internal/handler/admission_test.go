package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/ratelimit"
)

// TestApplyRateLimitHeaders locks the X-UniPost-RateLimit-* and
// X-UniPost-QueueDepth header contract. SDKs and dashboards branch
// on these names + formats; a refactor that silently flips the
// reset format from epoch-seconds to seconds-from-now (or drops
// the header on degraded paths) would break client-side backoff
// silently, so this is a contract test, not a coverage check.
func TestApplyRateLimitHeaders(t *testing.T) {
	t.Run("token bucket state populates 3 ratelimit headers", func(t *testing.T) {
		w := httptest.NewRecorder()
		applyRateLimitHeaders(w, ratelimit.Decision{
			Allowed:   true,
			Limit:     20,
			Remaining: 18,
			ResetUnix: 1714330000,
		})
		mustEqual(t, w.Header().Get("X-UniPost-RateLimit-Limit"), "20")
		mustEqual(t, w.Header().Get("X-UniPost-RateLimit-Remaining"), "18")
		mustEqual(t, w.Header().Get("X-UniPost-RateLimit-Reset"), "1714330000")
		mustEqual(t, w.Header().Get("X-UniPost-QueueDepth"), "")
	})

	t.Run("depth state populates queue-depth header", func(t *testing.T) {
		w := httptest.NewRecorder()
		applyRateLimitHeaders(w, ratelimit.Decision{
			Allowed:    true,
			QueueDepth: 47,
			QueueCap:   1000,
		})
		mustEqual(t, w.Header().Get("X-UniPost-QueueDepth"), "47/1000")
		mustEqual(t, w.Header().Get("X-UniPost-RateLimit-Limit"), "")
	})

	t.Run("noop / unpopulated decision writes no headers", func(t *testing.T) {
		w := httptest.NewRecorder()
		applyRateLimitHeaders(w, ratelimit.Decision{Allowed: true})
		for _, h := range []string{
			"X-UniPost-RateLimit-Limit",
			"X-UniPost-RateLimit-Remaining",
			"X-UniPost-RateLimit-Reset",
			"X-UniPost-QueueDepth",
		} {
			if got := w.Header().Get(h); got != "" {
				t.Errorf("%s = %q, want empty for unpopulated Decision", h, got)
			}
		}
	})

	t.Run("zero-tokens-remaining still emits the header", func(t *testing.T) {
		// Edge case: the bucket is empty, the request was denied.
		// We still emit Remaining=0 so the client can confirm the
		// state matches the 429 they just got.
		w := httptest.NewRecorder()
		applyRateLimitHeaders(w, ratelimit.Decision{
			Allowed:   false,
			Limit:     20,
			Remaining: 0,
			ResetUnix: 1714330000,
		})
		mustEqual(t, w.Header().Get("X-UniPost-RateLimit-Remaining"), "0")
	})
}

func mustEqual(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
