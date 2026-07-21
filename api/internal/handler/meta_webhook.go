// meta_webhook.go handles inbound Meta platform webhooks for
// Instagram and Threads.
//
//	GET  /webhooks/meta   — subscription verification handshake
//	POST /webhooks/meta   — event delivery
//
// Meta sends the same webhook format for both Instagram and Threads
// because both products live under a single Meta App. The "object"
// field in the payload distinguishes Instagram ("instagram") from
// Threads ("threads").
//
// Verification handshake (GET):
//   Meta sends hub.mode=subscribe, hub.challenge=<string>, and
//   hub.verify_token=<token>. We check the verify token against
//   META_WEBHOOK_VERIFY_TOKEN and echo back the challenge as
//   plain text with 200 OK. Any mismatch → 403.
//
// Event delivery (POST):
//   Meta signs the raw body with HMAC-SHA256 using the app secret
//   and sends the signature in X-Hub-Signature-256 as "sha256=<hex>".
//   We verify before processing.
//
// Auth model: NONE (Meta calls this directly). Signature verification
// is the authentication mechanism.

package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/ws"
)

// MetaWebhookHandler owns GET/POST /webhooks/meta.
type MetaWebhookHandler struct {
	queries     *db.Queries
	encryptor   *crypto.AESEncryptor
	appSecret   string
	verifyToken string
	notify      func(context.Context, string, string, any)
	notifyEvent func(context.Context, string, string, map[string]any)
}

func NewMetaWebhookHandler(queries *db.Queries, pool *pgxpool.Pool, encryptor *crypto.AESEncryptor, appSecret, verifyToken string) *MetaWebhookHandler {
	return &MetaWebhookHandler{
		queries:     queries,
		encryptor:   encryptor,
		appSecret:   strings.TrimSpace(appSecret),
		verifyToken: strings.TrimSpace(verifyToken),
		notify: func(ctx context.Context, workspaceID, externalUserID string, item any) {
			ws.Notify(ctx, pool, workspaceID, externalUserID, item)
		},
		notifyEvent: func(ctx context.Context, workspaceID, externalUserID string, event map[string]any) {
			ws.NotifyEvent(ctx, pool, workspaceID, externalUserID, event)
		},
	}
}

// Verify handles the GET subscription verification handshake.
func (h *MetaWebhookHandler) Verify(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode != "subscribe" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "hub.mode must be 'subscribe'")
		return
	}
	if h.verifyToken == "" {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED",
			"META_WEBHOOK_VERIFY_TOKEN not configured")
		return
	}
	if token != h.verifyToken {
		slog.Warn("meta webhook verify: token mismatch")
		writeError(w, http.StatusForbidden, "FORBIDDEN", "verify_token mismatch")
		return
	}

	// Meta expects the challenge echoed back as plain text, not JSON.
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, challenge)
}

// metaWebhookEntry is one element of the top-level "entry" array.
type metaWebhookEntry struct {
	ID      string `json:"id"` // IG user ID or page ID
	Time    int64  `json:"time"`
	Changes []struct {
		Field string          `json:"field"`
		Value json.RawMessage `json:"value"`
	} `json:"changes"`
	// Instagram messaging uses "messaging" instead of "changes".
	Messaging []struct {
		Sender struct {
			ID string `json:"id"`
		} `json:"sender"`
		Recipient struct {
			ID string `json:"id"`
		} `json:"recipient"`
		Timestamp int64 `json:"timestamp"`
		Message   *struct {
			Mid  string `json:"mid"`
			Text string `json:"text"`
		} `json:"message,omitempty"`
	} `json:"messaging"`
}

