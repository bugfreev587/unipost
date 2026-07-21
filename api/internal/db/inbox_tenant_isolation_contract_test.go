package db

import (
	"context"
	"errors"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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

const inboxManagedScopeOrTerm = "sqlc.arg('workspace_scope')::boolean or ( sa.connection_type = 'managed' and sa.external_user_id = sqlc.arg('external_user_id')::text )"
const inboxManagedScopePredicate = "and ( " + inboxManagedScopeOrTerm + " )"

func inboxManagedScopePredicateViolation(query, workspaceExpr string) string {
	if count := strings.Count(query, inboxManagedScopePredicate); count != 1 {
		return "managed scope predicate must occur exactly once with OR grouped inside parentheses"
	}

	storedWorkspaceAt := strings.Index(query, "i.workspace_id = "+workspaceExpr)
	derivedWorkspaceAt := strings.Index(query, "p.workspace_id = "+workspaceExpr)
	managedScopeAt := strings.Index(query, inboxManagedScopePredicate)
	if storedWorkspaceAt < 0 || derivedWorkspaceAt < 0 {
		return "stored and derived workspace predicates must both be present"
	}
	if managedScopeAt <= storedWorkspaceAt || managedScopeAt <= derivedWorkspaceAt {
		return "managed scope predicate must follow stored and derived workspace predicates"
	}
	return ""
}

func inboxAccountScopePredicateViolation(query string) string {
	if count := strings.Count(query, inboxManagedScopePredicate); count != 1 {
		return "account scope predicate must occur exactly once with OR grouped inside parentheses"
	}

	workspaceAt := strings.Index(query, "p.workspace_id = sqlc.arg('workspace_id')")
	managedScopeAt := strings.Index(query, inboxManagedScopePredicate)
	if workspaceAt < 0 {
		return "derived workspace predicate must be present"
	}
	if managedScopeAt <= workspaceAt {
		return "managed account scope predicate must follow the derived workspace predicate"
	}
	return ""
}

func TestInboxScopedNotificationOwnerProjectionContract(t *testing.T) {
	source := readInboxTenantIsolationContractFile(t, "queries/inbox.sql")
	generated := readInboxTenantIsolationContractFile(t, "inbox.sql.go")

	for _, queryName := range []string{
		"FindAllActiveInstagramAccountsByWebhookUserID",
		"FindAllSocialAccountsByPlatformAndExternalID",
		"ListAllInboxAccounts",
	} {
		t.Run(queryName, func(t *testing.T) {
			query := inboxTenantIsolationQuery(t, source, queryName)
			if count := strings.Count(query, "sa.external_user_id"); count != 1 {
				t.Fatalf("%s must project nullable DB notification owner exactly once, got %d in %s", queryName, count, query)
			}
			if strings.Contains(query, "coalesce(sa.external_user_id") {
				t.Fatalf("%s must preserve NULL/BYO ownership without COALESCE: %s", queryName, query)
			}

			structName := queryName + "Row"
			structPattern := regexp.MustCompile(`(?s)type ` + regexp.QuoteMeta(structName) + ` struct \{.*?ExternalUserID\s+pgtype\.Text\s+`)
			if !structPattern.MatchString(generated) {
				t.Fatalf("generated %s must expose ExternalUserID as pgtype.Text", structName)
			}
		})
	}
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
		managedRead   bool
	}{
		{name: "ListInboxItemsByWorkspace", workspaceExpr: "$1", managedRead: true},
		{name: "GetInboxItem", workspaceExpr: "$2", managedRead: true},
		{name: "MarkInboxItemRead", workspaceExpr: "$2", mutation: true},
		{name: "UpdateInboxItemAuthorMetadata", workspaceExpr: "@workspace_id", mutation: true},
		{name: "MarkAllInboxItemsRead", workspaceExpr: "@workspace_id", mutation: true},
		{name: "UpdateInboxThreadState", workspaceExpr: "$1", mutation: true},
		{name: "CountUnreadByWorkspace", workspaceExpr: "$1", managedRead: true},
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
			if tt.managedRead {
				if violation := inboxManagedScopePredicateViolation(query, tt.workspaceExpr); violation != "" {
					t.Errorf("%s %s: %s", tt.name, violation, query)
				}
				if strings.Contains(query, "coalesce(") {
					t.Errorf("%s must not infer workspace scope from a nullable or empty managed-user id: %s", tt.name, query)
				}
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

func TestInboxManagedUserReadScopeContractRejectsSemanticMutations(t *testing.T) {
	source := readInboxTenantIsolationContractFile(t, "queries/inbox.sql")
	orTerm := inboxManagedScopeOrTerm

	for _, tt := range []struct {
		name          string
		workspaceExpr string
	}{
		{name: "ListInboxItemsByWorkspace", workspaceExpr: "$1"},
		{name: "CountUnreadByWorkspace", workspaceExpr: "$1"},
		{name: "GetInboxItem", workspaceExpr: "$2"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			query := inboxTenantIsolationQuery(t, source, tt.name)
			if violation := inboxManagedScopePredicateViolation(query, tt.workspaceExpr); violation != "" {
				t.Fatalf("valid query rejected: %s", violation)
			}

			mutations := []struct {
				name  string
				query string
			}{
				{name: "OR changed to AND", query: strings.Replace(
					query,
					orTerm,
					"sqlc.arg('workspace_scope')::boolean and sa.external_user_id = sqlc.arg('external_user_id')::text",
					1,
				)},
				{name: "grouping removed", query: strings.Replace(query, "and ( "+orTerm+" )", "and "+orTerm, 1)},
			}
			for _, mutation := range mutations {
				t.Run(mutation.name, func(t *testing.T) {
					if mutation.query == query {
						t.Fatal("test mutation did not change the query")
					}
					if violation := inboxManagedScopePredicateViolation(mutation.query, tt.workspaceExpr); violation == "" {
						t.Fatalf("semantic mutation escaped the managed read scope contract: %s", mutation.query)
					}
				})
			}
		})
	}
}

func TestInboxManagedUserAccountEnumeration(t *testing.T) {
	source := readInboxTenantIsolationContractFile(t, "queries/inbox.sql")
	query := inboxTenantIsolationQuery(t, source, "FindInboxAccountsByWorkspace")
	if violation := inboxAccountScopePredicateViolation(query); violation != "" {
		t.Fatalf("valid account enumeration query rejected: %s: %s", violation, query)
	}
	for _, want := range []string{
		"select distinct sa.id, sa.profile_id, sa.platform, sa.access_token",
		"p.workspace_id = sqlc.arg('workspace_id')",
		"sa.status = 'active'",
		"sa.disconnected_at is null",
		"sa.platform in ('instagram', 'threads', 'facebook', 'twitter')",
		"order by sa.connected_at desc, sa.id",
	} {
		if !strings.Contains(query, want) {
			t.Errorf("FindInboxAccountsByWorkspace missing %q in %s", want, query)
		}
	}
	if strings.Contains(query, "coalesce(") {
		t.Fatal("account enumeration must not infer workspace scope from a nullable or empty managed-user id")
	}

	orTerm := inboxManagedScopeOrTerm
	mutations := []struct {
		name  string
		query string
	}{
		{name: "OR changed to AND", query: strings.Replace(
			query,
			orTerm,
			"sqlc.arg('workspace_scope')::boolean and sa.external_user_id = sqlc.arg('external_user_id')::text",
			1,
		)},
		{name: "grouping removed", query: strings.Replace(query, "and ( "+orTerm+" )", "and "+orTerm, 1)},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			if mutation.query == query {
				t.Fatal("test mutation did not change the account query")
			}
			if violation := inboxAccountScopePredicateViolation(mutation.query); violation == "" {
				t.Fatalf("semantic mutation escaped the account scope contract: %s", mutation.query)
			}
		})
	}
}

