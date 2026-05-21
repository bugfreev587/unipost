package platform

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

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

func TestFacebookOAuthScopesIncludeAnalyticsScopes(t *testing.T) {
	adapter := NewFacebookAdapter()
	config := adapter.DefaultOAuthConfig("https://dev-api.unipost.dev")
	got := adapter.GetAuthURL(config, "state-1")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	want := "business_management pages_show_list pages_manage_posts pages_read_engagement pages_read_user_content pages_manage_engagement pages_messaging pages_manage_metadata read_insights"
	if q := u.Query().Get("scope"); q != want {
		t.Fatalf("scope = %q, want %q", q, want)
	}
}

func TestFacebookFetchPagesCarriesBusinessMetadata(t *testing.T) {
	adapter := NewFacebookAdapter()
	adapter.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.Path; got != "/v22.0/me/accounts" {
			t.Fatalf("path = %q, want /me/accounts", got)
		}
		if fields := req.URL.Query().Get("fields"); !strings.Contains(fields, "business{id,name}") {
			t.Fatalf("fields = %q, want business metadata field", fields)
		}
		body := `{
			"data": [{
				"id": "page_1",
				"name": "Catherine's bakery store",
				"access_token": "page-token",
				"category": "Bakery",
				"tasks": ["CREATE_CONTENT"],
				"business": { "id": "biz_1", "name": "Bakery Group" },
				"picture": { "data": { "url": "https://example.com/page.jpg" } }
			}]
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}

	pages, err := adapter.FetchPages(context.Background(), "user-token")
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 {
		t.Fatalf("len(pages) = %d, want 1", len(pages))
	}
	got := pages[0]
	if got.BusinessID != "biz_1" {
		t.Fatalf("BusinessID = %q, want biz_1", got.BusinessID)
	}
	if got.BusinessName != "Bakery Group" {
		t.Fatalf("BusinessName = %q, want Bakery Group", got.BusinessName)
	}
	if got.BusinessRelationship != "page_business" {
		t.Fatalf("BusinessRelationship = %q, want page_business", got.BusinessRelationship)
	}
}

func TestFacebookFetchBusinessPageRelationships(t *testing.T) {
	adapter := NewFacebookAdapter()
	seen := map[string]bool{}
	adapter.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		seen[req.URL.Path] = true
		var body string
		switch req.URL.Path {
		case "/v22.0/biz_1/owned_pages":
			body = `{"data":[{"id":"page_owned"}]}`
		case "/v22.0/biz_1/client_pages":
			body = `{"data":[{"id":"page_client"},{"id":"page_owned"}]}`
		default:
			t.Fatalf("unexpected path %q", req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}

	got, err := adapter.FetchBusinessPageRelationships(context.Background(), "user-token", []FacebookBusiness{
		{ID: "biz_1", Name: "Bakery Group"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !seen["/v22.0/biz_1/owned_pages"] || !seen["/v22.0/biz_1/client_pages"] {
		t.Fatalf("expected owned_pages and client_pages calls, saw %#v", seen)
	}
	if got["page_owned"].BusinessRelationship != "owned" {
		t.Fatalf("owned relationship = %q, want owned", got["page_owned"].BusinessRelationship)
	}
	if got["page_client"].BusinessRelationship != "client" {
		t.Fatalf("client relationship = %q, want client", got["page_client"].BusinessRelationship)
	}
	if got["page_client"].BusinessID != "biz_1" || got["page_client"].BusinessName != "Bakery Group" {
		t.Fatalf("client business = %#v, want biz_1/Bakery Group", got["page_client"])
	}
}

func TestFacebookFetchCommentsCarriesAuthorMetadata(t *testing.T) {
	adapter := NewFacebookAdapter()
	adapter.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.Path; got != "/v22.0/page_1_post_1/comments" {
			t.Fatalf("path = %q, want comments edge", got)
		}
		if fields := req.URL.Query().Get("fields"); !strings.Contains(fields, "from{id,name") {
			t.Fatalf("fields = %q, want author fields", fields)
		}
		body := `{
			"data": [{
				"id": "comment_1",
				"message": "Hello Disneyland",
				"from": {
					"id": "user_1",
					"name": "Jack Ma",
					"picture": { "data": { "url": "https://example.com/jack.jpg" } }
				},
				"created_time": "2026-05-20T18:47:30+0000",
				"parent": { "id": "page_1_post_1" },
				"comments": { "data": [] }
			}]
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}

	entries, err := adapter.FetchComments(context.Background(), "page-token", "page_1_post_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	got := entries[0]
	if got.AuthorName != "Jack Ma" {
		t.Fatalf("AuthorName = %q, want Jack Ma", got.AuthorName)
	}
	if got.AuthorID != "user_1" {
		t.Fatalf("AuthorID = %q, want user_1", got.AuthorID)
	}
	if got.AuthorAvatarURL != "https://example.com/jack.jpg" {
		t.Fatalf("AuthorAvatarURL = %q, want avatar URL", got.AuthorAvatarURL)
	}
}
