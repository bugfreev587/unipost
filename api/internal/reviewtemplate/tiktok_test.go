package reviewtemplate

import (
	"strings"
	"testing"
)

func TestBuildTikTokDemoPlanContentPosting(t *testing.T) {
	plan, err := BuildTikTokDemoPlan(TikTokDemoPlanInput{Scopes: []string{"user.info.basic", "video.upload", "video.publish"}})
	if err != nil {
		t.Fatalf("BuildTikTokDemoPlan: %v", err)
	}
	if plan.UseCase != "content_posting" {
		t.Fatalf("use case = %q", plan.UseCase)
	}
	if !plan.OAuthPrelude.Required {
		t.Fatalf("oauth prelude must be required: %+v", plan.OAuthPrelude)
	}
	if len(plan.Segments) != 3 {
		t.Fatalf("expected 3 posting segments, got %+v", plan.Segments)
	}
	assertSegment(t, plan, "posting_part_1", []string{"user.info.basic", "video.upload"})
	assertSegment(t, plan, "posting_part_2", []string{"user.info.basic", "video.publish"})
	assertSegment(t, plan, "posting_part_3", []string{"video.publish"})
	if plan.Recording.Resolution != "1080p" || plan.Recording.MaxFileSizeMB != 50 {
		t.Fatalf("unexpected recording constraints: %+v", plan.Recording)
	}
}

func TestBuildTikTokDemoPlanAnalytics(t *testing.T) {
	plan, err := BuildTikTokDemoPlan(TikTokDemoPlanInput{Scopes: []string{"user.info.profile", "user.info.stats"}})
	if err != nil {
		t.Fatalf("BuildTikTokDemoPlan: %v", err)
	}
	if plan.UseCase != "analytics" {
		t.Fatalf("use case = %q", plan.UseCase)
	}
	if len(plan.Segments) != 2 {
		t.Fatalf("expected 2 analytics segments, got %+v", plan.Segments)
	}
	assertSegment(t, plan, "analytics_part_1", []string{"user.info.profile", "user.info.stats"})
	assertSegment(t, plan, "analytics_part_2", []string{"user.info.profile", "user.info.stats"})
	if findSegment(plan, "analytics_part_2").HasStep("video_list") {
		t.Fatalf("video.list evidence should not be included unless requested: %+v", findSegment(plan, "analytics_part_2").Steps)
	}
}

func TestBuildTikTokDemoPlanIncludesVideoListOnlyWhenRequested(t *testing.T) {
	plan, err := BuildTikTokDemoPlan(TikTokDemoPlanInput{Scopes: []string{"video.list", "user.info.profile", "user.info.stats"}})
	if err != nil {
		t.Fatalf("BuildTikTokDemoPlan: %v", err)
	}
	segment := findSegment(plan, "analytics_part_2")
	if !segment.HasStep("video_list") {
		t.Fatalf("expected video.list evidence step: %+v", segment.Steps)
	}
	assertSegment(t, plan, "analytics_part_2", []string{"user.info.profile", "user.info.stats", "video.list"})
}

func TestBuildTikTokDemoPlanMixedScopes(t *testing.T) {
	plan, err := BuildTikTokDemoPlan(TikTokDemoPlanInput{Scopes: []string{"video.publish", "user.info.basic", "user.info.stats"}})
	if err != nil {
		t.Fatalf("BuildTikTokDemoPlan: %v", err)
	}
	if plan.UseCase != "mixed" {
		t.Fatalf("use case = %q", plan.UseCase)
	}
	if len(plan.Segments) != 5 {
		t.Fatalf("expected posting and analytics segments, got %+v", plan.Segments)
	}
	if len(plan.Warnings) == 0 {
		t.Fatal("expected mixed-plan warning about separate uploads")
	}
}

func TestBuildTikTokDemoPlanRejectsUnsupportedScope(t *testing.T) {
	_, err := BuildTikTokDemoPlan(TikTokDemoPlanInput{Scopes: []string{"comment.list"}})
	if err == nil || !strings.Contains(err.Error(), "unsupported TikTok scope") {
		t.Fatalf("expected unsupported scope error, got %v", err)
	}
}

func TestListTikTokScopeTemplatesIsDeterministic(t *testing.T) {
	templates := ListTikTokScopeTemplates()
	if len(templates) != 6 {
		t.Fatalf("expected 6 templates, got %d", len(templates))
	}
	for i := 1; i < len(templates); i++ {
		if templates[i-1].Scope > templates[i].Scope {
			t.Fatalf("templates not sorted: %+v", templates)
		}
	}
}

func assertSegment(t *testing.T, plan TikTokDemoPlan, key string, scopes []string) {
	t.Helper()
	segment := findSegment(plan, key)
	if segment.Key == "" {
		t.Fatalf("segment %q not found in %+v", key, plan.Segments)
	}
	for _, scope := range scopes {
		if !contains(segment.Scopes, scope) {
			t.Fatalf("segment %q missing scope %q: %+v", key, scope, segment.Scopes)
		}
	}
}

func findSegment(plan TikTokDemoPlan, key string) TikTokDemoSegment {
	for _, segment := range plan.Segments {
		if segment.Key == key {
			return segment
		}
	}
	return TikTokDemoSegment{}
}
