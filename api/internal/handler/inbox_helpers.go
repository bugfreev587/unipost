package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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

var errXInboxIdempotencyReplay = errors.New("X Inbox reply idempotency replay")

func xInboxReplyPayloadHash(item db.InboxItem, text string) string {
	sum := sha256.Sum256([]byte(item.ID + "\x00" + item.Source + "\x00" + text))
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

type xBackfillConfirmationClaim struct {
	WorkspaceID  string `json:"workspace_id"`
	AccountID    string `json:"account_id,omitempty"`
	LookbackDays int    `json:"lookback_days"`
	MaxItems     int    `json:"max_items"`
	Replies      bool   `json:"replies"`
	DMs          bool   `json:"dms"`
	ExpiresAt    int64  `json:"expires_at"`
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
		estimate += int64(maxItems) * (xcredits.OperationWeight("post.read") +
			xcredits.OperationWeight("user.read"))
	}
	if request.IncludeDMs {
		estimate += int64(maxItems) * (xcredits.OperationWeight("dm.read") +
			xcredits.OperationWeight("user.read"))
	}
	return estimate
}

func signXBackfillConfirmationToken(
	secret []byte,
	claim xBackfillConfirmationClaim,
	now time.Time,
) (string, time.Time, error) {
	if len(secret) == 0 {
		return "", time.Time{}, errors.New("X backfill confirmation signing secret is not configured")
	}
	expiresAt := now.UTC().Add(xBackfillConfirmationTTL)
	claim.ExpiresAt = expiresAt.Unix()
	raw, err := json.Marshal(claim)
	if err != nil {
		return "", time.Time{}, err
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(raw)
	return base64.RawURLEncoding.EncodeToString(raw) + "." +
		base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), expiresAt, nil
}

func verifyXBackfillConfirmationToken(
	secret []byte,
	token string,
	expected xBackfillConfirmationClaim,
	now time.Time,
) error {
	if len(secret) == 0 {
		return errors.New("X backfill confirmation signing secret is not configured")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return errors.New("invalid X backfill confirmation token")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return errors.New("invalid X backfill confirmation token")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return errors.New("invalid X backfill confirmation token")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(raw)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return errors.New("invalid X backfill confirmation token")
	}
	var actual xBackfillConfirmationClaim
	if err := json.Unmarshal(raw, &actual); err != nil {
		return errors.New("invalid X backfill confirmation token")
	}
	if actual.ExpiresAt <= now.UTC().Unix() {
		return errors.New("X backfill confirmation token expired")
	}
	if actual.WorkspaceID != expected.WorkspaceID ||
		actual.AccountID != expected.AccountID ||
		actual.LookbackDays != expected.LookbackDays ||
		actual.MaxItems != expected.MaxItems ||
		actual.Replies != expected.Replies ||
		actual.DMs != expected.DMs {
		return errors.New("X backfill confirmation token does not match this request")
	}
	return nil
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

func xInboxReplyMissingScope(source string, scopes []string) string {
	required := "tweet.write"
	if source == "x_dm" {
		required = "dm.write"
	}
	for _, scope := range scopes {
		if strings.EqualFold(strings.TrimSpace(scope), required) {
			return ""
		}
	}
	return required
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
	Finalize(context.Context, string, int64) error
	Reverse(context.Context, string) error
}

type xInboxSendResult struct {
	ExternalID        string
	ConversationID    string
	URL               string
	XCreditsCounted   int64
	Operation         string
	CatalogVersion    string
	BillingMode       string
	SettlementPending bool
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
		Operation:      operation,
		CatalogVersion: xcredits.CatalogVersion,
		BillingMode:    string(mode),
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
				return xInboxSendResult{}, ErrXWriteOutcomePending
			}
			if reverseErr := credits.Reverse(ctx, event.ID); reverseErr != nil {
				return xInboxSendResult{}, errors.Join(sendErr, reverseErr)
			}
		}
		return xInboxSendResult{}, sendErr
	}
	if event.ID != "" {
		if err := credits.Finalize(ctx, event.ID, event.WeightedUnits); err != nil {
			result.SettlementPending = true
		}
		result.XCreditsCounted = event.WeightedUnits
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
