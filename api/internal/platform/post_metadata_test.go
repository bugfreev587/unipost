package platform

import "testing"

// TestEncodeDecodeRoundTrip exercises the v2 metadata round-trip the
// scheduler relies on. Whatever Create stores must come back out byte-
// equal so per-platform captions survive the trip through
// social_posts.metadata.
func TestEncodeDecodeRoundTrip(t *testing.T) {
	in := []PlatformPostInput{
		{
			AccountID:           "acc_a",
			Caption:             "tweet caption",
			MediaURLs:           []string{"https://x/y.jpg"},
			PlatformOptions:     map[string]any{"twitter_lang": "en"},
			DispatchEnvironment: "sandbox",
		},
		{
			AccountID: "acc_b",
			Caption:   "linkedin essay",
			InReplyTo: "li_post_123",
		},
	}
	raw, err := EncodePostMetadata(in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out, err := DecodePostMetadata(raw, "fallback")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("expected %d, got %d", len(in), len(out))
	}
	for i := range in {
		if in[i].AccountID != out[i].AccountID || in[i].Caption != out[i].Caption {
			t.Errorf("post %d: round-trip mismatch in=%+v out=%+v", i, in[i], out[i])
		}
		if i == 0 && len(out[i].MediaURLs) != 1 {
			t.Errorf("media urls lost on round trip")
		}
		if i == 0 && out[i].DispatchEnvironment != "sandbox" {
			t.Errorf("dispatch environment lost on round trip: %q", out[i].DispatchEnvironment)
		}
		if i == 1 && out[i].InReplyTo != "li_post_123" {
			t.Errorf("in_reply_to lost on round trip")
		}
	}
}

func TestDecodeLegacyMetadataLeavesDispatchEnvironmentEmpty(t *testing.T) {
	raw := []byte(`{"schema_version":2,"platform_posts":[{"account_id":"pin_legacy","caption":"legacy"}]}`)
	posts, err := DecodePostMetadata(raw, "")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(posts) != 1 || posts[0].DispatchEnvironment != "" {
		t.Fatalf("legacy dispatch environment = %#v, want empty", posts)
	}
}

// TestDecodeLegacyMetadata exercises the v1 → v2 expansion path. The
// scheduler must keep working for posts created BEFORE Sprint 1.
func TestDecodeLegacyMetadata(t *testing.T) {
	legacy := []byte(`{"account_ids":["a","b","c"],"media_urls":["https://x/y.jpg"]}`)
	out, err := DecodePostMetadata(legacy, "old caption")
	if err != nil {
		t.Fatalf("decode legacy: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 expanded posts, got %d", len(out))
	}
	for _, p := range out {
		if p.Caption != "old caption" {
			t.Errorf("legacy expansion should fall back to parent caption, got %q", p.Caption)
		}
		if len(p.MediaURLs) != 1 {
			t.Errorf("legacy media should propagate, got %v", p.MediaURLs)
		}
	}
}

// TestDecodeEmpty handles the nil-metadata case so callers can fall
// back gracefully.
func TestDecodeEmpty(t *testing.T) {
	out, err := DecodePostMetadata(nil, "x")
	if err != nil {
		t.Fatalf("decode nil: %v", err)
	}
	if out != nil {
		t.Errorf("expected nil slice, got %v", out)
	}
}

// TestLegacyV1Metadata returns the platform_options map for a v1
// row, nil for a v2 row.
func TestLegacyV1Metadata(t *testing.T) {
	v1 := []byte(`{"account_ids":["a"],"platform_options":{"tiktok":{"privacy_level":"PUBLIC"}}}`)
	opts := LegacyV1Metadata(v1)
	if opts == nil {
		t.Fatal("expected v1 options, got nil")
	}
	if opts["tiktok"]["privacy_level"] != "PUBLIC" {
		t.Errorf("v1 options round trip failed: %v", opts)
	}

	v2 := []byte(`{"schema_version":2,"platform_posts":[]}`)
	if got := LegacyV1Metadata(v2); got != nil {
		t.Errorf("expected nil for v2 row, got %v", got)
	}
}
