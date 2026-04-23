package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRetryDeliveryJobNowMarksDeprecated(t *testing.T) {
	h := &SocialPostHandler{}
	req := httptest.NewRequest(http.MethodPost, "/v1/post-delivery-jobs//retry-now", nil)
	rr := httptest.NewRecorder()

	h.RetryDeliveryJobNow(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when job id is missing", rr.Code)
	}
	if got := rr.Header().Get("Deprecation"); got != "true" {
		t.Fatalf("Deprecation header = %q, want true", got)
	}
	if got := rr.Header().Get("Sunset"); got == "" {
		t.Fatal("expected Sunset header on legacy retry-now alias")
	}
	if got := rr.Header().Get("Link"); got == "" {
		t.Fatal("expected Link header pointing to canonical retry route")
	}
}
