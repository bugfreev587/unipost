// inbox.go handles the unified inbox for Instagram comments/DMs
// and Threads replies.
//
// All endpoints are workspace-scoped and use Clerk session auth.
// The inbox is a flat list of items sorted by received_at DESC.

package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

type InboxHandler struct {
	queries   *db.Queries
	encryptor *crypto.AESEncryptor
}

func NewInboxHandler(queries *db.Queries, encryptor *crypto.AESEncryptor) *InboxHandler {
	return &InboxHandler{queries: queries, encryptor: encryptor}
}

// inboxItemResponse is the JSON shape returned to the frontend.
type inboxItemResponse struct {
	ID               string  `json:"id"`
	SocialAccountID  string  `json:"social_account_id"`
	WorkspaceID      string  `json:"workspace_id"`
	Source           string  `json:"source"`
	ExternalID       string  `json:"external_id"`
	ParentExternalID *string `json:"parent_external_id,omitempty"`
	AuthorName       *string `json:"author_name,omitempty"`
	AuthorID         *string `json:"author_id,omitempty"`
	AuthorAvatarURL  *string `json:"author_avatar_url,omitempty"`
	Body             *string `json:"body,omitempty"`
	IsRead           bool    `json:"is_read"`
	IsOwn            bool    `json:"is_own"`
	ReceivedAt       string  `json:"received_at"`
	CreatedAt        string  `json:"created_at"`
	// Joined fields from social_accounts for display.
	AccountName      string  `json:"account_name,omitempty"`
	AccountPlatform  string  `json:"account_platform,omitempty"`
	AccountAvatarURL string  `json:"account_avatar_url,omitempty"`
}

func toInboxResponse(item db.InboxItem) inboxItemResponse {
	r := inboxItemResponse{
		ID:              item.ID,
		SocialAccountID: item.SocialAccountID,
		WorkspaceID:     item.WorkspaceID,
		Source:          item.Source,
		ExternalID:      item.ExternalID,
		IsRead:          item.IsRead,
		IsOwn:           item.IsOwn,
		ReceivedAt:      item.ReceivedAt.Time.Format(time.RFC3339),
		CreatedAt:       item.CreatedAt.Time.Format(time.RFC3339),
	}
	if item.ParentExternalID.Valid {
		r.ParentExternalID = &item.ParentExternalID.String
	}
	if item.AuthorName.Valid {
		r.AuthorName = &item.AuthorName.String
	}
	if item.AuthorID.Valid {
		r.AuthorID = &item.AuthorID.String
	}
	if item.AuthorAvatarUrl.Valid {
		r.AuthorAvatarURL = &item.AuthorAvatarUrl.String
	}
	if item.Body.Valid {
		r.Body = &item.Body.String
	}
	return r
}

// List returns inbox items for a workspace.
// GET /v1/workspaces/{workspaceID}/inbox?source=ig_comment&is_read=false
func (h *InboxHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")

	params := db.ListInboxItemsByWorkspaceParams{
		WorkspaceID: workspaceID,
		Limit:       50,
	}
	if s := r.URL.Query().Get("source"); s != "" {
		params.Source = pgtype.Text{String: s, Valid: true}
	}
	if r.URL.Query().Get("is_read") == "false" {
		params.IsRead = pgtype.Bool{Bool: false, Valid: true}
	} else if r.URL.Query().Get("is_read") == "true" {
		params.IsRead = pgtype.Bool{Bool: true, Valid: true}
	}

	items, err := h.queries.ListInboxItemsByWorkspace(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list inbox items")
		return
	}

	// Build a map of social account info for display.
	accountMap := map[string]db.SocialAccount{}
	for _, item := range items {
		if _, ok := accountMap[item.SocialAccountID]; !ok {
			if acc, err := h.queries.GetSocialAccount(r.Context(), item.SocialAccountID); err == nil {
				accountMap[item.SocialAccountID] = acc
			}
		}
	}

	resp := make([]inboxItemResponse, 0, len(items))
	for _, item := range items {
		r := toInboxResponse(item)
		if acc, ok := accountMap[item.SocialAccountID]; ok {
			r.AccountName = acc.AccountName.String
			r.AccountPlatform = acc.Platform
			if acc.AccountAvatarUrl.Valid {
				r.AccountAvatarURL = acc.AccountAvatarUrl.String
			}
		}
		resp = append(resp, r)
	}

	writeSuccess(w, resp)
}

// UnreadCount returns the unread count for a workspace.
// GET /v1/workspaces/{workspaceID}/inbox/unread-count
func (h *InboxHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")
	count, err := h.queries.CountUnreadByWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to count unread")
		return
	}
	writeSuccess(w, map[string]int32{"count": count})
}

