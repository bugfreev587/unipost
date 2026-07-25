package socialconnections

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestSaveVerifiedCreatesConnectionAndBinding(t *testing.T) {
	queries := &fakeConnectionQueries{
		canonicalErr: pgx.ErrNoRows,
		profile:      db.Profile{ID: "profile-a", WorkspaceID: "workspace-a"},
		created: db.SocialConnection{
			ID: "connection-a", WorkspaceID: "workspace-a", Platform: "twitter",
			ProviderIdentity: pgtype.Text{String: "provider-a", Valid: true},
		},
		bound: db.SocialAccount{ID: "account-a", ProfileID: "profile-a"},
	}
	store, tx := newFakePostgresStore(queries)

	account, err := store.SaveVerified(context.Background(), SaveOAuthReuse, byoCredentialInput())
	if err != nil {
		t.Fatal(err)
	}
	if account.ID != "account-a" || queries.createCalls != 1 || queries.refreshCalls != 0 || queries.bindCalls != 1 {
		t.Fatalf("save result=%+v create=%d refresh=%d bind=%d", account, queries.createCalls, queries.refreshCalls, queries.bindCalls)
	}
	if queries.bindParams.ConnectionID.String != "connection-a" || !queries.bindParams.ConnectionID.Valid {
		t.Fatalf("binding connection = %+v, want connection-a", queries.bindParams.ConnectionID)
	}
	if tx.commitCalls != 1 || tx.lockCalls != 1 {
		t.Fatalf("transaction commits=%d locks=%d, want 1 each", tx.commitCalls, tx.lockCalls)
	}
}

func TestSaveVerifiedFailsClosedWhenLegacyIdentityHasNoConnection(t *testing.T) {
	queries := &fakeConnectionQueries{
		canonicalErr: pgx.ErrNoRows,
		profile:      db.Profile{ID: "profile-a", WorkspaceID: "workspace-a"},
		legacyMatches: []db.SocialAccount{{
			ID: "quarantined-account", ProfileID: "profile-a", Platform: "twitter",
			ExternalAccountID: "provider-a",
		}},
	}
	store, tx := newFakePostgresStore(queries)
	_, err := store.SaveVerified(context.Background(), SaveOAuthReuse, byoCredentialInput())
	if !errors.Is(err, ErrLegacyBinding) {
		t.Fatalf("SaveVerified() error = %v, want ErrLegacyBinding", err)
	}
	if queries.createCalls != 0 || queries.bindCalls != 0 || tx.commitCalls != 0 {
		t.Fatalf("quarantined identity mutated state: create=%d bind=%d commits=%d", queries.createCalls, queries.bindCalls, tx.commitCalls)
	}
}

func TestSaveVerifiedReusesDisconnectedConnectionAndCreatesSiblingBinding(t *testing.T) {
	queries := &fakeConnectionQueries{
		profile: db.Profile{ID: "profile-b", WorkspaceID: "workspace-a"},
		canonical: db.SocialConnection{
			ID: "stable-connection", WorkspaceID: "workspace-a", Platform: "twitter",
			ProviderIdentity: pgtype.Text{String: "provider-a", Valid: true},
			ConnectionType:   "managed", ExternalUserID: pgtype.Text{String: "managed-a", Valid: true},
			Status: "disconnected",
		},
		refreshed: db.SocialConnection{ID: "stable-connection", WorkspaceID: "workspace-a", Status: "active"},
		bound:     db.SocialAccount{ID: "profile-b-binding", ProfileID: "profile-b"},
	}
	store, _ := newFakePostgresStore(queries)
	input := managedCredentialInput()
	input.ProfileID = "profile-b"

	account, err := store.SaveVerified(context.Background(), SaveManagedReuse, input)
	if err != nil {
		t.Fatal(err)
	}
	if account.ID != "profile-b-binding" || queries.refreshParams.ID != "stable-connection" {
		t.Fatalf("account=%+v refresh=%+v", account, queries.refreshParams)
	}
	if got := queries.bindParams.ConnectionID.String; got != "stable-connection" {
		t.Fatalf("binding connection = %q, want stable-connection", got)
	}
}

