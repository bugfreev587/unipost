package postfailures

import (
	"testing"
)

type pinterestContractTestError struct {
	message  string
	provider map[string]any
	failure  map[string]any
}

func (e pinterestContractTestError) Error() string { return e.message }

func (e pinterestContractTestError) ProviderErrorFields() map[string]any {
	return e.provider
}

func (e pinterestContractTestError) FailureContractFields() map[string]any {
	return e.failure
}

func TestClassifyKnownPublishFailures(t *testing.T) {
	tests := []struct {
		name              string
		raw               string
		code              string
		source            string
		temporality       string
		platformErrorCode string
		provider          string
		providerCode      string
		providerReason    string
		httpStatus        int
		isTransient       *bool
		retriable         bool
	}{
		{
			name:        "tiktok file format",
			raw:         "tiktok publish failed: file_format_check_failed",
			code:        "media_error",
			source:      ErrorSourcePlatform,
			temporality: ErrorTemporalityPermanent,
			retriable:   false,
		},
		{
			name:              "tiktok invalid params",
			raw:               `TikTok rejected the photo publish request: TikTok reported invalid_params. Common fixes: keep photo captions/titles to 90 characters or fewer and use SELF_ONLY if the app is still in sandbox mode. (provider_error=invalid_params, status=400)`,
			code:              "platform_request_invalid",
			source:            ErrorSourcePlatform,
			temporality:       ErrorTemporalityPermanent,
			platformErrorCode: "invalid_params",
			provider:          "tiktok",
			providerCode:      "invalid_params",
			httpStatus:        400,
			retriable:         false,
		},
		{
			name:           "youtube upload quota",
			raw:            `youtube upload init failed (429): {"error":{"status":"RESOURCE_EXHAUSTED","errors":[{"domain":"youtube.quota","reason":"rateLimitExceeded"}],"message":"The request cannot be completed because you have exceeded your quota. Video Uploads per day"}} quota_scope=platform_project provider=youtube provider_reason=rateLimitExceeded quota_limit=defaultVideoInsertPerDayPerProject`,
			code:           "quota_exceeded",
			source:         ErrorSourcePlatform,
			temporality:    ErrorTemporalityPermanent,
			provider:       "youtube",
			providerReason: "rateLimitExceeded",
			httpStatus:     429,
			retriable:      false,
		},
		{
			name:        "managed X monthly allowance",
			raw:         "x_monthly_usage_limit_exceeded: managed X usage has reached this billing period's allowance",
			code:        "x_monthly_usage_limit_exceeded",
			source:      ErrorSourceUnipost,
			temporality: ErrorTemporalityPermanent,
			retriable:   false,
		},
		{
			name:        "threads invalid token",
			raw:         `threads get user id failed (401): {"error":{"message":"Invalid OAuth access token"}}`,
			code:        "account_reconnect_required",
			source:      ErrorSourcePlatform,
			temporality: ErrorTemporalityPermanent,
			retriable:   false,
		},
		{
			name:              "threads oauth 190 expired token",
			raw:               `threads get user id failed (400): {"error":{"message":"Error validating access token: Session has expired on Sunday, 10-May-26 10:00:00 PDT.","type":"OAuthException","code":190,"error_subcode":0}}`,
			code:              "account_reconnect_required",
			source:            ErrorSourcePlatform,
			temporality:       ErrorTemporalityPermanent,
			platformErrorCode: "190",
			provider:          "meta",
			providerCode:      "190",
			httpStatus:        400,
			retriable:         false,
		},
		{
			name:              "instagram get user id oauth 190 login challenge",
			raw:               `instagram get user id failed (400): {"error":{"message":"The user must log in again.","type":"OAuthException","code":190,"error_subcode":460}}`,
			code:              "account_reconnect_required",
			source:            ErrorSourcePlatform,
			temporality:       ErrorTemporalityPermanent,
			platformErrorCode: "190",
			provider:          "meta",
			providerCode:      "190",
			httpStatus:        400,
			retriable:         false,
		},
		{
			name:              "meta oauth 190 refresh expired token",
			raw:               `refresh failed (400): {"error":{"message":"Error validating access token: Session has expired on Sunday, 10-May-26 10:00:00 PDT.","type":"OAuthException","code":190,"error_subcode":0}}`,
			code:              "account_reconnect_required",
			source:            ErrorSourcePlatform,
			temporality:       ErrorTemporalityPermanent,
			platformErrorCode: "190",
			provider:          "meta",
			providerCode:      "190",
			httpStatus:        400,
			retriable:         false,
		},
		{
			name:        "threads missing permission",
			raw:         `threads get user id failed (403): {"error":{"message":"Missing required permission threads_basic"}}`,
			code:        "missing_permission",
			source:      ErrorSourcePlatform,
			temporality: ErrorTemporalityPermanent,
			retriable:   false,
		},
		{
			name:         "instagram transient media publish oauth code 2",
			raw:          `publish failed (500): {"error":{"message":"An unexpected error has occurred. Please retry your request later.","type":"OAuthException","is_transient":true,"code":2,"fbtrace_id":"AJ4uhascsOC2cf1lq0bwhgJ"}}`,
			code:         "temporary_platform_error",
			source:       ErrorSourcePlatform,
			temporality:  ErrorTemporalityTemporary,
			provider:     "meta",
			providerCode: "2",
			httpStatus:   500,
			isTransient:  boolPtr(true),
			retriable:    true,
		},
		{
			name:         "instagram transient flag without retry wording",
			raw:          `publish failed (500): {"error":{"message":"An unexpected error has occurred.","type":"OAuthException","is_transient":true,"code":2,"fbtrace_id":"TRACE"}}`,
			code:         "temporary_platform_error",
			source:       ErrorSourcePlatform,
			temporality:  ErrorTemporalityTemporary,
			provider:     "meta",
			providerCode: "2",
			httpStatus:   500,
			isTransient:  boolPtr(true),
			retriable:    true,
		},
		{
			name:         "meta retry later wording",
			raw:          `publish failed (500): {"error":{"message":"Please retry your request later.","type":"OAuthException","code":2}}`,
			code:         "temporary_platform_error",
			source:       ErrorSourcePlatform,
			temporality:  ErrorTemporalityTemporary,
			provider:     "meta",
			providerCode: "2",
			httpStatus:   500,
			retriable:    true,
		},
		{
			name:        "instagram timeout",
			raw:         "instagram container processing timed out: container_id=178900 poll_count=30 elapsed_ms=60000",
			code:        "temporary_platform_error",
			source:      ErrorSourcePlatform,
			temporality: ErrorTemporalityTemporary,
			retriable:   true,
		},
		{
			name:        "instagram container error",
			raw:         "instagram container processing failed: container_id=178900 status_code=ERROR poll_count=3 elapsed_ms=6000",
			code:        "media_error",
			source:      ErrorSourcePlatform,
			temporality: ErrorTemporalityPermanent,
			retriable:   false,
		},
		{
			name:        "local validation",
			raw:         "validation failed: caption is required",
			code:        "validation_error",
			source:      ErrorSourceUnipost,
			temporality: ErrorTemporalityPermanent,
			retriable:   false,
		},
		{
			name:        "X write outcome pending reconciliation",
			raw:         "x_write_outcome_pending_reconciliation: the prior X write outcome is unknown",
			code:        "x_write_outcome_pending_reconciliation",
			source:      ErrorSourcePlatform,
			temporality: ErrorTemporalityUnknown,
			retriable:   false,
		},
		{
			name:        "unknown empty",
			raw:         "",
			code:        "unknown_error",
			source:      ErrorSourceUnknown,
			temporality: ErrorTemporalityUnknown,
			retriable:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.raw)
			if got.ErrorCode != tt.code {
				t.Fatalf("ErrorCode = %q, want %q", got.ErrorCode, tt.code)
			}
			if got.ErrorSource != tt.source {
				t.Fatalf("ErrorSource = %q, want %q", got.ErrorSource, tt.source)
			}
			if got.ErrorTemporality != tt.temporality {
				t.Fatalf("ErrorTemporality = %q, want %q", got.ErrorTemporality, tt.temporality)
			}
			if tt.platformErrorCode != "" && got.PlatformErrorCode != tt.platformErrorCode {
				t.Fatalf("PlatformErrorCode = %q, want %q", got.PlatformErrorCode, tt.platformErrorCode)
			}
			if tt.provider != "" {
				if got.ProviderError == nil {
					t.Fatalf("ProviderError = nil, want provider %q", tt.provider)
				}
				if got.ProviderError.Provider != tt.provider {
					t.Fatalf("ProviderError.Provider = %q, want %q", got.ProviderError.Provider, tt.provider)
				}
			}
			if tt.providerCode != "" {
				if got.ProviderError == nil || got.ProviderError.Code != tt.providerCode {
					t.Fatalf("ProviderError.Code = %#v, want %q", got.ProviderError, tt.providerCode)
				}
			}
			if tt.providerReason != "" {
				if got.ProviderError == nil || got.ProviderError.Reason != tt.providerReason {
					t.Fatalf("ProviderError.Reason = %#v, want %q", got.ProviderError, tt.providerReason)
				}
			}
			if tt.httpStatus != 0 {
				if got.ProviderError == nil || got.ProviderError.HTTPStatus != tt.httpStatus {
					t.Fatalf("ProviderError.HTTPStatus = %#v, want %d", got.ProviderError, tt.httpStatus)
				}
			}
			if tt.isTransient != nil {
				if got.ProviderError == nil || got.ProviderError.IsTransient == nil || *got.ProviderError.IsTransient != *tt.isTransient {
					t.Fatalf("ProviderError.IsTransient = %#v, want %v", got.ProviderError, *tt.isTransient)
				}
			}
			if got.IsRetriable != tt.retriable {
				t.Fatalf("IsRetriable = %v, want %v", got.IsRetriable, tt.retriable)
			}
		})
	}
}

