package xinbox

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
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
	if !errors.Is(err, ErrInboxAccountNotFound) {
		t.Fatalf("error = %v, want conversion failure", err)
	}
	if accounts != nil {
		t.Fatalf("partial accounts escaped failed lookup: %#v", accounts)
	}
}
