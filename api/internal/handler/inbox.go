// inbox.go handles the unified inbox for Instagram comments/DMs
// and Threads replies.
//
// All endpoints are workspace-scoped and use Clerk session auth.
// The inbox is a flat list of items sorted by received_at DESC.

package handler

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/featureflags"
	"github.com/xiaoboyu/unipost-api/internal/inboxaccess"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/ws"
	"github.com/xiaoboyu/unipost-api/internal/xcredits"
	"github.com/xiaoboyu/unipost-api/internal/xinbox"
)

// mediaContextCacheTTL is how long a fetched media context is considered fresh.
// Posts don't change frequently, so 1 hour is a safe window and eliminates the
// per-refresh upstream call that was causing Railway proxy timeouts.
const mediaContextCacheTTL = 1 * time.Hour

// mediaContextFetchTimeout bounds how long we'll wait for graph.instagram.com /
// graph.threads.net before returning an error. Must be less than Railway's
// proxy timeout so our error response (with CORS headers) wins.
const mediaContextFetchTimeout = 15 * time.Second

const (
	defaultInboxListLimit       = int32(50)
	maxInboxListLimit           = int32(500)
	metaDMReplyWindow           = 24 * time.Hour
	defaultXBackfillSafeCredits = int64(250)
	defaultXBackfillMaxItems    = 20
	maxXBackfillItems           = 500
)

func metaDMReplyWindowClosed(item db.InboxItem, now time.Time) bool {
	if item.Source != "ig_dm" && item.Source != "fb_dm" {
		return false
	}
	if item.IsOwn || !item.ReceivedAt.Valid || item.ReceivedAt.Time.After(now) {
		return false
	}
	return now.Sub(item.ReceivedAt.Time) > metaDMReplyWindow
}

func metaDMReplyWindowMessage(source string) string {
	if source == "fb_dm" {
		return "Messenger reply failed because Meta considers this conversation outside the 24-hour reply window. Ask the Facebook user to send a new message, then retry."
	}
	return "Instagram DM reply failed because Meta considers this conversation outside the 24-hour reply window. Ask the Instagram user to send a new message, then retry."
}

func inboxReplyPlatformError(source string, err error) (message string, reconnect bool) {
	message = "Reply failed: " + err.Error()
	if source != "ig_dm" {
		return message, false
	}
	if strings.Contains(err.Error(), "2534022") {
		return metaDMReplyWindowMessage(source), false
	}
	if strings.Contains(err.Error(), "2534014") {
		return "Instagram DM reply failed because Meta could not resolve the recipient for this conversation. Reconnect the Instagram account with messaging permission and retry with an eligible tester or live account.", true
	}
	return message, false
}

type InboxHandler struct {
	queries                     *db.Queries
	encryptor                   *crypto.AESEncryptor
	pool                        *pgxpool.Pool
	xCredits                    xInboxCreditsService
	xIngestion                  *xinbox.IngestionService
	xTokenRefresher             xinbox.TokenRefresher
	xAdapterFactory             func() xInboxBackfillAdapter
	xBackfillConfirmationSecret []byte
	xBackfillSafeCredits        int64
	notifyEvent                 func(context.Context, string, string, map[string]any)
	notifyWorkspaceEvent        func(context.Context, string, map[string]any)
	featureFlags                interface {
		ForWorkspace(context.Context, string, string) (bool, error)
	}
}

func NewInboxHandler(queries *db.Queries, encryptor *crypto.AESEncryptor, pool *pgxpool.Pool) *InboxHandler {
	return &InboxHandler{
		queries:   queries,
		encryptor: encryptor,
		pool:      pool,
		xAdapterFactory: func() xInboxBackfillAdapter {
			return platform.NewTwitterAdapter()
		},
		xBackfillSafeCredits: defaultXBackfillSafeCredits,
		notifyEvent: func(ctx context.Context, workspaceID, externalUserID string, event map[string]any) {
			ws.NotifyEvent(ctx, pool, workspaceID, externalUserID, event)
		},
		notifyWorkspaceEvent: func(ctx context.Context, workspaceID string, event map[string]any) {
			ws.NotifyWorkspaceEvent(ctx, pool, workspaceID, event)
		},
	}
}

type inboxSyncNotificationCounts struct {
	total   int
	managed map[string]int
}

func newInboxSyncNotificationCounts() *inboxSyncNotificationCounts {
	return &inboxSyncNotificationCounts{managed: make(map[string]int)}
}

func (c *inboxSyncNotificationCounts) Record(externalUserID pgtype.Text) {
	c.RecordN(externalUserID, 1)
}

func (c *inboxSyncNotificationCounts) RecordN(externalUserID pgtype.Text, count int) {
	if c == nil || count <= 0 {
		return
	}
	c.total += count
	if externalUserID.Valid && externalUserID.String != "" && strings.TrimSpace(externalUserID.String) == externalUserID.String {
		c.managed[externalUserID.String] += count
	}
}

func (c *inboxSyncNotificationCounts) Total() int {
	if c == nil {
		return 0
	}
	return c.total
}

func (c *inboxSyncNotificationCounts) Managed() map[string]int {
	result := make(map[string]int)
	if c == nil {
		return result
	}
	for externalUserID, count := range c.managed {
		result[externalUserID] = count
	}
	return result
}

func notifyInboxSyncComplete(
	ctx context.Context,
	workspaceID string,
	counts *inboxSyncNotificationCounts,
	notifyManaged func(context.Context, string, string, map[string]any),
	notifyWorkspace func(context.Context, string, map[string]any),
) {
	if counts == nil || counts.total <= 0 {
		return
	}
	owners := make([]string, 0, len(counts.managed))
	for externalUserID := range counts.managed {
		owners = append(owners, externalUserID)
	}
	sort.Strings(owners)
	if notifyManaged != nil {
		for _, externalUserID := range owners {
			notifyManaged(ctx, workspaceID, externalUserID, map[string]any{
				"type":      "inbox.sync_complete",
				"new_items": counts.managed[externalUserID],
			})
		}
	}
	if notifyWorkspace != nil {
		notifyWorkspace(ctx, workspaceID, map[string]any{
			"type":      "inbox.sync_complete",
			"new_items": counts.total,
		})
	}
}

type xInboxCreditsService interface {
	xInboxReplyCredits
	ReverseByIdempotencyKey(context.Context, string, string) error
	Snapshot(context.Context, string, time.Time) (xcredits.Snapshot, error)
	AdmitInbound(context.Context, xcredits.InboundRequest) (xcredits.InboundAdmission, error)
	ReserveExposure(context.Context, xcredits.ExposureReservationRequest) (xcredits.ExposureReservation, error)
	MarkExposureReadStarted(context.Context, string) error
	MarkExposureFinalizePending(context.Context, string, int64, string) error
	FinalizeExposure(context.Context, string, int64) error
	ReleaseExposure(context.Context, string) error
	MarkExposureReleasePending(context.Context, string, string) error
	MarkExposureNeedsReconciliation(context.Context, string, string) error
}

type xInboxBackfillAdapter interface {
	xInboxReplyAdapter
	FetchInboxMentions(context.Context, string, string, time.Time, string, int) (platform.TwitterInboxPage, error)
	FetchInboxDMEvents(context.Context, string, time.Time, string, int) (platform.TwitterInboxPage, error)
}

func (h *InboxHandler) SetXInboxServices(
	credits xInboxCreditsService,
	ingestion *xinbox.IngestionService,
	refresher xinbox.TokenRefresher,
	confirmationSecret []byte,
) *InboxHandler {
	h.xCredits = credits
	h.xIngestion = ingestion
	h.xTokenRefresher = refresher
	h.xBackfillConfirmationSecret = append([]byte(nil), confirmationSecret...)
	return h
}

func (h *InboxHandler) SetXBackfillSafeCredits(limit int64) *InboxHandler {
	if limit > 0 {
		h.xBackfillSafeCredits = limit
	}
	return h
}

func (h *InboxHandler) SetFeatureFlags(flags interface {
	ForWorkspace(context.Context, string, string) (bool, error)
}) *InboxHandler {
	h.featureFlags = flags
	return h
}

func (h *InboxHandler) xDMsAvailable(ctx context.Context, workspaceID string) (bool, error) {
	if h == nil || h.featureFlags == nil {
		return true, nil
	}
	return h.featureFlags.ForWorkspace(ctx, workspaceID, featureflags.XDMSV1)
}

func (h *InboxHandler) writeXDMSUnavailable(w http.ResponseWriter) {
	writeError(w, http.StatusForbidden, "FEATURE_NOT_AVAILABLE", "X direct messages are not available yet")
}

func (h *InboxHandler) xDMAvailabilityForRequest(w http.ResponseWriter, r *http.Request, workspaceID string) (bool, bool) {
	available, err := h.xDMsAvailable(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to evaluate X DM availability")
		return false, false
	}
	return available, true
}

func inboxListLimit(r *http.Request) int32 {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return defaultInboxListLimit
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultInboxListLimit
	}
	if n > int(maxInboxListLimit) {
		return maxInboxListLimit
	}
	return int32(n)
}

func inboxQueryScope(ctx context.Context) (bool, string) {
	scope, ok := inboxaccess.FromContext(ctx)
	if !ok {
		return false, ""
	}
	return scope.WorkspaceWide(), scope.ExternalUserID
}

func validInboxAccessScope(scope inboxaccess.Scope) bool {
	if strings.TrimSpace(scope.WorkspaceID) == "" {
		return false
	}
	switch scope.Mode {
	case inboxaccess.ModeWorkspace:
		return strings.TrimSpace(scope.ExternalUserID) == ""
	case inboxaccess.ModeManagedUser:
		return strings.TrimSpace(scope.ExternalUserID) != ""
	default:
		return false
	}
}

func logInboxScopeObjectRejected(ctx context.Context, workspaceID, routeClass string) {
	scopeMode := "missing"
	if scope, ok := inboxaccess.FromContext(ctx); ok {
		scopeMode = string(scope.Mode)
	}
	slog.Warn("inbox_scope_object_rejected",
		"workspace_id", workspaceID,
		"route_class", routeClass,
		"scope_mode", scopeMode,
	)
}

