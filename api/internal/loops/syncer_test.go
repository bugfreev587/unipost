package loops

import (
	"context"
	"errors"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/emailpolicy"
	"github.com/xiaoboyu/unipost-api/internal/emailregistry"
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
	assertProperty(t, client.lastTransactional.DataVariables, "workspace_name", "Alex Workspace")
	assertProperty(t, client.lastTransactional.DataVariables, "old_plan_id", "free")
	assertProperty(t, client.lastTransactional.DataVariables, "new_plan_id", "basic")
	assertProperty(t, client.lastTransactional.DataVariables, "change_type", "upgrade")
	assertProperty(t, client.lastTransactional.DataVariables, "billing_url", "https://app.unipost.dev/settings/billing")
	assertMissingProperty(t, client.lastTransactional.DataVariables, "first_name")
}

func TestSyncerSendsBillingPaymentFailedTransactionalEmailWhenTemplateConfigured(t *testing.T) {
	client := &fakeLifecycleClient{}
	syncer := NewSyncer(client, Options{
		Enabled: func(context.Context, DashboardUser) bool { return true },
		TransactionalIDs: TransactionalIDs{
			BillingPaymentFailed: "tmpl_payment_failed",
		},
	})

	if err := syncer.SendLifecycleEvent(context.Background(), LifecycleEvent{
		UserID:         "user_123",
		Email:          "alex@example.com",
		Name:           "Alex Smith",
		WorkspaceID:    "ws_123",
		WorkspaceName:  "Alex Workspace",
		PlanID:         "growth",
		EventName:      "billing_payment_failed",
		IdempotencyKey: "billing_payment_failed:in_123:2",
		Properties: map[string]any{
			"workspace_name":       "Alex Workspace",
			"plan_id":              "growth",
			"billing_url":          "https://app.unipost.dev/settings/billing",
			"retry_message":        "Stripe will retry this payment automatically.",
			"attempt_count":        int64(2),
			"next_payment_attempt": "2026-07-01T12:00:00Z",
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
	if client.lastTransactional.TransactionalID != "tmpl_payment_failed" {
		t.Fatalf("transactional ID = %q, want tmpl_payment_failed", client.lastTransactional.TransactionalID)
	}
	assertProperty(t, client.lastTransactional.DataVariables, "workspace_name", "Alex Workspace")
	assertProperty(t, client.lastTransactional.DataVariables, "plan_id", "growth")
	assertProperty(t, client.lastTransactional.DataVariables, "billing_url", "https://app.unipost.dev/settings/billing")
	assertProperty(t, client.lastTransactional.DataVariables, "retry_message", "Stripe will retry this payment automatically.")
	assertProperty(t, client.lastTransactional.DataVariables, "attempt_count", float64(2))
	assertProperty(t, client.lastTransactional.DataVariables, "next_payment_attempt", "2026-07-01T12:00:00Z")
}

func TestSyncerSendsBillingPaymentRecoveredTransactionalEmailWhenTemplateConfigured(t *testing.T) {
	client := &fakeLifecycleClient{}
	syncer := NewSyncer(client, Options{
		Enabled: func(context.Context, DashboardUser) bool { return true },
		TransactionalIDs: TransactionalIDs{
			BillingPaymentRecovered: "tmpl_payment_recovered",
		},
	})

	if err := syncer.SendLifecycleEvent(context.Background(), LifecycleEvent{
		UserID:         "user_123",
		Email:          "alex@example.com",
		WorkspaceID:    "ws_123",
		WorkspaceName:  "Alex Workspace",
		PlanID:         "growth",
		EventName:      "billing_payment_recovered",
		IdempotencyKey: "billing_payment_recovered:in_123",
		Properties: map[string]any{
			"workspace_name": "Alex Workspace",
			"plan_id":        "growth",
			"billing_url":    "https://app.unipost.dev/settings/billing",
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
	if client.lastTransactional.TransactionalID != "tmpl_payment_recovered" {
		t.Fatalf("transactional ID = %q, want tmpl_payment_recovered", client.lastTransactional.TransactionalID)
	}
	assertProperty(t, client.lastTransactional.DataVariables, "workspace_name", "Alex Workspace")
	assertProperty(t, client.lastTransactional.DataVariables, "plan_id", "growth")
	assertProperty(t, client.lastTransactional.DataVariables, "billing_url", "https://app.unipost.dev/settings/billing")
}

func TestSyncerSendsBillingSubscriptionCanceledTransactionalEmailWhenTemplateConfigured(t *testing.T) {
	client := &fakeLifecycleClient{}
	syncer := NewSyncer(client, Options{
		Enabled: func(context.Context, DashboardUser) bool { return true },
		TransactionalIDs: TransactionalIDs{
			BillingSubscriptionCanceled: "tmpl_subscription_canceled",
		},
	})

	if err := syncer.SendLifecycleEvent(context.Background(), LifecycleEvent{
		UserID:         "user_123",
		Email:          "alex@example.com",
		WorkspaceID:    "ws_123",
		WorkspaceName:  "Alex Workspace",
		PlanID:         "growth",
		EventName:      "billing_subscription_canceled",
		IdempotencyKey: "billing_subscription_canceled:sub_123:2026-07-01T12:00:00Z",
		Properties: map[string]any{
			"workspace_name": "Alex Workspace",
			"plan_id":        "growth",
			"effective_at":   "2026-07-01T12:00:00Z",
			"billing_url":    "https://app.unipost.dev/settings/billing",
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
	if client.lastTransactional.TransactionalID != "tmpl_subscription_canceled" {
		t.Fatalf("transactional ID = %q, want tmpl_subscription_canceled", client.lastTransactional.TransactionalID)
	}
	assertProperty(t, client.lastTransactional.DataVariables, "workspace_name", "Alex Workspace")
	assertProperty(t, client.lastTransactional.DataVariables, "plan_id", "growth")
	assertProperty(t, client.lastTransactional.DataVariables, "effective_at", "2026-07-01T12:00:00Z")
	assertProperty(t, client.lastTransactional.DataVariables, "billing_url", "https://app.unipost.dev/settings/billing")
}

func TestSyncerSendsAccountDisconnectedTransactionalEmailWhenTemplateConfigured(t *testing.T) {
	client := &fakeLifecycleClient{}
	syncer := NewSyncer(client, Options{
		Enabled: func(context.Context, DashboardUser) bool { return true },
		TransactionalIDs: TransactionalIDs{
			AccountDisconnected: "tmpl_account_disconnected",
		},
	})

	if err := syncer.SendLifecycleEvent(context.Background(), LifecycleEvent{
		UserID:         "user_123",
		Email:          "alex@example.com",
		WorkspaceID:    "ws_123",
		WorkspaceName:  "Alex Workspace",
		EventName:      "account_disconnected",
		IdempotencyKey: "account_disconnected:acct_123:token_refresh_failed",
		Properties: map[string]any{
			"workspace_name": "Alex Workspace",
			"platform":       "instagram",
			"account_name":   "Alex Studio",
			"reconnect_url":  "https://app.unipost.dev/projects/ws_123/accounts",
			"reason":         "token_refresh_failed",
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
	if client.lastTransactional.TransactionalID != "tmpl_account_disconnected" {
		t.Fatalf("transactional ID = %q, want tmpl_account_disconnected", client.lastTransactional.TransactionalID)
	}
	assertProperty(t, client.lastTransactional.DataVariables, "workspace_name", "Alex Workspace")
	assertProperty(t, client.lastTransactional.DataVariables, "platform", "instagram")
	assertProperty(t, client.lastTransactional.DataVariables, "account_name", "Alex Studio")
	assertProperty(t, client.lastTransactional.DataVariables, "reconnect_url", "https://app.unipost.dev/projects/ws_123/accounts")
	assertProperty(t, client.lastTransactional.DataVariables, "reason", "token_refresh_failed")
}

func TestSyncerSendsLifecycleEventWhenNoTransactionalTemplateConfigured(t *testing.T) {
	client := &fakeLifecycleClient{}
	syncer := NewSyncer(client, Options{
		Enabled: func(context.Context, DashboardUser) bool { return true },
	})

	if err := syncer.SendLifecycleEvent(context.Background(), LifecycleEvent{
		UserID:         "user_123",
		Email:          "alex@example.com",
		WorkspaceID:    "ws_123",
		WorkspaceName:  "Alex Workspace",
		EventName:      "first_account_connected",
		IdempotencyKey: "first_account_connected:ws_123",
		Properties: map[string]any{
			"activation_state":         "has_account",
			"connected_accounts_count": int32(1),
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
	if client.transactionals != 0 {
		t.Fatalf("transactionals = %d, want 0", client.transactionals)
	}
	if client.lastEvent.Name != "first_account_connected" {
		t.Fatalf("event name = %q, want first_account_connected", client.lastEvent.Name)
	}
	assertProperty(t, client.lastEvent.Properties, "activation_state", "has_account")
	assertProperty(t, client.lastEvent.Properties, "connected_accounts_count", int32(1))
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
	assertProperty(t, client.lastTransactional.DataVariables, "canceled_at", "2026-05-25T12:00:00Z")
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
	assertProperty(t, client.lastTransactional.DataVariables, "workspace_name", "Alex Workspace")
	assertProperty(t, client.lastTransactional.DataVariables, "post_id", "post_123")
	assertProperty(t, client.lastTransactional.DataVariables, "platform", "youtube")
	assertProperty(t, client.lastTransactional.DataVariables, "error_code", "quota_exceeded")
	assertProperty(t, client.lastTransactional.DataVariables, "dashboard_url", "https://app.unipost.dev/projects/profile_123/logs?post_id=post_123")
	assertProperty(t, client.lastTransactional.DataVariables, "retriable", "false")
	assertMissingProperty(t, client.lastTransactional.DataVariables, "attempts")
}

func TestSyncerAppliesEmailPolicyFooterVariablesBeforeTransactionalSend(t *testing.T) {
	client := &fakeLifecycleClient{}
	policy := &fakeEmailPolicy{
		decision: emailpolicy.Decision{
			ShouldSend: true,
			DataVariables: map[string]any{
				"workspace_name":            "Alex Workspace",
				"post_id":                   "post_123",
				"platform":                  "youtube",
				"error_code":                "quota_exceeded",
				"dashboard_url":             "https://app.unipost.dev/projects/profile_123/logs?post_id=post_123",
				"retriable":                 "false",
				"footer_policy":             string(emailregistry.FooterManagePreferences),
				"manage_preferences_url":    "https://app.unipost.dev/settings/notifications",
				"preference_category_label": "Publishing failure alerts",
			},
		},
	}
	syncer := NewSyncer(client, Options{
		Enabled: func(context.Context, DashboardUser) bool { return true },
		TransactionalIDs: TransactionalIDs{
			PostFailed: "tmpl_post_failed",
		},
		EmailPolicy: policy,
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
			"retriable":     false,
		},
	}); err != nil {
		t.Fatalf("SendLifecycleEvent returned error: %v", err)
	}

	if policy.calls != 1 {
		t.Fatalf("policy calls = %d, want 1", policy.calls)
	}
	if policy.lastRequest.EventKey != "email.post.failed.v1" {
		t.Fatalf("policy event key = %q, want email.post.failed.v1", policy.lastRequest.EventKey)
	}
	if client.transactionals != 1 {
		t.Fatalf("transactionals = %d, want 1", client.transactionals)
	}
	assertProperty(t, client.lastTransactional.DataVariables, "footer_policy", string(emailregistry.FooterManagePreferences))
	assertProperty(t, client.lastTransactional.DataVariables, "manage_preferences_url", "https://app.unipost.dev/settings/notifications")
	assertProperty(t, client.lastTransactional.DataVariables, "preference_category_label", "Publishing failure alerts")
}

func TestSyncerSkipsPreferenceDisabledTransactionalEmailAndAuditsSkipped(t *testing.T) {
	client := &fakeLifecycleClient{}
	audit := &fakeEmailAuditStore{}
	policy := &fakeEmailPolicy{
		decision: emailpolicy.Decision{
			ShouldSend: false,
			SkipReason: emailpolicy.SkipReasonPreferenceDisabled,
			DataVariables: map[string]any{
				"workspace_name":            "Alex Workspace",
				"preference_category_label": "Publishing failure alerts",
			},
		},
	}
	syncer := NewSyncer(client, Options{
		Enabled: func(context.Context, DashboardUser) bool { return true },
		TransactionalIDs: TransactionalIDs{
			PostFailed: "tmpl_post_failed",
		},
		EmailAuditStore: audit,
		EmailPolicy:     policy,
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
			"retriable":     false,
		},
	}); err != nil {
		t.Fatalf("SendLifecycleEvent returned error: %v", err)
	}

	if client.transactionals != 0 {
		t.Fatalf("transactionals = %d, want 0", client.transactionals)
	}
	if client.events != 0 {
		t.Fatalf("events = %d, want 0", client.events)
	}
	if audit.skipped != 1 {
		t.Fatalf("skipped audit rows = %d, want 1", audit.skipped)
	}
	if audit.skippedReason != emailpolicy.SkipReasonPreferenceDisabled {
		t.Fatalf("skipped reason = %q, want %q", audit.skippedReason, emailpolicy.SkipReasonPreferenceDisabled)
	}
	if audit.lastAttempt.EventKey != "email.post.failed.v1" {
		t.Fatalf("audit event key = %q, want email.post.failed.v1", audit.lastAttempt.EventKey)
	}
	if audit.lastAttempt.ProviderTemplateID != "tmpl_post_failed" {
		t.Fatalf("audit template = %q, want tmpl_post_failed", audit.lastAttempt.ProviderTemplateID)
	}
	assertProperty(t, audit.lastAttempt.DataVariables, "preference_category_label", "Publishing failure alerts")
}

func TestSyncerAuditsLifecycleTransactionalEmailSuccess(t *testing.T) {
	client := &fakeLifecycleClient{}
	audit := &fakeEmailAuditStore{}
	syncer := NewSyncer(client, Options{
		Enabled: func(context.Context, DashboardUser) bool { return true },
		TransactionalIDs: TransactionalIDs{
			PostFailed: "tmpl_post_failed",
		},
		EmailAuditStore: audit,
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
			"retriable":     false,
		},
	}); err != nil {
		t.Fatalf("SendLifecycleEvent returned error: %v", err)
	}

	if audit.created != 1 {
		t.Fatalf("audit created = %d, want 1", audit.created)
	}
	if audit.markedSent != 1 {
		t.Fatalf("audit markedSent = %d, want 1", audit.markedSent)
	}
	if audit.markedFailed != 0 {
		t.Fatalf("audit markedFailed = %d, want 0", audit.markedFailed)
	}
	if audit.lastAttempt.EventKey != "email.post.failed.v1" {
		t.Fatalf("event key = %q, want email.post.failed.v1", audit.lastAttempt.EventKey)
	}
	if audit.lastAttempt.Provider != "loops" {
		t.Fatalf("provider = %q, want loops", audit.lastAttempt.Provider)
	}
	if audit.lastAttempt.ProviderTemplateID != "tmpl_post_failed" {
		t.Fatalf("provider template = %q, want tmpl_post_failed", audit.lastAttempt.ProviderTemplateID)
	}
	if audit.lastAttempt.IdempotencyKey != "post_failed:job_123" {
		t.Fatalf("idempotency key = %q, want post_failed:job_123", audit.lastAttempt.IdempotencyKey)
	}
	if audit.lastAttempt.RecipientEmail != "alex@example.com" || audit.lastAttempt.RecipientUserID != "user_123" {
		t.Fatalf("recipient = %q/%q, want alex@example.com/user_123", audit.lastAttempt.RecipientEmail, audit.lastAttempt.RecipientUserID)
	}
	if audit.lastAttempt.WorkspaceID != "ws_123" {
		t.Fatalf("workspace = %q, want ws_123", audit.lastAttempt.WorkspaceID)
	}
	assertProperty(t, audit.lastAttempt.DataVariables, "post_id", "post_123")
	assertProperty(t, audit.lastAttempt.DataVariables, "retriable", "false")
}

func TestSyncerAuditsLifecycleTransactionalEmailFailure(t *testing.T) {
	client := &fakeLifecycleClient{transactionalErr: errors.New("loops down")}
	audit := &fakeEmailAuditStore{}
	syncer := NewSyncer(client, Options{
		Enabled: func(context.Context, DashboardUser) bool { return true },
		TransactionalIDs: TransactionalIDs{
			BillingPaymentFailed: "tmpl_payment_failed",
		},
		EmailAuditStore: audit,
	})

	if err := syncer.SendLifecycleEvent(context.Background(), LifecycleEvent{
		UserID:         "user_123",
		Email:          "alex@example.com",
		WorkspaceID:    "ws_123",
		WorkspaceName:  "Alex Workspace",
		PlanID:         "growth",
		EventName:      "billing_payment_failed",
		IdempotencyKey: "billing_payment_failed:in_123:2",
		Properties: map[string]any{
			"workspace_name":       "Alex Workspace",
			"plan_id":              "growth",
			"billing_url":          "https://app.unipost.dev/settings/billing",
			"retry_message":        "Stripe will retry this payment automatically.",
			"attempt_count":        int64(2),
			"next_payment_attempt": "2026-07-01T12:00:00Z",
		},
	}); err != nil {
		t.Fatalf("SendLifecycleEvent returned error: %v", err)
	}

	if audit.created != 1 {
		t.Fatalf("audit created = %d, want 1", audit.created)
	}
	if audit.markedSent != 0 {
		t.Fatalf("audit markedSent = %d, want 0", audit.markedSent)
	}
	if audit.markedFailed != 1 {
		t.Fatalf("audit markedFailed = %d, want 1", audit.markedFailed)
	}
	if audit.failedReason != "loops down" {
		t.Fatalf("failure reason = %q, want loops down", audit.failedReason)
	}
	if audit.lastAttempt.EventKey != "email.billing.payment_failed.v1" {
		t.Fatalf("event key = %q, want email.billing.payment_failed.v1", audit.lastAttempt.EventKey)
	}
	assertProperty(t, audit.lastAttempt.DataVariables, "attempt_count", float64(2))
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

type fakeEmailAuditStore struct {
	created       int
	skipped       int
	markedSent    int
	markedFailed  int
	lastAttempt   EmailSendAttempt
	failedReason  string
	skippedReason string
}

func (f *fakeEmailAuditStore) CreateEmailSendAttempt(_ context.Context, attempt EmailSendAttempt) (EmailSendAttemptRecord, error) {
	f.created++
	f.lastAttempt = attempt
	return EmailSendAttemptRecord{ID: "audit_123"}, nil
}

func (f *fakeEmailAuditStore) CreateSkippedEmailSendAttempt(_ context.Context, attempt EmailSendAttempt, reason string) (EmailSendAttemptRecord, error) {
	f.skipped++
	f.lastAttempt = attempt
	f.skippedReason = reason
	return EmailSendAttemptRecord{ID: "audit_123"}, nil
}

func (f *fakeEmailAuditStore) MarkEmailSendAttemptSent(_ context.Context, id string) error {
	if id != "audit_123" {
		return errors.New("unexpected audit id")
	}
	f.markedSent++
	return nil
}

func (f *fakeEmailAuditStore) MarkEmailSendAttemptFailed(_ context.Context, id, reason string) error {
	if id != "audit_123" {
		return errors.New("unexpected audit id")
	}
	f.markedFailed++
	f.failedReason = reason
	return nil
}

type fakeEmailPolicy struct {
	decision    emailpolicy.Decision
	err         error
	calls       int
	lastRequest emailpolicy.Request
}

func (f *fakeEmailPolicy) Prepare(_ context.Context, request emailpolicy.Request) (emailpolicy.Decision, error) {
	f.calls++
	f.lastRequest = request
	if f.err != nil {
		return emailpolicy.Decision{}, f.err
	}
	return f.decision, nil
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
