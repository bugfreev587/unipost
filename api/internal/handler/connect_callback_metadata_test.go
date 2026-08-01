package handler

import (
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/connect"
)

// PRD 2026-07-31 (TikTok managed account identity accuracy): metadata must
// keep the verified TikTok username and the editable display name distinct,
// mark corrected rows with identity schema v2, and never present a display
// name as a verified username when TikTok omits `username`.
func TestBuildConnectAccountMetadataTikTokIdentitySchema(t *testing.T) {
	t.Run("username present stays distinct from display name", func(t *testing.T) {
		got := buildConnectAccountMetadata("tiktok", "open-123", &connect.Profile{
			ExternalAccountID: "open-123",
			Username:          "robyn6608",
			DisplayName:       "Robyn",
		})
		if got["username"] != "robyn6608" {
			t.Errorf("username: got %v", got["username"])
		}
		if got["display_name"] != "Robyn" {
			t.Errorf("display_name: got %v", got["display_name"])
		}
		if got["tiktok_identity_schema_version"] != tiktokIdentitySchemaVersion {
			t.Errorf("schema version: got %v", got["tiktok_identity_schema_version"])
		}
		if _, exists := got["username_missing"]; exists {
			t.Error("username_missing must be absent when TikTok returned a username")
		}
	})

	t.Run("missing username omits key and records username_missing", func(t *testing.T) {
		got := buildConnectAccountMetadata("tiktok", "open-123", &connect.Profile{
			ExternalAccountID: "open-123",
			Username:          "",
			DisplayName:       "Robyn",
		})
		if _, exists := got["username"]; exists {
			t.Error("username key must be omitted when TikTok did not return one")
		}
		if got["username_missing"] != true {
			t.Errorf("username_missing: got %v", got["username_missing"])
		}
		if got["tiktok_identity_schema_version"] != tiktokIdentitySchemaVersion {
			t.Errorf("schema version must still mark the post-fix fetch: got %v", got["tiktok_identity_schema_version"])
		}
		if got["display_name"] != "Robyn" {
			t.Errorf("display_name: got %v", got["display_name"])
		}
	})

	t.Run("other platforms keep the legacy shape", func(t *testing.T) {
		got := buildConnectAccountMetadata("twitter", "tw-1", &connect.Profile{
			ExternalAccountID: "tw-1",
			Username:          "handle",
			DisplayName:       "Handle Name",
		})
		if got["username"] != "handle" || got["display_name"] != "Handle Name" {
			t.Errorf("unexpected metadata: %v", got)
		}
		if _, exists := got["tiktok_identity_schema_version"]; exists {
			t.Error("non-TikTok platforms must not carry the TikTok schema marker")
		}
	})

	t.Run("instagram keeps its webhook identity", func(t *testing.T) {
		got := buildConnectAccountMetadata("instagram", "ig-webhook-9", &connect.Profile{
			ExternalAccountID: "ig-1",
			Username:          "iguser",
			DisplayName:       "IG User",
		})
		if got["instagram_webhook_user_id"] != "ig-webhook-9" {
			t.Errorf("instagram_webhook_user_id: got %v", got["instagram_webhook_user_id"])
		}
	})
}
