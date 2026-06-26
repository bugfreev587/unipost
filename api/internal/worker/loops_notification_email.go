package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
	"github.com/xiaoboyu/unipost-api/internal/loops"
)

type lifecycleEventSender interface {
	SendLifecycleEvent(context.Context, loops.LifecycleEvent) error
}

type LoopsNotificationEmailBus struct {
	queries    *db.Queries
	syncer     lifecycleEventSender
	appBaseURL string
}

func NewLoopsNotificationEmailBus(queries *db.Queries, syncer lifecycleEventSender, appBaseURL string) *LoopsNotificationEmailBus {
	return &LoopsNotificationEmailBus{
		queries:    queries,
		syncer:     syncer,
		appBaseURL: strings.TrimRight(strings.TrimSpace(appBaseURL), "/"),
	}
}

func (b *LoopsNotificationEmailBus) Publish(ctx context.Context, workspaceID, event string, data any) {
	if b == nil || b.queries == nil || b.syncer == nil || event != events.EventAccountDisconnected {
		return
	}

	workspace, err := b.queries.GetWorkspace(ctx, workspaceID)
	if err != nil {
		slog.Warn("loops: failed to load workspace for account_disconnected", "workspace_id", workspaceID, "error", err)
		return
	}
	owner, err := b.queries.GetUser(ctx, workspace.UserID)
	if err != nil {
		slog.Warn("loops: failed to load workspace owner for account_disconnected", "workspace_id", workspaceID, "user_id", workspace.UserID, "error", err)
		return
	}
	if strings.TrimSpace(owner.Email) == "" {
		return
	}

	payload := lifecyclePayload(data)
	lifecycleEvent := buildLoopsAccountDisconnectedEvent(owner, workspace, payload, b.appBaseURL)
	if err := b.syncer.SendLifecycleEvent(ctx, lifecycleEvent); err != nil {
		slog.Warn("loops: failed to send account_disconnected", "workspace_id", workspaceID, "user_id", owner.ID, "error", err)
	}
}

func buildLoopsAccountDisconnectedEvent(owner db.User, workspace db.Workspace, payload map[string]any, appBaseURL string) loops.LifecycleEvent {
	accountID := lifecycleString(payload, "social_account_id")
	reason := lifecycleString(payload, "reason")
	if reason == "" {
		reason = "unknown"
	}

	platform := lifecycleString(payload, "platform")
	accountName := lifecycleString(payload, "account_name")
	return loops.LifecycleEvent{
		UserID:         owner.ID,
		Email:          owner.Email,
		Name:           lifecycleUserName(owner),
		WorkspaceID:    workspace.ID,
		WorkspaceName:  workspace.Name,
		EventName:      "account_disconnected",
		IdempotencyKey: fmt.Sprintf("account_disconnected:%s:%s", firstNonEmpty(accountID, "unknown_account"), reason),
		Properties: map[string]any{
			"workspace_name": workspace.Name,
			"platform":       platform,
			"account_name":   accountName,
			"reconnect_url":  accountReconnectURL(appBaseURL, workspace.ID, lifecycleString(payload, "profile_id")),
			"reason":         reason,
		},
	}
}

func accountReconnectURL(appBaseURL, workspaceID, profileID string) string {
	base := strings.TrimRight(strings.TrimSpace(appBaseURL), "/")
	if base == "" {
		base = "https://app.unipost.dev"
	}
	projectID := strings.TrimSpace(profileID)
	if projectID == "" {
		projectID = strings.TrimSpace(workspaceID)
	}
	return fmt.Sprintf("%s/projects/%s/accounts", base, url.PathEscape(projectID))
}

func lifecyclePayload(data any) map[string]any {
	if data == nil {
		return map[string]any{}
	}
	if payload, ok := data.(map[string]any); ok {
		return payload
	}
	payload := map[string]any{}
	b, err := json.Marshal(data)
	if err != nil {
		return payload
	}
	_ = json.Unmarshal(b, &payload)
	return payload
}

func lifecycleString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	switch v := payload[key].(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return ""
	}
}

func lifecycleUserName(user db.User) string {
	if user.Name.Valid {
		return user.Name.String
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
