package handler

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func inboxThreadKey(source, externalID, parentExternalID, authorID string) string {
	if source == "ig_dm" {
		if parentExternalID != "" {
			return parentExternalID
		}
		if authorID != "" {
			return authorID
		}
		return externalID
	}
	if parentExternalID != "" {
		return parentExternalID
	}
	return externalID
}

func resolveInboxLinkedPostID(ctx context.Context, queries *db.Queries, socialAccountID, parentExternalID string) pgtype.Text {
	if parentExternalID == "" {
		return pgtype.Text{}
	}

	postID, err := queries.FindLinkedPostIDForInboxParent(ctx, db.FindLinkedPostIDForInboxParentParams{
		SocialAccountID: socialAccountID,
		ExternalID:      pgtype.Text{String: parentExternalID, Valid: true},
	})
	if err == nil && postID != "" {
		return pgtype.Text{String: postID, Valid: true}
	}

	parentItem, err := queries.GetInboxItemByExternalID(ctx, db.GetInboxItemByExternalIDParams{
		SocialAccountID: socialAccountID,
		ExternalID:      parentExternalID,
	})
	if err == nil && parentItem.LinkedPostID.Valid {
		return parentItem.LinkedPostID
	}

	return pgtype.Text{}
}

func resolveIGDMRecipientID(ctx context.Context, queries *db.Queries, item db.InboxItem, account db.SocialAccount) string {
	if item.ParentExternalID.Valid && item.ParentExternalID.String != "" {
		threadItems, err := queries.ListInboxItemsByParent(ctx, db.ListInboxItemsByParentParams{
			SocialAccountID:  item.SocialAccountID,
			ParentExternalID: item.ParentExternalID,
		})
		if err == nil {
			for i := len(threadItems) - 1; i >= 0; i-- {
				candidate := threadItems[i]
				if candidate.AuthorID.Valid && candidate.AuthorID.String != "" && candidate.AuthorID.String != account.ExternalAccountID {
					return candidate.AuthorID.String
				}
			}
		}
	}

	if item.AuthorID.Valid && item.AuthorID.String != "" && item.AuthorID.String != account.ExternalAccountID {
		return item.AuthorID.String
	}

	return ""
}