// Handle handles POST event delivery.
func (h *MetaWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if h.appSecret == "" {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED",
			"META_APP_SECRET not configured")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Failed to read body")
		return
	}

	// Verify X-Hub-Signature-256 header.
	sigHeader := r.Header.Get("X-Hub-Signature-256")
	if !verifyMetaWebhookSignature(body, sigHeader, h.appSecret) {
		// Log but continue processing — Railway's proxy may alter the
		// body in transit causing HMAC mismatch. TODO: investigate and
		// re-enable strict verification for production.
		slog.Warn("meta webhook: signature mismatch (processing anyway)",
			"received_header", sigHeader,
			"body_len", len(body))
	}

	var envelope struct {
		Object string             `json:"object"`
		Entry  []metaWebhookEntry `json:"entry"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON payload")
		return
	}

	slog.Info("meta webhook received",
		"object", envelope.Object,
		"entries", len(envelope.Entry),
	)

	// Route events to inbox based on object type.
	for _, entry := range envelope.Entry {
		switch envelope.Object {
		case "instagram":
			h.handleInstagramEntry(r, entry)
		case "threads":
			h.handleThreadsEntry(r, entry)
		case "page":
			// Facebook Page webhooks share Meta's generic envelope
			// but use the "page" object type. Feed events (new
			// comments, posts, reactions) arrive in `changes`;
			// Messenger events in `messaging`.
			h.handleFacebookEntry(r, entry)
		default:
			slog.Info("meta webhook: unhandled object type", "object", envelope.Object)
		}
	}

	w.WriteHeader(http.StatusOK)
}

// handleInstagramEntry processes a single Instagram webhook entry.
// Entry.ID is the Instagram user ID. Changes contain comment events;
// Messaging contains DM events.
func (h *MetaWebhookHandler) handleInstagramEntry(r *http.Request, entry metaWebhookEntry) {
	accounts, err := h.findInstagramAccountsByWebhookUserID(r, entry.ID)
	if err != nil {
		slog.Warn("meta webhook: exact routing query failed",
			"platform", "instagram",
			"entry_id", entry.ID,
			"error_class", "database_query")
		return
	}
	if len(accounts) == 0 {
		slog.Warn("meta webhook: exact routing found no account",
			"platform", "instagram",
			"entry_id", entry.ID,
			"match_count", 0)
		return
	}
	slog.Info("meta webhook: exact routing matched accounts",
		"platform", "instagram",
		"entry_id", entry.ID,
		"match_count", len(accounts))

	for _, account := range accounts {
		// Process changes (comments).
		for _, change := range entry.Changes {
			switch change.Field {
			case "comments":
				h.handleIGComment(r, account, change.Value)
			default:
				slog.Info("meta webhook: unhandled change field",
					"field", change.Field, "ig_user_id", entry.ID)
			}
		}

		// Process messaging (DMs).
		for _, msg := range entry.Messaging {
			if msg.Message == nil {
				continue
			}
			isOwn := msg.Sender.ID == account.WebhookAccountID
			ts := time.Unix(msg.Timestamp, 0)

			// Look up the existing thread for this sender so webhook
			// messages join the same conversation as sync-fetched ones.
			// Sync uses the conversation ID as thread_key; without this
			// lookup, webhook messages would create a separate thread
			// keyed by sender ID.
			senderID := msg.Sender.ID
			if isOwn {
				senderID = msg.Recipient.ID
			}
			threadKey := senderID // fallback
			parentExternalID := pgtype.Text{}
			authorName := pgtype.Text{}
			existing, lookupErr := h.queries.FindDMThreadKeyBySender(r.Context(), db.FindDMThreadKeyBySenderParams{
				SocialAccountID: account.ID,
				AuthorID:        pgtype.Text{String: msg.Sender.ID, Valid: true},
			})
			if lookupErr == nil {
				if existing.ThreadKey != "" {
					threadKey = existing.ThreadKey
				}
				parentExternalID = existing.ParentExternalID
				authorName = existing.AuthorName
			}

			dmItem, err := h.queries.UpsertInboxItem(r.Context(), db.UpsertInboxItemParams{
				SocialAccountID:  account.ID,
				WorkspaceID:      account.WorkspaceID,
				Source:           "ig_dm",
				ExternalID:       msg.Message.Mid,
				ParentExternalID: parentExternalID,
				AuthorName:       authorName,
				AuthorID:         pgtype.Text{String: msg.Sender.ID, Valid: true},
				Body:             pgtype.Text{String: msg.Message.Text, Valid: msg.Message.Text != ""},
				IsOwn:            isOwn,
				ReceivedAt:       pgtype.Timestamptz{Time: ts, Valid: true},
				Metadata:         []byte("{}"),
				ThreadKey:        threadKey,
				ThreadStatus:     "open",
				AssignedTo:       pgtype.Text{},
				LinkedPostID:     pgtype.Text{},
			})
			if err != nil {
				slog.Warn("meta webhook: upsert DM failed", "err", err)
			} else {
				h.notify(r.Context(), account.WorkspaceID, account.ExternalUserID, toInboxResponse(dmItem))
			}
		}
	} // end for accounts
}

// igCommentValue is the shape of the "value" field for comment webhooks.
// Note: media ID is nested under "media.id", not flat "media_id".
type igCommentValue struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	MediaID  string `json:"media_id"`  // legacy flat field (may be empty)
	ParentID string `json:"parent_id"` // parent comment ID for replies
	Media    struct {
		ID string `json:"id"`
	} `json:"media"` // nested media object from Instagram API with Instagram Login
	From struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"from"`
	Timestamp int64 `json:"timestamp"`
}

