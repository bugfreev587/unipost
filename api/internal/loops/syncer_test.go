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

func TestSyncerSendsPlanChangedEvent(t *testing.T) {
	client := &fakeLifecycleClient{}
	syncer := NewSyncer(client, Options{
		Enabled: func(context.Context, DashboardUser) bool { return true },
	})

	if err := syncer.SendLifecycleEvent(context.Background(), LifecycleEvent{
		UserID:         "user_123",
		Email:          "alex@example.com",
		Name:           "Alex Smith",
		WorkspaceID:    "ws_123",
		WorkspaceName:  "Alex Workspace",
		EventName:      "plan_changed",
		IdempotencyKey: "plan_changed:sub_123:basic",
		Properties: map[string]any{
			"old_plan_id": "free",
			"new_plan_id": "basic",
			"change_type": "upgrade",
			"billing_url": "https://app.unipost.dev/settings/billing",
		},
	}); err != nil {
		t.Fatalf("SendLifecycleEvent returned error: %v", err)
	}

	if client.contacts != 1 {
		t.Fatalf("contacts = %d, want 1", client.contacts)
	}
	if client.events != 1 {
		t.Fatalf("events = %d, want 1", client.events)
	}
	if client.lastEvent.Name != "plan_changed" {
		t.Fatalf("event name = %q", client.lastEvent.Name)
	}
	if client.lastEvent.IdempotencyKey != "plan_changed:sub_123:basic" {
		t.Fatalf("idempotency key = %q", client.lastEvent.IdempotencyKey)
	}
	assertProperty(t, client.lastEvent.Properties, "workspace_id", "ws_123")
	assertProperty(t, client.lastEvent.Properties, "workspace_name", "Alex Workspace")
	assertProperty(t, client.lastEvent.Properties, "old_plan_id", "free")
	assertProperty(t, client.lastEvent.Properties, "new_plan_id", "basic")
	assertProperty(t, client.lastEvent.Properties, "change_type", "upgrade")
}

func TestSyncerSendsAccountCanceledEventWithoutContactUpsert(t *testing.T) {
	client := &fakeLifecycleClient{}
	syncer := NewSyncer(client, Options{
		Enabled: func(context.Context, DashboardUser) bool { return true },
	})

	if err := syncer.SendLifecycleEvent(context.Background(), LifecycleEvent{
		UserID:         "user_123",
		Email:          "alex@example.com",
		EventName:      "user_account_canceled",
		IdempotencyKey: "user_account_canceled:user_123",
		SkipContact:    true,
		Properties: map[string]any{
			"canceled_at": "2026-05-25T12:00:00Z",
		},
	}); err != nil {
		t.Fatalf("SendLifecycleEvent returned error: %v", err)
	}

	if client.contacts != 0 {
		t.Fatalf("contacts = %d, want 0", client.contacts)
	}
	if client.events != 1 {
		t.Fatalf("events = %d, want 1", client.events)
	}
	if client.lastEvent.Name != "user_account_canceled" {
		t.Fatalf("event name = %q", client.lastEvent.Name)
	}
	assertProperty(t, client.lastEvent.Properties, "canceled_at", "2026-05-25T12:00:00Z")
}

func TestSyncerSendsPostFailedEvent(t *testing.T) {
	client := &fakeLifecycleClient{}
	syncer := NewSyncer(client, Options{
		Enabled: func(context.Context, DashboardUser) bool { return true },
	})

	if err := syncer.SendLifecycleEvent(context.Background(), LifecycleEvent{
		UserID:         "user_123",
		Email:          "alex@example.com",
		WorkspaceID:    "ws_123",
		WorkspaceName:  "Alex Workspace",
		EventName:      "post_failed",
		IdempotencyKey: "post_failed:job_123",
		Properties: map[string]any{
			"post_id":       "post_123",
			"platform":      "youtube",
			"error_code":    "quota_exceeded",
			"dashboard_url": "https://app.unipost.dev/projects/profile_123/logs?post_id=post_123",
		},
	}); err != nil {
		t.Fatalf("SendLifecycleEvent returned error: %v", err)
	}

	if client.events != 1 {
		t.Fatalf("events = %d, want 1", client.events)
	}
	if client.lastEvent.Name != "post_failed" {
		t.Fatalf("event name = %q", client.lastEvent.Name)
	}
	assertProperty(t, client.lastEvent.Properties, "platform", "youtube")
	assertProperty(t, client.lastEvent.Properties, "error_code", "quota_exceeded")
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
