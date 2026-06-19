package errortriage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestOpenAIAnalyzerMergesReviewAnalysis(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := `{
			"classification":"needs_human_review",
			"confidence":0.74,
			"summary":"Review the Instagram worker timeout.",
			"review_analysis":{
				"what_is_this_error":"Instagram hit a worker timeout while returning a generic platform error.",
				"why_it_happened":"The provider did not return a specific terminal state before UniPost stopped waiting.",
				"how_to_resolve":"Open the raw failure, inspect the provider response and worker timeout logs, then classify it before contacting the customer.",
				"missing_evidence":"Provider async processing status is missing.",
				"next_inspection_path":"Inspect the post failure row and social post result for the linked sample."
			}
		}`
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": content}}},
		})
	}))
	defer server.Close()

	analyzer := NewOpenAIAnalyzer("test-key", "test-model", server.URL, server.Client(), DeterministicAnalyzer{})
	item := analyzer.Analyze(testHumanReviewBucket())

	analysis, ok := item.Evidence["review_analysis"].(map[string]string)
	if !ok {
		t.Fatalf("review_analysis missing or wrong type: %#v", item.Evidence["review_analysis"])
	}
	if got, want := analysis["what_is_this_error"], "Instagram hit a worker timeout while returning a generic platform error."; got != want {
		t.Fatalf("what_is_this_error = %q, want %q", got, want)
	}
	if got, want := analysis["how_to_resolve"], "Open the raw failure, inspect the provider response and worker timeout logs, then classify it before contacting the customer."; got != want {
		t.Fatalf("how_to_resolve = %q, want %q", got, want)
	}
}

func TestOpenAIAnalyzerPreservesReviewAnalysisOnSafetyHumanReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := `{
			"classification":"user_action_needed",
			"confidence":0.92,
			"summary":"Maybe ask the customer to reconnect.",
			"email_draft":{"subject":"Reconnect","body":"Please reconnect."},
			"safety":{"requires_human_review":true,"reason":"customer-facing action is ambiguous"}
		}`
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": content}}},
		})
	}))
	defer server.Close()

	analyzer := NewOpenAIAnalyzer("test-key", "test-model", server.URL, server.Client(), DeterministicAnalyzer{})
	item := analyzer.Analyze(testHumanReviewBucket())

	analysis, ok := item.Evidence["review_analysis"].(map[string]string)
	if !ok {
		t.Fatalf("review_analysis missing or wrong type: %#v", item.Evidence["review_analysis"])
	}
	if !strings.Contains(analysis["why_it_happened"], "customer-facing action is ambiguous") {
		t.Fatalf("safety reason should be preserved in review analysis: %#v", analysis)
	}
	if analysis["how_to_resolve"] == "" {
		t.Fatalf("how_to_resolve should still be present: %#v", analysis)
	}
	if item.EmailDraft.Subject != "" || item.EmailDraft.Body != "" {
		t.Fatalf("unsafe email draft should be cleared: %#v", item.EmailDraft)
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

func TestOpenAIAnalyzerMarksLowConfidenceOutputForHumanReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := `{
			"classification":"user_action_needed",
			"confidence":0.42,
			"summary":"Maybe reconnect.",
			"email_draft":{"subject":"Reconnect","body":"Please reconnect."}
		}`
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": content}}},
		})
	}))
	defer server.Close()

	analyzer := NewOpenAIAnalyzer("test-key", "test-model", server.URL, server.Client(), DeterministicAnalyzer{})
	item := analyzer.Analyze(testUserActionBucket())

	if item.Classification != ClassificationNeedsHumanReview {
		t.Fatalf("classification = %q, want human review", item.Classification)
	}
	if item.ActionKind != ActionKindReview || item.WorkflowStatus != WorkflowStatusPendingReview {
		t.Fatalf("workflow = %q/%q, want review/pending", item.ActionKind, item.WorkflowStatus)
	}
	if item.EmailDraft.Subject != "" || item.EmailDraft.Body != "" {
		t.Fatalf("unsafe low-confidence email draft should be cleared: %#v", item.EmailDraft)
	}
}

func TestOpenAIAnalyzerHonorsSafetyHumanReviewFlag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := `{
			"classification":"user_action_needed",
			"confidence":0.91,
			"summary":"Reconnect needed.",
			"email_draft":{"subject":"Reconnect","body":"Please reconnect."},
			"safety":{"requires_human_review":true,"reason":"ambiguous customer impact"}
		}`
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": content}}},
		})
	}))
	defer server.Close()

	analyzer := NewOpenAIAnalyzer("test-key", "test-model", server.URL, server.Client(), DeterministicAnalyzer{})
	item := analyzer.Analyze(testUserActionBucket())

	if item.Classification != ClassificationNeedsHumanReview {
		t.Fatalf("classification = %q, want human review", item.Classification)
	}
	if item.EmailDraft.Subject != "" {
		t.Fatalf("email draft should be cleared when safety requests review")
	}
	if item.Summary == "" || item.Summary == "Reconnect needed." {
		t.Fatalf("summary should explain human review gate, got %q", item.Summary)
	}
}

func TestOpenAIAnalyzerBlocksSecretShapedEmailOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := `{
			"classification":"user_action_needed",
			"confidence":0.91,
			"summary":"Reconnect needed.",
			"email_draft":{"subject":"Reconnect","body":"Authorization: Bearer sk-secret should never be here."}
		}`
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": content}}},
		})
	}))
	defer server.Close()

	analyzer := NewOpenAIAnalyzer("test-key", "test-model", server.URL, server.Client(), DeterministicAnalyzer{})
	item := analyzer.Analyze(testUserActionBucket())

	if item.Classification != ClassificationNeedsHumanReview {
		t.Fatalf("classification = %q, want human review", item.Classification)
	}
	if item.EmailDraft.Body != "" {
		t.Fatalf("secret-shaped email output should be cleared: %q", item.EmailDraft.Body)
	}
}

func testUserActionBucket() Bucket {
	return Bucket{
		Key:                    "triage:key",
		AffectedUserCount:      1,
		AffectedWorkspaceCount: 1,
		AffectedPostCount:      1,
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
			ErrorCode:    "missing_permission",
			FailureStage: "publish",
			Message:      "reconnect required",
			CreatedAt:    time.Now(),
		}},
	}
}

func testHumanReviewBucket() Bucket {
	return Bucket{
		Key:                    "triage:key",
		AffectedUserCount:      1,
		AffectedWorkspaceCount: 1,
		AffectedPostCount:      1,
		LatestFailureAt:        time.Now(),
		Failures: []Failure{{
			PostID:             "post_1",
			SocialPostResultID: "spr_1",
			PostFailureID:      "pf_1",
			WorkspaceID:        "ws_1",
			UserID:             "user_1",
			UserEmail:          "user@example.com",
			Platform:           "instagram",
			Source:             "api",
			ErrorCode:          "platform_error",
			FailureStage:       "worker_timeout",
			Message:            "Instagram worker timed out while waiting for provider processing",
			CreatedAt:          time.Now(),
		}},
	}
}
