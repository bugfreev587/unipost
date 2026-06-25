package handler

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestDeriveRetryPolicyScheduledRetryJob(t *testing.T) {
	next := time.Date(2026, 6, 23, 22, 0, 30, 0, time.UTC)
	result := failedResult(true)
	policy := deriveRetryPolicy(result, []db.PostDeliveryJob{{
		ID:                 "job_retry",
		SocialPostResultID: result.ID,
		Kind:               "retry",
		State:              "pending",
		Attempts:           1,
		MaxAttempts:        5,
		NextRunAt:          pgtype.Timestamptz{Time: next, Valid: true},
		CreatedAt:          pgtype.Timestamptz{Time: next.Add(-time.Minute), Valid: true},
	}})

	if policy == nil {
		t.Fatal("policy = nil")
	}
	if !policy.IsRetriable || !policy.WillRetry || policy.RetryState != "scheduled" {
		t.Fatalf("policy retry state = %#v", policy)
	}
	if policy.NextRunAt == nil || *policy.NextRunAt != next.Format(time.RFC3339) {
		t.Fatalf("NextRunAt = %#v, want %s", policy.NextRunAt, next.Format(time.RFC3339))
	}
	if policy.AttemptsMade == nil || *policy.AttemptsMade != 1 {
		t.Fatalf("AttemptsMade = %#v, want 1", policy.AttemptsMade)
	}
	if policy.AttemptsRemaining == nil || *policy.AttemptsRemaining != 4 {
		t.Fatalf("AttemptsRemaining = %#v, want 4", policy.AttemptsRemaining)
	}
	if policy.ManualRetryAllowed {
		t.Fatal("manual retry should be false while an active job exists")
	}
}

func TestDeriveRetryPolicyExhaustedKeepsRetriableClassification(t *testing.T) {
	now := time.Date(2026, 6, 23, 22, 5, 0, 0, time.UTC)
	result := failedResult(true)
	policy := deriveRetryPolicy(result, []db.PostDeliveryJob{{
		ID:                 "job_retry",
		SocialPostResultID: result.ID,
		Kind:               "retry",
		State:              "dead",
		Attempts:           5,
		MaxAttempts:        5,
		ErrorCode:          pgtype.Text{String: "temporary_platform_error", Valid: true},
		CreatedAt:          pgtype.Timestamptz{Time: now, Valid: true},
		FinishedAt:         pgtype.Timestamptz{Time: now, Valid: true},
	}})

	if policy == nil {
		t.Fatal("policy = nil")
	}
	if !policy.IsRetriable {
		t.Fatal("is_retriable should remain true after attempts are exhausted")
	}
	if policy.WillRetry || policy.RetryState != "exhausted" || policy.Reason != "max_attempts_exhausted" {
		t.Fatalf("policy = %#v, want exhausted without automatic retry", policy)
	}
	if policy.AttemptsRemaining == nil || *policy.AttemptsRemaining != 0 {
		t.Fatalf("AttemptsRemaining = %#v, want 0", policy.AttemptsRemaining)
	}
	if !policy.ManualRetryAllowed {
		t.Fatal("manual retry should be allowed for failed rows with no active job")
	}
}

func TestDeriveRetryPolicyManualRetryEligibilityMirrorsEndpoint(t *testing.T) {
	failed := deriveRetryPolicy(failedResult(false), nil)
	if failed == nil || !failed.ManualRetryAllowed {
		t.Fatalf("failed row without active job should allow manual retry: %#v", failed)
	}

	published := db.SocialPostResult{
		ID:     "result_1",
		Status: "published",
	}
	publishedPolicy := deriveRetryPolicy(published, nil)
	if publishedPolicy == nil || publishedPolicy.ManualRetryAllowed {
		t.Fatalf("published row should not allow manual retry: %#v", publishedPolicy)
	}

	active := deriveRetryPolicy(failedResult(true), []db.PostDeliveryJob{{
		ID:                 "job_active",
		SocialPostResultID: "result_1",
		Kind:               "retry",
		State:              "running",
		Attempts:           2,
		MaxAttempts:        5,
	}})
	if active == nil || active.ManualRetryAllowed || !active.WillRetry || active.RetryState != "running" {
		t.Fatalf("active job policy = %#v", active)
	}
}

func TestApplyRetryPolicyToPostResultResponse(t *testing.T) {
	next := time.Date(2026, 6, 23, 22, 0, 30, 0, time.UTC)
	result := failedResult(true)
	response := postResultResponseFromDBResult(result, accountSummary{Platform: "instagram"})

	applyRetryPolicyToResponse(&response, result, []db.PostDeliveryJob{{
		ID:                 "job_retry",
		SocialPostResultID: result.ID,
		Kind:               "retry",
		State:              "pending",
		Attempts:           1,
		MaxAttempts:        5,
		NextRunAt:          pgtype.Timestamptz{Time: next, Valid: true},
	}})

	if response.RetryPolicy == nil {
		t.Fatal("RetryPolicy = nil")
	}
	if !response.RetryPolicy.WillRetry || response.RetryPolicy.RetryState != "scheduled" {
		t.Fatalf("RetryPolicy = %#v, want scheduled automatic retry", response.RetryPolicy)
	}
}

func failedResult(isRetriable bool) db.SocialPostResult {
	return db.SocialPostResult{
		ID:          "result_1",
		Status:      "failed",
		IsRetriable: pgtype.Bool{Bool: isRetriable, Valid: true},
	}
}
