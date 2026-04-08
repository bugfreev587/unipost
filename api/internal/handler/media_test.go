package handler

import "testing"

// TestPickExt covers the filename → extension fallback the media
// handler uses to give R2 a recognizable storage_key.
func TestPickExt(t *testing.T) {
	cases := []struct {
		filename, contentType, want string
	}{
		{"photo.jpg", "image/jpeg", ".jpg"},      // filename wins
		{"PHOTO.JPG", "image/jpeg", ".jpg"},      // case insensitive
		{"", "image/png", ".png"},                // fallback to content type
		{"", "video/mp4", ".mp4"},                // video fallback
		{"", "video/quicktime", ".mov"},          // mov fallback
		{"", "application/octet-stream", ".bin"}, // unknown
		{"unknown", "", ".bin"},                  // truly unknown
		{"a.tar.gz", "", ".gz"},                  // last ext wins (path.Ext semantics)
	}
	for _, c := range cases {
		got := pickExt(c.filename, c.contentType)
		if got != c.want {
			t.Errorf("pickExt(%q, %q) = %q, want %q", c.filename, c.contentType, got, c.want)
		}
	}
}

// TestPickContentType locks down the "trust R2 over the client" rule
// the hydration path uses. The client can lie at create time about
// content type, but R2 echoes back what was actually uploaded — that's
// the authoritative value.
func TestPickContentType(t *testing.T) {
	if got := pickContentType("image/png", "image/jpeg"); got != "image/png" {
		t.Errorf("R2 value should win: got %q", got)
	}
	if got := pickContentType("", "image/jpeg"); got != "image/jpeg" {
		t.Errorf("client fallback when R2 missing: got %q", got)
	}
}

// TestAllowedMimeTypes guards against accidentally narrowing the
// allowlist when refactoring. If you intentionally remove a type,
// update this test.
func TestAllowedMimeTypes(t *testing.T) {
	required := []string{
		"image/jpeg", "image/png", "image/webp", "image/gif",
		"video/mp4", "video/quicktime", "video/webm",
	}
	for _, m := range required {
		if !allowedMimeTypes[m] {
			t.Errorf("required mime %q not in allowlist", m)
		}
	}
}
