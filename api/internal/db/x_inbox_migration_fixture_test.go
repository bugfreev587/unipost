package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type xInboxMigrationTestDatabase interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func migrationVersionForTest(path string) (int, error) {
	name := filepath.Base(path)
	if filepath.Ext(name) != ".sql" {
		return 0, fmt.Errorf("migration %q must use the .sql extension", name)
	}
	prefix, _, ok := strings.Cut(name, "_")
	if !ok || prefix == "" {
		return 0, fmt.Errorf("migration %q must start with a numeric version and underscore", name)
	}
	for _, character := range prefix {
		if character < '0' || character > '9' {
			return 0, fmt.Errorf("migration %q has non-numeric version %q", name, prefix)
		}
	}
	version, err := strconv.Atoi(prefix)
	if err != nil || version < 1 {
		return 0, fmt.Errorf("migration %q has invalid version %q", name, prefix)
	}
	return version, nil
}

func collectMigrationPathsForTest(directory string, minimumVersion, maximumVersion int) ([]string, error) {
	if minimumVersion < 1 || maximumVersion < minimumVersion {
		return nil, fmt.Errorf("invalid migration bounds %d through %d", minimumVersion, maximumVersion)
	}
	paths, err := filepath.Glob(filepath.Join(directory, "*.sql"))
	if err != nil {
		return nil, fmt.Errorf("glob migrations: %w", err)
	}
	byVersion := make(map[int]string, len(paths))
	for _, path := range paths {
		version, err := migrationVersionForTest(path)
		if err != nil {
			return nil, err
		}
		if previous, exists := byVersion[version]; exists {
			return nil, fmt.Errorf("duplicate migration version %d: %s and %s", version, previous, path)
		}
		byVersion[version] = path
	}

	selected := make([]string, 0, maximumVersion-minimumVersion+1)
	for version := minimumVersion; version <= maximumVersion; version++ {
		path, exists := byVersion[version]
		if !exists {
			return nil, fmt.Errorf("missing migration version %d in %s", version, directory)
		}
		selected = append(selected, path)
	}
	sort.Slice(selected, func(i, j int) bool {
		left, _ := migrationVersionForTest(selected[i])
		right, _ := migrationVersionForTest(selected[j])
		return left < right
	})
	return selected, nil
}

func bootstrapMigrationBaselineIfEmptyForTest(
	t *testing.T,
	ctx context.Context,
	database xInboxMigrationTestDatabase,
	maximumVersion int,
) {
	t.Helper()
	if publicSchemaTableCountForTest(t, ctx, database) != 0 {
		return
	}
	applyMigrationRangeForTest(t, ctx, database, 1, maximumVersion)
}

func requireEmptyPublicSchemaForTest(
	t *testing.T,
	ctx context.Context,
	database xInboxMigrationTestDatabase,
) {
	t.Helper()
	if count := publicSchemaTableCountForTest(t, ctx, database); count != 0 {
		t.Fatalf("X_INBOX_TEST_DATABASE_URL must point to an empty disposable database; found %d public tables", count)
	}
}

func publicSchemaTableCountForTest(
	t *testing.T,
	ctx context.Context,
	database xInboxMigrationTestDatabase,
) int {
	t.Helper()
	var count int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_type = 'BASE TABLE'
	`).Scan(&count); err != nil {
		t.Fatalf("inspect disposable X Inbox migration database: %v", err)
	}
	return count
}

func applyMigrationRangeForTest(
	t *testing.T,
	ctx context.Context,
	database xInboxMigrationTestDatabase,
	minimumVersion int,
	maximumVersion int,
) {
	t.Helper()
	paths, err := collectMigrationPathsForTest("migrations", minimumVersion, maximumVersion)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		applyMigrationUp(t, ctx, database, path)
	}
}

func applyMigrationUp(
	t *testing.T,
	ctx context.Context,
	database xInboxMigrationTestDatabase,
	path string,
) {
	t.Helper()
	migration, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	parts := strings.Split(string(migration), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatalf("migration %s must have exactly one Goose Down marker", path)
	}
	upSQL := parts[0]
	if !strings.Contains(upSQL, "-- +goose Up") {
		t.Fatalf("migration %s is missing its Goose Up marker", path)
	}
	if strings.Contains(upSQL, "-- +goose NO TRANSACTION") {
		t.Fatalf("migration %s cannot be applied by the transactional fixture helper", path)
	}
	upSQL = strings.Replace(upSQL, "-- +goose Up", "", 1)
	if _, err := database.ExecContext(ctx, upSQL); err != nil {
		t.Fatalf("apply predecessor migration %s: %v", path, err)
	}
}

func TestXInboxMigrationFixtureCollectsOnlyRequestedVersions(t *testing.T) {
	paths, err := collectMigrationPathsForTest("migrations", 1, 115)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 115 {
		t.Fatalf("collected %d predecessor migrations, want 115", len(paths))
	}
	if got := filepath.Base(paths[0]); got != "001_create_users.sql" {
		t.Fatalf("first predecessor migration = %q", got)
	}
	if got := filepath.Base(paths[len(paths)-1]); got != "115_x_inbox_durable_operations.sql" {
		t.Fatalf("last predecessor migration = %q", got)
	}
	for _, path := range paths {
		version, err := migrationVersionForTest(path)
		if err != nil {
			t.Fatal(err)
		}
		if version < 1 || version > 115 {
			t.Fatalf("out-of-range predecessor migration %s has version %d", path, version)
		}
	}
}

func TestXInboxMigrationFixtureRejectsMalformedVersion(t *testing.T) {
	for _, name := range []string{"migration.sql", "x01_bad.sql", "_missing.sql", "120.sql"} {
		if _, err := migrationVersionForTest(name); err == nil {
			t.Fatalf("migrationVersionForTest(%q) unexpectedly succeeded", name)
		}
	}
}
