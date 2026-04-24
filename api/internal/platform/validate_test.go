package platform

import (
	"strings"
	"testing"
	"time"
)

// helper that returns a stub Capabilities map sized down to just the
// platforms each test needs. Tests should NOT pull from the global
// Capabilities map so the assertions stay decoupled from any real
// limit changes.
func stubCapabilities() map[string]Capability {
	return map[string]Capability{
		"twitter": {
			DisplayName: "Twitter / X",
			Text:        TextCapability{MaxLength: 280, SupportsThreads: true},
			Media: MediaCapability{
				AllowMixed: false,
				Images:     ImageCapability{MaxCount: 4},
				Videos:     VideoCapability{MaxCount: 1},
			},
			Thread:       ThreadCapability{Supported: true},
			FirstComment: FirstCommentCapability{Supported: true, MaxLength: 280},
		},
		"instagram": {
			DisplayName: "Instagram",
			Text:        TextCapability{MaxLength: 2200},
			Media: MediaCapability{
				RequiresMedia: true,
				AllowMixed:    true,
				Images:        ImageCapability{MaxCount: 10},
				Videos:        VideoCapability{MaxCount: 1},
			},
			FirstComment: FirstCommentCapability{Supported: true, MaxLength: 2200},
		},
		"linkedin": {
			DisplayName: "LinkedIn",
			Text:        TextCapability{MaxLength: 3000},
			Media: MediaCapability{
				AllowMixed: false,
				Images:     ImageCapability{MaxCount: 9},
				Videos:     VideoCapability{MaxCount: 1},
			},
			FirstComment: FirstCommentCapability{Supported: true, MaxLength: 1250},
		},
		"tiktok": {
			DisplayName: "TikTok",
			Text:        TextCapability{MaxLength: 2200},
			Media: MediaCapability{
				RequiresMedia: true,
				AllowMixed:    false,
				Images:        ImageCapability{MaxCount: 35, MaxFileSizeBytes: 20 * 1024 * 1024, AllowedFormats: []string{"jpg", "jpeg", "webp"}},
				Videos:        VideoCapability{MaxCount: 1, AllowedFormats: []string{"mp4", "mov", "webm"}},
			},
		},
		"bluesky": {
			DisplayName: "Bluesky",
			Text:        TextCapability{MaxLength: 300},
			Media: MediaCapability{
				AllowMixed: false,
				Images:     ImageCapability{MaxCount: 4, MaxFileSizeBytes: 2_000_000},
				Videos:     VideoCapability{MaxCount: 1},
			},
			Thread:       ThreadCapability{Supported: true},
			FirstComment: FirstCommentCapability{Supported: false},
		},
		"youtube": {
			DisplayName: "YouTube",
			Text:        TextCapability{MaxLength: 5000},
			Media: MediaCapability{
				RequiresMedia: true,
				Images:        ImageCapability{MaxCount: 0},
				Videos:        VideoCapability{MaxCount: 1},
			},
		},
		"facebook": {
			DisplayName: "Facebook Page",
			Text:        TextCapability{MaxLength: 63206},
			Media: MediaCapability{
				AllowMixed: false,
				Images:     ImageCapability{MaxCount: 1},
				Videos:     VideoCapability{MaxCount: 1},
			},
		},
	}
}

func stubAccounts() map[string]ValidateAccount {
	return map[string]ValidateAccount{
		"acc_twitter":   {Platform: "twitter"},
		"acc_instagram": {Platform: "instagram"},
		"acc_linkedin":  {Platform: "linkedin"},
		"acc_tiktok":    {Platform: "tiktok"},
		"acc_bluesky":   {Platform: "bluesky"},
		"acc_youtube":   {Platform: "youtube"},
		"acc_facebook":  {Platform: "facebook"},
		"acc_dead":      {Platform: "twitter", Disconnected: true},
		"acc_alien":     {Platform: "myspace"}, // unknown_platform path
	}
}

