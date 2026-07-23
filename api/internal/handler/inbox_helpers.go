package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/xcredits"
	"github.com/xiaoboyu/unipost-api/internal/xinbox"
)

const xBackfillConfirmationTTL = 10 * time.Minute
const xMentionsMinimumPageSize = 5

var errXInboxIdempotencyReplay = errors.New("X Inbox reply idempotency replay")
var ErrXUsageReversalPending = errors.New("X Inbox usage reversal is pending")

func xInboxReplyPayloadHash(item db.InboxItem, text string) string {
	sum := sha256.Sum256([]byte(item.ID + "\x00" + item.Source + "\x00" + text))
	return hex.EncodeToString(sum[:])
}

func xInboxReplyBodyHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

type xBackfillRequest struct {
	AccountID         string `json:"account_id,omitempty"`
	LookbackDays      int    `json:"lookback_days,omitempty"`
	MaxItems          int    `json:"max_items,omitempty"`
	IncludeReplies    bool   `json:"include_replies"`
	IncludeDMs        bool   `json:"include_dms"`
	ConfirmationToken string `json:"confirmation_token,omitempty"`
}

func estimateXBackfillCredits(appMode string, request xBackfillRequest) int64 {
	mode, err := xinbox.NormalizePersistedAppMode(appMode)
	if err != nil || mode != xinbox.AppModeUniPostManaged {
		return 0
	}
	maxItems := request.MaxItems
	if maxItems < 0 {
		maxItems = 0
	}
	estimate := int64(0)
	if request.IncludeReplies {
		replyItems := maxItems
		if replyItems > 0 && replyItems < xMentionsMinimumPageSize {
			replyItems = xMentionsMinimumPageSize
		}
		estimate += int64(replyItems) * (xcredits.OperationWeight("post.read") +
			xcredits.OperationWeight("user.read"))
	}
	if request.IncludeDMs {
		estimate += int64(maxItems) * (xcredits.OperationWeight("dm.read") +
			xcredits.OperationWeight("user.read"))
	}
	return estimate
}

func xBackfillExposureKey(
	runID string,
	accountID string,
	source string,
	startTime time.Time,
	paginationToken string,
	pageSize int,
) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf(
		"%s\x00%s\x00%s\x00%s\x00%s\x00%d",
		runID,
		accountID,
		source,
		startTime.UTC().Format(time.RFC3339Nano),
		paginationToken,
		pageSize,
	)))
	return "x-inbox-backfill:" + hex.EncodeToString(sum[:])
}

func newXBackfillRunID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err == nil {
		return hex.EncodeToString(raw)
	}
	return fmt.Sprintf("fallback-%d", time.Now().UTC().UnixNano())
}

func xInboxReadOutcomeAmbiguous(err error) bool {
	if err == nil {
		return false
	}
	var httpErr *platform.TwitterInboxHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Retryable && httpErr.StatusCode != 429
	}
	message := strings.ToLower(err.Error())
	return strings.HasPrefix(message, "x_inbox_read timeout") ||
		strings.HasPrefix(message, "x_inbox_read canceled") ||
		strings.HasPrefix(message, "x_inbox_read: decode x inbox response")
}

func xProviderAccountID(account db.SocialAccount) string {
	return strings.TrimSpace(account.ExternalAccountID)
}

func xBackfillSafeUpstreamError(err error) *xBackfillUpstreamError {
	var providerErr *platform.TwitterInboxHTTPError
	if !errors.As(err, &providerErr) || providerErr == nil || providerErr.StatusCode == 0 {
		return nil
	}
	return &xBackfillUpstreamError{
		Method:     providerErr.Method,
		Path:       providerErr.Path,
		StatusCode: providerErr.StatusCode,
		Code:       providerErr.Code,
		Title:      providerErr.Title,
		Message:    providerErr.Message,
	}
}

func validateXInboxReplyTarget(item db.InboxItem) error {
	if item.IsOwn {
		return errors.New("cannot reply to an outbound Inbox item")
	}
	switch item.Source {
	case "x_dm":
		return nil
	case "x_reply":
		var metadata struct {
			ReplyEligible bool `json:"reply_eligible"`
		}
		if err := json.Unmarshal(item.Metadata, &metadata); err != nil || !metadata.ReplyEligible {
			return errors.New("X reply is not eligible: the persisted Inbox item does not prove the author summoned this account")
		}
		return nil
	default:
		return fmt.Errorf("unsupported X Inbox source %q", item.Source)
	}
}

func xInboxReplyMissingScopes(source string, scopes []string) []string {
	required := []string{"tweet.read", "tweet.write", "users.read"}
	if source == "x_dm" {
		required = []string{"dm.read", "dm.write", "tweet.read", "users.read"}
	}
	return missingXScopes(scopes, required...)
}

func xInboxBackfillMissingScopes(source string, scopes []string) []string {
	required := []string{"tweet.read", "users.read"}
	if source == "x_dm" {
		required = []string{"dm.read", "tweet.read", "users.read"}
	}
	return missingXScopes(scopes, required...)
}

func missingXScopes(scopes []string, required ...string) []string {
	missing := make([]string, 0, len(required))
	for _, want := range required {
		if !hasXScopes(scopes, want) {
			missing = append(missing, want)
		}
	}
	return missing
}