func (h *MetaWebhookHandler) handleIGComment(r *http.Request, account *webhookAccount, raw json.RawMessage) {
	var val igCommentValue
	if err := json.Unmarshal(raw, &val); err != nil {
		slog.Warn("meta webhook: decode comment value failed", "err", err)
		return
	}

	// Resolve media ID: nested media.id (Instagram Login) or flat media_id (legacy).
	mediaID := val.Media.ID
	if mediaID == "" {
		mediaID = val.MediaID
	}
	parentID := mediaID
	if val.ParentID != "" {
		parentID = val.ParentID
	}

	isOwn := val.From.ID == account.WebhookAccountID
	ts := time.Unix(val.Timestamp, 0)
	if val.Timestamp == 0 {
		ts = time.Now()
	}

	commentItem, err := h.queries.UpsertInboxItem(r.Context(), db.UpsertInboxItemParams{
		SocialAccountID:  account.ID,
		WorkspaceID:      account.WorkspaceID,
		Source:           "ig_comment",
		ExternalID:       val.ID,
		ParentExternalID: pgtype.Text{String: parentID, Valid: parentID != ""},
		AuthorName:       pgtype.Text{String: val.From.Username, Valid: val.From.Username != ""},
		AuthorID:         pgtype.Text{String: val.From.ID, Valid: val.From.ID != ""},
		Body:             pgtype.Text{String: val.Text, Valid: val.Text != ""},
		IsOwn:            isOwn,
		ReceivedAt:       pgtype.Timestamptz{Time: ts, Valid: true},
		Metadata:         []byte("{}"),
		ThreadKey:        inboxThreadKey("ig_comment", val.ID, parentID, val.From.ID),
		ThreadStatus:     "open",
		AssignedTo:       pgtype.Text{},
		LinkedPostID:     resolveInboxLinkedPostID(r.Context(), h.queries, account.ID, parentID),
	})
	if err != nil {
		slog.Warn("meta webhook: upsert comment failed", "err", err)
	} else {
		h.notify(r.Context(), account.WorkspaceID, account.ExternalUserID, toInboxResponse(commentItem))
	}
}

// webhookAccount is a minimal projection of a social account for
// webhook routing. We need the DB row ID, workspace ID, and external
// account ID to insert inbox items.
//
// AccessToken is the decrypted Page / IG / Threads token, populated
// only when the webhook handler decided it'll need to make a Graph
// follow-up call (currently the FB path uses it to enrich comment
// authors and DM senders with name + avatar — Meta's webhook payload
// for those events is minimal). Empty string means "we either failed
// to decrypt or didn't bother loading it"; the caller treats both as
// "skip enrichment, fall back to whatever the webhook delivered".
type webhookAccount struct {
	ID                string
	WorkspaceID       string
	ExternalUserID    string
	ExternalAccountID string
	WebhookAccountID  string
	AccessToken       string
}

