package db

import (
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// AsSocialAccount preserves the public binding ID/Profile while replacing
// credential, ownership, and health fields with the resolved physical
// connection projection produced by GetResolvedSocialAccountByIDAndWorkspace.
func (row GetResolvedSocialAccountByIDAndWorkspaceRow) AsSocialAccount() SocialAccount {
	optionalText := func(value string) pgtype.Text {
		return pgtype.Text{String: value, Valid: strings.TrimSpace(value) != ""}
	}
	return SocialAccount{
		ID:                row.ID,
		ProfileID:         row.ProfileID,
		Platform:          row.Platform,
		AccessToken:       row.ResolvedAccessToken,
		RefreshToken:      optionalText(row.ResolvedRefreshToken),
		TokenExpiresAt:    row.ResolvedTokenExpiresAt,
		ExternalAccountID: row.ExternalAccountID,
		AccountName:       optionalText(row.ResolvedAccountName),
		AccountAvatarUrl:  optionalText(row.ResolvedAccountAvatarUrl),
		ConnectedAt:       row.ResolvedConnectedAt,
		DisconnectedAt:    row.ResolvedDisconnectedAt,
		Metadata:          row.ResolvedMetadata,
		Scope:             row.ResolvedScope,
		Status:            row.ResolvedStatus,
		ConnectionType:    row.ResolvedConnectionType,
		ConnectSessionID:  row.ConnectSessionID,
		ExternalUserID:    optionalText(row.ResolvedExternalUserID),
		ExternalUserEmail: optionalText(row.ResolvedExternalUserEmail),
		LastRefreshedAt:   row.ResolvedLastRefreshedAt,
		XAppMode:          optionalText(row.ResolvedXAppMode),
		ConnectionID:      row.ConnectionID,
		BindingVersion:    row.BindingVersion,
		BindingStatus:     row.BindingStatus,
	}
}
