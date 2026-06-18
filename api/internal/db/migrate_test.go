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
