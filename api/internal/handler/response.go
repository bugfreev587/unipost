package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/platform"
)

type SuccessResponse struct {
	Data      any           `json:"data"`
	Meta      *MetaResponse `json:"meta,omitempty"`
	RequestID string        `json:"request_id,omitempty"`
}

type MetaResponse struct {
	Total      *int   `json:"total,omitempty"`
	Limit      *int   `json:"limit,omitempty"`
	HasMore    *bool  `json:"has_more,omitempty"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type ErrorBody struct {
	Code           string           `json:"code"`
	NormalizedCode string           `json:"normalized_code,omitempty"`
	Message        string           `json:"message"`
	Hint           string           `json:"hint,omitempty"`
	NextAction     string           `json:"next_action,omitempty"`
	IsRetriable    *bool            `json:"is_retriable,omitempty"`
	DocsURL        string           `json:"docs_url,omitempty"`
	Issues         []platform.Issue `json:"issues,omitempty"`
}

type ErrorResponse struct {
	Error     ErrorBody `json:"error"`
	RequestID string    `json:"request_id,omitempty"`
}

type legacyCursorSuccessResponse struct {
	Data       any           `json:"data"`
	Meta       *MetaResponse `json:"meta,omitempty"`
	RequestID  string        `json:"request_id,omitempty"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

type ErrorDetails struct {
	Hint        string
	NextAction  string
	IsRetriable *bool
	DocsURL     string
	Issues      []platform.Issue
}

const (
	apiErrorsDocsURL        = "https://unipost.dev/docs/api/errors"
	internalErrorMessage    = "UniPost could not complete the request because of an internal error."
	upstreamErrorMessage    = "A downstream service could not complete the request."
	internalErrorHint       = "Retry the request. If it continues, contact support with the request_id."
	upstreamErrorHint       = "Retry the request later. If it continues, contact support with the request_id."
	internalErrorNextAction = "contact_support"
	upstreamErrorNextAction = "wait_and_retry"
)

func requestIDFromResponse(w http.ResponseWriter) string {
	return w.Header().Get("X-Request-Id")
}

var normalizedErrorCodeMap = map[string]string{
	"ACCOUNT_ALREADY_CONNECTED":          "account_already_connected",
	"ACCOUNT_NOT_AVAILABLE_ON_FREE_PLAN": "account_not_available_on_free_plan",
	"BAD_REQUEST":                        "bad_request",
	"CONFLICT":                           "conflict",
	"DEFAULT_PROFILE_PROTECTED":          "default_profile_protected",
	"DELIVERY_FAILED":                    "delivery_failed",
	"FACEBOOK_DISABLED":                  "facebook_disabled",
	"FORBIDDEN":                          "forbidden",
	"INTERNAL_ERROR":                     "internal_error",
	"INVALID_REQUEST":                    "invalid_request",
	"INVALID_SIGNATURE":                  "invalid_signature",
	"INVALID_TOKEN":                      "invalid_token",
	"NEEDS_RECONNECT":                    "needs_reconnect",
	"NOT_CONFIGURED":                     "not_configured",
	"NOT_FOUND":                          "not_found",
	"NO_PAGES_SELECTED":                  "no_pages_selected",
	"PAGE_LACKS_PUBLISH_PERMISSION":      "page_lacks_publish_permission",
	// Plan gate (migration 057, PR1): X / Twitter publishing and
	// connect attempts on plans that disallow it. 402 from
	// /v1/connect/sessions; surfaced as a fatal validator code on
	// the publish path.
	"PLAN_PLATFORM_NOT_ALLOWED": "plan_platform_not_allowed",
	// Plan gate (migration 059, PR-B): Inbox / Analytics endpoints on
	// plans that don't unlock those features. 402 — clients should
	// surface an upgrade CTA rather than retrying.
	"PLAN_FEATURE_NOT_AVAILABLE": "plan_feature_not_available",
	// Free plan monthly quota gate: paid plans keep soft overage
	// behavior, but Free workspaces cannot accept new publish work
	// once doing so would exceed their monthly quota.
	"PLAN_POST_QUOTA_EXCEEDED": "plan_post_quota_exceeded",
	// Profile-create cap (migration 059, PR-B): workspace already at
	// the per-plan profile cap. 402 with the cap value in the
	// message so clients can render an exact upgrade prompt.
	"PROFILE_LIMIT_REACHED": "profile_limit_reached",
	// RBAC role gate (migration 060, PR-C): the authenticated user's
	// workspace role is below the minimum the endpoint requires. 403.
	"INSUFFICIENT_ROLE": "insufficient_role",
	"PLATFORM_ERROR":    "platform_error",
	// Per-platform daily safety cap (PR2): one social_account_id has
	// hit its UTC-day publish ceiling. Surfaces on the per-result
	// row's error_message (publish path is partial-success, so the
	// HTTP status is still 200 — clients switch on this code in the
	// per-result error_message string).
	"PER_PLATFORM_DAILY_CAP_EXCEEDED": "per_platform_daily_cap_exceeded",
	"QUEUE_JOB_ACTIVE":                "queue_job_active",
	"RESULT_NOT_RETRYABLE":            "result_not_retryable",
	"SETUP_TOKEN_EXPIRED":             "setup_token_expired",
	"SETUP_TOKEN_INVALID":             "setup_token_invalid",
	"SETUP_TOKEN_USED":                "setup_token_used",
	"STORAGE_NOT_CONFIGURED":          "storage_not_configured",
	"TIKTOK_ERROR":                    "tiktok_error",
	"UNAUTHORIZED":                    "unauthorized",
	"UNKNOWN_PAGE":                    "unknown_page",
	"UPSTREAM_ERROR":                  "upstream_error",
	"VALIDATION_ERROR":                "validation_error",
	"WRONG_PLATFORM":                  "wrong_platform",
}

func normalizeErrorCode(code string) string {
	trimmed := strings.TrimSpace(code)
	if normalized, ok := normalizedErrorCodeMap[trimmed]; ok {
		return normalized
	}
	return strings.ToLower(trimmed)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeSuccess(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, SuccessResponse{
		Data:      data,
		RequestID: requestIDFromResponse(w),
	})
}

func writeSuccessWithListMeta(w http.ResponseWriter, data any, total int, limit int) {
	writeJSON(w, http.StatusOK, SuccessResponse{
		Data: data,
		Meta: &MetaResponse{
			Total: &total,
			Limit: &limit,
		},
		RequestID: requestIDFromResponse(w),
	})
}

func writeCreated(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusCreated, SuccessResponse{
		Data:      data,
		RequestID: requestIDFromResponse(w),
	})
}

