package platform

import "testing"

func TestBuildTikTokPostInfoIncludesRequiredToggles(t *testing.T) {
	info := buildTikTokPostInfo("hello", "PUBLIC_TO_EVERYONE", nil)

	if got := info["privacy_level"]; got != "PUBLIC_TO_EVERYONE" {
		t.Fatalf("privacy_level = %v, want PUBLIC_TO_EVERYONE", got)
	}
	if got := info["disable_comment"]; got != false {
		t.Fatalf("disable_comment = %v, want false", got)
	}
	if got := info["auto_add_music"]; got != true {
		t.Fatalf("auto_add_music = %v, want true", got)
	}
	if got := info["brand_content_toggle"]; got != false {
		t.Fatalf("brand_content_toggle = %v, want false", got)
	}
	if got := info["brand_organic_toggle"]; got != false {
		t.Fatalf("brand_organic_toggle = %v, want false", got)
	}
}