// hasError returns true if the result contains an error matching the
// given code at the given index. Tests use this in place of asserting
// the full slice so additional unrelated errors don't cause failures.
func hasError(t *testing.T, res ValidationResult, idx int, code string) {
	t.Helper()
	for _, e := range res.Errors {
		if e.PlatformPostIndex == idx && e.Code == code {
			return
		}
	}
	t.Errorf("expected error code %q at index %d, got errors: %#v", code, idx, res.Errors)
}

func hasNoError(t *testing.T, res ValidationResult, code string) {
	t.Helper()
	for _, e := range res.Errors {
		if e.Code == code {
			t.Errorf("did not expect error code %q, but found one: %+v", code, e)
		}
	}
}

func validOpts(posts []PlatformPostInput) ValidateOptions {
	return ValidateOptions{
		Capabilities: stubCapabilities(),
		Accounts:     stubAccounts(),
		Posts:        posts,
		Now:          time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC),
	}
}

// ─── happy path ───────────────────────────────────────────────────────

func TestValidate_HappyPath(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_twitter", Caption: "hi twitter"},
		{AccountID: "acc_linkedin", Caption: "hi linkedin", MediaURLs: []string{"https://x/y.jpg"}},
	}))
	if !res.Valid {
		t.Fatalf("expected valid, got %#v", res.Errors)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("unexpected warnings: %#v", res.Warnings)
	}
}

// ─── empty / oversized request ────────────────────────────────────────

func TestValidate_EmptyPosts(t *testing.T) {
	res := ValidatePlatformPosts(validOpts(nil))
	if res.Valid {
		t.Fatal("expected invalid")
	}
	if res.Errors[0].Code != CodeEmptyPosts {
		t.Errorf("expected empty_posts, got %q", res.Errors[0].Code)
	}
}

func TestValidate_TooManyPosts(t *testing.T) {
	posts := make([]PlatformPostInput, MaxPlatformPosts+1)
	for i := range posts {
		posts[i] = PlatformPostInput{AccountID: "acc_twitter", Caption: "x"}
	}
	res := ValidatePlatformPosts(validOpts(posts))
	if res.Valid {
		t.Fatal("expected invalid")
	}
	hasError(t, res, 0, CodeTooManyPosts)
}

// ─── account resolution errors ────────────────────────────────────────

func TestValidate_AccountNotFound(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "", Caption: "hi"},
	}))
	hasError(t, res, 0, CodeAccountNotFound)
}

func TestValidate_AccountNotInWorkspace(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_someone_else", Caption: "hi"},
	}))
	hasError(t, res, 0, CodeAccountNotInWorkspace)
}

func TestValidate_UnknownPlatform(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_alien", Caption: "hi"},
	}))
	hasError(t, res, 0, CodeUnknownPlatform)
}

func TestValidate_AccountDisconnected(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_dead", Caption: "hi"},
	}))
	hasError(t, res, 0, CodeAccountDisconnected)
}

// ─── caption length ───────────────────────────────────────────────────

func TestValidate_ExceedsMaxLength(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_twitter", Caption: strings.Repeat("a", 281)},
	}))
	hasError(t, res, 0, CodeExceedsMaxLength)
}

func TestValidate_BelowMinLength(t *testing.T) {
	caps := stubCapabilities()
	tw := caps["twitter"]
	tw.Text.MinLength = 5
	caps["twitter"] = tw

	res := ValidatePlatformPosts(ValidateOptions{
		Capabilities: caps,
		Accounts:     stubAccounts(),
		Posts:        []PlatformPostInput{{AccountID: "acc_twitter", Caption: "hi"}},
		Now:          time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC),
	})
	hasError(t, res, 0, CodeBelowMinLength)
}

func TestValidate_CaptionMissingRequired(t *testing.T) {
	caps := stubCapabilities()
	tw := caps["twitter"]
	tw.Text.Required = true
	caps["twitter"] = tw

	res := ValidatePlatformPosts(ValidateOptions{
		Capabilities: caps,
		Accounts:     stubAccounts(),
		Posts:        []PlatformPostInput{{AccountID: "acc_twitter", Caption: ""}},
		Now:          time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC),
	})
	hasError(t, res, 0, CodeMissingRequired)
}

