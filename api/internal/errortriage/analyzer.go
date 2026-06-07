package errortriage

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type Failure struct {
	PostID             string
	SocialPostResultID string
	PostFailureID      string
	WorkspaceID        string
	WorkspaceName      string
	UserID             string
	UserEmail          string
	Platform           string
	Source             string
	ErrorCode          string
	PlatformErrorCode  string
	FailureStage       string
	Message            string
	RawError           string
	DebugCurl          string
	Caption            string
	IsRetriable        bool
	CreatedAt          time.Time
}

type RecipientCandidate struct {
	ScopeKey    string
	WorkspaceID string
	UserID      string
	Email       string
}

type Bucket struct {
	Key                    string
	Failures               []Failure
	Recipients             []RecipientCandidate
	AffectedUserCount      int
	AffectedWorkspaceCount int
	AffectedPostCount      int
	LatestFailureAt        time.Time
}

type EmailDraft struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
	CTAURL  string `json:"cta_url,omitempty"`
}

type BugPlan struct {
	Title          string   `json:"title"`
	Impact         string   `json:"impact"`
	Evidence       []string `json:"evidence"`
	SuspectedArea  string   `json:"suspected_area"`
	ProposedFix    string   `json:"proposed_fix"`
	ValidationPlan string   `json:"validation_plan"`
	RollbackPlan   string   `json:"rollback_plan"`
}

type ItemDraft struct {
	DedupeKey              string
	Classification         Classification
	ActionKind             ActionKind
	WorkflowStatus         WorkflowStatus
	Confidence             float64
	Platform               string
	Source                 string
	ErrorCode              string
	PlatformErrorCode      string
	FailureStage           string
	AffectedUserCount      int
	AffectedWorkspaceCount int
	AffectedPostCount      int
	LatestFailureAt        time.Time
	Evidence               map[string]any
	Summary                string
	EmailDraft             EmailDraft
	BugPlan                BugPlan
	CTAURL                 string
}

type DeterministicAnalyzer struct{}

func BuildBuckets(failures []Failure) []Bucket {
	byKey := map[string]*Bucket{}
	for _, failure := range failures {
		classification := preclassify(failure)
		key := DedupeKey(BucketKeyParts{
			Classification:    string(classification),
			Platform:          failure.Platform,
			Source:            failure.Source,
			ErrorCode:         failure.ErrorCode,
			PlatformErrorCode: failure.PlatformErrorCode,
			FailureStage:      failure.FailureStage,
			Message:           firstNonEmpty(failure.Message, failure.RawError),
		})
		bucket := byKey[key]
		if bucket == nil {
			bucket = &Bucket{Key: key}
			byKey[key] = bucket
		}
		bucket.Failures = append(bucket.Failures, failure)
		if failure.CreatedAt.After(bucket.LatestFailureAt) {
			bucket.LatestFailureAt = failure.CreatedAt
		}
	}

	out := make([]Bucket, 0, len(byKey))
	for _, bucket := range byKey {
		deriveBucketCounts(bucket)
		out = append(out, *bucket)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].LatestFailureAt.After(out[j].LatestFailureAt)
	})
	return out
}