func TestCountInboxAccountsInScopeQueryContract(t *testing.T) {
	source := readInboxTenantIsolationContractFile(t, "queries/inbox.sql")
	raw := inboxTenantIsolationRawQuery(t, source, "CountInboxAccountsInScope")
	query := compactInboxTenantIsolationSQL(raw)
	if !strings.Contains(raw, "-- name: CountInboxAccountsInScope :one") {
		t.Fatal("CountInboxAccountsInScope must retain its :one annotation")
	}
	for _, want := range []string{
		"select count(*)::integer",
		"from social_accounts sa",
		"join profiles p on p.id = sa.profile_id",
		"p.workspace_id = @workspace_id",
		"sa.id = any(@account_ids::text[])",
		inboxManagedScopePredicate,
	} {
		if !strings.Contains(query, want) {
			t.Errorf("CountInboxAccountsInScope missing %q in %s", want, query)
		}
	}
	countScopeViolation := func(candidate string) string {
		if count := strings.Count(candidate, inboxManagedScopePredicate); count != 1 {
			return "managed scope predicate must occur exactly once with OR grouped inside parentheses"
		}
		workspaceAt := strings.Index(candidate, "p.workspace_id = @workspace_id")
		accountIDsAt := strings.Index(candidate, "sa.id = any(@account_ids::text[])")
		scopeAt := strings.Index(candidate, inboxManagedScopePredicate)
		if workspaceAt < 0 || accountIDsAt < 0 {
			return "workspace and account-ID predicates must both be present"
		}
		if scopeAt <= workspaceAt || scopeAt <= accountIDsAt {
			return "managed scope predicate must follow workspace and account-ID predicates"
		}
		return ""
	}
	if violation := countScopeViolation(query); violation != "" {
		t.Fatalf("CountInboxAccountsInScope scope placement is unsafe: %s: %s", violation, query)
	}
	for _, forbidden := range []string{"sa.status", "sa.disconnected_at", "coalesce("} {
		if strings.Contains(query, forbidden) {
			t.Errorf("CountInboxAccountsInScope must reconcile historical operation ownership without %q: %s", forbidden, query)
		}
	}

	orTerm := inboxManagedScopeOrTerm
	mutations := []struct {
		name  string
		query string
	}{
		{name: "OR changed to AND", query: strings.Replace(query, orTerm,
			"sqlc.arg('workspace_scope')::boolean and sa.external_user_id = sqlc.arg('external_user_id')::text", 1)},
		{name: "grouping removed", query: strings.Replace(query, "and ( "+orTerm+" )", "and "+orTerm, 1)},
		{name: "scope moved before workspace", query: strings.Replace(
			strings.Replace(query, " "+inboxManagedScopePredicate, "", 1),
			"where p.workspace_id", "where "+strings.TrimPrefix(inboxManagedScopePredicate, "and ")+" and p.workspace_id", 1,
		)},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			if mutation.query == query {
				t.Fatal("test mutation did not change CountInboxAccountsInScope")
			}
			if violation := countScopeViolation(mutation.query); violation == "" {
				t.Fatalf("semantic mutation escaped CountInboxAccountsInScope contract: %s", mutation.query)
			}
		})
	}

	generated := readInboxTenantIsolationContractFile(t, "inbox.sql.go")
	generatedQuery := inboxTenantIsolationQuery(t, generated, "CountInboxAccountsInScope")
	for _, want := range []string{
		"select count(*)::integer",
		"from social_accounts sa",
		"join profiles p on p.id = sa.profile_id",
		"where p.workspace_id = $1",
		"and sa.id = any($2::text[])",
		"and ( $3::boolean or ( sa.connection_type = 'managed' and sa.external_user_id = $4::text ) )",
	} {
		if !strings.Contains(generatedQuery, want) {
			t.Errorf("generated CountInboxAccountsInScope missing %q in %s", want, generatedQuery)
		}
	}

	paramsType := reflect.TypeOf(CountInboxAccountsInScopeParams{})
	wantFields := []struct {
		name   string
		typeOf reflect.Type
	}{
		{name: "WorkspaceID", typeOf: reflect.TypeOf("")},
		{name: "AccountIds", typeOf: reflect.TypeOf([]string{})},
		{name: "WorkspaceScope", typeOf: reflect.TypeOf(false)},
		{name: "ExternalUserID", typeOf: reflect.TypeOf("")},
	}
	if paramsType.NumField() != len(wantFields) {
		t.Fatalf("CountInboxAccountsInScopeParams fields=%d, want %d", paramsType.NumField(), len(wantFields))
	}
	for index, want := range wantFields {
		field := paramsType.Field(index)
		if field.Name != want.name || field.Type != want.typeOf {
			t.Errorf("CountInboxAccountsInScopeParams field %d = %s %s, want %s %s",
				index, field.Name, field.Type, want.name, want.typeOf)
		}
	}
	var _ func(*Queries, context.Context, CountInboxAccountsInScopeParams) (int32, error) = (*Queries).CountInboxAccountsInScope
}

