package xinbox

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestXInboxProviderUserRowsFailEntireLookupOnConversionError(t *testing.T) {
	rows := []providerUserAccountRow{
		{
			id: "account-1", workspaceID: "workspace-1", externalUserID: pgtype.Text{String: "owner-1", Valid: true},
			externalAccountID: "provider-1", appMode: string(AppModeUniPostManaged),
		},
		{
			id: "account-2", workspaceID: "workspace-2", externalUserID: pgtype.Text{String: "owner-2", Valid: true},
			externalAccountID: "provider-1", appMode: "invalid-mode",
		},
	}
	accounts, err := inboxAccountsFromProviderRows(rows)
	if err == nil || errors.Is(err, ErrInboxAccountNotFound) || !strings.Contains(err.Error(), "invalid persisted X app mode") {
		t.Fatalf("error = %v, want wrapped persisted-mode conversion error distinct from not found", err)
	}
	if accounts != nil {
		t.Fatalf("partial accounts escaped failed lookup: %#v", accounts)
	}
}

func TestPostgresIngestionStoreProviderLookupHandlesZeroOneAndManyRowsWithRouteIsolation(t *testing.T) {
	validRow := func(id string) providerUserAccountRow {
		return providerUserAccountRow{
			id: id, workspaceID: "workspace-" + id,
			externalUserID:    pgtype.Text{String: "owner-" + id, Valid: true},
			externalAccountID: "provider-user", appMode: string(AppModeUniPostManaged),
		}
	}
	for _, test := range []struct {
		name string
		rows []providerUserAccountRow
	}{
		{name: "zero"},
		{name: "one", rows: []providerUserAccountRow{validRow("one")}},
		{name: "two", rows: []providerUserAccountRow{validRow("one"), validRow("two")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fakeDB := &providerLookupDB{rows: &providerLookupRows{rows: test.rows}}
			store := NewPostgresIngestionStore(db.New(fakeDB), nil, "managed-route")
			accounts, err := store.AccountsForProviderUser(context.Background(), "workspace-route", "provider-user")
			if err != nil {
				t.Fatalf("AccountsForProviderUser: %v", err)
			}
			if len(accounts) != len(test.rows) {
				t.Fatalf("accounts = %#v, want %d", accounts, len(test.rows))
			}
			if !strings.Contains(fakeDB.query, "FindXInboxAccountsForProviderUserApp") {
				t.Fatalf("query = %q", fakeDB.query)
			}
			wantArgs := []any{"provider-user", "workspace-route", "managed-route"}
			if len(fakeDB.args) != len(wantArgs) {
				t.Fatalf("query args = %#v", fakeDB.args)
			}
			for index := range wantArgs {
				if fakeDB.args[index] != wantArgs[index] {
					t.Fatalf("query args = %#v, want %#v", fakeDB.args, wantArgs)
				}
			}
		})
	}
}

func TestPostgresIngestionStoreProviderLookupPropagatesScanError(t *testing.T) {
	scanFailure := errors.New("scan provider candidate")
	fakeDB := &providerLookupDB{rows: &providerLookupRows{
		rows: []providerUserAccountRow{{id: "account-1"}}, scanErrAt: 1, scanErr: scanFailure,
	}}
	store := NewPostgresIngestionStore(db.New(fakeDB), nil, "managed-route")
	accounts, err := store.AccountsForProviderUser(context.Background(), "workspace-route", "provider-user")
	if !errors.Is(err, scanFailure) || accounts != nil {
		t.Fatalf("accounts = %#v, error = %v; want scan failure", accounts, err)
	}
}

func TestPostgresIngestionStoreNilQueriesFailSafely(t *testing.T) {
	var typedNil *PostgresIngestionStore
	for _, test := range []struct {
		name string
		call func() error
	}{
		{name: "typed nil exact account", call: func() error {
			_, err := typedNil.AccountForApp(context.Background(), "route", "account")
			return err
		}},
		{name: "typed nil provider account", call: func() error {
			_, err := typedNil.AccountsForProviderUser(context.Background(), "route", "provider")
			return err
		}},
		{name: "nil queries exact account", call: func() error {
			_, err := (&PostgresIngestionStore{}).AccountForApp(context.Background(), "route", "account")
			return err
		}},
		{name: "nil queries provider account", call: func() error {
			_, err := (&PostgresIngestionStore{}).AccountsForProviderUser(context.Background(), "route", "provider")
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil || !strings.Contains(err.Error(), "not configured") {
				t.Fatalf("error = %v, want safe configuration error", err)
			}
		})
	}
}

type providerLookupDB struct {
	query string
	args  []any
	rows  pgx.Rows
}

func (*providerLookupDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected Exec")
}

func (f *providerLookupDB) Query(_ context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	f.query = query
	f.args = args
	return f.rows, nil
}

func (*providerLookupDB) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	return providerLookupRow{err: errors.New("unexpected QueryRow")}
}

type providerLookupRow struct{ err error }

func (r providerLookupRow) Scan(...any) error { return r.err }

type providerLookupRows struct {
	rows      []providerUserAccountRow
	index     int
	scanErrAt int
	scanErr   error
}

func (*providerLookupRows) Close()                                       {}
func (*providerLookupRows) Err() error                                   { return nil }
func (*providerLookupRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (*providerLookupRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (*providerLookupRows) Values() ([]any, error)                       { return nil, errors.New("unused") }
func (*providerLookupRows) RawValues() [][]byte                          { return nil }
func (*providerLookupRows) Conn() *pgx.Conn                              { return nil }

func (r *providerLookupRows) Next() bool {
	if r.index >= len(r.rows) {
		return false
	}
	r.index++
	return true
}

func (r *providerLookupRows) Scan(dest ...any) error {
	if r.index == 0 || r.index > len(r.rows) {
		return errors.New("scan without current row")
	}
	if r.scanErrAt == r.index {
		return r.scanErr
	}
	if len(dest) != 10 {
		return errors.New("unexpected provider lookup column count")
	}
	row := r.rows[r.index-1]
	*dest[0].(*string) = row.id
	*dest[1].(*string) = row.workspaceID
	*dest[2].(*pgtype.Text) = row.externalUserID
	*dest[3].(*string) = row.externalAccountID
	*dest[4].(*string) = row.accountName
	*dest[5].(*string) = row.appMode
	*dest[6].(*[]string) = row.scopes
	*dest[7].(*string) = row.connectionType
	*dest[8].(*string) = row.planID
	*dest[9].(*bool) = row.planAllowsInbox
	return nil
}
