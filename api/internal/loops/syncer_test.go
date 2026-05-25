package loops

import (
	"context"
	"errors"
	"testing"
)

func TestSyncerSkipsWhenFlagDisabled(t *testing.T) {
	client := &fakeLifecycleClient{}
	syncer := NewSyncer(client, Options{
		Enabled: func(context.Context, DashboardUser) bool { return false },
	})

	if err := syncer.SyncDashboardUser(context.Background(), DashboardUser{
		ID:    "user_123",
		Email: "alex@example.com",
		Event: "user.created",
	}); err != nil {
		t.Fatalf("SyncDashboardUser returned error: %v", err)
	}

	if client.contacts != 0 || client.events != 0 {
		t.Fatalf("expected no Loops calls, got contacts=%d events=%d", client.contacts, client.events)
	}
}

func TestSyncerUpsertsContactAndSendsSignupEvent(t *testing.T) {
	client := &fakeLifecycleClient{}
	syncer := NewSyncer(client, Options{
		Enabled: func(context.Context, DashboardUser) bool { return true },
	})

	if err := syncer.SyncDashboardUser(context.Background(), DashboardUser{
		ID:            "user_123",
		Email:         "alex@example.com",
		FirstName:     "Alex",
		LastName:      "Smith",
		Name:          "Alex Smith",
		WorkspaceID:   "ws_123",
		WorkspaceName: "Alex Workspace",
		Event:         "user.created",
	}); err != nil {
		t.Fatalf("SyncDashboardUser returned error: %v", err)
	}

	if client.contacts != 1 {
		t.Fatalf("contacts = %d, want 1", client.contacts)
	}
	if client.lastContact.Email != "alex@example.com" {
		t.Fatalf("contact email = %q", client.lastContact.Email)
	}
	if client.lastContact.FirstName != "Alex" || client.lastContact.LastName != "Smith" {
		t.Fatalf("contact name = %q %q", client.lastContact.FirstName, client.lastContact.LastName)
	}
	if client.lastContact.Source != "unipost_dashboard" {
		t.Fatalf("source = %q", client.lastContact.Source)
	}
	assertProperty(t, client.lastContact.Properties, "workspace_id", "ws_123")
	assertProperty(t, client.lastContact.Properties, "workspace_name", "Alex Workspace")
	assertProperty(t, client.lastContact.Properties, "source", "unipost_dashboard")

	if client.events != 1 {
		t.Fatalf("events = %d, want 1", client.events)
	}
	if client.lastEvent.Name != "user_signed_up" {
		t.Fatalf("event name = %q", client.lastEvent.Name)
	}
	if client.lastEvent.IdempotencyKey != "clerk_user.created:user_123" {
		t.Fatalf("idempotency key = %q", client.lastEvent.IdempotencyKey)
	}
	assertProperty(t, client.lastEvent.Properties, "workspace_id", "ws_123")
}

func TestSyncerDoesNotSendSignupEventForUserUpdated(t *testing.T) {
	client := &fakeLifecycleClient{}
	syncer := NewSyncer(client, Options{
		Enabled: func(context.Context, DashboardUser) bool { return true },
	})

	if err := syncer.SyncDashboardUser(context.Background(), DashboardUser{
		ID:    "user_123",
		Email: "alex@example.com",
		Event: "user.updated",
	}); err != nil {
		t.Fatalf("SyncDashboardUser returned error: %v", err)
	}

	if client.contacts != 1 {
		t.Fatalf("contacts = %d, want 1", client.contacts)
	}
	if client.events != 0 {
		t.Fatalf("events = %d, want 0", client.events)
	}
}

func TestSyncerSwallowsProviderErrors(t *testing.T) {
	client := &fakeLifecycleClient{contactErr: errors.New("provider down")}
	syncer := NewSyncer(client, Options{
		Enabled: func(context.Context, DashboardUser) bool { return true },
	})

	if err := syncer.SyncDashboardUser(context.Background(), DashboardUser{
		ID:    "user_123",
		Email: "alex@example.com",
		Event: "user.created",
	}); err != nil {
		t.Fatalf("SyncDashboardUser should not block callers: %v", err)
	}

	if client.contacts != 1 {
		t.Fatalf("contacts = %d, want 1", client.contacts)
	}
	if client.events != 0 {
		t.Fatalf("events = %d, want 0 when contact upsert fails", client.events)
	}
}

type fakeLifecycleClient struct {
	contacts    int
	events      int
	lastContact Contact
	lastEvent   Event
	contactErr  error
	eventErr    error
}

func (f *fakeLifecycleClient) Enabled() bool {
	return true
}

func (f *fakeLifecycleClient) UpsertContact(_ context.Context, contact Contact) error {
	f.contacts++
	f.lastContact = contact
	return f.contactErr
}

func (f *fakeLifecycleClient) SendEvent(_ context.Context, event Event) error {
	f.events++
	f.lastEvent = event
	return f.eventErr
}

func assertProperty(t *testing.T, props map[string]any, key string, want any) {
	t.Helper()
	if got := props[key]; got != want {
		t.Fatalf("property %s = %#v, want %#v", key, got, want)
	}
}
