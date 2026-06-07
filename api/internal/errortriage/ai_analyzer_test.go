package errortriage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenAIAnalyzerMergesStructuredResponseWithDeterministicFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization header = %q", got)
		}
		content := `{
			"classification":"user_action_needed",
			"confidence":0.91,
			"summary":"Customer must reconnect Threads.",
			"email_draft":{"subject":"Reconnect Threads","body":"Please reconnect Threads before retrying."}
		}`
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": content}}},
		})
	}))
	defer server.Close()

	bucket := Bucket{
		Key:                    "triage:key",
		AffectedUserCount:      2,
		AffectedWorkspaceCount: 2,
		AffectedPostCount:      3,
		LatestFailureAt:        time.Now(),
		Recipients: []RecipientCandidate{{
			ScopeKey:    "workspace:ws_1:user:user_1",
			WorkspaceID: "ws_1",
			UserID:      "user_1",
			Email:       "user@example.com",
		}},
		Failures: []Failure{{
			PostID:       "post_1",
			WorkspaceID:  "ws_1",
			UserID:       "user_1",
			UserEmail:    "user@example.com",
			Platform:     "threads",
			ErrorCode:    "bad_request",
			FailureStage: "publish",
			Message:      "bad request",
			CreatedAt:    time.Now(),
		}},
	}

	analyzer := NewOpenAIAnalyzer("test-key", "test-model", server.URL, server.Client(), DeterministicAnalyzer{})
	item := analyzer.Analyze(bucket)

	if item.DedupeKey != bucket.Key {
		t.Fatalf("dedupe key = %q, want deterministic key", item.DedupeKey)
	}
	if item.AffectedPostCount != 3 || item.AffectedUserCount != 2 {
		t.Fatalf("affected counts were not preserved: users=%d posts=%d", item.AffectedUserCount, item.AffectedPostCount)
	}
	if item.Classification != ClassificationUserActionNeeded {
		t.Fatalf("classification = %q", item.Classification)
	}
	if item.ActionKind != ActionKindEmail || item.WorkflowStatus != WorkflowStatusReady {
		t.Fatalf("workflow = %q/%q", item.ActionKind, item.WorkflowStatus)
	}
	if item.EmailDraft.Subject != "Reconnect Threads" {
		t.Fatalf("email subject = %q", item.EmailDraft.Subject)
	}
	if item.BugPlan.Title != "" {
		t.Fatalf("bug plan should have been cleared for email action")
	}
}

func TestOpenAIAnalyzerFallsBackOnProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	bucket := Bucket{
		Key:                    "triage:key",
		AffectedUserCount:      1,
		AffectedWorkspaceCount: 1,
		AffectedPostCount:      1,
		LatestFailureAt:        time.Now(),
		Failures: []Failure{{
			Platform:     "instagram",
			ErrorCode:    "internal_error",
			FailureStage: "publish",
			Message:      "nil pointer",
			CreatedAt:    time.Now(),
		}},
	}

	analyzer := NewOpenAIAnalyzer("test-key", "test-model", server.URL, server.Client(), DeterministicAnalyzer{})
	item := analyzer.Analyze(bucket)

	if item.Classification != ClassificationUnipostBug {
		t.Fatalf("classification = %q, want deterministic unipost bug fallback", item.Classification)
	}
	if item.BugPlan.Title == "" {
		t.Fatalf("expected deterministic bug plan fallback")
	}
}
