package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/railwaybackup"
)

type recordingBackupClient struct {
	identity    railwaybackup.Identity
	volume      railwaybackup.VolumeInstanceIdentity
	lists       [][]railwaybackup.Backup
	create      railwaybackup.CreateResult
	identityErr error
	volumeErr   error
	bindingErr  error
	listErr     error
	createErr   error
	lockErr     error
	calls       []string
	listIndex   int
	lockedID    string
	createdName string
	binding     railwaybackup.DatabaseBindingRequest
}

func (c *recordingBackupClient) Identity(context.Context) (railwaybackup.Identity, error) {
	c.calls = append(c.calls, "identity")
	return c.identity, c.identityErr
}

func (c *recordingBackupClient) VolumeInstanceIdentity(_ context.Context, volumeInstanceID string) (railwaybackup.VolumeInstanceIdentity, error) {
	c.calls = append(c.calls, "volume:"+volumeInstanceID)
	return c.volume, c.volumeErr
}

func (c *recordingBackupClient) VerifyDatabaseBinding(_ context.Context, binding railwaybackup.DatabaseBindingRequest) error {
	c.calls = append(c.calls, "binding")
	c.binding = binding
	return c.bindingErr
}

func (c *recordingBackupClient) List(_ context.Context, volumeInstanceID string) ([]railwaybackup.Backup, error) {
	c.calls = append(c.calls, "list:"+volumeInstanceID)
	if c.listErr != nil {
		return nil, c.listErr
	}
	if len(c.lists) == 0 {
		return nil, nil
	}
	index := c.listIndex
	if index >= len(c.lists) {
		index = len(c.lists) - 1
	}
	c.listIndex++
	return c.lists[index], nil
}

func (c *recordingBackupClient) Create(_ context.Context, volumeInstanceID, name string) (railwaybackup.CreateResult, error) {
	c.calls = append(c.calls, "create:"+volumeInstanceID)
	c.createdName = name
	return c.create, c.createErr
}

func (c *recordingBackupClient) Lock(_ context.Context, volumeInstanceID, backupID string) error {
	c.calls = append(c.calls, "lock:"+volumeInstanceID)
	c.lockedID = backupID
	return c.lockErr
}

func int64Pointer(value int64) *int64 { return &value }

func readyMigrationBackup(id, name string) railwaybackup.Backup {
	return railwaybackup.Backup{
		ID:           id,
		Name:         name,
		CreatedAt:    "2026-07-26T16:20:53.874Z",
		ExternalID:   "vs_123",
		ReferencedMB: int64Pointer(859),
	}
}

func testMigrationGateConfig() MigrationGateConfig {
	return MigrationGateConfig{
		ProjectID:            "project-1",
		EnvironmentID:        "environment-1",
		VolumeInstanceID:     "volume-instance-1",
		PostgresServiceID:    "postgres-service-1",
		ApplicationServiceID: "api-service-1",
		ApplicationSHA:       "9eefc1090cfb25b6f23b753603506e5c1c7dc1bc",
		PollInterval:         time.Millisecond,
		Timeout:              50 * time.Millisecond,
		attemptSuffix:        func() string { return "attempt1" },
		databaseURL:          "postgresql://runtime/test",
	}
}

func TestDisposablePreviewIdentityAcceptsSupportedRailwayEnvironmentNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		environmentName string
		serviceDomain   string
		want            bool
	}{
		{
			name:            "legacy PR environment",
			environmentName: "unipost-pr-301",
			serviceDomain:   "preview-api-unipost-pr-301.up.railway.app",
			want:            true,
		},
		{
			name:            "hash scoped PR environment",
			environmentName: "pr-5ba7dd-299",
			serviceDomain:   "preview-api-pr-5ba7dd-299.up.railway.app",
			want:            true,
		},
		{
			name:            "hash scoped domain mismatch",
			environmentName: "pr-5ba7dd-299",
			serviceDomain:   "preview-api-pr-ffffff-299.up.railway.app",
			want:            false,
		},
		{
			name:            "persistent environment",
			environmentName: "development",
			serviceDomain:   "dev-api.unipost.dev",
			want:            false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := isDisposablePreviewIdentity(test.environmentName, test.serviceDomain); got != test.want {
				t.Fatalf("isDisposablePreviewIdentity(%q, %q) = %t, want %t", test.environmentName, test.serviceDomain, got, test.want)
			}
		})
	}
}

