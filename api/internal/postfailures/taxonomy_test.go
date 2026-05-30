package postfailures

import "testing"

func TestClassifyKnownPublishFailures(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		code      string
		retriable bool
	}{
		{
			name:      "tiktok file format",
			raw:       "tiktok publish failed: file_format_check_failed",
			code:      "media_error",
			retriable: false,
		},
		{
			name:      "tiktok invalid params",
			raw:       `tiktok photo init (400): {"error":{"code":"invalid_params","message":"Invalid authorization header. Please check the format."}}`,
			code:      "validation_error",
			retriable: false,
		},
		{
			name:      "youtube upload quota",
			raw:       `youtube upload init failed (429): {"error":{"status":"RESOURCE_EXHAUSTED","errors":[{"reason":"rateLimitExceeded"}],"message":"The request cannot be completed because you have exceeded your quota. Video Uploads per day"}}`,
			code:      "quota_exceeded",
			retriable: false,
		},
		{
			name:      "threads invalid token",
			raw:       `threads get user id failed (401): {"error":{"message":"Invalid OAuth access token"}}`,
			code:      "account_reconnect_required",
			retriable: false,
		},
		{
			name:      "threads missing permission",
			raw:       `threads get user id failed (403): {"error":{"message":"Missing required permission threads_basic"}}`,
			code:      "missing_permission",
			retriable: false,
		},
		{
			name:      "instagram timeout",
			raw:       "instagram container processing timed out: container_id=178900 poll_count=30 elapsed_ms=60000",
			code:      "temporary_platform_error",
			retriable: true,
		},
		{
			name:      "instagram container error",
			raw:       "instagram container processing failed: container_id=178900 status_code=ERROR poll_count=3 elapsed_ms=6000",
			code:      "media_error",
			retriable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.raw)
			if got.ErrorCode != tt.code {
				t.Fatalf("ErrorCode = %q, want %q", got.ErrorCode, tt.code)
			}
			if got.IsRetriable != tt.retriable {
				t.Fatalf("IsRetriable = %v, want %v", got.IsRetriable, tt.retriable)
			}
		})
	}
}
