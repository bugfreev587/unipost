package runtimeenv

import "testing"

func TestCurrentDefaultsToDevelopment(t *testing.T) {
	t.Setenv(EnvVar, "")

	if got := Current(); got != "development" {
		t.Fatalf("Current() = %q, want development", got)
	}
	if IsProduction() {
		t.Fatal("empty runtime env should not be production")
	}
}

func TestIsProductionAliases(t *testing.T) {
	for _, value := range []string{"production", "prod", "live", " Production "} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(EnvVar, value)
			if !IsProduction() {
				t.Fatalf("IsProduction() = false for %q", value)
			}
		})
	}
}

func TestFeatureEnabledUsesExplicitFlag(t *testing.T) {
	t.Setenv("FEATURE_TEST_FLAG", "false")

	if FeatureEnabled("FEATURE_TEST_FLAG", true) {
		t.Fatal("explicit false flag should override default true")
	}
}

func TestFeatureEnabledUsesDefaultWhenUnset(t *testing.T) {
	if !FeatureEnabled("FEATURE_TEST_UNSET_FLAG", true) {
		t.Fatal("unset flag should use default true")
	}
	if FeatureEnabled("FEATURE_TEST_UNSET_FLAG", false) {
		t.Fatal("unset flag should use default false")
	}
}
