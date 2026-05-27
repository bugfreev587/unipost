package reviewtemplate

import (
	"fmt"
	"sort"
	"strings"
)

const TikTokTemplateVersion = "tiktok-review-2026-05-27"

var tiktokScopeOrder = []string{
	"user.info.basic",
	"user.info.profile",
	"user.info.stats",
	"video.list",
	"video.publish",
	"video.upload",
}

var tiktokScopeTemplates = map[string]TikTokScopeTemplate{
	"user.info.basic": {
		Scope:            "user.info.basic",
		Label:            "Basic user info",
		UseCase:          "content_posting",
		PrimarySurface:   "connection_flow,create_post_drawer",
		RequiredEvidence: "Connected TikTok account identity, posting account, and creator info-driven posting settings.",
	},
	"user.info.profile": {
		Scope:            "user.info.profile",
		Label:            "Profile info",
		UseCase:          "analytics",
		PrimarySurface:   "tiktok_analytics",
		RequiredEvidence: "TikTok profile card, avatar, display name, username, or equivalent profile identity fields.",
	},
	"user.info.stats": {
		Scope:            "user.info.stats",
		Label:            "Account stats",
		UseCase:          "analytics",
		PrimarySurface:   "tiktok_analytics",
		RequiredEvidence: "Followers, following, likes, video count, or equivalent TikTok account stats.",
	},
	"video.list": {
		Scope:            "video.list",
		Label:            "Video list",
		UseCase:          "analytics",
		PrimarySurface:   "tiktok_analytics,tiktok_profile",
		RequiredEvidence: "Public TikTok videos listed in UniPost and compared with the TikTok profile page.",
	},
	"video.publish": {
		Scope:            "video.publish",
		Label:            "Publish videos",
		UseCase:          "content_posting",
		PrimarySurface:   "create_post_drawer,posts_list,tiktok_profile",
		RequiredEvidence: "Publish click, publish status, post id or result, and TikTok-side verification where possible.",
	},
	"video.upload": {
		Scope:            "video.upload",
		Label:            "Upload videos",
		UseCase:          "content_posting",
		PrimarySurface:   "create_post_drawer",
		RequiredEvidence: "Video file selected or uploaded, validated, and previewed before publish.",
	},
}

type TikTokDemoPlanInput struct {
	Scopes []string `json:"scopes"`
}

type TikTokScopeTemplate struct {
	Scope            string `json:"scope"`
	Label            string `json:"label"`
	UseCase          string `json:"use_case"`
	PrimarySurface   string `json:"primary_surface"`
	RequiredEvidence string `json:"required_evidence"`
}

type TikTokDemoPlan struct {
	TemplateVersion string                 `json:"template_version"`
	Platform        string                 `json:"platform"`
	UseCase         string                 `json:"use_case"`
	RequestedScopes []string               `json:"requested_scopes"`
	OAuthPrelude    TikTokOAuthPrelude     `json:"oauth_prelude"`
	Recording       TikTokRecordingProfile `json:"recording"`
	Segments        []TikTokDemoSegment    `json:"segments"`
	ScopeCoverage   []TikTokScopeCoverage  `json:"scope_coverage"`
	Warnings        []string               `json:"warnings"`
}

type TikTokOAuthPrelude struct {
	Required     bool     `json:"required"`
	Title        string   `json:"title"`
	Instructions []string `json:"instructions"`
}

type TikTokRecordingProfile struct {
	Resolution         string `json:"resolution"`
	FPS                int    `json:"fps"`
	MaxFileSizeMB      int    `json:"max_file_size_mb"`
	ShowAddressBar     bool   `json:"show_address_bar"`
	SplitAutomatically bool   `json:"split_automatically"`
}

type TikTokDemoSegment struct {
	Key                  string           `json:"key"`
	Title                string           `json:"title"`
	Filename             string           `json:"filename"`
	Description          string           `json:"description"`
	PrimarySurface       string           `json:"primary_surface"`
	Scopes               []string         `json:"scopes"`
	EstimatedDurationSec int              `json:"estimated_duration_sec"`
	Steps                []TikTokDemoStep `json:"steps"`
}

