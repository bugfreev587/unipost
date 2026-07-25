package db

import (
	"os"
	"strings"
	"testing"
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
				"where id = sqlc.arg(id)",
				"and workspace_id = sqlc.arg(workspace_id)",
				"and plan_id = sqlc.arg(plan_id)",
				"and status = 'pending_activation'",
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
