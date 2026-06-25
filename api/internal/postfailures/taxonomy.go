package postfailures

import (
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type Classification struct {
	ErrorCode         string
	PlatformErrorCode string
	IsRetriable       bool
	ErrorSource       string
	ErrorTemporality  string
	ProviderError     *ProviderError
}

var (
	metaSubcodePattern       = regexp.MustCompile(`error_subcode["=: ]+([0-9]+)`)
	providerErrorCodePattern = regexp.MustCompile(`(?i)provider_error=([a-z0-9_.:-]+)`)
	jsonErrorCodePattern     = regexp.MustCompile(`"code"\s*:\s*"([^"]+)"`)
)

func Classify(raw string) Classification {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return Classification{
			ErrorCode:        "unknown_error",
			ErrorSource:      ErrorSourceUnknown,
			ErrorTemporality: ErrorTemporalityUnknown,
			IsRetriable:      false,
		}
	}

	c := Classification{
		ErrorCode:   "platform_error",
		IsRetriable: false,
	}

	if code := extractMetaSubcode(raw); code != "" {
		c.PlatformErrorCode = code
	} else if code := extractMetaCode(raw); code != "" {
		c.PlatformErrorCode = code
	}
	if strings.Contains(s, "tiktok") {
		if code := extractTikTokProviderCode(raw); code != "" {
			c.PlatformErrorCode = code
		}
	}
	if strings.Contains(s, "youtube") {
		if reason := regexpFirst(keyValueReasonPattern, raw); reason != "" {
			c.PlatformErrorCode = reason
		} else if reason := regexpFirst(youtubeJSONReasonPattern, raw); reason != "" {
			c.PlatformErrorCode = reason
		}
	}

	switch {
	case isMetaOAuthReconnectError(s):
		c.ErrorCode = "account_reconnect_required"
	case isMetaTransientError(s):
		c.ErrorCode = "temporary_platform_error"
		c.IsRetriable = true
	case strings.Contains(s, "tiktok") && strings.Contains(s, "file_format_check_failed"):
		c.ErrorCode = "media_error"
		c.PlatformErrorCode = "file_format_check_failed"
	case strings.Contains(s, "tiktok") && strings.Contains(s, "invalid_params"):
		c.ErrorCode = "platform_request_invalid"
		if c.PlatformErrorCode == "" {
			c.PlatformErrorCode = "invalid_params"
		}
	case strings.Contains(s, "threads get user id failed") && isMetaOAuthReconnectError(s):
		c.ErrorCode = "account_reconnect_required"
	case strings.Contains(s, "threads get user id failed") && strings.Contains(s, "(401)"):
		c.ErrorCode = "account_reconnect_required"
	case strings.Contains(s, "threads get user id failed") && strings.Contains(s, "(403)"):
		c.ErrorCode = "missing_permission"
	case strings.Contains(s, "instagram container processing failed") && strings.Contains(s, "status_code=error"):
		c.ErrorCode = "media_error"
	case strings.Contains(s, "container processing failed") || strings.Contains(s, "container processing timed out"):
		// Instagram's async media container can fail or stall on the
		// first try for transient reasons (IG-side transcoding hiccup,
		// source URL race). New explicit ERROR diagnostics include
		// status_code=ERROR and are classified above as media_error.
		c.ErrorCode = "temporary_platform_error"
		c.IsRetriable = true
	case strings.Contains(s, "rate limit") || strings.Contains(s, "too many requests"):
		c.ErrorCode = "rate_limit"
		c.IsRetriable = true
	case strings.Contains(s, "timeout") || strings.Contains(s, "temporarily unavailable") || strings.Contains(s, "try again later"):
		c.ErrorCode = "temporary_platform_error"
		c.IsRetriable = true
	case strings.Contains(s, "token") && (strings.Contains(s, "expired") || strings.Contains(s, "invalid")):
		c.ErrorCode = "auth_token_invalid"
	case strings.Contains(s, "disconnected") || strings.Contains(s, "reconnect"):
		c.ErrorCode = "account_reconnect_required"
	case strings.Contains(s, "quota") || strings.Contains(s, "limit exceeded"):
		c.ErrorCode = "quota_exceeded"
	case strings.Contains(s, "permission") || strings.Contains(s, "scope"):
		c.ErrorCode = "missing_permission"
	case strings.Contains(s, "validation") || strings.Contains(s, "invalid request"):
		c.ErrorCode = "validation_error"
	case strings.Contains(s, "media"):
		c.ErrorCode = "media_error"
	case strings.Contains(s, "not found") || strings.Contains(s, "cannot be found"):
		c.ErrorCode = "target_not_found"
	}

	return enrichClassification(c, raw)
}

func ShouldMarkReconnectRequired(raw string) bool {
	switch Classify(raw).ErrorCode {
	case "account_reconnect_required", "auth_token_invalid":
		return true
	default:
		return false
	}
}

