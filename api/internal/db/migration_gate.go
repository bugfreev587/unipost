package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/pressly/goose/v3/lock"
	"github.com/xiaoboyu/unipost-api/internal/railwaybackup"
)

type irreversibleMigration struct {
	Version       int64
	Description   string
	CountAffected func(context.Context, migrationQueryer, int64) (int64, error)
}

var irreversibleMigrations = []irreversibleMigration{
	{
		Version:       124,
		Description:   "overwrites existing media_post_usages retention_reason values",
		CountAffected: countMigration124AffectedRows,
	},
	{
		Version:       125,
		Description:   "overwrites existing failed email recipient retryable values",
		CountAffected: countMigration125AffectedRows,
	},
}

type AffectedMigration struct {
	Version int64
	Rows    int64
}

type MigrationGateConfig struct {
	ProjectID            string
	EnvironmentID        string
	EnvironmentName      string
	ServicePreviewURL    string
	VolumeInstanceID     string
	PostgresServiceID    string
	ApplicationServiceID string
	ApplicationSHA       string
	PollInterval         time.Duration
	Timeout              time.Duration

	attemptSuffix    func() string
	beforeMigrations func(context.Context) error
	databaseURL      string
}

func RunMigrationsWithBackupGate(
	ctx context.Context,
	databaseURL string,
	config MigrationGateConfig,
	client railwaybackup.Client,
) error {
	config.databaseURL = databaseURL
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open database for migration backup gate: %w", err)
	}
	defer database.Close()

	connection, err := database.Conn(ctx)
	if err != nil {
		return fmt.Errorf("reserve database connection for migration backup gate: %w", err)
	}
	defer connection.Close()
	sessionLocker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("create migration backup session locker: %w", err)
	}
	if err := sessionLocker.SessionLock(ctx, connection); err != nil {
		return fmt.Errorf("acquire migration backup session lock: %w", err)
	}
	defer func() {
		if unlockErr := sessionLocker.SessionUnlock(context.Background(), connection); unlockErr != nil {
			slog.Error("release migration backup session lock", "error", unlockErr)
		}
	}()

	currentVersion, err := readCurrentMigrationVersion(ctx, connection)
	if err != nil {
		return err
	}
	affected, err := countAffectedIrreversibleMigrations(ctx, connection, currentVersion)
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "irreversible migration preflight completed",
		"current_version", currentVersion,
		"affected_migrations", affected,
	)
	freshPreview, err := freshDisposablePreviewCanBypassBackup(ctx, connection, currentVersion, config, affected)
	if err != nil {
		return fmt.Errorf("inspect disposable Railway Preview before migration backup bypass: %w", err)
	}
	if freshPreview {
		slog.WarnContext(ctx, "fresh disposable Railway Preview migration backup bypass authorized",
			"project_id", config.ProjectID,
			"environment_id", config.EnvironmentID,
			"environment_name", config.EnvironmentName,
			"service_preview_url", config.ServicePreviewURL,
			"application_service_id", config.ApplicationServiceID,
			"application_sha", config.ApplicationSHA,
			"current_version", currentVersion,
			"affected_migrations", affected,
		)
		if config.beforeMigrations != nil {
			if err := config.beforeMigrations(ctx); err != nil {
				return fmt.Errorf("before migration execution: %w", err)
			}
		}
		return runMigrations(ctx, database, false)
	}

	return runAfterBackupGate(ctx, config, client, affected, func(context.Context) error {
		return runMigrations(ctx, database, false)
	})
}

var disposablePreviewEnvironmentPattern = regexp.MustCompile(`^unipost-pr-([1-9][0-9]*)$`)