func (DeterministicAnalyzer) Analyze(bucket Bucket) ItemDraft {
	first := Failure{}
	if len(bucket.Failures) > 0 {
		first = bucket.Failures[0]
	}
	classification := preclassify(first)
	action, status := workflowForClassification(classification)
	evidence := buildEvidence(bucket)
	item := ItemDraft{
		DedupeKey:              bucket.Key,
		Classification:         classification,
		ActionKind:             action,
		WorkflowStatus:         status,
		Confidence:             confidenceForClassification(classification),
		Platform:               first.Platform,
		Source:                 first.Source,
		ErrorCode:              first.ErrorCode,
		PlatformErrorCode:      first.PlatformErrorCode,
		FailureStage:           first.FailureStage,
		AffectedUserCount:      bucket.AffectedUserCount,
		AffectedWorkspaceCount: bucket.AffectedWorkspaceCount,
		AffectedPostCount:      bucket.AffectedPostCount,
		LatestFailureAt:        bucket.LatestFailureAt,
		Evidence:               evidence,
		Summary:                summaryFor(classification, first),
	}
	switch classification {
	case ClassificationUserActionNeeded:
		item.EmailDraft = emailDraftFor(first)
		item.CTAURL = item.EmailDraft.CTAURL
	case ClassificationUnipostBug:
		item.BugPlan = bugPlanFor(first, bucket)
	case ClassificationUpstreamPlatformIssue:
		item.ActionKind = ActionKindMonitor
	case ClassificationTransientNoAction:
		item.ActionKind = ActionKindNone
		item.WorkflowStatus = WorkflowStatusCompleted
	case ClassificationNeedsHumanReview:
		item.ActionKind = ActionKindReview
		item.WorkflowStatus = WorkflowStatusPendingReview
	}
	return item
}

func (draft ItemDraft) EvidenceJSON() []byte {
	raw, err := json.Marshal(draft.Evidence)
	if err != nil {
		return []byte(`{}`)
	}
	return raw
}

func deriveBucketCounts(bucket *Bucket) {
	users := map[string]bool{}
	workspaces := map[string]bool{}
	posts := map[string]bool{}
	recipients := map[string]RecipientCandidate{}
	for _, failure := range bucket.Failures {
		if failure.UserID != "" {
			users[failure.UserID] = true
		}
		if failure.WorkspaceID != "" {
			workspaces[failure.WorkspaceID] = true
		}
		if failure.PostID != "" {
			posts[failure.PostID] = true
		}
		if failure.WorkspaceID != "" && failure.UserID != "" {
			scope := "workspace:" + failure.WorkspaceID + ":user:" + failure.UserID
			recipients[scope] = RecipientCandidate{
				ScopeKey:    scope,
				WorkspaceID: failure.WorkspaceID,
				UserID:      failure.UserID,
				Email:       failure.UserEmail,
			}
		}
	}
	bucket.AffectedUserCount = len(users)
	bucket.AffectedWorkspaceCount = len(workspaces)
	bucket.AffectedPostCount = len(posts)
	bucket.Recipients = make([]RecipientCandidate, 0, len(recipients))
	for _, recipient := range recipients {
		bucket.Recipients = append(bucket.Recipients, recipient)
	}
	sort.Slice(bucket.Recipients, func(i, j int) bool {
		return bucket.Recipients[i].ScopeKey < bucket.Recipients[j].ScopeKey
	})
}

func preclassify(f Failure) Classification {
	joined := strings.ToLower(strings.Join([]string{f.ErrorCode, f.PlatformErrorCode, f.FailureStage, f.Message, f.RawError}, " "))
	switch {
	case f.IsRetriable:
		return ClassificationTransientNoAction
	case containsAny(joined, "missing_permission", "permission", "auth", "oauth", "expired", "reconnect", "revoked", "unauthorized"):
		return ClassificationUserActionNeeded
	case containsAny(joined, "quota", "rate limit", "rate_limit", "daily cap", "too long", "caption", "unsupported", "invalid media"):
		return ClassificationUserActionNeeded
	case containsAny(joined, "chunk size", "invalid_params", "bad request", "nil pointer", "panic", "storage", "internal_error"):
		return ClassificationUnipostBug
	case containsAny(joined, "platform outage", "temporarily unavailable", "server error", "502", "503", "504"):
		return ClassificationUpstreamPlatformIssue
	default:
		return ClassificationNeedsHumanReview
	}
}

func workflowForClassification(classification Classification) (ActionKind, WorkflowStatus) {
	switch classification {
	case ClassificationUserActionNeeded:
		return ActionKindEmail, WorkflowStatusReady
	case ClassificationUnipostBug:
		return ActionKindBugPlan, WorkflowStatusReady
	case ClassificationUpstreamPlatformIssue:
		return ActionKindMonitor, WorkflowStatusReady
	case ClassificationTransientNoAction:
		return ActionKindNone, WorkflowStatusCompleted
	default:
		return ActionKindReview, WorkflowStatusPendingReview
	}
}