// inboxItemResponse is the JSON shape returned to the frontend.
type inboxItemResponse struct {
	ID               string  `json:"id"`
	SocialAccountID  string  `json:"social_account_id"`
	WorkspaceID      string  `json:"workspace_id"`
	Source           string  `json:"source"`
	ExternalID       string  `json:"external_id"`
	ThreadKey        string  `json:"thread_key"`
	ThreadStatus     string  `json:"thread_status"`
	ParentExternalID *string `json:"parent_external_id,omitempty"`
	AssignedTo       *string `json:"assigned_to,omitempty"`
	LinkedPostID     *string `json:"linked_post_id,omitempty"`
	AuthorName       *string `json:"author_name,omitempty"`
	AuthorID         *string `json:"author_id,omitempty"`
	AuthorAvatarURL  *string `json:"author_avatar_url,omitempty"`
	Body             *string `json:"body,omitempty"`
	IsRead           bool    `json:"is_read"`
	IsOwn            bool    `json:"is_own"`
	ReceivedAt       string  `json:"received_at"`
	CreatedAt        string  `json:"created_at"`
	// Joined fields from social_accounts for display.
	AccountName        string  `json:"account_name,omitempty"`
	AccountPlatform    string  `json:"account_platform,omitempty"`
	AccountAvatarURL   string  `json:"account_avatar_url,omitempty"`
	XCreditsCounted    *int64  `json:"x_credits_counted,omitempty"`
	XCreditOperation   *string `json:"x_credit_operation,omitempty"`
	XCreditCatalog     *string `json:"x_credit_catalog_version,omitempty"`
	XCreditBillingMode *string `json:"x_credit_billing_mode,omitempty"`
	URL                *string `json:"url,omitempty"`
}

