package main

import (
	"context"
	"fmt"
	"strings"
	"time"

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

type partitionReadinessRunner func(context.Context, string) error

func handleMigrationCommand(
	ctx context.Context,
	args []string,
	getenv func(string) string,
	newBackupClient migrationBackupClientFactory,
	runMigrations gatedMigrationRunner,
	ensurePartitions partitionReadinessRunner,
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
		EnvironmentName:      strings.TrimSpace(getenv("RAILWAY_ENVIRONMENT_NAME")),
		ServicePublicDomain:  strings.TrimSpace(getenv("RAILWAY_PUBLIC_DOMAIN")),
		VolumeInstanceID:     strings.TrimSpace(getenv("RAILWAY_POSTGRES_VOLUME_INSTANCE_ID")),
		PostgresServiceID:    strings.TrimSpace(getenv("RAILWAY_POSTGRES_SERVICE_ID")),
		ApplicationServiceID: strings.TrimSpace(getenv("RAILWAY_SERVICE_ID")),
		ApplicationSHA:       strings.TrimSpace(getenv("RAILWAY_GIT_COMMIT_SHA")),
	}
	var backupClient railwaybackup.Client
	if token := strings.TrimSpace(getenv("RAILWAY_MIGRATION_BACKUP_TOKEN")); token != "" {
		backupClient = newBackupClient(token)
	}
	if err := runMigrations(ctx, databaseURL, config, backupClient); err != nil {
		return true, err
	}
	partitionCtx, partitionCancel := context.WithTimeout(ctx, 45*time.Second)
	defer partitionCancel()
	if err := ensurePartitions(partitionCtx, databaseURL); err != nil {
		return true, fmt.Errorf("ensure request-event partition readiness: %w", err)
	}
	return true, nil
}
