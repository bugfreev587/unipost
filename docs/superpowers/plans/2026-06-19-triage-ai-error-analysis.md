# Triage AI Error Analysis Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add actionable AI/fallback analysis and raw failure links to admin Error Triage review buckets.

**Architecture:** Persist structured review guidance inside existing `evidence_json` so the storage schema stays stable. Render that evidence in the current admin Triage item card as an additive analysis and samples section. Update repo workflow instructions so feature flags are explicit opt-in only.

**Tech Stack:** Go backend service tests and analyzer logic, Next.js/React Dashboard page, TypeScript API types, Markdown repo instructions.

---

## File Structure

- Modify `api/internal/errortriage/analyzer.go`: add review-analysis evidence and concrete sample IDs.
- Modify `api/internal/errortriage/ai_analyzer.go`: request, parse, and merge AI-provided review analysis while keeping deterministic fallback.
- Modify `api/internal/errortriage/analyzer_test.go`: red/green tests for fallback review analysis and sample IDs.
- Modify `api/internal/errortriage/ai_analyzer_test.go`: red/green tests for AI review-analysis merge and safety fallback preservation.
- Modify `dashboard/src/lib/api.ts`: add typed evidence/sample interfaces.
- Modify `dashboard/src/app/admin/error-triage/page.tsx`: render AI analysis and failure samples with raw-error links.
- Modify `AGENTS.md`: change feature flag default instruction to opt-in.

## Task 1: Backend Evidence Shape

- [x] **Step 1: Write failing deterministic analyzer tests**

Add tests to `api/internal/errortriage/analyzer_test.go`:

```go
func TestDeterministicAnalyzerAddsReviewAnalysisForHumanReview(t *testing.T) {
	analyzer := DeterministicAnalyzer{}
	item := analyzer.Analyze(Bucket{
		Key:                    "triage:worker-timeout",
		AffectedUserCount:      1,
		AffectedWorkspaceCount: 1,
		AffectedPostCount:      1,
		Failures: []Failure{{
			PostID:             "post_123",
			SocialPostResultID: "spr_123",
			PostFailureID:      "pf_123",
			WorkspaceID:        "ws_123",
			Platform:           "instagram",
			Source:             "api",
			ErrorCode:          "platform_error",
			FailureStage:       "worker_timeout",
			Message:            "Instagram worker timed out while waiting for provider processing",
		}},
	})

	analysis, ok := item.Evidence["review_analysis"].(map[string]string)
	if !ok {
		t.Fatalf("review_analysis missing or wrong type: %#v", item.Evidence["review_analysis"])
	}
	if analysis["what_is_this_error"] == "" {
		t.Fatalf("what_is_this_error should be present: %#v", analysis)
	}
	if analysis["why_it_happened"] == "" {
		t.Fatalf("why_it_happened should be present: %#v", analysis)
	}
	if analysis["how_to_resolve"] == "" {
		t.Fatalf("how_to_resolve should be present: %#v", analysis)
	}
}

func TestBuildEvidenceIncludesConcreteFailureIdentifiers(t *testing.T) {
	evidence := buildEvidence(Bucket{
		Failures: []Failure{{
			PostID:             "post_123",
			SocialPostResultID: "spr_123",
			PostFailureID:      "pf_123",
			WorkspaceID:        "ws_123",
			Platform:           "youtube",
			ErrorCode:          "platform_error",
			FailureStage:       "worker_timeout",
			Message:            "YouTube worker timed out",
		}},
	})
	samples, ok := evidence["samples"].([]map[string]any)
	if !ok || len(samples) != 1 {
		t.Fatalf("samples missing: %#v", evidence["samples"])
	}
	if samples[0]["post_failure_id"] != "pf_123" {
		t.Fatalf("post_failure_id missing: %#v", samples[0])
	}
	if samples[0]["social_post_result_id"] != "spr_123" {
		t.Fatalf("social_post_result_id missing: %#v", samples[0])
	}
}
```

