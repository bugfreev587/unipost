package handler

import (
	"context"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

const accountNotAvailableOnFreePlanMessage = "This social account is already connected to another workspace. Free plan workspaces cannot share the same connected social account."

func workspacePlanID(ctx context.Context, queries *db.Queries, workspaceID string) string {
	sub, err := queries.GetSubscriptionByWorkspace(ctx, workspaceID)
	if err != nil || sub.PlanID == "" {
		return "free"
	}
	return sub.PlanID
}

func freePlanSharingBlocked(ctx context.Context, queries *db.Queries, workspaceID, platformName, externalAccountID string) (bool, error) {
	if workspaceID == "" || platformName == "" || externalAccountID == "" {
		return false, nil
	}
	if workspacePlanID(ctx, queries, workspaceID) != "free" {
		return false, nil
	}

	rows, err := queries.FindAllSocialAccountsByPlatformAndExternalID(ctx, db.FindAllSocialAccountsByPlatformAndExternalIDParams{
		Platform:          platformName,
		ExternalAccountID: externalAccountID,
	})
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.WorkspaceID != workspaceID {
			return true, nil
		}
	}
	return false, nil
}
