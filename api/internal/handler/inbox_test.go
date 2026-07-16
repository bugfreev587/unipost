package handler

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/xcredits"
	"github.com/xiaoboyu/unipost-api/internal/xinbox"
)

func TestInboxXBackfillEstimateIsDeterministicAndBYOIsFree(t *testing.T) {
	request := xBackfillRequest{
		MaxItems:       25,
		IncludeReplies: true,
		IncludeDMs:     true,
	}
	if got := estimateXBackfillCredits(string(xinbox.AppModeUniPostManaged), request); got != 875 {
		t.Fatalf("managed estimate = %d, want 875", got)
	}
	if got := estimateXBackfillCredits(string(xinbox.AppModeWorkspace), request); got != 0 {
		t.Fatalf("BYO estimate = %d, want 0", got)
	}
}

func TestInboxXBackfillConfirmationTokenIsWorkspaceBoundAndExpires(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	claim := xBackfillConfirmationClaim{
		WorkspaceID:  "workspace-1",
		AccountID:    "account-1",
		LookbackDays: 30,
		MaxItems:     100,
		Replies:      true,
		DMs:          true,
	}
	token, expiresAt, err := signXBackfillConfirmationToken([]byte("confirmation-secret"), claim, now)
	if err != nil {
		t.Fatalf("signXBackfillConfirmationToken: %v", err)
	}
	if !expiresAt.Equal(now.Add(xBackfillConfirmationTTL)) {
		t.Fatalf("expiresAt = %s", expiresAt)
	}
	if err := verifyXBackfillConfirmationToken([]byte("confirmation-secret"), token, claim, now.Add(time.Minute)); err != nil {
		t.Fatalf("verify valid token: %v", err)
	}
	wrongWorkspace := claim
	wrongWorkspace.WorkspaceID = "workspace-2"
	if err := verifyXBackfillConfirmationToken([]byte("confirmation-secret"), token, wrongWorkspace, now.Add(time.Minute)); err == nil {
		t.Fatal("wrong workspace token verified")
	}
	if err := verifyXBackfillConfirmationToken([]byte("confirmation-secret"), token, claim, expiresAt.Add(time.Second)); err == nil {
		t.Fatal("expired token verified")
	}
}

func TestInboxXReplyEligibilityRequiresPersistedSummonedItem(t *testing.T) {
	eligible := db.InboxItem{
		Source:   "x_reply",
		Metadata: []byte(`{"reply_eligible":true}`),
	}
	if err := validateXInboxReplyTarget(eligible); err != nil {
		t.Fatalf("eligible target rejected: %v", err)
	}
	for _, item := range []db.InboxItem{
		{Source: "x_reply", Metadata: []byte(`{}`)},
		{Source: "x_reply", IsOwn: true, Metadata: []byte(`{"reply_eligible":true}`)},
		{Source: "ig_comment", Metadata: []byte(`{"reply_eligible":true}`)},
	} {
		if err := validateXInboxReplyTarget(item); err == nil {
			t.Fatalf("ineligible target accepted: %+v", item)
		}
	}
	if err := validateXInboxReplyTarget(db.InboxItem{Source: "x_dm"}); err != nil {
		t.Fatalf("X DM target rejected: %v", err)
	}
}

func TestInboxXReplyRequiresCapabilitySpecificWriteScope(t *testing.T) {
	if missing := xInboxReplyMissingScope("x_reply", []string{"tweet.read", "tweet.write"}); missing != "" {
		t.Fatalf("x_reply missing scope = %q", missing)
	}
	if missing := xInboxReplyMissingScope("x_dm", []string{"dm.read"}); missing != "dm.write" {
		t.Fatalf("x_dm missing scope = %q, want dm.write", missing)
	}
}

func TestInboxXReplyOperationUsesSummonedPriceUnlessTextContainsURL(t *testing.T) {
	if got := xInboxReplyOperation("thanks for reaching out"); got != "post.reply_summoned" {
		t.Fatalf("plain reply operation = %q", got)
	}
	if got := xInboxReplyOperation("details at https://unipost.dev/docs"); got != "post.create_url" {
		t.Fatalf("URL reply operation = %q", got)
	}
}

func TestInboxXIdempotencyPayloadHashRejectsKeyReuseWithDifferentText(t *testing.T) {
	item := db.InboxItem{ID: "item-1", Source: "x_reply"}
	first := xInboxReplyPayloadHash(item, "first reply")
	if first == xInboxReplyPayloadHash(item, "different reply") {
		t.Fatal("payload hash did not include reply text")
	}
	if first != xInboxReplyPayloadHash(item, "first reply") {
		t.Fatal("payload hash is not deterministic")
	}
}