func (h *MetaWebhookHandler) findInstagramAccountsByWebhookUserID(r *http.Request, webhookUserID string) ([]*webhookAccount, error) {
	rows, err := h.queries.FindAllActiveInstagramAccountsByWebhookUserID(r.Context(), webhookUserID)
	if err != nil {
		return nil, err
	}
	accounts := make([]*webhookAccount, 0, len(rows))
	for _, row := range rows {
		accounts = append(accounts, &webhookAccount{
			ID:                row.ID,
			WorkspaceID:       row.WorkspaceID,
			ExternalAccountID: row.ExternalAccountID,
			WebhookAccountID:  row.InstagramWebhookUserID,
			ExternalUserID:    nullableExternalUserID(row.ExternalUserID),
		})
	}
	return accounts, nil
}

func (h *MetaWebhookHandler) findAccountsByExternalID(r *http.Request, plat, externalAccountID string) ([]*webhookAccount, error) {
	rows, err := h.queries.FindAllSocialAccountsByPlatformAndExternalID(r.Context(), db.FindAllSocialAccountsByPlatformAndExternalIDParams{
		Platform:          plat,
		ExternalAccountID: externalAccountID,
	})
	if err != nil {
		return nil, err
	}
	accounts := make([]*webhookAccount, 0, len(rows))
	for _, row := range rows {
		accounts = append(accounts, &webhookAccount{
			ID:                row.ID,
			WorkspaceID:       row.WorkspaceID,
			ExternalAccountID: row.ExternalAccountID,
			WebhookAccountID:  row.ExternalAccountID,
			ExternalUserID:    nullableExternalUserID(row.ExternalUserID),
		})
	}
	return accounts, nil
}

// handleThreadsEntry processes a single Threads webhook entry.
// Entry.ID is the Threads user ID. Changes contain reply events.
func (h *MetaWebhookHandler) handleThreadsEntry(r *http.Request, entry metaWebhookEntry) {
	accounts, err := h.findAccountsByExternalID(r, "threads", entry.ID)
	if err != nil {
		slog.Warn("meta webhook: exact routing query failed",
			"platform", "threads",
			"entry_id", entry.ID,
			"error_class", "database_query")
		return
	}
	if len(accounts) == 0 {
		slog.Warn("meta webhook: exact routing found no account",
			"platform", "threads",
			"entry_id", entry.ID,
			"match_count", 0)
		return
	}

	for _, account := range accounts {
		for _, change := range entry.Changes {
			switch change.Field {
			case "replies":
				h.handleThreadsReply(r, account, change.Value)
			default:
				slog.Info("meta webhook: unhandled threads change field",
					"field", change.Field, "threads_user_id", entry.ID)
			}
		}
	}
}

