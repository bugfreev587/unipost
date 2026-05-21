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

func TestHostedConnectFacebookPinterestDefaultByEnvironment(t *testing.T) {
	SetProvider(EnvProvider{})
	t.Cleanup(func() { SetProvider(EnvProvider{}) })
	unsetenv(t, "FEATURE_CONNECT_SESSIONS_FACEBOOK_PINTEREST")

	if Enabled(context.Background(), HostedConnectFacebookPinterest, Target{Env: "production"}) {
		t.Fatal("Facebook/Pinterest hosted Connect should default off in production")
	}
	if !Enabled(context.Background(), HostedConnectFacebookPinterest, Target{Env: "development"}) {
		t.Fatal("Facebook/Pinterest hosted Connect should default on outside production")
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
