package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

func TestProcessBulkOneRejectsDuplicateSocialConnectionBeforePublish(t *testing.T) {
	h := &SocialPostHandler{}
	req := httptest.NewRequest(http.MethodPost, "/v1/social-posts/bulk", nil)
	body := publishRequestBody{PlatformPosts: []platformPostBody{
		{AccountID: "sa_dev", Caption: "same"},
		{AccountID: "sa_prod", Caption: "different"},
	}}
	accounts := map[string]platform.ValidateAccount{
		"sa_dev":  {Platform: "twitter", ConnectionID: "conn_1", ProfileID: "pr_dev"},
		"sa_prod": {Platform: "twitter", ConnectionID: "conn_1", ProfileID: "pr_prod"},
	}

	result, accepted := h.processBulkOne(
		req,
		"ws_1",
		body,
		accounts,
		quota.FreePlanHardBlockGate{},
		0,
		make(publishingRestrictionPolicySnapshot),
	)
	if result.Status != http.StatusUnprocessableEntity || result.Error == nil {
		t.Fatalf("result = %#v, want duplicate 422", result)
	}
	if result.Error.Code != "DUPLICATE_SOCIAL_CONNECTION" {
		t.Fatalf("error code = %q", result.Error.Code)
	}
	if accepted != 0 {
		t.Fatalf("accepted quota units = %d, want 0", accepted)
	}
	if len(result.Error.Issues) != 2 || result.Error.Details["payloads_match"] != false {
		t.Fatalf("error = %#v", result.Error)
	}
}

// TestBulkRequest_RejectEmpty — empty posts array → 422.
// Pure structural unit test that doesn't need a DB.
func TestBulkRequest_Marshal(t *testing.T) {
	body := bulkRequestBody{
		Posts: []publishRequestBody{
			{Caption: "post 1", AccountIDs: []string{"sa_1"}},
			{Caption: "post 2", AccountIDs: []string{"sa_2"}},
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var roundtrip bulkRequestBody
	if err := json.Unmarshal(b, &roundtrip); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(roundtrip.Posts) != 2 {
		t.Errorf("posts count: got %d, want 2", len(roundtrip.Posts))
	}
	if roundtrip.Posts[0].Caption != "post 1" {
		t.Errorf("caption: got %q", roundtrip.Posts[0].Caption)
	}
}

// TestBulkResultEntry_OmitsEmptyFields — Data and Error are
// mutually exclusive; the omitempty tags must keep the wire shape
// clean depending on which path produced the entry.
func TestBulkResultEntry_OmitsEmptyFields(t *testing.T) {
	successOnly := bulkResultEntry{
		Status: 200,
		Data:   &socialPostResponse{ID: "post_1"},
	}
	b, _ := json.Marshal(successOnly)
	if got := string(b); !strContains(got, `"data"`) || strContains(got, `"error"`) {
		t.Errorf("success entry should have data and not error: %s", got)
	}

	errorOnly := bulkResultEntry{
		Status: 422,
		Error:  &bulkErrorEnvelope{Code: "VALIDATION_ERROR", Message: "bad"},
	}
	b, _ = json.Marshal(errorOnly)
	if got := string(b); !strContains(got, `"error"`) || strContains(got, `"data"`) {
		t.Errorf("error entry should have error and not data: %s", got)
	}
}

// TestMaxBulkPosts locks the per-batch cap so a refactor can't
// silently bump it past what the synchronous handler can finish
// inside Railway's reverse-proxy timeout.
func TestMaxBulkPosts(t *testing.T) {
	if MaxBulkPosts != 50 {
		t.Errorf("MaxBulkPosts must be 50, got %d", MaxBulkPosts)
	}
}

func strContains(s, sub string) bool { //nolint:unused
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
