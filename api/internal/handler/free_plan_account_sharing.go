package handler

import (
	"context"

	"github.com/xiaoboyu/unipost-api/internal/auth"
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

func workspaceIsSuperAdmin(ctx context.Context, queries *db.Queries, checker *auth.SuperAdminChecker, workspaceID string) bool {
	if checker == nil || workspaceID == "" {
		return false
	}
	workspace, err := queries.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return false
	}
	user, err := queries.GetUser(ctx, workspace.UserID)
	if err != nil {
		return checker.IsSuperAdmin(ctx, workspace.UserID)
	}
	return checker.IsSuperAdminByUser(workspace.UserID, user.Email)
}

func freePlanSharingBlocked(ctx context.Context, queries *db.Queries, checker *auth.SuperAdminChecker, workspaceID, platformName, externalAccountID string) (bool, error) {
	if workspaceID == "" || platformName == "" || externalAccountID == "" {
		return false, nil
	}
	if workspacePlanID(ctx, queries, workspaceID) != "free" {
		return false, nil
	}
	if workspaceIsSuperAdmin(ctx, queries, checker, workspaceID) {
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

func freePlanManagedSharingBlocked(ctx context.Context, queries *db.Queries, checker *auth.SuperAdminChecker, workspaceID, platformName, providerIdentity string) (bool, error) {
	if workspaceID == "" || platformName == "" || providerIdentity == "" {
		return false, nil
	}
	if workspacePlanID(ctx, queries, workspaceID) != "free" {
		return false, nil
	}
	if workspaceIsSuperAdmin(ctx, queries, checker, workspaceID) {
		return false, nil
	}
	return queries.ExistsActiveAccountInOtherWorkspaceByProviderIdentity(ctx, db.ExistsActiveAccountInOtherWorkspaceByProviderIdentityParams{
		WorkspaceID:      workspaceID,
		Platform:         platformName,
		ProviderIdentity: providerIdentity,
	})
}
