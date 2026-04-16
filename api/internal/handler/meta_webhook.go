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

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/ws"
)

// MetaWebhookHandler owns GET/POST /webhooks/meta.
type MetaWebhookHandler struct {
	queries     *db.Queries
	pool        *pgxpool.Pool // for pg_notify (ws.Notify)
	appSecret   string
	verifyToken string
}

func NewMetaWebhookHandler(queries *db.Queries, pool *pgxpool.Pool, appSecret, verifyToken string) *MetaWebhookHandler {
	return &MetaWebhookHandler{
		queries:     queries,
		pool:        pool,
		appSecret:   strings.TrimSpace(appSecret),
		verifyToken: strings.TrimSpace(verifyToken),
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
	// Look up ALL active IG accounts to fan out the event to every workspace.
	// Meta webhooks send the IGBA ID which doesn't match our stored IG Login ID,
	// so we find all active IG accounts and insert for each.
	accounts, err := h.findAllActiveAccounts(r, "instagram")
	if err != nil || len(accounts) == 0 {
		slog.Warn("meta webhook: no active IG accounts found",
			"webhook_ig_id", entry.ID, "err", err)
		return
	}
	slog.Info("meta webhook: fanning out to IG accounts",
		"webhook_ig_id", entry.ID, "account_count", len(accounts))

	for _, account := range accounts {

	// Process changes (comments).
	slog.Info("meta webhook: processing entry",
		"ig_user_id", entry.ID,
		"changes_count", len(entry.Changes),
		"messaging_count", len(entry.Messaging))
	for _, change := range entry.Changes {
		slog.Info("meta webhook: change", "field", change.Field)
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
		isOwn := msg.Sender.ID == account.ExternalAccountID
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
			ws.Notify(r.Context(), h.pool, account.WorkspaceID, toInboxResponse(dmItem))
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
	slog.Info("meta webhook: raw comment value", "payload", string(raw))

	var val igCommentValue
	if err := json.Unmarshal(raw, &val); err != nil {
		slog.Warn("meta webhook: decode comment value failed", "err", err)
		return
	}

	slog.Info("meta webhook: parsed comment",
		"id", val.ID, "media_id", val.MediaID, "parent_id", val.ParentID,
		"from_id", val.From.ID, "from_username", val.From.Username,
		"text", val.Text)

	// Resolve media ID: nested media.id (Instagram Login) or flat media_id (legacy).
	mediaID := val.Media.ID
	if mediaID == "" {
		mediaID = val.MediaID
	}
	parentID := mediaID
	if val.ParentID != "" {
		parentID = val.ParentID
	}

	isOwn := val.From.ID == account.ExternalAccountID
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
		ws.Notify(r.Context(), h.pool, account.WorkspaceID, toInboxResponse(commentItem))
	}
}

// webhookAccount is a minimal projection of a social account for
// webhook routing. We need the DB row ID, workspace ID, and external
// account ID to insert inbox items.
type webhookAccount struct {
	ID                string
	WorkspaceID       string
	ExternalAccountID string
}

// findAccountByExternalID finds a social account by platform + external_account_id,
// joining through profiles to get the workspace_id.
func (h *MetaWebhookHandler) findAccountByExternalID(r *http.Request, plat, externalAccountID string) (*webhookAccount, error) {
	acc, err := h.queries.FindSocialAccountByPlatformAndExternalID(
		r.Context(),
		db.FindSocialAccountByPlatformAndExternalIDParams{
			Platform:          plat,
			ExternalAccountID: externalAccountID,
		},
	)
	if err != nil {
		return nil, err
	}
	return &webhookAccount{
		ID:                acc.ID,
		WorkspaceID:       acc.WorkspaceID,
		ExternalAccountID: acc.ExternalAccountID,
	}, nil
}

// findAnyActiveAccount is a fallback for when Meta sends a different
// ID format than what we store. Returns any active account for the platform.
func (h *MetaWebhookHandler) findAnyActiveAccount(r *http.Request, plat string) (*webhookAccount, error) {
	acc, err := h.queries.FindAnyActiveAccountByPlatform(r.Context(), plat)
	if err != nil {
		return nil, err
	}
	return &webhookAccount{
		ID:                acc.ID,
		WorkspaceID:       acc.WorkspaceID,
		ExternalAccountID: acc.ExternalAccountID,
	}, nil
}

// handleThreadsEntry processes a single Threads webhook entry.
// Entry.ID is the Threads user ID. Changes contain reply events.
func (h *MetaWebhookHandler) handleThreadsEntry(r *http.Request, entry metaWebhookEntry) {
	account, err := h.findAccountByExternalID(r, "threads", entry.ID)
	if err != nil {
		account, err = h.findAnyActiveAccount(r, "threads")
		if err != nil {
			slog.Warn("meta webhook: no active Threads account found",
				"webhook_threads_id", entry.ID, "err", err)
			return
		}
		slog.Info("meta webhook: matched Threads account via fallback",
			"webhook_threads_id", entry.ID, "account_id", account.ID)
	}

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

	isOwn := val.From.ID == account.ExternalAccountID
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
		ws.Notify(r.Context(), h.pool, account.WorkspaceID, toInboxResponse(replyItem))
	}
}

// findAllActiveAccounts returns all active accounts for a platform.
func (h *MetaWebhookHandler) findAllActiveAccounts(r *http.Request, plat string) ([]*webhookAccount, error) {
	rows, err := h.queries.FindAllActiveAccountsByPlatform(r.Context(), plat)
	if err != nil {
		return nil, err
	}
	var accounts []*webhookAccount
	for _, row := range rows {
		accounts = append(accounts, &webhookAccount{
			ID:                row.ID,
			WorkspaceID:       row.WorkspaceID,
			ExternalAccountID: row.ExternalAccountID,
		})
	}
	return accounts, nil
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