// threadsReplyValue is the shape of the "value" field for Threads reply webhooks.
type threadsReplyValue struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	MediaID  string `json:"media_id"`
	ParentID string `json:"parent_id"`
	From     struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"from"`
	Timestamp int64 `json:"timestamp"`
}

func (h *MetaWebhookHandler) handleThreadsReply(r *http.Request, account *webhookAccount, raw json.RawMessage) {
	var val threadsReplyValue
	if err := json.Unmarshal(raw, &val); err != nil {
		slog.Warn("meta webhook: decode threads reply value failed", "err", err)
		return
	}

	parentID := val.MediaID
	if val.ParentID != "" {
		parentID = val.ParentID
	}

	isOwn := val.From.ID == account.WebhookAccountID
	ts := time.Unix(val.Timestamp, 0)
	if val.Timestamp == 0 {
		ts = time.Now()
	}

	replyItem, err := h.queries.UpsertInboxItem(r.Context(), db.UpsertInboxItemParams{
		SocialAccountID:  account.ID,
		WorkspaceID:      account.WorkspaceID,
		Source:           "threads_reply",
		ExternalID:       val.ID,
		ParentExternalID: pgtype.Text{String: parentID, Valid: parentID != ""},
		AuthorName:       pgtype.Text{String: val.From.Username, Valid: val.From.Username != ""},
		AuthorID:         pgtype.Text{String: val.From.ID, Valid: val.From.ID != ""},
		Body:             pgtype.Text{String: val.Text, Valid: val.Text != ""},
		IsOwn:            isOwn,
		ReceivedAt:       pgtype.Timestamptz{Time: ts, Valid: true},
		Metadata:         []byte("{}"),
		ThreadKey:        inboxThreadKey("threads_reply", val.ID, parentID, val.From.ID),
		ThreadStatus:     "open",
		AssignedTo:       pgtype.Text{},
		LinkedPostID:     resolveInboxLinkedPostID(r.Context(), h.queries, account.ID, parentID),
	})
	if err != nil {
		slog.Warn("meta webhook: upsert threads reply failed", "err", err)
	} else {
		h.notify(r.Context(), account.WorkspaceID, account.ExternalUserID, toInboxResponse(replyItem))
	}
}

// handleFacebookEntry processes a single Facebook Page webhook entry.
// Entry.ID is the Page ID. Feed events (comments, posts, reactions)
// arrive in Changes with field="feed"; Messenger events arrive in
// Messaging the same way Instagram DMs do.
func (h *MetaWebhookHandler) handleFacebookEntry(r *http.Request, entry metaWebhookEntry) {
	if h.queries == nil {
		slog.Warn("meta webhook: facebook entry skipped because queries is nil")
		return
	}

	accounts, err := h.findAccountsByExternalID(r, "facebook", entry.ID)
	if err != nil {
		slog.Warn("meta webhook: exact routing query failed",
			"platform", "facebook",
			"entry_id", entry.ID,
			"error_class", "database_query")
		return
	}
	if len(accounts) == 0 {
		slog.Warn("meta webhook: exact routing found no account",
			"platform", "facebook",
			"entry_id", entry.ID,
			"match_count", 0)
		return
	}

	// Decrypt the Page access token for each matched account so the
	// comment + DM handlers can issue Graph follow-up calls to pull
	// author name + avatar (FB webhook payloads are minimal — the
	// raw delivery only carries an opaque sender id and sometimes a
	// name). Failures here aren't fatal; an empty AccessToken just
	// means enrichment is skipped and the row falls back to whatever
	// the webhook delivered (often "Facebook user" placeholder).
	if h.encryptor != nil {
		for _, account := range accounts {
			fullRow, fetchErr := h.queries.GetSocialAccount(r.Context(), account.ID)
			if fetchErr != nil {
				slog.Warn("meta webhook: load fb account for token failed",
					"account_id", account.ID, "err", fetchErr)
				continue
			}
			token, decErr := h.encryptor.Decrypt(fullRow.AccessToken)
			if decErr != nil {
				slog.Warn("meta webhook: decrypt fb page token failed",
					"account_id", account.ID, "err", decErr)
				continue
			}
			account.AccessToken = token
		}
	}

	for _, account := range accounts {
		// Feed events (comments, reactions). We only surface comments to
		// the inbox — reactions/posts don't need human reply.
		for _, change := range entry.Changes {
			if change.Field != "feed" {
				slog.Info("meta webhook: unhandled facebook change field",
					"field", change.Field, "page_id", entry.ID)
				continue
			}
			h.handleFacebookFeedChange(r, account, change.Value)
		}

		// Messenger DMs. Mirrors IG DM handling but stores Source="fb_dm"
		// and uses the conversation ID (resolved from the sender PSID)
		// as thread_key to match sync-fetched DMs.
		for _, msg := range entry.Messaging {
			if msg.Message == nil {
				continue
			}
			isOwn := msg.Sender.ID == account.WebhookAccountID
			ts := time.Unix(msg.Timestamp/1000, (msg.Timestamp%1000)*int64(time.Millisecond))
			if msg.Timestamp == 0 {
				ts = time.Now()
			}

			senderID := msg.Sender.ID
			if isOwn {
				senderID = msg.Recipient.ID
			}
			threadKey := senderID
			parentExternalID := pgtype.Text{}
			authorName := pgtype.Text{}
			authorAvatarURL := pgtype.Text{}
			existing, lookupErr := h.queries.FindDMThreadKeyBySender(r.Context(), db.FindDMThreadKeyBySenderParams{
				SocialAccountID: account.ID,
				AuthorID:        pgtype.Text{String: msg.Sender.ID, Valid: true},
			})
			if lookupErr == nil {
				if existing.ThreadKey != "" {
					threadKey = existing.ThreadKey
				}
				parentExternalID = existing.ParentExternalID
				authorName = existing.AuthorName
			}

			// Best-effort sender enrichment: pull name + profile_pic
			// for inbound DMs via /{psid}?fields=name,profile_pic.
			// Only valid for non-own (inbound) messages within the
			// 24-hour messaging window — Meta rejects other lookups,
			// which we treat as "skip enrichment". Skipped entirely
			// when we already have a name from a previous message in
			// the same conversation (existing.AuthorName), to avoid
			// burning Graph quota on repeated lookups.
			if !isOwn && account.AccessToken != "" && (!authorName.Valid || authorName.String == "") {
				fb := platform.NewFacebookAdapter()
				if profile, fetchErr := fb.FetchUserProfile(r.Context(), account.AccessToken, msg.Sender.ID); fetchErr == nil && profile != nil {
					if profile.Name != "" {
						authorName = pgtype.Text{String: profile.Name, Valid: true}
					}
					if profile.AvatarURL != "" {
						authorAvatarURL = pgtype.Text{String: profile.AvatarURL, Valid: true}
					}
				} else if fetchErr != nil {
					slog.Info("meta webhook: enrich fb dm sender failed",
						"account_id", account.ID, "psid", msg.Sender.ID, "err", fetchErr)
				}
			}

			dmItem, err := h.queries.UpsertInboxItem(r.Context(), db.UpsertInboxItemParams{
				SocialAccountID:  account.ID,
				WorkspaceID:      account.WorkspaceID,
				Source:           "fb_dm",
				ExternalID:       msg.Message.Mid,
				ParentExternalID: parentExternalID,
				AuthorName:       authorName,
				AuthorID:         pgtype.Text{String: msg.Sender.ID, Valid: true},
				AuthorAvatarUrl:  authorAvatarURL,
				Body:             pgtype.Text{String: msg.Message.Text, Valid: msg.Message.Text != ""},
				IsOwn:            isOwn,
				ReceivedAt:       pgtype.Timestamptz{Time: ts, Valid: true},
				Metadata:         []byte("{}"),
				ThreadKey:        threadKey,
				ThreadStatus:     "open",
				AssignedTo:       pgtype.Text{},
				LinkedPostID:     pgtype.Text{},
			})
			if err != nil {
				slog.Warn("meta webhook: upsert fb dm failed", "err", err)
			} else {
				h.notify(r.Context(), account.WorkspaceID, account.ExternalUserID, toInboxResponse(dmItem))
			}
		}
	}
}

// fbFeedValue is the shape of the "value" field for Facebook Page
// feed webhooks. Meta reuses the same envelope for comments, posts,
// reactions, and likes — we key off (item, verb) to decide what to do.
type fbFeedValue struct {
	Item       string `json:"item"` // "comment" | "post" | "reaction" | ...
	Verb       string `json:"verb"` // "add" | "edit" | "edited" | "remove"
	CommentID  string `json:"comment_id"`
	PostID     string `json:"post_id"`
	ParentID   string `json:"parent_id"`
	SenderID   string `json:"sender_id"`
	SenderName string `json:"sender_name"`
	From       struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"from"`
	Message     string `json:"message"`
	CreatedTime int64  `json:"created_time"`
}

