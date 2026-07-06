package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/postfailures"
)

func TestNormalizeErrorCode(t *testing.T) {
	tests := map[string]string{
		"VALIDATION_ERROR":                   "validation_error",
		"UNAUTHORIZED":                       "unauthorized",
		"NEEDS_RECONNECT":                    "needs_reconnect",
		"QUEUE_JOB_ACTIVE":                   "queue_job_active",
		"PLAN_SCHEDULED_POST_LIMIT_EXCEEDED": "plan_scheduled_post_limit_exceeded",
		"SOME_FUTURE_ERROR_CODE":             "some_future_error_code",
	}

	for input, want := range tests {
		if got := normalizeErrorCode(input); got != want {
			t.Fatalf("normalizeErrorCode(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestWriteSuccessContract(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-Id", "req_success")

	writeSuccess(rr, map[string]any{"id": "acc_123"})

	if rr.Code != http.StatusOK {
		t.Fatalf("writeSuccess status = %d, want 200", rr.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got["request_id"] != "req_success" {
		t.Fatalf("request_id = %#v, want req_success", got["request_id"])
	}
	if _, ok := got["meta"]; ok {
		t.Fatalf("writeSuccess should omit meta, got %#v", got["meta"])
	}
}

func TestWriteSuccessWithListMetaContract(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-Id", "req_list")

	writeSuccessWithListMeta(rr, []string{"a", "b"}, 27, 10)

	if rr.Code != http.StatusOK {
		t.Fatalf("writeSuccessWithListMeta status = %d, want 200", rr.Code)
	}

	var got struct {
		Meta struct {
			Total float64 `json:"total"`
			Limit float64 `json:"limit"`
		} `json:"meta"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Meta.Total != 27 || got.Meta.Limit != 10 {
		t.Fatalf("meta = %#v, want total=27 limit=10", got.Meta)
	}
	if got.RequestID != "req_list" {
		t.Fatalf("request_id = %q, want req_list", got.RequestID)
	}
}

func TestWriteSuccessWithCursorContract(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-Id", "req_cursor")

	writeSuccessWithCursor(rr, []string{"post_1"}, "cursor_2", true, 25)

	if rr.Code != http.StatusOK {
		t.Fatalf("writeSuccessWithCursor status = %d, want 200", rr.Code)
	}

	var got struct {
		Meta struct {
			Limit      float64 `json:"limit"`
			HasMore    bool    `json:"has_more"`
			NextCursor string  `json:"next_cursor"`
		} `json:"meta"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Meta.Limit != 25 || !got.Meta.HasMore || got.Meta.NextCursor != "cursor_2" {
		t.Fatalf("meta = %#v, want limit=25 has_more=true next_cursor=cursor_2", got.Meta)
	}
	if got.RequestID != "req_cursor" {
		t.Fatalf("request_id = %q, want req_cursor", got.RequestID)
	}
}

func TestWriteSuccessWithLegacyCursorContract(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-Id", "req_legacy")

	writeSuccessWithLegacyCursor(rr, []string{"post_1"}, "cursor_2", true, 25)

	if rr.Code != http.StatusOK {
		t.Fatalf("writeSuccessWithLegacyCursor status = %d, want 200", rr.Code)
	}

	var got struct {
		Meta struct {
			NextCursor string `json:"next_cursor"`
		} `json:"meta"`
		NextCursor string `json:"next_cursor"`
		RequestID  string `json:"request_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Meta.NextCursor != "cursor_2" || got.NextCursor != "cursor_2" {
		t.Fatalf("cursor fields = meta:%q top:%q, want both cursor_2", got.Meta.NextCursor, got.NextCursor)
	}
	if got.RequestID != "req_legacy" {
		t.Fatalf("request_id = %q, want req_legacy", got.RequestID)
	}
}

func TestWriteAcceptedContract(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-Id", "req_accepted")

	writeAccepted(rr, map[string]any{"id": "post_123"})

	if rr.Code != http.StatusAccepted {
		t.Fatalf("writeAccepted status = %d, want 202", rr.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got["request_id"] != "req_accepted" {
		t.Fatalf("request_id = %#v, want req_accepted", got["request_id"])
	}
}

func TestWriteErrorContract(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-Id", "req_error")

	writeError(rr, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "bad input")

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("writeError status = %d, want 422", rr.Code)
	}

	var got struct {
		Error struct {
			Code           string `json:"code"`
			NormalizedCode string `json:"normalized_code"`
			Message        string `json:"message"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Error.Code != "VALIDATION_ERROR" || got.Error.NormalizedCode != "validation_error" || got.Error.Message != "bad input" {
		t.Fatalf("error body = %#v, want validation error contract", got.Error)
	}
	if got.RequestID != "req_error" {
		t.Fatalf("request_id = %q, want req_error", got.RequestID)
	}
}

func TestWriteErrorWithDetailsContract(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-Id", "req_detailed_error")
	isRetriable := false

	writeErrorWithDetails(rr, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "bad input", ErrorDetails{
		Hint:             "Fix the listed fields and retry.",
		NextAction:       "fix_request",
		IsRetriable:      &isRetriable,
		DocsURL:          "https://unipost.dev/docs/api/posts/validate",
		ErrorSource:      postfailures.ErrorSourceUnipost,
		ErrorTemporality: postfailures.ErrorTemporalityPermanent,
		ProviderError: &postfailures.ProviderError{
			Provider:   "meta",
			HTTPStatus: 400,
			Code:       "100",
		},
		RetryPolicy: &retryPolicyResponse{
			IsRetriable:        false,
			WillRetry:          false,
			RetryState:         "not_retriable",
			ManualRetryAllowed: false,
			Reason:             "classification_not_retriable",
		},
	})

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("writeErrorWithDetails status = %d, want 422", rr.Code)
	}

	var got struct {
		Error struct {
			Code             string `json:"code"`
			NormalizedCode   string `json:"normalized_code"`
			Message          string `json:"message"`
			Hint             string `json:"hint"`
			NextAction       string `json:"next_action"`
			IsRetriable      *bool  `json:"is_retriable"`
			DocsURL          string `json:"docs_url"`
			ErrorSource      string `json:"error_source"`
			ErrorTemporality string `json:"error_temporality"`
			ProviderError    struct {
				Provider   string `json:"provider"`
				HTTPStatus int    `json:"http_status"`
				Code       string `json:"code"`
			} `json:"provider_error"`
			RetryPolicy struct {
				IsRetriable        bool   `json:"is_retriable"`
				WillRetry          bool   `json:"will_retry"`
				RetryState         string `json:"retry_state"`
				ManualRetryAllowed bool   `json:"manual_retry_allowed"`
				Reason             string `json:"reason"`
			} `json:"retry_policy"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Error.Code != "VALIDATION_ERROR" || got.Error.NormalizedCode != "validation_error" || got.Error.Message != "bad input" {
		t.Fatalf("error body = %#v, want validation error contract", got.Error)
	}
	if got.Error.Hint != "Fix the listed fields and retry." || got.Error.NextAction != "fix_request" {
		t.Fatalf("remediation fields = hint:%q next_action:%q, want actionable values", got.Error.Hint, got.Error.NextAction)
	}
	if got.Error.IsRetriable == nil || *got.Error.IsRetriable {
		t.Fatalf("is_retriable = %#v, want explicit false", got.Error.IsRetriable)
	}
	if got.Error.DocsURL != "https://unipost.dev/docs/api/posts/validate" {
		t.Fatalf("docs_url = %q, want validation docs URL", got.Error.DocsURL)
	}
	if got.Error.ErrorSource != "unipost" || got.Error.ErrorTemporality != "permanent" {
		t.Fatalf("source/temporality = %q/%q, want unipost/permanent", got.Error.ErrorSource, got.Error.ErrorTemporality)
	}
	if got.Error.ProviderError.Provider != "meta" || got.Error.ProviderError.HTTPStatus != 400 || got.Error.ProviderError.Code != "100" {
		t.Fatalf("provider_error = %#v", got.Error.ProviderError)
	}
	if got.Error.RetryPolicy.RetryState != "not_retriable" || got.Error.RetryPolicy.WillRetry {
		t.Fatalf("retry_policy = %#v", got.Error.RetryPolicy)
	}
	if got.RequestID != "req_detailed_error" {
		t.Fatalf("request_id = %q, want req_detailed_error", got.RequestID)
	}
}

func TestWriteErrorSanitizesInternalServerDetails(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-Id", "req_internal")

	writeError(rr, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load user: pq: password authentication failed for user app")

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("writeError status = %d, want 500", rr.Code)
	}

	var got struct {
		Error struct {
			Code           string `json:"code"`
			NormalizedCode string `json:"normalized_code"`
			Message        string `json:"message"`
			Hint           string `json:"hint"`
			NextAction     string `json:"next_action"`
			DocsURL        string `json:"docs_url"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Error.Code != "INTERNAL_ERROR" || got.Error.NormalizedCode != "internal_error" {
		t.Fatalf("error identifiers = %#v, want internal error contract", got.Error)
	}
	if got.Error.Message == "" || got.Error.Message == "Failed to load user: pq: password authentication failed for user app" {
		t.Fatalf("message = %q, want sanitized customer-safe copy", got.Error.Message)
	}
	if got.Error.Message == "" || got.Error.Hint == "" || got.Error.NextAction != "contact_support" {
		t.Fatalf("remediation = message:%q hint:%q next_action:%q, want actionable sanitized response", got.Error.Message, got.Error.Hint, got.Error.NextAction)
	}
	if got.Error.DocsURL != "https://unipost.dev/docs/api/errors" {
		t.Fatalf("docs_url = %q, want API errors docs", got.Error.DocsURL)
	}
	if got.RequestID != "req_internal" {
		t.Fatalf("request_id = %q, want req_internal", got.RequestID)
	}
}

func TestWriteErrorSanitizesUpstreamDetails(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-Id", "req_upstream")

	writeError(rr, http.StatusBadGateway, "TIKTOK_ERROR", `tiktok profile failed: {"error":"Invalid authorization header","log_id":"abc"}`)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("writeError status = %d, want 502", rr.Code)
	}

	var got struct {
		Error struct {
			Code           string `json:"code"`
			NormalizedCode string `json:"normalized_code"`
			Message        string `json:"message"`
			Hint           string `json:"hint"`
			NextAction     string `json:"next_action"`
			DocsURL        string `json:"docs_url"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Error.Code != "TIKTOK_ERROR" || got.Error.NormalizedCode != "tiktok_error" {
		t.Fatalf("error identifiers = %#v, want TikTok error contract", got.Error)
	}
	if got.Error.Message == "" || got.Error.Message == `tiktok profile failed: {"error":"Invalid authorization header","log_id":"abc"}` {
		t.Fatalf("message = %q, want sanitized upstream copy", got.Error.Message)
	}
	if got.Error.Hint == "" || got.Error.NextAction != "wait_and_retry" {
		t.Fatalf("remediation = hint:%q next_action:%q, want retry guidance", got.Error.Hint, got.Error.NextAction)
	}
	if got.Error.DocsURL != "https://unipost.dev/docs/api/errors" {
		t.Fatalf("docs_url = %q, want API errors docs", got.Error.DocsURL)
	}
	if got.RequestID != "req_upstream" {
		t.Fatalf("request_id = %q, want req_upstream", got.RequestID)
	}
}

func TestWriteValidationErrorsContract(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-Id", "req_validation")

	writeValidationErrors(rr, []platform.Issue{
		{
			PlatformPostIndex: 0,
			AccountID:         "acc_instagram",
			Platform:          "instagram",
			Field:             "media_ids",
			Code:              platform.CodeMediaNotUploaded,
			Message:           "media pending",
			Severity:          platform.SeverityError,
		},
	})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("writeValidationErrors status = %d, want 400", rr.Code)
	}

	var got struct {
		Error struct {
			Code           string           `json:"code"`
			NormalizedCode string           `json:"normalized_code"`
			Message        string           `json:"message"`
			Hint           string           `json:"hint"`
			NextAction     string           `json:"next_action"`
			IsRetriable    *bool            `json:"is_retriable"`
			DocsURL        string           `json:"docs_url"`
			Issues         []platform.Issue `json:"issues"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Error.Code != "VALIDATION_ERROR" || got.Error.NormalizedCode != "validation_error" {
		t.Fatalf("error identifiers = %#v, want validation error contract", got.Error)
	}
	if got.RequestID != "req_validation" {
		t.Fatalf("request_id = %q, want req_validation", got.RequestID)
	}
	if got.Error.Hint == "" || got.Error.NextAction != "fix_request" {
		t.Fatalf("validation remediation = hint:%q next_action:%q, want actionable fix_request", got.Error.Hint, got.Error.NextAction)
	}
	if got.Error.IsRetriable == nil || *got.Error.IsRetriable {
		t.Fatalf("validation is_retriable = %#v, want explicit false", got.Error.IsRetriable)
	}
	if got.Error.DocsURL != "https://unipost.dev/docs/api/posts/validate" {
		t.Fatalf("validation docs_url = %q, want validate docs URL", got.Error.DocsURL)
	}
	if len(got.Error.Issues) != 1 || got.Error.Issues[0].Code != platform.CodeMediaNotUploaded {
		t.Fatalf("issues = %#v, want media_not_uploaded issue", got.Error.Issues)
	}
}
