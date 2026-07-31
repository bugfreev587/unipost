package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/railwaybackup"
	"github.com/xiaoboyu/unipost-api/internal/socialconnectioncutover"
)

type socialConnectionCommandOptions struct {
	Command      string
	Mode         string
	JSON         bool
	DrainTimeout time.Duration
}

func parseSocialConnectionCommand(args []string) (socialConnectionCommandOptions, bool, error) {
	if len(args) <= 1 {
		return socialConnectionCommandOptions{}, false, nil
	}
	options := socialConnectionCommandOptions{DrainTimeout: 5 * time.Minute}
	switch args[1] {
	case "social-connections-preflight":
		options.Command = "preflight"
		options.Mode = "expand"
	case "social-connections-cutover":
		options.Command = "cutover"
		options.Mode = "cutover"
	default:
		return socialConnectionCommandOptions{}, false, nil
	}
	for _, argument := range args[2:] {
		switch {
		case argument == "--json":
			options.JSON = true
		case options.Command == "preflight" && strings.HasPrefix(argument, "--mode="):
			options.Mode = strings.TrimPrefix(argument, "--mode=")
			if options.Mode != "expand" && options.Mode != "cutover" {
				return options, true, fmt.Errorf("unsupported preflight mode %q", options.Mode)
			}
		case options.Command == "cutover" && strings.HasPrefix(argument, "--drain-timeout="):
			duration, err := time.ParseDuration(strings.TrimPrefix(argument, "--drain-timeout="))
			if err != nil || duration <= 0 {
				return options, true, fmt.Errorf("drain timeout must be a positive duration")
			}
			options.DrainTimeout = duration
		default:
			return options, true, fmt.Errorf("unknown %s argument %q", options.Command, argument)
		}
	}
	return options, true, nil
}

func handleSocialConnectionCommand(
	ctx context.Context,
	args []string,
	getenv func(string) string,
	output io.Writer,
) (bool, error) {
	options, handled, err := parseSocialConnectionCommand(args)
	if err != nil || !handled {
		return handled, err
	}
	databaseURL := strings.TrimSpace(getenv("DATABASE_URL"))
	if databaseURL == "" {
		return true, errors.New("DATABASE_URL is required for social connection commands")
	}
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return true, fmt.Errorf("parse cutover database config: %w", err)
	}
	serviceID := strings.TrimSpace(getenv("RAILWAY_SERVICE_ID"))
	deploymentID := strings.TrimSpace(getenv("RAILWAY_DEPLOYMENT_ID"))
	sha := strings.TrimSpace(getenv("RAILWAY_GIT_COMMIT_SHA"))
	applicationName, err := databaseApplicationName("social-connection-cutover", serviceID, deploymentID, sha)
	if err != nil {
		return true, err
	}
	poolConfig.ConnConfig.RuntimeParams["application_name"] = applicationName
	poolConfig.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return true, fmt.Errorf("open social connection command database: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return true, fmt.Errorf("ping social connection command database: %w", err)
	}

	preflight := socialconnectioncutover.NewPreflight(
		socialconnectioncutover.NewPostgresPreflightReader(pool),
		time.Now,
	)
	if options.Command == "preflight" {
		report, err := preflight.Run(ctx, options.Mode)
		if err != nil {
			return true, err
		}
		return true, writeCutoverReport(output, report, options.JSON)
	}

	projectID := strings.TrimSpace(getenv("RAILWAY_PROJECT_ID"))
	environmentID := strings.TrimSpace(getenv("RAILWAY_ENVIRONMENT_ID"))
	token := strings.TrimSpace(getenv("RAILWAY_MIGRATION_BACKUP_TOKEN"))
	volumeID := strings.TrimSpace(getenv("RAILWAY_POSTGRES_VOLUME_INSTANCE_ID"))
	postgresServiceID := strings.TrimSpace(getenv("RAILWAY_POSTGRES_SERVICE_ID"))
	serviceIDs := cutoverRuntimeServiceIDs(getenv)
	for name, value := range map[string]string{
		"RAILWAY_PROJECT_ID": projectID, "RAILWAY_ENVIRONMENT_ID": environmentID,
		"RAILWAY_SERVICE_ID": serviceID, "RAILWAY_DEPLOYMENT_ID": deploymentID,
		"RAILWAY_GIT_COMMIT_SHA": sha, "RAILWAY_MIGRATION_BACKUP_TOKEN": token,
		"RAILWAY_POSTGRES_VOLUME_INSTANCE_ID": volumeID,
		"RAILWAY_POSTGRES_SERVICE_ID":         postgresServiceID,
	} {
		if value == "" {
			return true, fmt.Errorf("%s is required for social connection cutover", name)
		}
	}
	if len(serviceIDs) == 0 {
		return true, errors.New("cutover runtime service IDs are required")
	}
	if !socialconnectioncutover.ValidCommitSHA(sha) {
		return true, errors.New("RAILWAY_GIT_COMMIT_SHA must be an exact lowercase 40-character SHA")
	}

	railwayClient := railwaybackup.New(token)
	proof := socialconnectioncutover.DeploymentProof{
		Inventory: railwayClient,
		Sessions:  socialconnectioncutover.NewPostgresRuntimeSessionReader(pool),
		ProjectID: projectID, EnvironmentID: environmentID, CommitSHA: sha,
		RequiredServiceIDs: serviceIDs,
	}
	reconciler := &socialconnectioncutover.Reconciler{Pool: pool, CommitSHA: sha, EnvironmentID: environmentID}
	backupConfig := db.MigrationGateConfig{
		DatabaseURL: databaseURL, ProjectID: projectID, EnvironmentID: environmentID,
		VolumeInstanceID: volumeID, PostgresServiceID: postgresServiceID,
		ApplicationServiceID: serviceID, ApplicationSHA: sha,
	}
	orchestrator := socialconnectioncutover.Orchestrator{
		Store:        socialconnectioncutover.NewPostgresRolloutStore(pool),
		Proof:        proof,
		Preflight:    preflight,
		DrainTimeout: options.DrainTimeout,
		PollInterval: time.Second,
		Backup: func(backupCtx context.Context, report socialconnectioncutover.Report, operation func(context.Context) error) error {
			affectedRows := report.Counts.Accounts + report.Counts.ActiveDeliveryJobs
			return db.RunRegisteredIrreversibleOperationWithBackupGate(
				backupCtx, db.SocialConnectionsCutoverOperation, backupConfig, railwayClient,
				[]db.AffectedMigration{{Version: 136, Rows: affectedRows}}, operation,
			)
		},
		Reconcile: reconciler.Run,
		Recorder:  socialconnectioncutover.NewPostgresCutoverRunRecorder(pool, sha, environmentID),
	}
	report, err := orchestrator.Run(ctx)
	if err != nil {
		return true, err
	}
	return true, writeCutoverReport(output, report, options.JSON)
}

func cutoverRuntimeServiceIDs(getenv func(string) string) []string {
	values := strings.Split(getenv("UNIPOST_CUTOVER_RUNTIME_SERVICE_IDS"), ",")
	values = append(values, getenv("RAILWAY_API_SERVICE_ID"), getenv("RAILWAY_POST_DELIVERY_WORKER_SERVICE_ID"))
	seen := make(map[string]bool)
	services := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			services = append(services, value)
		}
	}
	sort.Strings(services)
	return services
}

func writeCutoverReport(output io.Writer, report socialconnectioncutover.Report, compact bool) error {
	var body []byte
	var err error
	if compact {
		body, err = report.MarshalJSON()
	} else {
		body, err = json.MarshalIndent(report, "", "  ")
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, string(body))
	return err
}
