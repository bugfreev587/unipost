package handler

import "github.com/xiaoboyu/unipost-api/internal/db"

// resolvedSocialAccountFromRow keeps the public binding identity while moving
// credentials, owner, and connection health to their physical connection. The
// query already applies the explicit legacy fallback when connection_id is
// NULL; this conversion only restores the established db.SocialAccount shape
// used by adapters and state helpers.
func resolvedSocialAccountFromRow(row db.GetResolvedSocialAccountByIDAndWorkspaceRow) db.SocialAccount {
	return row.AsSocialAccount()
}
