package worker

import (
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
