package integrationlogs

import "testing"

func TestNormalize_AppliesDefaults(t *testing.T) {
	params := Normalize(Event{
		WorkspaceID: "ws_123",
		Action:      "custom.action",
	})

	if params.Level != string(LevelInfo) {
		t.Fatalf("level = %q, want %q", params.Level, LevelInfo)
	}
	if params.Status != string(StatusSuccess) {
		t.Fatalf("status = %q, want %q", params.Status, StatusSuccess)
	}
	if params.Category != string(CategorySystem) {
		t.Fatalf("category = %q, want %q", params.Category, CategorySystem)
	}
	if params.Source != string(SourceWorker) {
		t.Fatalf("source = %q, want %q", params.Source, SourceWorker)
	}
	if params.Message != "custom.action" {
		t.Fatalf("message = %q, want action fallback", params.Message)
	}
}

func TestNormalize_NormalizesPlatformAndErrorCode(t *testing.T) {
	params := Normalize(Event{
		WorkspaceID: "ws_123",
		Action:      "custom.action",
		Platform:    "Tik Tok",
		ErrorCode:   "Remote Forbidden",
	})

	if !params.Platform.Valid || params.Platform.String != "tik_tok" {
		t.Fatalf("platform = %#v, want tik_tok", params.Platform)
	}
	if !params.ErrorCode.Valid || params.ErrorCode.String != "remote_forbidden" {
		t.Fatalf("error_code = %#v, want remote_forbidden", params.ErrorCode)
	}
}

func TestPinterestPublishingActionNamesAreStable(t *testing.T) {
	want := map[string]string{
		"preflight_started":   "pinterest_destination_preflight_started",
		"preflight_succeeded": "pinterest_destination_preflight_succeeded",
		"preflight_failed":    "pinterest_destination_preflight_failed",
		"create_pin_failed":   "pinterest_create_pin_failed",
	}
	got := map[string]string{
		"preflight_started":   ActionPinterestDestinationPreflightStarted,
		"preflight_succeeded": ActionPinterestDestinationPreflightSucceeded,
		"preflight_failed":    ActionPinterestDestinationPreflightFailed,
		"create_pin_failed":   ActionPinterestCreatePinFailed,
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("%s action = %q, want %q", key, got[key], value)
		}
	}
}