func toInboxResponse(item db.InboxItem) inboxItemResponse {
	r := inboxItemResponse{
		ID:              item.ID,
		SocialAccountID: item.SocialAccountID,
		WorkspaceID:     item.WorkspaceID,
		Source:          item.Source,
		ExternalID:      item.ExternalID,
		ThreadKey:       item.ThreadKey,
		ThreadStatus:    item.ThreadStatus,
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
	if item.AssignedTo.Valid {
		r.AssignedTo = &item.AssignedTo.String
	}
	if item.LinkedPostID.Valid {
		r.LinkedPostID = &item.LinkedPostID.String
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
	if item.Source == "x_reply" || item.Source == "x_dm" {
		applyXReplyMetadata(&r, item.Metadata)
	}
	return r
}

// List returns inbox items for a workspace.
// GET /v1/inbox?source=ig_comment&is_read=false
func (h *InboxHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	workspaceScope, externalUserID := inboxQueryScope(r.Context())
	dmsAvailable, ok := h.xDMAvailabilityForRequest(w, r, workspaceID)
	if !ok {
		return
	}
	if r.URL.Query().Get("source") == "x_dm" && !dmsAvailable {
		h.writeXDMSUnavailable(w)
		return
	}

	params := db.ListInboxItemsByWorkspaceParams{
		WorkspaceID:    workspaceID,
		Limit:          inboxListLimit(r),
		WorkspaceScope: workspaceScope,
		ExternalUserID: externalUserID,
		ExcludeXDms:    !dmsAvailable,
	}
	if s := r.URL.Query().Get("source"); s != "" {
		params.Source = pgtype.Text{String: s, Valid: true}
	}
	if r.URL.Query().Get("is_read") == "false" {
		params.IsRead = pgtype.Bool{Bool: false, Valid: true}
	} else if r.URL.Query().Get("is_read") == "true" {
		params.IsRead = pgtype.Bool{Bool: true, Valid: true}
	}
	if r.URL.Query().Get("is_own") == "false" {
		params.IsOwn = pgtype.Bool{Bool: false, Valid: true}
	} else if r.URL.Query().Get("is_own") == "true" {
		params.IsOwn = pgtype.Bool{Bool: true, Valid: true}
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

	// Best-effort Facebook author enrichment. Some Page comment webhooks
	// arrive without sender metadata, which leaves the UI showing
	// "unknown". When we have a Page token and the item is missing author
	// fields, fetch the comment's `from` block and persist it. If the
	// comment-object lookup is denied but we still know the parent post,
	// fall back to reading the post's comments edge and matching the row.
	decryptedTokens := map[string]string{}
	fbAdapter := platform.NewFacebookAdapter()
	for idx := range items {
		item := &items[idx]
		if item.Source != "fb_comment" || item.IsOwn {
			continue
		}
		if item.AuthorName.Valid && item.AuthorName.String != "" && item.AuthorAvatarUrl.Valid && item.AuthorAvatarUrl.String != "" {
			continue
		}
		account, ok := accountMap[item.SocialAccountID]
		if !ok {
			continue
		}
		accessToken, ok := decryptedTokens[item.SocialAccountID]
		if !ok {
			decrypted, decErr := h.encryptor.Decrypt(account.AccessToken)
			if decErr != nil || decrypted == "" {
				continue
			}
			accessToken = decrypted
			decryptedTokens[item.SocialAccountID] = decrypted
		}
		author, fetchErr := fbAdapter.FetchCommentAuthor(r.Context(), accessToken, item.ExternalID)
		if (fetchErr != nil || author == nil || author.Name == "" || isFacebookPlaceholderAuthorName(author.Name)) && item.ParentExternalID.Valid && item.ParentExternalID.String != "" {
			parentID := item.ParentExternalID.String
			parentCandidates := []string{parentID}
			if resolved := fbAdapter.ResolvePostID(account.ExternalAccountID, parentID); resolved != parentID {
				parentCandidates = append(parentCandidates, resolved)
			}
			for _, parentCandidate := range parentCandidates {
				entries, commentsErr := fbAdapter.FetchComments(r.Context(), accessToken, parentCandidate)
				if commentsErr != nil {
					continue
				}
				for _, entry := range entries {
					if entry.ExternalID == item.ExternalID {
						author = &platform.FacebookCommentAuthor{
							ID:        entry.AuthorID,
							Name:      entry.AuthorName,
							AvatarURL: entry.AuthorAvatarURL,
						}
						break
					}
				}
				if author != nil && author.Name != "" && !isFacebookPlaceholderAuthorName(author.Name) {
					break
				}
			}
		}
		if author == nil {
			continue
		}
		if author.Name != "" && !isFacebookPlaceholderAuthorName(author.Name) {
			item.AuthorName = pgtype.Text{String: author.Name, Valid: true}
		}
		if author.ID != "" {
			item.AuthorID = pgtype.Text{String: author.ID, Valid: true}
		}
		if author.AvatarURL != "" {
			item.AuthorAvatarUrl = pgtype.Text{String: author.AvatarURL, Valid: true}
		}
		_, _ = h.queries.MergeInboxItemAuthorMetadataByExternalID(r.Context(), db.MergeInboxItemAuthorMetadataByExternalIDParams{
			SocialAccountID: item.SocialAccountID,
			ExternalID:      item.ExternalID,
			AuthorName:      author.Name,
			AuthorID:        author.ID,
			AuthorAvatarUrl: author.AvatarURL,
		})
	}

	resp := make([]inboxItemResponse, 0, len(items))
	for _, item := range items {
		if item.Source == "x_dm" && !dmsAvailable {
			continue
		}
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
// GET /v1/inbox/unread-count
func (h *InboxHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	workspaceScope, externalUserID := inboxQueryScope(r.Context())
	dmsAvailable, ok := h.xDMAvailabilityForRequest(w, r, workspaceID)
	if !ok {
		return
	}
	count, err := h.queries.CountUnreadByWorkspace(r.Context(), db.CountUnreadByWorkspaceParams{
		WorkspaceID:    workspaceID,
		WorkspaceScope: workspaceScope,
		ExternalUserID: externalUserID,
		ExcludeXDms:    !dmsAvailable,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to count unread")
		return
	}
	writeSuccess(w, map[string]int32{"count": count})
}

// Get returns a single inbox item.
// GET /v1/inbox/{id}
func (h *InboxHandler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	workspaceScope, externalUserID := inboxQueryScope(r.Context())
	id := chi.URLParam(r, "id")

	item, err := h.queries.GetInboxItem(r.Context(), db.GetInboxItemParams{
		ID: id, WorkspaceID: workspaceID, WorkspaceScope: workspaceScope, ExternalUserID: externalUserID,
	})
	if err != nil {
		logInboxScopeObjectRejected(r.Context(), workspaceID, "get")
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Inbox item not found")
		return
	}
	if item.Source == "x_dm" {
		available, ok := h.xDMAvailabilityForRequest(w, r, workspaceID)
		if !ok {
			return
		}
		if !available {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Inbox item not found")
			return
		}
	}
	writeSuccess(w, toInboxResponse(item))
}

// MarkRead marks a single inbox item as read.
// POST /v1/inbox/{id}/read
func (h *InboxHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	workspaceScope, externalUserID := inboxQueryScope(r.Context())
	id := chi.URLParam(r, "id")

	item, err := h.queries.GetInboxItem(r.Context(), db.GetInboxItemParams{
		ID: id, WorkspaceID: workspaceID, WorkspaceScope: workspaceScope, ExternalUserID: externalUserID,
	})
	if err != nil {
		logInboxScopeObjectRejected(r.Context(), workspaceID, "mark_read")
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Inbox item not found")
		return
	}
	if item.Source == "x_dm" {
		available, ok := h.xDMAvailabilityForRequest(w, r, workspaceID)
		if !ok {
			return
		}
		if !available {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Inbox item not found")
			return
		}
	}
	if err := h.queries.MarkInboxItemRead(r.Context(), db.MarkInboxItemReadParams{
		ID: id, WorkspaceID: workspaceID, WorkspaceScope: workspaceScope, ExternalUserID: externalUserID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to mark read")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// MarkAllRead marks all inbox items as read for a workspace.
// POST /v1/inbox/mark-all-read
func (h *InboxHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	workspaceScope, externalUserID := inboxQueryScope(r.Context())
	dmsAvailable, ok := h.xDMAvailabilityForRequest(w, r, workspaceID)
	if !ok {
		return
	}
	count, err := h.queries.MarkAllInboxItemsRead(r.Context(), db.MarkAllInboxItemsReadParams{
		WorkspaceID:    workspaceID,
		WorkspaceScope: workspaceScope,
		ExternalUserID: externalUserID,
		ExcludeXDms:    !dmsAvailable,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to mark all read")
		return
	}
	writeSuccess(w, map[string]int64{"marked": count})
}

// MediaContext returns media details for a comment's parent post.
// GET /v1/inbox/{id}/media-context
func (h *InboxHandler) MediaContext(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	workspaceScope, externalUserID := inboxQueryScope(r.Context())
	id := chi.URLParam(r, "id")

	item, err := h.queries.GetInboxItem(r.Context(), db.GetInboxItemParams{
		ID: id, WorkspaceID: workspaceID, WorkspaceScope: workspaceScope, ExternalUserID: externalUserID,
	})
	if err != nil {
		logInboxScopeObjectRejected(r.Context(), workspaceID, "media_context")
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Inbox item not found")
		return
	}
	if item.Source == "x_dm" {
		available, ok := h.xDMAvailabilityForRequest(w, r, workspaceID)
		if !ok {
			return
		}
		if !available {
			h.writeXDMSUnavailable(w)
			return
		}
	}

	// Resolve the media (post) ID. For comments/replies we already have it
	// in parent_external_id; for orphaned IG comments we need the Graph API.
	mediaID := ""
	if item.ParentExternalID.Valid && item.ParentExternalID.String != "" {
		mediaID = item.ParentExternalID.String
	}

	// Fast path: return a cached media context if it's fresh. This avoids
	// the upstream Graph API call on every dashboard refresh.
	if mediaID != "" {
		if cached, cErr := h.queries.GetInboxMediaCache(r.Context(), db.GetInboxMediaCacheParams{
			SocialAccountID: item.SocialAccountID,
			ExternalID:      mediaID,
		}); cErr == nil && time.Since(cached.FetchedAt.Time) < mediaContextCacheTTL {
			writeSuccess(w, &platform.MediaDetails{
				ID:        mediaID,
				Caption:   cached.Caption,
				MediaURL:  cached.MediaUrl,
				Timestamp: cached.Timestamp,
				MediaType: cached.MediaType,
				Permalink: cached.Permalink,
			})
			return
		}
	}

	account, err := h.queries.GetSocialAccountByIDAndWorkspace(r.Context(), db.GetSocialAccountByIDAndWorkspaceParams{
		ID:          item.SocialAccountID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Account not found")
		return
	}
	accessToken, err := h.encryptor.Decrypt(account.AccessToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Token decrypt failed")
		return
	}

	// Bound upstream calls so we return a proper error (with CORS headers)
	// before Railway's proxy times out and returns a plain 502.
	fetchCtx, cancel := context.WithTimeout(r.Context(), mediaContextFetchTimeout)
	defer cancel()

	if mediaID == "" && item.Source == "ig_comment" {
		// The comment has no parent_external_id — look up which
		// media it belongs to via the comment's own ID.
		igAdapter := platform.NewInstagramAdapter()
		type commentInfo struct {
			Media struct {
				ID string `json:"id"`
			} `json:"media"`
		}
		var info commentInfo
		if infoBytes, fetchErr := igAdapter.FetchRaw(fetchCtx, accessToken,
			"https://graph.instagram.com/v21.0/"+item.ExternalID+"?fields=media{id}"); fetchErr == nil {
			if jsonErr := json.Unmarshal(infoBytes, &info); jsonErr == nil && info.Media.ID != "" {
				mediaID = info.Media.ID
			}
		}
	}

	if mediaID == "" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Could not resolve parent media ID")
		return
	}

	var details *platform.MediaDetails
	switch item.Source {
	case "threads_reply":
		details, err = platform.NewThreadsAdapter().FetchMediaDetails(fetchCtx, accessToken, mediaID)
	case "fb_comment", "fb_dm":
		// Facebook Pages — the parent is a Page post. Using the IG
		// adapter here hits graph.instagram.com with a Page token and
		// 502s the whole request; FB has its own endpoint shape.
		details, err = platform.NewFacebookAdapter().FetchMediaDetails(fetchCtx, accessToken, mediaID)
	default:
		details, err = platform.NewInstagramAdapter().FetchMediaDetails(fetchCtx, accessToken, mediaID)
	}
	if err != nil {
		// Fall back to a stale cache entry if we have one — better to show
		// a slightly outdated image than break the inbox UI.
		if cached, cErr := h.queries.GetInboxMediaCache(r.Context(), db.GetInboxMediaCacheParams{
			SocialAccountID: item.SocialAccountID,
			ExternalID:      mediaID,
		}); cErr == nil {
			slog.Warn("inbox media context: upstream failed, serving stale cache",
				"media_id", mediaID, "err", err)
			writeSuccess(w, &platform.MediaDetails{
				ID:        mediaID,
				Caption:   cached.Caption,
				MediaURL:  cached.MediaUrl,
				Timestamp: cached.Timestamp,
				MediaType: cached.MediaType,
				Permalink: cached.Permalink,
			})
			return
		}
		writeError(w, http.StatusBadGateway, "PLATFORM_ERROR", "Failed to fetch media: "+err.Error())
		return
	}

	// Store in cache. Use a fresh context so a canceled request doesn't
	// skip the write after we already have the data.
	if cacheErr := h.queries.UpsertInboxMediaCache(context.Background(), db.UpsertInboxMediaCacheParams{
		SocialAccountID: item.SocialAccountID,
		ExternalID:      mediaID,
		MediaUrl:        details.MediaURL,
		Caption:         details.Caption,
		Timestamp:       details.Timestamp,
		MediaType:       details.MediaType,
		Permalink:       details.Permalink,
	}); cacheErr != nil {
		slog.Warn("inbox media context: cache write failed", "media_id", mediaID, "err", cacheErr)
	}
	writeSuccess(w, details)
}

// Reply sends a reply to a comment/DM/thread reply.
// POST /v1/inbox/{id}/reply
// Body: { "text": "..." }
func (h *InboxHandler) Reply(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	workspaceScope, externalUserID := inboxQueryScope(r.Context())
	id := chi.URLParam(r, "id")

	// Authorize the target before parsing the payload so callers cannot use
	// validation differences to probe whether another managed user's item exists.
	item, err := h.queries.GetInboxItem(r.Context(), db.GetInboxItemParams{
		ID: id, WorkspaceID: workspaceID, WorkspaceScope: workspaceScope, ExternalUserID: externalUserID,
	})
	if err != nil {
		logInboxScopeObjectRejected(r.Context(), workspaceID, "reply")
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Inbox item not found")
		return
	}

	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Text == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "text is required")
		return
	}
	if item.Source == "x_dm" {
		available, ok := h.xDMAvailabilityForRequest(w, r, workspaceID)
		if !ok {
			return
		}
		if !available {
			h.writeXDMSUnavailable(w)
			return
		}
	}
	if metaDMReplyWindowClosed(item, time.Now()) {
		writeError(w, http.StatusUnprocessableEntity, "PLATFORM_ERROR", metaDMReplyWindowMessage(item.Source))
		return
	}

	// Load the social account and decrypt the token.
	account, err := h.queries.GetSocialAccountByIDAndWorkspace(r.Context(), db.GetSocialAccountByIDAndWorkspaceParams{
		ID:          item.SocialAccountID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load social account")
		return
	}
	accessToken, err := h.encryptor.Decrypt(account.AccessToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to decrypt token")
		return
	}
	if account.Platform == "twitter" {
		if item.Source != "x_reply" && item.Source != "x_dm" {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Inbox source does not match the X account")
			return
		}
		if err := validateXInboxReplyTarget(item); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
			return
		}
		if missingScopes := xInboxReplyMissingScopes(item.Source, account.Scope); len(missingScopes) > 0 {
			writeError(
				w,
				http.StatusConflict,
				"X_RECONNECT_REQUIRED",
				"Reconnect the X account to grant missing scopes: "+strings.Join(missingScopes, ", "),
			)
			return
		}
		idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if idempotencyKey == "" {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Idempotency-Key is required for X Inbox replies")
			return
		}
		encryptedPayload, encryptErr := h.encryptor.Encrypt(body.Text)
		if encryptErr != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to protect X Inbox reply payload")
			return
		}
		payloadHash := xInboxReplyPayloadHash(item, body.Text)
		outboundRequest, claimErr := h.queries.ClaimXInboxOutboundRequest(
			r.Context(),
			db.ClaimXInboxOutboundRequestParams{
				WorkspaceID:      workspaceID,
				SocialAccountID:  account.ID,
				InboxItemID:      item.ID,
				IdempotencyKey:   idempotencyKey,
				PayloadHash:      payloadHash,
				EncryptedPayload: encryptedPayload,
				BodyHash:         pgtype.Text{String: xInboxReplyBodyHash(body.Text), Valid: true},
				ReconciliationDeadline: pgtype.Timestamptz{
					Time: time.Now().UTC().Add(xInboxOutcomeUnknownTimeout), Valid: true,
				},
			},
		)
		if stderrors.Is(claimErr, pgx.ErrNoRows) {
			outboundRequest, claimErr = h.queries.GetXInboxOutboundRequest(
				r.Context(),
				db.GetXInboxOutboundRequestParams{
					WorkspaceID:    workspaceID,
					InboxItemID:    item.ID,
					IdempotencyKey: idempotencyKey,
				},
			)
			if claimErr != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load X Inbox idempotency state")
				return
			}
			w.Header().Set("X-UniPost-Operation-Id", outboundRequest.ID)
			if outboundRequest.PayloadHash != payloadHash {
				writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_CONFLICT", "Idempotency-Key was already used with a different X Inbox reply payload")
				return
			}
			if (outboundRequest.Status == "completed" || outboundRequest.Status == "succeeded") &&
				outboundRequest.ResponseInboxItemID.Valid {
				replayed, replayErr := h.queries.GetInboxItem(r.Context(), db.GetInboxItemParams{
					ID:             outboundRequest.ResponseInboxItemID.String,
					WorkspaceID:    workspaceID,
					WorkspaceScope: workspaceScope,
					ExternalUserID: externalUserID,
				})
				if replayErr != nil {
					writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load the prior X Inbox reply")
					return
				}
				response := toInboxResponse(replayed)
				applyXReplyMetadata(&response, replayed.Metadata)
				replayResult := xInboxResultFromOutbound(outboundRequest)
				replayResult.BillingMode = account.XAppMode.String
				applyXReplyResult(&response, replayResult)
				writeSuccess(w, response)
				return
			}
			if outboundRequest.Status == "needs_reconciliation" {
				writeError(
					w,
					http.StatusConflict,
					"X_WRITE_NEEDS_RECONCILIATION",
					"UniPost cannot safely determine whether X accepted this reply. No automatic resend will occur; manual reconciliation is required.",
				)
				return
			}
			if outboundRequest.Status == "usage_reversal_pending" ||
				outboundRequest.Status == "pending_recovery" {
				writeXInboxUsageReversalPending(w)
				return
			}
			writeXInboxOutcomePending(w)
			return
		}
		if claimErr != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to claim X Inbox idempotency key")
			return
		}
		w.Header().Set("X-UniPost-Operation-Id", outboundRequest.ID)
		accessToken, err = h.refreshXAccessTokenIfNeeded(r.Context(), account, accessToken)
		if err != nil {
			_, _ = h.queries.DeletePendingXInboxOutboundRequest(r.Context(), outboundRequest.ID)
			writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "X account token refresh failed; reconnect the account")
			return
		}
		adapter := h.xAdapterFactory()
		sendResult, sendErr := sendXInboxReplyWithReservation(
			r.Context(),
			adapter,
			h.xCredits,
			workspaceID,
			account,
			item,
			accessToken,
			body.Text,
			idempotencyKey,
			func(reserved xInboxSendResult) error {
				updated, err := h.queries.MarkXInboxOutboundSending(
					r.Context(),
					db.MarkXInboxOutboundSendingParams{
						UsageEventID:  reserved.UsageEventID,
						OperationKey:  reserved.Operation,
						ReservedUnits: reserved.XCreditsCounted,
						ID:            outboundRequest.ID,
					},
				)
				if err != nil {
					return err
				}
				if updated != 1 {
					return stderrors.New("X Inbox outbound reservation state was not persisted")
				}
				return nil
			},
		)
		if stderrors.Is(sendErr, errXInboxIdempotencyReplay) {
			replayed, replayErr := h.queries.GetXInboxReplyByIdempotencyKey(r.Context(), db.GetXInboxReplyByIdempotencyKeyParams{
				WorkspaceID:        workspaceID,
				SocialAccountID:    item.SocialAccountID,
				Source:             item.Source,
				ReplyToInboxItemID: item.ID,
				IdempotencyKey:     idempotencyKey,
			})
			if replayErr != nil {
				writeXInboxOutcomePending(w)
				return
			}
			_ = h.queries.CompleteXInboxOutboundRequest(r.Context(), db.CompleteXInboxOutboundRequestParams{
				ResponseInboxItemID: pgtype.Text{String: replayed.ID, Valid: true},
				ID:                  outboundRequest.ID,
			})
			response := toInboxResponse(replayed)
			applyXReplyMetadata(&response, replayed.Metadata)
			writeSuccess(w, response)
			return
		}
		if sendErr != nil {
			if stderrors.Is(sendErr, ErrXUsageReversalPending) {
				completionCtx, cancel := detachedXInboxCompletionContext(r.Context())
				defer cancel()
				err := retryXInboxStatePersistence(completionCtx, func() error {
					updated, err := h.queries.MarkXInboxOutboundUsageReversalPending(
						completionCtx,
						db.MarkXInboxOutboundUsageReversalPendingParams{
							UsageEventID:  sendResult.UsageEventID,
							OperationKey:  sendResult.Operation,
							ReservedUnits: sendResult.XCreditsCounted,
							LastError:     sendErr.Error(),
							ID:            outboundRequest.ID,
						},
					)
					if err != nil {
						return err
					}
					if updated != 1 {
						return errXInboxStateTransitionConflict
					}
					return nil
				})
				if err != nil {
					slog.Error("persist X Inbox usage reversal state failed", "request_id", outboundRequest.ID, "error", err)
				}
				writeXInboxUsageReversalPending(w)
				return
			}
			if retainXInboxOutboundClaim(sendErr) {
				completionCtx, cancel := detachedXInboxCompletionContext(r.Context())
				defer cancel()
				if err := retryXInboxStatePersistence(completionCtx, func() error {
					updated, err := h.queries.MarkXInboxOutboundUnknown(
						completionCtx,
						db.MarkXInboxOutboundUnknownParams{
							UsageEventID:  sendResult.UsageEventID,
							OperationKey:  sendResult.Operation,
							ReservedUnits: sendResult.XCreditsCounted,
							LastError:     sendErr.Error(),
							ID:            outboundRequest.ID,
						},
					)
					if err != nil {
						return err
					}
					if updated != 1 {
						return errXInboxStateTransitionConflict
					}
					return nil
				}); err != nil {
					slog.Error("persist X Inbox unknown outcome failed", "request_id", outboundRequest.ID, "error", err)
				}
			} else {
				_, _ = h.queries.DeletePendingXInboxOutboundRequest(r.Context(), outboundRequest.ID)
			}
			h.writeXInboxReplyError(w, sendErr)
			return
		}
		completionCtx, cancel := detachedXInboxCompletionContext(r.Context())
		defer cancel()
		if err := retryXInboxStatePersistence(completionCtx, func() error {
			return h.recordXInboxRemoteSuccess(completionCtx, outboundRequest.ID, sendResult)
		}); err != nil {
			slog.Error("persist X Inbox remote outcome failed", "request_id", outboundRequest.ID, "error", err)
			writeError(w, http.StatusAccepted, "X_REMOTE_ACCEPTED_RECONCILING", "X accepted the reply; UniPost is reconciling the local Inbox result")
			return
		}
		replyItem, completedResult, completionErr := h.completeKnownXInboxOutbound(completionCtx, outboundRequest.ID)
		if completionErr != nil {
			_ = h.queries.DeferXInboxOutboundCompletion(completionCtx, db.DeferXInboxOutboundCompletionParams{
				NextAttemptAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(time.Minute), Valid: true},
				LastError:     completionErr.Error(),
				ID:            outboundRequest.ID,
			})
			slog.Error("complete X Inbox remote outcome failed", "request_id", outboundRequest.ID, "error", completionErr)
			writeError(w, http.StatusAccepted, "X_REMOTE_ACCEPTED_RECONCILING", "X accepted the reply; UniPost is reconciling the local Inbox result")
			return
		}
		response := toInboxResponse(replyItem)
		applyXReplyResult(&response, completedResult)
		writeSuccess(w, response)
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
		// Use the author_id from the last inbound message as the
		// recipient IGSID. ResolveDMRecipient can return our own
		// account's legacy IGBA ID (different from /me's ID), so
		// the direct author_id is more reliable.
		recipientID := resolveIGDMRecipientID(r.Context(), h.queries, item, account)
		resolveMethod := "inbox_author"
		if recipientID == "" && item.ParentExternalID.Valid && item.ParentExternalID.String != "" {
			if resolved, resolveErr := adapter.ResolveDMRecipient(r.Context(), accessToken, item.ParentExternalID.String); resolveErr == nil {
				recipientID = resolved
				resolveMethod = "conversation_participant"
			}
		}
		slog.Info("inbox dm reply: resolved recipient",
			"recipient_id", recipientID, "method", resolveMethod,
			"author_id", item.AuthorID.String,
			"parent_external_id", item.ParentExternalID.String,
			"external_account_id", account.ExternalAccountID)
		if recipientID == "" {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Cannot reply: missing DM recipient")
			return
		}
		replyResult, err = adapter.SendDM(r.Context(), accessToken, recipientID, body.Text)
	case "threads_reply":
		adapter := platform.NewThreadsAdapter()
		replyResult, err = adapter.ReplyToComment(r.Context(), accessToken, item.ExternalID, body.Text)
	case "fb_comment":
		// Facebook Page Token is already scoped to the Page, so the
		// reply is authored by the Page itself — no separate recipient
		// resolution like the IG DM path.
		adapter := platform.NewFacebookAdapter()
		replyResult, err = adapter.ReplyToComment(r.Context(), accessToken, item.ExternalID, body.Text)
	case "fb_dm":
		// Mirror the IG DM resolution pattern: prefer the item's
		// author_id when it isn't our own Page (covers the common
		// "reply to their message" flow), else fall back to
		// ResolveDMRecipient against the conversation id. The
		// dashboard already gates the Send button on the 24-hour
		// window, so by the time we get here we expect Meta to
		// accept the message — but we surface any rejection as a
		// normal error below.
		adapter := platform.NewFacebookAdapter()
		recipientID := ""
		if item.AuthorID.Valid && item.AuthorID.String != "" && item.AuthorID.String != account.ExternalAccountID {
			recipientID = item.AuthorID.String
		}
		resolveMethod := "inbox_author"
		if recipientID == "" && item.ParentExternalID.Valid && item.ParentExternalID.String != "" {
			if resolved, resolveErr := adapter.ResolveDMRecipient(r.Context(), accessToken, item.ParentExternalID.String); resolveErr == nil {
				recipientID = resolved
				resolveMethod = "conversation_participant"
			}
		}
		slog.Info("inbox fb dm reply: resolved recipient",
			"recipient_id", recipientID, "method", resolveMethod,
			"author_id", item.AuthorID.String,
			"parent_external_id", item.ParentExternalID.String,
			"external_account_id", account.ExternalAccountID)
		if recipientID == "" {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Cannot reply: missing Messenger recipient")
			return
		}
		replyResult, err = adapter.SendDM(r.Context(), accessToken, recipientID, body.Text)
	default:
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Unsupported source for reply")
		return
	}
	if err != nil {
		slog.Error("inbox reply failed", "source", item.Source, "err", err)
		message, reconnect := inboxReplyPlatformError(item.Source, err)
		if reconnect {
			_, _ = h.queries.MarkSocialAccountReconnectRequired(r.Context(), item.SocialAccountID)
		}
		writeError(w, http.StatusUnprocessableEntity, "PLATFORM_ERROR", message)
		return
	}

	// Insert the reply as an inbox item so it appears in the thread view.
	// For comment-style sources (ig_comment / fb_comment / threads_reply),
	// the reply's parent is the comment being replied to, so the tree
	// renderer on the dashboard nests it one level deeper. DMs keep the
	// existing thread-key-level parent (if any).
	parentID := item.ParentExternalID
	if item.Source == "ig_comment" || item.Source == "fb_comment" || item.Source == "threads_reply" {
		parentID = pgtype.Text{String: item.ExternalID, Valid: true}
	} else if !parentID.Valid {
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
		ThreadKey:        item.ThreadKey,
		ThreadStatus:     item.ThreadStatus,
		AssignedTo:       item.AssignedTo,
		LinkedPostID:     item.LinkedPostID,
	})

	writeSuccess(w, toInboxResponse(replyItem))
}

func (h *InboxHandler) refreshXAccessTokenIfNeeded(
	ctx context.Context,
	account db.SocialAccount,
	accessToken string,
) (string, error) {
	if !account.TokenExpiresAt.Valid ||
		account.TokenExpiresAt.Time.After(time.Now().Add(2*time.Minute)) {
		return accessToken, nil
	}
	if h.xTokenRefresher == nil || !account.RefreshToken.Valid {
		return "", stderrors.New("X token refresh is not configured")
	}
	refreshToken, err := h.encryptor.Decrypt(account.RefreshToken.String)
	if err != nil {
		return "", err
	}
	tokens, err := h.xTokenRefresher.Refresh(ctx, account, refreshToken)
	if err != nil {
		return "", err
	}
	if tokens == nil || strings.TrimSpace(tokens.AccessToken) == "" {
		return "", stderrors.New("X token refresh returned an empty access token")
	}
	encryptedAccess, err := h.encryptor.Encrypt(tokens.AccessToken)
	if err != nil {
		return "", err
	}
	rotatedRefresh := tokens.RefreshToken
	if rotatedRefresh == "" {
		rotatedRefresh = refreshToken
	}
	encryptedRefresh, err := h.encryptor.Encrypt(rotatedRefresh)
	if err != nil {
		return "", err
	}
	if err := h.queries.UpdateSocialAccountTokens(ctx, db.UpdateSocialAccountTokensParams{
		ID:             account.ID,
		AccessToken:    encryptedAccess,
		RefreshToken:   pgtype.Text{String: encryptedRefresh, Valid: true},
		TokenExpiresAt: pgtype.Timestamptz{Time: tokens.ExpiresAt, Valid: !tokens.ExpiresAt.IsZero()},
	}); err != nil {
		return "", err
	}
	return tokens.AccessToken, nil
}

func (h *InboxHandler) writeXInboxReplyError(w http.ResponseWriter, err error) {
	switch {
	case stderrors.Is(err, xcredits.ErrMonthlyLimitExceeded):
		writeError(w, http.StatusPaymentRequired, "X_MONTHLY_USAGE_LIMIT_EXCEEDED", err.Error())
	case stderrors.Is(err, ErrXWriteOutcomePending):
		writeXInboxOutcomePending(w)
	default:
		writeError(w, http.StatusUnprocessableEntity, "PLATFORM_ERROR", "X Inbox reply failed: "+err.Error())
	}
}

// XOutboundStatus returns safe reconciliation state without exposing the
// encrypted reply/DM payload.
func (h *InboxHandler) XOutboundStatus(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	workspaceScope, externalUserID := inboxQueryScope(r.Context())
	requestID := chi.URLParam(r, "requestID")
	outbound, err := h.queries.GetXInboxOutboundRequestByID(r.Context(), requestID)
	if err != nil || outbound.WorkspaceID != workspaceID {
		logInboxScopeObjectRejected(r.Context(), workspaceID, "x_outbound_status")
		writeError(w, http.StatusNotFound, "NOT_FOUND", "X Inbox outbound operation not found")
		return
	}
	target, targetErr := h.queries.GetInboxItem(r.Context(), db.GetInboxItemParams{
		ID:             outbound.InboxItemID,
		WorkspaceID:    workspaceID,
		WorkspaceScope: workspaceScope,
		ExternalUserID: externalUserID,
	})
	if targetErr != nil || target.SocialAccountID != outbound.SocialAccountID {
		logInboxScopeObjectRejected(r.Context(), workspaceID, "x_outbound_status")
		writeError(w, http.StatusNotFound, "NOT_FOUND", "X Inbox outbound operation not found")
		return
	}
	if target.Source == "x_dm" {
		available, ok := h.xDMAvailabilityForRequest(w, r, workspaceID)
		if !ok {
			return
		}
		if !available {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "X Inbox outbound operation not found")
			return
		}
	}
	writeSuccess(w, map[string]any{
		"id":                      outbound.ID,
		"status":                  outbound.Status,
		"completion_attempts":     outbound.CompletionAttempts,
		"reconciliation_deadline": outbound.ReconciliationDeadline,
		"reconciliation_required": outbound.Status == "needs_reconciliation",
		"response_inbox_item_id":  outbound.ResponseInboxItemID,
		"updated_at":              outbound.UpdatedAt,
	})
}

func retainXInboxOutboundClaim(err error) bool {
	return stderrors.Is(err, ErrXWriteOutcomePending)
}

func writeXInboxOutcomePending(w http.ResponseWriter) {
	writeError(
		w,
		http.StatusConflict,
		"X_WRITE_OUTCOME_PENDING",
		"X may have accepted this reply. UniPost retained the idempotency claim and provisional usage for reconciliation; retrying with the same Idempotency-Key will not send again.",
	)
}

func writeXInboxUsageReversalPending(w http.ResponseWriter) {
	writeError(
		w,
		http.StatusConflict,
		"X_USAGE_REVERSAL_PENDING",
		"X rejected this reply, but UniPost is still reversing the provisional X credits. Retry later with the same Idempotency-Key.",
	)
}

func applyXReplyResult(response *inboxItemResponse, result xInboxSendResult) {
	if response == nil {
		return
	}
	counted := result.XCreditsCounted
	response.XCreditsCounted = &counted
	response.XCreditOperation = &result.Operation
	response.XCreditCatalog = &result.CatalogVersion
	response.XCreditBillingMode = &result.BillingMode
	if result.URL != "" {
		response.URL = &result.URL
	}
}

func applyXReplyMetadata(response *inboxItemResponse, raw []byte) {
	if response == nil || len(raw) == 0 {
		return
	}
	var metadata struct {
		XCreditsCounted int64  `json:"x_credits_counted"`
		Operation       string `json:"x_credit_operation"`
		CatalogVersion  string `json:"x_credit_catalog_version"`
		BillingMode     string `json:"x_credit_billing_mode"`
		Permalink       string `json:"permalink"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return
	}
	if metadata.Operation != "" || metadata.CatalogVersion != "" || metadata.BillingMode != "" {
		applyXReplyResult(response, xInboxSendResult{
			XCreditsCounted: metadata.XCreditsCounted,
			Operation:       metadata.Operation,
			CatalogVersion:  metadata.CatalogVersion,
			BillingMode:     metadata.BillingMode,
		})
	}
	if metadata.Permalink != "" {
		response.URL = &metadata.Permalink
	}
}

// UpdateThreadState persists a thread-level status for a conversation.
// POST /v1/inbox/{id}/thread-state
// Body: { "thread_status": "open" | "assigned" | "resolved", "assigned_to": "..." }
func (h *InboxHandler) UpdateThreadState(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	workspaceScope, externalUserID := inboxQueryScope(r.Context())
	id := chi.URLParam(r, "id")

	// Keep object authorization ahead of payload validation to avoid exposing
	// whether an ID belongs to a different managed-user scope.
	item, err := h.queries.GetInboxItem(r.Context(), db.GetInboxItemParams{
		ID: id, WorkspaceID: workspaceID, WorkspaceScope: workspaceScope, ExternalUserID: externalUserID,
	})
	if err != nil {
		logInboxScopeObjectRejected(r.Context(), workspaceID, "thread_state")
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Inbox item not found")
		return
	}

	var body struct {
		ThreadStatus string `json:"thread_status"`
		AssignedTo   string `json:"assigned_to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	switch body.ThreadStatus {
	case "open", "assigned", "resolved":
	default:
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "thread_status must be open, assigned, or resolved")
		return
	}

	if item.Source == "x_dm" {
		available, ok := h.xDMAvailabilityForRequest(w, r, workspaceID)
		if !ok {
			return
		}
		if !available {
			h.writeXDMSUnavailable(w)
			return
		}
	}

	threadKey := item.ThreadKey
	if threadKey == "" {
		threadKey = inboxThreadKey(item.Source, item.ExternalID, item.ParentExternalID.String, item.AuthorID.String)
	}

	_, err = h.queries.UpdateInboxThreadState(r.Context(), db.UpdateInboxThreadStateParams{
		WorkspaceID:     workspaceID,
		SocialAccountID: item.SocialAccountID,
		Source:          item.Source,
		ThreadKey:       threadKey,
		ThreadStatus:    body.ThreadStatus,
		Column6:         body.AssignedTo,
		WorkspaceScope:  workspaceScope,
		ExternalUserID:  externalUserID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update thread state")
		return
	}

	updated, err := h.queries.GetInboxItem(r.Context(), db.GetInboxItemParams{
		ID: id, WorkspaceID: workspaceID, WorkspaceScope: workspaceScope, ExternalUserID: externalUserID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to reload inbox item")
		return
	}

	writeSuccess(w, toInboxResponse(updated))
}

// Sync manually fetches comments/replies from all connected IG/Threads accounts.
// POST /v1/inbox/sync
func (h *InboxHandler) Sync(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	workspaceScope, externalUserID := inboxQueryScope(r.Context())
	var request struct {
		XBackfill *xBackfillRequest `json:"x_backfill"`
	}
	if r.Body != nil {
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil && !stderrors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid Inbox sync request")
			return
		}
	}
	accounts, err := h.queries.FindInboxAccountsByWorkspace(r.Context(), db.FindInboxAccountsByWorkspaceParams{
		WorkspaceID:    workspaceID,
		WorkspaceScope: workspaceScope,
		ExternalUserID: externalUserID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to find accounts")
		return
	}
	if request.XBackfill != nil {
		h.syncXBackfill(w, r, workspaceID, accounts, *request.XBackfill)
		return
	}

	slog.Info("inbox sync starting", "workspace_id", workspaceID, "accounts", len(accounts))

	type syncError struct {
		AccountID string `json:"account_id"`
		Platform  string `json:"platform"`
		Step      string `json:"step"`
		Error     string `json:"error"`
	}
	type syncAccountDetail struct {
		AccountID     string `json:"account_id"`
		Platform      string `json:"platform"`
		AccountName   string `json:"account_name"`
		MediaFound    int    `json:"media_found"`
		CommentsFound int    `json:"comments_found"`
	}

	syncNotifications := newInboxSyncNotificationCounts()
	var errors []syncError
	var details []syncAccountDetail
	for _, acc := range accounts {
		detail := syncAccountDetail{
			AccountID:   acc.ID,
			Platform:    acc.Platform,
			AccountName: acc.AccountName.String,
		}

		accessToken, err := h.encryptor.Decrypt(acc.AccessToken)
		if err != nil {
			slog.Warn("inbox sync: decrypt failed", "account_id", acc.ID, "err", err)
			errors = append(errors, syncError{acc.ID, acc.Platform, "decrypt", err.Error()})
			details = append(details, detail)
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
				detail.MediaFound = len(mediaIDs)
				slog.Info("inbox sync: fetched ig recent media", "account_id", acc.ID, "count", len(mediaIDs))
				commentsFetched := 0
				for _, mediaID := range mediaIDs {
					entries, err := adapter.FetchComments(r.Context(), accessToken, mediaID)
					if err != nil {
						slog.Warn("inbox sync: fetch ig comments failed",
							"account_id", acc.ID, "media_id", mediaID, "err", err)
						errors = append(errors, syncError{acc.ID, acc.Platform, "fetch_comments:" + mediaID, err.Error()})
						continue
					}
					commentsFetched += len(entries)
					slog.Info("inbox sync: fetched ig comments",
						"media_id", mediaID, "count", len(entries))
					for _, e := range entries {
						isOwn := e.AuthorID == acc.ExternalAccountID || (e.AuthorName != "" && e.AuthorName == acc.AccountName.String)
						_, uErr := h.queries.UpsertInboxItem(r.Context(), db.UpsertInboxItemParams{
							SocialAccountID:  acc.ID,
							WorkspaceID:      workspaceID,
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
							LinkedPostID:     resolveInboxLinkedPostID(r.Context(), h.queries, acc.ID, e.ParentExternalID),
						})
						if uErr == nil {
							syncNotifications.Record(acc.ExternalUserID)
						}
					}
				}
				detail.CommentsFound = commentsFetched
			}
			// Fetch DMs.
			dmEntries, err := adapter.FetchConversations(r.Context(), accessToken)
			if err != nil {
				slog.Warn("inbox sync: fetch ig DMs failed", "account_id", acc.ID, "err", err)
			} else {
				// Track (senderID → convID) so we can reconcile webhook-created
				// threads that used senderID as fallback thread_key.
				senderConvMap := map[string]string{}
				for _, e := range dmEntries {
					isOwn := e.AuthorID == acc.ExternalAccountID || (e.AuthorName != "" && e.AuthorName == acc.AccountName.String)
					if !isOwn && e.ParentExternalID != "" {
						senderConvMap[e.AuthorID] = e.ParentExternalID
					}
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
						ReceivedAt:       pgtype.Timestamptz{Time: e.Timestamp, Valid: true},
						Metadata:         []byte("{}"),
						ThreadKey:        inboxThreadKey(e.Source, e.ExternalID, e.ParentExternalID, e.AuthorID),
						ThreadStatus:     "open",
						AssignedTo:       pgtype.Text{},
						LinkedPostID:     resolveInboxLinkedPostID(r.Context(), h.queries, acc.ID, e.ParentExternalID),
					})
					if uErr == nil {
						syncNotifications.Record(acc.ExternalUserID)
					}
				}
				// Reconcile: if webhook created items with thread_key = senderID,
				// update them to use the canonical conversation ID.
				for senderID, convID := range senderConvMap {
					if n, err := h.queries.ReconcileDMThreadKeys(r.Context(), db.ReconcileDMThreadKeysParams{
						SocialAccountID:  acc.ID,
						ThreadKey:        senderID,
						ThreadKey_2:      convID,
						ParentExternalID: pgtype.Text{String: convID, Valid: true},
					}); err == nil && n > 0 {
						slog.Info("inbox sync: reconciled DM thread keys",
							"sender_id", senderID, "conv_id", convID, "updated", n)
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
				detail.MediaFound = len(postIDs)
				slog.Info("inbox sync: fetched threads recent media", "account_id", acc.ID, "count", len(postIDs))
				threadRepliesFetched := 0
				for _, postID := range postIDs {
					entries, err := adapter.FetchComments(r.Context(), accessToken, postID)
					if err != nil {
						slog.Warn("inbox sync: fetch threads replies failed",
							"account_id", acc.ID, "post_id", postID, "err", err)
						errors = append(errors, syncError{acc.ID, acc.Platform, "fetch_replies:" + postID, err.Error()})
						continue
					}
					threadRepliesFetched += len(entries)
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
							AuthorAvatarUrl:  pgtype.Text{String: e.AuthorAvatarURL, Valid: e.AuthorAvatarURL != ""},
							Body:             pgtype.Text{String: e.Body, Valid: e.Body != ""},
							IsOwn:            isOwn,
							ReceivedAt:       pgtype.Timestamptz{Time: e.Timestamp, Valid: true},
							Metadata:         []byte("{}"),
							ThreadKey:        inboxThreadKey(e.Source, e.ExternalID, e.ParentExternalID, e.AuthorID),
							ThreadStatus:     "open",
							AssignedTo:       pgtype.Text{},
							LinkedPostID:     resolveInboxLinkedPostID(r.Context(), h.queries, acc.ID, e.ParentExternalID),
						})
						if uErr == nil {
							syncNotifications.Record(acc.ExternalUserID)
						}
					}
				}
				detail.CommentsFound = threadRepliesFetched
			}

		case "facebook":
			adapter := platform.NewFacebookAdapter()
			postIDs, err := h.queries.ListPublishedExternalIDsForInboxSync(r.Context(), db.ListPublishedExternalIDsForInboxSyncParams{
				SocialAccountID: acc.ID,
				Column2:         30,
			})
			if err != nil {
				slog.Warn("inbox sync: list facebook posts failed", "account_id", acc.ID, "err", err)
				errors = append(errors, syncError{acc.ID, acc.Platform, "list_posts", err.Error()})
			} else {
				detail.MediaFound = len(postIDs)
				facebookCommentsFetched := 0
				for _, postIDText := range postIDs {
					if !postIDText.Valid || postIDText.String == "" {
						continue
					}
					postID := postIDText.String
					// Resolve bare ids first — see worker/inbox_sync.go's
					// FB branch + ResolvePostID's doc for the full
					// rationale. Pure string op, no Graph call.
					canonicalID := adapter.ResolvePostID(acc.ExternalAccountID, postID)
					if canonicalID != postID {
						if cErr := h.queries.CanonicalizeFacebookExternalID(r.Context(), db.CanonicalizeFacebookExternalIDParams{
							SocialAccountID: acc.ID,
							ExternalID:      pgtype.Text{String: postID, Valid: true},
							ExternalID_2:    pgtype.Text{String: canonicalID, Valid: true},
						}); cErr != nil {
							slog.Warn("inbox sync: canonicalize facebook external id failed",
								"account_id", acc.ID, "old_id", postID, "new_id", canonicalID, "err", cErr)
						}
						postID = canonicalID
					}
					entries, err := adapter.FetchComments(r.Context(), accessToken, postID)
					if err != nil {
						// Mirror the worker's not-found handling: when
						// Meta says the post is gone (#100 subcode 33),
						// flip the row's remotely_deleted_at so the
						// next sync skips it entirely. Without this
						// branch the manual Sync button keeps surfacing
						// the deleted post as a sync error forever, even
						// though there's nothing the user can do about
						// it. Not added to the syncError slice — a
						// remotely-deleted row isn't an actionable
						// failure, just bookkeeping.
						if stderrors.Is(err, platform.ErrFacebookPostNotFound) {
							if mErr := h.queries.MarkSocialPostResultRemotelyDeleted(r.Context(), db.MarkSocialPostResultRemotelyDeletedParams{
								SocialAccountID: acc.ID,
								ExternalID:      pgtype.Text{String: postID, Valid: true},
								ErrorMessage:    pgtype.Text{String: "Post was deleted on Facebook; inbox sync stopped tracking it.", Valid: true},
							}); mErr != nil {
								slog.Warn("inbox sync: mark remotely-deleted failed",
									"account_id", acc.ID, "post_id", postID, "err", mErr)
							} else {
								slog.Info("inbox sync: marked facebook post as remotely deleted",
									"account_id", acc.ID, "post_id", postID)
							}
							continue
						}
						slog.Warn("inbox sync: fetch facebook comments failed",
							"account_id", acc.ID, "post_id", postID, "err", err)
						errors = append(errors, syncError{acc.ID, acc.Platform, "fetch_comments:" + postID, err.Error()})
						continue
					}
					facebookCommentsFetched += len(entries)
					slog.Info("inbox sync: fetched facebook comments",
						"account_id", acc.ID, "post_id", postID, "count", len(entries))
					for _, e := range entries {
						isOwn := e.AuthorID != "" && e.AuthorID == acc.ExternalAccountID
						_, uErr := h.queries.UpsertInboxItem(r.Context(), db.UpsertInboxItemParams{
							SocialAccountID:  acc.ID,
							WorkspaceID:      workspaceID,
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
							LinkedPostID:     resolveInboxLinkedPostID(r.Context(), h.queries, acc.ID, e.ParentExternalID),
						})
						if uErr == nil {
							syncNotifications.Record(acc.ExternalUserID)
						}
						mergeInboxItemAuthorMetadata(r.Context(), h.queries, acc.ID, e.ExternalID, e.AuthorName, e.AuthorID, e.AuthorAvatarURL)
					}
				}
				detail.CommentsFound = facebookCommentsFetched
			}

			dmEntries, err := adapter.FetchConversations(r.Context(), accessToken)
			if err != nil {
				slog.Warn("inbox sync: fetch facebook DMs failed", "account_id", acc.ID, "err", err)
				errors = append(errors, syncError{acc.ID, acc.Platform, "fetch_conversations", err.Error()})
			} else {
				for _, e := range dmEntries {
					isOwn := e.AuthorID != "" && e.AuthorID == acc.ExternalAccountID
					_, uErr := h.queries.UpsertInboxItem(r.Context(), db.UpsertInboxItemParams{
						SocialAccountID:  acc.ID,
						WorkspaceID:      workspaceID,
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
						LinkedPostID:     resolveInboxLinkedPostID(r.Context(), h.queries, acc.ID, e.ParentExternalID),
					})
					if uErr == nil {
						syncNotifications.Record(acc.ExternalUserID)
					}
					mergeInboxItemAuthorMetadata(r.Context(), h.queries, acc.ID, e.ExternalID, e.AuthorName, e.AuthorID, e.AuthorAvatarURL)
				}
			}
		}

		details = append(details, detail)
	}

	totalNew := syncNotifications.Total()
	slog.Info("inbox sync complete", "new_items", totalNew, "accounts", len(accounts), "errors", len(errors))

	// Notify all connected WebSocket clients to refresh if new items arrived.
	if totalNew > 0 {
		notifyInboxSyncComplete(r.Context(), workspaceID, syncNotifications, h.notifyEvent, h.notifyWorkspaceEvent)
	}

	writeSuccess(w, map[string]any{
		"new_items":        totalNew,
		"accounts_checked": len(accounts),
		"errors":           errors,
		"details":          details,
	})
}

