package handler

import (
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
			EventType:   "billing.payment_failed",
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