func TestValidate_YouTubeRequiresMadeForKids(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{
			AccountID:       "acc_youtube",
			Caption:         "Demo upload",
			MediaURLs:       []string{"https://x/video.mp4"},
			PlatformOptions: map[string]any{"privacy_status": "public", "title": "Demo upload"},
		},
	}))
	hasError(t, res, 0, CodeYouTubeMadeForKidsRequired)
}

func TestValidate_YouTubeRequiresExplicitTitle(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{
			AccountID: "acc_youtube",
			Caption:   "Description only",
			MediaURLs: []string{"https://x/video.mp4"},
			PlatformOptions: map[string]any{
				"made_for_kids":  true,
				"privacy_status": "public",
			},
		},
	}))
	hasError(t, res, 0, CodeYouTubeTitleRequired)
}

func TestValidate_YouTubePublishAtRequiresPrivate(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{
			AccountID: "acc_youtube",
			Caption:   "Demo upload",
			MediaURLs: []string{"https://x/video.mp4"},
			PlatformOptions: map[string]any{
				"made_for_kids":  true,
				"privacy_status": "public",
				"publish_at":     "2026-05-01T09:00:00Z",
			},
		},
	}))
	hasError(t, res, 0, CodeYouTubePublishAtRequiresPrivate)
}

func TestValidate_YouTubeAcceptsFullMetadata(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{
			AccountID: "acc_youtube",
			Caption:   "Demo upload",
			MediaURLs: []string{"https://x/video.mp4"},
			PlatformOptions: map[string]any{
				"title":                    "My Demo",
				"made_for_kids":            false,
				"privacy_status":           "private",
				"license":                  "youtube",
				"default_language":         "en-US",
				"publish_at":               "2026-05-01T09:00:00Z",
				"recording_date":           "2026-04-18",
				"notify_subscribers":       true,
				"embeddable":               true,
				"public_stats_viewable":    true,
				"contains_synthetic_media": false,
				"playlist_id":              "PL123",
				"tags":                     []any{"demo", "launch"},
			},
		},
	}))
	if !res.Valid {
		t.Fatalf("expected valid, got %#v", res.Errors)
	}
}

// ─── media count + mixing ─────────────────────────────────────────────

func TestValidate_RequiresMedia(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_instagram", Caption: "hi"},
	}))
	hasError(t, res, 0, CodeMissingRequired)
}

func TestValidate_MaxImagesExceeded(t *testing.T) {
	urls := make([]string, 5)
	for i := range urls {
		urls[i] = "https://x/img.jpg"
	}
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_twitter", Caption: "hi", MediaURLs: urls},
	}))
	hasError(t, res, 0, CodeMaxImagesExceeded)
}

func TestValidate_ImagesNotSupported(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_youtube", Caption: "hi", MediaURLs: []string{"https://x/img.jpg"}},
	}))
	// YouTube has Images.MaxCount = 0, so any image trips the same code.
	hasError(t, res, 0, CodeMaxImagesExceeded)
}

func TestValidate_MaxVideosExceeded(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_twitter", Caption: "hi", MediaURLs: []string{
			"https://x/a.mp4",
			"https://x/b.mp4",
		}},
	}))
	hasError(t, res, 0, CodeMaxVideosExceeded)
}

func TestValidate_MixedMediaUnsupported(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_twitter", Caption: "hi", MediaURLs: []string{
			"https://x/img.jpg",
			"https://x/clip.mp4",
		}},
	}))
	hasError(t, res, 0, CodeMixedMediaUnsupported)
}

func TestValidate_MixedAllowedOnInstagramCarousel(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_instagram", Caption: "hi", MediaURLs: []string{
			"https://x/img.jpg",
			"https://x/clip.mp4",
		}},
	}))
	hasNoError(t, res, CodeMixedMediaUnsupported)
}

func TestValidate_InstagramRejectsInvalidMediaType(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{
			AccountID:       "acc_instagram",
			Caption:         "hi",
			MediaURLs:       []string{"https://x/img.jpg"},
			PlatformOptions: map[string]any{"mediaType": "timeline"},
		},
	}))
	hasError(t, res, 0, CodeInvalidInstagramMediaType)
}