func TestInboxManagedUserAccountEnumerationCallSites(t *testing.T) {
	handlerSource := readInboxTenantIsolationContractFile(t, "../handler/inbox.go")
	if got := strings.Count(handlerSource, ".FindInboxAccountsByWorkspace("); got != 1 {
		t.Fatalf("HTTP Inbox account enumeration call sites=%d, want one shared scoped lookup", got)
	}
	for _, want := range []string{
		"workspaceScope, externalUserID := inboxQueryScope(r.Context())",
		"WorkspaceScope: workspaceScope",
		"ExternalUserID: externalUserID",
		"h.syncXBackfill(w, r, workspaceID, accounts, *request.XBackfill)",
	} {
		if !strings.Contains(handlerSource, want) {
			t.Fatalf("HTTP Inbox sync call site missing %q", want)
		}
	}

	workerSource := readInboxTenantIsolationContractFile(t, "../worker/inbox_sync.go")
	if strings.Contains(workerSource, "FindInboxAccountsByWorkspace(") {
		t.Fatal("background worker must not manufacture request-managed scope for account enumeration")
	}
	if got := strings.Count(workerSource, ".ListAllInboxAccounts(ctx)"); got != 1 {
		t.Fatalf("background internal all-workspace discovery calls=%d, want one explicit purpose-built lookup", got)
	}
}

func TestInboxManagedUserMutations(t *testing.T) {
	source := readInboxTenantIsolationContractFile(t, "queries/inbox.sql")
	orTerm := inboxManagedScopeOrTerm

	for _, tt := range []struct {
		name          string
		workspaceExpr string
	}{
		{name: "MarkInboxItemRead", workspaceExpr: "$2"},
		{name: "MarkAllInboxItemsRead", workspaceExpr: "@workspace_id"},
		{name: "UpdateInboxThreadState", workspaceExpr: "$1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			query := inboxTenantIsolationQuery(t, source, tt.name)
			if violation := inboxManagedScopePredicateViolation(query, tt.workspaceExpr); violation != "" {
				t.Fatalf("%s: %s", violation, query)
			}
			if strings.Contains(query, "coalesce(") {
				t.Fatalf("mutation must not infer workspace scope from a nullable or empty managed-user id: %s", query)
			}

			mutations := []struct {
				name  string
				query string
			}{
				{name: "OR changed to AND", query: strings.Replace(
					query,
					orTerm,
					"sqlc.arg('workspace_scope')::boolean and sa.external_user_id = sqlc.arg('external_user_id')::text",
					1,
				)},
				{name: "grouping removed", query: strings.Replace(query, "and ( "+orTerm+" )", "and "+orTerm, 1)},
			}
			for _, mutation := range mutations {
				t.Run(mutation.name, func(t *testing.T) {
					if mutation.query == query {
						t.Fatal("test mutation did not change the query")
					}
					if violation := inboxManagedScopePredicateViolation(mutation.query, tt.workspaceExpr); violation == "" {
						t.Fatalf("semantic mutation escaped the managed scope contract: %s", mutation.query)
					}
				})
			}
		})
	}
}