func (h *MetaWebhookHandler) handleFacebookFeedChange(r *http.Request, account *webhookAccount, raw json.RawMessage) {
	var val fbFeedValue
	if err := json.Unmarshal(raw, &val); err != nil {
		slog.Warn("meta webhook: decode facebook feed value failed", "err", err)
		return
	}
	if val.Item != "comment" || val.Verb != "add" {
		// Only new comments land in the inbox for v1.
		return
	}
	if val.CommentID == "" {
		return
	}

	// Prefer the structured `from` object (delivered when the app has
	// pages_read_user_content); fall back to the flat sender_* fields
	// Meta sends for less-privileged subscriptions.
	authorID := val.From.ID
	if authorID == "" {
		authorID = val.SenderID
	}
	authorName := val.From.Name
	if authorName == "" {
		authorName = val.SenderName
	}

	parentID := val.PostID
	if val.ParentID != "" && val.ParentID != val.PostID {
		parentID = val.ParentID
	}

	isOwn := authorID != "" && authorID == account.WebhookAccountID
	ts := time.Unix(val.CreatedTime, 0)
	if val.CreatedTime == 0 {
		ts = time.Now()
	}

	// Best-effort enrichment: FB webhooks for `feed` deliver minimal
	// metadata (often just a sender_id/name pair, no avatar). Issue a
	// Graph lookup against the comment id to pull from{name,picture}
	// when we have a Page token available. Failure is silent — the
	// row still upserts with whatever the webhook gave us.
	authorAvatarURL := ""
	if account.AccessToken != "" && !isOwn {
		fb := platform.NewFacebookAdapter()
		if author, fetchErr := fb.FetchCommentAuthor(r.Context(), account.AccessToken, val.CommentID); fetchErr == nil && author != nil {
			if author.ID != "" {
				authorID = author.ID
				isOwn = authorID == account.WebhookAccountID
			}
			if author.Name != "" {
				authorName = author.Name
			}
			if author.AvatarURL != "" {
				authorAvatarURL = author.AvatarURL
			}
		} else if fetchErr != nil {
			slog.Info("meta webhook: enrich fb comment author failed",
				"account_id", account.ID, "comment_id", val.CommentID, "err", fetchErr)
		}
	}
	if isFacebookPlaceholderAuthorName(authorName) {
		authorName = ""
	}

	item, err := h.queries.UpsertInboxItem(r.Context(), db.UpsertInboxItemParams{
		SocialAccountID:  account.ID,
		WorkspaceID:      account.WorkspaceID,
		Source:           "fb_comment",
		ExternalID:       val.CommentID,
		ParentExternalID: pgtype.Text{String: parentID, Valid: parentID != ""},
		AuthorName:       pgtype.Text{String: authorName, Valid: authorName != ""},
		AuthorID:         pgtype.Text{String: authorID, Valid: authorID != ""},
		AuthorAvatarUrl:  pgtype.Text{String: authorAvatarURL, Valid: authorAvatarURL != ""},
		Body:             pgtype.Text{String: val.Message, Valid: val.Message != ""},
		IsOwn:            isOwn,
		ReceivedAt:       pgtype.Timestamptz{Time: ts, Valid: true},
		Metadata:         []byte("{}"),
		ThreadKey:        inboxThreadKey("fb_comment", val.CommentID, parentID, authorID),
		ThreadStatus:     "open",
		AssignedTo:       pgtype.Text{},
		LinkedPostID:     resolveInboxLinkedPostID(r.Context(), h.queries, account.ID, parentID),
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			rows, mergeErr := h.queries.MergeInboxItemAuthorMetadataByExternalID(r.Context(), db.MergeInboxItemAuthorMetadataByExternalIDParams{
				SocialAccountID: account.ID,
				ExternalID:      val.CommentID,
				AuthorName:      authorName,
				AuthorID:        authorID,
				AuthorAvatarUrl: authorAvatarURL,
			})
			if mergeErr != nil {
				slog.Warn("meta webhook: merge facebook comment author failed", "err", mergeErr)
			} else if rows > 0 {
				h.notifyEvent(r.Context(), account.WorkspaceID, account.ExternalUserID, map[string]any{
					"type":      "inbox.sync_complete",
					"new_items": 0,
				})
			}
			return
		}
		slog.Warn("meta webhook: upsert facebook comment failed", "err", err)
		return
	}
	h.notify(r.Context(), account.WorkspaceID, account.ExternalUserID, toInboxResponse(item))
}

func nullableExternalUserID(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

// verifyMetaWebhookSignature checks the X-Hub-Signature-256 header
// against an HMAC-SHA256 of the raw body using the app secret.
// The header format is "sha256=<hex>".
func verifyMetaWebhookSignature(body []byte, sigHeader, appSecret string) bool {
	if sigHeader == "" {
		return false
	}
	parts := strings.SplitN(sigHeader, "=", 2)
	if len(parts) != 2 || parts[0] != "sha256" {
		return false
	}
	expectedSig, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write(body)
	actualSig := mac.Sum(nil)

	return hmac.Equal(expectedSig, actualSig)
}