func freshDisposablePreviewCanBypassBackup(
	ctx context.Context,
	queryer migrationQueryer,
	currentVersion int64,
	config MigrationGateConfig,
	affected []AffectedMigration,
) (bool, error) {
	if currentVersion != 0 || len(affected) == 0 {
		return false, nil
	}
	for _, migration := range affected {
		if migration.Rows != 0 {
			return false, nil
		}
	}
	environmentName := strings.TrimSpace(config.EnvironmentName)
	match := disposablePreviewEnvironmentPattern.FindStringSubmatch(environmentName)
	if len(match) != 2 {
		return false, nil
	}
	expectedPreviewURL := fmt.Sprintf("https://preview-api-unipost-pr-%s.up.railway.app", match[1])
	if strings.TrimSpace(config.ServicePreviewURL) != expectedPreviewURL {
		return false, nil
	}
	for _, value := range []string{
		config.ProjectID,
		config.EnvironmentID,
		config.ApplicationServiceID,
	} {
		if strings.TrimSpace(value) == "" {
			return false, nil
		}
	}
	sha := strings.TrimSpace(config.ApplicationSHA)
	decodedSHA, err := hex.DecodeString(sha)
	if err != nil || len(decodedSHA) != 20 || sha != strings.ToLower(sha) {
		return false, nil
	}
	var baseTables int64
	if err := queryer.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_schema = current_schema()
		  AND table_type = 'BASE TABLE'
	`).Scan(&baseTables); err != nil {
		return false, fmt.Errorf("count current-schema base tables: %w", err)
	}
	return baseTables == 0, nil
}

type migrationQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readCurrentMigrationVersion(ctx context.Context, queryer migrationQueryer) (int64, error) {
	exists, err := migrationTableExists(ctx, queryer, "goose_db_version")
	if err != nil {
		return 0, fmt.Errorf("inspect Goose migration table: %w", err)
	}
	if !exists {
		return 0, nil
	}
	var version int64
	err = queryer.QueryRowContext(ctx, `
		SELECT COALESCE((
			SELECT version_id
			FROM goose_db_version
			WHERE is_applied
			ORDER BY id DESC
			LIMIT 1
		), 0)
	`).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("read current Goose migration version: %w", err)
	}
	return version, nil
}

func countAffectedIrreversibleMigrations(
	ctx context.Context,
	queryer migrationQueryer,
	currentVersion int64,
) ([]AffectedMigration, error) {
	affected := make([]AffectedMigration, 0, len(irreversibleMigrations))
	for _, migration := range irreversibleMigrations {
		if migration.Version <= currentVersion {
			continue
		}
		if migration.CountAffected == nil {
			return nil, fmt.Errorf("irreversible migration %d has no affected-row classifier", migration.Version)
		}
		rows, err := migration.CountAffected(ctx, queryer, currentVersion)
		if err != nil {
			return nil, fmt.Errorf("count rows affected by irreversible migration %d: %w", migration.Version, err)
		}
		affected = append(affected, AffectedMigration{Version: migration.Version, Rows: rows})
	}
	return affected, nil
}

func countMigration124AffectedRows(ctx context.Context, queryer migrationQueryer, currentVersion int64) (int64, error) {
	exists, err := migrationTableExists(ctx, queryer, "media_post_usages")
	if err != nil || !exists {
		return 0, err
	}
	query := `SELECT COUNT(*) FROM media_post_usages WHERE cleanup_after_at IS NULL`
	if currentVersion >= 122 {
		query += ` AND retention_reason = 'plan_status'`
	}
	var rows int64
	if err := queryer.QueryRowContext(ctx, query).Scan(&rows); err != nil {
		return 0, err
	}
	return rows, nil
}

func countMigration125AffectedRows(ctx context.Context, queryer migrationQueryer, _ int64) (int64, error) {
	exists, err := migrationTableExists(ctx, queryer, "platform_publishing_restriction_email_recipients")
	if err != nil || !exists {
		return 0, err
	}
	var rows int64
	if err := queryer.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM platform_publishing_restriction_email_recipients
		WHERE status = 'failed'
	`).Scan(&rows); err != nil {
		return 0, err
	}
	return rows, nil
}

func migrationTableExists(ctx context.Context, queryer migrationQueryer, tableName string) (bool, error) {
	var exists bool
	if err := queryer.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, tableName).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func pendingIrreversibleMigrationVersions(currentVersion int64) []int64 {
	var pending []int64
	for _, migration := range irreversibleMigrations {
		if migration.Version > currentVersion {
			pending = append(pending, migration.Version)
		}
	}
	return pending
}

