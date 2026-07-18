package platform

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// TikTokAnalyticsReason is a stable, provider-specific reason that callers can
// translate without parsing TikTok's error text.
type TikTokAnalyticsReason string

const (
	TikTokAccountTokenInvalid    TikTokAnalyticsReason = "account_token_invalid"
	TikTokAnalyticsScopeRequired TikTokAnalyticsReason = "analytics_scope_required"
	TikTokProviderRateLimited    TikTokAnalyticsReason = "provider_rate_limited"
	TikTokProviderTemporaryError TikTokAnalyticsReason = "provider_temporary_error"
	TikTokVideoNotFound          TikTokAnalyticsReason = "video_not_found"
	TikTokVideoNotReady          TikTokAnalyticsReason = "video_not_ready"
)

type TikTokAnalyticsError struct {
	Reason       TikTokAnalyticsReason
	Operation    string
	HTTPStatus   int
	ProviderCode string
	Err          error
}

func NewTikTokAnalyticsError(
	reason TikTokAnalyticsReason,
	operation string,
	httpStatus int,
	providerCode string,
	err error,
) *TikTokAnalyticsError {
	return &TikTokAnalyticsError{
		Reason:       reason,
		Operation:    operation,
		HTTPStatus:   httpStatus,
		ProviderCode: providerCode,
		Err:          err,
	}
}

func (e *TikTokAnalyticsError) Error() string {
	if e == nil {
		return "tiktok analytics error"
	}
	message := fmt.Sprintf("tiktok analytics %s: %s", e.Operation, e.Reason)
	if e.ProviderCode != "" {
		message += " (" + e.ProviderCode + ")"
	}
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	return message
}

func (e *TikTokAnalyticsError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func TikTokAnalyticsErrorReasonOf(err error) (TikTokAnalyticsReason, bool) {
	var analyticsErr *TikTokAnalyticsError
	if !errors.As(err, &analyticsErr) || analyticsErr == nil {
		return "", false
	}
	return analyticsErr.Reason, true
}

func newTikTokAnalyticsTransportError(operation string, err error) error {
	return NewTikTokAnalyticsError(
		TikTokProviderTemporaryError,
		operation,
		0,
		"",
		err,
	)
}

func newTikTokAnalyticsResponseError(operation string, status int, body []byte) error {
	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &envelope)

	reason := classifyTikTokAnalyticsFailure(status, envelope.Error.Code)
	detail := strings.TrimSpace(envelope.Error.Message)
	if detail == "" {
		detail = strings.TrimSpace(string(body))
	}
	if detail == "" {
		detail = http.StatusText(status)
	}
	return NewTikTokAnalyticsError(
		reason,
		operation,
		status,
		envelope.Error.Code,
		errors.New(detail),
	)
}

func classifyTikTokAnalyticsFailure(status int, providerCode string) TikTokAnalyticsReason {
	code := strings.ToLower(strings.TrimSpace(providerCode))
	switch {
	case strings.Contains(code, "access_token") ||
		strings.Contains(code, "invalid_token") ||
		strings.Contains(code, "token_invalid") ||
		strings.Contains(code, "token_expired"):
		return TikTokAccountTokenInvalid
	case strings.Contains(code, "scope") ||
		strings.Contains(code, "permission") ||
		strings.Contains(code, "not_authorized"):
		return TikTokAnalyticsScopeRequired
	case status == http.StatusUnauthorized:
		return TikTokAccountTokenInvalid
	case status == http.StatusForbidden:
		return TikTokAnalyticsScopeRequired
	case status == http.StatusTooManyRequests:
		return TikTokProviderRateLimited
	default:
		return TikTokProviderTemporaryError
	}
}