func TestMigrationGateRejectsMiswiredDatabaseBeforeBackupListOrMigrations(t *testing.T) {
	config := testMigrationGateConfig()
	client := &recordingBackupClient{
		identity:   railwaybackup.Identity{ProjectID: config.ProjectID, EnvironmentID: config.EnvironmentID},
		volume:     trustedVolumeIdentity(config),
		bindingErr: errors.New("rendered DATABASE_URL does not match runtime DATABASE_URL"),
	}
	migrationsCalled := false
	err := runAfterBackupGate(
		context.Background(), config, client,
		[]AffectedMigration{{Version: 125, Rows: 0}},
		func(context.Context) error { migrationsCalled = true; return nil },
	)
	if err == nil || !strings.Contains(err.Error(), "rendered DATABASE_URL does not match runtime DATABASE_URL") {
		t.Fatalf("gate error = %v", err)
	}
	if migrationsCalled {
		t.Fatal("migration runner was called")
	}
	wantCalls := []string{"identity", "volume:volume-instance-1", "binding"}
	if !reflect.DeepEqual(client.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", client.calls, wantCalls)
	}
	if client.binding.ProjectID != config.ProjectID ||
		client.binding.EnvironmentID != config.EnvironmentID ||
		client.binding.ApplicationServiceID != config.ApplicationServiceID ||
		client.binding.PostgresServiceID != config.PostgresServiceID ||
		client.binding.RuntimeDatabaseURL != config.databaseURL {
		t.Fatalf("binding request = %#v", client.binding)
	}
}

func trustedVolumeIdentity(config MigrationGateConfig) railwaybackup.VolumeInstanceIdentity {
	return railwaybackup.VolumeInstanceIdentity{
		ID:            config.VolumeInstanceID,
		ProjectID:     config.ProjectID,
		EnvironmentID: config.EnvironmentID,
		ServiceID:     config.PostgresServiceID,
	}
}

