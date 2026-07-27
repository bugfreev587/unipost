package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrations embed.FS

func RequireCurrentSchema(ctx context.Context, databaseURL string) error {
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open database for schema check: %w", err)
	}
	defer database.Close()

	currentVersion, err := readCurrentMigrationVersion(ctx, database)
	if err != nil {
		return fmt.Errorf("check current database schema: %w", err)
	}
	requiredVersion, err := latestEmbeddedMigrationVersion()
	if err != nil {
		return fmt.Errorf("read required database schema: %w", err)
	}
	if currentVersion < requiredVersion {
		return fmt.Errorf("database schema current version %d is behind required version %d; run pre-deploy migrations", currentVersion, requiredVersion)
	}
	if currentVersion > requiredVersion {
		return fmt.Errorf("database schema current version %d is newer than binary required version %d; rollback is unsafe", currentVersion, requiredVersion)
	}
	return nil
}

func latestEmbeddedMigrationVersion() (int64, error) {
	var latest int64
	err := fs.WalkDir(migrations, "migrations", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".sql" {
			return nil
		}
		prefix, _, ok := strings.Cut(filepath.Base(path), "_")
		if !ok {
			return fmt.Errorf("migration %q does not start with a version", path)
		}
		version, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil {
			return fmt.Errorf("parse migration version from %q: %w", path, err)
		}
		if version > latest {
			latest = version
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if latest == 0 {
		return 0, fmt.Errorf("no embedded migrations found")
	}
	return latest, nil
}

// RunMigrations runs all pending database migrations using goose.
func RunMigrations(databaseURL string) error {
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database for migrations: %w", err)
	}
	defer database.Close()
	return runMigrations(context.Background(), database, true)
}

func runMigrations(ctx context.Context, database *sql.DB, acquireSessionLock bool) error {
	migrationFS, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("failed to open embedded migrations: %w", err)
	}
	providerOptions := make([]goose.ProviderOption, 0, 1)
	if acquireSessionLock {
		sessionLocker, err := lock.NewPostgresSessionLocker()
		if err != nil {
			return fmt.Errorf("failed to create migration session locker: %w", err)
		}
		providerOptions = append(providerOptions, goose.WithSessionLocker(sessionLocker))
	}
	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		database,
		migrationFS,
		providerOptions...,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	slog.Info("database migrations completed")
	return nil
}
