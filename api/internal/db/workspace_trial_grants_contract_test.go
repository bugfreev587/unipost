package db

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestWorkspaceTrialGrantMigrationContract(t *testing.T) {
	source, err := os.ReadFile("migrations/121_workspace_trial_grants.sql")
	if err != nil {
		t.Fatalf("read workspace trial grants migration: %v", err)
	}

	sql := strings.ToLower(strings.Join(strings.Fields(string(source)), " "))
	for _, want := range []string{
		"create table workspace_trial_grants",
		"id text primary key default gen_random_uuid()::text",
		"workspace_id text not null references workspaces(id) on delete cascade",
		"kind text not null check (kind in ('free_to_paid', 'paid_same_plan'))",
		"plan_id text not null references plans(id)",
		"duration_days integer not null check (duration_days between 1 and 730)",
		"status text not null check (status in ('provisioning', 'pending_activation', 'checkout_pending', 'scheduled', 'active', 'completed', 'canceled', 'revoked', 'superseded', 'failed'))",
		"granted_by_user_id text not null",
		"stripe_mode text",
		"stripe_customer_id text",
		"stripe_subscription_id text",
		"stripe_schedule_id text",
		"stripe_checkout_session_id text",
		"checkout_attempt integer not null default 0 check (checkout_attempt >= 0)",
		"granted_at timestamptz not null",
		"scheduled_start_at timestamptz",
		"started_at timestamptz",
		"ends_at timestamptz",
		"activated_at timestamptz",
		"canceled_at timestamptz",
		"revoked_at timestamptz",
		"superseded_at timestamptz",
		"completed_at timestamptz",
		"superseded_by_plan_id text references plans(id)",
		"failure_code text",
		"failure_message text",
		"created_at timestamptz not null default now()",
		"updated_at timestamptz not null default now()",
		"create unique index workspace_trial_grants_open_workspace_idx on workspace_trial_grants (workspace_id) where status in ('provisioning', 'pending_activation', 'checkout_pending', 'scheduled', 'active')",
		"create unique index workspace_trial_grants_checkout_session_idx on workspace_trial_grants (stripe_checkout_session_id) where stripe_checkout_session_id is not null",
		"create index workspace_trial_grants_subscription_idx on workspace_trial_grants (stripe_subscription_id)",
		"create index workspace_trial_grants_schedule_idx on workspace_trial_grants (stripe_schedule_id)",
		"create index workspace_trial_grants_workspace_history_idx on workspace_trial_grants (workspace_id, granted_at desc)",
		"create index workspace_trial_grants_status_ends_at_idx on workspace_trial_grants (status, ends_at)",
		"-- +goose down drop table workspace_trial_grants",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("workspace trial grants migration missing %q:\n%s", want, string(source))
		}
	}
	if strings.Contains(sql, "references users(id)") {
		t.Fatal("granted_by_user_id must remain an opaque actor ID so deleting the actor does not delete or block the grant")
	}
}