func TestMigrationGateRequiresBackupWhenPendingIrreversibleRowsAreZero(t *testing.T) {
	config := testMigrationGateConfig()
	wantName := migrationBackupName(config, []AffectedMigration{{Version: 124, Rows: 0}, {Version: 125, Rows: 0}})
	if !strings.Contains(wantName, "-m124-125-") {
		t.Fatalf("zero-row pending migration backup name = %q, want both pending versions", wantName)
	}
	fresh := readyMigrationBackup("zero-row-backup", wantName)
	client := &recordingBackupClient{
		identity: railwaybackup.Identity{ProjectID: config.ProjectID, EnvironmentID: config.EnvironmentID},
		volume:   trustedVolumeIdentity(config),
		create:   railwaybackup.CreateResult{WorkflowID: "zero-row-workflow"},
		lists:    [][]railwaybackup.Backup{{}, {fresh}, {fresh}, {fresh}},
	}
	migrationsCalled := false
	err := runAfterBackupGate(
		context.Background(),
		config,
		client,
		[]AffectedMigration{{Version: 124, Rows: 0}, {Version: 125, Rows: 0}},
		func(context.Context) error { migrationsCalled = true; return nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if client.createdName != wantName || client.lockedID != "zero-row-backup" {
		t.Fatalf("created=%q locked=%q, want %q/zero-row-backup", client.createdName, client.lockedID, wantName)
	}
	if !migrationsCalled {
		t.Fatal("migration runner was not called")
	}
}

func TestMigrationGateVerifiesFreshStableLockedBackupBeforeMigrations(t *testing.T) {
	config := testMigrationGateConfig()
	wantName := migrationBackupName(config, []AffectedMigration{{Version: 125, Rows: 2}})
	old := readyMigrationBackup("old-backup", "scheduled")
	fresh := readyMigrationBackup("new-backup", wantName)
	client := &recordingBackupClient{
		identity: railwaybackup.Identity{ProjectID: config.ProjectID, EnvironmentID: config.EnvironmentID},
		volume: railwaybackup.VolumeInstanceIdentity{
			ID: config.VolumeInstanceID, ProjectID: config.ProjectID,
			EnvironmentID: config.EnvironmentID, ServiceID: config.PostgresServiceID,
		},
		create: railwaybackup.CreateResult{WorkflowID: "createVolumeInstanceBackup/workflow"},
		lists: [][]railwaybackup.Backup{
			{old},
			{old, fresh},
			{old, fresh},
			{old, fresh},
		},
	}
	migrationsCalled := false
	err := runAfterBackupGate(
		context.Background(), config, client,
		[]AffectedMigration{{Version: 125, Rows: 2}},
		func(context.Context) error {
			client.calls = append(client.calls, "migrate")
			migrationsCalled = true
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !migrationsCalled {
		t.Fatal("migration runner was not called")
	}
	if client.lockedID != "new-backup" {
		t.Fatalf("locked backup ID = %q", client.lockedID)
	}
	if client.createdName != wantName || len(client.createdName) > 64 {
		t.Fatalf("created backup name = %q (length %d)", client.createdName, len(client.createdName))
	}
	wantCalls := []string{
		"identity",
		"volume:volume-instance-1",
		"binding",
		"list:volume-instance-1",
		"create:volume-instance-1",
		"list:volume-instance-1",
		"list:volume-instance-1",
		"lock:volume-instance-1",
		"list:volume-instance-1",
		"migrate",
	}
	if !reflect.DeepEqual(client.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", client.calls, wantCalls)
	}
}

func TestMigrationGateRejectsWorkflowIDAsBackupID(t *testing.T) {
	config := testMigrationGateConfig()
	client := &recordingBackupClient{
		identity: railwaybackup.Identity{ProjectID: config.ProjectID, EnvironmentID: config.EnvironmentID},
		volume: railwaybackup.VolumeInstanceIdentity{
			ID: config.VolumeInstanceID, ProjectID: config.ProjectID,
			EnvironmentID: config.EnvironmentID, ServiceID: config.PostgresServiceID,
		},
		create: railwaybackup.CreateResult{WorkflowID: "workflow-is-not-a-backup-id"},
		lists:  [][]railwaybackup.Backup{{}, {}, {}, {}},
	}
	migrationsCalled := false
	err := runAfterBackupGate(
		context.Background(), config, client,
		[]AffectedMigration{{Version: 125, Rows: 1}},
		func(context.Context) error { migrationsCalled = true; return nil },
	)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("gate error = %v", err)
	}
	if client.lockedID != "" || migrationsCalled {
		t.Fatalf("locked=%q migrationsCalled=%v", client.lockedID, migrationsCalled)
	}
}

func TestMigrationGateRejectsUntrustedVolumeBeforeListingBackupsOrMigrations(t *testing.T) {
	tests := []struct {
		name      string
		configure func(MigrationGateConfig, *recordingBackupClient)
		wantError string
	}{
		{
			name: "wrong volume instance",
			configure: func(_ MigrationGateConfig, client *recordingBackupClient) {
				client.volume.ID = "other-volume-instance"
			},
			wantError: "volume instance identity mismatch",
		},
		{
			name: "wrong project",
			configure: func(_ MigrationGateConfig, client *recordingBackupClient) {
				client.volume.ProjectID = "other-project"
			},
			wantError: "volume instance identity mismatch",
		},
		{
			name: "wrong environment",
			configure: func(_ MigrationGateConfig, client *recordingBackupClient) {
				client.volume.EnvironmentID = "other-environment"
			},
			wantError: "volume instance identity mismatch",
		},
		{
			name: "same environment but wrong service",
			configure: func(_ MigrationGateConfig, client *recordingBackupClient) {
				client.volume.ServiceID = "api-service-not-postgres"
			},
			wantError: "volume instance identity mismatch",
		},
		{
			name: "lookup error",
			configure: func(_ MigrationGateConfig, client *recordingBackupClient) {
				client.volumeErr = errors.New("volume lookup unavailable")
			},
			wantError: "volume lookup unavailable",
		},
		{
			name: "missing fields",
			configure: func(_ MigrationGateConfig, client *recordingBackupClient) {
				client.volume.ServiceID = ""
			},
			wantError: "missing",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := testMigrationGateConfig()
			client := &recordingBackupClient{
				identity: railwaybackup.Identity{ProjectID: config.ProjectID, EnvironmentID: config.EnvironmentID},
				volume: railwaybackup.VolumeInstanceIdentity{
					ID: config.VolumeInstanceID, ProjectID: config.ProjectID,
					EnvironmentID: config.EnvironmentID, ServiceID: config.PostgresServiceID,
				},
			}
			test.configure(config, client)
			migrationsCalled := false
			err := runAfterBackupGate(
				context.Background(), config, client,
				[]AffectedMigration{{Version: 125, Rows: 1}},
				func(context.Context) error { migrationsCalled = true; return nil },
			)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("gate error = %v, want containing %q", err, test.wantError)
			}
			if migrationsCalled {
				t.Fatal("migration runner was called")
			}
			wantCalls := []string{"identity", "volume:volume-instance-1"}
			if !reflect.DeepEqual(client.calls, wantCalls) {
				t.Fatalf("calls = %#v, want %#v", client.calls, wantCalls)
			}
		})
	}
}

func TestMigrationGateRequiresTrustedPostgresServiceIDBeforeRailwayLookup(t *testing.T) {
	config := testMigrationGateConfig()
	config.PostgresServiceID = ""
	client := &recordingBackupClient{}
	migrationsCalled := false
	err := runAfterBackupGate(
		context.Background(), config, client,
		[]AffectedMigration{{Version: 125, Rows: 1}},
		func(context.Context) error { migrationsCalled = true; return nil },
	)
	if err == nil || !strings.Contains(err.Error(), "Postgres service ID") {
		t.Fatalf("gate error = %v", err)
	}
	if len(client.calls) != 0 || migrationsCalled {
		t.Fatalf("Railway calls = %v migrationsCalled=%v", client.calls, migrationsCalled)
	}
}

func TestMigrationGateFailureIncludesEnvironmentAndAffectedRows(t *testing.T) {
	config := testMigrationGateConfig()
	client := &recordingBackupClient{identityErr: errors.New("identity unavailable")}
	affected := []AffectedMigration{{Version: 124, Rows: 7}, {Version: 125, Rows: 3}}
	err := runAfterBackupGate(
		context.Background(), config, client, affected,
		func(context.Context) error { t.Fatal("migration runner must not be called"); return nil },
	)
	if err == nil {
		t.Fatal("gate error is nil")
	}
	message := err.Error()
	for _, want := range []string{
		"environment-1",
		"version=124 rows=7",
		"version=125 rows=3",
		"identity unavailable",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("gate error %q missing %q", message, want)
		}
	}
}

