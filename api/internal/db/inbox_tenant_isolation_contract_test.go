package db

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func readInboxTenantIsolationContractFile(t *testing.T, path string) string {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

func compactInboxTenantIsolationSQL(sql string) string {
	return strings.Join(strings.Fields(strings.ToLower(sql)), " ")
}

func inboxTenantIsolationQuery(t *testing.T, source, name string) string {
	t.Helper()

	marker := "-- name: " + name + " "
	start := strings.Index(source, marker)
	if start < 0 {
		t.Fatalf("query %s is missing", name)
	}
	rest := source[start:]
	if next := strings.Index(rest[len(marker):], "-- name: "); next >= 0 {
		rest = rest[:len(marker)+next]
	}
	return compactInboxTenantIsolationSQL(rest)
}

func TestInboxTenantIsolationMigration119PreservesEvidence(t *testing.T) {
	source := readInboxTenantIsolationContractFile(t, "migrations/119_inbox_tenant_isolation.sql")
	parts := strings.Split(source, "-- +goose Down")
	if len(parts) != 2 {
		t.Fatalf("migration must have exactly one Goose Down section, got %d", len(parts)-1)
	}

	up := compactInboxTenantIsolationSQL(parts[0])
	down := compactInboxTenantIsolationSQL(parts[1])
	if !strings.Contains(up, "-- +goose no transaction") {
		t.Fatal("migration must use Goose NO TRANSACTION for concurrent indexes")
	}

	createTableStart := strings.Index(up, "create table inbox_item_quarantine (")
	if createTableStart < 0 {
		t.Fatal("migration must create inbox_item_quarantine")
	}
	createTableEnd := strings.Index(up[createTableStart:], ");")
	if createTableEnd < 0 {
		t.Fatal("inbox_item_quarantine definition is incomplete")
	}
	tableDefinition := up[createTableStart : createTableStart+createTableEnd]
	for _, want := range []string{
		"id text primary key default gen_random_uuid()::text",
		"incident_key text not null",
		"original_inbox_item_id text not null",
		"source text not null",
		"external_id text not null",
		"social_account_id text not null",
		"workspace_id text not null",
		"account_external_id text not null",
		"original_row jsonb not null check (jsonb_typeof(original_row) = 'object')",
		"quarantined_at timestamptz not null default now()",
		"unique (incident_key, original_inbox_item_id)",
	} {
		if !strings.Contains(tableDefinition, want) {
			t.Errorf("inbox_item_quarantine missing %q", want)
		}
	}
	if strings.Contains(tableDefinition, "references ") || strings.Contains(tableDefinition, "foreign key") {
		t.Fatal("inbox_item_quarantine must not have foreign keys so incident evidence survives source-row cleanup")
	}

	mutatesInboxItems := regexp.MustCompile(`(?i)\b(insert\s+into|update|delete\s+from)\s+inbox_items\b`)
	if mutatesInboxItems.MatchString(parts[0]) {
		t.Fatal("migration Up must not mutate inbox_items")
	}

	for _, want := range []string{
		"create index concurrently if not exists social_accounts_active_instagram_webhook_user_id_idx",
		"on social_accounts ((metadata->>'instagram_webhook_user_id'))",
		"where platform = 'instagram' and status = 'active' and disconnected_at is null",
		"create index concurrently if not exists social_accounts_active_platform_external_account_id_idx",
		"on social_accounts (platform, external_account_id)",
		"where status = 'active' and disconnected_at is null",
	} {
		if !strings.Contains(up, want) {
			t.Errorf("migration Up missing %q", want)
		}
	}
	if strings.Contains(up, "create unique index") {
		t.Fatal("routing indexes must be non-unique because an external identity may map to multiple workspaces")
	}

	for _, want := range []string{
		"if exists (select 1 from inbox_item_quarantine)",
		"raise exception",
		"drop index concurrently if exists social_accounts_active_instagram_webhook_user_id_idx",
		"drop index concurrently if exists social_accounts_active_platform_external_account_id_idx",
		"drop table inbox_item_quarantine",
	} {
		if !strings.Contains(down, want) {
			t.Errorf("migration Down missing %q", want)
		}
	}
	guardAt := strings.Index(down, "if exists (select 1 from inbox_item_quarantine)")
	dropAt := strings.Index(down, "drop table inbox_item_quarantine")
	if guardAt < 0 || dropAt < 0 || guardAt > dropAt {
		t.Fatal("migration Down must refuse non-empty evidence before dropping inbox_item_quarantine")
	}
	if strings.Contains(down, "drop table inbox_item_quarantine cascade") {
		t.Fatal("migration Down must not cascade evidence-table deletion")
	}
}

func TestInboxTenantIsolationAuthenticatedQueriesDeriveWorkspace(t *testing.T) {
	source := readInboxTenantIsolationContractFile(t, "queries/inbox.sql")
	tests := []struct {
		name          string
		workspaceExpr string
		mutation      bool
	}{
		{name: "ListInboxItemsByWorkspace", workspaceExpr: "$1"},
		{name: "GetInboxItem", workspaceExpr: "$2"},
		{name: "MarkInboxItemRead", workspaceExpr: "$2", mutation: true},
		{name: "UpdateInboxItemAuthorMetadata", workspaceExpr: "@workspace_id", mutation: true},
		{name: "MarkAllInboxItemsRead", workspaceExpr: "@workspace_id", mutation: true},
		{name: "UpdateInboxThreadState", workspaceExpr: "$1", mutation: true},
		{name: "CountUnreadByWorkspace", workspaceExpr: "$1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := inboxTenantIsolationQuery(t, source, tt.name)
			for _, want := range []string{
				"social_accounts sa",
				"profiles p",
				"p.id = sa.profile_id",
				"sa.id = i.social_account_id",
				"i.workspace_id = " + tt.workspaceExpr,
				"p.workspace_id = " + tt.workspaceExpr,
			} {
				if !strings.Contains(query, want) {
					t.Errorf("%s must fail closed on stored and derived workspace identity; missing %q in %s", tt.name, want, query)
				}
			}
			if tt.mutation && !strings.Contains(query, " from social_accounts sa") {
				t.Errorf("%s must authorize through UPDATE ... FROM, got %s", tt.name, query)
			}
		})
	}

	for _, name := range []string{"ListInboxItemsByWorkspace", "CountUnreadByWorkspace"} {
		query := inboxTenantIsolationQuery(t, source, name)
		for _, want := range []string{"sa.status = 'active'", "sa.disconnected_at is null"} {
			if !strings.Contains(query, want) {
				t.Errorf("%s must retain existing active-account filter %q", name, want)
			}
		}
	}
}

