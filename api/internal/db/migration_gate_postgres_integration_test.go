//go:build integration

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/xiaoboyu/unipost-api/internal/railwaybackup"
)

func openMigrationGateIntegrationDatabase(t *testing.T) (string, *sql.DB) {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv(publishingRestrictionIntegrationDatabaseEnv))
	if databaseURL == "" {
		t.Fatalf("%s is required and must point to an isolated PostgreSQL test service", publishingRestrictionIntegrationDatabaseEnv)
	}

	admin, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open isolated PostgreSQL service: %v", err)
	}
	schema := fmt.Sprintf("migration_gate_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(context.Background(), "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		admin.Close()
		t.Fatalf("create migration gate schema: %v", err)
	}

	parsedURL, err := url.Parse(databaseURL)
	if err != nil {
		admin.Close()
		t.Fatalf("parse isolated PostgreSQL URL: %v", err)
	}
	query := parsedURL.Query()
	query.Set("search_path", schema)
	parsedURL.RawQuery = query.Encode()
	isolatedURL := parsedURL.String()
	database, err := sql.Open("pgx", isolatedURL)
	if err != nil {
		admin.Close()
		t.Fatalf("open migration gate schema: %v", err)
	}
	if err := database.PingContext(context.Background()); err != nil {
		database.Close()
		admin.Close()
		t.Fatalf("ping migration gate schema: %v", err)
	}
	var currentSchema string
	if err := database.QueryRowContext(context.Background(), `SELECT current_schema()`).Scan(&currentSchema); err != nil {
		database.Close()
		admin.Close()
		t.Fatalf("read migration gate current schema: %v", err)
	}
	if currentSchema != schema {
		database.Close()
		admin.Close()
		t.Fatalf("migration gate current schema = %q, want %q", currentSchema, schema)
	}

	t.Cleanup(func() {
		database.Close()
		if _, err := admin.ExecContext(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Errorf("drop migration gate schema: %v", err)
		}
		admin.Close()
	})
	return isolatedURL, database
}