func DeriveLegacyContract(errorCode, message string, isRetriable bool) Classification {
	if strings.TrimSpace(errorCode) == "" {
		c := Classify(message)
		if isRetriable {
			c.IsRetriable = true
		}
		return c
	}
	c := Classification{
		ErrorCode:     strings.TrimSpace(errorCode),
		IsRetriable:   isRetriable,
		ProviderError: ExtractProviderError(message),
	}
	return enrichClassification(c, message)
}

func isMetaOAuthReconnectError(s string) bool {
	if strings.Contains(s, `"code":190`) || strings.Contains(s, `"code": 190`) {
		return true
	}
	return strings.Contains(s, "oauthexception") &&
		(strings.Contains(s, "session has expired") ||
			strings.Contains(s, "error validating access token") ||
			strings.Contains(s, "invalid oauth access token"))
}

func isMetaTransientError(s string) bool {
	if strings.Contains(s, `"is_transient":true`) || strings.Contains(s, `"is_transient": true`) {
		return true
	}
	if strings.Contains(s, "please retry your request later") {
		return true
	}
	return strings.Contains(s, "oauthexception") &&
		(strings.Contains(s, `"code":2`) || strings.Contains(s, `"code": 2`)) &&
		(strings.Contains(s, "unexpected error") || strings.Contains(s, "retry"))
}

func extractMetaSubcode(raw string) string {
	m := metaSubcodePattern.FindStringSubmatch(raw)
	if len(m) == 2 {
		if strings.TrimSpace(m[1]) != "0" {
			return m[1]
		}
	}
	return ""
}

func extractMetaCode(raw string) string {
	return trimJSONScalar(regexpFirst(metaJSONCodePattern, raw))
}

func extractTikTokProviderCode(raw string) string {
	if m := providerErrorCodePattern.FindStringSubmatch(raw); len(m) == 2 {
		return strings.ToLower(strings.TrimSpace(m[1]))
	}
	if m := jsonErrorCodePattern.FindStringSubmatch(raw); len(m) == 2 {
		code := strings.ToLower(strings.TrimSpace(m[1]))
		if code != "" && code != "ok" {
			return code
		}
	}
	return ""
}

func ToText(v string) pgtype.Text {
	return pgtype.Text{String: v, Valid: strings.TrimSpace(v) != ""}
}

func NextActionForErrorCode(errorCode string) string {
	switch strings.TrimSpace(errorCode) {
	case "validation_error":
		return "fix_request"
	case "platform_request_invalid":
		return "review_platform_options"
	case "media_error":
		return "fix_media"
	case "temporary_platform_error", "worker_stalled":
		return "retry_later"
	case "rate_limit":
		return "wait_and_retry"
	case "quota_exceeded":
		return "review_quota"
	case "account_reconnect_required", "auth_token_invalid":
		return "reconnect_account"
	case "missing_permission":
		return "reconnect_or_update_permissions"
	case "target_not_found":
		return "select_valid_target"
	case "unknown_error", "platform_error":
		return "contact_support"
	default:
		if strings.TrimSpace(errorCode) == "" {
			return ""
		}
		return "contact_support"
	}
}

func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func BuildParams(postID, socialPostResultID, workspaceID, socialAccountID, platform, failureStage, message, rawError string) db.CreatePostFailureParams {
	classification := Classify(message)
	return buildParamsFromClassification(postID, socialPostResultID, workspaceID, socialAccountID, platform, failureStage, message, rawError, classification)
}

func BuildParamsFromError(postID, socialPostResultID, workspaceID, socialAccountID, platform, failureStage string, err error, rawError string) db.CreatePostFailureParams {
	message := ""
	if err != nil {
		message = err.Error()
	}
	classification := classifyError(message, err)
	return buildParamsFromClassification(postID, socialPostResultID, workspaceID, socialAccountID, platform, failureStage, message, rawError, classification)
}

func buildParamsFromClassification(postID, socialPostResultID, workspaceID, socialAccountID, platform, failureStage, message, rawError string, classification Classification) db.CreatePostFailureParams {
	return db.CreatePostFailureParams{
		PostID:             postID,
		SocialPostResultID: ToText(socialPostResultID),
		WorkspaceID:        workspaceID,
		SocialAccountID:    ToText(socialAccountID),
		Platform:           FirstNonEmpty(platform, "unknown"),
		FailureStage:       failureStage,
		ErrorCode:          classification.ErrorCode,
		PlatformErrorCode:  ToText(classification.PlatformErrorCode),
		Message:            message,
		RawError:           ToText(rawError),
		IsRetriable:        classification.IsRetriable,
		ErrorSource:        ToText(classification.ErrorSource),
		ErrorTemporality:   ToText(classification.ErrorTemporality),
		ProviderError:      ProviderErrorJSON(classification.ProviderError),
	}
}