func TestSaveVerifiedRejectsConnectionOwnershipConflicts(t *testing.T) {
	tests := []struct {
		name       string
		connection db.SocialConnection
		mode       SaveMode
		input      CredentialInput
		want       error
	}{
		{
			name: "different managed owner",
			connection: db.SocialConnection{ID: "connection-a", ConnectionType: "managed",
				ExternalUserID: pgtype.Text{String: "managed-b", Valid: true}},
			mode: SaveManagedReuse, input: managedCredentialInput(), want: ErrOwnershipConflict,
		},
		{
			name:       "managed cannot reuse BYO",
			connection: db.SocialConnection{ID: "connection-a", ConnectionType: "byo"},
			mode:       SaveManagedReuse, input: managedCredentialInput(), want: ErrOwnershipConflict,
		},
		{
			name: "BYO cannot reuse managed",
			connection: db.SocialConnection{ID: "connection-a", ConnectionType: "managed",
				ExternalUserID: pgtype.Text{String: "managed-a", Valid: true}},
			mode: SaveOAuthReuse, input: byoCredentialInput(), want: ErrOwnershipConflict,
		},
		{
			name:       "direct connect preserves duplicate conflict",
			connection: db.SocialConnection{ID: "connection-a", ConnectionType: "byo"},
			mode:       SaveDirectCreate, input: byoCredentialInput(), want: ErrAlreadyConnected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queries := &fakeConnectionQueries{
				profile:   db.Profile{ID: tt.input.ProfileID, WorkspaceID: tt.input.WorkspaceID},
				canonical: tt.connection,
			}
			store, tx := newFakePostgresStore(queries)
			_, err := store.SaveVerified(context.Background(), tt.mode, tt.input)
			if !errors.Is(err, tt.want) {
				t.Fatalf("SaveVerified() error = %v, want %v", err, tt.want)
			}
			if queries.refreshCalls != 0 || queries.bindCalls != 0 || tx.commitCalls != 0 {
				t.Fatalf("conflict mutated state: refresh=%d bind=%d commits=%d", queries.refreshCalls, queries.bindCalls, tx.commitCalls)
			}
		})
	}
}

func TestBindExistingResolvesConnectionWithoutAcceptingCallerConnectionID(t *testing.T) {
	queries := &fakeConnectionQueries{
		profile: db.Profile{ID: "profile-b", WorkspaceID: "workspace-a"},
		source: db.GetResolvedSocialAccountByIDAndWorkspaceRow{
			ID: "account-a", ProfileID: "profile-a", Platform: "twitter",
			ExternalAccountID: "provider-a", ConnectionID: pgtype.Text{String: "connection-a", Valid: true},
			BindingStatus: "active", ResolvedAccessToken: "encrypted-access",
			ResolvedConnectionType: "managed", ResolvedExternalUserID: pgtype.Text{String: "managed-a", Valid: true},
		},
		connection: db.SocialConnection{ID: "connection-a", WorkspaceID: "workspace-a", ConnectionType: "managed",
			ExternalUserID: pgtype.Text{String: "managed-a", Valid: true}},
		bound: db.SocialAccount{ID: "account-b", ProfileID: "profile-b"},
	}
	store, _ := newFakePostgresStore(queries)

	account, err := store.BindExisting(context.Background(), "workspace-a", "account-a", "profile-b", "managed-a")
	if err != nil {
		t.Fatal(err)
	}
	if account.ID != "account-b" || queries.getConnectionParams.ID != "connection-a" {
		t.Fatalf("bound account=%+v locked connection=%+v", account, queries.getConnectionParams)
	}
}

func TestBindExistingRejectsLegacyNullConnection(t *testing.T) {
	queries := &fakeConnectionQueries{
		source: db.GetResolvedSocialAccountByIDAndWorkspaceRow{ID: "legacy-account", BindingStatus: "active"},
	}
	store, tx := newFakePostgresStore(queries)
	_, err := store.BindExisting(context.Background(), "workspace-a", "legacy-account", "profile-b", "")
	if !errors.Is(err, ErrLegacyBinding) {
		t.Fatalf("BindExisting() error = %v, want ErrLegacyBinding", err)
	}
	if queries.bindCalls != 0 || tx.commitCalls != 0 {
		t.Fatalf("legacy bind mutated state: bind=%d commits=%d", queries.bindCalls, tx.commitCalls)
	}
}

