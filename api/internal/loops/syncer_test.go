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

func TestSyncerSendsPlanChangedTransactionalEmailWhenTemplateConfigured(t *testing.T) {
	client := &fakeLifecycleClient{}
	syncer := NewSyncer(client, Options{
		Enabled: func(context.Context, DashboardUser) bool { return true },
		TransactionalIDs: TransactionalIDs{
			PlanChanged: "tmpl_plan_changed",
		},
	})

	if err := syncer.SendLifecycleEvent(context.Background(), LifecycleEvent{
		UserID:         "user_123",
		Email:          "alex@example.com",
		Name:           "Alex Smith",
		WorkspaceID:    "ws_123",
		WorkspaceName:  "Alex Workspace",
		PlanID:         "basic",
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
	if client.events != 0 {
		t.Fatalf("events = %d, want 0", client.events)
	}
	if client.transactionals != 1 {
		t.Fatalf("transactionals = %d, want 1", client.transactionals)
	}
	if client.lastTransactional.TransactionalID != "tmpl_plan_changed" {
		t.Fatalf("transactional ID = %q, want tmpl_plan_changed", client.lastTransactional.TransactionalID)
	}
	assertProperty(t, client.lastTransactional.DataVariables, "old_plan_id", "free")
	assertProperty(t, client.lastTransactional.DataVariables, "new_plan_id", "basic")
	assertMissingProperty(t, client.lastTransactional.DataVariables, "first_name")
	assertMissingProperty(t, client.lastTransactional.DataVariables, "workspace_name")
	assertMissingProperty(t, client.lastTransactional.DataVariables, "billing_url")
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

func TestSyncerSendsAccountCanceledTransactionalEmailWithoutContactUpsert(t *testing.T) {
	client := &fakeLifecycleClient{}
	syncer := NewSyncer(client, Options{
		Enabled: func(context.Context, DashboardUser) bool { return true },
		TransactionalIDs: TransactionalIDs{
			AccountCanceled: "tmpl_account_canceled",
		},
	})

	if err := syncer.SendLifecycleEvent(context.Background(), LifecycleEvent{
		UserID:         "user_123",
		Email:          "alex@example.com",
		Name:           "Alex Smith",
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
	if client.events != 0 {
		t.Fatalf("events = %d, want 0", client.events)
	}
	if client.transactionals != 1 {
		t.Fatalf("transactionals = %d, want 1", client.transactionals)
	}
	if client.lastTransactional.TransactionalID != "tmpl_account_canceled" {
		t.Fatalf("transactional ID = %q, want tmpl_account_canceled", client.lastTransactional.TransactionalID)
	}
	if len(client.lastTransactional.DataVariables) != 0 {
		t.Fatalf("data variables = %#v, want none for account cancellation template", client.lastTransactional.DataVariables)
	}
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

func TestSyncerSendsPostFailedTransactionalEmailWhenTemplateConfigured(t *testing.T) {
	client := &fakeLifecycleClient{}
	syncer := NewSyncer(client, Options{
		Enabled: func(context.Context, DashboardUser) bool { return true },
		TransactionalIDs: TransactionalIDs{
			PostFailed: "tmpl_post_failed",
		},
	})

	if err := syncer.SendLifecycleEvent(context.Background(), LifecycleEvent{
		UserID:         "user_123",
		Email:          "alex@example.com",
		Name:           "Alex Smith",
		WorkspaceID:    "ws_123",
		WorkspaceName:  "Alex Workspace",
		EventName:      "post_failed",
		IdempotencyKey: "post_failed:job_123",
		Properties: map[string]any{
			"post_id":       "post_123",
			"platform":      "youtube",
			"error_code":    "quota_exceeded",
			"dashboard_url": "https://app.unipost.dev/projects/profile_123/logs?post_id=post_123",
			"retriable":     false,
			"attempts":      1,
		},
	}); err != nil {
		t.Fatalf("SendLifecycleEvent returned error: %v", err)
	}

	if client.events != 0 {
		t.Fatalf("events = %d, want 0", client.events)
	}
	if client.transactionals != 1 {
		t.Fatalf("transactionals = %d, want 1", client.transactionals)
	}
	if client.lastTransactional.TransactionalID != "tmpl_post_failed" {
		t.Fatalf("transactional ID = %q, want tmpl_post_failed", client.lastTransactional.TransactionalID)
	}
	assertProperty(t, client.lastTransactional.DataVariables, "platform", "youtube")
	assertProperty(t, client.lastTransactional.DataVariables, "error_code", "quota_exceeded")
	assertMissingProperty(t, client.lastTransactional.DataVariables, "dashboard_url")
	assertMissingProperty(t, client.lastTransactional.DataVariables, "retriable")
	assertMissingProperty(t, client.lastTransactional.DataVariables, "attempts")
}

type fakeLifecycleClient struct {
	contacts          int
	events            int
	transactionals    int
	lastContact       Contact
	lastEvent         Event
	lastTransactional TransactionalEmail
	contactErr        error
	eventErr          error
	transactionalErr  error
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

func (f *fakeLifecycleClient) SendTransactional(_ context.Context, email TransactionalEmail) error {
	f.transactionals++
	f.lastTransactional = email
	return f.transactionalErr
}

func assertProperty(t *testing.T, props map[string]any, key string, want any) {
	t.Helper()
	if got := props[key]; got != want {
		t.Fatalf("property %s = %#v, want %#v", key, got, want)
	}
}

func assertMissingProperty(t *testing.T, props map[string]any, key string) {
	t.Helper()
	if _, ok := props[key]; ok {
		t.Fatalf("property %s is present, want missing", key)
	}
}
