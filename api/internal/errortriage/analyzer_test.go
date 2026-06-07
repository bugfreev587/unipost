package errortriage

import "testing"

func TestDeterministicAnalyzerClassifiesUserReconnect(t *testing.T) {
	analyzer := DeterministicAnalyzer{}
	item := analyzer.Analyze(Bucket{
		Key: "triage:auth",
		Failures: []Failure{{
			Platform:     "threads",
			ErrorCode:    "missing_permission",
			FailureStage: "publish",
			Message:      "Threads permissions are missing; reconnect the account",
			UserEmail:    "owner@example.com",
		}},
	})

	if item.Classification != ClassificationUserActionNeeded {
		t.Fatalf("classification = %q, want %q", item.Classification, ClassificationUserActionNeeded)
	}
	if item.ActionKind != ActionKindEmail || item.WorkflowStatus != WorkflowStatusReady {
		t.Fatalf("action/status = %q/%q, want email/ready", item.ActionKind, item.WorkflowStatus)
	}
	if item.EmailDraft.Subject == "" || item.EmailDraft.Body == "" {
		t.Fatalf("expected email draft, got %#v", item.EmailDraft)
	}
}

func TestDeterministicAnalyzerClassifiesUnipostBug(t *testing.T) {
	analyzer := DeterministicAnalyzer{}
	item := analyzer.Analyze(Bucket{
		Key: "triage:invalid-params",
		Failures: []Failure{{
			Platform:          "tiktok",
			ErrorCode:         "invalid_params",
			PlatformErrorCode: "invalid_params",
			FailureStage:      "upload_init",
			Message:           "The chunk size is invalid",
		}},
	})

	if item.Classification != ClassificationUnipostBug {
		t.Fatalf("classification = %q, want %q", item.Classification, ClassificationUnipostBug)
	}
	if item.ActionKind != ActionKindBugPlan || item.WorkflowStatus != WorkflowStatusReady {
		t.Fatalf("action/status = %q/%q, want bug_plan/ready", item.ActionKind, item.WorkflowStatus)
	}
	if item.BugPlan.Title == "" || item.BugPlan.ValidationPlan == "" {
		t.Fatalf("expected bug plan, got %#v", item.BugPlan)
	}
}

func TestBuildBucketsCreatesRecipientsPerWorkspaceOwner(t *testing.T) {
	failures := []Failure{
		{PostID: "post_1", WorkspaceID: "ws_1", UserID: "user_1", UserEmail: "one@example.com", Platform: "threads", Source: "dashboard", ErrorCode: "missing_permission", FailureStage: "publish", Message: "reconnect required"},
		{PostID: "post_2", WorkspaceID: "ws_1", UserID: "user_1", UserEmail: "one@example.com", Platform: "threads", Source: "dashboard", ErrorCode: "missing_permission", FailureStage: "publish", Message: "reconnect required again"},
		{PostID: "post_3", WorkspaceID: "ws_2", UserID: "user_2", UserEmail: "two@example.com", Platform: "threads", Source: "dashboard", ErrorCode: "missing_permission", FailureStage: "publish", Message: "reconnect required"},
	}

	buckets := BuildBuckets(failures)
	if len(buckets) != 1 {
		t.Fatalf("bucket count = %d, want 1", len(buckets))
	}
	if got, want := len(buckets[0].Recipients), 2; got != want {
		t.Fatalf("recipient count = %d, want %d", got, want)
	}
	if got, want := buckets[0].AffectedPostCount, 3; got != want {
		t.Fatalf("affected posts = %d, want %d", got, want)
	}
}
