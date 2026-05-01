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
				Videos: VideoCapability{
					MaxCount: 1,
					// Mirror production placement specs so the
					// placement-validation tests in this file run
					// against the same intervals real users hit.
					Placements: map[string]VideoPlacementSpec{
						"feed": {
							DisplayName:    "Facebook Feed",
							MinAspectRatio: 1.0,
							MinDurationMS:  1_000,
							MaxDurationMS:  240 * 60 * 1000,
							ReclassifyHint: "Facebook silently reclassifies vertical videos as Reels.",
						},
						"reel": {
							DisplayName:    "Facebook Reel",
							MinAspectRatio: 0.50,
							MaxAspectRatio: 0.62,
							MinWidth:       540,
							MinHeight:      960,
							MinDurationMS:  3_000,
							MaxDurationMS:  90_000,
							ReclassifyHint: "Facebook Reels require 9:16, 540×960+, 3-90s.",
						},
					},
				},
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

// TestValidate_PlanPlatformNotAllowed — the workspace's plan disallows
// the resolved platform (free plan + twitter, per migration 057). The
// validator should emit CodePlanPlatformNotAllowed and short-circuit
// further per-post checks for that entry. Caption length etc. are
// intentionally NOT reported alongside — once the plan blocks the
// platform, the rest of the per-post details are noise.
func TestValidate_PlanPlatformNotAllowed(t *testing.T) {
	opts := validOpts([]PlatformPostInput{
		{AccountID: "acc_twitter", Caption: "hi"},
	})
	opts.DisallowedPlatforms = map[string]bool{"twitter": true}
	res := ValidatePlatformPosts(opts)
	hasError(t, res, 0, CodePlanPlatformNotAllowed)
	hasNoError(t, res, CodeExceedsMaxLength)
}

// TestValidate_PlanPlatformAllowed — when the resolved platform is
// NOT in the disallowed set, the validator runs every other check
// normally. Regression guard: a non-empty DisallowedPlatforms map
// should not interfere with platforms that aren't in it.
func TestValidate_PlanPlatformAllowed(t *testing.T) {
	opts := validOpts([]PlatformPostInput{
		{AccountID: "acc_linkedin", Caption: "hi linkedin"},
	})
	opts.DisallowedPlatforms = map[string]bool{"twitter": true}
	res := ValidatePlatformPosts(opts)
	if !res.Valid {
		t.Fatalf("expected valid, got %#v", res.Errors)
	}
}

