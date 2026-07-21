package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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

type inboxNotificationEnvelope struct {
	Type           string  `json:"type"`
	WorkspaceID    string  `json:"workspace_id"`
	ExternalUserID *string `json:"external_user_id,omitempty"`
}

type pgNotifyExecutor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
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

		if err := l.forwardNotification(notification.Channel, []byte(notification.Payload)); err != nil {
			slog.Warn("ws pglistener: rejected notification", "channel", notification.Channel, "err", err)
		}
	}
}

// forwardNotification is the pure routing boundary used by the LISTEN loop.
// Inbox payloads are accepted only when their server-produced routing envelope
// is canonical. The original bytes are preserved for downstream clients.
func (l *PGListener) forwardNotification(channel string, payload []byte) error {
	switch channel {
	case inboxChannel:
		var envelope inboxNotificationEnvelope
		if err := json.Unmarshal(payload, &envelope); err != nil {
			return errors.New("invalid inbox notification JSON")
		}
		if strings.TrimSpace(envelope.Type) == "" ||
			strings.TrimSpace(envelope.WorkspaceID) == "" ||
			strings.TrimSpace(envelope.WorkspaceID) != envelope.WorkspaceID {
			return errors.New("invalid inbox notification route")
		}
		externalUserID := ""
		if envelope.ExternalUserID != nil {
			externalUserID = *envelope.ExternalUserID
			if strings.TrimSpace(externalUserID) == "" || strings.TrimSpace(externalUserID) != externalUserID {
				return errors.New("invalid inbox notification owner")
			}
		}
		if l.inboxHub != nil {
			l.inboxHub.BroadcastInbox(envelope.WorkspaceID, externalUserID, payload)
		}
		return nil
	case logsChannel:
		var envelope struct {
			WorkspaceID string `json:"workspace_id"`
		}
		if err := json.Unmarshal(payload, &envelope); err != nil {
			return errors.New("invalid logs notification JSON")
		}
		if strings.TrimSpace(envelope.WorkspaceID) == "" || strings.TrimSpace(envelope.WorkspaceID) != envelope.WorkspaceID {
			return errors.New("invalid logs notification route")
		}
		if l.logsHub != nil {
			l.logsHub.Broadcast(envelope.WorkspaceID, payload)
		}
		return nil
	default:
		return errors.New("unsupported notification channel")
	}
}

// Notify sends a pg_notify for a new inbox item. Call this after
// successfully inserting an inbox_items row.
func Notify(ctx context.Context, pool *pgxpool.Pool, workspaceID, externalUserID string, item any) {
	notifyItemWithExecutor(ctx, pool, workspaceID, externalUserID, item)
}

func notifyItemWithExecutor(ctx context.Context, executor pgNotifyExecutor, workspaceID, externalUserID string, item any) {
	payload := map[string]any{
		"type":         "inbox.new_item",
		"workspace_id": workspaceID,
		"item":         item,
	}
	if externalUserID != "" {
		payload["external_user_id"] = externalUserID
	}
	emitInboxNotification(ctx, executor, workspaceID, externalUserID, payload)
}

// NotifyEvent sends a managed-user Inbox event. Routing fields supplied in
// event are ignored; workspace and owner come only from the server call site.
func NotifyEvent(ctx context.Context, pool *pgxpool.Pool, workspaceID, externalUserID string, event map[string]any) {
	notifyEventWithExecutor(ctx, pool, workspaceID, externalUserID, event)
}

func notifyEventWithExecutor(ctx context.Context, executor pgNotifyExecutor, workspaceID, externalUserID string, event map[string]any) {
	emitInboxNotification(ctx, executor, workspaceID, externalUserID, event)
}

// NotifyWorkspaceEvent sends an aggregate-only Inbox event. It deliberately
// omits external_user_id instead of serializing an ambiguous empty owner.
func NotifyWorkspaceEvent(ctx context.Context, pool *pgxpool.Pool, workspaceID string, event map[string]any) {
	notifyWorkspaceEventWithExecutor(ctx, pool, workspaceID, event)
}

func notifyWorkspaceEventWithExecutor(ctx context.Context, executor pgNotifyExecutor, workspaceID string, event map[string]any) {
	emitInboxNotification(ctx, executor, workspaceID, "", event)
}

func emitInboxNotification(ctx context.Context, executor pgNotifyExecutor, workspaceID, externalUserID string, event map[string]any) {
	if executor == nil || strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(workspaceID) != workspaceID {
		return
	}
	if externalUserID != "" && (strings.TrimSpace(externalUserID) == "" || strings.TrimSpace(externalUserID) != externalUserID) {
		return
	}
	eventType, _ := event["type"].(string)
	if strings.TrimSpace(eventType) == "" || strings.TrimSpace(eventType) != eventType {
		return
	}
	payload := make(map[string]any, len(event)+1)
	for k, v := range event {
		if k == "type" || k == "workspace_id" || k == "external_user_id" {
			continue
		}
		payload[k] = v
	}
	payload["type"] = eventType
	payload["workspace_id"] = workspaceID
	if externalUserID != "" {
		payload["external_user_id"] = externalUserID
	}
	msg, err := json.Marshal(payload)
	if err != nil {
		return
	}
	// pg_notify payload limit is 8KB — inbox items are well under this.
	_, err = executor.Exec(ctx, "SELECT pg_notify($1, $2)", inboxChannel, string(msg))
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
