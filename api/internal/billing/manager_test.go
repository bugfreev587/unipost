package billing

import "testing"

func TestNewManagerLoadsModeSpecificTrialPortalConfigurations(t *testing.T) {
	t.Setenv("STRIPE_SECRET_KEY", "sk_live_test")
	t.Setenv("STRIPE_BILLING_PORTAL_TRIAL_CONFIGURATION_ID", "bpc_live_trial")
	t.Setenv("STRIPE_SANDBOX_SECRET_KEY", "sk_test_sandbox")
	t.Setenv("STRIPE_SANDBOX_BILLING_PORTAL_TRIAL_CONFIGURATION_ID", "bpc_sandbox_trial")

	manager, err := NewManager(nil)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if got := manager.Live.TrialPortalConfigurationID(); got != "bpc_live_trial" {
		t.Fatalf("live TrialPortalConfigurationID() = %q, want %q", got, "bpc_live_trial")
	}
	if got := manager.Sandbox.TrialPortalConfigurationID(); got != "bpc_sandbox_trial" {
		t.Fatalf("sandbox TrialPortalConfigurationID() = %q, want %q", got, "bpc_sandbox_trial")
	}
}

func TestManagerByNameOnlyReturnsConfiguredExactMode(t *testing.T) {
	live := &Mode{Name: "live"}
	sandbox := &Mode{Name: "sandbox"}
	manager := &Manager{Live: live, Sandbox: sandbox}

	for _, test := range []struct {
		name string
		want *Mode
	}{
		{name: "live", want: live},
		{name: "sandbox", want: sandbox},
		{name: "", want: nil},
		{name: "LIVE", want: nil},
		{name: "unknown", want: nil},
	} {
		if got := manager.ByName(test.name); got != test.want {
			t.Errorf("ByName(%q) = %p, want %p", test.name, got, test.want)
		}
	}

	manager.Sandbox = nil
	if got := manager.ByName("sandbox"); got != nil {
		t.Fatalf("ByName(sandbox) with unavailable sandbox = %p, want nil", got)
	}
	var nilManager *Manager
	if got := nilManager.ByName("live"); got != nil {
		t.Fatalf("nil Manager.ByName(live) = %p, want nil", got)
	}
}

func TestNilModeTrialPortalConfigurationIDIsEmpty(t *testing.T) {
	var mode *Mode
	if got := mode.TrialPortalConfigurationID(); got != "" {
		t.Fatalf("nil mode TrialPortalConfigurationID() = %q, want empty", got)
	}
}
