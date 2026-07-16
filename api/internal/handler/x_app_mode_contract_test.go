package handler

import (
	"os"
	"strings"
	"testing"
)

func TestXAppModePersistsAtAuthorizationStartAcrossConnectionFlows(t *testing.T) {
	checks := map[string][]string{
		"oauth.go": {
			"workspaceIDForProfile",
			"XAppMode:",
			"oauthState.XAppMode",
		},
		"connect_sessions.go": {
			"XAppMode:",
			"xinbox.AppModeWorkspace",
			"xinbox.AppModeUniPostManaged",
		},
		"connect_callback.go": {
			"session.XAppMode",
			"XAppMode:",
		},
		"social_accounts.go": {
			"xinbox.AppModeForManualConnection",
			"XAppMode:",
		},
		"social_posts.go": {
			"account.XAppMode",
			"AppMode:",
		},
	}
	for file, fragments := range checks {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(source)
		for _, fragment := range fragments {
			if !strings.Contains(text, fragment) {
				t.Fatalf("%s missing app-mode persistence fragment %q", file, fragment)
			}
		}
	}
}
