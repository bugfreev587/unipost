package trials

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapCreateErrorMapsOpenGrantUniqueConstraintToConflict(t *testing.T) {
	err := mapCreateError(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "workspace_trial_grants_open_workspace_idx",
	})
	if !errors.Is(err, ErrOpenGrantExists) {
		t.Fatalf("error = %v, want ErrOpenGrantExists", err)
	}
}

func TestMapCreateErrorPreservesOtherDatabaseErrors(t *testing.T) {
	want := errors.New("database unavailable")
	if got := mapCreateError(want); !errors.Is(got, want) {
		t.Fatalf("error = %v, want original", got)
	}
}
