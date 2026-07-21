package connectownership

import (
	"context"
	"errors"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestDecideOwnership(t *testing.T) {
	tests := []struct {
		name           string
		matches        []db.SocialAccount
		profileID      string
		externalUserID string
		want           Decision
	}{
		{
			name:           "no match creates",
			profileID:      "profile-a",
			externalUserID: "managed-a",
			want:           Decision{Kind: Create},
		},
		{
			name: "same profile and same nonempty managed user reconnects",
			matches: []db.SocialAccount{{
				ID:             "account-a",
				ProfileID:      "profile-a",
				ExternalUserID: pgtype.Text{String: "managed-a", Valid: true},
			}},
			profileID:      "profile-a",
			externalUserID: "managed-a",
			want:           Decision{Kind: Reconnect, AccountID: "account-a"},
		},
		{
			name: "different managed user conflicts",
			matches: []db.SocialAccount{{
				ID:             "account-b",
				ProfileID:      "profile-a",
				ExternalUserID: pgtype.Text{String: "managed-b", Valid: true},
			}},
			profileID:      "profile-a",
			externalUserID: "managed-a",
			want:           Decision{Kind: Conflict},
		},
		{
			name: "owner BYO null ownership conflicts",
			matches: []db.SocialAccount{{
				ID:             "account-owner",
				ProfileID:      "profile-a",
				ExternalUserID: pgtype.Text{},
			}},
			profileID:      "profile-a",
			externalUserID: "managed-a",
			want:           Decision{Kind: Conflict},
		},
		{
			name: "empty stored managed ownership conflicts",
			matches: []db.SocialAccount{{
				ID:             "account-empty",
				ProfileID:      "profile-a",
				ExternalUserID: pgtype.Text{String: "", Valid: true},
			}},
			profileID:      "profile-a",
			externalUserID: "managed-a",
			want:           Decision{Kind: Conflict},
		},
		{
			name: "same managed user in a different profile conflicts",
			matches: []db.SocialAccount{{
				ID:             "account-a",
				ProfileID:      "profile-other",
				ExternalUserID: pgtype.Text{String: "managed-a", Valid: true},
			}},
			profileID:      "profile-a",
			externalUserID: "managed-a",
			want:           Decision{Kind: Conflict},
		},
		{
			name: "multiple active matches conflict even when one is an exact reconnect",
			matches: []db.SocialAccount{
				{
					ID:             "account-a",
					ProfileID:      "profile-a",
					ExternalUserID: pgtype.Text{String: "managed-a", Valid: true},
				},
				{
					ID:             "account-b",
					ProfileID:      "profile-b",
					ExternalUserID: pgtype.Text{String: "managed-b", Valid: true},
				},
			},
			profileID:      "profile-a",
			externalUserID: "managed-a",
			want:           Decision{Kind: Conflict},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := decide(test.matches, test.profileID, test.externalUserID)
			if got != test.want {
				t.Fatalf("decide() = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestConnectOwnershipQuery(t *testing.T) {
	source, err := os.ReadFile("../db/queries/social_accounts.sql")
	if err != nil {
		t.Fatalf("read social account queries: %v", err)
	}

	query := extractSQLQuery(t, string(source), "ListActiveAccountsByWorkspaceProviderIdentity")
	compact := strings.Join(strings.Fields(strings.ToLower(query)), " ")
	contract := strings.ReplaceAll(compact, "::text", "")

	for _, want := range []string{
		"select sa.id, sa.profile_id, sa.platform, sa.access_token",
		"sa.connection_type, sa.connect_session_id, sa.external_user_id, sa.external_user_email",
		"sa.last_refreshed_at, sa.x_app_mode from social_accounts sa",
		"join profiles p on p.id = sa.profile_id",
		"p.workspace_id = @workspace_id",
		"sa.platform = @platform",
		"sa.status = 'active'",
		"sa.disconnected_at is null",
		"@platform = 'instagram' and sa.metadata->>'instagram_webhook_user_id' = @provider_identity",
		"@platform <> 'instagram' and sa.external_account_id = @provider_identity",
		"order by sa.connected_at desc, sa.id",
		"for update of sa",
	} {
		if !strings.Contains(contract, want) {
			t.Errorf("ownership query missing %q: %s", want, compact)
		}
	}

	for _, forbidden := range []string{
		"select sa.*",
		"sa.profile_id =",
		"sa.connection_type =",
		"limit ",
	} {
		if strings.Contains(compact, forbidden) {
			t.Errorf("ownership query must include every workspace profile and both managed/BYO rows; found %q in %s", forbidden, compact)
		}
	}

	generated, err := os.ReadFile("../db/social_accounts.sql.go")
	if err != nil {
		t.Fatalf("read generated social account queries: %v", err)
	}
	paramsPattern := regexp.MustCompile(`(?s)type ListActiveAccountsByWorkspaceProviderIdentityParams struct \{.*?ProviderIdentity\s+string\s+`)
	if !paramsPattern.Match(generated) {
		t.Fatal("generated ownership query must accept provider identity as text")
	}
}

func TestStoreCheckIsReadOnlyClassification(t *testing.T) {
	queries := &fakeOwnershipQueries{
		matches: []db.SocialAccount{{
			ID:             "account-a",
			ProfileID:      "profile-a",
			ExternalUserID: pgtype.Text{String: "managed-a", Valid: true},
		}},
	}
	store := &Store{queries: queries}

	decision, err := store.Check(context.Background(), OwnershipKey{
		WorkspaceID:      "workspace-a",
		ProfileID:        "profile-a",
		Platform:         "instagram",
		ProviderIdentity: "provider-a",
		ExternalUserID:   "managed-a",
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if decision != (Decision{Kind: Reconnect, AccountID: "account-a"}) {
		t.Fatalf("Check() decision = %+v", decision)
	}
	if queries.lookupCalls != 1 || queries.mutationCalls != 0 {
		t.Fatalf("lookup calls = %d, mutation calls = %d", queries.lookupCalls, queries.mutationCalls)
	}
	if got := queries.lookupParams; got.WorkspaceID != "workspace-a" || got.Platform != "instagram" || got.ProviderIdentity != "provider-a" {
		t.Fatalf("lookup params = %+v", got)
	}
}

func TestStoreSaveRepeatsLookupUnderAdvisoryLock(t *testing.T) {
	var events []string
	checkQueries := &fakeOwnershipQueries{events: &events}
	authoritativeQueries := &fakeOwnershipQueries{
		events: &events,
		matches: []db.SocialAccount{{
			ID:             "account-b",
			ProfileID:      "profile-a",
			ExternalUserID: pgtype.Text{String: "managed-b", Valid: true},
		}},
	}
	tx := &fakeOwnershipTx{events: &events}
	store := &Store{
		queries: checkQueries,
		beginTx: func(context.Context) (ownershipTx, error) {
			events = append(events, "begin")
			return tx, nil
		},
		queriesFor: func(db.DBTX) ownershipQueries { return authoritativeQueries },
	}

	decision, err := store.Check(context.Background(), OwnershipKey{
		WorkspaceID:      "workspace-a",
		ProfileID:        "profile-a",
		Platform:         "facebook",
		ProviderIdentity: "provider-a",
		ExternalUserID:   "managed-a",
	})
	if err != nil || decision.Kind != Create {
		t.Fatalf("Check() = %+v, %v", decision, err)
	}

	_, err = store.Save(context.Background(), SaveRequest{
		WorkspaceID:      "workspace-a",
		ProfileID:        "profile-a",
		Platform:         "facebook",
		ProviderIdentity: "provider-a",
		ExternalUserID:   "managed-a",
	})
	if !errors.Is(err, ErrOwnershipConflict) {
		t.Fatalf("Save() error = %v, want ownership conflict", err)
	}
	if err.Error() != "ACCOUNT_OWNERSHIP_CONFLICT" {
		t.Fatalf("conflict error leaked details: %q", err.Error())
	}
	var conflict *OwnershipConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("Save() error type = %T, want *OwnershipConflictError", err)
	}
	if authoritativeQueries.mutationCalls != 0 || tx.commitCalls != 0 {
		t.Fatalf("mutation calls = %d, commit calls = %d", authoritativeQueries.mutationCalls, tx.commitCalls)
	}
	wantEvents := []string{"lookup", "begin", "lock", "lookup", "rollback"}
	if strings.Join(events, ",") != strings.Join(wantEvents, ",") {
		t.Fatalf("events = %v, want %v", events, wantEvents)
	}
	if tx.lockSQL != "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))" {
		t.Fatalf("lock SQL = %q", tx.lockSQL)
	}
	if len(tx.lockArgs) != 1 || tx.lockArgs[0] != "workspace-a\x00facebook\x00provider-a" {
		t.Fatalf("lock args = %#v", tx.lockArgs)
	}
	if got := authoritativeQueries.lookupParams; got.WorkspaceID != "workspace-a" || got.Platform != "facebook" || got.ProviderIdentity != "provider-a" {
		t.Fatalf("authoritative lookup params = %+v", got)
	}
}

func TestStoreSaveAppliesOnlyCreateOrReconnect(t *testing.T) {
	tests := []struct {
		name             string
		platform         string
		matches          []db.SocialAccount
		wantAccountID    string
		wantRefreshCalls int
		wantUpsertCalls  int
		wantCreateCalls  int
	}{
		{
			name:     "reconnect refreshes the exact account",
			platform: "instagram",
			matches: []db.SocialAccount{{
				ID:             "existing-a",
				ProfileID:      "profile-a",
				ExternalUserID: pgtype.Text{String: "managed-a", Valid: true},
			}},
			wantAccountID:    "existing-a",
			wantRefreshCalls: 1,
		},
		{
			name:            "non-Bluesky create uses managed upsert",
			platform:        "threads",
			wantAccountID:   "upserted-a",
			wantUpsertCalls: 1,
		},
		{
			name:            "Bluesky create uses insert",
			platform:        "bluesky",
			wantAccountID:   "created-a",
			wantCreateCalls: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := &fakeOwnershipTx{}
			queries := &fakeOwnershipQueries{
				matches:       test.matches,
				refreshResult: db.SocialAccount{ID: "existing-a"},
				upsertResult:  db.SocialAccount{ID: "upserted-a"},
				createResult:  db.SocialAccount{ID: "created-a"},
			}
			store := &Store{
				beginTx:    func(context.Context) (ownershipTx, error) { return tx, nil },
				queriesFor: func(db.DBTX) ownershipQueries { return queries },
			}

			account, err := store.Save(context.Background(), SaveRequest{
				WorkspaceID:      "workspace-a",
				ProfileID:        "profile-a",
				Platform:         test.platform,
				ProviderIdentity: "provider-a",
				ExternalUserID:   "managed-a",
				Refresh:          db.RefreshConnectedSocialAccountParams{ID: "caller-controlled-id"},
			})
			if err != nil {
				t.Fatalf("Save() error = %v", err)
			}
			if account.ID != test.wantAccountID {
				t.Fatalf("Save() account ID = %q, want %q", account.ID, test.wantAccountID)
			}
			if queries.refreshCalls != test.wantRefreshCalls || queries.upsertCalls != test.wantUpsertCalls || queries.createCalls != test.wantCreateCalls {
				t.Fatalf("refresh/upsert/create calls = %d/%d/%d", queries.refreshCalls, queries.upsertCalls, queries.createCalls)
			}
			if test.wantRefreshCalls == 1 && queries.refreshParams.ID != "existing-a" {
				t.Fatalf("refresh account ID = %q, want DB-derived existing-a", queries.refreshParams.ID)
			}
			if tx.commitCalls != 1 {
				t.Fatalf("commit calls = %d, want 1", tx.commitCalls)
			}
		})
	}
}

func TestStoreSaveSerializesConcurrentOwnershipDecisions(t *testing.T) {
	database := &serializedOwnershipDB{}
	store := &Store{
		beginTx: database.begin,
		queriesFor: func(tx db.DBTX) ownershipQueries {
			return &serializedOwnershipQueries{tx: tx.(*serializedOwnershipTx)}
		},
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	for _, externalUserID := range []string{"managed-a", "managed-b"} {
		externalUserID := externalUserID
		go func() {
			<-start
			_, err := store.Save(context.Background(), SaveRequest{
				WorkspaceID:      "workspace-a",
				ProfileID:        "profile-a",
				Platform:         "threads",
				ProviderIdentity: "provider-a",
				ExternalUserID:   externalUserID,
				Upsert: db.UpsertManagedSocialAccountParams{
					ProfileID:      "profile-a",
					Platform:       "threads",
					ExternalUserID: pgtype.Text{String: externalUserID, Valid: true},
				},
			})
			results <- err
		}()
	}
	close(start)

	var successes, conflicts int
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrOwnershipConflict):
			conflicts++
		default:
			t.Fatalf("unexpected Save() error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes/conflicts = %d/%d, want 1/1", successes, conflicts)
	}
	database.mu.Lock()
	defer database.mu.Unlock()
	if database.upsertCalls != 1 || len(database.accounts) != 1 {
		t.Fatalf("upsert calls = %d, accounts = %d", database.upsertCalls, len(database.accounts))
	}
}

type fakeOwnershipQueries struct {
	events        *[]string
	matches       []db.SocialAccount
	lookupParams  db.ListActiveAccountsByWorkspaceProviderIdentityParams
	lookupCalls   int
	mutationCalls int
	refreshCalls  int
	upsertCalls   int
	createCalls   int
	refreshParams db.RefreshConnectedSocialAccountParams
	refreshResult db.SocialAccount
	upsertResult  db.SocialAccount
	createResult  db.SocialAccount
}

func (f *fakeOwnershipQueries) ListActiveAccountsByWorkspaceProviderIdentity(_ context.Context, params db.ListActiveAccountsByWorkspaceProviderIdentityParams) ([]db.SocialAccount, error) {
	f.lookupCalls++
	f.lookupParams = params
	f.record("lookup")
	return f.matches, nil
}

func (f *fakeOwnershipQueries) RefreshConnectedSocialAccount(_ context.Context, params db.RefreshConnectedSocialAccountParams) (db.SocialAccount, error) {
	f.mutationCalls++
	f.refreshCalls++
	f.refreshParams = params
	f.record("refresh")
	return f.refreshResult, nil
}

func (f *fakeOwnershipQueries) UpsertManagedSocialAccount(_ context.Context, _ db.UpsertManagedSocialAccountParams) (db.SocialAccount, error) {
	f.mutationCalls++
	f.upsertCalls++
	f.record("upsert")
	return f.upsertResult, nil
}

func (f *fakeOwnershipQueries) CreateManagedSocialAccount(_ context.Context, _ db.CreateManagedSocialAccountParams) (db.SocialAccount, error) {
	f.mutationCalls++
	f.createCalls++
	f.record("create")
	return f.createResult, nil
}

func (f *fakeOwnershipQueries) record(event string) {
	if f.events != nil {
		*f.events = append(*f.events, event)
	}
}

type fakeOwnershipTx struct {
	events      *[]string
	lockSQL     string
	lockArgs    []any
	commitCalls int
}

func (f *fakeOwnershipTx) Exec(_ context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	f.lockSQL = sql
	f.lockArgs = args
	f.record("lock")
	return pgconn.NewCommandTag("SELECT 1"), nil
}

func (*fakeOwnershipTx) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	panic("unexpected Query call")
}

func (*fakeOwnershipTx) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	panic("unexpected QueryRow call")
}

func (f *fakeOwnershipTx) Commit(context.Context) error {
	f.commitCalls++
	f.record("commit")
	return nil
}

func (f *fakeOwnershipTx) Rollback(context.Context) error {
	f.record("rollback")
	return nil
}

func (f *fakeOwnershipTx) record(event string) {
	if f.events != nil {
		*f.events = append(*f.events, event)
	}
}

type serializedOwnershipDB struct {
	mu          sync.Mutex
	accounts    []db.SocialAccount
	upsertCalls int
}

func (d *serializedOwnershipDB) begin(context.Context) (ownershipTx, error) {
	return &serializedOwnershipTx{database: d}, nil
}

type serializedOwnershipTx struct {
	database *serializedOwnershipDB
	locked   bool
}

func (tx *serializedOwnershipTx) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	tx.database.mu.Lock()
	tx.locked = true
	return pgconn.NewCommandTag("SELECT 1"), nil
}

func (*serializedOwnershipTx) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	panic("unexpected Query call")
}

