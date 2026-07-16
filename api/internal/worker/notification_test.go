package worker

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
)

func TestNotificationDeliveryPlanSkipsLoopsOwnedEmailEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		event       string
		channelKind string
		wantSkipped bool
	}{
		{name: "post failed email moves to loops", event: events.EventPostFailed, channelKind: "email", wantSkipped: true},
		{name: "account disconnected email moves to loops", event: events.EventAccountDisconnected, channelKind: "email", wantSkipped: true},
		{name: "post failed slack stays in notifications", event: events.EventPostFailed, channelKind: "slack_webhook", wantSkipped: false},
		{name: "account disconnected discord stays in notifications", event: events.EventAccountDisconnected, channelKind: "discord_webhook", wantSkipped: false},
		{name: "usage email is not claimed by this migration", event: events.EventBillingUsage80pct, channelKind: "email", wantSkipped: false},
		{name: "X inbound 80 percent email stays in notifications", event: events.EventBillingXInbound80pct, channelKind: "email", wantSkipped: false},
		{name: "X inbound cap reached slack stays in notifications", event: events.EventBillingXInboundCapReached, channelKind: "slack_webhook", wantSkipped: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			plan := notificationDeliveryPlan(tc.event, tc.channelKind)
			if plan.Skipped != tc.wantSkipped {
				t.Fatalf("Skipped = %v, want %v", plan.Skipped, tc.wantSkipped)
			}
			if tc.wantSkipped && plan.Reason == "" {
				t.Fatal("skipped plan should include audit reason")
			}
		})
	}
}

func TestXInboundNotificationRenderingIncludesCountsResetAndManagementLinkOnly(t *testing.T) {
	payload := []byte(`{
		"inbound_daily_usage": 320,
		"inbound_daily_limit": 400,
		"reset_at": "2026-07-17T00:00:00Z",
		"cap_management_url": "https://dev-app.unipost.dev/settings/billing#x-inbound-cap",
		"body": "private DM text must not render"
	}`)

	webhook := renderWebhookMessage(events.EventBillingXInbound80pct, payload, "https://dev-app.unipost.dev")
	email := renderEmail(events.EventBillingXInboundCapReached, payload, "https://dev-app.unipost.dev")
	for label, rendered := range map[string]string{
		"webhook": webhook,
		"email":   email.Text,
	} {
		for _, want := range []string{
			"320 / 400",
			"2026-07-17T00:00:00Z",
			"https://dev-app.unipost.dev/settings/billing#x-inbound-cap",
		} {
			if !strings.Contains(rendered, want) {
				t.Fatalf("%s missing %q: %s", label, want, rendered)
			}
		}
		if strings.Contains(rendered, "private DM text") {
			t.Fatalf("%s rendered private body: %s", label, rendered)
		}
	}
}

func TestBuildLoopsAccountDisconnectedEvent(t *testing.T) {
	t.Parallel()

	event := buildLoopsAccountDisconnectedEvent(
		db.User{ID: "user_123", Email: "alex@example.com", Name: pgtype.Text{String: "Alex Smith", Valid: true}},
		db.Workspace{ID: "ws_123", Name: "Alex Workspace"},
		map[string]any{
			"social_account_id": "acct_123",
			"profile_id":        "profile_123",
			"platform":          "instagram",
			"account_name":      "Alex Studio",
			"reason":            "token_refresh_failed",
		},
		"https://app.unipost.dev",
	)

	if event.EventName != "account_disconnected" {
		t.Fatalf("event name = %q, want account_disconnected", event.EventName)
	}
	if event.IdempotencyKey != "account_disconnected:acct_123:token_refresh_failed" {
		t.Fatalf("idempotency key = %q", event.IdempotencyKey)
	}
	assertLifecycleProperty(t, event.Properties, "workspace_name", "Alex Workspace")
	assertLifecycleProperty(t, event.Properties, "platform", "instagram")
	assertLifecycleProperty(t, event.Properties, "account_name", "Alex Studio")
	assertLifecycleProperty(t, event.Properties, "reason", "token_refresh_failed")
	assertLifecycleProperty(t, event.Properties, "reconnect_url", "https://app.unipost.dev/projects/profile_123/accounts")
}

func TestBuildLoopsFirstAccountConnectedEvent(t *testing.T) {
	t.Parallel()

	event := buildLoopsFirstAccountConnectedEvent(
		db.User{ID: "user_123", Email: "alex@example.com", Name: pgtype.Text{String: "Alex Smith", Valid: true}},
		db.Workspace{ID: "ws_123", Name: "Alex Workspace"},
		map[string]any{
			"social_account_id": "acct_123",
			"profile_id":        "profile_123",
			"platform":          "instagram",
			"account_name":      "Alex Studio",
		},
		"https://app.unipost.dev",
	)

	if event.EventName != "first_account_connected" {
		t.Fatalf("event name = %q, want first_account_connected", event.EventName)
	}
	if event.IdempotencyKey != "first_account_connected:ws_123" {
		t.Fatalf("idempotency key = %q", event.IdempotencyKey)
	}
	assertLifecycleProperty(t, event.Properties, "workspace_name", "Alex Workspace")
	assertLifecycleProperty(t, event.Properties, "platform", "instagram")
	assertLifecycleProperty(t, event.Properties, "account_name", "Alex Studio")
	assertLifecycleProperty(t, event.Properties, "activation_state", "has_account")
	assertLifecycleProperty(t, event.Properties, "connected_accounts_count", int32(1))
	assertLifecycleProperty(t, event.Properties, "dashboard_url", "https://app.unipost.dev/projects/profile_123/accounts")
}

func TestBuildLoopsFirstPostPublishedEvent(t *testing.T) {
	t.Parallel()

	event := buildLoopsFirstPostPublishedEvent(
		db.User{ID: "user_123", Email: "alex@example.com"},
		db.Workspace{ID: "ws_123", Name: "Alex Workspace"},
		map[string]any{
			"id":          "post_123",
			"profile_ids": []any{"profile_123"},
			"status":      "published",
		},
		"https://app.unipost.dev",
	)

	if event.EventName != "first_post_published" {
		t.Fatalf("event name = %q, want first_post_published", event.EventName)
	}
	if event.IdempotencyKey != "first_post_published:ws_123" {
		t.Fatalf("idempotency key = %q", event.IdempotencyKey)
	}
	assertLifecycleProperty(t, event.Properties, "workspace_name", "Alex Workspace")
	assertLifecycleProperty(t, event.Properties, "post_id", "post_123")
	assertLifecycleProperty(t, event.Properties, "activation_state", "activated")
	assertLifecycleProperty(t, event.Properties, "published_posts_count", int32(1))
	assertLifecycleProperty(t, event.Properties, "dashboard_url", "https://app.unipost.dev/projects/profile_123/logs?post_id=post_123")
}

func assertLifecycleProperty(t *testing.T, props map[string]any, key string, want any) {
	t.Helper()
	got, ok := props[key]
	if !ok {
		t.Fatalf("missing property %q", key)
	}
	if got != want {
		t.Fatalf("property %q = %#v, want %#v", key, got, want)
	}
}
