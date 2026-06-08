package errortriage

import (
	"strings"
	"testing"
	"time"
)

func TestPreviousPTDayWindowHandlesDSTSpringForward(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("load PT location: %v", err)
	}
	now := time.Date(2026, 3, 9, 0, 3, 0, 0, loc)

	start, end, err := PreviousPTDayWindow(now)
	if err != nil {
		t.Fatalf("PreviousPTDayWindow returned error: %v", err)
	}

	if got, want := start.In(loc).Format(time.RFC3339), "2026-03-08T00:00:00-08:00"; got != want {
		t.Fatalf("start = %s, want %s", got, want)
	}
	if got, want := end.In(loc).Format(time.RFC3339), "2026-03-09T00:00:00-07:00"; got != want {
		t.Fatalf("end = %s, want %s", got, want)
	}
	if got, want := end.Sub(start), 23*time.Hour; got != want {
		t.Fatalf("duration = %s, want %s", got, want)
	}
}

func TestDedupeKeyNormalizesFieldsAndAvoidsRawUserContent(t *testing.T) {
	a := DedupeKey(BucketKeyParts{
		Classification:    "User_Action_Needed",
		Platform:          " Threads ",
		Source:            "Dashboard",
		ErrorCode:         " AUTH_EXPIRED ",
		PlatformErrorCode: "190",
		FailureStage:      " Publish ",
		Message:           "Token expired for post caption: secret launch text",
		SuspectedArea:     "OAuth",
	})
	b := DedupeKey(BucketKeyParts{
		Classification:    "user_action_needed",
		Platform:          "threads",
		Source:            "dashboard",
		ErrorCode:         "auth_expired",
		PlatformErrorCode: "190",
		FailureStage:      "publish",
		Message:           "token expired for different private caption",
		SuspectedArea:     "oauth",
	})

	if a != b {
		t.Fatalf("dedupe key should normalize equivalent buckets: %q != %q", a, b)
	}
	if !strings.HasPrefix(a, "triage:") {
		t.Fatalf("dedupe key = %q, want triage prefix", a)
	}
	if strings.Contains(a, "secret") || strings.Contains(a, "caption") {
		t.Fatalf("dedupe key leaked raw user content: %q", a)
	}
}

func TestSanitizeForAITruncatesAndRedactsSecrets(t *testing.T) {
	raw := strings.Join([]string{
		"curl -X POST 'https://graph.example.com/me?access_token=abc123&safe=1'",
		"-H 'Authorization: Bearer sk-secret'",
		"-H 'Cookie: session=private'",
		`{"refresh_token":"rt-secret","caption":"hello"}`,
		strings.Repeat("x", 80),
	}, "\n")

	got, truncated := SanitizeForAI(raw, 140)
	if !truncated {
		t.Fatalf("expected truncation")
	}
	for _, forbidden := range []string{"abc123", "sk-secret", "session=private", "rt-secret"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("sanitized output leaked %q: %s", forbidden, got)
		}
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("sanitized output missing redaction marker: %s", got)
	}
	if len(got) > 140 {
		t.Fatalf("sanitized output length = %d, want <= 140", len(got))
	}
}

func TestSanitizeForAIDoesNotSplitUTF8(t *testing.T) {
	got, truncated := SanitizeForAI("prefix 中文 suffix", 10)
	if !truncated {
		t.Fatalf("expected truncation")
	}
	if !strings.HasPrefix(got, "prefix ") {
		t.Fatalf("unexpected prefix after truncation: %q", got)
	}
	if strings.Contains(got, "\uFFFD") {
		t.Fatalf("truncation produced replacement rune: %q", got)
	}
}

func TestContainsSecretPatternDetectsRedactableOutput(t *testing.T) {
	if !ContainsSecretPattern("Authorization: Bearer sk-live-secret") {
		t.Fatalf("expected authorization-like value to be detected")
	}
	if ContainsSecretPattern("Please reconnect your Threads account in UniPost.") {
		t.Fatalf("plain customer guidance should not be flagged")
	}
}

func TestRecipientSendStateAllowsRetryAfterFailureOnly(t *testing.T) {
	item := ItemState{
		Classification: ClassificationUserActionNeeded,
		ActionKind:     ActionKindEmail,
		WorkflowStatus: WorkflowStatusReady,
	}

	ok, reason := CanSendRecipient(item, RecipientState{Status: RecipientStatusPending}, true, "user@example.com")
	if !ok || reason != "" {
		t.Fatalf("pending recipient should be sendable, ok=%v reason=%q", ok, reason)
	}

	ok, reason = CanSendRecipient(item, RecipientState{Status: RecipientStatusSendFailed}, true, "user@example.com")
	if !ok || reason != "" {
		t.Fatalf("failed recipient should be retryable, ok=%v reason=%q", ok, reason)
	}

	ok, reason = CanSendRecipient(item, RecipientState{Status: RecipientStatusSent}, true, "user@example.com")
	if ok || reason != "recipient_already_final" {
		t.Fatalf("sent recipient should be blocked, ok=%v reason=%q", ok, reason)
	}

	ok, reason = CanSendRecipient(item, RecipientState{Status: RecipientStatusPending}, false, "user@example.com")
	if ok || reason != "loops_not_configured" {
		t.Fatalf("missing Loops should block send, ok=%v reason=%q", ok, reason)
	}
}

func TestSendIdempotencyKeyIsStablePerDraftVersion(t *testing.T) {
	first := SendIdempotencyKey("item_1", "workspace:ws_1:user:user_1", 1)
	retry := SendIdempotencyKey("item_1", "workspace:ws_1:user:user_1", 1)
	editedDraft := SendIdempotencyKey("item_1", "workspace:ws_1:user:user_1", 2)

	if first != retry {
		t.Fatalf("retry key changed: %q != %q", first, retry)
	}
	if first == editedDraft {
		t.Fatalf("edited draft should receive a distinct idempotency key")
	}
	if want := "error_triage:item_1:workspace:ws_1:user:user_1"; first != want {
		t.Fatalf("key = %q, want %q", first, want)
	}
}

func TestDeriveRunHealthStatus(t *testing.T) {
	cases := []struct {
		name  string
		items []ItemState
		want  RunHealthStatus
	}{
		{name: "empty", want: RunHealthNoActionableIssues},
		{name: "completed no action", items: []ItemState{{ActionKind: ActionKindNone, WorkflowStatus: WorkflowStatusCompleted}}, want: RunHealthNoActionableIssues},
		{name: "needs review", items: []ItemState{{ActionKind: ActionKindReview, WorkflowStatus: WorkflowStatusPendingReview}}, want: RunHealthNeedsReview},
		{name: "actionable", items: []ItemState{{ActionKind: ActionKindEmail, WorkflowStatus: WorkflowStatusReady}}, want: RunHealthActionableItems},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DeriveRunHealthStatus(tc.items); got != tc.want {
				t.Fatalf("health = %q, want %q", got, tc.want)
			}
		})
	}
}
