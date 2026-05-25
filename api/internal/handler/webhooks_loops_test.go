package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/loops"
)

func TestWebhookHandlerSyncLoopsDashboardUserBestEffort(t *testing.T) {
	syncer := &fakeLoopsSyncer{err: errors.New("provider unavailable")}
	h := NewWebhookHandler(nil, nil, "").SetLoopsSyncer(syncer)

	h.syncLoopsDashboardUser(context.Background(), "user.created", clerkUserData{
		ID:        "user_123",
		FirstName: "Alex",
		LastName:  "Smith",
	}, "alex@example.com", "Alex Smith", "ws_123", "Alex Workspace")

	if syncer.calls != 1 {
		t.Fatalf("calls = %d, want 1", syncer.calls)
	}
	got := syncer.lastUser
	if got.ID != "user_123" {
		t.Fatalf("user id = %q", got.ID)
	}
	if got.Email != "alex@example.com" {
		t.Fatalf("email = %q", got.Email)
	}
	if got.FirstName != "Alex" || got.LastName != "Smith" {
		t.Fatalf("name = %q %q", got.FirstName, got.LastName)
	}
	if got.WorkspaceID != "ws_123" || got.WorkspaceName != "Alex Workspace" {
		t.Fatalf("workspace = %q/%q", got.WorkspaceID, got.WorkspaceName)
	}
	if got.Event != "user.created" {
		t.Fatalf("event = %q", got.Event)
	}
}

type fakeLoopsSyncer struct {
	calls    int
	lastUser loops.DashboardUser
	err      error
}

func (f *fakeLoopsSyncer) SyncDashboardUser(_ context.Context, user loops.DashboardUser) error {
	f.calls++
	f.lastUser = user
	return f.err
}
