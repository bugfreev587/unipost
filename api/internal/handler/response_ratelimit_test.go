package handler

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// TestWriteRateLimited locks the 429 response shape and headers so a
// future refactor cannot accidentally drop the Retry-After header or
// flip the normalized_code semantics. The dashboard and SDK clients
// branch on normalized_code (rate_limited vs enqueue_rate_limited
// vs queue_depth_exceeded) so this is a contract test, not a mere
// sanity check.
func TestWriteRateLimited(t *testing.T) {
	cases := []struct {
		name           string
		normalizedCode string
		retryAfter     time.Duration
		wantRetry      string
	}{
		{"request_limited", "rate_limited", 5 * time.Second, "5"},
		{"enqueue_limited", "enqueue_rate_limited", 12 * time.Second, "12"},
		{"depth_exceeded", "queue_depth_exceeded", 30 * time.Second, "30"},
		{"min_clamp", "rate_limited", 250 * time.Millisecond, "1"}, // sub-second clamps to 1
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeRateLimited(w, tc.normalizedCode, "slow down", tc.retryAfter)

			if w.Code != 429 {
				t.Fatalf("status = %d, want 429", w.Code)
			}
			if got := w.Header().Get("Retry-After"); got != tc.wantRetry {
				t.Fatalf("Retry-After = %q, want %q", got, tc.wantRetry)
			}
			retrySecs, err := strconv.Atoi(w.Header().Get("Retry-After"))
			if err != nil || retrySecs < 1 {
				t.Fatalf("Retry-After must be a positive int, got %q (err=%v)",
					w.Header().Get("Retry-After"), err)
			}

			var got struct {
				Error struct {
					Code           string `json:"code"`
					NormalizedCode string `json:"normalized_code"`
					Message        string `json:"message"`
					Hint           string `json:"hint"`
					NextAction     string `json:"next_action"`
					IsRetriable    *bool  `json:"is_retriable"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if got.Error.Code != "RATE_LIMITED" {
				t.Errorf("error.code = %q, want RATE_LIMITED", got.Error.Code)
			}
			if got.Error.NormalizedCode != tc.normalizedCode {
				t.Errorf("error.normalized_code = %q, want %q", got.Error.NormalizedCode, tc.normalizedCode)
			}
			if got.Error.Message != "slow down" {
				t.Errorf("error.message = %q, want %q", got.Error.Message, "slow down")
			}
			if got.Error.Hint == "" {
				t.Errorf("error.hint should tell the caller to wait before retrying")
			}
			if got.Error.NextAction != "wait_and_retry" {
				t.Errorf("error.next_action = %q, want wait_and_retry", got.Error.NextAction)
			}
			if got.Error.IsRetriable == nil || !*got.Error.IsRetriable {
				t.Errorf("error.is_retriable = %#v, want explicit true", got.Error.IsRetriable)
			}
		})
	}
}
