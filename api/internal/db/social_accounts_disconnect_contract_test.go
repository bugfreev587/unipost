package db

import (
	"strings"
	"testing"
)

func TestDisconnectSocialAccountClearsStoredYouTubeAuthorizedData(t *testing.T) {
	requiredFragments := []string{
		"access_token = CASE WHEN platform = 'youtube' THEN '' ELSE access_token END",
		"refresh_token = CASE WHEN platform = 'youtube' THEN NULL ELSE refresh_token END",
		"token_expires_at = CASE WHEN platform = 'youtube' THEN NULL ELSE token_expires_at END",
		"external_account_id = CASE WHEN platform = 'youtube' THEN 'disconnected:' || id ELSE external_account_id END",
		"account_name = CASE WHEN platform = 'youtube' THEN NULL ELSE account_name END",
		"account_avatar_url = CASE WHEN platform = 'youtube' THEN NULL ELSE account_avatar_url END",
		"metadata = CASE WHEN platform = 'youtube' THEN '{}'::jsonb ELSE metadata END",
		"scope = CASE WHEN platform = 'youtube' THEN ARRAY[]::TEXT[] ELSE scope END",
		"last_refreshed_at = CASE WHEN platform = 'youtube' THEN NOW() ELSE last_refreshed_at END",
	}

	normalized := strings.Join(strings.Fields(disconnectSocialAccount), " ")
	for _, fragment := range requiredFragments {
		if !strings.Contains(normalized, fragment) {
			t.Fatalf("DisconnectSocialAccount must clear %q; query was: %s", fragment, normalized)
		}
	}
}
