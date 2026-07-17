package handler

import (
	"os"
	"strings"
	"testing"
)

func TestXInlineTokenRefreshPathsUsePersistedIdentityResolver(t *testing.T) {
	for _, file := range []string{"social_posts.go", "social_account_metrics.go"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if !strings.Contains(text, "xTokenRefresher") || !strings.Contains(text, ".Refresh(") {
			t.Fatalf("%s does not route X refresh through the persisted app identity resolver", file)
		}
	}
}
