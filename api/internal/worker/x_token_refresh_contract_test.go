package worker

import (
	"os"
	"strings"
	"testing"
)

func TestXTokenRefreshPathsUsePersistedIdentityResolver(t *testing.T) {
	for _, file := range []string{"token_refresh.go", "managed_token_refresh.go", "analytics_refresh.go"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if !strings.Contains(text, `acc.Platform == "twitter"`) && !strings.Contains(text, `r.Platform == "twitter"`) {
			t.Fatalf("%s has no explicit X refresh route", file)
		}
		if !strings.Contains(text, ".xRefresher.Refresh(") {
			t.Fatalf("%s does not route X refresh through the persisted app identity resolver", file)
		}
	}
}
