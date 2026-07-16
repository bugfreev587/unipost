package db

import (
	"os"
	"strings"
	"testing"
)

func TestPlatformCredentialOptionalSecretsUseAtomicSuppliedFlags(t *testing.T) {
	data, err := os.ReadFile("queries/platform_credentials.sql")
	if err != nil {
		t.Fatal(err)
	}
	query := string(data)
	for _, required := range []string{
		"app_bearer_token_supplied",
		"consumer_secret_supplied",
		"CASE",
		"platform_credentials.app_bearer_token",
		"platform_credentials.consumer_secret",
	} {
		if !strings.Contains(query, required) {
			t.Fatalf("platform_credentials.sql missing %q", required)
		}
	}
}
