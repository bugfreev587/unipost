package errortriage

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Classification string

const (
	ClassificationUnipostBug            Classification = "unipost_bug"
	ClassificationUserActionNeeded      Classification = "user_action_needed"
	ClassificationUpstreamPlatformIssue Classification = "upstream_platform_issue"
	ClassificationTransientNoAction     Classification = "transient_no_action"
	ClassificationNeedsHumanReview      Classification = "needs_human_review"
)

type ActionKind string

const (
	ActionKindNone    ActionKind = "none"
	ActionKindEmail   ActionKind = "email"
	ActionKindBugPlan ActionKind = "bug_plan"
	ActionKindMonitor ActionKind = "monitor"
	ActionKindReview  ActionKind = "review"
)

type WorkflowStatus string

const (
	WorkflowStatusPendingReview      WorkflowStatus = "pending_review"
	WorkflowStatusReady              WorkflowStatus = "ready"
	WorkflowStatusPartiallyCompleted WorkflowStatus = "partially_completed"
	WorkflowStatusCompleted          WorkflowStatus = "completed"
	WorkflowStatusDismissed          WorkflowStatus = "dismissed"
	WorkflowStatusFailed             WorkflowStatus = "failed"
)

type RecipientStatus string

const (
	RecipientStatusPending    RecipientStatus = "pending"
	RecipientStatusSent       RecipientStatus = "sent"
	RecipientStatusDismissed  RecipientStatus = "dismissed"
	RecipientStatusSendFailed RecipientStatus = "send_failed"
)

type RunHealthStatus string

const (
	RunHealthNoActionableIssues RunHealthStatus = "no_actionable_issues"
	RunHealthActionableItems    RunHealthStatus = "actionable_items"
	RunHealthNeedsReview        RunHealthStatus = "needs_review"
)

type BucketKeyParts struct {
	Classification    string
	Platform          string
	Source            string
	ErrorCode         string
	PlatformErrorCode string
	FailureStage      string
	Message           string
	SuspectedArea     string
}

type ItemState struct {
	Classification Classification
	ActionKind     ActionKind
	WorkflowStatus WorkflowStatus
}

type RecipientState struct {
	Status RecipientStatus
}

func PreviousPTDayWindow(now time.Time) (time.Time, time.Time, error) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	ptNow := now.In(loc)
	end := time.Date(ptNow.Year(), ptNow.Month(), ptNow.Day(), 0, 0, 0, 0, loc)
	start := end.AddDate(0, 0, -1)
	return start, end, nil
}

func DedupeKey(parts BucketKeyParts) string {
	normalized := []string{
		norm(parts.Classification),
		norm(parts.Platform),
		norm(parts.Source),
		norm(parts.ErrorCode),
		norm(parts.PlatformErrorCode),
		norm(parts.FailureStage),
		messageFingerprint(parts.Message),
		norm(parts.SuspectedArea),
	}
	sum := sha256.Sum256([]byte(strings.Join(normalized, "|")))
	return "triage:" + hex.EncodeToString(sum[:])[:24]
}

func SanitizeForAI(value string, maxLen int) (string, bool) {
	out := value
	out = authorizationPattern.ReplaceAllString(out, "$1[REDACTED]$3")
	out = cookiePattern.ReplaceAllString(out, "$1[REDACTED]$3")
	out = jsonSecretPattern.ReplaceAllString(out, `${1}[REDACTED]${3}`)
	out = redactURLSecrets(out)
	truncated := false
	if maxLen > 0 && len(out) > maxLen {
		out = out[:maxLen]
		truncated = true
	}
	return out, truncated
}

func CanSendRecipient(item ItemState, recipient RecipientState, loopsConfigured bool, currentEmail string) (bool, string) {
	if !loopsConfigured {
		return false, "loops_not_configured"
	}
	if strings.TrimSpace(currentEmail) == "" {
		return false, "recipient_email_missing"
	}
	if item.Classification != ClassificationUserActionNeeded || item.ActionKind != ActionKindEmail {
		return false, "item_not_email_action"
	}
	if item.WorkflowStatus != WorkflowStatusReady && item.WorkflowStatus != WorkflowStatusPartiallyCompleted {
		return false, "item_not_sendable"
	}
	switch recipient.Status {
	case RecipientStatusPending, RecipientStatusSendFailed:
		return true, ""
	case RecipientStatusSent, RecipientStatusDismissed:
		return false, "recipient_already_final"
	default:
		return false, "recipient_not_sendable"
	}
}

func SendIdempotencyKey(itemID, recipientScopeKey string, draftVersion int) string {
	base := "error_triage:" + strings.TrimSpace(itemID) + ":" + strings.TrimSpace(recipientScopeKey)
	if draftVersion > 1 {
		return base + ":draft:" + strconvItoa(draftVersion)
	}
	return base
}

func DeriveRunHealthStatus(items []ItemState) RunHealthStatus {
	actionable := false
	for _, item := range items {
		if item.ActionKind == ActionKindReview || item.WorkflowStatus == WorkflowStatusPendingReview || item.Classification == ClassificationNeedsHumanReview {
			return RunHealthNeedsReview
		}
		if item.ActionKind != ActionKindNone && item.WorkflowStatus != WorkflowStatusCompleted && item.WorkflowStatus != WorkflowStatusDismissed {
			actionable = true
		}
	}
	if actionable {
		return RunHealthActionableItems
	}
	return RunHealthNoActionableIssues
}

func norm(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func messageFingerprint(value string) string {
	v := norm(value)
	replacements := []string{
		"caption",
		"post",
		"text",
		"message",
	}
	for _, marker := range replacements {
		if idx := strings.Index(v, marker+":"); idx >= 0 {
			v = strings.TrimSpace(v[:idx])
		}
	}
	fields := strings.Fields(v)
	if len(fields) > 2 {
		fields = fields[:2]
	}
	return strings.Join(fields, " ")
}

var (
	authorizationPattern = regexp.MustCompile(`(?i)(Authorization:\s*(?:Bearer|Basic)?\s*)([^'\s]+)(['\s]?)`)
	cookiePattern        = regexp.MustCompile(`(?i)(Cookie:\s*)([^'\n]+)(['\n]?)`)
	jsonSecretPattern    = regexp.MustCompile(`(?i)("(?:access_token|refresh_token|client_secret|api_key|token|cookie)"\s*:\s*")([^"]+)(")`)
)

func redactURLSecrets(value string) string {
	for _, key := range []string{"access_token", "refresh_token", "client_secret", "api_key", "token"} {
		pattern := regexp.MustCompile(`(?i)([?&]` + regexp.QuoteMeta(key) + `=)([^&'\s]+)`)
		value = pattern.ReplaceAllStringFunc(value, func(match string) string {
			parts := strings.SplitN(match, "=", 2)
			if len(parts) != 2 {
				return match
			}
			decoded, err := url.QueryUnescape(parts[1])
			if err == nil && strings.EqualFold(decoded, "[REDACTED]") {
				return match
			}
			return parts[0] + "=[REDACTED]"
		})
	}
	return value
}

func strconvItoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	n := v
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