func hasXScopes(scopes []string, required ...string) bool {
	for _, want := range required {
		found := false
		for _, scope := range scopes {
			if strings.EqualFold(strings.TrimSpace(scope), want) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func xInboxReplyOperation(text string) string {
	if xURLCandidatePattern.MatchString(text) {
		return "post.create_url"
	}
	return "post.reply_summoned"
}

type xInboxReplyAdapter interface {
	SendInboxReply(context.Context, string, string, string) (*platform.PostResult, error)
	SendInboxDMToConversation(context.Context, string, string, string) (*platform.TwitterDMSendResult, error)
	SendInboxDMToParticipant(context.Context, string, string, string) (*platform.TwitterDMSendResult, error)
}

type xInboxReplyCredits interface {
	Reserve(context.Context, xcredits.ReserveRequest) (xcredits.UsageEvent, error)
	Reverse(context.Context, string) error
}

type xInboxSendResult struct {
	ExternalID      string
	ConversationID  string
	URL             string
	XCreditsCounted int64
	Operation       string
	CatalogVersion  string
	BillingMode     string
	UsageEventID    string
}

func sendXInboxReply(
	ctx context.Context,
	adapter xInboxReplyAdapter,
	credits xInboxReplyCredits,
	workspaceID string,
	account db.SocialAccount,
	item db.InboxItem,
	accessToken string,
	text string,
	idempotencyKey string,
) (xInboxSendResult, error) {
	return sendXInboxReplyWithReservation(
		ctx,
		adapter,
		credits,
		workspaceID,
		account,
		item,
		accessToken,
		text,
		idempotencyKey,
		nil,
	)
}

func sendXInboxReplyWithReservation(
	ctx context.Context,
	adapter xInboxReplyAdapter,
	credits xInboxReplyCredits,
	workspaceID string,
	account db.SocialAccount,
	item db.InboxItem,
	accessToken string,
	text string,
	idempotencyKey string,
	onReserved func(xInboxSendResult) error,
) (xInboxSendResult, error) {
	if err := validateXInboxReplyTarget(item); err != nil {
		return xInboxSendResult{}, err
	}
	if account.Platform != "twitter" || account.ID != item.SocialAccountID && item.SocialAccountID != "" {
		return xInboxSendResult{}, errors.New("Inbox item does not belong to this X account")
	}
	mode, err := xinbox.NormalizePersistedAppMode(account.XAppMode.String)
	if err != nil {
		return xInboxSendResult{}, err
	}
	operation := "dm.send"
	if item.Source == "x_reply" {
		operation = xInboxReplyOperation(text)
	}
	event := xcredits.UsageEvent{Status: xcredits.UsageStatusBypassed}
	if mode == xinbox.AppModeUniPostManaged {
		if credits == nil {
			return xInboxSendResult{}, errors.New("X credits service is not configured")
		}
		event, err = credits.Reserve(ctx, xcredits.ReserveRequest{
			WorkspaceID:     workspaceID,
			SocialAccountID: account.ID,
			AppMode:         string(mode),
			ConnectionType:  account.ConnectionType,
			OperationKey:    operation,
			Source:          "reply",
			IdempotencyKey:  "inbox:" + item.ID + ":" + idempotencyKey,
			RequestedUnits:  xcredits.OperationWeight(operation),
		})
		if err != nil {
			return xInboxSendResult{}, err
		}
		if event.Duplicate &&
			(event.Status == xcredits.UsageStatusProvisional || event.Status == xcredits.UsageStatusFinalized) {
			return xInboxSendResult{}, errXInboxIdempotencyReplay
		}
	}

	result := xInboxSendResult{
		Operation:       operation,
		CatalogVersion:  xcredits.CatalogVersion,
		BillingMode:     string(mode),
		UsageEventID:    event.ID,
		XCreditsCounted: event.WeightedUnits,
	}
	if onReserved != nil {
		if err := onReserved(result); err != nil {
			if event.ID != "" {
				if reverseErr := credits.Reverse(ctx, event.ID); reverseErr != nil {
					return result, fmt.Errorf(
						"%w: reservation persistence failed: %v; credit reversal failed: %v",
						ErrXUsageReversalPending, err, reverseErr,
					)
				}
			}
			return xInboxSendResult{}, err
		}
	}
	var sendErr error
	switch item.Source {
	case "x_reply":
		var reply *platform.PostResult
		reply, sendErr = adapter.SendInboxReply(ctx, accessToken, item.ExternalID, text)
		if reply != nil {
			result.ExternalID = reply.ExternalID
			result.URL = reply.URL
		}
	case "x_dm":
		conversationID := strings.TrimSpace(item.ParentExternalID.String)
		if conversationID == "" {
			conversationID = strings.TrimSpace(item.ThreadKey)
		}
		if strings.HasPrefix(conversationID, "x-dm:") {
			conversationID = ""
		}
		var dm *platform.TwitterDMSendResult
		if conversationID != "" {
			dm, sendErr = adapter.SendInboxDMToConversation(ctx, accessToken, conversationID, text)
		} else if item.AuthorID.Valid && strings.TrimSpace(item.AuthorID.String) != "" {
			dm, sendErr = adapter.SendInboxDMToParticipant(ctx, accessToken, item.AuthorID.String, text)
		} else {
			sendErr = errors.New("X DM reply requires a persisted conversation or participant")
		}
		if dm != nil {
			result.ExternalID = dm.ExternalID
			result.ConversationID = dm.ConversationID
		}
	}
	if sendErr != nil {
		if event.ID != "" {
			if xWriteOutcomeUnknown(sendErr) {
				return result, ErrXWriteOutcomePending
			}
			if reverseErr := credits.Reverse(ctx, event.ID); reverseErr != nil {
				return result, fmt.Errorf(
					"%w: X rejected the write: %v; credit reversal failed: %v",
					ErrXUsageReversalPending, sendErr, reverseErr,
				)
			}
		}
		return xInboxSendResult{}, sendErr
	}
	return result, nil
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
