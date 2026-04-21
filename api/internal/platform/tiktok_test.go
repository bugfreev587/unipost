package platform

import "testing"

func TestBuildTikTokPostInfoIncludesRequiredToggles(t *testing.T) {
	info := buildTikTokPostInfo("hello", "PUBLIC_TO_EVERYONE", nil, "video")

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
	if got := info["disable_duet"]; got != false {
		t.Fatalf("disable_duet = %v, want false", got)
	}
	if got := info["disable_stitch"]; got != false {
		t.Fatalf("disable_stitch = %v, want false", got)
	}
}

func TestBuildTikTokPostInfoPhotoOmitsDuetStitch(t *testing.T) {
	info := buildTikTokPostInfo("hello", "PUBLIC_TO_EVERYONE", nil, "photo")

	if _, ok := info["disable_duet"]; ok {
		t.Fatal("photo post_info must not include disable_duet (TikTok rejects it)")
	}
	if _, ok := info["disable_stitch"]; ok {
		t.Fatal("photo post_info must not include disable_stitch (TikTok rejects it)")
	}
}

func TestShouldRetryTikTokWithSelfOnly(t *testing.T) {
	body := []byte(`{"error":{"code":"invalid_params","message":"Invalid authorization header. Please check the format."}}`)

	if !shouldRetryTikTokWithSelfOnly(400, body, "PUBLIC_TO_EVERYONE") {
		t.Fatal("expected retry for invalid_params with non-SELF_ONLY privacy")
	}
	if shouldRetryTikTokWithSelfOnly(400, body, "SELF_ONLY") {
		t.Fatal("did not expect retry when already using SELF_ONLY")
	}
	if shouldRetryTikTokWithSelfOnly(500, body, "PUBLIC_TO_EVERYONE") {
		t.Fatal("did not expect retry for non-400 responses")
	}
}

func TestTikTokPublicPostURLFromStatusData(t *testing.T) {
	data := map[string]any{
		"status":                      "PUBLISH_COMPLETE",
		"publicaly_available_post_id": []any{"7350123456789012345"},
	}
	got := TikTokPublicPostURLFromStatusData(data)
	want := "https://www.tiktok.com/player/v1/7350123456789012345"
	if got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
}

func TestTikTokPublicPostURLFromStatusDataMissingID(t *testing.T) {
	data := map[string]any{
		"status":                      "PUBLISH_COMPLETE",
		"publicaly_available_post_id": []any{},
	}
	if got := TikTokPublicPostURLFromStatusData(data); got != "" {
		t.Fatalf("url = %q, want empty", got)
	}
}