func byoCredentialInput() CredentialInput {
	return CredentialInput{
		WorkspaceID: "workspace-a", ProfileID: "profile-a", Platform: "twitter",
		ProviderIdentity: "provider-a", ExternalAccountID: "provider-a",
		AccessToken: "encrypted-access", RefreshToken: "encrypted-refresh",
		AccountName: "Alice", Scopes: []string{"tweet.write"}, Metadata: []byte(`{}`),
		TokenExpiresAt: time.Now().Add(time.Hour), Ownership: Ownership{ConnectionType: "byo"},
	}
}

func managedCredentialInput() CredentialInput {
	input := byoCredentialInput()
	input.Ownership = Ownership{ConnectionType: "managed", ExternalUserID: "managed-a", ExternalUserEmail: "a@example.com"}
	return input
}

type fakeConnectionQueries struct {
	canonical     db.SocialConnection
	canonicalErr  error
	created       db.SocialConnection
	refreshed     db.SocialConnection
	bound         db.SocialAccount
	source        db.GetResolvedSocialAccountByIDAndWorkspaceRow
	connection    db.SocialConnection
	profile       db.Profile
	legacyMatches []db.SocialAccount

	createCalls  int
	refreshCalls int
	bindCalls    int

	refreshParams       db.RefreshSocialConnectionParams
	bindParams          db.CreateOrReactivateSocialAccountBindingParams
	getConnectionParams db.GetSocialConnectionForUpdateParams
}

func (f *fakeConnectionQueries) FindCanonicalSocialConnectionForUpdate(context.Context, db.FindCanonicalSocialConnectionForUpdateParams) (db.SocialConnection, error) {
	return f.canonical, f.canonicalErr
}

func (f *fakeConnectionQueries) ListActiveAccountsByWorkspaceProviderIdentity(context.Context, db.ListActiveAccountsByWorkspaceProviderIdentityParams) ([]db.SocialAccount, error) {
	return f.legacyMatches, nil
}

func (f *fakeConnectionQueries) CreateSocialConnection(context.Context, db.CreateSocialConnectionParams) (db.SocialConnection, error) {
	f.createCalls++
	return f.created, nil
}

func (f *fakeConnectionQueries) RefreshSocialConnection(_ context.Context, params db.RefreshSocialConnectionParams) (db.SocialConnection, error) {
	f.refreshCalls++
	f.refreshParams = params
	return f.refreshed, nil
}

func (f *fakeConnectionQueries) CreateOrReactivateSocialAccountBinding(_ context.Context, params db.CreateOrReactivateSocialAccountBindingParams) (db.SocialAccount, error) {
	f.bindCalls++
	f.bindParams = params
	return f.bound, nil
}

func (f *fakeConnectionQueries) GetResolvedSocialAccountByIDAndWorkspace(context.Context, db.GetResolvedSocialAccountByIDAndWorkspaceParams) (db.GetResolvedSocialAccountByIDAndWorkspaceRow, error) {
	return f.source, nil
}

func (f *fakeConnectionQueries) GetSocialConnectionForUpdate(_ context.Context, params db.GetSocialConnectionForUpdateParams) (db.SocialConnection, error) {
	f.getConnectionParams = params
	return f.connection, nil
}

func (f *fakeConnectionQueries) GetProfile(context.Context, string) (db.Profile, error) {
	return f.profile, nil
}

type fakeConnectionTx struct {
	lockCalls   int
	commitCalls int
}

func (f *fakeConnectionTx) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	f.lockCalls++
	return pgconn.NewCommandTag("SELECT 1"), nil
}

func (*fakeConnectionTx) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	panic("unexpected Query")
}

func (*fakeConnectionTx) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	panic("unexpected QueryRow")
}

func (f *fakeConnectionTx) Commit(context.Context) error {
	f.commitCalls++
	return nil
}

func (*fakeConnectionTx) Rollback(context.Context) error { return nil }

func newFakePostgresStore(queries *fakeConnectionQueries) (*PostgresStore, *fakeConnectionTx) {
	tx := &fakeConnectionTx{}
	return &PostgresStore{
		beginTx:    func(context.Context) (connectionTx, error) { return tx, nil },
		queriesFor: func(db.DBTX) connectionQueries { return queries },
	}, tx
}
