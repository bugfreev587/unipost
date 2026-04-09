package handler

import "testing"

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
		"10b981",         // missing #
		"#10b98",         // 5 digits
		"#10b9810",       // 7 digits
		"#10b98z",        // non-hex char
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
