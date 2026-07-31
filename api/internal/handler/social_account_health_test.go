package handler

import (
	"os"
	"strings"
	"testing"
)

func TestAccountHealthLoadsResolvedConnectionState(t *testing.T) {
	source, err := os.ReadFile("social_account_health.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	if !strings.Contains(body, "GetResolvedSocialAccountByIDAndWorkspace") {
		t.Fatal("account health must resolve connection-owned status and token expiry")
	}
	if strings.Contains(body, "GetSocialAccountByIDAndWorkspace(") {
		t.Fatal("account health must not read binding-owned credentials/state")
	}
}