func TestValidate_InstagramReelsRequireExactlyOneVideo(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{
			AccountID:       "acc_instagram",
			Caption:         "hi",
			MediaURLs:       []string{"https://x/img.jpg"},
			PlatformOptions: map[string]any{"mediaType": "reels"},
		},
	}))
	hasError(t, res, 0, CodeInstagramReelsRequireVideo)
}

func TestValidate_InstagramStoryRequiresSingleMedia(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{
			AccountID: "acc_instagram",
			Caption:   "hi",
			MediaURLs: []string{
				"https://x/img.jpg",
				"https://x/img2.jpg",
			},
			PlatformOptions: map[string]any{"mediaType": "story"},
		},
	}))
	hasError(t, res, 0, CodeInstagramStorySingleMediaOnly)
}

func TestValidate_InstagramStorySingleImageIsValid(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{
			AccountID:       "acc_instagram",
			Caption:         "hi",
			MediaURLs:       []string{"https://x/img.jpg"},
			PlatformOptions: map[string]any{"mediaType": "story"},
		},
	}))
	hasNoError(t, res, CodeInstagramStorySingleMediaOnly)
}

// ─── threading ────────────────────────────────────────────────────────

func TestValidate_UnsupportedInReplyTo(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_instagram", Caption: "hi", MediaURLs: []string{"https://x/img.jpg"}, InReplyTo: "ig_post_123"},
	}))
	hasError(t, res, 0, CodeUnsupportedInReplyTo)
}

func TestValidate_InReplyToOKOnTwitter(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_twitter", Caption: "hi", InReplyTo: "tweet_123"},
	}))
	hasNoError(t, res, CodeUnsupportedInReplyTo)
}

// ─── scheduling ───────────────────────────────────────────────────────

func TestValidate_ScheduledTooSoon(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	soon := now.Add(5 * time.Second)
	res := ValidatePlatformPosts(ValidateOptions{
		Capabilities: stubCapabilities(),
		Accounts:     stubAccounts(),
		Posts:        []PlatformPostInput{{AccountID: "acc_twitter", Caption: "hi"}},
		ScheduledAt:  &soon,
		Now:          now,
	})
	if res.Valid {
		t.Fatal("expected invalid")
	}
	found := false
	for _, e := range res.Errors {
		if e.Code == CodeScheduledTooSoon {
			found = true
		}
	}
	if !found {
		t.Errorf("expected scheduled_too_soon, got %#v", res.Errors)
	}
}

func TestValidate_ScheduledTooFar(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	far := now.Add(200 * 24 * time.Hour)
	res := ValidatePlatformPosts(ValidateOptions{
		Capabilities: stubCapabilities(),
		Accounts:     stubAccounts(),
		Posts:        []PlatformPostInput{{AccountID: "acc_twitter", Caption: "hi"}},
		ScheduledAt:  &far,
		Now:          now,
	})
	found := false
	for _, e := range res.Errors {
		if e.Code == CodeScheduledTooFar {
			found = true
		}
	}
	if !found {
		t.Errorf("expected scheduled_too_far, got %#v", res.Errors)
	}
}

// ─── warnings ─────────────────────────────────────────────────────────

func TestValidate_LinkedInTextOnlyWarning(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_linkedin", Caption: "no media here"},
	}))
	if !res.Valid {
		t.Fatalf("expected valid, got %#v", res.Errors)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("expected at least one warning")
	}
	if res.Warnings[0].Code != "low_engagement_likely" {
		t.Errorf("unexpected warning code: %s", res.Warnings[0].Code)
	}
}

// ─── multi-post determinism ───────────────────────────────────────────

func TestValidate_MultiplePostsReportAllErrors(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_twitter", Caption: strings.Repeat("a", 999)}, // exceeds_max_length
		{AccountID: "acc_instagram", Caption: "no media"},             // missing_required (media)
		{AccountID: "acc_someone_else", Caption: "x"},                 // account_not_in_project
	}))
	hasError(t, res, 0, CodeExceedsMaxLength)
	hasError(t, res, 1, CodeMissingRequired)
	hasError(t, res, 2, CodeAccountNotInWorkspace)
}

// ─── thread validation (Sprint 2) ─────────────────────────────────────

