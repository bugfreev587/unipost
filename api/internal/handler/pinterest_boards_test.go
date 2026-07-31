package handler

import (
	"testing"
)

type pinterestBoardEndpointTestError struct {
	message  string
	provider map[string]any
	failure  map[string]any
}

func (e pinterestBoardEndpointTestError) Error() string { return e.message }
func (e pinterestBoardEndpointTestError) ProviderErrorFields() map[string]any {
	return e.provider
}
func (e pinterestBoardEndpointTestError) FailureContractFields() map[string]any {
	return e.failure
}

func TestClassifyPinterestBoardEndpointError(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		wantStatus    int
		wantCode      string
		wantReconnect bool
	}{
		{
			name: "invalid token",
			err: pinterestBoardEndpointTestError{
				message:  "sanitized token failure",
				provider: map[string]any{"provider": "pinterest", "http_status": 401, "code": "2", "reason": "token_invalid"},
				failure:  map[string]any{"error_code": "auth_token_invalid", "is_retriable": false},
			},
			wantStatus: 409, wantCode: "NEEDS_RECONNECT", wantReconnect: true,
		},
		{
			name: "missing scope",
			err: pinterestBoardEndpointTestError{
				message:  "sanitized permission failure",
				provider: map[string]any{"provider": "pinterest", "http_status": 403, "code": "29", "reason": "missing_permission"},
				failure:  map[string]any{"error_code": "missing_permission", "is_retriable": false},
			},
			wantStatus: 409, wantCode: "MISSING_PERMISSION", wantReconnect: false,
		},
		{
			name: "resource code 29",
			err: pinterestBoardEndpointTestError{
				message:  "sanitized target failure",
				provider: map[string]any{"provider": "pinterest", "http_status": 403, "code": "29", "reason": "board_not_accessible"},
				failure:  map[string]any{"error_code": "target_not_found", "is_retriable": false},
			},
			wantStatus: 409, wantCode: "PINTEREST_BOARD_UNAVAILABLE", wantReconnect: false,
		},
		{
			name: "rate limit",
			err: pinterestBoardEndpointTestError{
				message:  "sanitized temporary failure",
				provider: map[string]any{"provider": "pinterest", "http_status": 429, "reason": "rate_limited"},
				failure:  map[string]any{"error_code": "rate_limit", "is_retriable": true},
			},
			wantStatus: 503, wantCode: "PINTEREST_TEMPORARY", wantReconnect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyPinterestBoardEndpointError(tt.err)
			if got.status != tt.wantStatus || got.code != tt.wantCode || got.reconnect != tt.wantReconnect {
				t.Fatalf("classification = %#v", got)
			}
			if got.message == "" || got.message == tt.err.Error() {
				t.Fatalf("message = %q, want fixed customer copy", got.message)
			}
		})
	}
}

func TestLooksLikePinterestAuthErrorDoesNotTreatResource403AsReconnect(t *testing.T) {
	resourceErr := pinterestBoardEndpointTestError{
		message:  "resource denied",
		provider: map[string]any{"provider": "pinterest", "http_status": 403, "code": "29", "reason": "board_not_accessible"},
		failure:  map[string]any{"error_code": "target_not_found"},
	}
	if looksLikePinterestAuthError(resourceErr) {
		t.Fatal("resource-level code 29 must not require reconnect")
	}

	tokenErr := pinterestBoardEndpointTestError{
		message:  "token denied",
		provider: map[string]any{"provider": "pinterest", "http_status": 401, "code": "2", "reason": "token_invalid"},
		failure:  map[string]any{"error_code": "auth_token_invalid"},
	}
	if !looksLikePinterestAuthError(tokenErr) {
		t.Fatal("token code 2 should require reconnect")
	}
}
