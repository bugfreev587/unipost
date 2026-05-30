package handler

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// Sprint 4 PR4: white-label branding validation locks.

func TestIsHexColor(t *testing.T) {
	good := []string{"#10b981", "#000000", "#FFFFFF", "#aBcDeF"}
	for _, c := range good {
		if !isHexColor(c) {
			t.Errorf("%s should be a valid hex color", c)
		}
	}
	bad := []string{
		"",
		"10b981",   // missing #
		"#10b98",   // 5 digits
		"#10b9810", // 7 digits
		"#10b98z",  // non-hex char
		"rgb(16,185,129)",
		"red",
	}
	for _, c := range bad {
		if isHexColor(c) {
			t.Errorf("%q should NOT be a valid hex color", c)
		}
	}
}

func TestValidateBrandingLogoURL(t *testing.T) {
	good := []string{
		"https://acme.com/logo.png",
		"https://cdn.acme.com/static/logo-2026.svg",
	}
	for _, u := range good {
		if err := validateBrandingLogoURL(u); err != nil {
			t.Errorf("%s should be valid: %v", u, err)
		}
	}

	cases := []struct {
		in       string
		wantErr  bool
		contains string
	}{
		{"http://acme.com/logo.png", true, "https"},
		{"javascript:alert(1)", true, "https"},
		{"https://", true, "host"},
		{"not a url", true, "https"},
	}
	for _, c := range cases {
		err := validateBrandingLogoURL(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("validateBrandingLogoURL(%q): err=%v wantErr=%v", c.in, err, c.wantErr)
		}
	}

	// Length cap.
	long := "https://example.com/" + repeat("x", 600)
	if err := validateBrandingLogoURL(long); err == nil {
		t.Error("oversized logo URL should fail")
	}
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}

func TestValidateBrandingLogoUploadAcceptsPNGAndJPEG(t *testing.T) {
	for _, tc := range []struct {
		name            string
		body            []byte
		wantContentType string
		wantExt         string
	}{
		{"png", testPNGBytes(t), "image/png", ".png"},
		{"jpeg", testJPEGBytes(t), "image/jpeg", ".jpg"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateBrandingLogoUpload(tc.body)
			if err != nil {
				t.Fatalf("validateBrandingLogoUpload: %v", err)
			}
			if got.contentType != tc.wantContentType || got.ext != tc.wantExt {
				t.Fatalf("got %#v, want content type %q ext %q", got, tc.wantContentType, tc.wantExt)
			}
		})
	}
}

func TestValidateBrandingLogoUploadRejectsSVGAndOversize(t *testing.T) {
	if _, err := validateBrandingLogoUpload([]byte(`<svg></svg>`)); err == nil {
		t.Fatal("svg should be rejected")
	}
	if _, err := validateBrandingLogoUpload(make([]byte, brandingLogoMaxBytes+1)); err == nil {
		t.Fatal("oversize logo should be rejected")
	}
}

func TestBrandingLogoStorageKeysAreNeverDeletable(t *testing.T) {
	for _, key := range []string{
		"branding/ws_1/pr_1/logo_a.png",
		"branding/ws_2/pr_1/logo_a.png",
		"branding/ws_1/pr_2/logo_a.png",
		"media/file.png",
		"../branding/ws_1/pr_1/logo.png",
	} {
		if canDeleteBrandingLogoStorageKey(key, "ws_1", "pr_1") {
			t.Fatalf("key %q should never be deletable by profile branding cleanup", key)
		}
	}
}

func testPNGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 10, G: 185, B: 129, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func testJPEGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 37, G: 99, B: 235, A: 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}
