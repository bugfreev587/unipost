package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
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

func TestInboxXReplyRequiresFullCapabilityScopes(t *testing.T) {
	tests := []struct {
		source string
		scopes []string
		want   []string
	}{
		{
			source: "x_reply",
			scopes: []string{"tweet.write"},
			want:   []string{"tweet.read", "users.read"},
		},
		{
			source: "x_dm",
			scopes: []string{"dm.write"},
			want:   []string{"dm.read", "tweet.read", "users.read"},
		},
	}
	for _, tt := range tests {
		if got := xInboxReplyMissingScopes(tt.source, tt.scopes); !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%s missing scopes = %v, want %v", tt.source, got, tt.want)
		}
	}
}

func TestInboxXBackfillRequiresPreciseLookupScopes(t *testing.T) {
	if got := xInboxBackfillMissingScopes("x_reply", []string{"users.read"}); !reflect.DeepEqual(got, []string{"tweet.read"}) {
		t.Fatalf("x_reply missing scopes = %v", got)
	}
	if got := xInboxBackfillMissingScopes("x_dm", []string{"dm.read"}); !reflect.DeepEqual(got, []string{"tweet.read", "users.read"}) {
		t.Fatalf("x_dm missing scopes = %v", got)
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
	event        xcredits.UsageEvent
	events       []xcredits.UsageEvent
	reserve      xcredits.ReserveRequest
	reserveCalls int
	finalizedID  string
	reversedID   string
	reserveErr   error
	finalizeErr  error
	reverseErr   error
	snapshot     xcredits.Snapshot
}

func (f *fakeXInboxCredits) Reserve(_ context.Context, req xcredits.ReserveRequest) (xcredits.UsageEvent, error) {
	f.reserve = req
	f.reserveCalls++
	if len(f.events) > 0 {
		event := f.events[0]
		f.events = f.events[1:]
		return event, f.reserveErr
	}
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

type fakeXInboxBackfillAdapter struct {
	mentionPageSizes []int
	dmPageSizes      []int
	mentionTokens    []string
	mentionPages     []platform.TwitterInboxPage
	dmTokens         []string
	dmPages          []platform.TwitterInboxPage
}

func (f *fakeXInboxBackfillAdapter) FetchInboxMentions(
	_ context.Context,
	_ string,
	_ string,
	_ time.Time,
	paginationToken string,
	maxResults int,
) (platform.TwitterInboxPage, error) {
	f.mentionPageSizes = append(f.mentionPageSizes, maxResults)
	f.mentionTokens = append(f.mentionTokens, paginationToken)
	if len(f.mentionPages) > 0 {
		page := f.mentionPages[0]
		f.mentionPages = f.mentionPages[1:]
		return page, nil
	}
	return platform.TwitterInboxPage{}, nil
}

func (f *fakeXInboxBackfillAdapter) FetchInboxDMEvents(
	_ context.Context,
	_ string,
	_ time.Time,
	paginationToken string,
	maxResults int,
) (platform.TwitterInboxPage, error) {
	f.dmPageSizes = append(f.dmPageSizes, maxResults)
	f.dmTokens = append(f.dmTokens, paginationToken)
	if len(f.dmPages) > 0 {
		page := f.dmPages[0]
		f.dmPages = f.dmPages[1:]
		return page, nil
	}
	return platform.TwitterInboxPage{}, nil
}

func (f *fakeXInboxBackfillAdapter) SendInboxReply(context.Context, string, string, string) (*platform.PostResult, error) {
	return nil, errors.New("not used")
}

func (f *fakeXInboxBackfillAdapter) SendInboxDMToConversation(context.Context, string, string, string) (*platform.TwitterDMSendResult, error) {
	return nil, errors.New("not used")
}

func (f *fakeXInboxBackfillAdapter) SendInboxDMToParticipant(context.Context, string, string, string) (*platform.TwitterDMSendResult, error) {
	return nil, errors.New("not used")
}

func TestInboxXBackfillSendsOneResultPageWhenOnlyOneItemIsAffordable(t *testing.T) {
	monthly := int64(15)
	daily := int64(100)
	credits := &fakeXInboxCredits{snapshot: xcredits.Snapshot{
		MonthlyRemaining:  &monthly,
		InboundDailyLimit: &daily,
	}}
	handler := &InboxHandler{xCredits: credits}
	adapter := &fakeXInboxBackfillAdapter{}
	result := &xBackfillAccountResult{}
	handler.runXBackfillPages(
		context.Background(),
		"workspace-1",
		db.SocialAccount{
			ID:                "account-1",
			ExternalAccountID: "user-1",
			XAppMode:          pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
			Scope:             []string{"tweet.read", "users.read"},
		},
		"user-token",
		adapter,
		"x_reply",
		xBackfillRequest{LookbackDays: 7, MaxItems: 100},
		time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC),
		result,
	)
	if !reflect.DeepEqual(adapter.mentionPageSizes, []int{1}) {
		t.Fatalf("mention page sizes = %v, want [1]", adapter.mentionPageSizes)
	}
	if result.StoppedAtBoundary {
		t.Fatalf("one affordable result was incorrectly treated as a boundary: %+v", result)
	}
}

type fakeXInboxIngestionStore struct{}

func (fakeXInboxIngestionStore) AccountForApp(context.Context, string, string) (xinbox.InboxAccount, error) {
	return xinbox.InboxAccount{}, errors.New("not used")
}

func (fakeXInboxIngestionStore) AccountsForExternalUser(context.Context, string, string) ([]xinbox.InboxAccount, error) {
	return nil, errors.New("not used")
}

func (fakeXInboxIngestionStore) InsertInboxItem(context.Context, xinbox.InboxItem) (xinbox.InboxItem, bool, error) {
	return xinbox.InboxItem{}, false, errors.New("not used")
}

func TestInboxXBackfillStopsPaginationAtLocalHorizon(t *testing.T) {
	adapter := &fakeXInboxBackfillAdapter{
		dmPages: []platform.TwitterInboxPage{
			{
				Entries: []platform.TwitterInboxEntry{{
					ExternalID: "dm-1",
					ThreadKey:  "conversation-1",
					Timestamp:  time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC),
					Source:     "x_dm",
				}},
				NextToken:      "page-2",
				HorizonReached: true,
			},
			{},
		},
	}
	ingestion := xinbox.NewIngestionService(xinbox.IngestionConfig{
		Store: fakeXInboxIngestionStore{},
		AtomicProcess: func(
			context.Context,
			xinbox.InboundAdmissionRequest,
			xinbox.InboxItem,
		) (xinbox.InboundAdmission, xinbox.InboxItem, bool, error) {
			return xinbox.InboundAdmission{Accepted: true}, xinbox.InboxItem{}, false, nil
		},
	})
	handler := &InboxHandler{
		xCredits:   &fakeXInboxCredits{},
		xIngestion: ingestion,
	}
	result := &xBackfillAccountResult{}
	handler.runXBackfillPages(
		context.Background(),
		"workspace-1",
		db.SocialAccount{
			ID:                "account-1",
			ExternalAccountID: "user-1",
			XAppMode:          pgtype.Text{String: string(xinbox.AppModeWorkspace), Valid: true},
			Scope:             []string{"dm.read", "tweet.read", "users.read"},
		},
		"user-token",
		adapter,
		"x_dm",
		xBackfillRequest{LookbackDays: 30, MaxItems: 2},
		time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC),
		result,
	)
	if !reflect.DeepEqual(adapter.dmTokens, []string{""}) {
		t.Fatalf("pagination tokens = %v, want one initial request", adapter.dmTokens)
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

func TestInboxXUnknownWriteOutcomeSameKeyRetryDoesNotSendAgain(t *testing.T) {
	tests := []struct {
		name  string
		item  db.InboxItem
		stage string
	}{
		{
			name:  "reply",
			stage: "create_tweet_reply",
			item: db.InboxItem{
				ID:         "item-reply",
				Source:     "x_reply",
				ExternalID: "tweet-1",
				Metadata:   []byte(`{"reply_eligible":true}`),
			},
		},
		{
			name:  "DM",
			stage: "create_dm",
			item: db.InboxItem{
				ID:               "item-dm",
				Source:           "x_dm",
				ExternalID:       "dm-1",
				ParentExternalID: pgtype.Text{String: "conversation-1", Valid: true},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := &fakeFailingXInboxReplyAdapter{
				err: errors.New(tt.stage + ": X inbox API returned HTTP 502"),
			}
			credits := &fakeXInboxCredits{events: []xcredits.UsageEvent{
				{
					ID:             "usage-1",
					Status:         xcredits.UsageStatusProvisional,
					OperationKey:   "post.reply_summoned",
					CatalogVersion: xcredits.CatalogVersion,
					WeightedUnits:  10,
				},
				{
					ID:             "usage-1",
					Status:         xcredits.UsageStatusProvisional,
					OperationKey:   "post.reply_summoned",
					CatalogVersion: xcredits.CatalogVersion,
					WeightedUnits:  10,
					Duplicate:      true,
				},
			}}
			account := db.SocialAccount{
				ID:             "account-1",
				Platform:       "twitter",
				XAppMode:       pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
				ConnectionType: "managed",
			}
			_, firstErr := sendXInboxReply(
				context.Background(), adapter, credits, "workspace-1", account, tt.item,
				"user-token", "thanks", "same-key",
			)
			if !errors.Is(firstErr, ErrXWriteOutcomePending) {
				t.Fatalf("first err = %v, want pending", firstErr)
			}
			_, retryErr := sendXInboxReply(
				context.Background(), adapter, credits, "workspace-1", account, tt.item,
				"user-token", "thanks", "same-key",
			)
			if !errors.Is(retryErr, errXInboxIdempotencyReplay) {
				t.Fatalf("retry err = %v, want idempotency replay", retryErr)
			}
			if adapter.calls != 1 {
				t.Fatalf("upstream calls = %d, want 1", adapter.calls)
			}
			if credits.reserveCalls != 2 {
				t.Fatalf("credit reserve calls = %d, want 2 idempotent attempts", credits.reserveCalls)
			}
			if credits.reversedID != "" || credits.finalizedID != "" {
				t.Fatalf("unknown result was settled: %+v", credits)
			}
		})
	}
}

func TestInboxXOutboundClaimIsRetainedOnlyForUnknownWriteOutcome(t *testing.T) {
	if !retainXInboxOutboundClaim(ErrXWriteOutcomePending) {
		t.Fatal("unknown X write outcome must retain the durable claim")
	}
	if retainXInboxOutboundClaim(errors.New("X inbox API returned HTTP 429")) {
		t.Fatal("definitive 429 rejection must not retain the durable claim")
	}
}

func TestInboxXUnknownWriteOutcomeReturnsReconciliationState(t *testing.T) {
	recorder := httptest.NewRecorder()
	(&InboxHandler{}).writeXInboxReplyError(recorder, ErrXWriteOutcomePending)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
	var response struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error.Code != "X_WRITE_OUTCOME_PENDING" {
		t.Fatalf("code = %q, want X_WRITE_OUTCOME_PENDING", response.Error.Code)
	}
	if !strings.Contains(response.Error.Message, "will not send again") ||
		!strings.Contains(response.Error.Message, "reconciliation") {
		t.Fatalf("message = %q", response.Error.Message)
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
	err   error
	calls int
}

func (f *fakeFailingXInboxReplyAdapter) SendInboxReply(context.Context, string, string, string) (*platform.PostResult, error) {
	f.calls++
	return nil, f.err
}

func (f *fakeFailingXInboxReplyAdapter) SendInboxDMToConversation(context.Context, string, string, string) (*platform.TwitterDMSendResult, error) {
	f.calls++
	return nil, f.err
}

func (f *fakeFailingXInboxReplyAdapter) SendInboxDMToParticipant(context.Context, string, string, string) (*platform.TwitterDMSendResult, error) {
	f.calls++
	return nil, f.err
}