type xBackfillAccountResult struct {
	AccountID         string   `json:"account_id"`
	Accepted          int      `json:"accepted"`
	Suppressed        int      `json:"suppressed"`
	Duplicates        int      `json:"duplicates"`
	Read              int      `json:"read"`
	StoppedAtBoundary bool     `json:"stopped_at_boundary,omitempty"`
	StopReason        string   `json:"stop_reason,omitempty"`
	MissingScopes     []string `json:"missing_scopes,omitempty"`
}

func normalizeXBackfillRequest(request xBackfillRequest) xBackfillRequest {
	if request.LookbackDays <= 0 {
		request.LookbackDays = 7
	}
	if request.LookbackDays > 30 {
		request.LookbackDays = 30
	}
	if request.MaxItems <= 0 {
		request.MaxItems = defaultXBackfillMaxItems
	}
	if request.MaxItems > maxXBackfillItems {
		request.MaxItems = maxXBackfillItems
	}
	if !request.IncludeReplies && !request.IncludeDMs {
		request.IncludeReplies = true
		request.IncludeDMs = true
	}
	request.AccountID = strings.TrimSpace(request.AccountID)
	request.ConfirmationToken = strings.TrimSpace(request.ConfirmationToken)
	return request
}

func (h *InboxHandler) syncXBackfill(
	w http.ResponseWriter,
	r *http.Request,
	workspaceID string,
	accounts []db.SocialAccount,
	request xBackfillRequest,
) {
	requestedOnlyDMs := request.IncludeDMs && !request.IncludeReplies
	request = normalizeXBackfillRequest(request)
	dmsAvailable, ok := h.xDMAvailabilityForRequest(w, r, workspaceID)
	if !ok {
		return
	}
	if !dmsAvailable {
		if requestedOnlyDMs {
			h.writeXDMSUnavailable(w)
			return
		}
		request.IncludeDMs = false
		request.IncludeReplies = true
	}
	xAccounts := make([]db.SocialAccount, 0, len(accounts))
	estimate := int64(0)
	for _, account := range accounts {
		if account.Platform != "twitter" ||
			(request.AccountID != "" && account.ID != request.AccountID) {
			continue
		}
		mode, modeErr := xinbox.NormalizePersistedAppMode(account.XAppMode.String)
		if modeErr != nil || mode == xinbox.AppModeLegacyUnknown {
			continue
		}
		xAccounts = append(xAccounts, account)
		estimate += estimateXBackfillCredits(account.XAppMode.String, request)
	}
	if len(xAccounts) == 0 && request.ConfirmationToken == "" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No eligible X account found for Inbox backfill")
		return
	}

	confirmationOperationID := ""
	confirmationExecutionOwner := ""
	if request.ConfirmationToken != "" || estimate > h.xBackfillSafeCredits {
		if request.ConfirmationToken == "" {
			operation, token, operationErr := h.createXBackfillConfirmationOperation(
				r.Context(),
				workspaceID,
				xAccounts,
				request,
				estimate,
				time.Now(),
			)
			if operationErr != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", operationErr.Error())
				return
			}
			writeSuccess(w, map[string]any{
				"estimated_x_credits":       estimate,
				"confirmation_required":     true,
				"confirmation_operation_id": operation.ID,
				"confirmation_token":        token,
				"confirmation_expires_at":   operation.ExpiresAt.Format(time.RFC3339),
				"accounts_checked":          len(xAccounts),
				"accepted":                  0,
				"suppressed":                0,
			})
			return
		}
		confirmationScope, scopeOK := inboxaccess.FromContext(r.Context())
		if !scopeOK || !validInboxAccessScope(confirmationScope) || confirmationScope.WorkspaceID != workspaceID {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", errXBackfillConfirmationOutsideScope.Error())
			return
		}
		operation, operationErr := h.beginXBackfillConfirmationOperation(
			r.Context(),
			confirmationScope,
			request.ConfirmationToken,
			time.Now(),
		)
		if operationErr != nil {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", operationErr.Error())
			return
		}
		if operation.Status == "completed" {
			var storedResult any
			if err := json.Unmarshal(operation.Result, &storedResult); err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Stored X backfill result is invalid")
				return
			}
			writeSuccess(w, storedResult)
			return
		}
		if operation.Status == "running" && !operation.StartedByThisCall {
			writeSuccess(w, map[string]any{
				"confirmation_operation_id":  operation.ID,
				"status":                     "in_progress",
				"estimated_x_credits":        operation.EstimatedXCredits,
				"execution_lease_expires_at": operation.ExecutionLease.Format(time.RFC3339),
			})
			return
		}
		if operation.Status != "running" {
			writeError(w, http.StatusConflict, "VALIDATION_ERROR", "X backfill confirmation operation is "+operation.Status)
			return
		}
		requestWithoutToken := request
		requestWithoutToken.ConfirmationToken = ""
		if operation.Request != requestWithoutToken ||
			operation.EstimatedXCredits != estimate ||
			operation.AccountFingerprint != xBackfillAccountFingerprint(xBackfillAccountSnapshots(xAccounts)) {
			_ = h.completeXBackfillConfirmationOperation(
				r.Context(),
				operation.ID,
				operation.ExecutionOwner,
				map[string]any{"status": "failed", "reason": "account_or_request_changed"},
				stderrors.New("X backfill account selection or request changed after confirmation"),
			)
			writeError(w, http.StatusConflict, "VALIDATION_ERROR", "X backfill account selection or request changed after confirmation")
			return
		}
		confirmationOperationID = operation.ID
		confirmationExecutionOwner = operation.ExecutionOwner
	}
	if h.xCredits == nil || h.xIngestion == nil || h.xAdapterFactory == nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "X Inbox sync is not configured")
		return
	}

	now := time.Now().UTC()
	backfillRunID := newXBackfillRunID()
	if confirmationOperationID != "" {
		backfillRunID = confirmationOperationID
	}
	results := make([]xBackfillAccountResult, 0, len(xAccounts))
	totalAccepted, totalSuppressed, totalDuplicates, totalRead := 0, 0, 0, 0
	var beforePaidRead func(context.Context) error
	if confirmationOperationID != "" {
		beforePaidRead = func(ctx context.Context) error {
			return h.renewXBackfillExecutionLease(
				ctx, confirmationOperationID, confirmationExecutionOwner, time.Now().UTC(),
			)
		}
	}
	for _, account := range xAccounts {
		result := xBackfillAccountResult{AccountID: account.ID}
		accessToken, decryptErr := h.encryptor.Decrypt(account.AccessToken)
		if decryptErr != nil {
			result.StopReason = "token_decrypt_failed"
			results = append(results, result)
			continue
		}
		accessToken, refreshErr := h.refreshXAccessTokenIfNeeded(r.Context(), account, accessToken)
		if refreshErr != nil {
			result.StopReason = "reconnect_required"
			results = append(results, result)
			continue
		}
		adapter := h.xAdapterFactory()
		if request.IncludeReplies {
			h.runXBackfillPagesWithLease(
				r.Context(), workspaceID, account, accessToken, adapter, "x_reply",
				request, now, backfillRunID, &result, beforePaidRead,
			)
		}
		if request.IncludeDMs {
			h.runXBackfillPagesWithLease(
				r.Context(), workspaceID, account, accessToken, adapter, "x_dm",
				request, now, backfillRunID, &result, beforePaidRead,
			)
		}
		totalAccepted += result.Accepted
		totalSuppressed += result.Suppressed
		totalDuplicates += result.Duplicates
		totalRead += result.Read
		results = append(results, result)
	}
	response := map[string]any{
		"estimated_x_credits":   estimate,
		"confirmation_required": false,
		"accounts_checked":      len(xAccounts),
		"accepted":              totalAccepted,
		"suppressed":            totalSuppressed,
		"duplicates":            totalDuplicates,
		"read":                  totalRead,
		"details":               results,
	}
	if confirmationOperationID != "" {
		response["confirmation_operation_id"] = confirmationOperationID
		completionCtx, cancel := detachedXInboxCompletionContext(r.Context())
		defer cancel()
		if err := h.completeXBackfillConfirmationOperation(
			completionCtx, confirmationOperationID, confirmationExecutionOwner, response, nil,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to persist X backfill result")
			return
		}
	}
	notifyInboxSyncComplete(
		r.Context(), workspaceID, xBackfillSyncNotificationCounts(xAccounts, results),
		h.notifyEvent, h.notifyWorkspaceEvent,
	)
	writeSuccess(w, response)
}

