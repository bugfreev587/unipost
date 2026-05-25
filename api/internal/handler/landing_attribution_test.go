package handler

import "testing"

func TestNormalizeLandingCodeCanonicalizesKnownAliases(t *testing.T) {
	cases := map[string]string{
		"ProductHunt":   "ph",
		"product_hunt":  "ph",
		"product-hunt":  "ph",
		"twitter":       "x",
		"t.co":          "x",
		"redd.it":       "rd",
		"indie hackers": "ih",
		"Google_Ads":    "google",
		"facebook":      "meta",
		"bing":          "microsoft",
	}

	for raw, want := range cases {
		if got := normalizeLandingCode(raw); got != want {
			t.Fatalf("normalizeLandingCode(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestNormalizeLandingCodeRejectsInvalidValues(t *testing.T) {
	for _, raw := range []string{"", "two words", "<script>", "source.with.dot", "abcdefghijklmnopqrstuvwxyz123456789"} {
		if got := normalizeLandingCode(raw); got != "" {
			t.Fatalf("normalizeLandingCode(%q) = %q, want empty", raw, got)
		}
	}
}

func TestResolveSourceWithAttributionPrefersCanonicalUTM(t *testing.T) {
	h := NewLandingAttributionHandler(nil)

	got := h.resolveSourceWithAttribution("rd", "", map[string]string{
		"utm_source": "producthunt",
	})
	if got != "ph" {
		t.Fatalf("source = %q, want ph", got)
	}
}

func TestResolveSourceWithAttributionUsesOtherForUnknownUTM(t *testing.T) {
	h := NewLandingAttributionHandler(nil)

	got := h.resolveSourceWithAttribution("", "", map[string]string{
		"utm_source": "newsletter",
	})
	if got != "o" {
		t.Fatalf("source = %q, want o", got)
	}
}

func TestAdminPathBreakdownRowUsesPathLabel(t *testing.T) {
	h := NewLandingAttributionHandler(nil)

	got := h.adminPathBreakdownRow("/docs/api", 12)

	if got.Path != "/docs/api" {
		t.Fatalf("Path = %q, want /docs/api", got.Path)
	}
	if got.Label != "/docs/api" {
		t.Fatalf("Label = %q, want /docs/api", got.Label)
	}
	if got.Count != 12 {
		t.Fatalf("Count = %d, want 12", got.Count)
	}
}

func TestSanitizeLandingAttributionKeepsAllowedKeys(t *testing.T) {
	got := sanitizeLandingAttribution(map[string]string{
		"utm_source":   "ProductHunt",
		"utm_medium":   "social",
		"utm_campaign": "Launch 2026",
		"email":        "hidden@example.com",
	})

	if len(got) != 3 {
		t.Fatalf("got %d keys, want 3: %#v", len(got), got)
	}
	if got["utm_campaign"] != "Launch 2026" {
		t.Fatalf("utm_campaign = %q", got["utm_campaign"])
	}
	if _, ok := got["email"]; ok {
		t.Fatalf("unexpected email key in attribution: %#v", got)
	}
}

func TestSanitizeLandingAttributionExpandsShortAliases(t *testing.T) {
	got := sanitizeLandingAttribution(map[string]string{
		"s": "ph",
		"m": "launch",
		"c": "l0526",
	})

	if got["utm_source"] != "ph" {
		t.Fatalf("utm_source = %q, want ph", got["utm_source"])
	}
	if got["utm_medium"] != "launch" {
		t.Fatalf("utm_medium = %q, want launch", got["utm_medium"])
	}
	if got["utm_campaign"] != "l0526" {
		t.Fatalf("utm_campaign = %q, want l0526", got["utm_campaign"])
	}
}

func TestLandingBotUserAgentFilterDoesNotMatchSafariPreview(t *testing.T) {
	if isLandingBotUserAgent("Mozilla/5.0 Safari Technology Preview") {
		t.Fatal("Safari Technology Preview should not be filtered as a bot")
	}
	if !isLandingBotUserAgent("LinkedInBot/1.0") {
		t.Fatal("LinkedInBot should be filtered")
	}
	if !isLandingBotUserAgent("WhatsApp/2.0") {
		t.Fatal("WhatsApp link preview should be filtered")
	}
}