- [x] **Step 2: Run red test**

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/errortriage -run 'TestDeterministicAnalyzerAddsReviewAnalysisForHumanReview|TestBuildEvidenceIncludesConcreteFailureIdentifiers'`

Expected: FAIL because `review_analysis`, `post_failure_id`, and `social_post_result_id` are not present.

- [x] **Step 3: Implement deterministic evidence**

Add a `ReviewAnalysis` type, add `ReviewAnalysis` to `ItemDraft`, generate deterministic review text from classification/platform/error/stage/message, attach it to `Evidence`, and include concrete IDs in sample maps.

- [x] **Step 4: Run green test**

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/errortriage -run 'TestDeterministicAnalyzerAddsReviewAnalysisForHumanReview|TestBuildEvidenceIncludesConcreteFailureIdentifiers'`

Expected: PASS.

## Task 2: AI Review Analysis Merge

- [x] **Step 1: Write failing AI merge tests**

Add tests to `api/internal/errortriage/ai_analyzer_test.go` proving AI `review_analysis` is merged and safety downgrades retain deterministic review analysis.

- [x] **Step 2: Run red test**

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/errortriage -run 'TestOpenAIAnalyzerMergesReviewAnalysis|TestOpenAIAnalyzerPreservesReviewAnalysisOnSafetyHumanReview'`

Expected: FAIL because AI suggestions do not yet parse or merge `review_analysis`.

- [x] **Step 3: Implement AI review analysis**

Extend `aiTriageSuggestion`, prompt instructions, `mergeAISuggestion`, and `aiNeedsHumanReview` to carry safe structured review analysis without exposing email drafts or bug plans on unsafe review paths.

- [x] **Step 4: Run green test**

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/errortriage -run 'TestOpenAIAnalyzerMergesReviewAnalysis|TestOpenAIAnalyzerPreservesReviewAnalysisOnSafetyHumanReview'`

Expected: PASS.

## Task 3: Dashboard Rendering

- [x] **Step 1: Update typed evidence contracts**

Add TypeScript interfaces for `ErrorTriageEvidence`, `ErrorTriageEvidenceSample`, and `ErrorTriageReviewAnalysis` in `dashboard/src/lib/api.ts`.

- [x] **Step 2: Render analysis and samples**

Update `dashboard/src/app/admin/error-triage/page.tsx` with helpers that read `item.evidence_json`, display What / Why / How rows, display failure samples, and create `/admin/errors` links using `post_failure_id` or `post_id`.

- [x] **Step 3: Build dashboard**

Run: `npm run build`

Expected: PASS.

## Task 4: AGENTS.md Instruction Update

- [x] **Step 1: Edit feature flag rules**

Change `AGENTS.md` so agents do not proactively ask for feature flag protection. A flag is added only when the user explicitly requests one.

- [x] **Step 2: Verify wording**

Run: `rg -n "feature flag|Before starting implementation|explicitly" AGENTS.md`

Expected: no instruction remains requiring proactive pre-implementation feature-flag prompts.

## Task 5: Final Validation and Standard Flow

- [x] **Step 1: Run backend checks on task branch**

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/errortriage`

Run: `GOCACHE=/tmp/unipost-go-build go test ./...`

Expected: PASS.

- [x] **Step 2: Run dashboard check on task branch**

Run from `dashboard/`: `npm run build`

Expected: PASS.

- [ ] **Step 3: Merge to local dev**

In the main repo checkout, inspect `git status`, update local `dev` from `origin/dev`, merge `dev-triage-ai-error-analysis`, and preserve unrelated local user files.

- [ ] **Step 4: Rerun required checks on local dev**

Run the same backend and dashboard validation on local `dev`.

- [ ] **Step 5: Push dev and monitor**

Push local `dev` to `origin/dev`, monitor triggered checks/deployments until complete, then verify the development admin Triage page at the appropriate dev domain.
