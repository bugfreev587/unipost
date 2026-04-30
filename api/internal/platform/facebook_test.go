package platform

import "testing"

// TestResolvePostID exercises the pure-string canonicalizer the
// inbox sync uses to convert bare Facebook video / object ids into
// the "{page_id}_{story_id}" combined form Meta's modern Graph
// endpoints expect. The bare-id case is the actual fix path: this
// is what unblocks the "(#12) singular statuses API is deprecated"
// rejection on /{bare_id}/comments calls.
func TestResolvePostID(t *testing.T) {
	a := NewFacebookAdapter()
	cases := []struct {
		name   string
		pageID string
		id     string
		want   string
	}{
		{
			name:   "combined_passes_through",
			pageID: "999",
			id:     "123456_789012",
			want:   "123456_789012", // already combined; pageID ignored
		},
		{
			name:   "bare_gets_prefixed",
			pageID: "999888777",
			id:     "122331150824222923", // the production-failing shape
			want:   "999888777_122331150824222923",
		},
		{
			name:   "empty_pageID_returns_bare_unchanged",
			pageID: "",
			id:     "122331150824222923",
			want:   "122331150824222923",
		},
		{
			name:   "single_underscore_treated_as_combined",
			pageID: "999",
			id:     "a_b",
			want:   "a_b",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := a.ResolvePostID(c.pageID, c.id)
			if got != c.want {
				t.Errorf("ResolvePostID(%q, %q) = %q, want %q", c.pageID, c.id, got, c.want)
			}
		})
	}
}