func TestValidate_ThreadHappyPath(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_twitter", Caption: "1/", ThreadPosition: 1},
		{AccountID: "acc_twitter", Caption: "2/", ThreadPosition: 2},
		{AccountID: "acc_twitter", Caption: "3/", ThreadPosition: 3},
	}))
	if !res.Valid {
		t.Fatalf("expected valid, got %#v", res.Errors)
	}
}

func TestValidate_ThreadOnUnsupportedPlatform(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_instagram", Caption: "x", MediaURLs: []string{"https://x/y.jpg"}, ThreadPosition: 1},
	}))
	hasError(t, res, 0, CodeThreadsUnsupported)
}

func TestValidate_ThreadMissingPositionOne(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_twitter", Caption: "x", ThreadPosition: 2},
		{AccountID: "acc_twitter", Caption: "y", ThreadPosition: 3},
	}))
	hasError(t, res, 0, CodeThreadPositionsNotContiguous)
}

func TestValidate_ThreadDuplicatePosition(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_twitter", Caption: "x", ThreadPosition: 1},
		{AccountID: "acc_twitter", Caption: "y", ThreadPosition: 1},
	}))
	hasError(t, res, 0, CodeThreadPositionsNotContiguous)
}

func TestValidate_ThreadMixedWithSingle(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_twitter", Caption: "thread 1", ThreadPosition: 1},
		{AccountID: "acc_twitter", Caption: "thread 2", ThreadPosition: 2},
		{AccountID: "acc_twitter", Caption: "standalone"}, // no position
	}))
	hasError(t, res, 0, CodeThreadMixedWithSingle)
}

func TestValidate_ThreadOfOne(t *testing.T) {
	// One entry with position=1, no siblings → silently treated as
	// a single tweet. No error.
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_twitter", Caption: "lonely", ThreadPosition: 1},
	}))
	if !res.Valid {
		t.Fatalf("expected valid, got %#v", res.Errors)
	}
}

func TestValidate_ThreadAcrossDifferentAccounts(t *testing.T) {
	// Two threads, one per account — they don't interfere.
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_twitter", Caption: "tw 1", ThreadPosition: 1},
		{AccountID: "acc_twitter", Caption: "tw 2", ThreadPosition: 2},
		{AccountID: "acc_bluesky", Caption: "bs 1", ThreadPosition: 1},
		{AccountID: "acc_bluesky", Caption: "bs 2", ThreadPosition: 2},
	}))
	// Bluesky doesn't support threads in our stub caps map → reject
	// just the bluesky entries.
	hasError(t, res, 2, CodeThreadsUnsupported)
	hasError(t, res, 3, CodeThreadsUnsupported)
	// Twitter ones should NOT have a thread error.
	for _, e := range res.Errors {
		if e.AccountID == "acc_twitter" && (e.Code == CodeThreadsUnsupported || e.Code == CodeThreadPositionsNotContiguous) {
			t.Errorf("twitter thread should be valid, got %v", e)
		}
	}
}

// ─── media_ids validation (Sprint 2) ──────────────────────────────────

func TestValidate_MediaIDNotInWorkspace(t *testing.T) {
	res := ValidatePlatformPosts(ValidateOptions{
		Capabilities: stubCapabilities(),
		Accounts:     stubAccounts(),
		Media: map[string]ValidateMedia{
			"med_known": {Status: "uploaded"},
		},
		Posts: []PlatformPostInput{
			{AccountID: "acc_twitter", Caption: "x", MediaIDs: []string{"med_unknown"}},
		},
		Now: time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC),
	})
	hasError(t, res, 0, CodeMediaIDNotInWorkspace)
}

func TestValidate_MediaNotUploaded(t *testing.T) {
	res := ValidatePlatformPosts(ValidateOptions{
		Capabilities: stubCapabilities(),
		Accounts:     stubAccounts(),
		Media: map[string]ValidateMedia{
			"med_pending": {Status: "pending"},
		},
		Posts: []PlatformPostInput{
			{AccountID: "acc_twitter", Caption: "x", MediaIDs: []string{"med_pending"}},
		},
		Now: time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC),
	})
	hasError(t, res, 0, CodeMediaNotUploaded)
}