type fakeXInboxReplyAdapter struct {
	replyCalls       int
	conversationDMs  int
	participantDMs   int
	lastConversation string
	lastParticipant  string
}

func (f *fakeXInboxReplyAdapter) SendInboxReply(context.Context, string, string, string) (*platform.PostResult, error) {
	f.replyCalls++
	return &platform.PostResult{ExternalID: "tweet-2", URL: "https://x.com/i/status/tweet-2"}, nil
}

func (f *fakeXInboxReplyAdapter) SendInboxDMToConversation(_ context.Context, _ string, conversationID string, _ string) (*platform.TwitterDMSendResult, error) {
	f.conversationDMs++
	f.lastConversation = conversationID
	return &platform.TwitterDMSendResult{ExternalID: "dm-2", ConversationID: conversationID}, nil
}

func (f *fakeXInboxReplyAdapter) SendInboxDMToParticipant(_ context.Context, _ string, participantID string, _ string) (*platform.TwitterDMSendResult, error) {
	f.participantDMs++
	f.lastParticipant = participantID
	return &platform.TwitterDMSendResult{ExternalID: "dm-3", ConversationID: "conversation-new"}, nil
}

type fakeXInboxCredits struct {
	event       xcredits.UsageEvent
	reserve     xcredits.ReserveRequest
	finalizedID string
	reversedID  string
	reserveErr  error
	finalizeErr error
	reverseErr  error
	snapshot    xcredits.Snapshot
}

func (f *fakeXInboxCredits) Reserve(_ context.Context, req xcredits.ReserveRequest) (xcredits.UsageEvent, error) {
	f.reserve = req
	return f.event, f.reserveErr
}

func (f *fakeXInboxCredits) Finalize(_ context.Context, id string, _ int64) error {
	f.finalizedID = id
	return f.finalizeErr
}

func (f *fakeXInboxCredits) Reverse(_ context.Context, id string) error {
	f.reversedID = id
	return f.reverseErr
}

func (f *fakeXInboxCredits) Snapshot(context.Context, string, time.Time) (xcredits.Snapshot, error) {
	return f.snapshot, nil
}

func (f *fakeXInboxCredits) AdmitInboundWithMutation(
	context.Context,
	xcredits.InboundRequest,
	xcredits.InboundMutation,
) (xcredits.InboundAdmission, error) {
	return xcredits.InboundAdmission{}, nil
}

func (f *fakeXInboxCredits) AdmitInbound(
	context.Context,
	xcredits.InboundRequest,
) (xcredits.InboundAdmission, error) {
	return xcredits.InboundAdmission{Decision: xcredits.InboundDecisionAccepted}, nil
}

func TestInboxXBackfillRechecksAffordablePageAtDailyAndMonthlyBoundaries(t *testing.T) {
	monthly := int64(100)
	daily := int64(200)
	credits := &fakeXInboxCredits{snapshot: xcredits.Snapshot{
		MonthlyRemaining:  &monthly,
		InboundDailyLimit: &daily,
	}}
	handler := &InboxHandler{xCredits: credits}
	got, reason, err := handler.xBackfillAffordablePageSize(
		context.Background(),
		"workspace-1",
		string(xinbox.AppModeUniPostManaged),
		"post.read",
		100,
	)
	if err != nil {
		t.Fatalf("xBackfillAffordablePageSize: %v", err)
	}
	if got != 6 || reason != "" {
		t.Fatalf("affordable = %d, reason = %q, want 6", got, reason)
	}

	credits.snapshot.PausePaidSources = true
	credits.snapshot.InboundPauseReason = xcredits.PauseReasonDailySafetyBuffer
	got, reason, err = handler.xBackfillAffordablePageSize(
		context.Background(),
		"workspace-1",
		string(xinbox.AppModeUniPostManaged),
		"post.read",
		100,
	)
	if err != nil || got != 0 || reason != xcredits.PauseReasonDailySafetyBuffer {
		t.Fatalf("paused affordable = %d, reason = %q, err = %v", got, reason, err)
	}
}

