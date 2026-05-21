package handler

import (
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/platform"
)

func TestDecodePendingFacebookConnectionAcceptsLegacyPageArray(t *testing.T) {
	got, err := decodePendingFacebookConnection([]byte(`[
		{"id":"page_1","name":"Bakery","tasks":["CREATE_CONTENT"],"page_access_token_enc":"enc"}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Pages) != 1 {
		t.Fatalf("len(Pages) = %d, want 1", len(got.Pages))
	}
	if got.Pages[0].ID != "page_1" {
		t.Fatalf("page id = %q, want page_1", got.Pages[0].ID)
	}
	if len(got.Businesses) != 0 {
		t.Fatalf("len(Businesses) = %d, want 0", len(got.Businesses))
	}
}

func TestDecodePendingFacebookConnectionAcceptsBusinessWrappedPayload(t *testing.T) {
	got, err := decodePendingFacebookConnection([]byte(`{
		"pages":[{"id":"page_1","name":"Bakery","business_id":"biz_1","business_name":"Bakery Group"}],
		"businesses":[{"id":"biz_1","name":"Bakery Group"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Pages) != 1 || got.Pages[0].BusinessID != "biz_1" {
		t.Fatalf("pages = %#v, want one page with biz_1", got.Pages)
	}
	if len(got.Businesses) != 1 || got.Businesses[0].Name != "Bakery Group" {
		t.Fatalf("businesses = %#v, want Bakery Group", got.Businesses)
	}
}

func TestApplyFacebookBusinessContextPrefersOwnedRelationship(t *testing.T) {
	pages := []platform.FacebookPage{{ID: "page_1", Name: "Bakery"}}
	got := applyFacebookBusinessContext(pages, []platform.FacebookBusiness{
		{ID: "biz_1", Name: "Bakery Group"},
	}, map[string]platform.FacebookBusinessPageRelationship{
		"page_1": {
			BusinessID:           "biz_1",
			BusinessName:         "Bakery Group",
			BusinessRelationship: "owned",
		},
	})
	if got[0].BusinessID != "biz_1" {
		t.Fatalf("BusinessID = %q, want biz_1", got[0].BusinessID)
	}
	if got[0].BusinessRelationship != "owned" {
		t.Fatalf("BusinessRelationship = %q, want owned", got[0].BusinessRelationship)
	}
}
