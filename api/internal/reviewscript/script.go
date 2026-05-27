package reviewscript

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/xiaoboyu/unipost-api/internal/reviewtemplate"
)

type Action string

const (
	ActionGoto               Action = "goto"
	ActionClick              Action = "click"
	ActionFill               Action = "fill"
	ActionAssertVisible      Action = "assert_visible"
	ActionAssertURLContains  Action = "assert_url_contains"
	ActionOpenLink           Action = "open_link"
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
	ActionOpenLink:           true,
	ActionManualPause:        true,
	ActionWaitForNavigation:  true,
	ActionWaitForNetworkIdle: true,
	ActionScreenshot:         true,
	ActionEmitMarker:         true,
}

type Script struct {
	JobID           string        `json:"job_id"`
	Platform        string        `json:"platform"`
	AgentVersion    string        `json:"agent_version"`
	StartURL        string        `json:"start_url"`
	RequestedScopes []string      `json:"requested_scopes,omitempty"`
	ReviewSession   SessionSpec   `json:"review_session"`
	Recording       RecordingSpec `json:"recording"`
	Segments        []SegmentSpec `json:"segments,omitempty"`
	Steps           []Step        `json:"steps"`
}

type SegmentSpec struct {
	Key                  string   `json:"key"`
	Title                string   `json:"title"`
	Filename             string   `json:"filename"`
	Scopes               []string `json:"scopes"`
	EstimatedDurationSec int      `json:"estimated_duration_sec"`
}

type SessionSpec struct {
	Delivery  string `json:"delivery"`
	Cookie    string `json:"cookie_name"`
	ExpiresAt string `json:"expires_at"`
}

type RecordingSpec struct {
	WindowWidth        int    `json:"window_width"`
	WindowHeight       int    `json:"window_height"`
	CaptureMode        string `json:"capture_mode"`
	ShowAddressBar     bool   `json:"show_address_bar"`
	MaxArtifactBytes   int64  `json:"max_artifact_bytes,omitempty"`
	SplitAutomatically bool   `json:"split_automatically,omitempty"`
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
	Plan                *reviewtemplate.TikTokDemoPlan
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
	return BuildTikTokScriptFromPlan(input)
}

func BuildTikTokScriptFromPlan(input BuildTikTokScriptInput) Script {
	reviewDomain := strings.TrimSpace(input.ReviewDomain)
	useCase := "content_posting"
	if input.Plan != nil && strings.TrimSpace(input.Plan.UseCase) != "" {
		useCase = input.Plan.UseCase
	}
	startURL := reviewStartURL(reviewDomain, useCase)
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
		JobID:           input.JobID,
		Platform:        "tiktok",
		AgentVersion:    input.AgentVersion,
		StartURL:        startURL,
		RequestedScopes: requestedScopes(input.Plan),
		ReviewSession: SessionSpec{
			Delivery:  "cookie",
			Cookie:    cookieName,
			ExpiresAt: input.SessionExpiresAt,
		},
		Recording: RecordingSpec{
			WindowWidth:        width,
			WindowHeight:       height,
			CaptureMode:        captureMode(input.RequireAddressBar),
			ShowAddressBar:     input.RequireAddressBar,
			MaxArtifactBytes:   maxArtifactBytes(input.Plan),
			SplitAutomatically: splitAutomatically(input.Plan),
		},
		Segments: segmentSpecs(input.Plan),
		Steps:    buildTikTokSteps(startURL, useCase, input.Plan),
	}
}

func maxArtifactBytes(plan *reviewtemplate.TikTokDemoPlan) int64 {
	if plan == nil || plan.Recording.MaxFileSizeMB <= 0 {
		return 0
	}
	return int64(plan.Recording.MaxFileSizeMB) * 1000 * 1000
}

func splitAutomatically(plan *reviewtemplate.TikTokDemoPlan) bool {
	if plan == nil {
		return false
	}
	return plan.Recording.SplitAutomatically
}

func requestedScopes(plan *reviewtemplate.TikTokDemoPlan) []string {
	if plan == nil {
		return nil
	}
	return append([]string(nil), plan.RequestedScopes...)
}

func reviewStartURL(reviewDomain, useCase string) string {
	host := strings.TrimPrefix(reviewDomain, "https://")
	if useCase == "analytics" {
		return "https://" + host + "/tiktok/analytics"
	}
	return "https://" + host + "/tiktok/posting"
}

func segmentSpecs(plan *reviewtemplate.TikTokDemoPlan) []SegmentSpec {
	if plan == nil {
		return nil
	}
	out := make([]SegmentSpec, 0, len(plan.Segments))
	for _, segment := range plan.Segments {
		out = append(out, SegmentSpec{
			Key:                  segment.Key,
			Title:                segment.Title,
			Filename:             segment.Filename,
			Scopes:               append([]string(nil), segment.Scopes...),
			EstimatedDurationSec: segment.EstimatedDurationSec,
		})
	}
	return out
}

