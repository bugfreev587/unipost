package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func identityTestAccount(platform string, metadata string) db.SocialAccount {
	return db.SocialAccount{
		ID:                "sa_1",
		ProfileID:         "pr_1",
		Platform:          platform,
		ExternalAccountID: "open-123",
		AccountName:       pgtype.Text{String: "Robyn", Valid: true},
		ConnectedAt:       pgtype.Timestamptz{Time: time.Date(2026, 6, 17, 21, 42, 27, 0, time.UTC), Valid: true},
		Metadata:          []byte(metadata),
		Status:            "active",
		ConnectionType:    "managed",
	}
}

// PRD 2026-07-31 §9.5: /accounts identity contract for TikTok schema states.
func TestSocialAccountResponseTikTokIdentityContract(t *testing.T) {
	t.Run("corrected v2 row returns verified username and display name", func(t *testing.T) {
		acc := identityTestAccount("tiktok", `{"username":"robyn6608","display_name":"Robyn","tiktok_identity_schema_version":2}`)
		acc.LastConnectedAt = pgtype.Timestamptz{Time: time.Date(2026, 7, 31, 20, 42, 26, 0, time.UTC), Valid: true}
		resp := toSocialAccountResponse(acc)
		if resp.Username == nil || *resp.Username != "robyn6608" {
			t.Fatalf("username: got %v", resp.Username)
		}
		if resp.DisplayName == nil || *resp.DisplayName != "Robyn" {
			t.Fatalf("display_name: got %v", resp.DisplayName)
		}
		if resp.IdentityRefreshRequired {
			t.Fatal("v2 row must not require identity refresh")
		}
		if resp.LastConnectedAt == nil || !resp.LastConnectedAt.Equal(time.Date(2026, 7, 31, 20, 42, 26, 0, time.UTC)) {
			t.Fatalf("last_connected_at: got %v", resp.LastConnectedAt)
		}
	})

	t.Run("legacy row returns null username and requires refresh", func(t *testing.T) {
		// Pre-fix rows duplicated display_name into metadata.username.
		acc := identityTestAccount("tiktok", `{"username":"Robyn","display_name":"Robyn"}`)
		resp := toSocialAccountResponse(acc)
		if resp.Username != nil {
			t.Fatalf("legacy username must be null, got %q", *resp.Username)
		}
		if !resp.IdentityRefreshRequired {
			t.Fatal("legacy active managed TikTok row must require identity refresh")
		}
		if resp.AccountName == nil || *resp.AccountName != "Robyn" {
			t.Fatalf("account_name compatibility value: got %v", resp.AccountName)
		}
	})

	t.Run("v2 row with username_missing stays null without refresh loop", func(t *testing.T) {
		acc := identityTestAccount("tiktok", `{"display_name":"Robyn","username_missing":true,"tiktok_identity_schema_version":2}`)
		resp := toSocialAccountResponse(acc)
		if resp.Username != nil {
			t.Fatalf("username must stay null, got %q", *resp.Username)
		}
		if resp.IdentityRefreshRequired {
			t.Fatal("username_missing on a v2 row must not create an endless reconnect prompt")
		}
	})

	t.Run("legacy detection ignores value equality", func(t *testing.T) {
		// A valid v2 account may legitimately use the same value for both
		// fields; the schema marker, not equality, decides legacy state.
		acc := identityTestAccount("tiktok", `{"username":"robyn","display_name":"robyn","tiktok_identity_schema_version":2}`)
		resp := toSocialAccountResponse(acc)
		if resp.Username == nil || *resp.Username != "robyn" {
			t.Fatalf("username: got %v", resp.Username)
		}
		if resp.IdentityRefreshRequired {
			t.Fatal("equal username/display_name on a v2 row is not legacy state")
		}
	})

	t.Run("disconnected legacy row is not prompted", func(t *testing.T) {
		acc := identityTestAccount("tiktok", `{"username":"Robyn","display_name":"Robyn"}`)
		acc.DisconnectedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		resp := toSocialAccountResponse(acc)
		if resp.IdentityRefreshRequired {
			t.Fatal("disconnected rows go through the reconnect flow already; no extra prompt")
		}
	})

	t.Run("non-tiktok platforms expose metadata identity without refresh flag", func(t *testing.T) {
		acc := identityTestAccount("twitter", `{"username":"handle","display_name":"Handle Name"}`)
		resp := toSocialAccountResponse(acc)
		if resp.Username == nil || *resp.Username != "handle" {
			t.Fatalf("username: got %v", resp.Username)
		}
		if resp.IdentityRefreshRequired {
			t.Fatal("non-TikTok platforms never require identity refresh")
		}
	})

	t.Run("response does not add an avatar url field", func(t *testing.T) {
		// PRD §9.7: stored TikTok avatars are expiring signed provider URLs;
		// P0 deliberately keeps them out of the list contract.
		raw, err := json.Marshal(toSocialAccountResponse(identityTestAccount("tiktok", `{}`)))
		if err != nil {
			t.Fatal(err)
		}
		var asMap map[string]any
		if err := json.Unmarshal(raw, &asMap); err != nil {
			t.Fatal(err)
		}
		if _, exists := asMap["account_avatar_url"]; exists {
			t.Fatal("account_avatar_url must not be part of the P0 contract")
		}
		for _, key := range []string{"username", "display_name", "identity_refresh_required", "last_connected_at"} {
			if _, exists := asMap[key]; !exists {
				t.Fatalf("expected additive field %q in response", key)
			}
		}
	})
}
