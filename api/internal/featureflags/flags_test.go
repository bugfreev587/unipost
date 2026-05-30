package featureflags

import (
	"context"
	"os"
	"testing"
)

func TestTikTokAnalyticsScopesDefaultByEnvironment(t *testing.T) {
	SetProvider(EnvProvider{})
	t.Cleanup(func() { SetProvider(EnvProvider{}) })
	unsetenv(t, "FEATURE_TIKTOK_ANALYTICS_SCOPES")
	unsetenv(t, "TIKTOK_ANALYTICS_SCOPES_ENABLED")

	if Enabled(context.Background(), TikTokAnalyticsScopes, Target{Env: "production"}) {
		t.Fatal("TikTok analytics scopes should default off in production")
	}
	if !Enabled(context.Background(), TikTokAnalyticsScopes, Target{Env: "development"}) {
		t.Fatal("TikTok analytics scopes should default on outside production")
	}
}

func TestEnvProviderExplicitFlagOverridesDefault(t *testing.T) {
	SetProvider(EnvProvider{})
	t.Cleanup(func() { SetProvider(EnvProvider{}) })
	t.Setenv("FEATURE_TIKTOK_ANALYTICS_SCOPES", "true")
	unsetenv(t, "TIKTOK_ANALYTICS_SCOPES_ENABLED")

	if !Enabled(context.Background(), TikTokAnalyticsScopes, Target{Env: "production"}) {
		t.Fatal("explicit env flag should enable TikTok analytics scopes")
	}

	t.Setenv("FEATURE_TIKTOK_ANALYTICS_SCOPES", "false")
	if Enabled(context.Background(), TikTokAnalyticsScopes, Target{Env: "development"}) {
		t.Fatal("explicit env flag should disable TikTok analytics scopes")
	}
}

func TestEnvProviderSupportsLegacyTikTokFlag(t *testing.T) {
	SetProvider(EnvProvider{})
	t.Cleanup(func() { SetProvider(EnvProvider{}) })
	unsetenv(t, "FEATURE_TIKTOK_ANALYTICS_SCOPES")
	t.Setenv("TIKTOK_ANALYTICS_SCOPES_ENABLED", "true")

	if !Enabled(context.Background(), TikTokAnalyticsScopes, Target{Env: "production"}) {
		t.Fatal("legacy TikTok flag should still be honored")
	}
}

func TestInboxDefaultsEnabled(t *testing.T) {
	SetProvider(EnvProvider{})
	t.Cleanup(func() { SetProvider(EnvProvider{}) })

	if !Enabled(context.Background(), Inbox, Target{Env: "production"}) {
		t.Fatal("Inbox should default on because it is already generally supported")
	}
}

func TestLoopsIntegrationDefaultsOffInProduction(t *testing.T) {
	SetProvider(EnvProvider{})
	t.Cleanup(func() { SetProvider(EnvProvider{}) })
	unsetenv(t, "FEATURE_EMAIL_LOOPS_INTEGRATION_V1")

	if Enabled(context.Background(), LoopsIntegrationV1, Target{Env: "production"}) {
		t.Fatal("Loops integration should default off in production")
	}
	if !Enabled(context.Background(), LoopsIntegrationV1, Target{Env: "development"}) {
		t.Fatal("Loops integration should default on outside production")
	}
}

func TestAppReviewAutopilotDefaultsOffInProduction(t *testing.T) {
	SetProvider(EnvProvider{})
	t.Cleanup(func() { SetProvider(EnvProvider{}) })
	unsetenv(t, "FEATURE_APP_REVIEW_AUTOPILOT_V1")

	def, ok := DefinitionFor(AppReviewAutopilotV1)
	if !ok {
		t.Fatal("app review autopilot flag should be registered")
	}
	if def.EnvVar != "FEATURE_APP_REVIEW_AUTOPILOT_V1" {
		t.Fatalf("unexpected env var: %s", def.EnvVar)
	}
	if Enabled(context.Background(), AppReviewAutopilotV1, Target{Env: "production"}) {
		t.Fatal("App Review Autopilot should default off in production")
	}
	if !Enabled(context.Background(), AppReviewAutopilotV1, Target{Env: "development"}) {
		t.Fatal("App Review Autopilot should default on outside production")
	}
}

func TestAppReviewAIAgentFlagDefinition(t *testing.T) {
	unsetenv(t, "FEATURE_APP_REVIEW_AI_AGENT_V1")

	def, ok := DefinitionFor(AppReviewAIAgentV1)
	if !ok {
		t.Fatal("AppReviewAIAgentV1 definition missing")
	}
	if def.EnvVar != "FEATURE_APP_REVIEW_AI_AGENT_V1" {
		t.Fatalf("unexpected env var: %s", def.EnvVar)
	}
	if def.DefaultEnabled(Target{Env: "production"}) {
		t.Fatal("AI review agent must default off in production")
	}
	if !def.DefaultEnabled(Target{Env: "development"}) {
		t.Fatal("AI review agent may default on outside production")
	}
}

func TestPostsCalendarViewFlagDefinition(t *testing.T) {
	unsetenv(t, "FEATURE_POSTS_CALENDAR_VIEW_V1")

	def, ok := DefinitionFor(PostsCalendarViewV1)
	if !ok {
		t.Fatal("PostsCalendarViewV1 definition missing")
	}
	if def.EnvVar != "FEATURE_POSTS_CALENDAR_VIEW_V1" {
		t.Fatalf("unexpected env var: %s", def.EnvVar)
	}
	if def.DefaultEnabled(Target{Env: "production"}) {
		t.Fatal("Posts calendar view must default off in production")
	}
	if !def.DefaultEnabled(Target{Env: "development"}) {
		t.Fatal("Posts calendar view may default on outside production")
	}
}

func unsetenv(t *testing.T, name string) {
	t.Helper()
	old, ok := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("unset %s: %v", name, err)
	}
	t.Cleanup(func() {
		if ok {
			_ = os.Setenv(name, old)
		} else {
			_ = os.Unsetenv(name)
		}
	})
}
