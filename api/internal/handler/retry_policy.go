package handler

import (
	"time"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type retryPolicyResponse struct {
	IsRetriable        bool    `json:"is_retriable"`
	WillRetry          bool    `json:"will_retry"`
	RetryState         string  `json:"retry_state"`
	NextRunAt          *string `json:"next_run_at,omitempty"`
	AttemptsMade       *int    `json:"attempts_made,omitempty"`
	MaxAttempts        *int    `json:"max_attempts,omitempty"`
	AttemptsRemaining  *int    `json:"attempts_remaining,omitempty"`
	ManualRetryAllowed bool    `json:"manual_retry_allowed"`
	Reason             string  `json:"reason,omitempty"`
}

func deriveRetryPolicy(result db.SocialPostResult, jobs []db.PostDeliveryJob) *retryPolicyResponse {
	isRetriable := result.IsRetriable.Valid && result.IsRetriable.Bool
	active := newestRelevantJob(result.ID, jobs, true)
	terminal := newestRelevantJob(result.ID, jobs, false)
	policy := &retryPolicyResponse{
		IsRetriable:        isRetriable,
		RetryState:         "not_retriable",
		ManualRetryAllowed: result.Status == "failed" && active == nil,
	}

	if active != nil {
		applyJobAttempts(policy, *active)
		switch active.State {
		case "pending":
			policy.WillRetry = true
			policy.RetryState = "scheduled"
			if active.NextRunAt.Valid {
				v := active.NextRunAt.Time.Format(time.RFC3339)
				policy.NextRunAt = &v
			}
		case "running", "retrying":
			policy.WillRetry = true
			policy.RetryState = "running"
		default:
			policy.RetryState = "unknown"
			policy.Reason = "active_job_unknown_state"
		}
		return policy
	}

	if result.Status != "failed" {
		policy.RetryState = "not_retriable"
		policy.Reason = "result_not_failed"
		return policy
	}

	if terminal == nil {
		if isRetriable {
			policy.RetryState = "manual_only"
			policy.Reason = "no_delivery_job"
		} else {
			policy.RetryState = "not_retriable"
			policy.Reason = "classification_not_retriable"
		}
		return policy
	}

	applyJobAttempts(policy, *terminal)
	switch terminal.State {
	case "dead":
		if isRetriable && terminal.Kind == "retry" && terminal.MaxAttempts > 0 && terminal.Attempts >= terminal.MaxAttempts {
			policy.RetryState = "exhausted"
			policy.Reason = "max_attempts_exhausted"
			return policy
		}
		policy.RetryState = "not_retriable"
		policy.Reason = "classification_not_retriable"
	case "cancelled":
		policy.RetryState = "manual_only"
		policy.Reason = "cancelled"
	case "failed":
		policy.RetryState = "unknown"
		policy.Reason = "no_retry_job"
	case "succeeded":
		policy.RetryState = "not_retriable"
		policy.Reason = "result_not_failed"
	default:
		policy.RetryState = "unknown"
		policy.Reason = "terminal_job_unknown_state"
	}
	return policy
}

func applyRetryPolicyToResponse(resp *postResultResponse, result db.SocialPostResult, jobs []db.PostDeliveryJob) {
	if resp == nil {
		return
	}
	if result.Status != "failed" &&
		newestRelevantJob(result.ID, jobs, true) == nil &&
		newestRelevantJob(result.ID, jobs, false) == nil {
		return
	}
	resp.RetryPolicy = deriveRetryPolicy(result, jobs)
}

func retryPolicyFromFailureDetails(status string, failure db.CreatePostFailureParams) *retryPolicyResponse {
	if status != "failed" {
		return nil
	}
	state := "not_retriable"
	reason := "classification_not_retriable"
	if failure.IsRetriable {
		state = "manual_only"
		reason = "no_delivery_job"
	}
	return &retryPolicyResponse{
		IsRetriable:        failure.IsRetriable,
		WillRetry:          false,
		RetryState:         state,
		ManualRetryAllowed: true,
		Reason:             reason,
	}
}

func newestRelevantJob(resultID string, jobs []db.PostDeliveryJob, activeOnly bool) *db.PostDeliveryJob {
	var newest *db.PostDeliveryJob
	for i := range jobs {
		job := jobs[i]
		if job.SocialPostResultID != resultID {
			continue
		}
		active := isActiveDeliveryJobState(job.State)
		if activeOnly != active {
			continue
		}
		if newest == nil || deliveryJobSortTime(job).After(deliveryJobSortTime(*newest)) {
			newest = &jobs[i]
		}
	}
	return newest
}

func isActiveDeliveryJobState(state string) bool {
	return state == "pending" || state == "running" || state == "retrying"
}

func deliveryJobSortTime(job db.PostDeliveryJob) time.Time {
	switch {
	case job.CreatedAt.Valid:
		return job.CreatedAt.Time
	case job.UpdatedAt.Valid:
		return job.UpdatedAt.Time
	case job.FinishedAt.Valid:
		return job.FinishedAt.Time
	default:
		return time.Time{}
	}
}

func applyJobAttempts(policy *retryPolicyResponse, job db.PostDeliveryJob) {
	attempts := int(job.Attempts)
	maxAttempts := int(job.MaxAttempts)
	remaining := maxAttempts - attempts
	if remaining < 0 {
		remaining = 0
	}
	policy.AttemptsMade = &attempts
	policy.MaxAttempts = &maxAttempts
	policy.AttemptsRemaining = &remaining
}