func TestWorkspaceTrialGrantKeepsOpaqueGrantingActorAfterUserDeletion(t *testing.T) {
	databaseURL := os.Getenv("X_INBOX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("X_INBOX_TEST_DATABASE_URL is not configured")
	}

	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	ctx := context.Background()
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	bootstrapMigrationBaselineIfEmptyForTest(t, ctx, tx, 120)
	var hasTrialGrants bool
	if err := tx.QueryRowContext(ctx, `
		SELECT to_regclass('public.workspace_trial_grants') IS NOT NULL
	`).Scan(&hasTrialGrants); err != nil {
		t.Fatal(err)
	}
	if !hasTrialGrants {
		applyMigrationUp(t, ctx, tx, "migrations/121_workspace_trial_grants.sql")
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	ownerID := "trial_grant_owner_" + suffix
	adminID := "trial_grant_admin_" + suffix
	workspaceID := "trial_grant_workspace_" + suffix
	planID := "trial_grant_plan_" + suffix
	grantID := "trial_grant_" + suffix

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (id, email, name) VALUES
		  ($1, $2, 'Trial Grant Owner'),
		  ($3, $4, 'Trial Grant Admin')
	`, ownerID, ownerID+"@example.com", adminID, adminID+"@example.com"); err != nil {
		t.Fatalf("insert trial grant users: %v", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workspaces (id, user_id, name) VALUES ($1, $2, 'Trial Grant Test')
	`, workspaceID, ownerID); err != nil {
		t.Fatalf("insert trial grant workspace: %v", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO plans (id, name, price_cents, post_limit) VALUES ($1, 'Trial Grant Test', 100, 100)
	`, planID); err != nil {
		t.Fatalf("insert trial grant plan: %v", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workspace_trial_grants (
		  id, workspace_id, kind, plan_id, duration_days, status, granted_by_user_id
		) VALUES ($1, $2, 'free_to_paid', $3, 14, 'pending_activation', $4)
	`, grantID, workspaceID, planID, adminID); err != nil {
		t.Fatalf("insert workspace trial grant: %v", err)
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM users WHERE id = $1", adminID); err != nil {
		t.Fatalf("delete granting actor: %v", err)
	}

	var grantedByUserID string
	if err := tx.QueryRowContext(ctx, `
		SELECT granted_by_user_id FROM workspace_trial_grants WHERE id = $1
	`, grantID).Scan(&grantedByUserID); err != nil {
		t.Fatalf("read retained workspace trial grant: %v", err)
	}
	if grantedByUserID != adminID {
		t.Fatalf("retained granted_by_user_id = %q, want %q", grantedByUserID, adminID)
	}
}

func TestWorkspaceTrialGrantCheckoutQueriesUseExactCorrelation(t *testing.T) {
	source, err := os.ReadFile("queries/workspace_trial_grants.sql")
	if err != nil {
		t.Fatalf("read workspace trial grant queries: %v", err)
	}

	queries := strings.ToLower(strings.Join(strings.Fields(string(source)), " "))
	tests := []struct {
		name      string
		queryName string
		want      []string
	}{
		{
			name:      "checkout claim matches workspace and plan",
			queryName: "-- name: claimworkspacetrialgrantcheckout :one",
			want: []string{
				"checkout_attempt = checkout_attempt + 1",
				"stripe_customer_id = sqlc.arg(stripe_customer_id)",
				"where id = sqlc.arg(id)",
				"and workspace_id = sqlc.arg(workspace_id)",
				"and plan_id = sqlc.arg(plan_id)",
				"and sqlc.arg(stripe_customer_id)::text <> ''",
				"and status = 'pending_activation'",
			},
		},
		{
			name:      "checkout session record is fenced by attempt",
			queryName: "-- name: recordworkspacetrialgrantcheckoutsession :one",
			want: []string{
				"where id = sqlc.arg(id)",
				"and status = 'checkout_pending'",
				"and stripe_checkout_session_id is null",
				"and checkout_attempt = sqlc.arg(expected_checkout_attempt)",
			},
		},
		{
			name:      "expired checkout release matches session",
			queryName: "-- name: releaseexpiredworkspacetrialgrantcheckout :one",
			want: []string{
				"where id = sqlc.arg(id)",
				"and status = 'checkout_pending'",
				"and stripe_checkout_session_id = sqlc.arg(stripe_checkout_session_id)",
			},
		},
		{
			name:      "rejected checkout reopens only an unrecorded claim",
			queryName: "-- name: reopenunrecordedworkspacetrialgrantcheckout :one",
			want: []string{
				"where id = sqlc.arg(id)",
				"and status = 'checkout_pending'",
				"and stripe_checkout_session_id is null",
				"and checkout_attempt = sqlc.arg(expected_checkout_attempt)",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start := strings.Index(queries, test.queryName)
			if start < 0 {
				t.Fatalf("missing query %q", test.queryName)
			}
			section := queries[start:]
			if next := strings.Index(section[len(test.queryName):], "-- name:"); next >= 0 {
				section = section[:len(test.queryName)+next]
			}
			for _, want := range test.want {
				if !strings.Contains(section, want) {
					t.Errorf("query %q missing %q: %s", test.queryName, want, section)
				}
			}
		})
	}
}

func TestWorkspaceTrialGrantPersistsPartialScheduleWhileProvisioning(t *testing.T) {
	source, err := os.ReadFile("queries/workspace_trial_grants.sql")
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(strings.Join(strings.Fields(string(source)), " "))
	for _, want := range []string{
		"-- name: recordworkspacetrialgrantprovisioningschedule :one",
		"failure_code = sqlc.arg(failure_code)",
		"and status = 'provisioning'",
		"coalesce(sqlc.narg(stripe_schedule_id), stripe_schedule_id)",
		"and (sqlc.narg(stripe_schedule_id) is null or stripe_schedule_id is null or stripe_schedule_id = sqlc.narg(stripe_schedule_id))",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("partial schedule query missing %q", want)
		}
	}
}

func TestWorkspaceTrialGrantRenewalCancellationKeepsGrantActive(t *testing.T) {
	source, err := os.ReadFile("queries/workspace_trial_grants.sql")
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(strings.Join(strings.Fields(string(source)), " "))
	start := strings.Index(body, "-- name: recordworkspacetrialgrantrenewalcancellation :one")
	if start < 0 {
		t.Fatal("missing renewal cancellation query")
	}
	section := body[start:]
	if next := strings.Index(section[len("-- name: recordworkspacetrialgrantrenewalcancellation :one"):], "-- name:"); next >= 0 {
		section = section[:len("-- name: recordworkspacetrialgrantrenewalcancellation :one")+next]
	}
	for _, want := range []string{"set canceled_at = sqlc.arg(canceled_at)", "and status = 'active'", "returning *"} {
		if !strings.Contains(section, want) {
			t.Errorf("renewal cancellation query missing %q: %s", want, section)
		}
	}
	if strings.Contains(section, "set status") {
		t.Fatalf("renewal cancellation must not make the grant terminal: %s", section)
	}
}
