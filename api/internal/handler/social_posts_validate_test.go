package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/platform"
)

func TestWriteDuplicateSocialConnectionErrorUsesDedicated422Contract(t *testing.T) {
	posts := []platform.PlatformPostInput{
		{AccountID: "sa_twitter_dev", Caption: "v1.4 is live"},
		{AccountID: "sa_twitter_staging", Caption: "v1.4 is live"},
	}
	accounts := map[string]platform.ValidateAccount{
		"sa_twitter_dev":     {Platform: "twitter", ConnectionID: "conn_1", ProfileID: "pr_dev"},
		"sa_twitter_staging": {Platform: "twitter", ConnectionID: "conn_1", ProfileID: "pr_staging"},
	}
	conflict, ok := firstDuplicateSocialConnectionConflict(posts, accounts)
	if !ok {
		t.Fatal("expected duplicate physical connection conflict")
	}

	rr := httptest.NewRecorder()
	writeDuplicateSocialConnectionError(rr, conflict)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
	var got ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Error.Code != "DUPLICATE_SOCIAL_CONNECTION" || got.Error.NormalizedCode != "duplicate_social_connection" {
		t.Fatalf("error code = %#v", got.Error)
	}
	if len(got.Error.Issues) != 2 {
		t.Fatalf("issues = %#v, want two", got.Error.Issues)
	}
	if got.Error.Details["platform"] != "twitter" {
		t.Fatalf("details = %#v", got.Error.Details)
	}
	if _, leaked := got.Error.Details["connection_id"]; leaked {
		t.Fatalf("details leaked connection_id: %#v", got.Error.Details)
	}
}

func TestParsedRequestResolveLegacyPlatformOptions(t *testing.T) {
	body := publishRequestBody{
		Caption:    "Demo upload",
		MediaURLs:  []string{"https://cdn.example.com/demo.mp4"},
		AccountIDs: []string{"acc_youtube", "acc_tiktok"},
		PlatformOptions: map[string]map[string]any{
			"youtube": {
				"title":         "Demo upload title",
				"tags":          []any{"demo", "mandarin"},
				"made_for_kids": false,
			},
			"tiktok": {
				"privacy_level": "PUBLIC_TO_EVERYONE",
			},
		},
	}

	parsed, status, msg := parsePublishRequest(body)
	if status != 0 {
		t.Fatalf("parse status = %d, msg = %q", status, msg)
	}

	parsed.resolveLegacyPlatformOptions(map[string]platform.ValidateAccount{
		"acc_youtube": {Platform: "youtube"},
		"acc_tiktok":  {Platform: "tiktok"},
	})

	youtubeOpts := parsed.Posts[0].PlatformOptions
	if youtubeOpts["title"] != "Demo upload title" {
		t.Fatalf("youtube title option = %v, want Demo upload title", youtubeOpts["title"])
	}
	if tags, ok := youtubeOpts["tags"].([]any); !ok || len(tags) != 2 {
		t.Fatalf("youtube tags option = %#v, want two tags", youtubeOpts["tags"])
	}
	if youtubeOpts["privacy_level"] != nil {
		t.Fatalf("youtube options should not include TikTok privacy_level: %#v", youtubeOpts)
	}

	tiktokOpts := parsed.Posts[1].PlatformOptions
	if tiktokOpts["privacy_level"] != "PUBLIC_TO_EVERYONE" {
		t.Fatalf("tiktok privacy option = %v, want PUBLIC_TO_EVERYONE", tiktokOpts["privacy_level"])
	}
	if tiktokOpts["title"] != nil {
		t.Fatalf("tiktok options should not include YouTube title: %#v", tiktokOpts)
	}
}

func TestStampPinterestDispatchEnvironmentsOnlyTouchesPinterest(t *testing.T) {
	t.Setenv("PINTEREST_USE_SANDBOX", "true")
	posts := []platform.PlatformPostInput{
		{AccountID: "pin_1"},
		{AccountID: "ig_1"},
		{AccountID: "missing"},
	}
	stampPinterestDispatchEnvironments(posts, map[string]platform.ValidateAccount{
		"pin_1": {Platform: "pinterest"},
		"ig_1":  {Platform: "instagram"},
	})

	if posts[0].DispatchEnvironment != "sandbox" {
		t.Fatalf("Pinterest environment = %q, want sandbox", posts[0].DispatchEnvironment)
	}
	if posts[1].DispatchEnvironment != "" || posts[2].DispatchEnvironment != "" {
		t.Fatalf("non-Pinterest markers changed: %#v", posts)
	}
}

func TestParsePublishRequestRejectsPlatformScopedOptionsInPlatformPosts(t *testing.T) {
	body := publishRequestBody{
		PlatformPosts: []platformPostBody{
			{
				AccountID: "acc_instagram",
				Caption:   "story",
				PlatformOptions: map[string]any{
					"instagram": map[string]any{"mediaType": "story"},
				},
			},
		},
	}

	_, status, msg := parsePublishRequest(body)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("parse status = %d, want 422", status)
	}
	if !strings.Contains(msg, "platform_posts[0].platform_options.instagram") ||
		!strings.Contains(msg, "new platform_posts shape") ||
		!strings.Contains(msg, "flat") {
		t.Fatalf("parse message = %q, want field-specific strict-format guidance", msg)
	}
}

func TestParsePublishRequestRejectsTopLevelPlatformOptionsWithPlatformPosts(t *testing.T) {
	body := publishRequestBody{
		PlatformOptions: map[string]map[string]any{
			"instagram": {"mediaType": "story"},
		},
		PlatformPosts: []platformPostBody{
			{
				AccountID: "acc_instagram",
				Caption:   "story",
			},
		},
	}

	_, status, msg := parsePublishRequest(body)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("parse status = %d, want 422", status)
	}
	if !strings.Contains(msg, "top-level platform_options") || !strings.Contains(msg, "platform_posts[].platform_options") {
		t.Fatalf("parse message = %q, want top-level platform_options guidance", msg)
	}
}

func TestParsePublishRequestAllowsFlatPlatformOptionsInPlatformPosts(t *testing.T) {
	body := publishRequestBody{
		PlatformPosts: []platformPostBody{
			{
				AccountID: "acc_instagram",
				Caption:   "story",
				PlatformOptions: map[string]any{
					"mediaType": "story",
				},
			},
		},
	}

	parsed, status, msg := parsePublishRequest(body)
	if status != 0 {
		t.Fatalf("parse status = %d, msg = %q", status, msg)
	}
	if len(parsed.Posts) != 1 || parsed.Posts[0].PlatformOptions["mediaType"] != "story" {
		t.Fatalf("parsed posts = %#v, want flat mediaType option preserved", parsed.Posts)
	}
}

func TestTikTokImageWithin1080p(t *testing.T) {
	tests := []struct {
		name         string
		width        int
		height       int
		expectWithin bool
	}{
		{name: "portrait 1080p", width: 1080, height: 1920, expectWithin: true},
		{name: "landscape 1080p", width: 1920, height: 1080, expectWithin: true},
		{name: "square 1080", width: 1080, height: 1080, expectWithin: true},
		{name: "too tall", width: 1080, height: 2400, expectWithin: false},
		{name: "too wide", width: 2200, height: 1080, expectWithin: false},
		{name: "square too large", width: 1500, height: 1500, expectWithin: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tiktokImageWithin1080p(tc.width, tc.height); got != tc.expectWithin {
				t.Fatalf("tiktokImageWithin1080p(%d, %d) = %v, want %v", tc.width, tc.height, got, tc.expectWithin)
			}
		})
	}
}
