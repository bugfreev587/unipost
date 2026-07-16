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
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/instagramwebhooks"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/ws"
)

type inboxInstagramWebhookSubscriber interface {
	Subscribe(context.Context, string, string) error
}

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
	pool      *pgxpool.Pool

	instagramWebhookSubscriber inboxInstagramWebhookSubscriber
	igWebhookSubscriptions     map[string]bool

	// fbDMFailureCounts tracks consecutive FetchConversations
	// failures per Facebook account that hit the
	// ErrFacebookConversationsUnsupported sentinel (Meta's
	// "code 1 / subcode 99" generic). Once a count reaches
	// fbDMFailureThreshold we skip future DM polls for that
	// account to save Graph quota and log noise — these failures
	// almost always mean the Page has Messenger disabled and the
	// state won't change without operator action. Resets on
	// process restart and on the first successful fetch.
	//
	// Plain map (no mutex) is safe because poll() runs on a single
	// ticker goroutine; if we ever add concurrent polling this
	// becomes a sync.Map.
	fbDMFailureCounts map[string]int
}

// fbDMFailureThreshold is how many consecutive
// ErrFacebookConversationsUnsupported hits before the per-account
// breaker trips. Three matches the cadence: 3 ticks × 5 minutes is
// a 15-minute window, long enough to absorb a transient Meta hiccup
// but short enough that a permanently-broken Page stops emitting
// retry traffic within an hour.
const fbDMFailureThreshold = 3

func NewInboxSyncWorker(queries *db.Queries, encryptor *crypto.AESEncryptor, pool *pgxpool.Pool) *InboxSyncWorker {
	return &InboxSyncWorker{
		queries:                    queries,
		encryptor:                  encryptor,
		pool:                       pool,
		fbDMFailureCounts:          make(map[string]int),
		instagramWebhookSubscriber: instagramwebhooks.NewSubscriber(nil, ""),
		igWebhookSubscriptions:     make(map[string]bool),
	}
}