func runAfterBackupGate(
	ctx context.Context,
	config MigrationGateConfig,
	client railwaybackup.Client,
	affected []AffectedMigration,
	runMigrations func(context.Context) error,
) (err error) {
	if len(affected) == 0 {
		return runMigrations(ctx)
	}
	defer func() {
		if err != nil {
			err = fmt.Errorf(
				"Railway backup gate blocked environment %q with affected migrations [%s]: %w",
				config.EnvironmentID,
				formatAffectedMigrations(affected),
				err,
			)
		}
	}()
	if client == nil {
		return fmt.Errorf("Railway backup is required but the backup client is missing")
	}
	if err := validateMigrationGateConfig(config); err != nil {
		return err
	}

	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	gateContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	identity, err := client.Identity(gateContext)
	if err != nil {
		return fmt.Errorf("verify Railway token identity: %w", err)
	}
	if identity.ProjectID != config.ProjectID || identity.EnvironmentID != config.EnvironmentID {
		return fmt.Errorf(
			"Railway token identity mismatch: got project %q environment %q, expected project %q environment %q",
			identity.ProjectID,
			identity.EnvironmentID,
			config.ProjectID,
			config.EnvironmentID,
		)
	}
	volumeIdentity, err := client.VolumeInstanceIdentity(gateContext, config.VolumeInstanceID)
	if err != nil {
		return fmt.Errorf("verify Railway volume instance identity: %w", err)
	}
	if volumeIdentity.ID == "" || volumeIdentity.ProjectID == "" ||
		volumeIdentity.EnvironmentID == "" || volumeIdentity.ServiceID == "" {
		return fmt.Errorf("Railway volume instance identity response is missing required fields")
	}
	if volumeIdentity.ID != config.VolumeInstanceID ||
		volumeIdentity.ProjectID != config.ProjectID ||
		volumeIdentity.EnvironmentID != config.EnvironmentID ||
		volumeIdentity.ServiceID != config.PostgresServiceID {
		return fmt.Errorf(
			"Railway volume instance identity mismatch: got volume %q project %q environment %q service %q, expected volume %q project %q environment %q service %q",
			volumeIdentity.ID,
			volumeIdentity.ProjectID,
			volumeIdentity.EnvironmentID,
			volumeIdentity.ServiceID,
			config.VolumeInstanceID,
			config.ProjectID,
			config.EnvironmentID,
			config.PostgresServiceID,
		)
	}
	if err := client.VerifyDatabaseBinding(gateContext, railwaybackup.DatabaseBindingRequest{
		ProjectID:            config.ProjectID,
		EnvironmentID:        config.EnvironmentID,
		ApplicationServiceID: config.ApplicationServiceID,
		PostgresServiceID:    config.PostgresServiceID,
		RuntimeDatabaseURL:   config.databaseURL,
	}); err != nil {
		return fmt.Errorf("verify Railway database target binding: %w", err)
	}

	before, err := client.List(gateContext, config.VolumeInstanceID)
	if err != nil {
		return fmt.Errorf("list Railway backups before creation: %w", err)
	}
	existingIDs := make(map[string]struct{}, len(before))
	for _, backup := range before {
		existingIDs[backup.ID] = struct{}{}
	}

	backupName := migrationBackupName(config, affected)
	created, err := client.Create(gateContext, config.VolumeInstanceID, backupName)
	if err != nil {
		return fmt.Errorf("create Railway pre-migration backup: %w", err)
	}
	if created.WorkflowID == "" {
		return fmt.Errorf("create Railway pre-migration backup: missing workflow ID")
	}

	backup, err := waitForStableNewBackup(gateContext, config, client, backupName, existingIDs)
	if err != nil {
		return err
	}
	if err := client.Lock(gateContext, config.VolumeInstanceID, backup.ID); err != nil {
		return fmt.Errorf("lock Railway pre-migration backup %q: %w", backup.ID, err)
	}
	if err := verifyPostLockBackup(gateContext, config.VolumeInstanceID, backup, client); err != nil {
		return err
	}

	slog.InfoContext(ctx, "Railway pre-migration backup verified",
		"project_id", config.ProjectID,
		"environment_id", config.EnvironmentID,
		"volume_instance_id", config.VolumeInstanceID,
		"postgres_service_id", config.PostgresServiceID,
		"application_sha", config.ApplicationSHA,
		"backup_workflow_id", created.WorkflowID,
		"backup_id", backup.ID,
		"backup_name", backup.Name,
		"backup_created_at", backup.CreatedAt,
		"backup_external_id", backup.ExternalID,
		"backup_referenced_mb", *backup.ReferencedMB,
		"affected_migrations", affected,
	)
	if config.beforeMigrations != nil {
		if err := config.beforeMigrations(ctx); err != nil {
			return fmt.Errorf("before migration execution: %w", err)
		}
	}
	return runMigrations(ctx)
}