func TestInboxXReplyCountsFinalizesAndClassifiesSummonedReply(t *testing.T) {
	adapter := &fakeXInboxReplyAdapter{}
	credits := &fakeXInboxCredits{event: xcredits.UsageEvent{
		ID:             "usage-1",
		Status:         xcredits.UsageStatusProvisional,
		OperationKey:   "post.reply_summoned",
		CatalogVersion: xcredits.CatalogVersion,
		WeightedUnits:  xcredits.OperationWeight("post.reply_summoned"),
	}}
	account := db.SocialAccount{
		ID:             "account-1",
		Platform:       "twitter",
		XAppMode:       pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
		ConnectionType: "managed",
	}
	item := db.InboxItem{
		ID:         "item-1",
		Source:     "x_reply",
		ExternalID: "tweet-1",
		Metadata:   []byte(`{"reply_eligible":true}`),
	}
	result, err := sendXInboxReply(
		context.Background(),
		adapter,
		credits,
		"workspace-1",
		account,
		item,
		"user-token",
		"thanks",
		"client-key",
	)
	if err != nil {
		t.Fatalf("sendXInboxReply: %v", err)
	}
	if adapter.replyCalls != 1 || credits.finalizedID != "usage-1" || credits.reversedID != "" {
		t.Fatalf("adapter/settlement = %+v %+v", adapter, credits)
	}
	if credits.reserve.OperationKey != "post.reply_summoned" ||
		credits.reserve.IdempotencyKey != "inbox:item-1:client-key" {
		t.Fatalf("reserve = %+v", credits.reserve)
	}
	if result.XCreditsCounted != 10 || result.Operation != "post.reply_summoned" ||
		result.CatalogVersion != xcredits.CatalogVersion ||
		result.BillingMode != string(xinbox.AppModeUniPostManaged) {
		t.Fatalf("result = %+v", result)
	}
}

func TestInboxXDMHasNoMetaWindowAndUsesConversationThenParticipantFallback(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	if metaDMReplyWindowClosed(db.InboxItem{
		Source:     "x_dm",
		ReceivedAt: pgtype.Timestamptz{Time: now.Add(-60 * 24 * time.Hour), Valid: true},
	}, now) {
		t.Fatal("X DM must not use Meta's 24-hour reply window")
	}

	account := db.SocialAccount{
		ID:             "account-1",
		Platform:       "twitter",
		XAppMode:       pgtype.Text{String: string(xinbox.AppModeWorkspace), Valid: true},
		ConnectionType: "byo",
	}
	adapter := &fakeXInboxReplyAdapter{}
	credits := &fakeXInboxCredits{event: xcredits.UsageEvent{Status: xcredits.UsageStatusBypassed}}
	item := db.InboxItem{
		ID:               "item-1",
		Source:           "x_dm",
		ParentExternalID: pgtype.Text{String: "conversation-1", Valid: true},
		AuthorID:         pgtype.Text{String: "participant-1", Valid: true},
	}
	result, err := sendXInboxReply(
		context.Background(), adapter, credits, "workspace-1", account, item,
		"user-token", "hello", "client-key",
	)
	if err != nil {
		t.Fatalf("conversation send: %v", err)
	}
	if adapter.conversationDMs != 1 || adapter.lastConversation != "conversation-1" ||
		result.XCreditsCounted != 0 || result.Operation != "dm.send" ||
		result.BillingMode != string(xinbox.AppModeWorkspace) {
		t.Fatalf("conversation result = %+v adapter = %+v", result, adapter)
	}

	item.ParentExternalID = pgtype.Text{String: "x-dm:account-1:participant-1", Valid: true}
	item.ThreadKey = "x-dm:account-1:participant-1"
	_, err = sendXInboxReply(
		context.Background(), adapter, credits, "workspace-1", account, item,
		"user-token", "hello again", "client-key-2",
	)
	if err != nil {
		t.Fatalf("participant send: %v", err)
	}
	if adapter.participantDMs != 1 || adapter.lastParticipant != "participant-1" {
		t.Fatalf("participant adapter = %+v", adapter)
	}
}

func TestInboxXIdempotencyDuplicateDoesNotCallUpstream(t *testing.T) {
	adapter := &fakeXInboxReplyAdapter{}
	credits := &fakeXInboxCredits{event: xcredits.UsageEvent{
		ID:             "usage-1",
		Status:         xcredits.UsageStatusFinalized,
		OperationKey:   "post.reply_summoned",
		CatalogVersion: xcredits.CatalogVersion,
		WeightedUnits:  10,
		Duplicate:      true,
	}}
	account := db.SocialAccount{
		ID:             "account-1",
		Platform:       "twitter",
		XAppMode:       pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
		ConnectionType: "managed",
	}
	item := db.InboxItem{
		ID:         "item-1",
		Source:     "x_reply",
		ExternalID: "tweet-1",
		Metadata:   []byte(`{"reply_eligible":true}`),
	}
	_, err := sendXInboxReply(
		context.Background(), adapter, credits, "workspace-1", account, item,
		"user-token", "thanks", "same-key",
	)
	if !errors.Is(err, errXInboxIdempotencyReplay) {
		t.Fatalf("err = %v, want idempotency replay", err)
	}
	if adapter.replyCalls != 0 {
		t.Fatalf("reply calls = %d, want 0", adapter.replyCalls)
	}
}

