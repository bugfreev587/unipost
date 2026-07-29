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
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/xiaoboyu/unipost-api/internal/railwaybackup"
	"github.com/xiaoboyu/unipost-api/internal/testdbguard"
)

func openMigrationGateIntegrationDatabase(t *testing.T) (string, *sql.DB) {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv(publishingRestrictionIntegrationDatabaseEnv))
	if databaseURL == "" {
		t.Fatalf("%s is required and must point to an isolated PostgreSQL test service", publishingRestrictionIntegrationDatabaseEnv)
	}

	admin, _, err := testdbguard.OpenValidated(
		publishingRestrictionIntegrationDatabaseEnv,
		databaseURL,
		func(validatedURL string) (*sql.DB, error) {
			return sql.Open("pgx", validatedURL)
		},
	)
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
		CREATE TABLE integration_logs (
			id BIGSERIAL PRIMARY KEY,
			workspace_id TEXT NOT NULL DEFAULT '',
			ts TIMESTAMPTZ NOT NULL
		);
		CREATE TABLE social_posts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			idempotency_key TEXT,
			status TEXT NOT NULL
		);
		CREATE TABLE workspaces (
			id TEXT PRIMARY KEY
		);
		CREATE TABLE feature_flags (
			key TEXT PRIMARY KEY CHECK (key IN ('x_dms_v1', 'x_credits_billing_v1')),
			enabled BOOLEAN NOT NULL DEFAULT FALSE,
			description TEXT NOT NULL,
			updated_by TEXT NOT NULL DEFAULT 'system',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE feature_flag_changes (
			id TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
			flag_key TEXT NOT NULL REFERENCES feature_flags(key) ON DELETE RESTRICT,
			previous_enabled BOOLEAN NOT NULL,
			enabled BOOLEAN NOT NULL,
			changed_by TEXT NOT NULL,
			changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE webhooks (
			id TEXT PRIMARY KEY
		);
		CREATE TABLE webhook_deliveries (
			id TEXT PRIMARY KEY,
			webhook_id TEXT NOT NULL REFERENCES webhooks(id),
			event TEXT NOT NULL,
			payload JSONB NOT NULL
		);
		CREATE UNIQUE INDEX social_posts_workspace_scheduled_idempotency_uniq
		  ON social_posts (workspace_id, idempotency_key)
		  WHERE idempotency_key IS NOT NULL AND status = 'scheduled';
		CREATE TABLE platform_publishing_restriction_email_recipients (
			id TEXT PRIMARY KEY,
			canonical_user_id TEXT NOT NULL,
			represented_workspace_ids TEXT[] NOT NULL,
			status TEXT NOT NULL,
			retryable BOOLEAN NOT NULL DEFAULT TRUE
		);
		INSERT INTO platform_publishing_restriction_email_recipients (
			id, canonical_user_id, represented_workspace_ids, status, retryable
		) VALUES (
			'failed-recipient', 'canonical-user', ARRAY['workspace-1','workspace-2']::TEXT[], 'failed', TRUE
		);
	`)
	if err != nil {
		t.Fatalf("seed migration 124 state: %v", err)
	}
	seedAppliedMigrationVersions(t, database, 124)
}

func seedMigration123State(t *testing.T, database *sql.DB) {
	t.Helper()
	_, err := database.ExecContext(context.Background(), `
		CREATE TABLE goose_db_version (
			id SERIAL PRIMARY KEY,
			version_id BIGINT NOT NULL,
			is_applied BOOLEAN NOT NULL,
			tstamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE integration_logs (
			id BIGSERIAL PRIMARY KEY,
			workspace_id TEXT NOT NULL DEFAULT '',
			ts TIMESTAMPTZ NOT NULL
		);
		CREATE TABLE social_posts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			idempotency_key TEXT,
			status TEXT NOT NULL
		);
		CREATE TABLE workspaces (
			id TEXT PRIMARY KEY
		);
		CREATE TABLE feature_flags (
			key TEXT PRIMARY KEY CHECK (key IN ('x_dms_v1', 'x_credits_billing_v1')),
			enabled BOOLEAN NOT NULL DEFAULT FALSE,
			description TEXT NOT NULL,
			updated_by TEXT NOT NULL DEFAULT 'system',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE feature_flag_changes (
			id TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
			flag_key TEXT NOT NULL REFERENCES feature_flags(key) ON DELETE RESTRICT,
			previous_enabled BOOLEAN NOT NULL,
			enabled BOOLEAN NOT NULL,
			changed_by TEXT NOT NULL,
			changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE webhooks (
			id TEXT PRIMARY KEY
		);
		CREATE TABLE webhook_deliveries (
			id TEXT PRIMARY KEY,
			webhook_id TEXT NOT NULL REFERENCES webhooks(id),
			event TEXT NOT NULL,
			payload JSONB NOT NULL
		);
		CREATE UNIQUE INDEX social_posts_workspace_scheduled_idempotency_uniq
		  ON social_posts (workspace_id, idempotency_key)
		  WHERE idempotency_key IS NOT NULL AND status = 'scheduled';
		CREATE TABLE media_post_usages (
			id TEXT PRIMARY KEY,
			cleanup_after_at TIMESTAMPTZ,
			retention_reason TEXT NOT NULL
		);
		CREATE TABLE platform_publishing_restriction_email_recipients (
			id TEXT PRIMARY KEY,
			canonical_user_id TEXT NOT NULL,
			represented_workspace_ids TEXT[] NOT NULL,
			status TEXT NOT NULL
		);
		INSERT INTO media_post_usages (id, cleanup_after_at, retention_reason)
		VALUES ('active-usage', NULL, 'plan_status');
		INSERT INTO platform_publishing_restriction_email_recipients (
			id, canonical_user_id, represented_workspace_ids, status
		) VALUES (
			'failed-recipient', 'canonical-user', ARRAY['workspace-1','workspace-2']::TEXT[], 'failed'
		);
	`)
	if err != nil {
		t.Fatalf("seed migration 123 state: %v", err)
	}
	seedAppliedMigrationVersions(t, database, 123)
}

func seedAppliedMigrationVersions(t *testing.T, database *sql.DB, through int) {
	t.Helper()
	versions := make([]int, 0, through)
	err := fs.WalkDir(migrations, "migrations", func(path string, entry fs.DirEntry, walkErr error) error {
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
		if version <= through {
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
		volume:   trustedVolumeIdentity(config),
		create:   railwaybackup.CreateResult{WorkflowID: "workflow-125"},
		lists: [][]railwaybackup.Backup{
			{},
			{backup},
			{backup},
			{backup},
		},
	}
}

func freshPreviewGateConfig() MigrationGateConfig {
	config := testMigrationGateConfig()
	config.EnvironmentName = "unipost-pr-301"
	config.ServicePublicDomain = "preview-api-unipost-pr-301.up.railway.app"
	return config
}

func TestMigrationGatePostgresFreshDisposablePreviewBypassesBackup(t *testing.T) {
	databaseURL, database := openMigrationGateIntegrationDatabase(t)
	config := freshPreviewGateConfig()

	if err := RunMigrationsWithBackupGate(context.Background(), databaseURL, config, nil); err != nil {
		t.Fatalf("fresh disposable Preview migration gate: %v", err)
	}
	var version int64
	if err := database.QueryRowContext(context.Background(), `
		SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1
	`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 132 {
		t.Fatalf("fresh disposable Preview final version = %d, want 132", version)
	}
}

func TestMigrationGatePostgresDisposablePreviewWithExistingTableStillRequiresBackup(t *testing.T) {
	databaseURL, database := openMigrationGateIntegrationDatabase(t)
	if _, err := database.ExecContext(context.Background(), `CREATE TABLE existing_preview_data (id BIGINT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	config := freshPreviewGateConfig()

	err := RunMigrationsWithBackupGate(context.Background(), databaseURL, config, nil)
	if err == nil || !strings.Contains(err.Error(), "backup client is missing") {
		t.Fatalf("dirty disposable Preview gate error = %v", err)
	}
	var exists bool
	if err := database.QueryRowContext(context.Background(), `SELECT to_regclass('goose_db_version') IS NOT NULL`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("dirty disposable Preview must remain unmigrated")
	}
}

func TestMigrationGatePostgresMismatchedPreviewIdentityStillRequiresBackup(t *testing.T) {
	databaseURL, database := openMigrationGateIntegrationDatabase(t)
	config := freshPreviewGateConfig()
	config.ServicePublicDomain = "preview-api-unipost-pr-302.up.railway.app"

	err := RunMigrationsWithBackupGate(context.Background(), databaseURL, config, nil)
	if err == nil || !strings.Contains(err.Error(), "backup client is missing") {
		t.Fatalf("mismatched disposable Preview gate error = %v", err)
	}
	var exists bool
	if err := database.QueryRowContext(context.Background(), `SELECT to_regclass('goose_db_version') IS NOT NULL`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("mismatched disposable Preview must remain unmigrated")
	}
}

func TestMigrationGatePostgresApplies125AfterVerifiedBackupThenContinues127(t *testing.T) {
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
	var ownerUserIDs string
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
		SELECT retryable, ARRAY_TO_STRING(represented_owner_user_ids, ',')
		FROM platform_publishing_restriction_email_recipients
		WHERE id='failed-recipient'
	`).Scan(&retryable, &ownerUserIDs); err != nil {
		t.Fatal(err)
	}
	if version != 132 || retryable || ownerUserIDs != "canonical-user,canonical-user" {
		t.Fatalf(
			"version=%d retryable=%v owner_user_ids=%v, want version=132 retryable=false canonical owner backfill",
			version, retryable, ownerUserIDs,
		)
	}
	if client.lockedID != "backup-125" {
		t.Fatalf("locked backup ID = %q", client.lockedID)
	}

	zeroRowURL, zeroRowDatabase := openMigrationGateIntegrationDatabase(t)
	seedMigration124State(t, zeroRowDatabase)
	if _, err := zeroRowDatabase.ExecContext(context.Background(), `
		DELETE FROM platform_publishing_restriction_email_recipients
		WHERE status = 'failed'
	`); err != nil {
		t.Fatalf("remove migration 125 affected rows: %v", err)
	}
	zeroRowConfig := testMigrationGateConfig()
	zeroRowClient := successfulGateClient(zeroRowConfig, []AffectedMigration{{Version: 125, Rows: 0}})
	if err := RunMigrationsWithBackupGate(context.Background(), zeroRowURL, zeroRowConfig, zeroRowClient); err != nil {
		t.Fatalf("zero-row pending irreversible migration gate: %v", err)
	}
	if zeroRowClient.lockedID != "backup-125" {
		t.Fatalf("zero-row pending irreversible migration locked backup = %q", zeroRowClient.lockedID)
	}
	if err := zeroRowDatabase.QueryRowContext(context.Background(), `
		SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1
	`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 132 {
		t.Fatalf("zero-row pending irreversible migration final version = %d, want 132", version)
	}
}

func TestMigrationGatePostgresFailureBeforeVerificationLeaves124Unchanged(t *testing.T) {
	databaseURL, database := openMigrationGateIntegrationDatabase(t)
	seedMigration124State(t, database)
	config := testMigrationGateConfig()
	client := &recordingBackupClient{
		identity: railwaybackup.Identity{ProjectID: config.ProjectID, EnvironmentID: config.EnvironmentID},
		volume:   trustedVolumeIdentity(config),
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

func TestMigrationGatePostgresRejectsSameEnvironmentVolumeFromWrongServiceBeforeGoose(t *testing.T) {
	databaseURL, database := openMigrationGateIntegrationDatabase(t)
	seedMigration124State(t, database)
	config := testMigrationGateConfig()
	client := &recordingBackupClient{
		identity: railwaybackup.Identity{ProjectID: config.ProjectID, EnvironmentID: config.EnvironmentID},
		volume:   trustedVolumeIdentity(config),
	}
	client.volume.ServiceID = "api-service-not-postgres"

	err := RunMigrationsWithBackupGate(context.Background(), databaseURL, config, client)
	if err == nil || !strings.Contains(err.Error(), "volume instance identity mismatch") {
		t.Fatalf("gate error = %v", err)
	}
	wantCalls := []string{"identity", "volume:" + config.VolumeInstanceID}
	if !reflect.DeepEqual(client.calls, wantCalls) {
		t.Fatalf("Railway calls = %#v, want %#v", client.calls, wantCalls)
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
}

type concurrentBackupClient struct {
	mu            sync.Mutex
	identity      railwaybackup.Identity
	volume        railwaybackup.VolumeInstanceIdentity
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
	volume           railwaybackup.VolumeInstanceIdentity
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

func (c *pausedReadinessBackupClient) VolumeInstanceIdentity(context.Context, string) (railwaybackup.VolumeInstanceIdentity, error) {
	return c.volume, nil
}

func (c *pausedReadinessBackupClient) VerifyDatabaseBinding(context.Context, railwaybackup.DatabaseBindingRequest) error {
	return nil
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

func migrationURLWithApplicationName(t *testing.T, databaseURL, applicationName string) string {
	t.Helper()
	parsedURL, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatalf("parse migration database URL: %v", err)
	}
	query := parsedURL.Query()
	query.Set("application_name", applicationName)
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String()
}

func TestMigrationGatePostgresExcludesHistoricalRunMigrationsUntilBackupVerified(t *testing.T) {
	databaseURL, database := openMigrationGateIntegrationDatabase(t)
	seedMigration123State(t, database)
	config := testMigrationGateConfig()
	config.Timeout = 10 * time.Second
	client := &pausedReadinessBackupClient{
		identity:         railwaybackup.Identity{ProjectID: config.ProjectID, EnvironmentID: config.EnvironmentID},
		volume:           trustedVolumeIdentity(config),
		readinessStarted: make(chan struct{}),
		releaseReadiness: make(chan struct{}),
	}
	defer client.release()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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

	legacyApplicationName := fmt.Sprintf("migration-gate-historical-%d", time.Now().UnixNano())
	legacyDatabaseURL := migrationURLWithApplicationName(t, databaseURL, legacyApplicationName)
	legacyResult := make(chan error, 1)
	go func() {
		legacyResult <- RunMigrations(legacyDatabaseURL)
	}()

	lockAttemptContext, cancelLockAttempt := context.WithTimeout(ctx, 5*time.Second)
	defer cancelLockAttempt()
	poll := time.NewTicker(5 * time.Millisecond)
	defer poll.Stop()
	lockAttemptObserved := false
	legacyCompleted := false
	var legacyErr error
	for !lockAttemptObserved && !legacyCompleted {
		if err := database.QueryRowContext(lockAttemptContext, `
			SELECT EXISTS (
				SELECT 1
				FROM pg_stat_activity
				WHERE application_name = $1
				  AND query ILIKE '%pg_try_advisory_lock%'
			)
		`, legacyApplicationName).Scan(&lockAttemptObserved); err != nil {
			t.Fatalf("observe historical Goose lock attempt: %v", err)
		}
		if lockAttemptObserved {
			break
		}
		select {
		case legacyErr = <-legacyResult:
			legacyCompleted = true
		case <-poll.C:
		case <-lockAttemptContext.Done():
			t.Fatalf("historical Goose lock attempt was not observed: %v", lockAttemptContext.Err())
		}
	}
	if legacyCompleted {
		t.Errorf("historical RunMigrations completed before its Goose lock attempt could be observed: %v", legacyErr)
	} else {
		select {
		case legacyErr = <-legacyResult:
			legacyCompleted = true
			t.Errorf("historical RunMigrations completed after its Goose lock attempt but before backup readiness was released: %v", legacyErr)
		default:
		}
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
	var retentionReason string
	if err := database.QueryRowContext(ctx, `
		SELECT retention_reason
		FROM media_post_usages
		WHERE id = 'active-usage'
	`).Scan(&retentionReason); err != nil {
		t.Fatal(err)
	}
	var migration124Columns int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = 'platform_publishing_restriction_email_recipients'
		  AND column_name IN ('retryable', 'attempt_generation')
	`).Scan(&migration124Columns); err != nil {
		t.Fatal(err)
	}
	if version != 123 || retentionReason != "plan_status" || migration124Columns != 0 {
		t.Errorf(
			"before backup readiness release version=%d retention_reason=%q migration_124_columns=%d, want version=123 retention_reason=plan_status migration_124_columns=0",
			version,
			retentionReason,
			migration124Columns,
		)
	}

	client.release()
	select {
	case err := <-gateResult:
		if err != nil {
			t.Fatalf("migration gate: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("migration gate did not complete after backup readiness release: %v", ctx.Err())
	}
	if !legacyCompleted {
		select {
		case legacyErr = <-legacyResult:
			legacyCompleted = true
		case <-ctx.Done():
			t.Fatalf("historical RunMigrations did not complete after backup readiness release: %v", ctx.Err())
		}
	}
	if legacyErr != nil {
		t.Fatalf("historical RunMigrations: %v", legacyErr)
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
		SELECT retention_reason
		FROM media_post_usages
		WHERE id = 'active-usage'
	`).Scan(&retentionReason); err != nil {
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
	var ownerUserIDs string
	if err := database.QueryRowContext(ctx, `
		SELECT ARRAY_TO_STRING(represented_owner_user_ids, ',')
		FROM platform_publishing_restriction_email_recipients
		WHERE id = 'failed-recipient'
	`).Scan(&ownerUserIDs); err != nil {
		t.Fatal(err)
	}
	if version != 132 || retentionReason != "active_post" || retryable || ownerUserIDs != "canonical-user,canonical-user" {
		t.Fatalf(
			"after backup verification version=%d retention_reason=%q retryable=%v owner_user_ids=%v, want version=132 retention_reason=active_post retryable=false canonical owner backfill",
			version,
			retentionReason,
			retryable,
			ownerUserIDs,
		)
	}
	if createCalls := client.creates(); createCalls != 1 {
		t.Fatalf("backup create calls = %d, want 1", createCalls)
	}
}

func (c *concurrentBackupClient) Identity(context.Context) (railwaybackup.Identity, error) {
	return c.identity, nil
}

func (c *concurrentBackupClient) VolumeInstanceIdentity(context.Context, string) (railwaybackup.VolumeInstanceIdentity, error) {
	return c.volume, nil
}

func (c *concurrentBackupClient) VerifyDatabaseBinding(context.Context, railwaybackup.DatabaseBindingRequest) error {
	return nil
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
		volume:        trustedVolumeIdentity(config),
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
	if version != 132 {
		t.Fatalf("final migration version = %d, want 132", version)
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
		volume:   trustedVolumeIdentity(firstConfig),
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
		volume:   trustedVolumeIdentity(secondConfig),
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
	var version int64
	if err := database.QueryRowContext(context.Background(), `
		SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1
	`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 132 {
		t.Fatalf("replacement runner final migration version = %d, want 132", version)
	}
}

func TestRequireCurrentSchemaRejects124AndAccepts132(t *testing.T) {
	databaseURL, database := openMigrationGateIntegrationDatabase(t)
	seedMigration124State(t, database)

	err := RequireCurrentSchema(context.Background(), databaseURL)
	if err == nil || !strings.Contains(err.Error(), "current version 124") || !strings.Contains(err.Error(), "required version 132") {
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
		INSERT INTO goose_db_version (version_id, is_applied) VALUES (133, TRUE);
	`)
	if err != nil {
		t.Fatal(err)
	}

	err = RequireCurrentSchema(context.Background(), databaseURL)
	if err == nil || !strings.Contains(err.Error(), "newer than binary required version 132") || !strings.Contains(err.Error(), "rollback is unsafe") {
		t.Fatalf("schema-ahead guard error = %v", err)
	}
}