func TestValidate_MediaIDUploaded(t *testing.T) {
	res := ValidatePlatformPosts(ValidateOptions{
		Capabilities: stubCapabilities(),
		Accounts:     stubAccounts(),
		Media: map[string]ValidateMedia{
			"med_ok": {Status: "uploaded"},
		},
		Posts: []PlatformPostInput{
			{AccountID: "acc_twitter", Caption: "x", MediaIDs: []string{"med_ok"}},
		},
		Now: time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC),
	})
	if !res.Valid {
		t.Fatalf("expected valid, got %#v", res.Errors)
	}
}

func TestValidate_MediaIDTooLarge(t *testing.T) {
	res := ValidatePlatformPosts(ValidateOptions{
		Capabilities: stubCapabilities(),
		Accounts:     stubAccounts(),
		Media: map[string]ValidateMedia{
			"med_big": {
				Status:      "uploaded",
				ContentType: "image/jpeg",
				SizeBytes:   5_890_375,
			},
		},
		Posts: []PlatformPostInput{
			{AccountID: "acc_bluesky", Caption: "x", MediaIDs: []string{"med_big"}},
		},
		Now: time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC),
	})
	hasError(t, res, 0, CodeFileTooLarge)
	if len(res.Errors) == 0 {
		t.Fatal("expected file_too_large error")
	}
	if !strings.Contains(res.Errors[0].Message, "5.89 MB") || !strings.Contains(res.Errors[0].Message, "2.00 MB") {
		t.Fatalf("expected human-readable size message, got %q", res.Errors[0].Message)
	}
}

func TestValidate_MediaIDAtLimitAllowed(t *testing.T) {
	res := ValidatePlatformPosts(ValidateOptions{
		Capabilities: stubCapabilities(),
		Accounts:     stubAccounts(),
		Media: map[string]ValidateMedia{
			"med_ok": {
				Status:      "uploaded",
				ContentType: "image/jpeg",
				SizeBytes:   2_000_000,
			},
		},
		Posts: []PlatformPostInput{
			{AccountID: "acc_bluesky", Caption: "x", MediaIDs: []string{"med_ok"}},
		},
		Now: time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC),
	})
	hasNoError(t, res, CodeFileTooLarge)
}

func TestValidate_MediaIDUnsupportedFormat(t *testing.T) {
	res := ValidatePlatformPosts(ValidateOptions{
		Capabilities: stubCapabilities(),
		Accounts:     stubAccounts(),
		Media: map[string]ValidateMedia{
			"med_png": {
				Status:      "uploaded",
				ContentType: "image/png",
				SizeBytes:   512_000,
			},
		},
		Posts: []PlatformPostInput{
			{AccountID: "acc_tiktok", Caption: "x", MediaIDs: []string{"med_png"}},
		},
		Now: time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC),
	})
	hasError(t, res, 0, CodeUnsupportedFormat)
}

// ─── Sprint 4 PR3: first_comment validation ───────────────────────────

func TestValidate_FirstComment_Supported(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_twitter", Caption: "main", FirstComment: "reply"},
	}))
	if !res.Valid {
		t.Fatalf("twitter first_comment should validate, got %#v", res.Errors)
	}
}

func TestValidate_FirstComment_RejectedOnBluesky(t *testing.T) {
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_bluesky", Caption: "hi", FirstComment: "comment"},
	}))
	if res.Valid {
		t.Fatal("bluesky should reject first_comment per Sprint 4 PR3 PRD §W4 D10")
	}
	hasError(t, res, 0, CodeFirstCommentUnsupported)
}

func TestValidate_FirstComment_TooLong(t *testing.T) {
	long := strings.Repeat("x", 281)
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_twitter", Caption: "ok", FirstComment: long},
	}))
	if res.Valid {
		t.Fatal("281-char first_comment on twitter should fail (max 280)")
	}
	hasError(t, res, 0, CodeFirstCommentTooLong)
}

func TestValidate_FirstComment_EmptyAllowed(t *testing.T) {
	// Empty FirstComment string means "don't post one" — must not error.
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{AccountID: "acc_bluesky", Caption: "hi", FirstComment: ""},
	}))
	if !res.Valid {
		t.Fatalf("empty first_comment must be allowed everywhere, got %#v", res.Errors)
	}
}

