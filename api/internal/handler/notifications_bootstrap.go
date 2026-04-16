package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

// ensureDefaultNotifications gives a freshly-bootstrapped user sensible
// notification defaults: a verified email channel pointed at their
// Clerk signup address, plus a subscription per event that ships with
// DefaultOn=true. Idempotent — noop if the user already has a channel.
//
// Called from MeHandler.Bootstrap. All errors are logged but never
// propagate — a failed provision must not break Bootstrap, since the
// user can still configure notifications manually from /settings/notifications.
func ensureDefaultNotifications(ctx context.Context, queries *db.Queries, user db.User) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("notifications bootstrap: panic recovered", "user_id", user.ID, "panic", r)
		}
	}()

	if user.Email == "" {
		return // no email on record = nothing to point the channel at
	}

	existing, err := queries.ListNotificationChannelsByUser(ctx, user.ID)
	if err != nil {
		slog.Warn("notifications bootstrap: list channels failed", "user_id", user.ID, "error", err)
		return
	}
	if len(existing) > 0 {
		return // already provisioned (or user created their own channel)
	}

	cfg, _ := json.Marshal(map[string]string{"address": user.Email})
	channel, err := queries.CreateNotificationChannel(ctx, db.CreateNotificationChannelParams{
		UserID:      user.ID,
		WorkspaceID: pgtype.Text{}, // account-level
		Kind:        "email",
		Config:      cfg,
		Label:       pgtype.Text{}, // no label — default channel, UI shows address
		VerifiedAt:  pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		slog.Warn("notifications bootstrap: create channel failed", "user_id", user.ID, "error", err)
		return
	}

	for _, ev := range SupportedNotificationEvents {
		if !ev.DefaultOn {
			continue
		}
		if _, subErr := queries.CreateNotificationSubscription(ctx, db.CreateNotificationSubscriptionParams{
			UserID:      user.ID,
			WorkspaceID: pgtype.Text{}, // account-level
			EventType:   ev.Type,
			ChannelID:   channel.ID,
			Enabled:     true,
			Filter:      nil,
		}); subErr != nil {
			slog.Warn("notifications bootstrap: create subscription failed",
				"user_id", user.ID, "event", ev.Type, "error", subErr)
			// Keep going — each event is independent.
		}
	}

	slog.Info("notifications bootstrap: provisioned defaults", "user_id", user.ID, "channel_id", channel.ID)
}
