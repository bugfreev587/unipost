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
