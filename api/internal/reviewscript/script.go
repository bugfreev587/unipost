package reviewscript

import (
	"fmt"
	"net/url"
	"strings"
)

type Action string

const (
	ActionGoto               Action = "goto"
	ActionClick              Action = "click"
	ActionFill               Action = "fill"
	ActionAssertVisible      Action = "assert_visible"
	ActionAssertURLContains  Action = "assert_url_contains"
	ActionManualPause        Action = "manual_pause"
	ActionWaitForNavigation  Action = "wait_for_navigation"
	ActionWaitForNetworkIdle Action = "wait_for_network_idle"
	ActionScreenshot         Action = "screenshot"
	ActionEmitMarker         Action = "emit_marker"
)

var allowedActions = map[Action]bool{
	ActionGoto:               true,
	ActionClick:              true,
	ActionFill:               true,
	ActionAssertVisible:      true,
	ActionAssertURLContains:  true,
	ActionManualPause:        true,
	ActionWaitForNavigation:  true,
	ActionWaitForNetworkIdle: true,
	ActionScreenshot:         true,
	ActionEmitMarker:         true,
}

type Script struct {
	JobID         string        `json:"job_id"`
	Platform      string        `json:"platform"`
	AgentVersion  string        `json:"agent_version"`
	StartURL      string        `json:"start_url"`
	ReviewSession SessionSpec   `json:"review_session"`
	Recording     RecordingSpec `json:"recording"`
	Steps         []Step        `json:"steps"`
}

type SessionSpec struct {
	Delivery  string `json:"delivery"`
	Cookie    string `json:"cookie_name"`
	ExpiresAt string `json:"expires_at"`
}

type RecordingSpec struct {
	WindowWidth    int    `json:"window_width"`
	WindowHeight   int    `json:"window_height"`
	CaptureMode    string `json:"capture_mode"`
	ShowAddressBar bool   `json:"show_address_bar"`
}

type Step struct {
	ID                    string `json:"id"`
	Action                Action `json:"action"`
	URL                   string `json:"url,omitempty"`
	Selector              string `json:"selector,omitempty"`
	Value                 string `json:"value,omitempty"`
	Text                  string `json:"text,omitempty"`
	ResumeWhenURLContains string `json:"resume_when_url_contains,omitempty"`
	Overlay               string `json:"overlay,omitempty"`
	Marker                string `json:"marker,omitempty"`
}

type BuildTikTokScriptInput struct {
	JobID               string
	AgentVersion        string
	ReviewDomain        string
	SessionCookieName   string
	SessionExpiresAt    string
	RequireAddressBar   bool
	BrowserWindowWidth  int
	BrowserWindowHeight int
}

func (s Script) Validate() error {
	if strings.TrimSpace(s.JobID) == "" {
		return fmt.Errorf("job_id is required")
	}
	if s.Platform != "tiktok" {
		return fmt.Errorf("unsupported platform %q", s.Platform)
	}
	if _, err := url.ParseRequestURI(s.StartURL); err != nil {
		return fmt.Errorf("start_url is invalid: %w", err)
	}
	if s.Recording.CaptureMode != "" && s.Recording.CaptureMode != "native-browser-window" && s.Recording.CaptureMode != "playwright-page-video" {
		return fmt.Errorf("recording.capture_mode %q is not allowed", s.Recording.CaptureMode)
	}
	if s.Recording.ShowAddressBar && s.Recording.CaptureMode == "playwright-page-video" {
		return fmt.Errorf("recording.show_address_bar requires native-browser-window capture")
	}
	if len(s.Steps) == 0 {
		return fmt.Errorf("steps are required")
	}
	for i, step := range s.Steps {
		if strings.TrimSpace(step.ID) == "" {
			return fmt.Errorf("steps[%d].id is required", i)
		}
		if !allowedActions[step.Action] {
			return fmt.Errorf("steps[%d].action %q is not allowed", i, step.Action)
		}
		if step.Action == ActionGoto && strings.TrimSpace(step.URL) == "" {
			return fmt.Errorf("steps[%d].url is required for goto", i)
		}
		if requiresSelector(step.Action) && strings.TrimSpace(step.Selector) == "" {
			return fmt.Errorf("steps[%d].selector is required for %s", i, step.Action)
		}
	}
	return nil
}