func writeAccepted(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusAccepted, SuccessResponse{
		Data:      data,
		RequestID: requestIDFromResponse(w),
	})
}

func writeSuccessWithCursor(w http.ResponseWriter, data any, nextCursor string, hasMore bool, limit int) {
	writeJSON(w, http.StatusOK, SuccessResponse{
		Data: data,
		Meta: &MetaResponse{
			Limit:      &limit,
			HasMore:    &hasMore,
			NextCursor: nextCursor,
		},
		RequestID: requestIDFromResponse(w),
	})
}

func writeSuccessWithLegacyCursor(w http.ResponseWriter, data any, nextCursor string, hasMore bool, limit int) {
	writeJSON(w, http.StatusOK, legacyCursorSuccessResponse{
		Data: data,
		Meta: &MetaResponse{
			Limit:      &limit,
			HasMore:    &hasMore,
			NextCursor: nextCursor,
		},
		RequestID:  requestIDFromResponse(w),
		NextCursor: nextCursor,
	})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeErrorWithDetails(w, status, code, message, ErrorDetails{})
}

func writeErrorWithDetails(w http.ResponseWriter, status int, code, message string, details ErrorDetails) {
	requestID := requestIDFromResponse(w)
	safeMessage, safeDetails := customerFacingError(status, code, message, details)
	if safeMessage != message {
		slog.Warn("sanitized customer-facing error response",
			"status", status,
			"code", code,
			"request_id", requestID,
			"internal_message", message,
		)
	}
	writeJSON(w, status, ErrorResponse{
		Error: ErrorBody{
			Code:           code,
			NormalizedCode: normalizeErrorCode(code),
			Message:        safeMessage,
			Hint:           safeDetails.Hint,
			NextAction:     safeDetails.NextAction,
			IsRetriable:    safeDetails.IsRetriable,
			DocsURL:        safeDetails.DocsURL,
			Issues:         safeDetails.Issues,
		},
		RequestID: requestID,
	})
}

func customerFacingError(status int, code, message string, details ErrorDetails) (string, ErrorDetails) {
	if !shouldSanitizeError(status, code) {
		return message, details
	}

	safeDetails := details
	if strings.TrimSpace(safeDetails.DocsURL) == "" {
		safeDetails.DocsURL = apiErrorsDocsURL
	}
	if isUpstreamError(code) {
		if strings.TrimSpace(safeDetails.Hint) == "" {
			safeDetails.Hint = upstreamErrorHint
		}
		if strings.TrimSpace(safeDetails.NextAction) == "" {
			safeDetails.NextAction = upstreamErrorNextAction
		}
		return upstreamErrorMessage, safeDetails
	}

	if strings.TrimSpace(safeDetails.Hint) == "" {
		safeDetails.Hint = internalErrorHint
	}
	if strings.TrimSpace(safeDetails.NextAction) == "" {
		safeDetails.NextAction = internalErrorNextAction
	}
	return internalErrorMessage, safeDetails
}

func shouldSanitizeError(status int, code string) bool {
	if status < http.StatusInternalServerError {
		return false
	}
	switch normalizeErrorCode(code) {
	case "internal_error", "upstream_error", "tiktok_error":
		return true
	default:
		return false
	}
}

func isUpstreamError(code string) bool {
	switch normalizeErrorCode(code) {
	case "upstream_error", "tiktok_error":
		return true
	default:
		return false
	}
}

// writeRateLimited writes a 429 with the precise normalized_code
// returned by the limiter (rate_limited / enqueue_rate_limited /
// queue_depth_exceeded) and a Retry-After header derived from the
// limiter's recommendation. Falls back to a 1-second hint if the
// limiter could not compute one.
func writeRateLimited(w http.ResponseWriter, normalizedCode, message string, retryAfter time.Duration) {
	if retryAfter < time.Second {
		retryAfter = time.Second
	}
	w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
	isRetriable := true
	writeJSON(w, http.StatusTooManyRequests, ErrorResponse{
		Error: ErrorBody{
			Code:           "RATE_LIMITED",
			NormalizedCode: normalizedCode,
			Message:        message,
			Hint:           "Wait for the Retry-After window to pass, then retry the request.",
			NextAction:     "wait_and_retry",
			IsRetriable:    &isRetriable,
		},
		RequestID: requestIDFromResponse(w),
	})
}