func TestMigrationGateRejectsInvalidBackupEvidence(t *testing.T) {
	tests := []struct {
		name       string
		configure  func(MigrationGateConfig, *recordingBackupClient, string)
		wantError  string
		wantLocked bool
	}{
		{
			name: "wrong token identity",
			configure: func(_ MigrationGateConfig, client *recordingBackupClient, _ string) {
				client.identity.EnvironmentID = "other-environment"
			},
			wantError: "identity mismatch",
		},
		{
			name: "duplicate exact name",
			configure: func(_ MigrationGateConfig, client *recordingBackupClient, backupName string) {
				client.lists = [][]railwaybackup.Backup{{}, {
					readyMigrationBackup("new-1", backupName),
					readyMigrationBackup("new-2", backupName),
				}}
			},
			wantError: "ambiguous",
		},
		{
			name: "old matching backup",
			configure: func(_ MigrationGateConfig, client *recordingBackupClient, backupName string) {
				old := readyMigrationBackup("old", backupName)
				client.lists = [][]railwaybackup.Backup{{old}, {old}}
			},
			wantError: "timed out",
		},
		{
			name: "missing readiness field",
			configure: func(_ MigrationGateConfig, client *recordingBackupClient, backupName string) {
				missing := readyMigrationBackup("new", backupName)
				missing.ExternalID = ""
				client.lists = [][]railwaybackup.Backup{{}, {missing}}
			},
			wantError: "timed out",
		},
		{
			name: "unstable readiness metadata",
			configure: func(_ MigrationGateConfig, client *recordingBackupClient, backupName string) {
				first := readyMigrationBackup("new", backupName)
				second := readyMigrationBackup("new", backupName)
				second.ExternalID = "vs_changed"
				client.lists = [][]railwaybackup.Backup{{}, {first}, {second}}
			},
			wantError: "unstable",
		},
		{
			name: "lock failure",
			configure: func(_ MigrationGateConfig, client *recordingBackupClient, backupName string) {
				fresh := readyMigrationBackup("new", backupName)
				client.lists = [][]railwaybackup.Backup{{}, {fresh}, {fresh}}
				client.lockErr = errors.New("lock refused")
			},
			wantError:  "lock refused",
			wantLocked: true,
		},
		{
			name: "post-lock record disappears",
			configure: func(_ MigrationGateConfig, client *recordingBackupClient, backupName string) {
				fresh := readyMigrationBackup("new", backupName)
				client.lists = [][]railwaybackup.Backup{{}, {fresh}, {fresh}, {}}
			},
			wantError:  "post-lock",
			wantLocked: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := testMigrationGateConfig()
			backupName := migrationBackupName(config, []AffectedMigration{{Version: 125, Rows: 1}})
			client := &recordingBackupClient{
				identity: railwaybackup.Identity{ProjectID: config.ProjectID, EnvironmentID: config.EnvironmentID},
				volume:   trustedVolumeIdentity(config),
				create:   railwaybackup.CreateResult{WorkflowID: "workflow"},
			}
			test.configure(config, client, backupName)
			migrationsCalled := false
			err := runAfterBackupGate(
				context.Background(), config, client,
				[]AffectedMigration{{Version: 125, Rows: 1}},
				func(context.Context) error { migrationsCalled = true; return nil },
			)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("gate error = %v, want containing %q", err, test.wantError)
			}
			if migrationsCalled {
				t.Fatal("migration runner was called")
			}
			if test.wantLocked != (client.lockedID != "") {
				t.Fatalf("locked ID = %q", client.lockedID)
			}
		})
	}
}