func xBackfillSyncNotificationCounts(accounts []db.SocialAccount, results []xBackfillAccountResult) *inboxSyncNotificationCounts {
	owners := make(map[string]pgtype.Text, len(accounts))
	for _, account := range accounts {
		owners[account.ID] = account.ExternalUserID
	}
	counts := newInboxSyncNotificationCounts()
	for _, result := range results {
		owner, ok := owners[result.AccountID]
		if !ok {
			continue
		}
		counts.RecordN(owner, result.Accepted)
	}
	return counts
}

func (h *InboxHandler) runXBackfillPages(
	ctx context.Context,
	workspaceID string,
	account db.SocialAccount,
	accessToken string,
	adapter xInboxBackfillAdapter,
	source string,
	request xBackfillRequest,
	now time.Time,
	runID string,
	result *xBackfillAccountResult,
) {
	h.runXBackfillPagesWithLease(
		ctx, workspaceID, account, accessToken, adapter, source,
		request, now, runID, result, nil,
	)
}

func (h *InboxHandler) runXBackfillPagesWithLease(
	ctx context.Context,
	workspaceID string,
	account db.SocialAccount,
	accessToken string,
	adapter xInboxBackfillAdapter,
	source string,
	request xBackfillRequest,
	now time.Time,
	runID string,
	result *xBackfillAccountResult,
	beforePaidRead func(context.Context) error,
) {
	remaining := request.MaxItems
	nextToken := ""
	lookbackDays := request.LookbackDays
	if source == "x_reply" && lookbackDays > 7 {
		lookbackDays = 7
	}
	startTime := now.Add(-time.Duration(lookbackDays) * 24 * time.Hour)
	operation := "post.read"
	minPageSize := 1
	if source == "x_dm" {
		operation = "dm.read"
	} else {
		minPageSize = xMentionsMinimumPageSize
	}
	if missingScopes := xInboxBackfillMissingScopes(source, account.Scope); len(missingScopes) > 0 {
		result.StopReason = "reconnect_required"
		result.MissingScopes = missingScopes
		return
	}
	for remaining > 0 {
		if beforePaidRead != nil {
			if err := beforePaidRead(ctx); err != nil {
				result.StoppedAtBoundary = true
				result.StopReason = "confirmation_execution_lease_lost"
				return
			}
		}
		pageSize := remaining
		if pageSize > 100 {
			pageSize = 100
		}
		if pageSize < minPageSize {
			result.StoppedAtBoundary = true
			result.StopReason = "x_inbound_or_monthly_boundary"
			return
		}
		unitsPerResource := xcredits.OperationWeight(operation) + xcredits.OperationWeight("user.read")
		reservation, err := h.xCredits.ReserveExposure(
			ctx,
			xcredits.ExposureReservationRequest{
				WorkspaceID:        workspaceID,
				SocialAccountID:    account.ID,
				AppMode:            account.XAppMode.String,
				OperationKey:       operation,
				IdempotencyKey:     xBackfillExposureKey(runID, account.ID, source, startTime, nextToken, pageSize),
				RequestedResources: pageSize,
				MinimumResources:   minPageSize,
				UnitsPerResource:   unitsPerResource,
				Now:                now,
			},
		)
		if err != nil {
			result.StoppedAtBoundary = true
			switch {
			case stderrors.Is(err, xcredits.ErrMonthlyLimitExceeded):
				result.StopReason = xcredits.PauseReasonMonthlyAllowance
			case stderrors.Is(err, xcredits.ErrInboundDailyCapExceeded):
				result.StopReason = xcredits.PauseReasonDailyCap
			default:
				result.StopReason = "usage_reservation_failed"
			}
			return
		}
		if reservation.Duplicate {
			result.StoppedAtBoundary = true
			result.StopReason = "duplicate_exposure_reservation"
			return
		}
		if reservation.ID != "" {
			if err := h.persistXExposureReadStarted(ctx, reservation.ID); err != nil {
				if releaseErr := h.xCredits.ReleaseExposure(ctx, reservation.ID); releaseErr != nil {
					_ = h.persistXExposureReleasePending(
						ctx,
						reservation.ID,
						"paid read was not started and reservation release failed: "+releaseErr.Error(),
					)
				}
				result.StoppedAtBoundary = true
				result.StopReason = "usage_reservation_read_start_persist_failed"
				return
			}
		}
		if beforePaidRead != nil {
			if err := beforePaidRead(ctx); err != nil {
				if reservation.ID != "" {
					if releaseErr := h.xCredits.ReleaseExposure(ctx, reservation.ID); releaseErr != nil {
						if markErr := h.persistXExposureReleasePending(
							ctx,
							reservation.ID,
							"confirmation execution lease was lost and reservation release failed: "+releaseErr.Error(),
						); markErr != nil {
							result.StoppedAtBoundary = true
							result.StopReason = "usage_reservation_reconciliation_persist_failed"
							return
						}
						result.StoppedAtBoundary = true
						result.StopReason = "usage_reservation_release_needs_reconciliation"
						return
					}
				}
				result.StoppedAtBoundary = true
				result.StopReason = "confirmation_execution_lease_lost"
				return
			}
		}
		pageSize = reservation.ReservedResources
		var page platform.TwitterInboxPage
		if source == "x_reply" {
			page, err = adapter.FetchInboxMentions(
				ctx,
				accessToken,
				firstNonEmptyString(account.ExternalUserID.String, account.ExternalAccountID),
				startTime,
				nextToken,
				pageSize,
			)
		} else {
			page, err = adapter.FetchInboxDMEvents(
				ctx,
				accessToken,
				startTime,
				nextToken,
				pageSize,
			)
		}
		if err != nil {
			if reservation.ID != "" {
				if xInboxReadOutcomeAmbiguous(err) {
					if markErr := h.persistXExposureNeedsReconciliation(
						ctx, reservation.ID, err.Error(),
					); markErr != nil {
						result.StopReason = "usage_reservation_reconciliation_persist_failed"
						return
					}
				} else {
					if releaseErr := h.xCredits.ReleaseExposure(ctx, reservation.ID); releaseErr != nil {
						if markErr := h.persistXExposureReleasePending(
							ctx,
							reservation.ID,
							"upstream read failed definitively and reservation release failed: "+releaseErr.Error(),
						); markErr != nil {
							result.StopReason = "usage_reservation_reconciliation_persist_failed"
							return
						}
						result.StopReason = "usage_reservation_release_needs_reconciliation"
						return
					}
				}
			}
			result.StopReason = "upstream_read_failed"
			return
		}
		if len(page.Entries) > remaining {
			page.Entries = page.Entries[:remaining]
		}
		result.Read += len(page.Entries)
		remaining -= len(page.Entries)
		for _, entry := range page.Entries {
			ingestion, admissionErr := h.ingestXBackfillEntry(
				ctx,
				workspaceID,
				account,
				entry,
				operation,
				!reservation.Bypassed,
			)
			if admissionErr != nil {
				if reservation.ID != "" {
					actualUnits := int64(len(page.Entries)) * unitsPerResource
					if settleErr := h.settleXExposure(ctx, reservation.ID, actualUnits); settleErr != nil {
						result.StopReason = "usage_reservation_settlement_failed"
						return
					}
				}
				result.StopReason = "usage_admission_failed"
				return
			}
			switch ingestion.Admission.Decision {
			case xcredits.InboundDecisionSuppressedDailyCap,
				xcredits.InboundDecisionSuppressedMonthlyAllowance:
				result.Suppressed++
				result.StoppedAtBoundary = true
				result.StopReason = ingestion.Admission.PauseReason
				if reservation.ID != "" {
					actualUnits := int64(len(page.Entries)) * unitsPerResource
					if settleErr := h.settleXExposure(ctx, reservation.ID, actualUnits); settleErr != nil {
						result.StopReason = "usage_reservation_settlement_failed"
					}
				}
				return
			}
			if ingestion.Admission.Duplicate {
				result.Duplicates++
				continue
			}
			if ingestion.Admission.Accepted && ingestion.Inserted {
				result.Accepted++
			}
		}
		if reservation.ID != "" {
			actualUnits := int64(len(page.Entries)) * unitsPerResource
			if err := h.settleXExposure(ctx, reservation.ID, actualUnits); err != nil {
				result.StopReason = "usage_reservation_settlement_failed"
				return
			}
		}
		if page.HorizonReached {
			return
		}
		nextToken = page.NextToken
		if nextToken == "" || len(page.Entries) == 0 {
			return
		}
	}
}