type TikTokDemoStep struct {
	Key        string   `json:"key"`
	Title      string   `json:"title"`
	Surface    string   `json:"surface"`
	Evidence   string   `json:"evidence"`
	Scopes     []string `json:"scopes"`
	UserAction bool     `json:"user_action"`
}

type TikTokScopeCoverage struct {
	Scope    string   `json:"scope"`
	Segments []string `json:"segments"`
	Evidence []string `json:"evidence"`
}

func ListTikTokScopeTemplates() []TikTokScopeTemplate {
	templates := make([]TikTokScopeTemplate, 0, len(tiktokScopeTemplates))
	for _, scope := range tiktokScopeOrder {
		templates = append(templates, tiktokScopeTemplates[scope])
	}
	sort.SliceStable(templates, func(i, j int) bool {
		return templates[i].Scope < templates[j].Scope
	})
	return templates
}

func BuildTikTokDemoPlan(input TikTokDemoPlanInput) (TikTokDemoPlan, error) {
	scopes, err := normalizeTikTokScopes(input.Scopes)
	if err != nil {
		return TikTokDemoPlan{}, err
	}
	if len(scopes) == 0 {
		return TikTokDemoPlan{}, fmt.Errorf("at least one TikTok scope is required")
	}

	useCase := tikTokPlanUseCase(scopes)
	segments := make([]TikTokDemoSegment, 0, 5)
	if hasAnyTikTokScope(scopes, "user.info.basic", "video.upload", "video.publish") {
		segments = append(segments, postingSegments(scopes)...)
	}
	if hasAnyTikTokScope(scopes, "user.info.profile", "user.info.stats", "video.list") {
		segments = append(segments, analyticsSegments(scopes)...)
	}

	plan := TikTokDemoPlan{
		TemplateVersion: TikTokTemplateVersion,
		Platform:        "tiktok",
		UseCase:         useCase,
		RequestedScopes: scopes,
		OAuthPrelude: TikTokOAuthPrelude{
			Required: true,
			Title:    "Connect TikTok and show authorization scopes",
			Instructions: []string{
				"Show TikTok disconnected in the customer app.",
				"Click Connect TikTok.",
				"Record the TikTok OAuth authorization page with the customer's app name and requested scopes.",
				"Return to the customer domain and show TikTok connected.",
			},
		},
		Recording: TikTokRecordingProfile{
			Resolution:         "1080p",
			FPS:                30,
			MaxFileSizeMB:      50,
			ShowAddressBar:     true,
			SplitAutomatically: true,
		},
		Segments:      segments,
		ScopeCoverage: buildScopeCoverage(scopes, segments),
		Warnings:      tikTokPlanWarnings(scopes, useCase),
	}
	return plan, nil
}