func (w *InboxSyncWorker) ensureInstagramWebhookSubscription(ctx context.Context, acc db.ListAllInboxAccountsRow, accessToken string) {
	if acc.Platform != "instagram" || w.igWebhookSubscriptions[acc.ID] {
		return
	}
	if w.instagramWebhookSubscriber == nil {
		w.instagramWebhookSubscriber = instagramwebhooks.NewSubscriber(nil, "")
	}
	if w.igWebhookSubscriptions == nil {
		w.igWebhookSubscriptions = make(map[string]bool)
	}

	if err := w.instagramWebhookSubscriber.Subscribe(ctx, acc.ExternalAccountID, accessToken); err != nil {
		slog.Warn("inbox sync worker: instagram webhook subscription repair failed",
			"account_id", acc.ID,
			"external_account_id", acc.ExternalAccountID,
			"err", err)
		return
	}

	w.igWebhookSubscriptions[acc.ID] = true
	slog.Info("inbox sync worker: instagram webhook subscription active",
		"account_id", acc.ID,
		"external_account_id", acc.ExternalAccountID)
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
	// Cleanup: remove inbox items for accounts disconnected > 7 days.
	if cleaned, cleanErr := w.queries.CleanupStaleInboxItems(ctx); cleanErr == nil && cleaned > 0 {
		slog.Info("inbox sync worker: cleaned up stale items", "deleted", cleaned)
	}

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
	newByWorkspace := map[string]int{}
	recordNew := func(workspaceID string) {
		totalNew++
		newByWorkspace[workspaceID]++
	}
	for _, acc := range accounts {
		accessToken, err := w.encryptor.Decrypt(acc.AccessToken)
		if err != nil {
			slog.Warn("inbox sync worker: decrypt failed", "account_id", acc.ID, "err", err)
			continue
		}

		w.ensureInstagramWebhookSubscription(ctx, acc, accessToken)

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
							recordNew(acc.WorkspaceID)
						}
					}
				}
			}

			// Also fetch DMs.
			dmEntries, dmErr := adapter.FetchConversations(ctx, accessToken)
			if dmErr == nil {
				senderConvMap := map[string]string{}
				for _, e := range dmEntries {
					isOwn := e.AuthorID == acc.ExternalAccountID || (e.AuthorName != "" && e.AuthorName == acc.AccountName.String)
					if !isOwn && e.ParentExternalID != "" {
						senderConvMap[e.AuthorID] = e.ParentExternalID
					}
					_, uErr := w.queries.UpsertInboxItem(ctx, db.UpsertInboxItemParams{
						SocialAccountID:  acc.ID,
						WorkspaceID:      acc.WorkspaceID,
						Source:           e.Source,
						ExternalID:       e.ExternalID,
						ParentExternalID: pgtype.Text{String: e.ParentExternalID, Valid: e.ParentExternalID != ""},
						AuthorName:       pgtype.Text{String: e.AuthorName, Valid: e.AuthorName != ""},
						AuthorID:         pgtype.Text{String: e.AuthorID, Valid: e.AuthorID != ""},
						AuthorAvatarUrl:  pgtype.Text{String: e.AuthorAvatarURL, Valid: e.AuthorAvatarURL != ""},
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
						recordNew(acc.WorkspaceID)
					}
				}
				// Reconcile: if webhook created items with thread_key = senderID,
				// update them to use the canonical conversation ID.
				for senderID, convID := range senderConvMap {
					if n, err := w.queries.ReconcileDMThreadKeys(ctx, db.ReconcileDMThreadKeysParams{
						SocialAccountID:  acc.ID,
						ThreadKey:        senderID,
						ThreadKey_2:      convID,
						ParentExternalID: pgtype.Text{String: convID, Valid: true},
					}); err == nil && n > 0 {
						slog.Info("inbox sync worker: reconciled DM thread keys",
							"sender_id", senderID, "conv_id", convID, "updated", n)
					}
				}
			}

		case "facebook":
			// Facebook's comment sync is scoped to posts we
			// published through UniPost (Q&A decision #11): 30-day
			// window, pull external_ids from social_post_results
			// instead of asking the Graph API for every Page post.
			// Keeps us well under Meta's per-user rate limit even
			// for Pages with lots of content.
			fbAdapter := platform.NewFacebookAdapter()
			postIDs, listErr := w.queries.ListPublishedExternalIDsForInboxSync(ctx, db.ListPublishedExternalIDsForInboxSyncParams{
				SocialAccountID: acc.ID,
				Column2:         30,
			})
			if listErr != nil {
				slog.Warn("inbox sync worker: list facebook posts failed", "account_id", acc.ID, "err", listErr)
				break
			}
			for _, pidText := range postIDs {
				if !pidText.Valid || pidText.String == "" {
					continue
				}
				// Resolve bare video / object ids to the combined
				// "{page_id}_{story_id}" form before fetching comments.
				// Bare ids trigger Meta's "(#12) singular statuses API
				// is deprecated for v2.4+" rejection because Graph
				// routes them through the legacy status-object path.
				// Pure string op (see ResolvePostID's doc) — no Graph
				// call. We canonicalize the row so subsequent ticks
				// see the combined form already.
				canonicalID := fbAdapter.ResolvePostID(acc.ExternalAccountID, pidText.String)
				if canonicalID != pidText.String {
					if cErr := w.queries.CanonicalizeFacebookExternalID(ctx, db.CanonicalizeFacebookExternalIDParams{
						SocialAccountID: acc.ID,
						ExternalID:      pgtype.Text{String: pidText.String, Valid: true},
						ExternalID_2:    pgtype.Text{String: canonicalID, Valid: true},
					}); cErr != nil {
						// Soft-fail — the fetch below still works on
						// the resolved id, we just pay the resolve hop
						// again on the next tick.
						slog.Warn("inbox sync worker: canonicalize facebook external id failed",
							"account_id", acc.ID, "old_id", pidText.String, "new_id", canonicalID, "err", cErr)
					} else {
						slog.Info("inbox sync worker: canonicalized facebook external id",
							"account_id", acc.ID, "old_id", pidText.String, "new_id", canonicalID)
					}
				}
				entries, fetchErr := fbAdapter.FetchComments(ctx, accessToken, canonicalID)
				if fetchErr != nil {
					// Post was deleted on FB (or became inaccessible):
					// flip the result row so the inbox-sync query
					// skips it forever. Otherwise we'd log the same
					// 400 on every sync tick.
					if errors.Is(fetchErr, platform.ErrFacebookPostNotFound) {
						if mErr := w.queries.MarkSocialPostResultRemotelyDeleted(ctx, db.MarkSocialPostResultRemotelyDeletedParams{
							SocialAccountID: acc.ID,
							ExternalID:      pgtype.Text{String: canonicalID, Valid: true},
							ErrorMessage:    pgtype.Text{String: "Post was deleted on Facebook; inbox sync stopped tracking it.", Valid: true},
						}); mErr != nil {
							slog.Warn("inbox sync worker: mark remotely-deleted failed",
								"account_id", acc.ID, "post_id", canonicalID, "err", mErr)
						} else {
							slog.Info("inbox sync worker: marked facebook post as remotely deleted",
								"account_id", acc.ID, "post_id", canonicalID)
						}
						continue
					}
					slog.Warn("inbox sync worker: facebook fetch comments failed",
						"account_id", acc.ID, "post_id", canonicalID, "err", fetchErr)
					continue
				}
				for _, e := range entries {
					// Comments authored by the Page itself shouldn't
					// show in the inbox as "to review" — flag them.
					isOwn := e.AuthorID != "" && e.AuthorID == acc.ExternalAccountID
					_, uErr := w.queries.UpsertInboxItem(ctx, db.UpsertInboxItemParams{
						SocialAccountID:  acc.ID,
						WorkspaceID:      acc.WorkspaceID,
						Source:           e.Source,
						ExternalID:       e.ExternalID,
						ParentExternalID: pgtype.Text{String: e.ParentExternalID, Valid: e.ParentExternalID != ""},
						AuthorName:       pgtype.Text{String: e.AuthorName, Valid: e.AuthorName != ""},
						AuthorID:         pgtype.Text{String: e.AuthorID, Valid: e.AuthorID != ""},
						AuthorAvatarUrl:  pgtype.Text{String: e.AuthorAvatarURL, Valid: e.AuthorAvatarURL != ""},
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
						recordNew(acc.WorkspaceID)
					}
					mergeInboxEntryAuthorMetadata(ctx, w.queries, acc.ID, e)
				}
			}

			// Messenger DMs — not scoped to UniPost-published posts
			// because DMs aren't attached to any specific post.
			// FB webhooks ARE wired (meta_webhook.go handles fb_dm),
			// so this poll is the backfill-only path. The circuit
			// breaker below skips the call entirely once an account
			// has tripped fbDMFailureThreshold consecutive
			// "Messenger likely disabled" hits, which saves a Graph
			// call per tick and stops the warn-spam.
			if w.fbDMFailureCounts[acc.ID] >= fbDMFailureThreshold {
				break
			}
			dmEntries, dmErr := fbAdapter.FetchConversations(ctx, accessToken)
			if dmErr != nil {
				if errors.Is(dmErr, platform.ErrFacebookConversationsUnsupported) {
					w.fbDMFailureCounts[acc.ID]++
					if w.fbDMFailureCounts[acc.ID] == fbDMFailureThreshold {
						// Log the trip event ONCE (==, not >=) so a
						// long-running stuck account doesn't refill
						// the log every 5 minutes. Subsequent ticks
						// short-circuit at the gate above.
						slog.Info("inbox sync worker: tripping facebook DM polling breaker",
							"account_id", acc.ID,
							"consecutive_failures", w.fbDMFailureCounts[acc.ID],
							"hint", "Page likely has Messenger disabled — re-enable in Page Settings → Messaging, then restart the API to retry")
					}
					break
				}
				slog.Warn("inbox sync worker: facebook fetch conversations failed",
					"account_id", acc.ID, "err", dmErr)
			} else {
				// Reset on success so transient hiccups don't trip
				// the breaker over time.
				if w.fbDMFailureCounts[acc.ID] != 0 {
					delete(w.fbDMFailureCounts, acc.ID)
				}
				for _, e := range dmEntries {
					isOwn := e.AuthorID != "" && e.AuthorID == acc.ExternalAccountID
					_, uErr := w.queries.UpsertInboxItem(ctx, db.UpsertInboxItemParams{
						SocialAccountID:  acc.ID,
						WorkspaceID:      acc.WorkspaceID,
						Source:           e.Source,
						ExternalID:       e.ExternalID,
						ParentExternalID: pgtype.Text{String: e.ParentExternalID, Valid: e.ParentExternalID != ""},
						AuthorName:       pgtype.Text{String: e.AuthorName, Valid: e.AuthorName != ""},
						AuthorID:         pgtype.Text{String: e.AuthorID, Valid: e.AuthorID != ""},
						AuthorAvatarUrl:  pgtype.Text{String: e.AuthorAvatarURL, Valid: e.AuthorAvatarURL != ""},
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
						recordNew(acc.WorkspaceID)
					}
					mergeInboxEntryAuthorMetadata(ctx, w.queries, acc.ID, e)
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
							recordNew(acc.WorkspaceID)
						}
					}
				}
			}
		}
	}

	if totalNew > 0 {
		slog.Info("inbox sync worker: new items", "count", totalNew)
		for workspaceID, newItems := range newByWorkspace {
			ws.NotifyEvent(ctx, w.pool, workspaceID, map[string]any{
				"type":      "inbox.sync_complete",
				"new_items": newItems,
			})
		}
	}
}

func mergeInboxEntryAuthorMetadata(ctx context.Context, queries *db.Queries, socialAccountID string, entry platform.InboxEntry) {
	if entry.AuthorName == "" && entry.AuthorID == "" && entry.AuthorAvatarURL == "" {
		return
	}
	_, _ = queries.MergeInboxItemAuthorMetadataByExternalID(ctx, db.MergeInboxItemAuthorMetadataByExternalIDParams{
		SocialAccountID: socialAccountID,
		ExternalID:      entry.ExternalID,
		AuthorName:      entry.AuthorName,
		AuthorID:        entry.AuthorID,
		AuthorAvatarUrl: entry.AuthorAvatarURL,
	})
}
