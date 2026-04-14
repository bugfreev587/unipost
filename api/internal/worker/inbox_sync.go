// inbox_sync.go is a background worker that periodically polls
// Instagram and Threads for new comments/replies and inserts them
// into the inbox_items table.
//
// Instagram comments/DMs also arrive via webhooks (meta_webhook.go),
// but the poller serves as a backfill mechanism and covers Threads
// which has no webhook support for replies.
//
// Runs every 5 minutes. Each tick iterates all active IG/Threads
// social accounts, fetches comments on their 5 most recent posts,
// and upserts into inbox_items (idempotent via the UNIQUE constraint
// on social_account_id + external_id).

package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
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

type InboxSyncWorker struct {
	queries   *db.Queries
	encryptor *crypto.AESEncryptor
}

func NewInboxSyncWorker(queries *db.Queries, encryptor *crypto.AESEncryptor) *InboxSyncWorker {
	return &InboxSyncWorker{queries: queries, encryptor: encryptor}
}

func (w *InboxSyncWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	slog.Info("inbox sync worker started")

	// Run once on startup after a short delay to let the DB warm up.
	time.Sleep(30 * time.Second)
	w.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("inbox sync worker stopped")
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

func (w *InboxSyncWorker) poll(ctx context.Context) {
	// Find all workspaces that have IG/Threads accounts by scanning
	// all active accounts across all workspaces.
	accounts, err := w.queries.ListAllInboxAccounts(ctx)
	if err != nil {
		slog.Error("inbox sync worker: list accounts failed", "err", err)
		return
	}
	if len(accounts) == 0 {
		return
	}

	totalNew := 0
	for _, acc := range accounts {
		accessToken, err := w.encryptor.Decrypt(acc.AccessToken)
		if err != nil {
			slog.Warn("inbox sync worker: decrypt failed", "account_id", acc.ID, "err", err)
			continue
		}

		switch acc.Platform {
		case "instagram":
			adapter := platform.NewInstagramAdapter()
			// Fetch recent media directly from IG API.
			mediaIDs, mediaErr := adapter.FetchRecentMedia(ctx, accessToken)
			if mediaErr != nil {
				slog.Warn("inbox sync worker: fetch ig media failed", "account_id", acc.ID, "err", mediaErr)
			} else {
				for _, mediaID := range mediaIDs {
					entries, fetchErr := adapter.FetchComments(ctx, accessToken, mediaID)
					if fetchErr != nil {
						continue
					}
					for _, e := range entries {
						_, uErr := w.queries.UpsertInboxItem(ctx, db.UpsertInboxItemParams{
							SocialAccountID:  acc.ID,
							WorkspaceID:      acc.WorkspaceID,
							Source:           e.Source,
							ExternalID:       e.ExternalID,
							ParentExternalID: pgtype.Text{String: e.ParentExternalID, Valid: e.ParentExternalID != ""},
							AuthorName:       pgtype.Text{String: e.AuthorName, Valid: e.AuthorName != ""},
							AuthorID:         pgtype.Text{String: e.AuthorID, Valid: e.AuthorID != ""},
							Body:             pgtype.Text{String: e.Body, Valid: e.Body != ""},
							IsOwn:            false,
							ReceivedAt:       pgtype.Timestamptz{Time: e.Timestamp, Valid: true},
							Metadata:         []byte("{}"),
							ThreadKey:        inboxThreadKey(e.Source, e.ExternalID, e.ParentExternalID, e.AuthorID),
							ThreadStatus:     "open",
							AssignedTo:       pgtype.Text{},
							LinkedPostID:     resolveInboxLinkedPostID(ctx, w.queries, acc.ID, e.ParentExternalID),
						})
						if uErr == nil {
							totalNew++
						}
					}
				}
			}

			// Also fetch DMs.
			dmEntries, dmErr := adapter.FetchConversations(ctx, accessToken)
			if dmErr == nil {
				for _, e := range dmEntries {
					isOwn := e.AuthorID == acc.ExternalAccountID
					_, uErr := w.queries.UpsertInboxItem(ctx, db.UpsertInboxItemParams{
						SocialAccountID:  acc.ID,
						WorkspaceID:      acc.WorkspaceID,
						Source:           e.Source,
						ExternalID:       e.ExternalID,
						ParentExternalID: pgtype.Text{String: e.ParentExternalID, Valid: e.ParentExternalID != ""},
						AuthorName:       pgtype.Text{String: e.AuthorName, Valid: e.AuthorName != ""},
						AuthorID:         pgtype.Text{String: e.AuthorID, Valid: e.AuthorID != ""},
						Body:             pgtype.Text{String: e.Body, Valid: e.Body != ""},
						IsOwn:            isOwn,
						ReceivedAt:       pgtype.Timestamptz{Time: e.Timestamp, Valid: true},
						Metadata:         []byte("{}"),
						ThreadKey:        inboxThreadKey(e.Source, e.ExternalID, e.ParentExternalID, e.AuthorID),
						ThreadStatus:     "open",
						AssignedTo:       pgtype.Text{},
						LinkedPostID:     resolveInboxLinkedPostID(ctx, w.queries, acc.ID, e.ParentExternalID),
					})
					if uErr == nil {
						totalNew++
					}
				}
			}

		case "threads":
			adapter := platform.NewThreadsAdapter()
			// Fetch recent posts directly from Threads API.
			postIDs, mediaErr := adapter.FetchRecentMedia(ctx, accessToken)
			if mediaErr != nil {
				slog.Warn("inbox sync worker: fetch threads media failed", "account_id", acc.ID, "err", mediaErr)
			} else {
				for _, postID := range postIDs {
					entries, fetchErr := adapter.FetchComments(ctx, accessToken, postID)
					if fetchErr != nil {
						continue
					}
					for _, e := range entries {
						isOwn := e.AuthorName == acc.AccountName.String
						_, uErr := w.queries.UpsertInboxItem(ctx, db.UpsertInboxItemParams{
							SocialAccountID:  acc.ID,
							WorkspaceID:      acc.WorkspaceID,
							Source:           e.Source,
							ExternalID:       e.ExternalID,
							ParentExternalID: pgtype.Text{String: e.ParentExternalID, Valid: e.ParentExternalID != ""},
							AuthorName:       pgtype.Text{String: e.AuthorName, Valid: e.AuthorName != ""},
							AuthorID:         pgtype.Text{String: e.AuthorID, Valid: e.AuthorID != ""},
							Body:             pgtype.Text{String: e.Body, Valid: e.Body != ""},
							IsOwn:            isOwn,
							ReceivedAt:       pgtype.Timestamptz{Time: e.Timestamp, Valid: true},
							Metadata:         []byte("{}"),
							ThreadKey:        inboxThreadKey(e.Source, e.ExternalID, e.ParentExternalID, e.AuthorID),
							ThreadStatus:     "open",
							AssignedTo:       pgtype.Text{},
							LinkedPostID:     resolveInboxLinkedPostID(ctx, w.queries, acc.ID, e.ParentExternalID),
						})
						if uErr == nil {
							totalNew++
						}
					}
				}
			}
		}
	}

	if totalNew > 0 {
		slog.Info("inbox sync worker: new items", "count", totalNew)
	}
}