// TestValidate_PlanPlatformNotAllowed_NilMap — the common case where
// the caller passes no plan restriction. Must behave identically to
// the legacy path (every platform allowed).
func TestValidate_PlanPlatformNotAllowed_NilMap(t *testing.T) {
	opts := validOpts([]PlatformPostInput{
		{AccountID: "acc_twitter", Caption: "hi"},
	})
	opts.DisallowedPlatforms = nil
	res := ValidatePlatformPosts(opts)
	if !res.Valid {
		t.Fatalf("expected valid, got %#v", res.Errors)
	}
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

// ─── Sprint 5: Facebook placement-spec checks ────────────────────────
//
// These tests cover the validator's pre-flight on the FB feed/reel
// placement specs added to capabilities.go. The key invariant under
// test is "no surprise reclassification" — Meta will silently route
// vertical-9:16 video posted to /{page_id}/videos into the Reels
// pipeline, and the validator must reject that combination at submit
// time so the publish call never reaches Meta in a state Meta will
// mutate.

// hasWarning asserts at least one warning has the given code.
func hasWarning(t *testing.T, res ValidationResult, code string) {
	t.Helper()
	for _, w := range res.Warnings {
		if w.Code == code {
			return
		}
	}
	t.Errorf("expected warning code %q, got warnings: %#v", code, res.Warnings)
}

// fbPlacementOpts wires one media row into a single-post request
// against acc_facebook. Tests use this to drive the placement check
// without re-stating the validOpts boilerplate every time.
func fbPlacementOpts(mediaType string, m ValidateMedia) ValidateOptions {
	post := PlatformPostInput{
		AccountID: "acc_facebook",
		Caption:   "preflight",
		MediaIDs:  []string{"med1"},
	}
	if mediaType != "" {
		post.PlatformOptions = map[string]any{"mediaType": mediaType}
	}
	o := validOpts([]PlatformPostInput{post})
	o.Media = map[string]ValidateMedia{
		"med1": m,
	}
	return o
}

func TestValidate_Facebook_Feed_VerticalReclassified(t *testing.T) {
	// 1080×1920 — the user's exact pain case: vertical 9:16 video
	// posted to feed gets silently reclassified by Meta as a Reel.
	// Must reject at submit time. The default mediaType (omitted)
	// is "feed", so this test also exercises the implicit-feed path.
	res := ValidatePlatformPosts(fbPlacementOpts("", ValidateMedia{
		Status: "uploaded", ContentType: "video/mp4",
		Width: 1080, Height: 1920, DurationMS: 30_000,
	}))
	hasError(t, res, 0, CodeFacebookVideoAspectReclassified)
}

func TestValidate_Facebook_Feed_VerticalReclassified_ExplicitMediaType(t *testing.T) {
	// Same case, but with explicit mediaType=feed — verifies the
	// rule applies whether the user defaulted in or asked for feed.
	res := ValidatePlatformPosts(fbPlacementOpts("feed", ValidateMedia{
		Status: "uploaded", ContentType: "video/mp4",
		Width: 720, Height: 1280, DurationMS: 15_000,
	}))
	hasError(t, res, 0, CodeFacebookVideoAspectReclassified)
}

func TestValidate_Facebook_Feed_SquareAccepted(t *testing.T) {
	// 1:1 square sits at the MinAspectRatio=1.0 boundary. This is
	// the canonical "stays in feed" case — Meta won't reclassify.
	res := ValidatePlatformPosts(fbPlacementOpts("feed", ValidateMedia{
		Status: "uploaded", ContentType: "video/mp4",
		Width: 1080, Height: 1080, DurationMS: 30_000,
	}))
	hasNoError(t, res, CodeFacebookVideoAspectReclassified)
	hasNoError(t, res, CodeFacebookVideoDimensionsTooSmall)
}

func TestValidate_Facebook_Feed_LandscapeAccepted(t *testing.T) {
	// 16:9 — definitely stays in feed.
	res := ValidatePlatformPosts(fbPlacementOpts("feed", ValidateMedia{
		Status: "uploaded", ContentType: "video/mp4",
		Width: 1920, Height: 1080, DurationMS: 30_000,
	}))
	hasNoError(t, res, CodeFacebookVideoAspectReclassified)
}

func TestValidate_Facebook_Reel_VerticalAccepted(t *testing.T) {
	// 9:16 vertical, 1080×1920, 30s — the canonical happy-path Reel.
	t.Setenv("FEATURE_FACEBOOK_REELS", "true")
	res := ValidatePlatformPosts(fbPlacementOpts("reel", ValidateMedia{
		Status: "uploaded", ContentType: "video/mp4",
		Width: 1080, Height: 1920, DurationMS: 30_000,
	}))
	hasNoError(t, res, CodeFacebookVideoAspectReclassified)
	hasNoError(t, res, CodeFacebookVideoAspectUnsupported)
	hasNoError(t, res, CodeFacebookVideoDimensionsTooSmall)
	hasNoError(t, res, CodeFacebookVideoDurationOutOfRange)
}

func TestValidate_Facebook_Reel_LandscapeRejected(t *testing.T) {
	// 16:9 landscape posted as a Reel — aspect way above 0.62 cap.
	// Should fail with the unsupported-aspect code (distinct from
	// reclassified, since on Reel side Meta just rejects rather
	// than reclassifying).
	t.Setenv("FEATURE_FACEBOOK_REELS", "true")
	res := ValidatePlatformPosts(fbPlacementOpts("reel", ValidateMedia{
		Status: "uploaded", ContentType: "video/mp4",
		Width: 1920, Height: 1080, DurationMS: 30_000,
	}))
	hasError(t, res, 0, CodeFacebookVideoAspectUnsupported)
}

func TestValidate_Facebook_Reel_DimensionsTooSmall(t *testing.T) {
	// 360×640 — vertical aspect is fine, but below Reels' 540×960
	// minimum. Should fire CodeFacebookVideoDimensionsTooSmall on
	// both width AND height (separate errors so the user sees
	// both gaps in one shot).
	t.Setenv("FEATURE_FACEBOOK_REELS", "true")
	res := ValidatePlatformPosts(fbPlacementOpts("reel", ValidateMedia{
		Status: "uploaded", ContentType: "video/mp4",
		Width: 360, Height: 640, DurationMS: 10_000,
	}))
	hasError(t, res, 0, CodeFacebookVideoDimensionsTooSmall)
	// Aspect at 0.5625 is fine — confirm we didn't accidentally
	// flag aspect alongside the dimension error.
	hasNoError(t, res, CodeFacebookVideoAspectUnsupported)
	hasNoError(t, res, CodeFacebookVideoAspectReclassified)
}

func TestValidate_Facebook_Reel_DurationTooLong(t *testing.T) {
	// 120s exceeds the Meta 90s Reel cap.
	t.Setenv("FEATURE_FACEBOOK_REELS", "true")
	res := ValidatePlatformPosts(fbPlacementOpts("reel", ValidateMedia{
		Status: "uploaded", ContentType: "video/mp4",
		Width: 1080, Height: 1920, DurationMS: 120_000,
	}))
	hasError(t, res, 0, CodeFacebookVideoDurationOutOfRange)
}

func TestValidate_Facebook_Reel_DurationTooShort(t *testing.T) {
	// 1.5s is below Meta's 3s Reel floor — Meta rejects at upload.
	t.Setenv("FEATURE_FACEBOOK_REELS", "true")
	res := ValidatePlatformPosts(fbPlacementOpts("reel", ValidateMedia{
		Status: "uploaded", ContentType: "video/mp4",
		Width: 1080, Height: 1920, DurationMS: 1_500,
	}))
	hasError(t, res, 0, CodeFacebookVideoDurationOutOfRange)
}

func TestValidate_Facebook_Reel_DisabledFlagSuppressesPlacement(t *testing.T) {
	// When FEATURE_FACEBOOK_REELS is off, the existing
	// CodeFacebookReelsUnsupported error already forces the user
	// to switch placements. Stacking dimension errors on top would
	// be noisy and confusing — assert we DON'T pile on.
	t.Setenv("FEATURE_FACEBOOK_REELS", "")
	res := ValidatePlatformPosts(fbPlacementOpts("reel", ValidateMedia{
		Status: "uploaded", ContentType: "video/mp4",
		Width: 360, Height: 640, DurationMS: 100_000,
	}))
	hasError(t, res, 0, CodeFacebookReelsUnsupported)
	hasNoError(t, res, CodeFacebookVideoDimensionsTooSmall)
	hasNoError(t, res, CodeFacebookVideoDurationOutOfRange)
	hasNoError(t, res, CodeFacebookVideoAspectUnsupported)
}

func TestValidate_Facebook_Feed_RawURL_WarnsNotErrors(t *testing.T) {
	// Raw media_urls submission: no media library row, no probe
	// data. The validator can't pre-flight the placement, so it
	// must DOWNGRADE to a warning rather than block — the user
	// might know their video is fine and just not be using the
	// media library upload flow.
	res := ValidatePlatformPosts(validOpts([]PlatformPostInput{
		{
			AccountID: "acc_facebook",
			Caption:   "raw url submit",
			MediaURLs: []string{"https://example.com/video.mp4"},
		},
	}))
	hasNoError(t, res, CodeFacebookVideoAspectReclassified)
	hasNoError(t, res, CodeFacebookVideoMetadataUnknown) // no error
	hasWarning(t, res, CodeFacebookVideoMetadataUnknown)
}

func TestValidate_Facebook_Feed_MissingProbeMetadata_Warns(t *testing.T) {
	// Media library row exists but probing didn't extract dims
	// (non-mp4 container, moov-at-end > 16 MB, etc.). Must warn,
	// not error — same reasoning as the raw-URL case.
	res := ValidatePlatformPosts(fbPlacementOpts("feed", ValidateMedia{
		Status: "uploaded", ContentType: "video/webm",
		// Width/Height/DurationMS all zero — simulates probe miss.
	}))
	hasNoError(t, res, CodeFacebookVideoAspectReclassified)
	hasWarning(t, res, CodeFacebookVideoMetadataUnknown)
}

func TestValidate_Facebook_Feed_AspectErrorMessageMentionsHint(t *testing.T) {
	// The error MUST surface the WHY ("Meta will reclassify"), not
	// just the numeric mismatch — that's the whole point of the
	// ReclassifyHint field on VideoPlacementSpec.
	res := ValidatePlatformPosts(fbPlacementOpts("feed", ValidateMedia{
		Status: "uploaded", ContentType: "video/mp4",
		Width: 720, Height: 1280, DurationMS: 30_000,
	}))
	for _, e := range res.Errors {
		if e.Code == CodeFacebookVideoAspectReclassified {
			if !strings.Contains(strings.ToLower(e.Message), "reclassif") {
				t.Errorf("aspect error message should reference reclassification, got: %q", e.Message)
			}
			return
		}
	}
	t.Fatalf("expected CodeFacebookVideoAspectReclassified error, got %+v", res.Errors)
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
