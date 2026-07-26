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
	lists       [][]railwaybackup.Backup
	create      railwaybackup.CreateResult
	identityErr error
	listErr     error
	createErr   error
	lockErr     error
	calls       []string
	listIndex   int
	lockedID    string
	createdName string
}

func (c *recordingBackupClient) Identity(context.Context) (railwaybackup.Identity, error) {
	c.calls = append(c.calls, "identity")
	return c.identity, c.identityErr
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
		ProjectID:        "project-1",
		EnvironmentID:    "environment-1",
		VolumeInstanceID: "volume-instance-1",
		ApplicationSHA:   "9eefc1090cfb25b6f23b753603506e5c1c7dc1bc",
		PollInterval:     time.Millisecond,
		Timeout:          50 * time.Millisecond,
		attemptSuffix:    func() string { return "attempt1" },
	}
}

func TestMigrationGateSkipsRailwayWhenAffectedRowsAreZero(t *testing.T) {
	client := &recordingBackupClient{}
	migrationsCalled := false
	err := runAfterBackupGate(
		context.Background(),
		testMigrationGateConfig(),
		client,
		[]AffectedMigration{{Version: 124, Rows: 0}, {Version: 125, Rows: 0}},
		func(context.Context) error { migrationsCalled = true; return nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(client.calls) != 0 {
		t.Fatalf("Railway calls = %v", client.calls)
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
		create:   railwaybackup.CreateResult{WorkflowID: "createVolumeInstanceBackup/workflow"},
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
		create:   railwaybackup.CreateResult{WorkflowID: "workflow-is-not-a-backup-id"},
		lists:    [][]railwaybackup.Backup{{}, {}, {}, {}},
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
	type safetyManifest struct {
		BaselineVersion            int64           `json:"baseline_version"`
		IrreversibleDataMigrations []manifestEntry `json:"irreversible_data_migrations"`
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
		"TestRequireCurrentSchema",
	} {
		if !strings.Contains(workflow, testName) {
			t.Fatalf("required PostgreSQL CI job does not run %s", testName)
		}
	}
	if !strings.Contains(workflow, "PUBLISHING_RESTRICTION_TEST_DATABASE_URL") {
		t.Fatal("required PostgreSQL CI job must provide the isolated database URL")
	}
}
