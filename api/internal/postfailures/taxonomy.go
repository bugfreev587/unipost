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
}

var metaSubcodePattern = regexp.MustCompile(`error_subcode["=: ]+([0-9]+)`)

func Classify(raw string) Classification {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return Classification{
			ErrorCode:   "unknown_error",
			IsRetriable: false,
		}
	}

	c := Classification{
		ErrorCode:   "platform_error",
		IsRetriable: false,
	}

	if code := extractMetaSubcode(raw); code != "" {
		c.PlatformErrorCode = code
	}

	switch {
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
	case strings.Contains(s, "permission") || strings.Contains(s, "scope"):
		c.ErrorCode = "missing_permission"
	case strings.Contains(s, "quota") || strings.Contains(s, "limit exceeded"):
		c.ErrorCode = "quota_exceeded"
	case strings.Contains(s, "validation") || strings.Contains(s, "invalid request"):
		c.ErrorCode = "validation_error"
	case strings.Contains(s, "media"):
		c.ErrorCode = "media_error"
	case strings.Contains(s, "not found") || strings.Contains(s, "cannot be found"):
		c.ErrorCode = "target_not_found"
	}

	return c
}

func extractMetaSubcode(raw string) string {
	m := metaSubcodePattern.FindStringSubmatch(raw)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

func ToText(v string) pgtype.Text {
	return pgtype.Text{String: v, Valid: strings.TrimSpace(v) != ""}
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
	}
}
