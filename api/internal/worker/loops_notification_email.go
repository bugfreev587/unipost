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
	if b == nil || b.queries == nil || b.syncer == nil {
		return
	}
	switch event {
	case events.EventAccountDisconnected:
		b.publishAccountDisconnected(ctx, workspaceID, data)
	case events.EventAccountConnected:
		b.publishFirstAccountConnected(ctx, workspaceID, data)
	case events.EventPostPublished:
		b.publishFirstPostPublished(ctx, workspaceID, data)
	}
}

func (b *LoopsNotificationEmailBus) publishAccountDisconnected(ctx context.Context, workspaceID string, data any) {
	workspace, owner, ok := b.workspaceOwner(ctx, workspaceID, "account_disconnected")
	if !ok {
		return
	}
	payload := lifecyclePayload(data)
	lifecycleEvent := buildLoopsAccountDisconnectedEvent(owner, workspace, payload, b.appBaseURL)
	if err := b.syncer.SendLifecycleEvent(ctx, lifecycleEvent); err != nil {
		slog.Warn("loops: failed to send account_disconnected", "workspace_id", workspaceID, "user_id", owner.ID, "error", err)
	}
}

func (b *LoopsNotificationEmailBus) publishFirstAccountConnected(ctx context.Context, workspaceID string, data any) {
	accounts, err := b.queries.ListSocialAccountsByWorkspace(ctx, workspaceID)
	if err != nil {
		slog.Warn("loops: failed to count connected accounts for first_account_connected", "workspace_id", workspaceID, "error", err)
		return
	}
	if len(accounts) != 1 {
		return
	}
	workspace, owner, ok := b.workspaceOwner(ctx, workspaceID, "first_account_connected")
	if !ok {
		return
	}
	payload := lifecyclePayload(data)
	lifecycleEvent := buildLoopsFirstAccountConnectedEvent(owner, workspace, payload, b.appBaseURL)
	if err := b.syncer.SendLifecycleEvent(ctx, lifecycleEvent); err != nil {
		slog.Warn("loops: failed to send first_account_connected", "workspace_id", workspaceID, "user_id", owner.ID, "error", err)
	}
}

func (b *LoopsNotificationEmailBus) publishFirstPostPublished(ctx context.Context, workspaceID string, data any) {
	count, err := b.queries.CountPublishedPostsByWorkspace(ctx, workspaceID)
	if err != nil {
		slog.Warn("loops: failed to count published posts for first_post_published", "workspace_id", workspaceID, "error", err)
		return
	}
	if count != 1 {
		return
	}
	workspace, owner, ok := b.workspaceOwner(ctx, workspaceID, "first_post_published")
	if !ok {
		return
	}
	payload := lifecyclePayload(data)
	lifecycleEvent := buildLoopsFirstPostPublishedEvent(owner, workspace, payload, b.appBaseURL)
	if err := b.syncer.SendLifecycleEvent(ctx, lifecycleEvent); err != nil {
		slog.Warn("loops: failed to send first_post_published", "workspace_id", workspaceID, "user_id", owner.ID, "error", err)
	}
}

func (b *LoopsNotificationEmailBus) workspaceOwner(ctx context.Context, workspaceID, eventName string) (db.Workspace, db.User, bool) {
	workspace, err := b.queries.GetWorkspace(ctx, workspaceID)
	if err != nil {
		slog.Warn("loops: failed to load workspace for lifecycle event", "workspace_id", workspaceID, "event", eventName, "error", err)
		return db.Workspace{}, db.User{}, false
	}
	owner, err := b.queries.GetUser(ctx, workspace.UserID)
	if err != nil {
		slog.Warn("loops: failed to load workspace owner for lifecycle event", "workspace_id", workspaceID, "user_id", workspace.UserID, "event", eventName, "error", err)
		return db.Workspace{}, db.User{}, false
	}
	if strings.TrimSpace(owner.Email) == "" {
		return db.Workspace{}, db.User{}, false
	}
	return workspace, owner, true
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

func buildLoopsFirstAccountConnectedEvent(owner db.User, workspace db.Workspace, payload map[string]any, appBaseURL string) loops.LifecycleEvent {
	profileID := lifecycleString(payload, "profile_id")
	return loops.LifecycleEvent{
		UserID:         owner.ID,
		Email:          owner.Email,
		Name:           lifecycleUserName(owner),
		WorkspaceID:    workspace.ID,
		WorkspaceName:  workspace.Name,
		EventName:      "first_account_connected",
		IdempotencyKey: "first_account_connected:" + workspace.ID,
		Properties: map[string]any{
			"workspace_name":           workspace.Name,
			"social_account_id":        lifecycleString(payload, "social_account_id"),
			"profile_id":               profileID,
			"platform":                 lifecycleString(payload, "platform"),
			"account_name":             lifecycleString(payload, "account_name"),
			"dashboard_url":            accountReconnectURL(appBaseURL, workspace.ID, profileID),
			"activation_state":         "has_account",
			"connected_accounts_count": int32(1),
		},
	}
}

func buildLoopsFirstPostPublishedEvent(owner db.User, workspace db.Workspace, payload map[string]any, appBaseURL string) loops.LifecycleEvent {
	postID := lifecycleString(payload, "id")
	profileID := firstProfileID(payload)
	return loops.LifecycleEvent{
		UserID:         owner.ID,
		Email:          owner.Email,
		Name:           lifecycleUserName(owner),
		WorkspaceID:    workspace.ID,
		WorkspaceName:  workspace.Name,
		EventName:      "first_post_published",
		IdempotencyKey: "first_post_published:" + workspace.ID,
		Properties: map[string]any{
			"workspace_name":        workspace.Name,
			"post_id":               postID,
			"profile_id":            profileID,
			"dashboard_url":         postDashboardURL(appBaseURL, workspace.ID, profileID, postID),
			"activation_state":      "activated",
			"published_posts_count": int32(1),
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

func postDashboardURL(appBaseURL, workspaceID, profileID, postID string) string {
	base := strings.TrimRight(strings.TrimSpace(appBaseURL), "/")
	if base == "" {
		base = "https://app.unipost.dev"
	}
	projectID := strings.TrimSpace(profileID)
	if projectID == "" {
		projectID = strings.TrimSpace(workspaceID)
	}
	return fmt.Sprintf("%s/projects/%s/logs?post_id=%s", base, url.PathEscape(projectID), url.QueryEscape(postID))
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

func firstProfileID(payload map[string]any) string {
	if value := lifecycleString(payload, "profile_id"); value != "" {
		return value
	}
	if payload == nil {
		return ""
	}
	switch v := payload["profile_ids"].(type) {
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	case []string:
		for _, item := range v {
			if strings.TrimSpace(item) != "" {
				return strings.TrimSpace(item)
			}
		}
	}
	return ""
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
