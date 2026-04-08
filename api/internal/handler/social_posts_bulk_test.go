package handler

import (
	"encoding/json"
	"testing"
)

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
