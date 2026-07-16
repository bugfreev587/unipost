package handler

import (
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestMatchingAccountLevelSubscriptions(t *testing.T) {
	rows := []db.ListNotificationSubscriptionsByUserRow{
		{
			ID:          "acct-1",
			EventType:   "post.failed",
			ChannelID:   "channel-1",
			WorkspaceID: pgtype.Text{},
		},
		{
			ID:          "workspace-1",
			EventType:   "post.failed",
			ChannelID:   "channel-1",
			WorkspaceID: pgtype.Text{String: "ws_123", Valid: true},
		},
		{
			ID:          "acct-2",
			EventType:   "post.failed",
			ChannelID:   "channel-1",
			WorkspaceID: pgtype.Text{},
		},
		{
			ID:          "other-event",
			EventType:   "account.disconnected",
			ChannelID:   "channel-1",
			WorkspaceID: pgtype.Text{},
		},
	}

	got := matchingAccountLevelSubscriptions(rows, "post.failed", "channel-1")
	if len(got) != 2 {
		t.Fatalf("expected 2 account-level matches, got %d", len(got))
	}
	if got[0].ID != "acct-1" || got[1].ID != "acct-2" {
		t.Fatalf("unexpected match order/ids: %+v", got)
	}
}

func TestSupportedNotificationEventsIncludesXInboundCapWarnings(t *testing.T) {
	for _, eventType := range []string{"billing.x_inbound_80pct", "billing.x_inbound_cap_reached"} {
		if !isSupportedEvent(eventType) {
			t.Fatalf("supported notification events missing %q", eventType)
		}
	}
}

func TestEmailPreferenceRoutesAreRegistered(t *testing.T) {
	source, err := os.ReadFile("../../cmd/api/main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	body := string(source)

	for _, want := range []string{
		`r.Get("/v1/me/notifications/email-preferences", notificationHandler.ListEmailPreferences)`,
		`r.Put("/v1/me/notifications/email-preferences/{category}", notificationHandler.UpdateEmailPreference)`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("notification email preference route missing %q", want)
		}
	}
}

func TestNotificationHandlerExposesEmailPreferenceContract(t *testing.T) {
	source, err := os.ReadFile("notifications.go")
	if err != nil {
		t.Fatalf("read notifications.go: %v", err)
	}
	body := string(source)

	for _, want := range []string{
		"type emailPreferenceResponse struct",
		"`json:\"category_key\"`",
		"`json:\"locked\"`",
		"`json:\"enabled\"`",
		"func (h *NotificationHandler) ListEmailPreferences",
		"func (h *NotificationHandler) UpdateEmailPreference",
		"emailregistry.EmailPreferenceCategories()",
		"UpsertEmailPreference",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("notification email preference contract missing %q", want)
		}
	}
}
