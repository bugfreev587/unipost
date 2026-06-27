package db

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedMigrationVersionsAreUnique(t *testing.T) {
	seen := map[string]string{}

	err := fs.WalkDir(migrations, "migrations", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".sql" {
			return nil
		}
		name := filepath.Base(path)
		version, _, ok := strings.Cut(name, "_")
		if !ok {
			t.Fatalf("migration %s does not start with a numeric version prefix", name)
		}
		if previous, exists := seen[version]; exists {
			t.Fatalf("duplicate migration version %s: %s and %s", version, previous, name)
		}
		seen[version] = name
		return nil
	})
	if err != nil {
		t.Fatalf("walk migrations: %v", err)
	}
}

func TestTeamPlanUnlimitedPostsMigrationExists(t *testing.T) {
	body, err := fs.ReadFile(migrations, "migrations/088_team_unlimited_posts.sql")
	if err != nil {
		t.Fatalf("read team unlimited posts migration: %v", err)
	}

	sql := strings.ToLower(string(body))
	if !strings.Contains(sql, "update plans") ||
		!strings.Contains(sql, "post_limit = -1") ||
		!strings.Contains(sql, "id = 'team'") {
		t.Fatalf("team unlimited migration should set plans.post_limit to -1 for team, got:\n%s", string(body))
	}
	if !strings.Contains(sql, "post_limit = 25000") {
		t.Fatalf("team unlimited migration should include a down migration restoring 25000, got:\n%s", string(body))
	}
}

func TestPostgresDoBlocksAreGooseStatementBlocks(t *testing.T) {
	err := fs.WalkDir(migrations, "migrations", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".sql" {
			return nil
		}

		body, err := fs.ReadFile(migrations, path)
		if err != nil {
			t.Fatalf("read migration %s: %v", path, err)
		}
		sql := string(body)
		if strings.Contains(sql, "DO $$") &&
			(!strings.Contains(sql, "-- +goose StatementBegin") ||
				!strings.Contains(sql, "-- +goose StatementEnd")) {
			t.Fatalf("%s contains a PostgreSQL DO block and must wrap it with goose StatementBegin/StatementEnd", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk migrations: %v", err)
	}
}