func boolPtr(v bool) *bool {
	return &v
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
		{code: "x_monthly_usage_limit_exceeded", want: "review_quota"},
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

func TestDeriveLegacyContractFromStoredFields(t *testing.T) {
	tests := []struct {
		name        string
		errorCode   string
		message     string
		retriable   bool
		source      string
		temporality string
	}{
		{
			name:        "temporary platform code without new columns",
			errorCode:   "temporary_platform_error",
			retriable:   true,
			source:      ErrorSourcePlatform,
			temporality: ErrorTemporalityTemporary,
		},
		{
			name:        "worker stalled code without new columns",
			errorCode:   "worker_stalled",
			retriable:   true,
			source:      ErrorSourceWorker,
			temporality: ErrorTemporalityTemporary,
		},
		{
			name:        "unknown code remains honest",
			errorCode:   "unknown_error",
			retriable:   false,
			source:      ErrorSourceUnknown,
			temporality: ErrorTemporalityUnknown,
		},
		{
			name:        "missing code falls back to message parse",
			message:     `publish failed (500): {"error":{"message":"Please retry your request later.","type":"OAuthException","code":2}}`,
			retriable:   false,
			source:      ErrorSourcePlatform,
			temporality: ErrorTemporalityTemporary,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveLegacyContract(tt.errorCode, tt.message, tt.retriable)
			if got.ErrorSource != tt.source {
				t.Fatalf("ErrorSource = %q, want %q", got.ErrorSource, tt.source)
			}
			if got.ErrorTemporality != tt.temporality {
				t.Fatalf("ErrorTemporality = %q, want %q", got.ErrorTemporality, tt.temporality)
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

func TestBuildParamsFromErrorUsesTypedPinterestContract(t *testing.T) {
	tests := []struct {
		name             string
		err              error
		wantCode         string
		wantProviderCode string
		wantReason       string
		wantHTTPStatus   int
		wantRetriable    bool
		wantTemporality  string
		wantNextAction   string
	}{
		{
			name: "destination code 29",
			err: pinterestContractTestError{
				message: "The selected Pinterest board is unavailable for this connected account.",
				provider: map[string]any{
					"provider": "pinterest", "http_status": 403, "code": "29",
					"reason": "board_not_accessible", "is_transient": false,
				},
				failure: map[string]any{
					"error_code": "target_not_found", "failure_stage": "destination_preflight", "is_retriable": false,
				},
			},
			wantCode: "target_not_found", wantProviderCode: "29", wantReason: "board_not_accessible",
			wantHTTPStatus: 403, wantRetriable: false, wantTemporality: ErrorTemporalityPermanent,
			wantNextAction: "select_valid_target",
		},
		{
			name: "rate limit",
			err: pinterestContractTestError{
				message: "Pinterest is rate limiting this request.",
				provider: map[string]any{
					"provider": "pinterest", "http_status": 429, "code": "8", "reason": "rate_limited", "is_transient": true,
				},
				failure: map[string]any{
					"error_code": "rate_limit", "failure_stage": "destination_preflight", "is_retriable": true,
				},
			},
			wantCode: "rate_limit", wantProviderCode: "8", wantReason: "rate_limited", wantHTTPStatus: 429,
			wantRetriable: true, wantTemporality: ErrorTemporalityTemporary, wantNextAction: "wait_and_retry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := BuildParamsFromError("post_1", "result_1", "ws_1", "sa_1", "pinterest", "dispatch", tt.err, "")
			if params.ErrorCode != tt.wantCode {
				t.Fatalf("ErrorCode = %q, want %q", params.ErrorCode, tt.wantCode)
			}
			if params.PlatformErrorCode.String != tt.wantProviderCode {
				t.Fatalf("PlatformErrorCode = %q, want %q", params.PlatformErrorCode.String, tt.wantProviderCode)
			}
			if params.IsRetriable != tt.wantRetriable {
				t.Fatalf("IsRetriable = %v, want %v", params.IsRetriable, tt.wantRetriable)
			}
			if params.ErrorSource.String != ErrorSourcePlatform {
				t.Fatalf("ErrorSource = %q, want %q", params.ErrorSource.String, ErrorSourcePlatform)
			}
			if params.ErrorTemporality.String != tt.wantTemporality {
				t.Fatalf("ErrorTemporality = %q, want %q", params.ErrorTemporality.String, tt.wantTemporality)
			}
			if got := NextActionForErrorCode(params.ErrorCode); got != tt.wantNextAction {
				t.Fatalf("next action = %q, want %q", got, tt.wantNextAction)
			}
			provider := ParseProviderErrorJSON(params.ProviderError)
			if provider == nil {
				t.Fatal("ProviderError = nil")
			}
			if provider.Provider != "pinterest" || provider.HTTPStatus != tt.wantHTTPStatus || provider.Code != tt.wantProviderCode || provider.Reason != tt.wantReason {
				t.Fatalf("ProviderError = %#v", provider)
			}
		})
	}
}

func TestClassifyLegacyPinterestFailures(t *testing.T) {
	tests := []struct {
		name             string
		raw              string
		wantCode         string
		wantProviderCode string
		wantReason       string
		wantRetriable    bool
	}{
		{
			name:     "board inaccessible",
			raw:      "pinterest destination_preflight failed status=403 code=29 provider_reason=board_not_accessible",
			wantCode: "target_not_found", wantProviderCode: "29", wantReason: "board_not_accessible",
		},
		{
			name:     "stored create pin code 29 incident",
			raw:      `pinterest create pin (403): {"code":29,"message":"You are not permitted to access that resource."}`,
			wantCode: "target_not_found", wantProviderCode: "29", wantReason: "board_not_accessible",
		},
		{
			name:     "board not found",
			raw:      "pinterest destination_preflight failed status=404 code=40 provider_reason=board_not_found",
			wantCode: "target_not_found", wantProviderCode: "40", wantReason: "board_not_found",
		},
		{
			name:     "token invalid",
			raw:      "pinterest user_account failed status=401 code=2 provider_reason=token_invalid",
			wantCode: "auth_token_invalid", wantProviderCode: "2", wantReason: "token_invalid",
		},
		{
			name:     "rate limited",
			raw:      "pinterest destination_preflight failed status=429 provider_reason=rate_limited",
			wantCode: "rate_limit", wantProviderCode: "rate_limited", wantReason: "rate_limited", wantRetriable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.raw)
			if got.ErrorCode != tt.wantCode || got.PlatformErrorCode != tt.wantProviderCode || got.IsRetriable != tt.wantRetriable {
				t.Fatalf("classification = %#v", got)
			}
			if got.ProviderError == nil || got.ProviderError.Provider != "pinterest" || got.ProviderError.Reason != tt.wantReason {
				t.Fatalf("ProviderError = %#v", got.ProviderError)
			}
		})
	}
}
