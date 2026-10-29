package db

import (
	"os"
	"strings"
	"testing"
)

func TestPlatformOperationalControlsMigrationContract(t *testing.T) {
	body, err := os.ReadFile("migrations/138_platform_operational_controls.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(body))
	required := []string{
		"create table platform_operational_controls",
		"create table platform_operational_control_events",
		"'active', 'publishing_disabled', 'fully_disabled'",
		"expected_version",
		"resulting_version",
		"before_state",
		"after_state",
		"on conflict (platform) do nothing",
		"-- +goose down",
	}
	for _, fragment := range required {
		if !strings.Contains(sql, fragment) {
			t.Errorf("migration missing %q", fragment)
		}
	}
	for _, platform := range []string{"twitter", "linkedin", "bluesky", "youtube", "tiktok", "instagram", "threads", "facebook", "pinterest"} {
		if !strings.Contains(sql, "('"+platform+"'") {
			t.Errorf("migration missing %s seed", platform)
		}
	}
}