func (*serializedOwnershipTx) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	panic("unexpected QueryRow call")
}

func (tx *serializedOwnershipTx) Commit(context.Context) error {
	tx.unlock()
	return nil
}

func (tx *serializedOwnershipTx) Rollback(context.Context) error {
	tx.unlock()
	return nil
}

func (tx *serializedOwnershipTx) unlock() {
	if tx.locked {
		tx.locked = false
		tx.database.mu.Unlock()
	}
}

type serializedOwnershipQueries struct {
	tx *serializedOwnershipTx
}

func (q *serializedOwnershipQueries) ListActiveAccountsByWorkspaceProviderIdentity(context.Context, db.ListActiveAccountsByWorkspaceProviderIdentityParams) ([]db.SocialAccount, error) {
	return append([]db.SocialAccount(nil), q.tx.database.accounts...), nil
}

func (*serializedOwnershipQueries) RefreshConnectedSocialAccount(context.Context, db.RefreshConnectedSocialAccountParams) (db.SocialAccount, error) {
	panic("unexpected refresh")
}

func (q *serializedOwnershipQueries) UpsertManagedSocialAccount(_ context.Context, params db.UpsertManagedSocialAccountParams) (db.SocialAccount, error) {
	account := db.SocialAccount{
		ID:             "created-account",
		ProfileID:      params.ProfileID,
		Platform:       params.Platform,
		ExternalUserID: params.ExternalUserID,
	}
	q.tx.database.accounts = append(q.tx.database.accounts, account)
	q.tx.database.upsertCalls++
	return account, nil
}

func (*serializedOwnershipQueries) CreateManagedSocialAccount(context.Context, db.CreateManagedSocialAccountParams) (db.SocialAccount, error) {
	panic("unexpected create")
}

func extractSQLQuery(t *testing.T, source, name string) string {
	t.Helper()

	pattern := regexp.MustCompile(`(?ms)^-- name: ` + regexp.QuoteMeta(name) + ` [^\n]*\n(.*?)(?:^-- name: |\z)`)
	match := pattern.FindStringSubmatch(source)
	if len(match) != 2 {
		t.Fatalf("query %s is missing", name)
	}
	return match[1]
}