func buildTikTokSteps(startURL string, useCase string, plan *reviewtemplate.TikTokDemoPlan) []Step {
	segments := []string{"posting_part_1", "posting_part_2", "posting_part_3"}
	videoListSegments := map[string]bool{}
	if plan != nil && len(plan.Segments) > 0 {
		segments = make([]string, 0, len(plan.Segments))
		for _, segment := range plan.Segments {
			segments = append(segments, segment.Key)
			for _, step := range segment.Steps {
				if step.Key == "video_list" {
					videoListSegments[segment.Key] = true
				}
			}
		}
	}
	steps := []Step{
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
			ResumeWhenURLContains: reviewResumePath(useCase),
			Overlay:               "Log in to TikTok and approve access. UniPost cannot see or store your password or verification code.",
			Marker:                "Customer completes TikTok login and consent",
		},
	}
	for _, segment := range segments {
		steps = append(steps, stepsForSegment(segment, startURL, videoListSegments[segment])...)
	}
	return steps
}

func reviewResumePath(useCase string) string {
	if useCase == "analytics" {
		return "/tiktok/analytics"
	}
	return "/tiktok/posting"
}

func stepsForSegment(segment string, startURL string, includeVideoList bool) []Step {
	switch segment {
	case "posting_part_1":
		return []Step{
			segmentMarker("posting_part_1", "Content Posting Part 1 - Creator Info, Upload, and Content Details"),
			{ID: "assert_creator_info", Action: ActionAssertVisible, Selector: "[data-review-step='creator-info']", Marker: "1. Retrieve Creator Info"},
			{ID: "select_video", Action: ActionClick, Selector: "[data-review-step='select-video']", Marker: "2. User Uploads Video And Enters Post Details"},
			{ID: "assert_video_upload_ready", Action: ActionAssertVisible, Selector: "[data-review-step='video-upload-ready']", Marker: "Confirm uploaded video is ready for TikTok video.upload"},
			{ID: "select_self_only", Action: ActionClick, Selector: "[data-review-step='privacy-self-only']", Marker: "Choose TikTok visibility"},
			{ID: "assert_disclosure", Action: ActionAssertVisible, Selector: "[data-review-step='content-disclosure']", Marker: "3a. Content Disclosure Settings"},
		}
	case "posting_part_2":
		return []Step{
			segmentMarker("posting_part_2", "Content Posting Part 2 - Privacy Management and Compliance"),
			{ID: "assert_privacy_management", Action: ActionAssertVisible, Selector: "[data-review-step='privacy-selector']", Marker: "3b. Privacy Management"},
			{ID: "assert_interactions", Action: ActionAssertVisible, Selector: "[data-review-step='interaction-controls']", Marker: "Show interaction controls"},
			{ID: "assert_music_confirmation", Action: ActionAssertVisible, Selector: "[data-review-step='music-confirmation']", Marker: "4. Compliance Requirements"},
			{ID: "open_music_usage_confirmation", Action: ActionOpenLink, Selector: "[data-review-step='music-usage-confirmation-link']", Value: "music-usage-confirmation", Marker: "Open TikTok Music Usage Confirmation"},
			{ID: "assert_branded_policy", Action: ActionAssertVisible, Selector: "[data-review-step='branded-content-policy']", Marker: "Show Branded Content Policy"},
			{ID: "open_branded_content_policy", Action: ActionOpenLink, Selector: "[data-review-step='branded-content-policy-link']", Value: "bc-policy", Marker: "Open TikTok Branded Content Policy"},
		}
	case "posting_part_3":
		return []Step{
			segmentMarker("posting_part_3", "Content Posting Part 3 - Preview, Publish, and Verification"),
			{ID: "assert_preview", Action: ActionAssertVisible, Selector: "[data-review-step='post-preview']", Marker: "5. Preview And Publish"},
			{ID: "publish", Action: ActionClick, Selector: "[data-review-step='publish-tiktok']", Marker: "Publish test video"},
			{ID: "assert_result", Action: ActionAssertVisible, Selector: "[data-review-step='publish-result']", Marker: "Show publish result"},
		}
	case "analytics_part_1":
		return []Step{
			segmentMarker("analytics_part_1", "TikTok Analytics Part 1 - Login, OAuth, and Navigation"),
			{ID: "open_tiktok_analytics", Action: ActionGoto, URL: analyticsStartURL(startURL), Marker: "Open TikTok Analytics"},
			{ID: "assert_analytics_loading", Action: ActionAssertVisible, Selector: "[data-review-step='analytics-loading']", Marker: "Navigate to TikTok Analytics"},
		}
	case "analytics_part_2":
		steps := []Step{
			segmentMarker("analytics_part_2", "TikTok Analytics Part 2 - Profile and Stats Evidence"),
			{ID: "assert_profile_card", Action: ActionAssertVisible, Selector: "[data-review-step='analytics-profile-card']", Marker: "1. user.info.profile"},
			{ID: "assert_account_stats", Action: ActionAssertVisible, Selector: "[data-review-step='analytics-account-stats']", Marker: "2. user.info.stats"},
		}
		if includeVideoList {
			steps = append(steps, Step{ID: "assert_video_list", Action: ActionAssertVisible, Selector: "[data-review-step='analytics-video-list']", Marker: "3. video.list"})
		}
		return steps
	default:
		return []Step{segmentMarker(segment, segment)}
	}
}

func analyticsStartURL(startURL string) string {
	if strings.Contains(startURL, "/tiktok/analytics") {
		return startURL
	}
	return strings.Replace(startURL, "/tiktok/posting", "/tiktok/analytics", 1)
}

func segmentMarker(key, title string) Step {
	return Step{
		ID:     "segment_" + key,
		Action: ActionEmitMarker,
		Marker: title,
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
	case ActionClick, ActionFill, ActionAssertVisible, ActionOpenLink:
		return true
	default:
		return false
	}
}