func TestInboxManagedUserGeneratedReadScopeParamsAreExplicit(t *testing.T) {
	type expectedField struct {
		name   string
		typeOf reflect.Type
	}

	stringType := reflect.TypeOf("")
	boolType := reflect.TypeOf(false)
	int32Type := reflect.TypeOf(int32(0))
	pgTextType := reflect.TypeOf(pgtype.Text{})
	pgBoolType := reflect.TypeOf(pgtype.Bool{})
	interfaceType := reflect.TypeOf((*interface{})(nil)).Elem()

	assertFields := func(t *testing.T, value any, want []expectedField) {
		t.Helper()
		got := reflect.TypeOf(value)
		if got.NumField() != len(want) {
			t.Fatalf("%s has %d fields, want %d explicit ordered fields", got.Name(), got.NumField(), len(want))
		}
		for index, expected := range want {
			field := got.Field(index)
			if field.Name != expected.name || field.Type != expected.typeOf {
				t.Errorf("%s field %d = %s %s, want %s %s", got.Name(), index, field.Name, field.Type, expected.name, expected.typeOf)
			}
		}
	}

	assertFields(t, ListInboxItemsByWorkspaceParams{}, []expectedField{
		{name: "WorkspaceID", typeOf: stringType},
		{name: "Limit", typeOf: int32Type},
		{name: "WorkspaceScope", typeOf: boolType},
		{name: "ExternalUserID", typeOf: stringType},
		{name: "ExcludeXDms", typeOf: boolType},
		{name: "Source", typeOf: pgTextType},
		{name: "IsRead", typeOf: pgBoolType},
		{name: "IsOwn", typeOf: pgBoolType},
	})
	assertFields(t, CountUnreadByWorkspaceParams{}, []expectedField{
		{name: "WorkspaceID", typeOf: stringType},
		{name: "WorkspaceScope", typeOf: boolType},
		{name: "ExternalUserID", typeOf: stringType},
		{name: "ExcludeXDms", typeOf: boolType},
	})
	assertFields(t, GetInboxItemParams{}, []expectedField{
		{name: "ID", typeOf: stringType},
		{name: "WorkspaceID", typeOf: stringType},
		{name: "WorkspaceScope", typeOf: boolType},
		{name: "ExternalUserID", typeOf: stringType},
	})
	assertFields(t, MarkInboxItemReadParams{}, []expectedField{
		{name: "ID", typeOf: stringType},
		{name: "WorkspaceID", typeOf: stringType},
		{name: "WorkspaceScope", typeOf: boolType},
		{name: "ExternalUserID", typeOf: stringType},
	})
	assertFields(t, MarkAllInboxItemsReadParams{}, []expectedField{
		{name: "WorkspaceID", typeOf: stringType},
		{name: "WorkspaceScope", typeOf: boolType},
		{name: "ExternalUserID", typeOf: stringType},
		{name: "ExcludeXDms", typeOf: boolType},
	})
	assertFields(t, UpdateInboxThreadStateParams{}, []expectedField{
		{name: "WorkspaceID", typeOf: stringType},
		{name: "SocialAccountID", typeOf: stringType},
		{name: "Source", typeOf: stringType},
		{name: "ThreadKey", typeOf: stringType},
		{name: "ThreadStatus", typeOf: stringType},
		{name: "Column6", typeOf: interfaceType},
		{name: "WorkspaceScope", typeOf: boolType},
		{name: "ExternalUserID", typeOf: stringType},
	})
	assertFields(t, FindInboxAccountsByWorkspaceParams{}, []expectedField{
		{name: "WorkspaceID", typeOf: stringType},
		{name: "WorkspaceScope", typeOf: boolType},
		{name: "ExternalUserID", typeOf: stringType},
	})

	var _ func(*Queries, context.Context, ListInboxItemsByWorkspaceParams) ([]InboxItem, error) = (*Queries).ListInboxItemsByWorkspace
	var _ func(*Queries, context.Context, CountUnreadByWorkspaceParams) (int32, error) = (*Queries).CountUnreadByWorkspace
	var _ func(*Queries, context.Context, GetInboxItemParams) (InboxItem, error) = (*Queries).GetInboxItem
	var _ func(*Queries, context.Context, MarkInboxItemReadParams) error = (*Queries).MarkInboxItemRead
	var _ func(*Queries, context.Context, MarkAllInboxItemsReadParams) (int64, error) = (*Queries).MarkAllInboxItemsRead
	var _ func(*Queries, context.Context, UpdateInboxThreadStateParams) (int64, error) = (*Queries).UpdateInboxThreadState
	var _ func(*Queries, context.Context, FindInboxAccountsByWorkspaceParams) ([]SocialAccount, error) = (*Queries).FindInboxAccountsByWorkspace
}

