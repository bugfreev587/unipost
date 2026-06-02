package handler

import (
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/platform"
)

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