func formatAffectedMigrations(affected []AffectedMigration) string {
	items := make([]string, 0, len(affected))
	for _, migration := range affected {
		items = append(items, fmt.Sprintf("version=%d rows=%d", migration.Version, migration.Rows))
	}
	return strings.Join(items, ", ")
}

func validateMigrationGateConfig(config MigrationGateConfig) error {
	missing := make([]string, 0, 7)
	for name, value := range map[string]string{
		"project ID":             config.ProjectID,
		"environment ID":         config.EnvironmentID,
		"volume instance ID":     config.VolumeInstanceID,
		"Postgres service ID":    config.PostgresServiceID,
		"application service ID": config.ApplicationServiceID,
		"application SHA":        config.ApplicationSHA,
		"runtime DATABASE_URL":   config.databaseURL,
	} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("Railway backup configuration is missing %s", strings.Join(missing, ", "))
	}
	return nil
}

func waitForStableNewBackup(
	ctx context.Context,
	config MigrationGateConfig,
	client railwaybackup.Client,
	backupName string,
	existingIDs map[string]struct{},
) (railwaybackup.Backup, error) {
	pollInterval := config.PollInterval
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	var previous *railwaybackup.Backup
	for {
		backups, err := client.List(ctx, config.VolumeInstanceID)
		if err != nil {
			return railwaybackup.Backup{}, fmt.Errorf("poll Railway pre-migration backup: %w", err)
		}
		matching := make([]railwaybackup.Backup, 0, 1)
		for _, backup := range backups {
			if backup.Name == backupName {
				matching = append(matching, backup)
			}
		}
		if len(matching) > 1 {
			return railwaybackup.Backup{}, fmt.Errorf("Railway pre-migration backup evidence is ambiguous for name %q", backupName)
		}
		if len(matching) == 1 {
			candidate := matching[0]
			_, existedBefore := existingIDs[candidate.ID]
			if !existedBefore && backupReady(candidate) {
				if previous != nil {
					if candidate.ID == previous.ID && !reflect.DeepEqual(candidate, *previous) {
						return railwaybackup.Backup{}, fmt.Errorf("Railway pre-migration backup readiness metadata is unstable for ID %q", candidate.ID)
					}
					if reflect.DeepEqual(candidate, *previous) {
						return candidate, nil
					}
				}
				copy := candidate
				previous = &copy
			}
		}

		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return railwaybackup.Backup{}, fmt.Errorf("timed out waiting for Railway pre-migration backup %q: %w", backupName, ctx.Err())
		case <-timer.C:
		}
	}
}

func verifyPostLockBackup(
	ctx context.Context,
	volumeInstanceID string,
	want railwaybackup.Backup,
	client railwaybackup.Client,
) error {
	backups, err := client.List(ctx, volumeInstanceID)
	if err != nil {
		return fmt.Errorf("post-lock Railway backup reread failed: %w", err)
	}
	for _, backup := range backups {
		if backup.ID == want.ID && backup.Name == want.Name {
			if !reflect.DeepEqual(backup, want) {
				return fmt.Errorf("post-lock Railway backup evidence changed for ID %q", want.ID)
			}
			return nil
		}
	}
	return fmt.Errorf("post-lock Railway backup %q is missing", want.ID)
}

func backupReady(backup railwaybackup.Backup) bool {
	return backup.ID != "" &&
		backup.Name != "" &&
		backup.CreatedAt != "" &&
		backup.ExternalID != "" &&
		backup.ReferencedMB != nil
}

func migrationBackupName(config MigrationGateConfig, affected []AffectedMigration) string {
	versions := make([]string, 0, len(affected))
	for _, migration := range affected {
		versions = append(versions, fmt.Sprintf("%d", migration.Version))
	}
	suffix := ""
	if config.attemptSuffix != nil {
		suffix = config.attemptSuffix()
	} else {
		suffix = randomBackupSuffix()
	}
	name := fmt.Sprintf(
		"unipost-%s-%s-m%s-%s",
		shortIdentifier(config.EnvironmentID, 8),
		shortIdentifier(config.ApplicationSHA, 8),
		strings.Join(versions, "-"),
		shortIdentifier(suffix, 12),
	)
	if len(name) > 64 {
		return name[:64]
	}
	return name
}

func shortIdentifier(value string, limit int) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			if builder.Len() == limit {
				break
			}
		}
	}
	if builder.Len() == 0 {
		return "unknown"
	}
	return builder.String()
}

func randomBackupSuffix() string {
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("time%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}