func confidenceForClassification(classification Classification) float64 {
	switch classification {
	case ClassificationNeedsHumanReview:
		return 0.45
	case ClassificationTransientNoAction:
		return 0.75
	default:
		return 0.82
	}
}

func buildEvidence(bucket Bucket) map[string]any {
	samples := make([]map[string]any, 0, minInt(len(bucket.Failures), 5))
	truncated := false
	for i, failure := range bucket.Failures {
		if i >= 5 {
			truncated = true
			break
		}
		msg, msgTruncated := SanitizeForAI(firstNonEmpty(failure.Message, failure.RawError), 500)
		curl, curlTruncated := SanitizeForAI(failure.DebugCurl, 1000)
		truncated = truncated || msgTruncated || curlTruncated
		samples = append(samples, map[string]any{
			"post_id":       failure.PostID,
			"workspace_id":  failure.WorkspaceID,
			"platform":      failure.Platform,
			"error_code":    failure.ErrorCode,
			"failure_stage": failure.FailureStage,
			"message":       msg,
			"debug_curl":    curl,
			"created_at":    failure.CreatedAt,
		})
	}
	return map[string]any{
		"samples":                  samples,
		"truncated":                truncated,
		"failure_count":            len(bucket.Failures),
		"affected_user_count":      bucket.AffectedUserCount,
		"affected_workspace_count": bucket.AffectedWorkspaceCount,
		"affected_post_count":      bucket.AffectedPostCount,
	}
}

func summaryFor(classification Classification, f Failure) string {
	platform := firstNonEmpty(f.Platform, "unknown platform")
	switch classification {
	case ClassificationUserActionNeeded:
		return "A customer action appears to be needed for " + platform + "."
	case ClassificationUnipostBug:
		return "Failures suggest UniPost may need a platform integration fix for " + platform + "."
	case ClassificationUpstreamPlatformIssue:
		return "Failures look related to an upstream " + platform + " issue."
	case ClassificationTransientNoAction:
		return "Failures appear transient or retryable; no immediate action is needed."
	default:
		return "The bucket needs human review before taking action."
	}
}

func emailDraftFor(f Failure) EmailDraft {
	platform := title(firstNonEmpty(f.Platform, "the platform"))
	return EmailDraft{
		Subject: "Action needed: review your " + platform + " connection in UniPost",
		Body: strings.Join([]string{
			"Hi,",
			"",
			"We reviewed a recent publishing failure for " + platform + " and it looks like your account or post settings need attention before UniPost can retry successfully.",
			"",
			"Please open UniPost, review the failed post, and reconnect or update the account if prompted.",
			"",
			"If you already received a publish-failed notification, this is an admin-reviewed follow-up with the likely next step.",
			"",
			"UniPost Support",
		}, "\n"),
		CTAURL: "",
	}
}

func bugPlanFor(f Failure, bucket Bucket) BugPlan {
	platform := firstNonEmpty(f.Platform, "unknown")
	return BugPlan{
		Title:         "Investigate " + platform + " publishing failure bucket",
		Impact:        "Affects " + strconvItoa(bucket.AffectedPostCount) + " failed post attempt(s) across " + strconvItoa(bucket.AffectedWorkspaceCount) + " workspace(s).",
		Evidence:      []string{firstNonEmpty(f.Message, f.RawError, f.ErrorCode)},
		SuspectedArea: firstNonEmpty(f.FailureStage, platform+" adapter"),
		ProposedFix:   "Inspect the adapter request construction and validation path for this failure bucket, then add a regression test for the normalized provider error.",
		ValidationPlan: strings.Join([]string{
			"Run focused adapter tests for " + platform + ".",
			"Run GOCACHE=/tmp/unipost-go-build go test ./...",
			"Verify the failing publish flow in the development environment after deployment.",
		}, " "),
		RollbackPlan: "Rollback the adapter change or disable the affected publish path while preserving existing validation errors.",
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func title(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
