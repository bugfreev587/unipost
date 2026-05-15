package platform

import (
	"net/url"
	"os"
	"testing"
)

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

func TestTikTokOAuthScopesDefaultToApprovedProductionSet(t *testing.T) {
	t.Setenv("UNIPOST_ENV", "production")
	unsetenv(t, "TIKTOK_ANALYTICS_SCOPES_ENABLED")

	adapter := NewTikTokAdapter()
	config := adapter.DefaultOAuthConfig("https://api.unipost.dev")
	got := adapter.GetAuthURL(config, "state-1")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	want := "video.publish,video.upload,user.info.basic"
	if q := u.Query().Get("scope"); q != want {
		t.Fatalf("scope = %q, want %q", q, want)
	}
}

func TestTikTokOAuthScopesIncludeAnalyticsWhenEnabled(t *testing.T) {
	t.Setenv("UNIPOST_ENV", "production")
	t.Setenv("TIKTOK_ANALYTICS_SCOPES_ENABLED", "true")

	adapter := NewTikTokAdapter()
	config := adapter.DefaultOAuthConfig("https://api.unipost.dev")
	got := adapter.GetAuthURL(config, "state-1")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	want := "video.publish,video.upload,user.info.basic,user.info.profile,user.info.stats,video.list"
	if q := u.Query().Get("scope"); q != want {
		t.Fatalf("scope = %q, want %q", q, want)
	}
}

func TestTikTokOAuthScopesIncludeAnalyticsByDefaultOutsideProduction(t *testing.T) {
	t.Setenv("UNIPOST_ENV", "development")
	unsetenv(t, "TIKTOK_ANALYTICS_SCOPES_ENABLED")

	adapter := NewTikTokAdapter()
	config := adapter.DefaultOAuthConfig("https://dev-api.unipost.dev")
	got := adapter.GetAuthURL(config, "state-1")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	want := "video.publish,video.upload,user.info.basic,user.info.profile,user.info.stats,video.list"
	if q := u.Query().Get("scope"); q != want {
		t.Fatalf("scope = %q, want %q", q, want)
	}
}

func unsetenv(t *testing.T, name string) {
	t.Helper()
	old, ok := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("unset %s: %v", name, err)
	}
	t.Cleanup(func() {
		if ok {
			_ = os.Setenv(name, old)
		} else {
			_ = os.Unsetenv(name)
		}
	})
}

func TestTikTokBasicUserInfoFieldsStayWithinBasicScope(t *testing.T) {
	disallowed := map[string]bool{
		"username":         true,
		"profile_web_link": true,
		"is_verified":      true,
		"follower_count":   true,
		"following_count":  true,
		"likes_count":      true,
		"video_count":      true,
	}
	for _, field := range tiktokBasicUserInfoFields {
		if disallowed[field] {
			t.Fatalf("basic user-info fields include %q, which requires non-basic TikTok scopes", field)
		}
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

func TestTikTokProfileURL(t *testing.T) {
	got := TikTokProfileURL("@magicxiaobo")
	want := "https://www.tiktok.com/@magicxiaobo"
	if got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
}
