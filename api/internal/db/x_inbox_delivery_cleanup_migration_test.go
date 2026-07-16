package db

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestXInboxDeliveryCleanupMigrationCapturesWorkspaceCascadeBeforeChildrenDisappear(t *testing.T) {
	databaseURL := os.Getenv("X_INBOX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("X_INBOX_TEST_DATABASE_URL is not configured")
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	var hasXAppMode bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'social_accounts'
			  AND column_name = 'x_app_mode'
		)
	`).Scan(&hasXAppMode); err != nil {
		t.Fatal(err)
	}
	if !hasXAppMode {
		applyMigrationUp(t, ctx, tx, "migrations/108_x_inbox_oauth_and_delivery.sql")
	}

	var hasCleanupTable bool
	if err := tx.QueryRowContext(ctx, `
		SELECT to_regclass('public.x_inbox_delivery_cleanup_intents') IS NOT NULL
	`).Scan(&hasCleanupTable); err != nil {
		t.Fatal(err)
	}
	if !hasCleanupTable {
		applyMigrationUp(t, ctx, tx, "migrations/110_x_inbox_delivery_cleanup_intents.sql")
	}
	var hasCredentialUpdateTrigger bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_trigger
			WHERE tgrelid = 'platform_credentials'::regclass
			  AND tgname = 'platform_credentials_x_inbox_delivery_replacement_cleanup'
			  AND NOT tgisinternal
			  AND (tgtype & 2) <> 0
			  AND (tgtype & 16) <> 0
		)
	`).Scan(&hasCredentialUpdateTrigger); err != nil {
		t.Fatal(err)
	}
	if !hasCredentialUpdateTrigger {
		applyMigrationUp(t, ctx, tx, "migrations/111_x_inbox_credential_replacement_cleanup.sql")
	}

	var hasWorkspaceTrigger bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_trigger
			WHERE tgrelid = 'workspaces'::regclass
			  AND tgname = 'workspaces_x_inbox_delivery_cleanup'
			  AND NOT tgisinternal
			  AND (tgtype & 2) <> 0
			  AND (tgtype & 8) <> 0
		)
	`).Scan(&hasWorkspaceTrigger); err != nil {
		t.Fatal(err)
	}
	if !hasWorkspaceTrigger {
		t.Fatal("cleanup migration must install a BEFORE DELETE trigger on workspaces")
	}
	for _, column := range []string{
		"cleanup_key",
		"source_app_identity",
		"lease_owner",
		"lease_until",
		"next_attempt_at",
	} {
		var exists bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = 'public'
				  AND table_name = 'x_inbox_delivery_cleanup_intents'
				  AND column_name = $1
			)
		`, column).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("cleanup migration missing %s", column)
		}
	}
	var hasAccountUnique, hasCleanupKeyUnique bool
	if err := tx.QueryRowContext(ctx, `
		SELECT
		  EXISTS (
		    SELECT 1
		    FROM pg_constraint c
		    JOIN unnest(c.conkey) WITH ORDINALITY AS key(attnum, ordinality)
		      ON TRUE
		    JOIN pg_attribute a
		      ON a.attrelid = c.conrelid
		     AND a.attnum = key.attnum
		    WHERE c.conrelid = 'x_inbox_delivery_cleanup_intents'::regclass
		      AND c.contype = 'u'
		    GROUP BY c.oid
		    HAVING array_agg(a.attname ORDER BY key.ordinality)
		      = ARRAY['social_account_id']::name[]
		  ),
		  EXISTS (
		    SELECT 1
		    FROM pg_constraint c
		    JOIN unnest(c.conkey) WITH ORDINALITY AS key(attnum, ordinality)
		      ON TRUE
		    JOIN pg_attribute a
		      ON a.attrelid = c.conrelid
		     AND a.attnum = key.attnum
		    WHERE c.conrelid = 'x_inbox_delivery_cleanup_intents'::regclass
		      AND c.contype = 'u'
		    GROUP BY c.oid
		    HAVING array_agg(a.attname ORDER BY key.ordinality)
		      = ARRAY['cleanup_key']::name[]
		  )
	`).Scan(&hasAccountUnique, &hasCleanupKeyUnique); err != nil {
		t.Fatal(err)
	}
	if hasAccountUnique || !hasCleanupKeyUnique {
		t.Fatalf(
			"cleanup uniqueness = account:%v cleanup_key:%v, want generation key only",
			hasAccountUnique,
			hasCleanupKeyUnique,
		)
	}

	const (
		userID      = "x-inbox-cascade-test-user"
		workspaceID = "x-inbox-cascade-test-workspace"
		profileID   = "x-inbox-cascade-test-profile"
		accountID   = "x-inbox-cascade-test-account"
	)
	seedStatements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO users (id, email) VALUES ($1, $2)`, []any{userID, userID + "@example.invalid"}},
		{`INSERT INTO workspaces (id, user_id, name) VALUES ($1, $2, 'X Inbox Cascade Test')`, []any{workspaceID, userID}},
		{`INSERT INTO profiles (id, workspace_id, name) VALUES ($1, $2, 'X Inbox Cascade Test')`, []any{profileID, workspaceID}},
		{`
			INSERT INTO platform_credentials (
				id, workspace_id, platform, client_id, client_secret, app_bearer_token
			) VALUES (
				'x-inbox-cascade-test-credential', $1, 'twitter', 'client', 'secret',
				'encrypted-workspace-bearer'
			)
		`, []any{workspaceID}},
		{`
			INSERT INTO social_accounts (
				id, profile_id, platform, access_token, external_account_id, status,
				connection_type, x_app_mode
			) VALUES (
				$1, $2, 'twitter', 'encrypted-user-token', 'x-user', 'active',
				'managed', 'workspace_x_app'
			)
		`, []any{accountID, profileID}},
		{`
			INSERT INTO x_inbox_delivery_resources (
				social_account_id, filtered_stream_rule_id, activity_dm_subscription_id
			) VALUES ($1, 'rule-exact', 'subscription-exact')
		`, []any{accountID}},
	}
	for _, statement := range seedStatements {
		if _, err := tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed workspace topology: %v", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM workspaces WHERE id = $1`, workspaceID); err != nil {
		t.Fatalf("delete workspace: %v", err)
	}

	var (
		appMode        string
		appBearer      string
		ruleID         string
		subscriptionID string
	)
	if err := tx.QueryRowContext(ctx, `
		SELECT x_app_mode, app_bearer_token, filtered_stream_rule_id, activity_dm_subscription_id
		FROM x_inbox_delivery_cleanup_intents
		WHERE social_account_id = $1
	`, accountID).Scan(&appMode, &appBearer, &ruleID, &subscriptionID); err != nil {
		t.Fatalf("query workspace cleanup intent: %v", err)
	}
	if appMode != "workspace_x_app" ||
		appBearer != "encrypted-workspace-bearer" ||
		ruleID != "rule-exact" ||
		subscriptionID != "subscription-exact" {
		t.Fatalf(
			"cleanup intent = mode %q bearer %q rule %q subscription %q",
			appMode,
			appBearer,
			ruleID,
			subscriptionID,
		)
	}
	var workspaceCleanupCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM x_inbox_delivery_cleanup_intents
		WHERE social_account_id = $1
	`, accountID).Scan(&workspaceCleanupCount); err != nil {
		t.Fatalf("count workspace cleanup generations: %v", err)
	}
	if workspaceCleanupCount != 1 {
		t.Fatalf(
			"workspace cleanup generations = %d, want duplicate cascade triggers idempotent",
			workspaceCleanupCount,
		)
	}

	const (
		credentialWorkspaceID = "x-inbox-credential-test-workspace"
		credentialProfileID   = "x-inbox-credential-test-profile"
		credentialAccountID   = "x-inbox-credential-test-account"
	)
	credentialStatements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO workspaces (id, user_id, name) VALUES ($1, $2, 'X Credential Test')`, []any{credentialWorkspaceID, userID}},
		{`INSERT INTO profiles (id, workspace_id, name) VALUES ($1, $2, 'X Credential Test')`, []any{credentialProfileID, credentialWorkspaceID}},
		{`
			INSERT INTO platform_credentials (
				id, workspace_id, platform, client_id, client_secret, app_bearer_token
			) VALUES (
				'x-inbox-credential-test-credential', $1, 'twitter', 'client', 'secret',
				'encrypted-credential-bearer'
			)
		`, []any{credentialWorkspaceID}},
		{`
			INSERT INTO social_accounts (
				id, profile_id, platform, access_token, external_account_id, status,
				connection_type, x_app_mode
			) VALUES (
				$1, $2, 'twitter', 'encrypted-user-token', 'x-user-credential', 'active',
				'managed', 'workspace_x_app'
			)
		`, []any{credentialAccountID, credentialProfileID}},
		{`
			INSERT INTO x_inbox_delivery_resources (
				social_account_id, filtered_stream_rule_id, activity_dm_subscription_id,
				delivery_status
			) VALUES ($1, 'credential-rule', 'credential-subscription', 'active')
		`, []any{credentialAccountID}},
	}
	for _, statement := range credentialStatements {
		if _, err := tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed credential deletion topology: %v", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM platform_credentials
		WHERE workspace_id = $1 AND platform = 'twitter'
	`, credentialWorkspaceID); err != nil {
		t.Fatalf("delete workspace X credential: %v", err)
	}

	if err := tx.QueryRowContext(ctx, `
		SELECT app_bearer_token, filtered_stream_rule_id, activity_dm_subscription_id
		FROM x_inbox_delivery_cleanup_intents
		WHERE social_account_id = $1
	`, credentialAccountID).Scan(&appBearer, &ruleID, &subscriptionID); err != nil {
		t.Fatalf("query credential cleanup intent: %v", err)
	}
	if appBearer != "encrypted-credential-bearer" ||
		ruleID != "credential-rule" ||
		subscriptionID != "credential-subscription" {
		t.Fatalf(
			"credential cleanup intent = bearer %q rule %q subscription %q",
			appBearer,
			ruleID,
			subscriptionID,
		)
	}
	var (
		localRule         sql.NullString
		localSubscription sql.NullString
		deliveryStatus    string
		lastError         sql.NullString
	)
	if err := tx.QueryRowContext(ctx, `
		SELECT filtered_stream_rule_id, activity_dm_subscription_id, delivery_status, last_error
		FROM x_inbox_delivery_resources
		WHERE social_account_id = $1
	`, credentialAccountID).Scan(
		&localRule,
		&localSubscription,
		&deliveryStatus,
		&lastError,
	); err != nil {
		t.Fatalf("query credential-cleared delivery state: %v", err)
	}
	if localRule.Valid || localSubscription.Valid ||
		deliveryStatus != "error" ||
		!lastError.Valid ||
		!strings.Contains(lastError.String, "credential") {
		t.Fatalf(
			"credential-cleared state = rule %v subscription %v status %q error %q",
			localRule,
			localSubscription,
			deliveryStatus,
			lastError.String,
		)
	}

	const (
		replacementWorkspaceID = "x-inbox-replacement-test-workspace"
		replacementProfileID   = "x-inbox-replacement-test-profile"
		replacementAccountID   = "x-inbox-replacement-test-account"
	)
	replacementStatements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO workspaces (id, user_id, name) VALUES ($1, $2, 'X Replacement Test')`, []any{replacementWorkspaceID, userID}},
		{`INSERT INTO profiles (id, workspace_id, name) VALUES ($1, $2, 'X Replacement Test')`, []any{replacementProfileID, replacementWorkspaceID}},
		{`
			INSERT INTO platform_credentials (
				id, workspace_id, platform, client_id, client_secret,
				app_bearer_token, consumer_secret
			) VALUES (
				'x-inbox-replacement-test-credential', $1, 'twitter',
				'old-client', 'old-client-secret',
				'old-encrypted-bearer', 'old-encrypted-consumer'
			)
		`, []any{replacementWorkspaceID}},
		{`
			INSERT INTO social_accounts (
				id, profile_id, platform, access_token, external_account_id, status,
				connection_type, x_app_mode
			) VALUES (
				$1, $2, 'twitter', 'encrypted-user-token', 'x-user-replacement', 'active',
				'managed', 'workspace_x_app'
			)
		`, []any{replacementAccountID, replacementProfileID}},
		{`
			INSERT INTO x_inbox_delivery_resources (
				social_account_id, filtered_stream_rule_id, activity_dm_subscription_id,
				delivery_status
			) VALUES ($1, 'old-exact-rule', 'old-exact-subscription', 'active')
		`, []any{replacementAccountID}},
	}
	for _, statement := range replacementStatements {
		if _, err := tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed credential replacement topology: %v", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE platform_credentials
		SET client_id = 'new-client',
		    client_secret = 'new-client-secret',
		    app_bearer_token = 'new-encrypted-bearer',
		    consumer_secret = 'new-encrypted-consumer'
		WHERE workspace_id = $1 AND platform = 'twitter'
	`, replacementWorkspaceID); err != nil {
		t.Fatalf("replace workspace X credential: %v", err)
	}
	if err := tx.QueryRowContext(ctx, `
		SELECT app_bearer_token, filtered_stream_rule_id, activity_dm_subscription_id
		FROM x_inbox_delivery_cleanup_intents
		WHERE social_account_id = $1
	`, replacementAccountID).Scan(&appBearer, &ruleID, &subscriptionID); err != nil {
		t.Fatalf("query replacement cleanup intent: %v", err)
	}
	if appBearer != "old-encrypted-bearer" ||
		ruleID != "old-exact-rule" ||
		subscriptionID != "old-exact-subscription" {
		t.Fatalf(
			"replacement cleanup intent = bearer %q rule %q subscription %q",
			appBearer,
			ruleID,
			subscriptionID,
		)
	}
	if err := tx.QueryRowContext(ctx, `
		SELECT filtered_stream_rule_id, activity_dm_subscription_id, delivery_status, last_error
		FROM x_inbox_delivery_resources
		WHERE social_account_id = $1
	`, replacementAccountID).Scan(
		&localRule,
		&localSubscription,
		&deliveryStatus,
		&lastError,
	); err != nil {
		t.Fatalf("query replacement-cleared delivery state: %v", err)
	}
	if localRule.Valid || localSubscription.Valid ||
		deliveryStatus != "error" ||
		!lastError.Valid ||
		!strings.Contains(lastError.String, "identity changed") {
		t.Fatalf(
			"replacement-cleared state = rule %v subscription %v status %q error %q",
			localRule,
			localSubscription,
			deliveryStatus,
			lastError.String,
		)
	}
	var (
		clientID       string
		storedBearer   string
		storedConsumer string
	)
	if err := tx.QueryRowContext(ctx, `
		SELECT client_id, app_bearer_token, consumer_secret
		FROM platform_credentials
		WHERE workspace_id = $1 AND platform = 'twitter'
	`, replacementWorkspaceID).Scan(&clientID, &storedBearer, &storedConsumer); err != nil {
		t.Fatalf("query replaced credential: %v", err)
	}
	if clientID != "new-client" ||
		storedBearer != "new-encrypted-bearer" ||
		storedConsumer != "new-encrypted-consumer" {
		t.Fatalf(
			"stored replacement = client %q bearer %q consumer %q",
			clientID,
			storedBearer,
			storedConsumer,
		)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE x_inbox_delivery_cleanup_intents
		SET lease_owner = 'delayed-generation-a',
		    lease_until = NOW() + INTERVAL '1 hour',
		    next_attempt_at = NOW() + INTERVAL '1 hour'
		WHERE social_account_id = $1
		  AND source_app_identity = 'old-client'
	`, replacementAccountID); err != nil {
		t.Fatalf("delay first cleanup generation: %v", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE x_inbox_delivery_resources
		SET filtered_stream_rule_id = 'new-exact-rule',
		    activity_dm_subscription_id = 'new-exact-subscription',
		    delivery_status = 'active',
		    last_error = NULL
		WHERE social_account_id = $1
	`, replacementAccountID); err != nil {
		t.Fatalf("provision replacement app resources: %v", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE platform_credentials
		SET client_id = 'third-client',
		    client_secret = 'third-client-secret',
		    app_bearer_token = 'third-encrypted-bearer',
		    consumer_secret = 'third-encrypted-consumer'
		WHERE workspace_id = $1 AND platform = 'twitter'
	`, replacementWorkspaceID); err != nil {
		t.Fatalf("replace workspace X credential a second time: %v", err)
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT source_app_identity, app_bearer_token,
		       filtered_stream_rule_id, activity_dm_subscription_id,
		       COALESCE(lease_owner, '')
		FROM x_inbox_delivery_cleanup_intents
		WHERE social_account_id = $1
		ORDER BY source_app_identity
	`, replacementAccountID)
	if err != nil {
		t.Fatalf("query cleanup generations: %v", err)
	}
	defer rows.Close()
	type cleanupGeneration struct {
		sourceAppIdentity string
		appBearer         string
		ruleID            string
		subscriptionID    string
		leaseOwner        string
	}
	var generations []cleanupGeneration
	for rows.Next() {
		var generation cleanupGeneration
		if err := rows.Scan(
			&generation.sourceAppIdentity,
			&generation.appBearer,
			&generation.ruleID,
			&generation.subscriptionID,
			&generation.leaseOwner,
		); err != nil {
			t.Fatalf("scan cleanup generation: %v", err)
		}
		generations = append(generations, generation)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate cleanup generations: %v", err)
	}
	wantGenerations := []cleanupGeneration{
		{
			sourceAppIdentity: "new-client",
			appBearer:         "new-encrypted-bearer",
			ruleID:            "new-exact-rule",
			subscriptionID:    "new-exact-subscription",
			leaseOwner:        "",
		},
		{
			sourceAppIdentity: "old-client",
			appBearer:         "old-encrypted-bearer",
			ruleID:            "old-exact-rule",
			subscriptionID:    "old-exact-subscription",
			leaseOwner:        "delayed-generation-a",
		},
	}
	if len(generations) != len(wantGenerations) {
		t.Fatalf("cleanup generations = %+v, want %+v", generations, wantGenerations)
	}
	for i := range wantGenerations {
		if generations[i] != wantGenerations[i] {
			t.Fatalf("cleanup generations = %+v, want %+v", generations, wantGenerations)
		}
	}

	const (
		oldReplicaWorkspaceID = "x-inbox-old-replica-test-workspace"
		oldReplicaProfileID   = "x-inbox-old-replica-test-profile"
		oldReplicaAccountID   = "x-inbox-old-replica-test-account"
	)
	oldReplicaStatements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO workspaces (id, user_id, name) VALUES ($1, $2, 'X Old Replica Test')`, []any{oldReplicaWorkspaceID, userID}},
		{`INSERT INTO profiles (id, workspace_id, name) VALUES ($1, $2, 'X Old Replica Test')`, []any{oldReplicaProfileID, oldReplicaWorkspaceID}},
		{`
			INSERT INTO platform_credentials (
				id, workspace_id, platform, client_id, client_secret,
				app_bearer_token, consumer_secret
			) VALUES (
				'x-inbox-old-replica-test-credential', $1, 'twitter',
				'old-replica-client', 'old-replica-client-secret',
				'old-replica-bearer', 'old-replica-consumer'
			)
		`, []any{oldReplicaWorkspaceID}},
		{`
			INSERT INTO social_accounts (
				id, profile_id, platform, access_token, external_account_id, status,
				connection_type, x_app_mode
			) VALUES (
				$1, $2, 'twitter', 'encrypted-user-token', 'x-user-old-replica', 'active',
				'managed', 'workspace_x_app'
			)
		`, []any{oldReplicaAccountID, oldReplicaProfileID}},
		{`
			INSERT INTO x_inbox_delivery_resources (
				social_account_id, filtered_stream_rule_id, activity_dm_subscription_id,
				delivery_status
			) VALUES ($1, 'old-replica-rule', 'old-replica-subscription', 'active')
		`, []any{oldReplicaAccountID}},
	}
	for _, statement := range oldReplicaStatements {
		if _, err := tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed old replica topology: %v", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO platform_credentials (
		  workspace_id, platform, client_id, client_secret,
		  app_bearer_token, consumer_secret
		)
		VALUES ($1, 'twitter', 'old-replica-new-client', 'new-client-secret', NULL, NULL)
		ON CONFLICT (workspace_id, platform) DO UPDATE
		SET client_id = EXCLUDED.client_id,
		    client_secret = EXCLUDED.client_secret,
		    app_bearer_token = CASE
		      WHEN FALSE THEN EXCLUDED.app_bearer_token
		      ELSE platform_credentials.app_bearer_token
		    END,
		    consumer_secret = CASE
		      WHEN FALSE THEN EXCLUDED.consumer_secret
		      ELSE platform_credentials.consumer_secret
		    END
	`, oldReplicaWorkspaceID); err != nil {
		t.Fatalf("execute old replica credential upsert: %v", err)
	}
	var (
		oldReplicaClientID string
		oldReplicaBearer   sql.NullString
		oldReplicaConsumer sql.NullString
	)
	if err := tx.QueryRowContext(ctx, `
		SELECT client_id, app_bearer_token, consumer_secret
		FROM platform_credentials
		WHERE workspace_id = $1 AND platform = 'twitter'
	`, oldReplicaWorkspaceID).Scan(
		&oldReplicaClientID,
		&oldReplicaBearer,
		&oldReplicaConsumer,
	); err != nil {
		t.Fatalf("query old replica replacement: %v", err)
	}
	if oldReplicaClientID != "old-replica-new-client" ||
		oldReplicaBearer.Valid ||
		oldReplicaConsumer.Valid {
		t.Fatalf(
			"old replica replacement = client %q bearer %v consumer %v",
			oldReplicaClientID,
			oldReplicaBearer,
			oldReplicaConsumer,
		)
	}
	if err := tx.QueryRowContext(ctx, `
		SELECT source_app_identity, app_bearer_token,
		       filtered_stream_rule_id, activity_dm_subscription_id
		FROM x_inbox_delivery_cleanup_intents
		WHERE social_account_id = $1
	`, oldReplicaAccountID).Scan(
		&clientID,
		&appBearer,
		&ruleID,
		&subscriptionID,
	); err != nil {
		t.Fatalf("query old replica cleanup intent: %v", err)
	}
	if clientID != "old-replica-client" ||
		appBearer != "old-replica-bearer" ||
		ruleID != "old-replica-rule" ||
		subscriptionID != "old-replica-subscription" {
		t.Fatalf(
			"old replica cleanup = client %q bearer %q rule %q subscription %q",
			clientID,
			appBearer,
			ruleID,
			subscriptionID,
		)
	}

	const (
		rotationWorkspaceID = "x-inbox-rotation-test-workspace"
		rotationProfileID   = "x-inbox-rotation-test-profile"
		rotationAccountID   = "x-inbox-rotation-test-account"
	)
	rotationStatements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO workspaces (id, user_id, name) VALUES ($1, $2, 'X Rotation Test')`, []any{rotationWorkspaceID, userID}},
		{`INSERT INTO profiles (id, workspace_id, name) VALUES ($1, $2, 'X Rotation Test')`, []any{rotationProfileID, rotationWorkspaceID}},
		{`
			INSERT INTO platform_credentials (
				id, workspace_id, platform, client_id, client_secret,
				app_bearer_token, consumer_secret
			) VALUES (
				'x-inbox-rotation-test-credential', $1, 'twitter',
				'same-client', 'old-client-secret',
				'rotation-old-bearer', 'rotation-old-consumer'
			)
		`, []any{rotationWorkspaceID}},
		{`
			INSERT INTO social_accounts (
				id, profile_id, platform, access_token, external_account_id, status,
				connection_type, x_app_mode
			) VALUES (
				$1, $2, 'twitter', 'encrypted-user-token', 'x-user-rotation', 'active',
				'managed', 'workspace_x_app'
			)
		`, []any{rotationAccountID, rotationProfileID}},
		{`
			INSERT INTO x_inbox_delivery_resources (
				social_account_id, filtered_stream_rule_id, activity_dm_subscription_id,
				delivery_status
			) VALUES ($1, 'rotation-rule', 'rotation-subscription', 'active')
		`, []any{rotationAccountID}},
	}
	for _, statement := range rotationStatements {
		if _, err := tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed same-app rotation topology: %v", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE platform_credentials
		SET client_secret = 'rotated-client-secret',
		    app_bearer_token = 'rotation-new-bearer',
		    consumer_secret = 'rotation-new-consumer'
		WHERE workspace_id = $1 AND platform = 'twitter'
	`, rotationWorkspaceID); err != nil {
		t.Fatalf("rotate same workspace X app secrets: %v", err)
	}
	var rotationIntentCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM x_inbox_delivery_cleanup_intents
		WHERE social_account_id = $1
	`, rotationAccountID).Scan(&rotationIntentCount); err != nil {
		t.Fatalf("count same-app rotation cleanup intents: %v", err)
	}
	if rotationIntentCount != 0 {
		t.Fatalf("same-app rotation cleanup intents = %d, want none", rotationIntentCount)
	}
	if err := tx.QueryRowContext(ctx, `
		SELECT filtered_stream_rule_id, activity_dm_subscription_id, delivery_status
		FROM x_inbox_delivery_resources
		WHERE social_account_id = $1
	`, rotationAccountID).Scan(&ruleID, &subscriptionID, &deliveryStatus); err != nil {
		t.Fatalf("query same-app rotation delivery state: %v", err)
	}
	if ruleID != "rotation-rule" ||
		subscriptionID != "rotation-subscription" ||
		deliveryStatus != "active" {
		t.Fatalf(
			"same-app rotation state = rule %q subscription %q status %q",
			ruleID,
			subscriptionID,
			deliveryStatus,
		)
	}
}

func applyMigrationUp(t *testing.T, ctx context.Context, tx *sql.Tx, path string) {
	t.Helper()
	migration, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	upSQL := strings.Split(string(migration), "-- +goose Down")[0]
	upSQL = strings.Replace(upSQL, "-- +goose Up", "", 1)
	if _, err := tx.ExecContext(ctx, upSQL); err != nil {
		t.Fatalf("apply %s in transaction: %v", path, err)
	}
}