func TestInboxXConfirmedFailureReversesUsage(t *testing.T) {
	adapter := &fakeFailingXInboxReplyAdapter{
		err: errors.New("X inbox API returned HTTP 403"),
	}
	credits := &fakeXInboxCredits{event: xcredits.UsageEvent{
		ID:             "usage-1",
		Status:         xcredits.UsageStatusProvisional,
		OperationKey:   "post.reply_summoned",
		CatalogVersion: xcredits.CatalogVersion,
		WeightedUnits:  10,
	}}
	account := db.SocialAccount{
		ID:             "account-1",
		Platform:       "twitter",
		XAppMode:       pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
		ConnectionType: "managed",
	}
	item := db.InboxItem{
		ID:         "item-1",
		Source:     "x_reply",
		ExternalID: "tweet-1",
		Metadata:   []byte(`{"reply_eligible":true}`),
	}
	_, err := sendXInboxReply(
		context.Background(), adapter, credits, "workspace-1", account, item,
		"user-token", "thanks", "client-key",
	)
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err = %v", err)
	}
	if credits.reversedID != "usage-1" || credits.finalizedID != "" {
		t.Fatalf("settlement = %+v", credits)
	}
}

func TestInboxXUnknownWriteOutcomeKeepsProvisionalUsage(t *testing.T) {
	adapter := &fakeFailingXInboxReplyAdapter{
		err: errors.New("create_tweet_reply timeout after 20s: context deadline exceeded"),
	}
	credits := &fakeXInboxCredits{event: xcredits.UsageEvent{
		ID:             "usage-1",
		Status:         xcredits.UsageStatusProvisional,
		OperationKey:   "post.reply_summoned",
		CatalogVersion: xcredits.CatalogVersion,
		WeightedUnits:  10,
	}}
	account := db.SocialAccount{
		ID:             "account-1",
		Platform:       "twitter",
		XAppMode:       pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
		ConnectionType: "managed",
	}
	item := db.InboxItem{
		ID:         "item-1",
		Source:     "x_reply",
		ExternalID: "tweet-1",
		Metadata:   []byte(`{"reply_eligible":true}`),
	}
	_, err := sendXInboxReply(
		context.Background(), adapter, credits, "workspace-1", account, item,
		"user-token", "thanks", "client-key",
	)
	if !errors.Is(err, ErrXWriteOutcomePending) {
		t.Fatalf("err = %v, want ErrXWriteOutcomePending", err)
	}
	if credits.reversedID != "" || credits.finalizedID != "" {
		t.Fatalf("unknown result was settled: %+v", credits)
	}
}

func TestInboxXFinalizeFailureStillReturnsAcceptedUpstreamResult(t *testing.T) {
	adapter := &fakeXInboxReplyAdapter{}
	credits := &fakeXInboxCredits{
		event: xcredits.UsageEvent{
			ID:             "usage-1",
			Status:         xcredits.UsageStatusProvisional,
			OperationKey:   "post.reply_summoned",
			CatalogVersion: xcredits.CatalogVersion,
			WeightedUnits:  10,
		},
		finalizeErr: errors.New("database unavailable"),
	}
	account := db.SocialAccount{
		ID:             "account-1",
		Platform:       "twitter",
		XAppMode:       pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
		ConnectionType: "managed",
	}
	item := db.InboxItem{
		ID:         "item-1",
		Source:     "x_reply",
		ExternalID: "tweet-1",
		Metadata:   []byte(`{"reply_eligible":true}`),
	}
	result, err := sendXInboxReply(
		context.Background(), adapter, credits, "workspace-1", account, item,
		"user-token", "thanks", "client-key",
	)
	if err != nil {
		t.Fatalf("err = %v, want accepted upstream result", err)
	}
	if adapter.replyCalls != 1 || credits.reversedID != "" {
		t.Fatalf("adapter/credits = %+v %+v", adapter, credits)
	}
	if !result.SettlementPending || result.ExternalID != "tweet-2" {
		t.Fatalf("result = %+v", result)
	}
}

type fakeFailingXInboxReplyAdapter struct {
	err error
}

func (f *fakeFailingXInboxReplyAdapter) SendInboxReply(context.Context, string, string, string) (*platform.PostResult, error) {
	return nil, f.err
}

func (f *fakeFailingXInboxReplyAdapter) SendInboxDMToConversation(context.Context, string, string, string) (*platform.TwitterDMSendResult, error) {
	return nil, f.err
}

func (f *fakeFailingXInboxReplyAdapter) SendInboxDMToParticipant(context.Context, string, string, string) (*platform.TwitterDMSendResult, error) {
	return nil, f.err
}