func postingSegments(scopes []string) []TikTokDemoSegment {
	return []TikTokDemoSegment{
		{
			Key:                  "posting_part_1",
			Title:                "Content Posting Part 1 - Creator Info, Upload, and Content Details",
			Filename:             "tiktok-content-posting-part-1.mp4",
			Description:          "Show creator info, select a review-safe video, enter caption, and begin disclosure settings.",
			PrimarySurface:       "create_post_drawer",
			Scopes:               selectedScopes(scopes, "user.info.basic", "video.upload"),
			EstimatedDurationSec: 70,
			Steps: []TikTokDemoStep{
				step("creator_info", "1. Retrieve Creator Info", "create_post_drawer", "Show posting account identity, privacy options, interaction controls, and max video duration.", scopes, true, "user.info.basic"),
				step("upload_video", "2. User Uploads Video And Enters Post Details", "create_post_drawer", "Upload or select a review-safe video and show preview/validation.", scopes, true, "video.upload"),
				step("caption_visibility", "2. User Uploads Video And Enters Post Details", "create_post_drawer", "Enter caption and choose explicit review-safe visibility.", scopes, true, "video.publish"),
				step("content_disclosure", "3a. Content Disclosure Settings", "create_post_drawer", "Show disclose video content, Your Brand, and Branded Content controls.", scopes, true, "video.publish"),
			},
		},
		{
			Key:                  "posting_part_2",
			Title:                "Content Posting Part 2 - Privacy Management and Compliance",
			Filename:             "tiktok-content-posting-part-2.mp4",
			Description:          "Show privacy management, interaction controls, music usage, and TikTok policy links.",
			PrimarySurface:       "create_post_drawer,tiktok_policy",
			Scopes:               selectedScopes(scopes, "user.info.basic", "video.publish"),
			EstimatedDurationSec: 65,
			Steps: []TikTokDemoStep{
				step("privacy_management", "3b. Privacy Management", "create_post_drawer", "Open visibility selector and show available privacy options from TikTok account capabilities.", scopes, true, "user.info.basic", "video.publish"),
				step("interaction_controls", "3b. Privacy Management", "create_post_drawer", "Show comment, duet, and stitch controls including disabled states when unavailable.", scopes, false, "user.info.basic", "video.publish"),
				step("music_confirmation", "4. Compliance Requirements", "create_post_drawer,tiktok_policy", "Show and open Music Usage Confirmation or equivalent TikTok policy link.", scopes, true, "video.publish"),
				step("branded_policy", "4. Compliance Requirements", "create_post_drawer,tiktok_policy", "Show and open Branded Content Policy or relevant disclosure policy link.", scopes, true, "video.publish"),
			},
		},
		{
			Key:                  "posting_part_3",
			Title:                "Content Posting Part 3 - Preview, Publish, and Verification",
			Filename:             "tiktok-content-posting-part-3.mp4",
			Description:          "Preview the post, publish it, show UniPost status, and verify on TikTok where possible.",
			PrimarySurface:       "create_post_drawer,posts_list,tiktok_profile",
			Scopes:               selectedScopes(scopes, "video.publish"),
			EstimatedDurationSec: 50,
			Steps: []TikTokDemoStep{
				step("preview_publish", "5. Preview And Publish", "create_post_drawer", "Show final preview and click Publish.", scopes, true, "video.publish"),
				step("publish_status", "5. Preview And Publish", "posts_list", "Show publish progress, status, post id, or final result.", scopes, false, "video.publish"),
				step("tiktok_verification", "5. Preview And Publish", "tiktok_profile", "Open TikTok profile or post page and show the review video where possible.", scopes, true, "video.publish"),
			},
		},
	}
}

func analyticsSegments(scopes []string) []TikTokDemoSegment {
	part2Steps := []TikTokDemoStep{
		step("profile_card", "1. user.info.profile", "tiktok_analytics", "Show TikTok profile card, avatar, display name, username, or equivalent profile fields.", scopes, false, "user.info.profile"),
		step("account_stats", "2. user.info.stats", "tiktok_analytics", "Show followers, following, likes, video count, or equivalent TikTok account stats.", scopes, false, "user.info.stats"),
	}
	if contains(scopes, "video.list") {
		part2Steps = append(part2Steps, step("video_list", "3. video.list", "tiktok_analytics,tiktok_profile", "Show TikTok videos list in UniPost and compare with the TikTok profile page.", scopes, true, "video.list"))
	}

	return []TikTokDemoSegment{
		{
			Key:                  "analytics_part_1",
			Title:                "TikTok Analytics Part 1 - Login, OAuth, and Navigation",
			Filename:             "tiktok-analytics-part-1.mp4",
			Description:          "Show login, TikTok connection, OAuth authorization, and navigation to TikTok Analytics.",
			PrimarySurface:       "connection_flow,tiktok_oauth,tiktok_analytics",
			Scopes:               selectedScopes(scopes, "user.info.profile", "user.info.stats", "video.list"),
			EstimatedDurationSec: 50,
			Steps: []TikTokDemoStep{
				step("login_customer_app", "Login To Customer App", "dashboard", "Log in to the customer-branded workspace and open TikTok connection.", scopes, true, "user.info.profile", "user.info.stats", "video.list"),
				step("authorize_analytics", "Authorize Access To Customer App", "tiktok_oauth", "Show TikTok OAuth consent page with selected analytics scopes.", scopes, true, "user.info.profile", "user.info.stats", "video.list"),
				step("open_analytics", "Authorize Access To Customer App", "tiktok_analytics", "Return to the customer domain and navigate to Analytics > Platforms > TikTok Analytics.", scopes, true, "user.info.profile", "user.info.stats", "video.list"),
			},
		},
		{
			Key:                  "analytics_part_2",
			Title:                "TikTok Analytics Part 2 - Profile and Stats Evidence",
			Filename:             "tiktok-analytics-part-2.mp4",
			Description:          "Show profile, account stats, and optional video list evidence.",
			PrimarySurface:       "tiktok_analytics",
			Scopes:               selectedScopes(scopes, "user.info.profile", "user.info.stats", "video.list"),
			EstimatedDurationSec: 45,
			Steps:                part2Steps,
		},
	}
}

