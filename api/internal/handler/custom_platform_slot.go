package handler

import (
	"context"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func planAllowsPlatformCredentials(planID string) bool {
	switch planID {
	case "basic", "growth", "team", "enterprise":
		return true
	default:
		return false
	}
}

func workspaceAllowsPlatformCredentialsForPlatform(ctx context.Context, queries *db.Queries, workspaceID, platform string) bool {
	if workspaceID == "" {
		return false
	}
	planID := "free"
	if sub, err := queries.GetSubscriptionByWorkspace(ctx, workspaceID); err == nil && sub.PlanID != "" {
		planID = sub.PlanID
	}
	if !planAllowsPlatformCredentials(planID) {
		return false
	}
	if planID != "basic" {
		return true
	}
	ws, err := queries.GetWorkspace(ctx, workspaceID)
	if err != nil || !ws.CustomPlatformSlot.Valid {
		return false
	}
	return ws.CustomPlatformSlot.String == platform
}
