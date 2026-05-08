package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const inboxChannel = "inbox_events"
const logsChannel = "logs_events"

// PGListener subscribes to PostgreSQL LISTEN/NOTIFY and forwards
// inbox events to the WebSocket Hub. This ensures all API replicas
// push events to their connected clients, regardless of which
// replica processed the webhook.
type PGListener struct {
	inboxHub *Hub
	logsHub  *Hub
	pool     *pgxpool.Pool
}

func NewPGListener(inboxHub, logsHub *Hub, pool *pgxpool.Pool) *PGListener {
	return &PGListener{inboxHub: inboxHub, logsHub: logsHub, pool: pool}
}

func (l *PGListener) Start(ctx context.Context) {
	slog.Info("ws pglistener started")
	for {
		if err := l.listen(ctx); err != nil {
			if ctx.Err() != nil {
				slog.Info("ws pglistener stopped")
				return
			}
			slog.Warn("ws pglistener: connection lost, reconnecting in 3s", "err", err)
			time.Sleep(3 * time.Second)
		}
	}
}

func (l *PGListener) listen(ctx context.Context) error {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, "LISTEN "+inboxChannel)
	if err != nil {
		return err
	}
	_, err = conn.Exec(ctx, "LISTEN "+logsChannel)
	if err != nil {
		return err
	}
	slog.Info("ws pglistener: listening", "channels", []string{inboxChannel, logsChannel})

	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}

		var envelope struct {
			WorkspaceID string `json:"workspace_id"`
		}
		if err := json.Unmarshal([]byte(notification.Payload), &envelope); err != nil {
			slog.Warn("ws pglistener: bad payload", "err", err)
			continue
		}

		switch notification.Channel {
		case inboxChannel:
			if l.inboxHub != nil {
				l.inboxHub.Broadcast(envelope.WorkspaceID, []byte(notification.Payload))
			}
		case logsChannel:
			if l.logsHub != nil {
				l.logsHub.Broadcast(envelope.WorkspaceID, []byte(notification.Payload))
			}
		}
	}
}

// Notify sends a pg_notify for a new inbox item. Call this after
// successfully inserting an inbox_items row.
func Notify(ctx context.Context, pool *pgxpool.Pool, workspaceID string, item any) {
	msg, err := json.Marshal(map[string]any{
		"type":         "inbox.new_item",
		"workspace_id": workspaceID,
		"item":         item,
	})
	if err != nil {
		return
	}
	// pg_notify payload limit is 8KB — inbox items are well under this.
	_, err = pool.Exec(ctx, "SELECT pg_notify($1, $2)", inboxChannel, string(msg))
	if err != nil {
		slog.Warn("ws notify failed", "err", err)
	}
}

func NotifyLog(ctx context.Context, pool *pgxpool.Pool, msg any) {
	raw, err := json.Marshal(msg)
	if err != nil {
		return
	}
	_, err = pool.Exec(ctx, "SELECT pg_notify($1, $2)", logsChannel, string(raw))
	if err != nil {
		slog.Warn("ws log notify failed", "err", err)
	}
}
