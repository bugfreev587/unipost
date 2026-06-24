package postfailures

import "testing"

func TestClassifyKnownPublishFailures(t *testing.T) {
	tests := []struct {
		name              string
		raw               string
		code              string
		platformErrorCode string
		retriable         bool
	}{
		{
			name:      "tiktok file format",
			raw:       "tiktok publish failed: file_format_check_failed",
			code:      "media_error",
			retriable: false,
		},
		{
			name:              "tiktok invalid params",
			raw:               `TikTok rejected the photo publish request: TikTok reported invalid_params. Common fixes: keep photo captions/titles to 90 characters or fewer and use SELF_ONLY if the app is still in sandbox mode. (provider_error=invalid_params, status=400)`,
			code:              "platform_request_invalid",
			platformErrorCode: "invalid_params",
			retriable:         false,
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
			name:      "threads oauth 190 expired token",
			raw:       `threads get user id failed (400): {"error":{"message":"Error validating access token: Session has expired on Sunday, 10-May-26 10:00:00 PDT.","type":"OAuthException","code":190,"error_subcode":0}}`,
			code:      "account_reconnect_required",
			retriable: false,
		},
		{
			name:      "meta oauth 190 refresh expired token",
			raw:       `refresh failed (400): {"error":{"message":"Error validating access token: Session has expired on Sunday, 10-May-26 10:00:00 PDT.","type":"OAuthException","code":190,"error_subcode":0}}`,
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
			name:      "instagram transient media publish oauth code 2",
			raw:       `publish failed (500): {"error":{"message":"An unexpected error has occurred. Please retry your request later.","type":"OAuthException","is_transient":true,"code":2,"fbtrace_id":"AJ4uhascsOC2cf1lq0bwhgJ"}}`,
			code:      "temporary_platform_error",
			retriable: true,
		},
		{
			name:      "instagram transient flag without retry wording",
			raw:       `publish failed (500): {"error":{"message":"An unexpected error has occurred.","type":"OAuthException","is_transient":true,"code":2,"fbtrace_id":"TRACE"}}`,
			code:      "temporary_platform_error",
			retriable: true,
		},
		{
			name:      "meta retry later wording",
			raw:       `publish failed (500): {"error":{"message":"Please retry your request later.","type":"OAuthException","code":2}}`,
			code:      "temporary_platform_error",
			retriable: true,
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
			if tt.platformErrorCode != "" && got.PlatformErrorCode != tt.platformErrorCode {
				t.Fatalf("PlatformErrorCode = %q, want %q", got.PlatformErrorCode, tt.platformErrorCode)
			}
			if got.IsRetriable != tt.retriable {
				t.Fatalf("IsRetriable = %v, want %v", got.IsRetriable, tt.retriable)
			}
		})
	}
}

func TestNextActionForErrorCode(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{code: "validation_error", want: "fix_request"},
		{code: "media_error", want: "fix_media"},
		{code: "temporary_platform_error", want: "retry_later"},
		{code: "rate_limit", want: "wait_and_retry"},
		{code: "account_reconnect_required", want: "reconnect_account"},
		{code: "missing_permission", want: "reconnect_or_update_permissions"},
		{code: "target_not_found", want: "select_valid_target"},
		{code: "platform_error", want: "contact_support"},
		{code: "", want: ""},
		{code: "new_future_code", want: "contact_support"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if got := NextActionForErrorCode(tt.code); got != tt.want {
				t.Fatalf("NextActionForErrorCode(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestShouldMarkReconnectRequired(t *testing.T) {
	if !ShouldMarkReconnectRequired(`refresh failed (400): {"error":{"message":"Error validating access token: Session has expired","type":"OAuthException","code":190}}`) {
		t.Fatal("expected Meta OAuth 190 refresh failure to require reconnect")
	}
	if ShouldMarkReconnectRequired(`refresh failed (500): {"error":{"message":"temporarily unavailable"}}`) {
		t.Fatal("temporary platform refresh failure should not require reconnect")
	}
}