func (h *InboxHandler) persistXExposureReadStarted(ctx context.Context, reservationID string) error {
	persistCtx, cancel := detachedXInboxCompletionContext(ctx)
	defer cancel()
	return retryXInboxStatePersistence(persistCtx, func() error {
		return h.xCredits.MarkExposureReadStarted(persistCtx, reservationID)
	})
}

func (h *InboxHandler) persistXExposureNeedsReconciliation(
	ctx context.Context,
	reservationID string,
	message string,
) error {
	persistCtx, cancel := detachedXInboxCompletionContext(ctx)
	defer cancel()
	return retryXInboxStatePersistence(persistCtx, func() error {
		return h.xCredits.MarkExposureNeedsReconciliation(persistCtx, reservationID, message)
	})
}

func (h *InboxHandler) settleXExposure(
	ctx context.Context,
	reservationID string,
	actualUnits int64,
) error {
	persistCtx, cancel := detachedXInboxCompletionContext(ctx)
	defer cancel()
	if err := retryXInboxStatePersistence(persistCtx, func() error {
		return h.xCredits.MarkExposureFinalizePending(
			persistCtx, reservationID, actualUnits, "X read completed; settlement is pending",
		)
	}); err != nil {
		return err
	}
	return h.xCredits.FinalizeExposure(persistCtx, reservationID, actualUnits)
}