// ─── Facebook mediaType (Feed vs Reel) ────────────────────────────────

func TestValidate_Facebook_InvalidMediaType(t *testing.T) {
	t.Setenv("FEATURE_FACEBOOK_REELS", "true")
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{
			AccountID:       "acc_facebook",
			Caption:         "hi",
			MediaURLs:       []string{"https://x/y.mp4"},
			PlatformOptions: map[string]any{"mediaType": "clip"},
		},
	}))
	hasError(t, res, 0, CodeInvalidFacebookMediaType)
}

func TestValidate_Facebook_Reel_DisabledByFlag(t *testing.T) {
	// Without FEATURE_FACEBOOK_REELS the validator must reject `reel`
	// with the "not enabled" error code so existing integrators keep
	// seeing a clear explanation.
	t.Setenv("FEATURE_FACEBOOK_REELS", "")
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{
			AccountID:       "acc_facebook",
			Caption:         "hi",
			MediaURLs:       []string{"https://x/y.mp4"},
			PlatformOptions: map[string]any{"mediaType": "reel"},
		},
	}))
	hasError(t, res, 0, CodeFacebookReelsUnsupported)
}

func TestValidate_Facebook_Reel_HappyPath(t *testing.T) {
	t.Setenv("FEATURE_FACEBOOK_REELS", "true")
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{
			AccountID:       "acc_facebook",
			Caption:         "teaser",
			MediaURLs:       []string{"https://x/y.mp4"},
			PlatformOptions: map[string]any{"mediaType": "reel"},
		},
	}))
	hasNoError(t, res, CodeFacebookReelsUnsupported)
	hasNoError(t, res, CodeInvalidFacebookMediaType)
	hasNoError(t, res, CodeMixedMediaUnsupported)
	hasNoError(t, res, CodeMissingRequired)
}

func TestValidate_Facebook_Reel_RequiresVideo(t *testing.T) {
	// A Reel without media must fail — caption-only Reels aren't a
	// supported flow.
	t.Setenv("FEATURE_FACEBOOK_REELS", "true")
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{
			AccountID:       "acc_facebook",
			Caption:         "teaser",
			PlatformOptions: map[string]any{"mediaType": "reel"},
		},
	}))
	hasError(t, res, 0, CodeMissingRequired)
}

func TestValidate_Facebook_Reel_RejectsLink(t *testing.T) {
	t.Setenv("FEATURE_FACEBOOK_REELS", "true")
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{
			AccountID: "acc_facebook",
			Caption:   "teaser",
			MediaURLs: []string{"https://x/y.mp4"},
			PlatformOptions: map[string]any{
				"mediaType": "reel",
				"link":      "https://example.com",
			},
		},
	}))
	hasError(t, res, 0, CodeFacebookLinkWithMedia)
}

func TestValidate_Facebook_Reel_ThumbOffsetBounds(t *testing.T) {
	t.Setenv("FEATURE_FACEBOOK_REELS", "true")
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{
			AccountID: "acc_facebook",
			Caption:   "teaser",
			MediaURLs: []string{"https://x/y.mp4"},
			PlatformOptions: map[string]any{
				"mediaType":       "reel",
				"thumb_offset_ms": 70_000, // past the 60s cap
			},
		},
	}))
	hasError(t, res, 0, CodeInvalidFacebookMediaType)
}

// ─── benchmark for the §5.4 p95 < 50ms requirement ────────────────────

func BenchmarkValidate(b *testing.B) {
	posts := []PlatformPostInput{
		{AccountID: "acc_twitter", Caption: "hi twitter"},
		{AccountID: "acc_instagram", Caption: "hi ig", MediaURLs: []string{"https://x/y.jpg"}},
		{AccountID: "acc_linkedin", Caption: "hi linkedin", MediaURLs: []string{"https://x/y.jpg"}},
		{AccountID: "acc_bluesky", Caption: "hi bsky"},
	}
	opts := validOpts(posts)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidatePlatformPosts(opts)
	}
}