func BuildTikTokScript(input BuildTikTokScriptInput) Script {
	reviewDomain := strings.TrimSpace(input.ReviewDomain)
	startURL := "https://" + strings.TrimPrefix(reviewDomain, "https://") + "/tiktok/posting"
	cookieName := strings.TrimSpace(input.SessionCookieName)
	if cookieName == "" {
		cookieName = "__unipost_review_session"
	}
	width := input.BrowserWindowWidth
	if width == 0 {
		width = 1440
	}
	height := input.BrowserWindowHeight
	if height == 0 {
		height = 1000
	}

	return Script{
		JobID:        input.JobID,
		Platform:     "tiktok",
		AgentVersion: input.AgentVersion,
		StartURL:     startURL,
		ReviewSession: SessionSpec{
			Delivery:  "cookie",
			Cookie:    cookieName,
			ExpiresAt: input.SessionExpiresAt,
		},
		Recording: RecordingSpec{
			WindowWidth:    width,
			WindowHeight:   height,
			CaptureMode:    captureMode(input.RequireAddressBar),
			ShowAddressBar: input.RequireAddressBar,
		},
		Steps: []Step{
			{
				ID:     "marker_start",
				Action: ActionEmitMarker,
				Marker: "Open customer review domain",
			},
			{
				ID:     "open_review_app",
				Action: ActionGoto,
				URL:    startURL,
				Marker: "Open customer review domain",
			},
			{
				ID:       "connect_tiktok",
				Action:   ActionClick,
				Selector: "[data-review-step='connect-tiktok']",
				Marker:   "Start TikTok OAuth",
			},
			{
				ID:                    "wait_for_oauth",
				Action:                ActionManualPause,
				ResumeWhenURLContains: "/tiktok/posting",
				Overlay:               "Log in to TikTok and approve access. UniPost cannot see or store your password or verification code.",
				Marker:                "Customer completes TikTok login and consent",
			},
			{
				ID:       "assert_creator_info",
				Action:   ActionAssertVisible,
				Selector: "[data-review-step='creator-info']",
				Marker:   "Show TikTok creator_info",
			},
			{
				ID:       "select_video",
				Action:   ActionClick,
				Selector: "[data-review-step='select-video']",
				Marker:   "Select the TailTales review video",
			},
			{
				ID:       "select_self_only",
				Action:   ActionClick,
				Selector: "[data-review-step='privacy-self-only']",
				Marker:   "Choose TikTok visibility",
			},
			{
				ID:       "assert_interactions",
				Action:   ActionAssertVisible,
				Selector: "[data-review-step='interaction-controls']",
				Marker:   "Show interaction controls",
			},
			{
				ID:       "assert_disclosure",
				Action:   ActionAssertVisible,
				Selector: "[data-review-step='content-disclosure']",
				Marker:   "Show content disclosure controls",
			},
			{
				ID:       "assert_music_confirmation",
				Action:   ActionAssertVisible,
				Selector: "[data-review-step='music-confirmation']",
				Marker:   "Show Music Usage Confirmation",
			},
			{
				ID:       "publish",
				Action:   ActionClick,
				Selector: "[data-review-step='publish-tiktok']",
				Marker:   "Publish test video",
			},
			{
				ID:       "assert_result",
				Action:   ActionAssertVisible,
				Selector: "[data-review-step='publish-result']",
				Marker:   "Show publish result",
			},
		},
	}
}

func captureMode(requireAddressBar bool) string {
	if requireAddressBar {
		return "native-browser-window"
	}
	return "playwright-page-video"
}

func requiresSelector(action Action) bool {
	switch action {
	case ActionClick, ActionFill, ActionAssertVisible:
		return true
	default:
		return false
	}
}
