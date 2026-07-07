package db

import (
	"os"
	"strings"
	"testing"
)

func TestPublishTokenMigrationContract(t *testing.T) {
	source, err := os.ReadFile("migrations/101_social_post_result_publish_token.sql")
	if err != nil {
		t.Fatalf("read publish token migration: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		"ALTER TABLE social_post_results ADD COLUMN publish_token TEXT",
		"DROP COLUMN IF EXISTS publish_token",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("publish token migration missing %q:\n%s", want, sql)
		}
	}
}

func TestPublishTokenSetterExists(t *testing.T) {
	source, err := os.ReadFile("social_post_results.sql.go")
	if err != nil {
		t.Fatalf("read generated social_post_results queries: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		"SetSocialPostResultPublishToken",
		"SET publish_token =",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("publish token setter query missing %q", want)
		}
	}
}