func (h *InboxHandler) persistXExposureReleasePending(
	ctx context.Context,
	reservationID string,
	message string,
) error {
	persistCtx, cancel := detachedXInboxCompletionContext(ctx)
	defer cancel()
	return retryXInboxStatePersistence(persistCtx, func() error {
		return h.xCredits.MarkExposureReleasePending(persistCtx, reservationID, message)
	})
}

func (h *InboxHandler) ingestXBackfillEntry(
	ctx context.Context,
	workspaceID string,
	account db.SocialAccount,
	entry platform.TwitterInboxEntry,
	operation string,
	exposureReserved bool,
) (xinbox.IngestionResult, error) {
	if entry.Source == "x_dm" {
		available, err := h.xDMsAvailable(ctx, workspaceID)
		if err != nil {
			return xinbox.IngestionResult{}, err
		}
		if !available {
			return xinbox.IngestionResult{}, stderrors.New("X direct messages are not available")
		}
	}
	isOwn := entry.AuthorID != "" &&
		(entry.AuthorID == account.ExternalUserID.String || entry.AuthorID == account.ExternalAccountID)
	metadata := map[string]any{
		"conversation_id": entry.ThreadKey,
		"permalink": func() string {
			if entry.Source == "x_reply" {
				return "https://x.com/i/status/" + entry.ExternalID
			}
			return ""
		}(),
		"reply_eligible": entry.ReplyEligible && !isOwn,
		"backfill":       true,
	}
	linkedPostID := resolveInboxLinkedPostID(ctx, h.queries, account.ID, entry.ParentExternalID)
	mode, err := xinbox.NormalizePersistedAppMode(account.XAppMode.String)
	if err != nil {
		return xinbox.IngestionResult{}, err
	}
	if entry.AuthorID != "" && !exposureReserved {
		userAdmission, admissionErr := h.xCredits.AdmitInbound(ctx, xcredits.InboundRequest{
			WorkspaceID:          workspaceID,
			SocialAccountID:      account.ID,
			AppMode:              account.XAppMode.String,
			OperationKey:         "user.read",
			Source:               "backfill",
			UpstreamResourceType: "x_user",
			UpstreamResourceID:   entry.AuthorID,
			Now:                  time.Now().UTC(),
		})
		if admissionErr != nil {
			return xinbox.IngestionResult{}, admissionErr
		}
		if userAdmission.Decision == xcredits.InboundDecisionSuppressedDailyCap ||
			userAdmission.Decision == xcredits.InboundDecisionSuppressedMonthlyAllowance {
			return xinbox.IngestionResult{
				Admission: xinbox.InboundAdmission{
					Suppressed:  true,
					Duplicate:   userAdmission.Duplicate,
					Decision:    userAdmission.Decision,
					PauseReason: userAdmission.PauseReason,
				},
			}, nil
		}
	}
	ingestionMode := mode
	if exposureReserved {
		ingestionMode = xinbox.AppModeWorkspace
	}
	return h.xIngestion.IngestRecovery(
		ctx,
		xinbox.InboxAccount{
			ID:                account.ID,
			WorkspaceID:       workspaceID,
			ExternalUserID:    account.ExternalUserID.String,
			ExternalAccountID: account.ExternalAccountID,
			AccountName:       account.AccountName.String,
			AppMode:           ingestionMode,
			Scopes:            account.Scope,
			ConnectionType:    account.ConnectionType,
			PlanAllowsInbox:   true,
		},
		xinbox.InboxItem{
			SocialAccountID:  account.ID,
			WorkspaceID:      workspaceID,
			Source:           entry.Source,
			ExternalID:       entry.ExternalID,
			ParentExternalID: entry.ParentExternalID,
			AuthorName:       entry.AuthorName,
			AuthorID:         entry.AuthorID,
			AuthorAvatarURL:  entry.AuthorAvatarURL,
			Body:             entry.Body,
			IsOwn:            isOwn,
			ReceivedAt:       entry.Timestamp,
			Metadata:         metadata,
			ThreadKey:        firstNonEmptyString(entry.ThreadKey, entry.ExternalID),
			ThreadStatus:     "open",
			LinkedPostID:     linkedPostID.String,
		},
		operation,
		"backfill",
	)
}

func isFacebookPlaceholderAuthorName(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), "facebook user")
}

func mergeInboxItemAuthorMetadata(ctx context.Context, queries *db.Queries, socialAccountID, externalID, authorName, authorID, authorAvatarURL string) {
	if authorName == "" && authorID == "" && authorAvatarURL == "" {
		return
	}
	_, _ = queries.MergeInboxItemAuthorMetadataByExternalID(ctx, db.MergeInboxItemAuthorMetadataByExternalIDParams{
		SocialAccountID: socialAccountID,
		ExternalID:      externalID,
		AuthorName:      authorName,
		AuthorID:        authorID,
		AuthorAvatarUrl: authorAvatarURL,
	})
}