// Get returns a single inbox item.
// GET /v1/workspaces/{workspaceID}/inbox/{id}
func (h *InboxHandler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")
	id := chi.URLParam(r, "id")

	item, err := h.queries.GetInboxItem(r.Context(), db.GetInboxItemParams{
		ID: id, WorkspaceID: workspaceID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Inbox item not found")
		return
	}
	writeSuccess(w, toInboxResponse(item))
}

// MarkRead marks a single inbox item as read.
// POST /v1/workspaces/{workspaceID}/inbox/{id}/read
func (h *InboxHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")
	id := chi.URLParam(r, "id")

	if err := h.queries.MarkInboxItemRead(r.Context(), db.MarkInboxItemReadParams{
		ID: id, WorkspaceID: workspaceID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to mark read")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// MarkAllRead marks all inbox items as read for a workspace.
// POST /v1/workspaces/{workspaceID}/inbox/mark-all-read
func (h *InboxHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")
	count, err := h.queries.MarkAllInboxItemsRead(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to mark all read")
		return
	}
	writeSuccess(w, map[string]int64{"marked": count})
}

// Reply sends a reply to a comment/DM/thread reply.
// POST /v1/workspaces/{workspaceID}/inbox/{id}/reply
// Body: { "text": "..." }
func (h *InboxHandler) Reply(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")
	id := chi.URLParam(r, "id")

	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Text == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "text is required")
		return
	}

	// Load the inbox item.
	item, err := h.queries.GetInboxItem(r.Context(), db.GetInboxItemParams{
		ID: id, WorkspaceID: workspaceID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Inbox item not found")
		return
	}

	// Load the social account and decrypt the token.
	account, err := h.queries.GetSocialAccount(r.Context(), item.SocialAccountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load social account")
		return
	}
	accessToken, err := h.encryptor.Decrypt(account.AccessToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to decrypt token")
		return
	}

	// Dispatch reply based on source.
	var replyResult *platform.PostResult
	switch item.Source {
	case "ig_comment":
		adapter := platform.NewInstagramAdapter()
		replyResult, err = adapter.ReplyToComment(r.Context(), accessToken, item.ExternalID, body.Text)
	case "ig_dm":
		adapter := platform.NewInstagramAdapter()
		if !item.AuthorID.Valid {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Cannot reply: missing author ID")
			return
		}
		replyResult, err = adapter.SendDM(r.Context(), accessToken, item.AuthorID.String, body.Text)
	case "threads_reply":
		adapter := platform.NewThreadsAdapter()
		replyResult, err = adapter.ReplyToComment(r.Context(), accessToken, item.ExternalID, body.Text)
	default:
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Unsupported source for reply")
		return
	}
	if err != nil {
		slog.Error("inbox reply failed", "source", item.Source, "err", err)
		writeError(w, http.StatusBadGateway, "PLATFORM_ERROR", "Reply failed: "+err.Error())
		return
	}

	// Insert the reply as an inbox item so it appears in the thread view.
	parentID := item.ParentExternalID
	if !parentID.Valid {
		parentID = pgtype.Text{String: item.ExternalID, Valid: true}
	}
	replyItem, _ := h.queries.UpsertInboxItem(r.Context(), db.UpsertInboxItemParams{
		SocialAccountID:  item.SocialAccountID,
		WorkspaceID:      workspaceID,
		Source:           item.Source,
		ExternalID:       replyResult.ExternalID,
		ParentExternalID: parentID,
		AuthorName:       pgtype.Text{String: account.AccountName.String, Valid: account.AccountName.Valid},
		AuthorID:         pgtype.Text{String: account.ExternalAccountID, Valid: true},
		Body:             pgtype.Text{String: body.Text, Valid: true},
		IsOwn:            true,
		ReceivedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
		Metadata:         []byte("{}"),
	})

	writeSuccess(w, toInboxResponse(replyItem))
}

// Sync manually fetches comments/replies from all connected IG/Threads accounts.
// POST /v1/workspaces/{workspaceID}/inbox/sync
func (h *InboxHandler) Sync(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")

	accounts, err := h.queries.FindInboxAccountsByWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to find accounts")
		return
	}

	slog.Info("inbox sync starting", "workspace_id", workspaceID, "accounts", len(accounts))

	type syncError struct {
		AccountID string `json:"account_id"`
		Platform  string `json:"platform"`
		Step      string `json:"step"`
		Error     string `json:"error"`
	}

	totalNew := 0
	var errors []syncError
	for _, acc := range accounts {
		accessToken, err := h.encryptor.Decrypt(acc.AccessToken)
		if err != nil {
			slog.Warn("inbox sync: decrypt failed", "account_id", acc.ID, "err", err)
			errors = append(errors, syncError{acc.ID, acc.Platform, "decrypt", err.Error()})
			continue
		}

		slog.Info("inbox sync: processing account",
			"account_id", acc.ID, "platform", acc.Platform)

		switch acc.Platform {
		case "instagram":
			adapter := platform.NewInstagramAdapter()
			// Fetch recent media directly from IG API (covers all posts,
			// not just those published through UniPost).
			mediaIDs, err := adapter.FetchRecentMedia(r.Context(), accessToken)
			if err != nil {
				slog.Warn("inbox sync: fetch ig recent media failed", "account_id", acc.ID, "err", err)
				errors = append(errors, syncError{acc.ID, acc.Platform, "fetch_media", err.Error()})
			} else {
				slog.Info("inbox sync: fetched ig recent media", "account_id", acc.ID, "count", len(mediaIDs))
				for _, mediaID := range mediaIDs {
					entries, err := adapter.FetchComments(r.Context(), accessToken, mediaID)
					if err != nil {
						slog.Warn("inbox sync: fetch ig comments failed",
							"account_id", acc.ID, "media_id", mediaID, "err", err)
						continue
					}
					slog.Info("inbox sync: fetched ig comments",
						"media_id", mediaID, "count", len(entries))
					for _, e := range entries {
						isOwn := e.AuthorID == acc.ExternalAccountID
						_, uErr := h.queries.UpsertInboxItem(r.Context(), db.UpsertInboxItemParams{
							SocialAccountID:  acc.ID,
							WorkspaceID:      workspaceID,
							Source:           e.Source,
							ExternalID:       e.ExternalID,
							ParentExternalID: pgtype.Text{String: e.ParentExternalID, Valid: e.ParentExternalID != ""},
							AuthorName:       pgtype.Text{String: e.AuthorName, Valid: e.AuthorName != ""},
							AuthorID:         pgtype.Text{String: e.AuthorID, Valid: e.AuthorID != ""},
							Body:             pgtype.Text{String: e.Body, Valid: e.Body != ""},
							IsOwn:            isOwn,
							ReceivedAt:       pgtype.Timestamptz{Time: e.Timestamp, Valid: !e.Timestamp.IsZero()},
							Metadata:         []byte("{}"),
						})
						if uErr == nil {
							totalNew++
						}
					}
				}
			}
			// Fetch DMs.
			dmEntries, err := adapter.FetchConversations(r.Context(), accessToken)
			if err != nil {
				slog.Warn("inbox sync: fetch ig DMs failed", "account_id", acc.ID, "err", err)
			} else {
				for _, e := range dmEntries {
					isOwn := e.AuthorID == acc.ExternalAccountID
					_, uErr := h.queries.UpsertInboxItem(r.Context(), db.UpsertInboxItemParams{
						SocialAccountID:  acc.ID,
						WorkspaceID:      workspaceID,
						Source:           e.Source,
						ExternalID:       e.ExternalID,
						ParentExternalID: pgtype.Text{String: e.ParentExternalID, Valid: e.ParentExternalID != ""},
						AuthorName:       pgtype.Text{String: e.AuthorName, Valid: e.AuthorName != ""},
						AuthorID:         pgtype.Text{String: e.AuthorID, Valid: e.AuthorID != ""},
						Body:             pgtype.Text{String: e.Body, Valid: e.Body != ""},
						IsOwn:            isOwn,
						ReceivedAt:       pgtype.Timestamptz{Time: e.Timestamp, Valid: !e.Timestamp.IsZero()},
						Metadata:         []byte("{}"),
					})
					if uErr == nil {
						totalNew++
					}
				}
			}

		case "threads":
			adapter := platform.NewThreadsAdapter()
			// Fetch recent posts directly from Threads API.
			postIDs, err := adapter.FetchRecentMedia(r.Context(), accessToken)
			if err != nil {
				slog.Warn("inbox sync: fetch threads recent media failed", "account_id", acc.ID, "err", err)
				errors = append(errors, syncError{acc.ID, acc.Platform, "fetch_media", err.Error()})
			} else {
				slog.Info("inbox sync: fetched threads recent media", "account_id", acc.ID, "count", len(postIDs))
				for _, postID := range postIDs {
					entries, err := adapter.FetchComments(r.Context(), accessToken, postID)
					if err != nil {
						slog.Warn("inbox sync: fetch threads replies failed",
							"account_id", acc.ID, "post_id", postID, "err", err)
						continue
					}
					slog.Info("inbox sync: fetched threads replies",
						"post_id", postID, "count", len(entries))
					for _, e := range entries {
						isOwn := e.AuthorName == acc.AccountName.String
						_, uErr := h.queries.UpsertInboxItem(r.Context(), db.UpsertInboxItemParams{
							SocialAccountID:  acc.ID,
							WorkspaceID:      workspaceID,
							Source:           e.Source,
							ExternalID:       e.ExternalID,
							ParentExternalID: pgtype.Text{String: e.ParentExternalID, Valid: e.ParentExternalID != ""},
							AuthorName:       pgtype.Text{String: e.AuthorName, Valid: e.AuthorName != ""},
							AuthorID:         pgtype.Text{String: e.AuthorID, Valid: e.AuthorID != ""},
							Body:             pgtype.Text{String: e.Body, Valid: e.Body != ""},
							IsOwn:            isOwn,
							ReceivedAt:       pgtype.Timestamptz{Time: e.Timestamp, Valid: !e.Timestamp.IsZero()},
							Metadata:         []byte("{}"),
						})
						if uErr == nil {
							totalNew++
						}
					}
				}
			}
		}
	}

	slog.Info("inbox sync complete", "new_items", totalNew, "accounts", len(accounts), "errors", len(errors))
	writeSuccess(w, map[string]any{
		"new_items":        totalNew,
		"accounts_checked": len(accounts),
		"errors":           errors,
	})
}