func TestInboxTenantIsolationWebhookRoutingQueriesAreExact(t *testing.T) {
	source := readInboxTenantIsolationContractFile(t, "queries/inbox.sql")

	for _, legacy := range []string{
		"FindAnyActiveAccountByPlatform",
		"FindAllActiveAccountsByPlatform",
		"FindSocialAccountByPlatformAndExternalID",
	} {
		if strings.Contains(source, "-- name: "+legacy+" ") {
			t.Errorf("unsafe legacy webhook routing query %s must be removed", legacy)
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
		  ('inbox-isolation-profile-byo', 'Inbox Isolation BYO', 'inbox-isolation-workspace-1'),
		  ('inbox-isolation-profile-2', 'Inbox Isolation Two', 'inbox-isolation-workspace-2')
	`); err != nil {
		t.Fatalf("seed profiles: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO social_accounts (
		  id, profile_id, platform, access_token, external_account_id, metadata,
		  connection_type, external_user_id, status, disconnected_at
		)
		VALUES
		  (
		    'inbox-isolation-account-a', 'inbox-isolation-profile-1', 'instagram',
		    'token-a', 'external-a', '{"instagram_webhook_user_id":"webhook-user-a"}'::jsonb,
		    'managed', 'managed-a', 'active', NULL
		  ),
		  (
		    'inbox-isolation-account-a-facebook', 'inbox-isolation-profile-1', 'facebook',
		    'token-a-facebook', 'external-a-facebook', '{}'::jsonb,
		    'managed', 'managed-a', 'active', NULL
		  ),
		  (
		    'inbox-isolation-account-a-threads', 'inbox-isolation-profile-1', 'threads',
		    'token-a-threads', 'external-a-threads', '{}'::jsonb,
		    'managed', 'managed-a', 'active', NULL
		  ),
		  (
		    'inbox-isolation-account-a-twitter', 'inbox-isolation-profile-1', 'twitter',
		    'token-a-twitter', 'external-a-twitter', '{}'::jsonb,
		    'managed', 'managed-a', 'active', NULL
		  ),
		  (
		    'inbox-isolation-account-b', 'inbox-isolation-profile-1', 'instagram',
		    'token-b', 'external-b', '{"instagram_webhook_user_id":"webhook-user-b"}'::jsonb,
		    'managed', 'managed-b', 'active', NULL
		  ),
		  (
		    'inbox-isolation-account-b-facebook', 'inbox-isolation-profile-1', 'facebook',
		    'token-b-facebook', 'external-b-facebook', '{}'::jsonb,
		    'managed', 'managed-b', 'active', NULL
		  ),
		  (
		    'inbox-isolation-account-b-threads', 'inbox-isolation-profile-1', 'threads',
		    'token-b-threads', 'external-b-threads', '{}'::jsonb,
		    'managed', 'managed-b', 'active', NULL
		  ),
		  (
		    'inbox-isolation-account-b-twitter', 'inbox-isolation-profile-1', 'twitter',
		    'token-b-twitter', 'external-b-twitter', '{}'::jsonb,
		    'managed', 'managed-b', 'active', NULL
		  ),
		  -- Deliberately model a legacy/anomalous BYO row that carries a managed-user ID.
		  (
		    'inbox-isolation-account-byo', 'inbox-isolation-profile-byo', 'instagram',
		    'token-byo', 'external-byo', '{"instagram_webhook_user_id":"webhook-user-byo"}'::jsonb,
		    'byo', 'managed-a', 'active', NULL
		  ),
		  (
		    'inbox-isolation-account-byo-facebook', 'inbox-isolation-profile-1', 'facebook',
		    'token-byo-facebook', 'external-byo-facebook', '{}'::jsonb,
		    'byo', NULL, 'active', NULL
		  ),
		  (
		    'inbox-isolation-account-byo-threads', 'inbox-isolation-profile-1', 'threads',
		    'token-byo-threads', 'external-byo-threads', '{}'::jsonb,
		    'byo', NULL, 'active', NULL
		  ),
		  (
		    'inbox-isolation-account-byo-twitter', 'inbox-isolation-profile-1', 'twitter',
		    'token-byo-twitter', 'external-byo-twitter', '{}'::jsonb,
		    'byo', NULL, 'active', NULL
		  ),
		  (
		    'inbox-isolation-account-inactive', 'inbox-isolation-profile-1', 'facebook',
		    'token-inactive', 'external-inactive', '{}'::jsonb,
		    'managed', 'managed-inactive', 'reconnect_required', NULL
		  ),
		  (
		    'inbox-isolation-account-disconnected', 'inbox-isolation-profile-1', 'threads',
		    'token-disconnected', 'external-disconnected', '{}'::jsonb,
		    'managed', 'managed-disconnected', 'disconnected', NOW()
		  ),
		  (
		    'inbox-isolation-account-2', 'inbox-isolation-profile-2', 'instagram',
		    'token-2', 'external-2', '{"instagram_webhook_user_id":"webhook-user-2"}'::jsonb,
		    'managed', 'managed-other', 'active', NULL
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
		    'inbox-isolation-valid-a', 'inbox-isolation-account-a',
		    'inbox-isolation-workspace-1', 'ig_comment', 'valid-external-a',
		    'valid a', 'valid-thread-a'
		  ),
		  (
		    'inbox-isolation-valid-b', 'inbox-isolation-account-b',
		    'inbox-isolation-workspace-1', 'ig_comment', 'valid-external-b',
		    'valid b', 'valid-thread-b'
		  ),
		  (
		    'inbox-isolation-valid-byo', 'inbox-isolation-account-byo',
		    'inbox-isolation-workspace-1', 'ig_comment', 'valid-external-byo',
		    'valid byo', 'valid-thread-byo'
		  ),
		  (
		    'inbox-isolation-valid-2', 'inbox-isolation-account-2',
		    'inbox-isolation-workspace-2', 'ig_comment', 'valid-external-2',
		    'valid two', 'valid-thread-2'
		  ),
		  (
		    'inbox-isolation-forged-1', 'inbox-isolation-account-a',
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
	assertAccountIDs := func(label string, accounts []SocialAccount, want ...string) {
		t.Helper()
		got := make(map[string]int, len(accounts))
		for _, account := range accounts {
			got[account.ID]++
		}
		if len(got) != len(want) {
			t.Fatalf("%s IDs = %v, want %v", label, got, want)
		}
		for _, id := range want {
			if got[id] != 1 {
				t.Fatalf("%s IDs = %v, want exactly one %s", label, got, id)
			}
		}
	}
	accounts, err := queries.FindInboxAccountsByWorkspace(ctx, FindInboxAccountsByWorkspaceParams{
		WorkspaceID:    "inbox-isolation-workspace-1",
		WorkspaceScope: false,
		ExternalUserID: "managed-a",
	})
	if err != nil {
		t.Fatalf("managed-a FindInboxAccountsByWorkspace: %v", err)
	}
	assertAccountIDs(
		"managed-a account enumeration",
		accounts,
		"inbox-isolation-account-a",
		"inbox-isolation-account-a-facebook",
		"inbox-isolation-account-a-threads",
		"inbox-isolation-account-a-twitter",
	)
	accounts, err = queries.FindInboxAccountsByWorkspace(ctx, FindInboxAccountsByWorkspaceParams{
		WorkspaceID:    "inbox-isolation-workspace-1",
		WorkspaceScope: false,
		ExternalUserID: "managed-b",
	})
	if err != nil {
		t.Fatalf("managed-b FindInboxAccountsByWorkspace: %v", err)
	}
	assertAccountIDs(
		"managed-b account enumeration",
		accounts,
		"inbox-isolation-account-b",
		"inbox-isolation-account-b-facebook",
		"inbox-isolation-account-b-threads",
		"inbox-isolation-account-b-twitter",
	)
	accounts, err = queries.FindInboxAccountsByWorkspace(ctx, FindInboxAccountsByWorkspaceParams{
		WorkspaceID:    "inbox-isolation-workspace-1",
		WorkspaceScope: true,
		ExternalUserID: "",
	})
	if err != nil {
		t.Fatalf("workspace FindInboxAccountsByWorkspace: %v", err)
	}
	assertAccountIDs(
		"workspace account enumeration",
		accounts,
		"inbox-isolation-account-a",
		"inbox-isolation-account-a-facebook",
		"inbox-isolation-account-a-threads",
		"inbox-isolation-account-a-twitter",
		"inbox-isolation-account-b",
		"inbox-isolation-account-b-facebook",
		"inbox-isolation-account-b-threads",
		"inbox-isolation-account-b-twitter",
		"inbox-isolation-account-byo",
		"inbox-isolation-account-byo-facebook",
		"inbox-isolation-account-byo-threads",
		"inbox-isolation-account-byo-twitter",
	)
	assertScopedAccountCount := func(
		label, workspaceID string,
		accountIDs []string,
		workspaceScope bool,
		externalUserID string,
		want int32,
	) {
		t.Helper()
		got, err := queries.CountInboxAccountsInScope(ctx, CountInboxAccountsInScopeParams{
			WorkspaceID:    workspaceID,
			AccountIds:     accountIDs,
			WorkspaceScope: workspaceScope,
			ExternalUserID: externalUserID,
		})
		if err != nil || got != want {
			t.Fatalf("%s CountInboxAccountsInScope = %d, %v; want %d", label, got, err, want)
		}
	}
	managedAAccountIDs := []string{
		"inbox-isolation-account-a",
		"inbox-isolation-account-a-facebook",
		"inbox-isolation-account-a-threads",
		"inbox-isolation-account-a-twitter",
	}
	assertScopedAccountCount(
		"managed A exact snapshot", "inbox-isolation-workspace-1",
		managedAAccountIDs, false, "managed-a", int32(len(managedAAccountIDs)),
	)
	assertScopedAccountCount(
		"managed A snapshot under managed B", "inbox-isolation-workspace-1",
		managedAAccountIDs, false, "managed-b", 0,
	)
	assertScopedAccountCount(
		"mixed managed A and B snapshot under managed A", "inbox-isolation-workspace-1",
		[]string{"inbox-isolation-account-a", "inbox-isolation-account-b"}, false, "managed-a", 1,
	)
	assertScopedAccountCount(
		"workspace snapshot includes BYO", "inbox-isolation-workspace-1",
		[]string{"inbox-isolation-account-a", "inbox-isolation-account-b", "inbox-isolation-account-byo"}, true, "", 3,
	)
	assertScopedAccountCount(
		"workspace snapshot excludes another workspace", "inbox-isolation-workspace-1",
		[]string{"inbox-isolation-account-a", "inbox-isolation-account-2"}, true, "", 1,
	)
	assertIDs := func(label string, items []InboxItem, want ...string) {
		t.Helper()
		got := make(map[string]int, len(items))
		for _, item := range items {
			got[item.ID]++
		}
		if len(got) != len(want) {
			t.Fatalf("%s IDs = %v, want %v", label, got, want)
		}
		for _, id := range want {
			if got[id] != 1 {
				t.Fatalf("%s IDs = %v, want exactly one %s", label, got, id)
			}
		}
	}
	assertUnread := func(label, workspaceID string, workspaceScope bool, externalUserID string, want int32) {
		t.Helper()
		got, err := queries.CountUnreadByWorkspace(ctx, CountUnreadByWorkspaceParams{
			WorkspaceID:    workspaceID,
			WorkspaceScope: workspaceScope,
			ExternalUserID: externalUserID,
			ExcludeXDms:    false,
		})
		if err != nil || got != want {
			t.Fatalf("%s unread count = %d, error %v; want %d", label, got, err, want)
		}
	}
	assertGet := func(label, id, workspaceID string, workspaceScope bool, externalUserID string, want bool) {
		t.Helper()
		item, err := queries.GetInboxItem(ctx, GetInboxItemParams{
			ID:             id,
			WorkspaceID:    workspaceID,
			WorkspaceScope: workspaceScope,
			ExternalUserID: externalUserID,
		})
		if !want {
			if !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("%s GetInboxItem error = %v, want pgx.ErrNoRows", label, err)
			}
			return
		}
		if err != nil || item.ID != id {
			t.Fatalf("%s GetInboxItem = %+v, %v; want %s", label, item, err, id)
		}
	}

	items, err := queries.ListInboxItemsByWorkspace(ctx, ListInboxItemsByWorkspaceParams{
		WorkspaceID:    "inbox-isolation-workspace-1",
		Limit:          20,
		WorkspaceScope: false,
		ExternalUserID: "managed-a",
	})
	if err != nil {
		t.Fatalf("managed-a ListInboxItemsByWorkspace: %v", err)
	}
	assertIDs("managed-a Inbox list", items, "inbox-isolation-valid-a")
	assertUnread("managed-a", "inbox-isolation-workspace-1", false, "managed-a", 1)
	assertGet("managed-a own row", "inbox-isolation-valid-a", "inbox-isolation-workspace-1", false, "managed-a", true)
	assertGet("managed-a cross-scope row", "inbox-isolation-valid-b", "inbox-isolation-workspace-1", false, "managed-a", false)

	items, err = queries.ListInboxItemsByWorkspace(ctx, ListInboxItemsByWorkspaceParams{
		WorkspaceID:    "inbox-isolation-workspace-1",
		Limit:          20,
		WorkspaceScope: false,
		ExternalUserID: "managed-b",
	})
	if err != nil {
		t.Fatalf("managed-b ListInboxItemsByWorkspace: %v", err)
	}
	assertIDs("managed-b Inbox list", items, "inbox-isolation-valid-b")
	assertUnread("managed-b", "inbox-isolation-workspace-1", false, "managed-b", 1)
	assertGet("managed-b own row", "inbox-isolation-valid-b", "inbox-isolation-workspace-1", false, "managed-b", true)
	assertGet("managed-b cross-scope row", "inbox-isolation-valid-a", "inbox-isolation-workspace-1", false, "managed-b", false)

	items, err = queries.ListInboxItemsByWorkspace(ctx, ListInboxItemsByWorkspaceParams{
		WorkspaceID:    "inbox-isolation-workspace-1",
		Limit:          20,
		WorkspaceScope: true,
		ExternalUserID: "ignored-in-workspace-mode",
	})
	if err != nil {
		t.Fatalf("workspace ListInboxItemsByWorkspace: %v", err)
	}
	assertIDs(
		"workspace Inbox list",
		items,
		"inbox-isolation-valid-a",
		"inbox-isolation-valid-b",
		"inbox-isolation-valid-byo",
	)
	assertUnread("workspace", "inbox-isolation-workspace-1", true, "ignored-in-workspace-mode", 3)
	for _, id := range []string{
		"inbox-isolation-valid-a",
		"inbox-isolation-valid-b",
		"inbox-isolation-valid-byo",
	} {
		assertGet("workspace row "+id, id, "inbox-isolation-workspace-1", true, "ignored-in-workspace-mode", true)
	}

	items, err = queries.ListInboxItemsByWorkspace(ctx, ListInboxItemsByWorkspaceParams{
		WorkspaceID:    "inbox-isolation-workspace-2",
		Limit:          20,
		WorkspaceScope: true,
		ExternalUserID: "ignored-in-workspace-mode",
	})
	if err != nil {
		t.Fatalf("ListInboxItemsByWorkspace: %v", err)
	}
	assertIDs("workspace-2 Inbox list", items, "inbox-isolation-valid-2")
	assertUnread("workspace-2", "inbox-isolation-workspace-2", true, "ignored-in-workspace-mode", 1)

	_, err = queries.GetInboxItem(ctx, GetInboxItemParams{
		ID:             "inbox-isolation-forged-1",
		WorkspaceID:    "inbox-isolation-workspace-2",
		WorkspaceScope: true,
		ExternalUserID: "ignored-in-workspace-mode",
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetInboxItem forged row error = %v, want pgx.ErrNoRows", err)
	}
	item, err := queries.GetInboxItem(ctx, GetInboxItemParams{
		ID:             "inbox-isolation-valid-2",
		WorkspaceID:    "inbox-isolation-workspace-2",
		WorkspaceScope: true,
		ExternalUserID: "ignored-in-workspace-mode",
	})
	if err != nil || item.ID != "inbox-isolation-valid-2" {
		t.Fatalf("GetInboxItem consistent row = %+v, %v", item, err)
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
	markItemRead := func(label, id, workspaceID string, workspaceScope bool, externalUserID string) {
		t.Helper()
		if err := queries.MarkInboxItemRead(ctx, MarkInboxItemReadParams{
			ID:             id,
			WorkspaceID:    workspaceID,
			WorkspaceScope: workspaceScope,
			ExternalUserID: externalUserID,
		}); err != nil {
			t.Fatalf("%s MarkInboxItemRead: %v", label, err)
		}
	}
	resetRead := func(ids ...string) {
		t.Helper()
		if _, err := tx.Exec(ctx, "UPDATE inbox_items SET is_read = false WHERE id = ANY($1)", ids); err != nil {
			t.Fatalf("reset read state for %v: %v", ids, err)
		}
	}

	markItemRead("forged cross-workspace row", "inbox-isolation-forged-1", "inbox-isolation-workspace-2", true, "ignored-in-workspace-mode")
	assertRead("inbox-isolation-forged-1", false)

	markItemRead("managed-a own row", "inbox-isolation-valid-a", "inbox-isolation-workspace-1", false, "managed-a")
	assertRead("inbox-isolation-valid-a", true)
	resetRead("inbox-isolation-valid-a")

	markItemRead("managed-a cross-scope row", "inbox-isolation-valid-b", "inbox-isolation-workspace-1", false, "managed-a")
	assertRead("inbox-isolation-valid-b", false)
	markItemRead("managed-b own row", "inbox-isolation-valid-b", "inbox-isolation-workspace-1", false, "managed-b")
	assertRead("inbox-isolation-valid-b", true)
	resetRead("inbox-isolation-valid-b")

	for _, id := range []string{
		"inbox-isolation-valid-a",
		"inbox-isolation-valid-b",
		"inbox-isolation-valid-byo",
	} {
		markItemRead("workspace row "+id, id, "inbox-isolation-workspace-1", true, "ignored-in-workspace-mode")
		assertRead(id, true)
	}
	resetRead(
		"inbox-isolation-valid-a",
		"inbox-isolation-valid-b",
		"inbox-isolation-valid-byo",
	)

	markItemRead("consistent workspace-2 row", "inbox-isolation-valid-2", "inbox-isolation-workspace-2", true, "ignored-in-workspace-mode")
	assertRead("inbox-isolation-valid-2", true)
	resetRead("inbox-isolation-valid-2")

	updated, err := queries.MarkAllInboxItemsRead(ctx, MarkAllInboxItemsReadParams{
		WorkspaceID:    "inbox-isolation-workspace-1",
		WorkspaceScope: false,
		ExternalUserID: "managed-a",
		ExcludeXDms:    false,
	})
	if err != nil || updated != 1 {
		t.Fatalf("managed-a MarkAllInboxItemsRead updated %d rows, error %v; want 1", updated, err)
	}
	assertRead("inbox-isolation-valid-a", true)
	assertRead("inbox-isolation-valid-b", false)
	assertRead("inbox-isolation-valid-byo", false)

	updated, err = queries.MarkAllInboxItemsRead(ctx, MarkAllInboxItemsReadParams{
		WorkspaceID:    "inbox-isolation-workspace-1",
		WorkspaceScope: false,
		ExternalUserID: "managed-b",
		ExcludeXDms:    false,
	})
	if err != nil || updated != 1 {
		t.Fatalf("managed-b MarkAllInboxItemsRead updated %d rows, error %v; want 1", updated, err)
	}
	assertRead("inbox-isolation-valid-a", true)
	assertRead("inbox-isolation-valid-b", true)
	assertRead("inbox-isolation-valid-byo", false)

	resetRead(
		"inbox-isolation-valid-a",
		"inbox-isolation-valid-b",
		"inbox-isolation-valid-byo",
	)
	updated, err = queries.MarkAllInboxItemsRead(ctx, MarkAllInboxItemsReadParams{
		WorkspaceID:    "inbox-isolation-workspace-1",
		WorkspaceScope: true,
		ExternalUserID: "ignored-in-workspace-mode",
		ExcludeXDms:    false,
	})
	if err != nil || updated != 3 {
		t.Fatalf("workspace MarkAllInboxItemsRead updated %d rows, error %v; want 3", updated, err)
	}
	assertRead("inbox-isolation-valid-a", true)
	assertRead("inbox-isolation-valid-b", true)
	assertRead("inbox-isolation-valid-byo", true)

	updated, err = queries.MarkAllInboxItemsRead(ctx, MarkAllInboxItemsReadParams{
		WorkspaceID:    "inbox-isolation-workspace-2",
		WorkspaceScope: true,
		ExternalUserID: "ignored-in-workspace-mode",
		ExcludeXDms:    false,
	})
	if err != nil || updated != 1 {
		t.Fatalf("workspace-2 MarkAllInboxItemsRead updated %d rows, error %v; want 1", updated, err)
	}
	assertRead("inbox-isolation-valid-2", true)
	assertRead("inbox-isolation-forged-1", false)

	updated, err = queries.UpdateInboxThreadState(ctx, UpdateInboxThreadStateParams{
		WorkspaceID:     "inbox-isolation-workspace-2",
		SocialAccountID: "inbox-isolation-account-a",
		Source:          "ig_comment",
		ThreadKey:       "forged-thread-1",
		ThreadStatus:    "resolved",
		Column6:         "",
		WorkspaceScope:  true,
		ExternalUserID:  "ignored-in-workspace-mode",
	})
	if err != nil || updated != 0 {
		t.Fatalf("UpdateInboxThreadState forged row updated %d rows, error %v; want 0", updated, err)
	}
	assertThreadState := func(id, want string) {
		t.Helper()
		var got string
		if err := tx.QueryRow(ctx, "SELECT thread_status FROM inbox_items WHERE id = $1", id).Scan(&got); err != nil {
			t.Fatalf("read %s thread_status: %v", id, err)
		}
		if got != want {
			t.Fatalf("%s thread_status = %q, want %q", id, got, want)
		}
	}
	assertThreadState("inbox-isolation-forged-1", "open")

	updated, err = queries.UpdateInboxThreadState(ctx, UpdateInboxThreadStateParams{
		WorkspaceID:     "inbox-isolation-workspace-1",
		SocialAccountID: "inbox-isolation-account-a",
		Source:          "ig_comment",
		ThreadKey:       "valid-thread-a",
		ThreadStatus:    "resolved",
		Column6:         "",
		WorkspaceScope:  false,
		ExternalUserID:  "managed-a",
	})
	if err != nil || updated != 1 {
		t.Fatalf("managed-a own UpdateInboxThreadState updated %d rows, error %v; want 1", updated, err)
	}
	assertThreadState("inbox-isolation-valid-a", "resolved")

	updated, err = queries.UpdateInboxThreadState(ctx, UpdateInboxThreadStateParams{
		WorkspaceID:     "inbox-isolation-workspace-1",
		SocialAccountID: "inbox-isolation-account-b",
		Source:          "ig_comment",
		ThreadKey:       "valid-thread-b",
		ThreadStatus:    "resolved",
		Column6:         "",
		WorkspaceScope:  false,
		ExternalUserID:  "managed-a",
	})
	if err != nil || updated != 0 {
		t.Fatalf("managed-a cross-scope UpdateInboxThreadState updated %d rows, error %v; want 0", updated, err)
	}
	assertThreadState("inbox-isolation-valid-b", "open")

	updated, err = queries.UpdateInboxThreadState(ctx, UpdateInboxThreadStateParams{
		WorkspaceID:     "inbox-isolation-workspace-1",
		SocialAccountID: "inbox-isolation-account-b",
		Source:          "ig_comment",
		ThreadKey:       "valid-thread-b",
		ThreadStatus:    "resolved",
		Column6:         "",
		WorkspaceScope:  false,
		ExternalUserID:  "managed-b",
	})
	if err != nil || updated != 1 {
		t.Fatalf("managed-b own UpdateInboxThreadState updated %d rows, error %v; want 1", updated, err)
	}
	assertThreadState("inbox-isolation-valid-b", "resolved")

	for _, tt := range []struct {
		accountID string
		threadKey string
		itemID    string
	}{
		{accountID: "inbox-isolation-account-a", threadKey: "valid-thread-a", itemID: "inbox-isolation-valid-a"},
		{accountID: "inbox-isolation-account-b", threadKey: "valid-thread-b", itemID: "inbox-isolation-valid-b"},
		{accountID: "inbox-isolation-account-byo", threadKey: "valid-thread-byo", itemID: "inbox-isolation-valid-byo"},
	} {
		updated, err = queries.UpdateInboxThreadState(ctx, UpdateInboxThreadStateParams{
			WorkspaceID:     "inbox-isolation-workspace-1",
			SocialAccountID: tt.accountID,
			Source:          "ig_comment",
			ThreadKey:       tt.threadKey,
			ThreadStatus:    "assigned",
			Column6:         "",
			WorkspaceScope:  true,
			ExternalUserID:  "ignored-in-workspace-mode",
		})
		if err != nil || updated != 1 {
			t.Fatalf("workspace UpdateInboxThreadState for %s updated %d rows, error %v; want 1", tt.itemID, updated, err)
		}
		assertThreadState(tt.itemID, "assigned")
	}

	updated, err = queries.UpdateInboxThreadState(ctx, UpdateInboxThreadStateParams{
		WorkspaceID:     "inbox-isolation-workspace-2",
		SocialAccountID: "inbox-isolation-account-2",
		Source:          "ig_comment",
		ThreadKey:       "valid-thread-2",
		ThreadStatus:    "resolved",
		Column6:         "",
		WorkspaceScope:  true,
		ExternalUserID:  "ignored-in-workspace-mode",
	})
	if err != nil || updated != 1 {
		t.Fatalf("UpdateInboxThreadState consistent row updated %d rows, error %v; want 1", updated, err)
	}
}