func step(key, title, surface, evidence string, selected []string, userAction bool, scopes ...string) TikTokDemoStep {
	return TikTokDemoStep{
		Key:        key,
		Title:      title,
		Surface:    surface,
		Evidence:   evidence,
		Scopes:     selectedScopes(selected, scopes...),
		UserAction: userAction,
	}
}

func buildScopeCoverage(scopes []string, segments []TikTokDemoSegment) []TikTokScopeCoverage {
	coverage := make([]TikTokScopeCoverage, 0, len(scopes))
	for _, scope := range scopes {
		item := TikTokScopeCoverage{Scope: scope}
		for _, segment := range segments {
			if contains(segment.Scopes, scope) {
				item.Segments = append(item.Segments, segment.Key)
			}
			for _, step := range segment.Steps {
				if contains(step.Scopes, scope) && step.Evidence != "" {
					item.Evidence = append(item.Evidence, step.Evidence)
				}
			}
		}
		coverage = append(coverage, item)
	}
	return coverage
}

func tikTokPlanUseCase(scopes []string) string {
	hasPosting := hasAnyTikTokScope(scopes, "user.info.basic", "video.upload", "video.publish")
	hasAnalytics := hasAnyTikTokScope(scopes, "user.info.profile", "user.info.stats", "video.list")
	if hasPosting && hasAnalytics {
		return "mixed"
	}
	if hasAnalytics {
		return "analytics"
	}
	return "content_posting"
}

func tikTokPlanWarnings(scopes []string, useCase string) []string {
	warnings := []string{}
	if useCase == "mixed" {
		warnings = append(warnings, "Posting and analytics scopes should be recorded as separate ordered video groups so each file stays under 50 MB and each scope is easy to review.")
	}
	if contains(scopes, "video.list") {
		warnings = append(warnings, "Include video.list evidence only if the TikTok app requested video.list, and use public videos that can be compared with the TikTok profile page.")
	}
	if hasAnyTikTokScope(scopes, "video.publish", "video.upload") {
		warnings = append(warnings, "During TikTok review, publish visibility may be SELF_ONLY; explain this review-safe visibility in the submission notes.")
	}
	return warnings
}

func normalizeTikTokScopes(values []string) ([]string, error) {
	seen := map[string]bool{}
	for _, value := range values {
		scope := strings.TrimSpace(value)
		if scope == "" {
			continue
		}
		if _, ok := tiktokScopeTemplates[scope]; !ok {
			return nil, fmt.Errorf("unsupported TikTok scope %q", scope)
		}
		seen[scope] = true
	}
	scopes := make([]string, 0, len(seen))
	for _, scope := range tiktokScopeOrder {
		if seen[scope] {
			scopes = append(scopes, scope)
		}
	}
	return scopes, nil
}

func selectedScopes(selected []string, candidates ...string) []string {
	result := []string{}
	for _, candidate := range candidates {
		if contains(selected, candidate) && !contains(result, candidate) {
			result = append(result, candidate)
		}
	}
	return result
}

func hasAnyTikTokScope(scopes []string, candidates ...string) bool {
	for _, candidate := range candidates {
		if contains(scopes, candidate) {
			return true
		}
	}
	return false
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func (s TikTokDemoSegment) HasStep(key string) bool {
	for _, step := range s.Steps {
		if step.Key == key {
			return true
		}
	}
	return false
}
