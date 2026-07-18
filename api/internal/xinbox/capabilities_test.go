package xinbox

import (
	"reflect"
	"testing"
)

func boolPointer(value bool) *bool { return &value }

func TestXInboxCapabilityFlagOffKeepsCommentsWithoutDMReconnect(t *testing.T) {
	got := EvaluateCapabilities(CapabilityInput{
		PlanAllowsInbox: true,
		DMsAvailable:    boolPointer(false),
		AccountStatus:   "active",
		Scopes:          []string{"tweet.read", "tweet.write", "users.read", "offline.access"},
		AppMode:         AppModeUniPostManaged,
	})

	if !got.CommentsEnabled || got.DMsEnabled {
		t.Fatalf("capabilities = %+v, want comments only", got)
	}
	if got.ReconnectRequired || len(got.MissingScopes) != 0 {
		t.Fatalf("capabilities = %+v, want no DM reconnect prompt while rollout is off", got)
	}
}

func TestXInboxCapabilityKeepsPublishingAndCommentsWhenDMScopesAreMissing(t *testing.T) {
	got := EvaluateCapabilities(CapabilityInput{
		PlanAllowsInbox: true,
		AccountStatus:   "active",
		Scopes:          []string{"tweet.read", "tweet.write", "users.read", "offline.access"},
		AppMode:         AppModeUniPostManaged,
	})

	if !got.CommentsEnabled {
		t.Fatal("comments_enabled = false, want true")
	}
	if got.DMsEnabled {
		t.Fatal("dms_enabled = true, want false")
	}
	if want := []string{"dm.read", "dm.write"}; !reflect.DeepEqual(got.MissingScopes, want) {
		t.Fatalf("missing_scopes = %v, want %v", got.MissingScopes, want)
	}
	if !got.ReconnectRequired {
		t.Fatal("reconnect_required = false, want true")
	}
	if got.DeliveryStatus != DeliveryStatusPending {
		t.Fatalf("delivery_status = %q, want %q", got.DeliveryStatus, DeliveryStatusPending)
	}
	if got.AppMode != AppModeUniPostManaged {
		t.Fatalf("app_mode = %q, want %q", got.AppMode, AppModeUniPostManaged)
	}
	if len(got.MissingAppCredentials) != 0 {
		t.Fatalf("missing_app_credentials = %v, want empty", got.MissingAppCredentials)
	}
}

func TestXInboxCapabilityAPIPlanDoesNotPromptReconnect(t *testing.T) {
	got := EvaluateCapabilities(CapabilityInput{
		PlanAllowsInbox: false,
		AccountStatus:   "active",
		Scopes:          []string{"tweet.read", "tweet.write", "users.read"},
		AppMode:         AppModeUniPostManaged,
	})

	if got.CommentsEnabled || got.DMsEnabled {
		t.Fatalf("comments_enabled=%v dms_enabled=%v, want both false", got.CommentsEnabled, got.DMsEnabled)
	}
	if got.ReconnectRequired {
		t.Fatal("reconnect_required = true, want false")
	}
	if len(got.MissingScopes) != 0 {
		t.Fatalf("missing_scopes = %v, want empty for plan-ineligible workspace", got.MissingScopes)
	}
	if got.DeliveryStatus != DeliveryStatusPausedPlan {
		t.Fatalf("delivery_status = %q, want %q", got.DeliveryStatus, DeliveryStatusPausedPlan)
	}
}

func TestXInboxCapabilityWorkspaceAppListsExactMissingCredentials(t *testing.T) {
	got := EvaluateCapabilities(CapabilityInput{
		PlanAllowsInbox: true,
		AccountStatus:   "active",
		Scopes:          RequiredInboxScopes(),
		AppMode:         AppModeWorkspace,
		AppCredentials: AppCredentials{
			ClientIDConfigured:     true,
			ClientSecretConfigured: true,
		},
	})

	if got.CommentsEnabled || got.DMsEnabled {
		t.Fatalf("comments_enabled=%v dms_enabled=%v, want both false without app-level credentials", got.CommentsEnabled, got.DMsEnabled)
	}
	if want := []string{"app_bearer_token", "consumer_secret"}; !reflect.DeepEqual(got.MissingAppCredentials, want) {
		t.Fatalf("missing_app_credentials = %v, want %v", got.MissingAppCredentials, want)
	}
	if got.ReconnectRequired {
		t.Fatal("reconnect_required = true, want false when OAuth scopes are complete")
	}
}

