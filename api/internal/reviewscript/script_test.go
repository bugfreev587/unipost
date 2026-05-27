package reviewscript

import (
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/reviewtemplate"
)

func TestValidateRejectsUnknownActions(t *testing.T) {
	script := Script{
		JobID:        "rvjob_1",
		Platform:     "tiktok",
		AgentVersion: "0.1.0",
		StartURL:     "https://review.example.com/tiktok/posting",
		Steps: []Step{
			{ID: "open", Action: ActionGoto, URL: "https://review.example.com/tiktok/posting"},
			{ID: "bad", Action: Action("eval"), Selector: "body"},
		},
	}

	if err := script.Validate(); err == nil {
		t.Fatal("expected unknown action to fail validation")
	}
}

func TestBuildTikTokScriptUsesClosedActionSet(t *testing.T) {
	script := BuildTikTokScript(BuildTikTokScriptInput{
		JobID:               "rvjob_1",
		AgentVersion:        "0.1.0",
		ReviewDomain:        "review.example.com",
		SessionCookieName:   "__unipost_review_session",
		SessionExpiresAt:    "2026-05-26T21:00:00Z",
		RequireAddressBar:   true,
		BrowserWindowWidth:  1440,
		BrowserWindowHeight: 1000,
	})

	if err := script.Validate(); err != nil {
		t.Fatalf("script should validate: %v", err)
	}
	if script.StartURL != "https://review.example.com/tiktok/posting" {
		t.Fatalf("unexpected start url: %s", script.StartURL)
	}
	if script.AgentVersion != "0.1.0" {
		t.Fatalf("unexpected agent version: %s", script.AgentVersion)
	}
	if script.Recording.CaptureMode != "native-browser-window" || !script.Recording.ShowAddressBar {
		t.Fatalf("unexpected recording settings: %+v", script.Recording)
	}

	seen := map[Action]bool{}
	seenSelector := map[string]bool{}
	for _, step := range script.Steps {
		seen[step.Action] = true
		seenSelector[step.Selector] = true
	}
	for _, action := range []Action{
		ActionGoto,
		ActionClick,
		ActionAssertVisible,
		ActionManualPause,
		ActionEmitMarker,
	} {
		if !seen[action] {
			t.Fatalf("expected script to include action %s", action)
		}
	}
	for _, selector := range []string{
		"[data-review-step='select-video']",
		"[data-review-step='privacy-self-only']",
		"[data-review-step='interaction-controls']",
		"[data-review-step='content-disclosure']",
		"[data-review-step='music-confirmation']",
	} {
		if !seenSelector[selector] {
			t.Fatalf("expected script to include selector %s", selector)
		}
	}
}

func TestValidateRequiresNativeCaptureForAddressBar(t *testing.T) {
	script := Script{
		JobID:        "rvjob_1",
		Platform:     "tiktok",
		AgentVersion: "0.1.0",
		StartURL:     "https://review.example.com/tiktok/posting",
		Recording: RecordingSpec{
			CaptureMode:    "playwright-page-video",
			ShowAddressBar: true,
		},
		Steps: []Step{{ID: "open", Action: ActionGoto, URL: "https://review.example.com/tiktok/posting"}},
	}

	if err := script.Validate(); err == nil {
		t.Fatal("expected address-bar script to require native capture")
	}
}

func TestBuildTikTokScriptFromPlanIncludesPostingSegments(t *testing.T) {
	plan, err := reviewtemplate.BuildTikTokDemoPlan(reviewtemplate.TikTokDemoPlanInput{Scopes: []string{"user.info.basic", "video.upload", "video.publish"}})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	script := BuildTikTokScriptFromPlan(BuildTikTokScriptInput{
		JobID:               "rvjob_1",
		AgentVersion:        "0.1.0",
		ReviewDomain:        "review.example.com",
		SessionCookieName:   "__unipost_review_session",
		SessionExpiresAt:    "2026-05-26T21:00:00Z",
		RequireAddressBar:   true,
		BrowserWindowWidth:  1920,
		BrowserWindowHeight: 1080,
		Plan:                &plan,
	})

	if err := script.Validate(); err != nil {
		t.Fatalf("script should validate: %v", err)
	}
	if len(script.Segments) != 3 {
		t.Fatalf("expected 3 script segments, got %+v", script.Segments)
	}
	assertStep(t, script, "segment_posting_part_1")
	assertStep(t, script, "assert_creator_info")
	assertStep(t, script, "select_video")
	assertStep(t, script, "assert_music_confirmation")
	assertStep(t, script, "publish")
	assertStep(t, script, "assert_result")
	if script.Recording.WindowWidth != 1920 || script.Recording.WindowHeight != 1080 {
		t.Fatalf("expected 1080p window, got %+v", script.Recording)
	}
	if script.Recording.MaxArtifactBytes != 50000000 || !script.Recording.SplitAutomatically {
		t.Fatalf("expected TikTok artifact split constraints, got %+v", script.Recording)
	}
	if script.Segments[0].Key != "posting_part_1" || !containsString(script.Segments[0].Scopes, "video.upload") {
		t.Fatalf("missing segment metadata: %+v", script.Segments)
	}
}

func TestBuildTikTokScriptFromPlanIncludesAnalyticsWithoutVideoList(t *testing.T) {
	plan, err := reviewtemplate.BuildTikTokDemoPlan(reviewtemplate.TikTokDemoPlanInput{Scopes: []string{"user.info.profile", "user.info.stats"}})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	script := BuildTikTokScriptFromPlan(BuildTikTokScriptInput{
		JobID:             "rvjob_analytics",
		AgentVersion:      "0.1.0",
		ReviewDomain:      "review.example.com",
		SessionCookieName: "__unipost_review_session",
		SessionExpiresAt:  "2026-05-26T21:00:00Z",
		RequireAddressBar: true,
		Plan:              &plan,
	})

	if err := script.Validate(); err != nil {
		t.Fatalf("script should validate: %v", err)
	}
	if script.StartURL != "https://review.example.com/tiktok/analytics" {
		t.Fatalf("unexpected analytics start url: %s", script.StartURL)
	}
	assertStep(t, script, "open_tiktok_analytics")
	assertStep(t, script, "assert_profile_card")
	assertStep(t, script, "assert_account_stats")
	assertNoStep(t, script, "assert_video_list")
}

func TestBuildTikTokScriptFromPlanIncludesVideoListOnlyWhenRequested(t *testing.T) {
	plan, err := reviewtemplate.BuildTikTokDemoPlan(reviewtemplate.TikTokDemoPlanInput{Scopes: []string{"user.info.profile", "user.info.stats", "video.list"}})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	script := BuildTikTokScriptFromPlan(BuildTikTokScriptInput{
		JobID:             "rvjob_video_list",
		AgentVersion:      "0.1.0",
		ReviewDomain:      "review.example.com",
		SessionCookieName: "__unipost_review_session",
		SessionExpiresAt:  "2026-05-26T21:00:00Z",
		RequireAddressBar: true,
		Plan:              &plan,
	})

	assertStep(t, script, "assert_video_list")
}

func assertStep(t *testing.T, script Script, id string) {
	t.Helper()
	for _, step := range script.Steps {
		if step.ID == id {
			return
		}
	}
	t.Fatalf("step %q not found in %+v", id, script.Steps)
}

func assertNoStep(t *testing.T, script Script, id string) {
	t.Helper()
	for _, step := range script.Steps {
		if step.ID == id {
			t.Fatalf("step %q should not be present: %+v", id, script.Steps)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
