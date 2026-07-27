package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/railwaybackup"
)

func mapEnvironment(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestHandleMigrationCommandBuildsEnvironmentScopedGate(t *testing.T) {
	environment := map[string]string{
		"DATABASE_URL":                        "postgresql://isolated/test",
		"RAILWAY_MIGRATION_BACKUP_TOKEN":      "project-token",
		"RAILWAY_PROJECT_ID":                  "project-1",
		"RAILWAY_ENVIRONMENT_ID":              "environment-1",
		"RAILWAY_POSTGRES_VOLUME_INSTANCE_ID": "volume-instance-1",
		"RAILWAY_POSTGRES_SERVICE_ID":         "postgres-service-1",
		"RAILWAY_SERVICE_ID":                  "api-service-1",
		"RAILWAY_GIT_COMMIT_SHA":              "9eefc1090cfb25b6f23b753603506e5c1c7dc1bc",
	}
	var gotToken, gotDatabaseURL string
	var gotConfig db.MigrationGateConfig
	client := &commandBackupClient{}
	handled, err := handleMigrationCommand(
		context.Background(),
		[]string{"api", "migrate"},
		mapEnvironment(environment),
		func(token string) railwaybackup.Client { gotToken = token; return client },
		func(_ context.Context, databaseURL string, config db.MigrationGateConfig, gotClient railwaybackup.Client) error {
			gotDatabaseURL = databaseURL
			gotConfig = config
			if gotClient != client {
				t.Fatal("migration runner received the wrong backup client")
			}
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("migrate command was not handled")
	}
	if gotDatabaseURL != environment["DATABASE_URL"] || gotToken != environment["RAILWAY_MIGRATION_BACKUP_TOKEN"] {
		t.Fatalf("database=%q token=%q", gotDatabaseURL, gotToken)
	}
	if gotConfig.ProjectID != "project-1" || gotConfig.EnvironmentID != "environment-1" || gotConfig.VolumeInstanceID != "volume-instance-1" || gotConfig.PostgresServiceID != "postgres-service-1" || gotConfig.ApplicationServiceID != "api-service-1" || gotConfig.ApplicationSHA != environment["RAILWAY_GIT_COMMIT_SHA"] {
		t.Fatalf("gate config = %#v", gotConfig)
	}
}

func TestHandleMigrationCommandAllowsServeModeWithoutRunningMigration(t *testing.T) {
	runnerCalled := false
	handled, err := handleMigrationCommand(
		context.Background(),
		[]string{"api"},
		mapEnvironment(nil),
		func(string) railwaybackup.Client { return &commandBackupClient{} },
		func(context.Context, string, db.MigrationGateConfig, railwaybackup.Client) error {
			runnerCalled = true
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if handled || runnerCalled {
		t.Fatalf("handled=%v runnerCalled=%v", handled, runnerCalled)
	}
}

func TestHandleMigrationCommandRejectsMissingDatabaseAndUnknownCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantError string
	}{
		{name: "missing database", args: []string{"api", "migrate"}, wantError: "DATABASE_URL"},
		{name: "unknown command", args: []string{"api", "other"}, wantError: "unknown command"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handled, err := handleMigrationCommand(
				context.Background(), test.args, mapEnvironment(nil),
				func(string) railwaybackup.Client { return &commandBackupClient{} },
				func(context.Context, string, db.MigrationGateConfig, railwaybackup.Client) error { return nil },
			)
			if !handled || err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("handled=%v error=%v, want %q", handled, err, test.wantError)
			}
		})
	}
}

func TestHandleMigrationCommandDefersBackupConfigurationUntilAffectedRowsExist(t *testing.T) {
	var gotClient railwaybackup.Client
	handled, err := handleMigrationCommand(
		context.Background(),
		[]string{"api", "migrate"},
		mapEnvironment(map[string]string{"DATABASE_URL": "postgresql://isolated/test"}),
		func(string) railwaybackup.Client {
			t.Fatal("empty token must not construct a Railway client")
			return nil
		},
		func(_ context.Context, _ string, _ db.MigrationGateConfig, client railwaybackup.Client) error {
			gotClient = client
			return nil
		},
	)
	if err != nil || !handled {
		t.Fatalf("handled=%v error=%v", handled, err)
	}
	if gotClient != nil {
		t.Fatalf("backup client = %#v, want nil", gotClient)
	}
}

func TestRailwayConfigRunsMigrationsAsPreDeploy(t *testing.T) {
	body, err := os.ReadFile("../../railway.toml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, `preDeployCommand = ["./bin/api migrate"]`) {
		t.Fatalf("railway.toml missing migration pre-deploy command:\n%s", text)
	}
	if !strings.Contains(text, `startCommand = "./bin/api"`) {
		t.Fatalf("railway.toml missing normal start command:\n%s", text)
	}
}

func TestMainUsesReadOnlySchemaGuardInsteadOfStartupMigration(t *testing.T) {
	body, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "handleMigrationCommand(") {
		t.Fatal("main must route the migrate subcommand before application initialization")
	}
	if !strings.Contains(text, "db.RequireCurrentSchema(ctx, databaseURL)") {
		t.Fatal("serve mode must require the current schema")
	}
	if strings.Contains(text, "db.RunMigrations(databaseURL)") {
		t.Fatal("serve mode must not run migrations during startup")
	}
}

type commandBackupClient struct{}

func (*commandBackupClient) Identity(context.Context) (railwaybackup.Identity, error) {
	return railwaybackup.Identity{}, nil
}
func (*commandBackupClient) VolumeInstanceIdentity(context.Context, string) (railwaybackup.VolumeInstanceIdentity, error) {
	return railwaybackup.VolumeInstanceIdentity{}, nil
}
func (*commandBackupClient) VerifyDatabaseBinding(context.Context, railwaybackup.DatabaseBindingRequest) error {
	return nil
}
func (*commandBackupClient) List(context.Context, string) ([]railwaybackup.Backup, error) {
	return nil, nil
}
func (*commandBackupClient) Create(context.Context, string, string) (railwaybackup.CreateResult, error) {
	return railwaybackup.CreateResult{}, nil
}
func (*commandBackupClient) Lock(context.Context, string, string) error { return nil }
