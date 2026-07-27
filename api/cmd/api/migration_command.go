package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/railwaybackup"
)

type migrationBackupClientFactory func(string) railwaybackup.Client

type gatedMigrationRunner func(
	context.Context,
	string,
	db.MigrationGateConfig,
	railwaybackup.Client,
) error

func handleMigrationCommand(
	ctx context.Context,
	args []string,
	getenv func(string) string,
	newBackupClient migrationBackupClientFactory,
	runMigrations gatedMigrationRunner,
) (bool, error) {
	if len(args) <= 1 {
		return false, nil
	}
	if args[1] != "migrate" {
		return true, fmt.Errorf("unknown command %q", args[1])
	}
	if len(args) != 2 {
		return true, fmt.Errorf("migrate command does not accept arguments")
	}
	databaseURL := strings.TrimSpace(getenv("DATABASE_URL"))
	if databaseURL == "" {
		return true, fmt.Errorf("DATABASE_URL is required for migrate command")
	}

	config := db.MigrationGateConfig{
		ProjectID:            strings.TrimSpace(getenv("RAILWAY_PROJECT_ID")),
		EnvironmentID:        strings.TrimSpace(getenv("RAILWAY_ENVIRONMENT_ID")),
		VolumeInstanceID:     strings.TrimSpace(getenv("RAILWAY_POSTGRES_VOLUME_INSTANCE_ID")),
		PostgresServiceID:    strings.TrimSpace(getenv("RAILWAY_POSTGRES_SERVICE_ID")),
		ApplicationServiceID: strings.TrimSpace(getenv("RAILWAY_SERVICE_ID")),
		ApplicationSHA:       strings.TrimSpace(getenv("RAILWAY_GIT_COMMIT_SHA")),
	}
	var backupClient railwaybackup.Client
	if token := strings.TrimSpace(getenv("RAILWAY_MIGRATION_BACKUP_TOKEN")); token != "" {
		backupClient = newBackupClient(token)
	}
	return true, runMigrations(ctx, databaseURL, config, backupClient)
}