func TestMigrationGateNeverReusesOrphanFromPriorAttempt(t *testing.T) {
	first := testMigrationGateConfig()
	first.attemptSuffix = func() string { return "orphan1" }
	second := testMigrationGateConfig()
	second.attemptSuffix = func() string { return "attempt2" }
	affected := []AffectedMigration{{Version: 125, Rows: 1}}

	firstName := migrationBackupName(first, affected)
	secondName := migrationBackupName(second, affected)
	if firstName == secondName {
		t.Fatalf("backup names must differ: %q", firstName)
	}
	orphan := readyMigrationBackup("orphan-backup", firstName)
	fresh := readyMigrationBackup("fresh-backup", secondName)
	client := &recordingBackupClient{
		identity: railwaybackup.Identity{ProjectID: second.ProjectID, EnvironmentID: second.EnvironmentID},
		volume:   trustedVolumeIdentity(second),
		create:   railwaybackup.CreateResult{WorkflowID: "new-workflow"},
		lists: [][]railwaybackup.Backup{
			{orphan},
			{orphan, fresh},
			{orphan, fresh},
			{orphan, fresh},
		},
	}
	if err := runAfterBackupGate(context.Background(), second, client, affected, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if client.createdName != secondName || client.lockedID != "fresh-backup" {
		t.Fatalf("created=%q locked=%q", client.createdName, client.lockedID)
	}
}

func TestIrreversibleMigrationRegistryCoversHistoricalDataUpdates(t *testing.T) {
	if len(irreversibleMigrations) != 2 {
		t.Fatalf("registry length = %d", len(irreversibleMigrations))
	}
	if irreversibleMigrations[0].Version != 124 || irreversibleMigrations[1].Version != 125 {
		t.Fatalf("registry versions = %d, %d", irreversibleMigrations[0].Version, irreversibleMigrations[1].Version)
	}
	if !strings.Contains(strings.ToLower(irreversibleMigrations[0].Description), "retention_reason") {
		t.Fatalf("migration 124 description = %q", irreversibleMigrations[0].Description)
	}
	if !strings.Contains(strings.ToLower(irreversibleMigrations[1].Description), "retryable") {
		t.Fatalf("migration 125 description = %q", irreversibleMigrations[1].Description)
	}
	for _, migration := range irreversibleMigrations {
		if migration.CountAffected == nil {
			t.Fatalf("irreversible migration %d has no affected-row classifier", migration.Version)
		}
	}
}

func TestIrreversibleMigrationSafetyManifestMatchesRegistry(t *testing.T) {
	type manifestEntry struct {
		Version     int64  `json:"version"`
		Description string `json:"description"`
	}
	type operationEntry struct {
		Key                    string `json:"key"`
		Description            string `json:"description"`
		AffectedRowsClassifier string `json:"affected_rows_classifier"`
		BackupAction           string `json:"backup_action"`
		RollbackAction         string `json:"rollback_action"`
	}
	type safetyManifest struct {
		BaselineVersion            int64            `json:"baseline_version"`
		IrreversibleDataMigrations []manifestEntry  `json:"irreversible_data_migrations"`
		IrreversibleOperations     []operationEntry `json:"irreversible_operations"`
	}

	body, err := os.ReadFile("migrations/irreversible_data_migrations.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest safetyManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatalf("decode irreversible migration manifest: %v", err)
	}
	if manifest.BaselineVersion != 125 {
		t.Fatalf("manifest baseline = %d, want 125", manifest.BaselineVersion)
	}

	registry := make(map[int64]irreversibleMigration, len(irreversibleMigrations))
	for _, migration := range irreversibleMigrations {
		registry[migration.Version] = migration
	}
	marked := make(map[int64]manifestEntry, len(manifest.IrreversibleDataMigrations))
	for _, entry := range manifest.IrreversibleDataMigrations {
		marked[entry.Version] = entry
		migration, ok := registry[entry.Version]
		if !ok {
			t.Fatalf("manifest marks migration %d irreversible but runtime registry is missing it", entry.Version)
		}
		if migration.CountAffected == nil {
			t.Fatalf("manifest migration %d has no runtime classifier", entry.Version)
		}
	}
	for version := range registry {
		if _, ok := marked[version]; !ok {
			t.Fatalf("runtime registry migration %d is missing from irreversible manifest", version)
		}
	}
	operationRegistry := make(map[string]irreversibleOperation, len(irreversibleOperations))
	for _, operation := range irreversibleOperations {
		operationRegistry[operation.Key] = operation
	}
	if len(manifest.IrreversibleOperations) != len(operationRegistry) {
		t.Fatalf("irreversible operation manifest count = %d, registry count = %d", len(manifest.IrreversibleOperations), len(operationRegistry))
	}
	for _, entry := range manifest.IrreversibleOperations {
		operation, ok := operationRegistry[entry.Key]
		if !ok {
			t.Fatalf("manifest operation %q is missing from runtime registry", entry.Key)
		}
		if entry.Description != operation.Description || entry.AffectedRowsClassifier != operation.AffectedRowsClassifier ||
			entry.BackupAction != operation.BackupAction || entry.RollbackAction != operation.RollbackAction {
			t.Fatalf("manifest operation %q does not match runtime registry", entry.Key)
		}
	}

	err = fs.WalkDir(migrations, "migrations", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".sql" {
			return nil
		}
		prefix, _, ok := strings.Cut(filepath.Base(path), "_")
		if !ok {
			return nil
		}
		version, parseErr := strconv.ParseInt(prefix, 10, 64)
		if parseErr != nil || version <= manifest.BaselineVersion {
			return parseErr
		}
		migrationBody, readErr := fs.ReadFile(migrations, path)
		if readErr != nil {
			return readErr
		}
		text := strings.ToLower(string(migrationBody))
		irreversible := strings.Contains(text, "-- unipost:safety irreversible")
		reversible := strings.Contains(text, "-- unipost:safety reversible")
		if irreversible == reversible {
			return fmt.Errorf("migration %d must declare exactly one unipost:safety marker", version)
		}
		_, isMarked := marked[version]
		if irreversible != isMarked {
			return fmt.Errorf("migration %d safety marker disagrees with irreversible manifest", version)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPendingIrreversibleMigrationsUsesCurrentVersion(t *testing.T) {
	tests := []struct {
		current int64
		want    []int64
	}{
		{current: 0, want: []int64{124, 125}},
		{current: 123, want: []int64{124, 125}},
		{current: 124, want: []int64{125}},
		{current: 125, want: nil},
	}
	for _, test := range tests {
		if got := pendingIrreversibleMigrationVersions(test.current); !reflect.DeepEqual(got, test.want) {
			t.Fatalf("current %d: got %v, want %v", test.current, got, test.want)
		}
	}
}

func TestCIRequiresMigrationGatePostgresIntegration(t *testing.T) {
	body, err := os.ReadFile("../../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(body)
	for _, testName := range []string{
		"TestMigrationGatePostgres",
		"TestMigration133UpgradeAndGuardedDown",
		"TestRequireCurrentSchemaRejects124AndAccepts140",
		"GOOSE_MIGRATION_TEST_DATABASE_URL=\"$REQUEST_EVENTS_TEST_DATABASE_URL\" go test ./internal/db -run '^TestRunMigrationsAppliesAllEmbeddedMigrationsWithGoose$' -count=1",
		"go test -tags=integration ./internal/requestevents -count=1",
		"go test -tags=integration ./internal/requesteventpartitions -count=1",
		"go test -tags=integration ./internal/observabilityreads -count=1",
	} {
		if !strings.Contains(workflow, testName) {
			t.Fatalf("required PostgreSQL CI job does not run %s", testName)
		}
	}
	if !strings.Contains(workflow, "PUBLISHING_RESTRICTION_TEST_DATABASE_URL") {
		t.Fatal("required PostgreSQL CI job must provide the isolated database URL")
	}
	if !strings.Contains(workflow, "REQUEST_EVENTS_TEST_DATABASE_URL") {
		t.Fatal("observability PostgreSQL CI gate must use the isolated database URL")
	}
	if strings.Contains(workflow, "TestRequireCurrentSchemaRejects124AndAccepts139") {
		t.Fatal("required PostgreSQL CI selector still names the pre-migration-140 schema test")
	}

	integrationTestBody, err := os.ReadFile("migration_gate_postgres_integration_test.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(integrationTestBody), "want version=139") {
		t.Fatal("migration gate PostgreSQL diagnostics still name the pre-migration-140 schema version")
	}
}
