package mediaprocessing

import (
	"testing"
	"time"
)

func TestEvaluateAdmissionCountsRetryWaitAsActive(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	decision := EvaluateAdmission(Limits{ActiveJobs: 2, GIFConversions24H: 50}, 2, 0, time.Time{}, now)
	if decision.Code != AdmissionCapacityExceeded || decision.RetryAfter != 30*time.Second {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestEvaluateAdmissionReturnsRollingReset(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	oldest := now.Add(-23 * time.Hour)
	decision := EvaluateAdmission(Limits{ActiveJobs: 2, GIFConversions24H: 50}, 1, 50, oldest, now)
	if decision.Code != AdmissionGIFRateExceeded || !decision.ResetAt.Equal(now.Add(time.Hour)) || decision.RetryAfter != time.Hour {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestEvaluateAdmissionAcceptsBelowBothLimits(t *testing.T) {
	decision := EvaluateAdmission(Limits{ActiveJobs: 2, GIFConversions24H: 50}, 1, 49, time.Time{}, time.Now())
	if decision.Code != AdmissionAccepted {
		t.Fatalf("decision = %#v", decision)
	}
}