func TestInboxTenantIsolationWebhookRoutingQueriesAreExact(t *testing.T) {
	source := readInboxTenantIsolationContractFile(t, "queries/inbox.sql")

	for _, legacy := range []string{
		"FindAnyActiveAccountByPlatform",
		"FindAllActiveAccountsByPlatform",
		"FindSocialAccountByPlatformAndExternalID",
	} {
		query := inboxTenantIsolationQuery(t, source, legacy)
		if !strings.Contains(query, "deprecated") || !strings.Contains(query, "unsafe") {
			t.Errorf("temporarily retained legacy query %s must be marked deprecated and unsafe", legacy)
		}
	}

	instagram := inboxTenantIsolationQuery(t, source, "FindAllActiveInstagramAccountsByWebhookUserID")
	for _, want := range []string{
		":many",
		"select sa.id, sa.external_account_id",
		"cast(coalesce(sa.metadata->>'instagram_webhook_user_id', '') as text) as instagram_webhook_user_id",
		"p.workspace_id",
		"sa.platform = 'instagram'",
		"sa.metadata->>'instagram_webhook_user_id' = @instagram_webhook_user_id::text",
		"sa.disconnected_at is null",
		"sa.status = 'active'",
		"order by sa.connected_at desc, sa.id",
	} {
		if !strings.Contains(instagram, want) {
			t.Errorf("Instagram webhook routing query missing %q in %s", want, instagram)
		}
	}

	exact := inboxTenantIsolationQuery(t, source, "FindAllSocialAccountsByPlatformAndExternalID")
	for _, want := range []string{
		":many",
		"sa.platform = $1",
		"sa.external_account_id = $2",
		"sa.disconnected_at is null",
		"sa.status = 'active'",
	} {
		if !strings.Contains(exact, want) {
			t.Errorf("exact platform routing query missing %q in %s", want, exact)
		}
	}

	accounts := inboxTenantIsolationQuery(t, source, "ListAllInboxAccounts")
	if !strings.Contains(accounts, "cast(coalesce(sa.metadata->>'instagram_webhook_user_id', '') as text) as instagram_webhook_user_id") {
		t.Fatal("ListAllInboxAccounts must expose instagram_webhook_user_id")
	}
}

func TestInboxTenantIsolationCanPersistInstagramWebhookIdentity(t *testing.T) {
	source := readInboxTenantIsolationContractFile(t, "queries/social_accounts.sql")
	query := inboxTenantIsolationQuery(t, source, "SetInstagramWebhookUserID")
	for _, want := range []string{
		":execrows",
		"update social_accounts",
		"set metadata = coalesce(metadata, '{}'::jsonb) || jsonb_build_object('instagram_webhook_user_id', @instagram_webhook_user_id::text)",
		"where id = @id",
		"platform = 'instagram'",
		"status = 'active'",
		"disconnected_at is null",
	} {
		if !strings.Contains(query, want) {
			t.Errorf("SetInstagramWebhookUserID missing %q in %s", want, query)
		}
	}
	if strings.Contains(query, "is distinct from") {
		t.Fatal("SetInstagramWebhookUserID must report every matching write without a distinct-value condition")
	}
}

func TestInboxTenantIsolationGeneratedWebhookIdentitiesAreStrings(t *testing.T) {
	generated := readInboxTenantIsolationContractFile(t, "inbox.sql.go")
	if strings.Contains(generated, "InstagramWebhookUserID interface{}") {
		t.Fatal("sqlc must expose projected Instagram webhook identities as strings, not interface{}")
	}
	if got := strings.Count(generated, "InstagramWebhookUserID string"); got != 2 {
		t.Fatalf("generated Inbox routing rows contain %d string webhook identity fields, want 2", got)
	}
}
