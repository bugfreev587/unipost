package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
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
	Code           string `json:"code"`
	NormalizedCode string `json:"normalized_code,omitempty"`
	Message        string `json:"message"`
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

func requestIDFromResponse(w http.ResponseWriter) string {
	return w.Header().Get("X-Request-Id")
}

var normalizedErrorCodeMap = map[string]string{
	"ACCOUNT_ALREADY_CONNECTED":     "account_already_connected",
	"BAD_REQUEST":                   "bad_request",
	"CONFLICT":                      "conflict",
	"DEFAULT_PROFILE_PROTECTED":     "default_profile_protected",
	"DELIVERY_FAILED":               "delivery_failed",
	"FACEBOOK_DISABLED":             "facebook_disabled",
	"FORBIDDEN":                     "forbidden",
	"INTERNAL_ERROR":                "internal_error",
	"INVALID_REQUEST":               "invalid_request",
	"INVALID_SIGNATURE":             "invalid_signature",
	"INVALID_TOKEN":                 "invalid_token",
	"NEEDS_RECONNECT":               "needs_reconnect",
	"NOT_CONFIGURED":                "not_configured",
	"NOT_FOUND":                     "not_found",
	"NO_PAGES_SELECTED":             "no_pages_selected",
	"PAGE_LACKS_PUBLISH_PERMISSION": "page_lacks_publish_permission",
	// Plan gate (migration 057, PR1): X / Twitter publishing and
	// connect attempts on plans that disallow it. 402 from
	// /v1/connect/sessions; surfaced as a fatal validator code on
	// the publish path.
	"PLAN_PLATFORM_NOT_ALLOWED": "plan_platform_not_allowed",
	// Plan gate (migration 059, PR-B): Inbox / Analytics endpoints on
	// plans that don't unlock those features. 402 — clients should
	// surface an upgrade CTA rather than retrying.
	"PLAN_FEATURE_NOT_AVAILABLE": "plan_feature_not_available",
	// Profile-create cap (migration 059, PR-B): workspace already at
	// the per-plan profile cap. 402 with the cap value in the
	// message so clients can render an exact upgrade prompt.
	"PROFILE_LIMIT_REACHED": "profile_limit_reached",
	// RBAC role gate (migration 060, PR-C): the authenticated user's
	// workspace role is below the minimum the endpoint requires. 403.
	"INSUFFICIENT_ROLE": "insufficient_role",
	"PLATFORM_ERROR":            "platform_error",
	// Per-platform daily safety cap (PR2): one social_account_id has
	// hit its UTC-day publish ceiling. Surfaces on the per-result
	// row's error_message (publish path is partial-success, so the
	// HTTP status is still 200 — clients switch on this code in the
	// per-result error_message string).
	"PER_PLATFORM_DAILY_CAP_EXCEEDED": "per_platform_daily_cap_exceeded",
	"QUEUE_JOB_ACTIVE":              "queue_job_active",
	"RESULT_NOT_RETRYABLE":          "result_not_retryable",
	"STORAGE_NOT_CONFIGURED":        "storage_not_configured",
	"TIKTOK_ERROR":                  "tiktok_error",
	"UNAUTHORIZED":                  "unauthorized",
	"UNKNOWN_PAGE":                  "unknown_page",
	"UPSTREAM_ERROR":                "upstream_error",
	"VALIDATION_ERROR":              "validation_error",
	"WRONG_PLATFORM":                "wrong_platform",
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
	writeJSON(w, status, ErrorResponse{
		Error: ErrorBody{
			Code:           code,
			NormalizedCode: normalizeErrorCode(code),
			Message:        message,
		},
		RequestID: requestIDFromResponse(w),
	})
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
	writeJSON(w, http.StatusTooManyRequests, ErrorResponse{
		Error: ErrorBody{
			Code:           "RATE_LIMITED",
			NormalizedCode: normalizedCode,
			Message:        message,
		},
		RequestID: requestIDFromResponse(w),
	})
}
