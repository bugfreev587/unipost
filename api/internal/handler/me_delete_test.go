package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestMeDeleteSynchronouslyDeletesClerkThenDatabase(t *testing.T) {
	calls := []string{}
	store := &meDeleteDB{calls: &calls}
	h := NewMeHandler(db.New(store), nil, nil)
	h.deleteClerkUser = func(_ context.Context, userID string) error {
		calls = append(calls, "clerk:"+userID)
		return nil
	}
	rec := httptest.NewRecorder()

	h.Delete(rec, authenticatedMeDeleteRequest("user_synthetic"))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	want := []string{"clerk:user_synthetic", "database:user_synthetic"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("delete calls = %#v, want %#v", calls, want)
	}
}

func TestMeDeleteReportsDatabaseFailureAfterClerkDeletion(t *testing.T) {
	calls := []string{}
	store := &meDeleteDB{calls: &calls, deleteErr: errors.New("database unavailable")}
	h := NewMeHandler(db.New(store), nil, nil)
	h.deleteClerkUser = func(_ context.Context, userID string) error {
		calls = append(calls, "clerk:"+userID)
		return nil
	}
	rec := httptest.NewRecorder()

	h.Delete(rec, authenticatedMeDeleteRequest("user_synthetic"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	want := []string{"clerk:user_synthetic", "database:user_synthetic"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("delete calls = %#v, want %#v", calls, want)
	}
	if !strings.Contains(rec.Body.String(), `"normalized_code":"internal_error"`) {
		t.Fatalf("body = %q, want sanitized internal_error", rec.Body.String())
	}
}

func authenticatedMeDeleteRequest(userID string) *http.Request {
	req := httptest.NewRequest(http.MethodDelete, "/v1/me", nil)
	return req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, userID))
}

type meDeleteDB struct {
	calls     *[]string
	deleteErr error
}

func (f *meDeleteDB) Exec(_ context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	if strings.Contains(query, "-- name: DeleteUser :exec") {
		userID, _ := args[0].(string)
		*f.calls = append(*f.calls, "database:"+userID)
		return pgconn.CommandTag{}, f.deleteErr
	}
	return pgconn.CommandTag{}, nil
}

func (*meDeleteDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, nil
}

func (*meDeleteDB) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	return meDeleteRow{}
}

type meDeleteRow struct{}

func (meDeleteRow) Scan(...interface{}) error {
	return pgx.ErrNoRows
}
