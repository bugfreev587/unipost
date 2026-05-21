package connect

import "testing"

func TestNewManagedConnector(t *testing.T) {
	cases := []struct {
		platform string
		wantType string
	}{
		{platform: "twitter", wantType: "*connect.TwitterConnector"},
		{platform: "linkedin", wantType: "*connect.LinkedInConnector"},
		{platform: "youtube", wantType: "*connect.YouTubeConnector"},
		{platform: "instagram", wantType: "*connect.InstagramConnector"},
		{platform: "tiktok", wantType: "*connect.TikTokConnector"},
		{platform: "threads", wantType: "*connect.ThreadsConnector"},
	}

	for _, tc := range cases {
		connector := NewManagedConnector(tc.platform, "client-id", "client-secret", "https://api.example.com")
		if connector == nil {
			t.Fatalf("%s: expected connector, got nil", tc.platform)
		}
		if got := connector.Platform(); got != tc.platform {
			t.Fatalf("%s: Platform() = %q", tc.platform, got)
		}
	}

	if got := NewManagedConnector("unknown", "id", "secret", "https://api.example.com"); got != nil {
		t.Fatalf("unknown platform: expected nil, got %#v", got)
	}
}
