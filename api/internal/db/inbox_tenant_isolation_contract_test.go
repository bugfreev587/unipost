package db

import (
	"context"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
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
	withoutBlockComments := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(sql, " ")
	lines := strings.Split(withoutBlockComments, "\n")
	for index, line := range lines {
		if commentAt := strings.Index(line, "--"); commentAt >= 0 {
			lines[index] = line[:commentAt]
		}
	}
	return strings.Join(strings.Fields(strings.ToLower(strings.Join(lines, "\n"))), " ")
}

func inboxTenantIsolationRawQuery(t *testing.T, source, name string) string {
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
	return rest
}

func inboxTenantIsolationQuery(t *testing.T, source, name string) string {
	t.Helper()
	return compactInboxTenantIsolationSQL(inboxTenantIsolationRawQuery(t, source, name))
}

func TestInboxTenantIsolationMigration119PreservesEvidence(t *testing.T) {
	source := readInboxTenantIsolationContractFile(t, "migrations/119_inbox_tenant_isolation.sql")
	parts := strings.Split(source, "-- +goose Down")
	if len(parts) != 2 {
		t.Fatalf("migration must have exactly one Goose Down section, got %d", len(parts)-1)
	}

	if !strings.Contains(strings.ToLower(parts[0]), "-- +goose no transaction") {
		t.Fatal("migration must use Goose NO TRANSACTION for concurrent indexes")
	}

	up := compactInboxTenantIsolationSQL(parts[0])
	down := compactInboxTenantIsolationSQL(parts[1])

	createTableStart := strings.Index(up, "create table if not exists inbox_item_quarantine (")
	if createTableStart < 0 {
		t.Fatal("migration must create inbox_item_quarantine idempotently")
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

	mutatesInboxItems := regexp.MustCompile(`\b(insert\s+into|update|delete\s+from)\s+inbox_items\b`)
	if mutatesInboxItems.MatchString(up) {
		t.Fatal("migration Up must not mutate inbox_items")
	}

	indexes := []struct {
		name       string
		definition string
		predicate  string
	}{
		{
			name:       "social_accounts_active_instagram_webhook_user_id_idx",
			definition: "on social_accounts ((metadata->>'instagram_webhook_user_id'))",
			predicate:  "where platform = 'instagram' and status = 'active' and disconnected_at is null",
		},
		{
			name:       "social_accounts_active_platform_external_account_id_idx",
			definition: "on social_accounts (platform, external_account_id)",
			predicate:  "where status = 'active' and disconnected_at is null",
		},
	}
	for _, index := range indexes {
		drop := "drop index concurrently if exists " + index.name
		create := "create index concurrently if not exists " + index.name
		for _, want := range []string{drop, create, index.definition, index.predicate} {
			if !strings.Contains(up, want) {
				t.Errorf("migration Up missing %q", want)
			}
		}
		dropAt := strings.Index(up, drop)
		createAt := strings.Index(up, create)
		if dropAt < 0 || createAt < 0 || dropAt > createAt {
			t.Errorf("migration Up must drop %s before recreating it", index.name)
		}
	}
	if strings.Contains(up, "create unique index") {
		t.Fatal("routing indexes must be non-unique because an external identity may map to multiple workspaces")
	}

	for _, want := range []string{
		"drop index concurrently if exists social_accounts_active_instagram_webhook_user_id_idx",
		"drop index concurrently if exists social_accounts_active_platform_external_account_id_idx",
		"to_regclass('public.inbox_item_quarantine')",
		"execute 'lock table inbox_item_quarantine in access exclusive mode'",
		"execute 'select exists (select 1 from inbox_item_quarantine)' into has_rows",
		"if has_rows then raise exception",
		"execute 'drop table if exists inbox_item_quarantine'",
	} {
		if !strings.Contains(down, want) {
			t.Errorf("migration Down missing executable guard %q", want)
		}
	}
	lockAt := strings.Index(down, "execute 'lock table inbox_item_quarantine in access exclusive mode'")
	checkAt := strings.Index(down, "execute 'select exists (select 1 from inbox_item_quarantine)' into has_rows")
	dropAt := strings.Index(down, "execute 'drop table if exists inbox_item_quarantine'")
	doStart := strings.Index(down, "do $$")
	doEnd := strings.LastIndex(down, "$$;")
	if doStart < 0 || doEnd < 0 || lockAt < doStart || checkAt < lockAt || dropAt < checkAt || dropAt > doEnd {
		t.Fatal("migration Down must lock, inspect, and dynamically drop evidence atomically in one DO statement")
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
		query := strings.ToLower(inboxTenantIsolationRawQuery(t, source, legacy))
		if !strings.Contains(query, "deprecated") || !strings.Contains(query, "unsafe") {
			t.Errorf("temporarily retained legacy query %s must be marked deprecated and unsafe", legacy)
		}
	}

	instagram := inboxTenantIsolationQuery(t, source, "FindAllActiveInstagramAccountsByWebhookUserID")
	if !strings.Contains(source, "-- name: FindAllActiveInstagramAccountsByWebhookUserID :many") {
		t.Fatal("FindAllActiveInstagramAccountsByWebhookUserID must retain its :many annotation")
	}
	for _, want := range []string{
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
	if !strings.Contains(source, "-- name: FindAllSocialAccountsByPlatformAndExternalID :many") {
		t.Fatal("FindAllSocialAccountsByPlatformAndExternalID must retain its :many annotation")
	}
	for _, want := range []string{
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
	if !strings.Contains(source, "-- name: SetInstagramWebhookUserID :execrows") {
		t.Fatal("SetInstagramWebhookUserID must retain its :execrows annotation")
	}
	for _, want := range []string{
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
	var route FindAllActiveInstagramAccountsByWebhookUserIDRow
	var account ListAllInboxAccountsRow
	var _ string = route.InstagramWebhookUserID
	var _ string = account.InstagramWebhookUserID
	var _ func(*Queries, context.Context, string) ([]FindAllActiveInstagramAccountsByWebhookUserIDRow, error) = (*Queries).FindAllActiveInstagramAccountsByWebhookUserID
	var _ func(*Queries, context.Context, SetInstagramWebhookUserIDParams) (int64, error) = (*Queries).SetInstagramWebhookUserID
}

func verifyInboxTenantIsolationAgainstPostgres(t *testing.T, databaseURL string) {
	t.Helper()

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect for Inbox tenant-isolation verification: %v", err)
	}
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin Inbox tenant-isolation verification: %v", err)
	}
	defer tx.Rollback(ctx)

	for _, indexName := range []string{
		"social_accounts_active_instagram_webhook_user_id_idx",
		"social_accounts_active_platform_external_account_id_idx",
	} {
		var valid, unique bool
		err := tx.QueryRow(ctx, `
			SELECT i.indisvalid, i.indisunique
			FROM pg_index i
			JOIN pg_class c ON c.oid = i.indexrelid
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = current_schema()
			  AND c.relname = $1
		`, indexName).Scan(&valid, &unique)
		if err != nil {
			t.Fatalf("inspect routing index %s: %v", indexName, err)
		}
		if !valid || unique {
			t.Fatalf("routing index %s valid=%t unique=%t, want valid non-unique", indexName, valid, unique)
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, email, name)
		VALUES
		  ('inbox-isolation-user-1', 'inbox-isolation-1@example.com', 'Inbox Isolation One'),
		  ('inbox-isolation-user-2', 'inbox-isolation-2@example.com', 'Inbox Isolation Two')
	`); err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO workspaces (id, user_id, name)
		VALUES
		  ('inbox-isolation-workspace-1', 'inbox-isolation-user-1', 'Inbox Isolation One'),
		  ('inbox-isolation-workspace-2', 'inbox-isolation-user-2', 'Inbox Isolation Two')
	`); err != nil {
		t.Fatalf("seed workspaces: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO profiles (id, name, workspace_id)
		VALUES
		  ('inbox-isolation-profile-1', 'Inbox Isolation One', 'inbox-isolation-workspace-1'),
		  ('inbox-isolation-profile-2', 'Inbox Isolation Two', 'inbox-isolation-workspace-2')
	`); err != nil {
		t.Fatalf("seed profiles: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO social_accounts (
		  id, profile_id, platform, access_token, external_account_id, metadata
		)
		VALUES
		  (
		    'inbox-isolation-account-1', 'inbox-isolation-profile-1', 'instagram',
		    'token-1', 'external-1', '{"instagram_webhook_user_id":"webhook-user-1"}'::jsonb
		  ),
		  (
		    'inbox-isolation-account-2', 'inbox-isolation-profile-2', 'instagram',
		    'token-2', 'external-2', '{"instagram_webhook_user_id":"webhook-user-2"}'::jsonb
		  )
	`); err != nil {
		t.Fatalf("seed social accounts: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO inbox_items (
		  id, social_account_id, workspace_id, source, external_id, body, thread_key
		)
		VALUES
		  (
		    'inbox-isolation-valid-1', 'inbox-isolation-account-1',
		    'inbox-isolation-workspace-1', 'ig_comment', 'valid-external-1',
		    'valid one', 'valid-thread-1'
		  ),
		  (
		    'inbox-isolation-valid-2', 'inbox-isolation-account-2',
		    'inbox-isolation-workspace-2', 'ig_comment', 'valid-external-2',
		    'valid two', 'valid-thread-2'
		  ),
		  (
		    'inbox-isolation-forged-1', 'inbox-isolation-account-1',
		    'inbox-isolation-workspace-2', 'ig_comment', 'forged-external-1',
		    'forged one', 'forged-thread-1'
		  ),
		  (
		    'inbox-isolation-forged-2', 'inbox-isolation-account-2',
		    'inbox-isolation-workspace-1', 'ig_comment', 'forged-external-2',
		    'forged two', 'forged-thread-2'
		  )
	`); err != nil {
		t.Fatalf("seed inbox items: %v", err)
	}

	queries := New(tx)
	items, err := queries.ListInboxItemsByWorkspace(ctx, ListInboxItemsByWorkspaceParams{
		WorkspaceID: "inbox-isolation-workspace-2",
		Limit:       20,
	})
	if err != nil {
		t.Fatalf("ListInboxItemsByWorkspace: %v", err)
	}
	if len(items) != 1 || items[0].ID != "inbox-isolation-valid-2" {
		t.Fatalf("workspace-2 Inbox list = %+v, want only its consistent row", items)
	}

	_, err = queries.GetInboxItem(ctx, GetInboxItemParams{
		ID:          "inbox-isolation-forged-1",
		WorkspaceID: "inbox-isolation-workspace-2",
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetInboxItem forged row error = %v, want pgx.ErrNoRows", err)
	}
	item, err := queries.GetInboxItem(ctx, GetInboxItemParams{
		ID:          "inbox-isolation-valid-2",
		WorkspaceID: "inbox-isolation-workspace-2",
	})
	if err != nil || item.ID != "inbox-isolation-valid-2" {
		t.Fatalf("GetInboxItem consistent row = %+v, %v", item, err)
	}

	if err := queries.MarkInboxItemRead(ctx, MarkInboxItemReadParams{
		ID:          "inbox-isolation-forged-1",
		WorkspaceID: "inbox-isolation-workspace-2",
	}); err != nil {
		t.Fatalf("MarkInboxItemRead forged row: %v", err)
	}
	assertRead := func(id string, want bool) {
		t.Helper()
		var got bool
		if err := tx.QueryRow(ctx, "SELECT is_read FROM inbox_items WHERE id = $1", id).Scan(&got); err != nil {
			t.Fatalf("read %s is_read: %v", id, err)
		}
		if got != want {
			t.Fatalf("%s is_read = %t, want %t", id, got, want)
		}
	}
	assertRead("inbox-isolation-forged-1", false)
	if err := queries.MarkInboxItemRead(ctx, MarkInboxItemReadParams{
		ID:          "inbox-isolation-valid-2",
		WorkspaceID: "inbox-isolation-workspace-2",
	}); err != nil {
		t.Fatalf("MarkInboxItemRead consistent row: %v", err)
	}
	assertRead("inbox-isolation-valid-2", true)
	if _, err := tx.Exec(ctx, "UPDATE inbox_items SET is_read = false WHERE id = 'inbox-isolation-valid-2'"); err != nil {
		t.Fatalf("reset consistent row read state: %v", err)
	}

	updated, err := queries.MarkAllInboxItemsRead(ctx, MarkAllInboxItemsReadParams{
		WorkspaceID: "inbox-isolation-workspace-2",
	})
	if err != nil || updated != 1 {
		t.Fatalf("MarkAllInboxItemsRead updated %d rows, error %v; want 1", updated, err)
	}
	assertRead("inbox-isolation-valid-2", true)
	assertRead("inbox-isolation-forged-1", false)

	updated, err = queries.UpdateInboxThreadState(ctx, UpdateInboxThreadStateParams{
		WorkspaceID:     "inbox-isolation-workspace-2",
		SocialAccountID: "inbox-isolation-account-1",
		Source:          "ig_comment",
		ThreadKey:       "forged-thread-1",
		ThreadStatus:    "resolved",
		Column6:         "",
	})
	if err != nil || updated != 0 {
		t.Fatalf("UpdateInboxThreadState forged row updated %d rows, error %v; want 0", updated, err)
	}
	updated, err = queries.UpdateInboxThreadState(ctx, UpdateInboxThreadStateParams{
		WorkspaceID:     "inbox-isolation-workspace-2",
		SocialAccountID: "inbox-isolation-account-2",
		Source:          "ig_comment",
		ThreadKey:       "valid-thread-2",
		ThreadStatus:    "resolved",
		Column6:         "",
	})
	if err != nil || updated != 1 {
		t.Fatalf("UpdateInboxThreadState consistent row updated %d rows, error %v; want 1", updated, err)
	}
}