func TestXInboxCapabilityReportsAllRequiredScopesInStableOrder(t *testing.T) {
	got := EvaluateCapabilities(CapabilityInput{
		PlanAllowsInbox: true,
		AccountStatus:   "active",
		AppMode:         AppModeUniPostManaged,
	})

	if want := []string{"tweet.read", "tweet.write", "users.read", "dm.read", "dm.write"}; !reflect.DeepEqual(got.MissingScopes, want) {
		t.Fatalf("missing_scopes = %v, want %v", got.MissingScopes, want)
	}
	if got.CommentsEnabled || got.DMsEnabled {
		t.Fatalf("comments_enabled=%v dms_enabled=%v, want both false", got.CommentsEnabled, got.DMsEnabled)
	}
	if !got.ReconnectRequired {
		t.Fatal("reconnect_required = false, want true")
	}
}

func TestXInboxCapabilityLegacyUnknownRequiresReconnectWithoutChangingAccountStatus(t *testing.T) {
	got := EvaluateCapabilities(CapabilityInput{
		PlanAllowsInbox: true,
		AccountStatus:   "active",
		Scopes:          RequiredInboxScopes(),
		AppMode:         AppModeLegacyUnknown,
		DeliveryStatus:  DeliveryStatusActive,
	})

	if got.CommentsEnabled || got.DMsEnabled {
		t.Fatalf("comments_enabled=%v dms_enabled=%v, want both false", got.CommentsEnabled, got.DMsEnabled)
	}
	if !got.ReconnectRequired {
		t.Fatal("reconnect_required = false, want true")
	}
	if got.DeliveryStatus != DeliveryStatusActive {
		t.Fatalf("delivery_status = %q, want unchanged %q", got.DeliveryStatus, DeliveryStatusActive)
	}
}

func TestParseXAppModeRejectsEmptyAndInvalidValues(t *testing.T) {
	for _, raw := range []string{"", "managed", "garbage"} {
		if _, err := ParseAppMode(raw); err == nil {
			t.Fatalf("ParseAppMode(%q) error = nil, want validation error", raw)
		}
	}
	for _, mode := range []AppMode{AppModeUniPostManaged, AppModeWorkspace, AppModeLegacyUnknown} {
		got, err := ParseAppMode(string(mode))
		if err != nil || got != mode {
			t.Fatalf("ParseAppMode(%q) = %q, %v", mode, got, err)
		}
	}
}

func TestNormalizePersistedXAppModeTreatsBlankAsLegacyUnknown(t *testing.T) {
	for _, raw := range []string{"", "   "} {
		got, err := NormalizePersistedAppMode(raw)
		if err != nil {
			t.Fatalf("NormalizePersistedAppMode(%q): %v", raw, err)
		}
		if got != AppModeLegacyUnknown {
			t.Fatalf("NormalizePersistedAppMode(%q) = %q, want %q", raw, got, AppModeLegacyUnknown)
		}
	}
	if _, err := NormalizePersistedAppMode("garbage"); err == nil {
		t.Fatal("invalid non-empty persisted mode error = nil, want validation error")
	}
}

func TestXInboxCapabilityNormalizesEmptyAppModeToLegacyReconnect(t *testing.T) {
	got := EvaluateCapabilities(CapabilityInput{
		PlanAllowsInbox: true,
		AccountStatus:   "active",
		Scopes:          RequiredInboxScopes(),
	})
	if got.AppMode != AppModeLegacyUnknown {
		t.Fatalf("app_mode = %q, want %q", got.AppMode, AppModeLegacyUnknown)
	}
	if got.CommentsEnabled || got.DMsEnabled || !got.ReconnectRequired {
		t.Fatalf("capabilities = %+v, want Inbox disabled with reconnect required", got)
	}
}

func TestXAppModeForManualTwitterConnectionUsesWorkspaceApp(t *testing.T) {
	mode, ok := AppModeForManualConnection("twitter")
	if !ok || mode != AppModeWorkspace {
		t.Fatalf("mode=%q ok=%v, want workspace X app", mode, ok)
	}
	if _, ok := AppModeForManualConnection("linkedin"); ok {
		t.Fatal("non-X manual connection unexpectedly received an X app mode")
	}
}
