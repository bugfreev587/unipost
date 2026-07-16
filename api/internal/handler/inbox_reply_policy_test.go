package handler

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestMetaDMReplyWindowClosed(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		item db.InboxItem
		want bool
	}{
		{
			name: "instagram older than 24 hours",
			item: db.InboxItem{
				Source:     "ig_dm",
				ReceivedAt: pgtype.Timestamptz{Time: now.Add(-24*time.Hour - time.Second), Valid: true},
			},
			want: true,
		},
		{
			name: "facebook older than 24 hours",
			item: db.InboxItem{
				Source:     "fb_dm",
				ReceivedAt: pgtype.Timestamptz{Time: now.Add(-24*time.Hour - time.Second), Valid: true},
			},
			want: true,
		},
		{
			name: "instagram exactly 24 hours",
			item: db.InboxItem{
				Source:     "ig_dm",
				ReceivedAt: pgtype.Timestamptz{Time: now.Add(-24 * time.Hour), Valid: true},
			},
			want: false,
		},
		{
			name: "own message does not define reply window",
			item: db.InboxItem{
				Source:     "ig_dm",
				IsOwn:      true,
				ReceivedAt: pgtype.Timestamptz{Time: now.Add(-48 * time.Hour), Valid: true},
			},
			want: false,
		},
		{
			name: "comment is ignored",
			item: db.InboxItem{
				Source:     "ig_comment",
				ReceivedAt: pgtype.Timestamptz{Time: now.Add(-48 * time.Hour), Valid: true},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := metaDMReplyWindowClosed(tt.item, now); got != tt.want {
				t.Fatalf("metaDMReplyWindowClosed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInboxReplyPlatformErrorMapsInstagramWindowFailure(t *testing.T) {
	message, reconnect := inboxReplyPlatformError(
		"ig_dm",
		errors.New("instagram send dm 403: code 10 error_subcode 2534022"),
	)

	if reconnect {
		t.Fatal("reconnect = true, want false")
	}
	if !strings.Contains(message, "24-hour reply window") || !strings.Contains(message, "new message") {
		t.Fatalf("message = %q", message)
	}
}

func TestInboxReplyPlatformErrorKeepsRecipientReconnectMapping(t *testing.T) {
	message, reconnect := inboxReplyPlatformError(
		"ig_dm",
		errors.New("instagram send dm 400: error_subcode 2534014"),
	)

	if !reconnect {
		t.Fatal("reconnect = false, want true")
	}
	if !strings.Contains(message, "Reconnect") {
		t.Fatalf("message = %q", message)
	}
}
