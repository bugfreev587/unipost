package reviewscript

import "testing"

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
	for _, step := range script.Steps {
		seen[step.Action] = true
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
