package integrationlogs

import (
	"context"
	"errors"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type fakeIntegrationLogStore struct {
	params db.InsertIntegrationLogParams
	row    db.IntegrationLog
	err    error
}

func (f *fakeIntegrationLogStore) InsertIntegrationLog(_ context.Context, params db.InsertIntegrationLogParams) (db.IntegrationLog, error) {
	f.params = params
	return f.row, f.err
}

func TestLoggerWriteSyncPersistsBeforeReturning(t *testing.T) {
	store := &fakeIntegrationLogStore{row: db.IntegrationLog{ID: 42}}
	notified := false
	logger := NewLogger(store, func(_ context.Context, row db.IntegrationLog) {
		notified = row.ID == 42
	})

	err := logger.WriteSync(context.Background(), Event{
		WorkspaceID: "ws_1",
		Action:      ActionAccountConnectCallbackOK,
	})

	if err != nil || store.params.WorkspaceID != "ws_1" || !notified {
		t.Fatalf("err=%v params=%+v notified=%v", err, store.params, notified)
	}
}

func TestLoggerWriteSyncReturnsInsertFailureWithoutNotification(t *testing.T) {
	store := &fakeIntegrationLogStore{err: errors.New("insert failed")}
	notified := false
	logger := NewLogger(store, func(context.Context, db.IntegrationLog) {
		notified = true
	})

	err := logger.WriteSync(context.Background(), Event{
		WorkspaceID: "ws_1",
		Action:      ActionAccountConnectCallbackFailed,
	})

	if !errors.Is(err, store.err) {
		t.Fatalf("error = %v, want %v", err, store.err)
	}
	if notified {
		t.Fatal("failed insert must not notify afterWrite")
	}
}

func TestLoggerWriteSyncRejectsUnattributedEvent(t *testing.T) {
	logger := NewLogger(&fakeIntegrationLogStore{}, nil)
	if err := logger.WriteSync(context.Background(), Event{Action: ActionAccountConnectCallbackFailed}); err == nil {
		t.Fatal("missing workspace must return an error")
	}
}
