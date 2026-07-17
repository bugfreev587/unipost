package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestLogWritesStructuredEventAndOmitsUnserializableOptionalJSON(t *testing.T) {
	store := &auditTestDB{}
	Log(context.Background(), db.New(store), Event{
		WorkspaceID:  "ws_1",
		ActorUserID:  "user_1",
		Action:       ActionAPIKeyCreated,
		ResourceType: "api_key",
		ResourceID:   "key_1",
		Category:     CategoryConfig,
		After:        map[string]any{"name": "Release key"},
		Metadata:     func() {},
	})

	if store.calls != 1 {
		t.Fatalf("audit writes=%d, want 1", store.calls)
	}
	if got := string(store.args[10].([]byte)); got != `{"name":"Release key"}` {
		t.Fatalf("after_json=%q", got)
	}
	metadata, ok := store.args[11].([]byte)
	if !ok || len(metadata) != 0 {
		t.Fatalf("metadata=%#v, want empty JSON for unserializable value", store.args[11])
	}
}

func TestLogSwallowsDatabaseFailure(t *testing.T) {
	store := &auditTestDB{err: errors.New("database unavailable")}

	Log(context.Background(), db.New(store), Event{
		WorkspaceID:  "ws_1",
		Action:       ActionAPIKeyRevoked,
		ResourceType: "api_key",
	})

	if store.calls != 1 {
		t.Fatalf("audit writes=%d, want 1", store.calls)
	}
}

type auditTestDB struct {
	calls int
	args  []any
	err   error
}

func (f *auditTestDB) Exec(_ context.Context, _ string, args ...interface{}) (pgconn.CommandTag, error) {
	f.calls++
	f.args = append([]any(nil), args...)
	return pgconn.CommandTag{}, f.err
}

func (*auditTestDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query")
}

func (*auditTestDB) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	return auditTestRow{}
}

type auditTestRow struct{}

func (auditTestRow) Scan(...any) error {
	return errors.New("unexpected QueryRow")
}