func seedMigration124State(t *testing.T, database *sql.DB) {
	t.Helper()
	_, err := database.ExecContext(context.Background(), `
		CREATE TABLE goose_db_version (
			id SERIAL PRIMARY KEY,
			version_id BIGINT NOT NULL,
			is_applied BOOLEAN NOT NULL,
			tstamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE platform_publishing_restriction_email_recipients (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL,
			retryable BOOLEAN NOT NULL DEFAULT TRUE
		);
		INSERT INTO platform_publishing_restriction_email_recipients (id, status, retryable)
		VALUES ('failed-recipient', 'failed', TRUE);
	`)
	if err != nil {
		t.Fatalf("seed migration 124 state: %v", err)
	}

	versions := make([]int, 0, 124)
	err = fs.WalkDir(migrations, "migrations", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".sql" {
			return nil
		}
		prefix, _, ok := strings.Cut(filepath.Base(path), "_")
		if !ok {
			return fmt.Errorf("migration filename %q has no version prefix", path)
		}
		version, parseErr := strconv.Atoi(prefix)
		if parseErr != nil {
			return parseErr
		}
		if version <= 124 {
			versions = append(versions, version)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("enumerate embedded migrations: %v", err)
	}
	sort.Ints(versions)
	for _, version := range versions {
		if _, err := database.ExecContext(context.Background(), `
			INSERT INTO goose_db_version (version_id, is_applied) VALUES ($1, TRUE)
		`, version); err != nil {
			t.Fatalf("seed applied migration %d: %v", version, err)
		}
	}
}

func successfulGateClient(config MigrationGateConfig, affected []AffectedMigration) *recordingBackupClient {
	name := migrationBackupName(config, affected)
	backup := readyMigrationBackup("backup-125", name)
	return &recordingBackupClient{
		identity: railwaybackup.Identity{ProjectID: config.ProjectID, EnvironmentID: config.EnvironmentID},
		create:   railwaybackup.CreateResult{WorkflowID: "workflow-125"},
		lists: [][]railwaybackup.Backup{
			{},
			{backup},
			{backup},
			{backup},
		},
	}
}

func TestMigrationGatePostgresApplies125OnlyAfterVerifiedBackup(t *testing.T) {
	databaseURL, database := openMigrationGateIntegrationDatabase(t)
	seedMigration124State(t, database)
	config := testMigrationGateConfig()
	affected := []AffectedMigration{{Version: 125, Rows: 1}}
	client := successfulGateClient(config, affected)

	if err := RunMigrationsWithBackupGate(context.Background(), databaseURL, config, client); err != nil {
		t.Fatal(err)
	}
	var version int64
	var retryable bool
	if err := database.QueryRowContext(context.Background(), `
		SELECT version_id
		FROM goose_db_version
		WHERE is_applied
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(context.Background(), `
		SELECT retryable
		FROM platform_publishing_restriction_email_recipients
		WHERE id='failed-recipient'
	`).Scan(&retryable); err != nil {
		t.Fatal(err)
	}
	if version != 125 || retryable {
		t.Fatalf("version=%d retryable=%v, want version=125 retryable=false", version, retryable)
	}
	if client.lockedID != "backup-125" {
		t.Fatalf("locked backup ID = %q", client.lockedID)
	}
}

func TestMigrationGatePostgresFailureBeforeVerificationLeaves124Unchanged(t *testing.T) {
	databaseURL, database := openMigrationGateIntegrationDatabase(t)
	seedMigration124State(t, database)
	config := testMigrationGateConfig()
	client := &recordingBackupClient{
		identity: railwaybackup.Identity{ProjectID: config.ProjectID, EnvironmentID: config.EnvironmentID},
		create:   railwaybackup.CreateResult{WorkflowID: "workflow-125"},
		lists:    [][]railwaybackup.Backup{{}, {}, {}, {}},
	}

	err := RunMigrationsWithBackupGate(context.Background(), databaseURL, config, client)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("gate error = %v", err)
	}
	var version int64
	var retryable bool
	if err := database.QueryRowContext(context.Background(), `
		SELECT version_id
		FROM goose_db_version
		WHERE is_applied
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(context.Background(), `
		SELECT retryable
		FROM platform_publishing_restriction_email_recipients
		WHERE id='failed-recipient'
	`).Scan(&retryable); err != nil {
		t.Fatal(err)
	}
	if version != 124 || !retryable {
		t.Fatalf("version=%d retryable=%v, want unchanged version=124 retryable=true", version, retryable)
	}
	if client.lockedID != "" {
		t.Fatalf("unexpected locked backup ID %q", client.lockedID)
	}
}

type concurrentBackupClient struct {
	mu            sync.Mutex
	identity      railwaybackup.Identity
	createStarted chan struct{}
	releaseCreate chan struct{}
	createdName   string
	createCalls   int
	lockCalls     int
	startOnce     sync.Once
}

type pausedReadinessBackupClient struct {
	mu               sync.Mutex
	identity         railwaybackup.Identity
	readinessStarted chan struct{}
	releaseReadiness chan struct{}
	readinessOnce    sync.Once
	releaseOnce      sync.Once
	createdName      string
	createCalls      int
}

func (c *pausedReadinessBackupClient) Identity(context.Context) (railwaybackup.Identity, error) {
	return c.identity, nil
}

func (c *pausedReadinessBackupClient) List(ctx context.Context, _ string) ([]railwaybackup.Backup, error) {
	c.mu.Lock()
	createdName := c.createdName
	c.mu.Unlock()
	if createdName == "" {
		return nil, nil
	}

	c.readinessOnce.Do(func() { close(c.readinessStarted) })
	select {
	case <-c.releaseReadiness:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return []railwaybackup.Backup{readyMigrationBackup("historical-exclusion-backup", createdName)}, nil
}

func (c *pausedReadinessBackupClient) Create(_ context.Context, _ string, name string) (railwaybackup.CreateResult, error) {
	c.mu.Lock()
	c.createCalls++
	c.createdName = name
	c.mu.Unlock()
	return railwaybackup.CreateResult{WorkflowID: "historical-exclusion-workflow"}, nil
}

func (c *pausedReadinessBackupClient) Lock(context.Context, string, string) error {
	return nil
}

func (c *pausedReadinessBackupClient) release() {
	c.releaseOnce.Do(func() { close(c.releaseReadiness) })
}

func (c *pausedReadinessBackupClient) creates() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.createCalls
}

func TestMigrationGatePostgresExcludesHistoricalRunMigrationsUntilBackupVerified(t *testing.T) {
	databaseURL, database := openMigrationGateIntegrationDatabase(t)
	seedMigration124State(t, database)
	config := testMigrationGateConfig()
	config.Timeout = 10 * time.Second
	client := &pausedReadinessBackupClient{
		identity:         railwaybackup.Identity{ProjectID: config.ProjectID, EnvironmentID: config.EnvironmentID},
		readinessStarted: make(chan struct{}),
		releaseReadiness: make(chan struct{}),
	}
	defer client.release()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	gateResult := make(chan error, 1)
	go func() {
		gateResult <- RunMigrationsWithBackupGate(ctx, databaseURL, config, client)
	}()

	select {
	case <-client.readinessStarted:
	case err := <-gateResult:
		t.Fatalf("migration gate completed before backup readiness pause: %v", err)
	case <-ctx.Done():
		t.Fatalf("migration gate did not reach backup readiness verification: %v", ctx.Err())
	}

	legacyResult := make(chan error, 1)
	go func() {
		legacyResult <- RunMigrations(databaseURL)
	}()

	select {
	case err := <-legacyResult:
		t.Errorf("historical RunMigrations completed before backup readiness was released: %v", err)
		legacyResult <- err
	case <-time.After(250 * time.Millisecond):
	}

	var version int64
	if err := database.QueryRowContext(ctx, `
		SELECT version_id
		FROM goose_db_version
		WHERE is_applied
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	var retryable bool
	if err := database.QueryRowContext(ctx, `
		SELECT retryable
		FROM platform_publishing_restriction_email_recipients
		WHERE id = 'failed-recipient'
	`).Scan(&retryable); err != nil {
		t.Fatal(err)
	}
	if version != 124 || !retryable {
		t.Errorf("before backup readiness release version=%d retryable=%v, want version=124 retryable=true", version, retryable)
	}

	client.release()
	if err := <-gateResult; err != nil {
		t.Fatalf("migration gate: %v", err)
	}
	if err := <-legacyResult; err != nil {
		t.Fatalf("historical RunMigrations: %v", err)
	}

	if err := database.QueryRowContext(ctx, `
		SELECT version_id
		FROM goose_db_version
		WHERE is_applied
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT retryable
		FROM platform_publishing_restriction_email_recipients
		WHERE id = 'failed-recipient'
	`).Scan(&retryable); err != nil {
		t.Fatal(err)
	}
	if version != 125 || retryable {
		t.Fatalf("after backup verification version=%d retryable=%v, want version=125 retryable=false", version, retryable)
	}
	if createCalls := client.creates(); createCalls != 1 {
		t.Fatalf("backup create calls = %d, want 1", createCalls)
	}
}

func (c *concurrentBackupClient) Identity(context.Context) (railwaybackup.Identity, error) {
	return c.identity, nil
}

func (c *concurrentBackupClient) List(context.Context, string) ([]railwaybackup.Backup, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.createdName == "" {
		return nil, nil
	}
	return []railwaybackup.Backup{readyMigrationBackup("concurrent-backup", c.createdName)}, nil
}

func (c *concurrentBackupClient) Create(_ context.Context, _ string, name string) (railwaybackup.CreateResult, error) {
	c.mu.Lock()
	c.createCalls++
	c.createdName = name
	c.mu.Unlock()
	c.startOnce.Do(func() { close(c.createStarted) })
	<-c.releaseCreate
	return railwaybackup.CreateResult{WorkflowID: "concurrent-workflow"}, nil
}

func (c *concurrentBackupClient) Lock(context.Context, string, string) error {
	c.mu.Lock()
	c.lockCalls++
	c.mu.Unlock()
	return nil
}

func (c *concurrentBackupClient) counts() (int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.createCalls, c.lockCalls
}

func TestMigrationGatePostgresConcurrentPreDeploysCreateOneBackup(t *testing.T) {
	databaseURL, database := openMigrationGateIntegrationDatabase(t)
	seedMigration124State(t, database)
	config := testMigrationGateConfig()
	config.Timeout = 2 * time.Second
	client := &concurrentBackupClient{
		identity:      railwaybackup.Identity{ProjectID: config.ProjectID, EnvironmentID: config.EnvironmentID},
		createStarted: make(chan struct{}),
		releaseCreate: make(chan struct{}),
	}

	const runners = 4
	start := make(chan struct{})
	errorsByRunner := make(chan error, runners)
	for range runners {
		go func() {
			<-start
			errorsByRunner <- RunMigrationsWithBackupGate(context.Background(), databaseURL, config, client)
		}()
	}
	close(start)
	select {
	case <-client.createStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("migration leader did not reach backup create")
	}
	time.Sleep(100 * time.Millisecond)
	if createCalls, _ := client.counts(); createCalls != 1 {
		t.Fatalf("backup creates while leader blocked = %d, want 1", createCalls)
	}
	close(client.releaseCreate)
	for range runners {
		if err := <-errorsByRunner; err != nil {
			t.Fatalf("concurrent migration runner: %v", err)
		}
	}
	createCalls, lockCalls := client.counts()
	if createCalls != 1 || lockCalls != 1 {
		t.Fatalf("backup create calls=%d lock calls=%d, want 1/1", createCalls, lockCalls)
	}
	var version int64
	if err := database.QueryRowContext(context.Background(), `
		SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1
	`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 125 {
		t.Fatalf("final migration version = %d, want 125", version)
	}
}

func TestMigrationGatePostgresReplacementAfterLockedOrphanCreatesFreshBackup(t *testing.T) {
	databaseURL, database := openMigrationGateIntegrationDatabase(t)
	seedMigration124State(t, database)
	affected := []AffectedMigration{{Version: 125, Rows: 1}}

	firstConfig := testMigrationGateConfig()
	firstConfig.attemptSuffix = func() string { return "orphan1" }
	firstConfig.beforeMigrations = func(context.Context) error { return errors.New("simulated leader crash") }
	firstName := migrationBackupName(firstConfig, affected)
	orphan := readyMigrationBackup("orphan-backup", firstName)
	firstClient := &recordingBackupClient{
		identity: railwaybackup.Identity{ProjectID: firstConfig.ProjectID, EnvironmentID: firstConfig.EnvironmentID},
		create:   railwaybackup.CreateResult{WorkflowID: "orphan-workflow"},
		lists:    [][]railwaybackup.Backup{{}, {orphan}, {orphan}, {orphan}},
	}
	err := RunMigrationsWithBackupGate(context.Background(), databaseURL, firstConfig, firstClient)
	if err == nil || !strings.Contains(err.Error(), "simulated leader crash") {
		t.Fatalf("first runner error = %v", err)
	}
	if firstClient.lockedID != "orphan-backup" {
		t.Fatalf("first locked backup = %q", firstClient.lockedID)
	}

	secondConfig := testMigrationGateConfig()
	secondConfig.attemptSuffix = func() string { return "attempt2" }
	secondName := migrationBackupName(secondConfig, affected)
	fresh := readyMigrationBackup("fresh-backup", secondName)
	secondClient := &recordingBackupClient{
		identity: railwaybackup.Identity{ProjectID: secondConfig.ProjectID, EnvironmentID: secondConfig.EnvironmentID},
		create:   railwaybackup.CreateResult{WorkflowID: "fresh-workflow"},
		lists: [][]railwaybackup.Backup{
			{orphan},
			{orphan, fresh},
			{orphan, fresh},
			{orphan, fresh},
		},
	}
	if err := RunMigrationsWithBackupGate(context.Background(), databaseURL, secondConfig, secondClient); err != nil {
		t.Fatal(err)
	}
	if secondClient.createdName != secondName || secondClient.lockedID != "fresh-backup" {
		t.Fatalf("second created=%q locked=%q", secondClient.createdName, secondClient.lockedID)
	}
	if secondClient.lockedID == firstClient.lockedID {
		t.Fatal("replacement runner reused orphan backup")
	}
}

func TestRequireCurrentSchemaRejects124AndAccepts125(t *testing.T) {
	databaseURL, database := openMigrationGateIntegrationDatabase(t)
	seedMigration124State(t, database)

	err := RequireCurrentSchema(context.Background(), databaseURL)
	if err == nil || !strings.Contains(err.Error(), "current version 124") || !strings.Contains(err.Error(), "required version 125") {
		t.Fatalf("schema guard error = %v", err)
	}

	config := testMigrationGateConfig()
	client := successfulGateClient(config, []AffectedMigration{{Version: 125, Rows: 1}})
	if err := RunMigrationsWithBackupGate(context.Background(), databaseURL, config, client); err != nil {
		t.Fatal(err)
	}
	if err := RequireCurrentSchema(context.Background(), databaseURL); err != nil {
		t.Fatalf("schema guard rejected current schema: %v", err)
	}
}

func TestRequireCurrentSchemaRejectsNewerDatabaseAsUnsafeRollback(t *testing.T) {
	databaseURL, database := openMigrationGateIntegrationDatabase(t)
	_, err := database.ExecContext(context.Background(), `
		CREATE TABLE goose_db_version (
			id SERIAL PRIMARY KEY,
			version_id BIGINT NOT NULL,
			is_applied BOOLEAN NOT NULL,
			tstamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		INSERT INTO goose_db_version (version_id, is_applied) VALUES (126, TRUE);
	`)
	if err != nil {
		t.Fatal(err)
	}

	err = RequireCurrentSchema(context.Background(), databaseURL)
	if err == nil || !strings.Contains(err.Error(), "newer than binary required version 125") || !strings.Contains(err.Error(), "rollback is unsafe") {
		t.Fatalf("schema-ahead guard error = %v", err)
	}
}
