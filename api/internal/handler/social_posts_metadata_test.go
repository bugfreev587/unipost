package handler

import (
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/platform"
)

// TestEncodeDecodeRoundTrip exercises the v2 metadata round-trip the
// scheduler will rely on in PR6. Whatever Create stores must come back
// out byte-equal so per-platform captions survive the trip through
// social_posts.metadata.
func TestEncodeDecodeRoundTrip(t *testing.T) {
	in := []platform.PlatformPostInput{
		{
			AccountID:       "acc_a",
			Caption:         "tweet caption",
			MediaURLs:       []string{"https://x/y.jpg"},
			PlatformOptions: map[string]any{"twitter_lang": "en"},
		},
		{
			AccountID: "acc_b",
			Caption:   "linkedin essay",
			InReplyTo: "li_post_123",
		},
	}
	raw, err := encodePostMetadata(in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out, err := decodePostMetadata(raw, "fallback")
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
		if i == 1 && out[i].InReplyTo != "li_post_123" {
			t.Errorf("in_reply_to lost on round trip")
		}
	}
}

// TestDecodeLegacyMetadata exercises the v1 → v2 expansion path. The
// scheduler must keep working for posts created BEFORE this PR landed.
func TestDecodeLegacyMetadata(t *testing.T) {
	legacy := []byte(`{"account_ids":["a","b","c"],"media_urls":["https://x/y.jpg"]}`)
	out, err := decodePostMetadata(legacy, "old caption")
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

// TestDecodeEmpty handles the nil-metadata case (e.g. immediate posts
// before PR5 that didn't store anything). Should return nil, nil so
// callers can fall back gracefully.
func TestDecodeEmpty(t *testing.T) {
	out, err := decodePostMetadata(nil, "x")
	if err != nil {
		t.Fatalf("decode nil: %v", err)
	}
	if out != nil {
		t.Errorf("expected nil slice, got %v", out)
	}
}
