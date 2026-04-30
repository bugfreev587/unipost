package platform

import (
	"context"
	"testing"
)

// TestResolvePostID_PassThrough locks down the contract that ids
// already in the canonical "{page_id}_{story_id}" combined form
// short-circuit without making a Graph call. This is the hot path
// the inbox sync hits on every tick after the bare-id resolve fix
// has already canonicalized the row, so it must NOT touch the
// network — a regression here would burn a Graph quota call per
// post per sync interval.
//
// We pass an empty access token to make the assertion sharp: if
// the function ever started making the HTTP request for combined
// ids, Meta would respond with an OAuth error and our context
// would surface it. The empty-token call here returns instantly
// because the Contains("_") check fires first.
func TestResolvePostID_PassThrough(t *testing.T) {
	a := NewFacebookAdapter()
	cases := []string{
		"123456_789012",                        // realistic combined form
		"100012345678901_10101010101010101010", // long both sides
		"a_b",                                  // anything with "_"
	}
	for _, id := range cases {
		got, err := a.ResolvePostID(context.Background(), "", id)
		if err != nil {
			t.Errorf("ResolvePostID(%q) errored: %v (should have short-circuited)", id, err)
		}
		if got != id {
			t.Errorf("ResolvePostID(%q) = %q, want unchanged", id, got)
		}
	}
}
