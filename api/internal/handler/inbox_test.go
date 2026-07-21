package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	appcrypto "github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/inboxaccess"
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
	minimumPage := xBackfillRequest{MaxItems: 1, IncludeReplies: true, IncludeDMs: true}
	if got := estimateXBackfillCredits(string(xinbox.AppModeUniPostManaged), minimumPage); got != 95 {
		t.Fatalf("minimum-page estimate = %d, want 95", got)
	}
}

func TestInboxXBackfillOperationTokenReferencesOnlyPersistedOperation(t *testing.T) {
	token, err := signXBackfillOperationToken([]byte("secret"), "operation-1", "nonce-1")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	operationID, nonce, err := verifyXBackfillOperationToken([]byte("secret"), token)
	if err != nil || operationID != "operation-1" || nonce != "nonce-1" {
		t.Fatalf("verify = %q, %q, %v", operationID, nonce, err)
	}
	if _, _, err := verifyXBackfillOperationToken([]byte("different"), token); err == nil {
		t.Fatal("token verified with another secret")
	}
	parts := strings.Split(token, ".")
	parts[1] = "nonce-2"
	if _, _, err := verifyXBackfillOperationToken([]byte("secret"), strings.Join(parts, ".")); err == nil {
		t.Fatal("token verified after nonce tampering")
	}
}

func TestInboxXBackfillExecutionOwnersAreOpaqueAndUnique(t *testing.T) {
	first, err := newXBackfillExecutionOwner()
	if err != nil {
		t.Fatalf("first owner: %v", err)
	}
	second, err := newXBackfillExecutionOwner()
	if err != nil {
		t.Fatalf("second owner: %v", err)
	}
	if first == "" || second == "" || first == second || strings.Contains(first, ".") {
		t.Fatalf("owners = %q, %q", first, second)
	}
}

func TestInboxXBackfillAccountFingerprintFreezesExactAccountSetAndModes(t *testing.T) {
	first := []xBackfillAccountSnapshot{
		{ID: "account-b", AppMode: "workspace_app"},
		{ID: "account-a", AppMode: "unipost_managed_app"},
	}
	sort.Slice(first, func(i, j int) bool { return first[i].ID < first[j].ID })
	baseline := xBackfillAccountFingerprint(first)
	if baseline == xBackfillAccountFingerprint([]xBackfillAccountSnapshot{
		{ID: "account-a", AppMode: "unipost_managed_app"},
	}) {
		t.Fatal("removing an account did not change the fingerprint")
	}
	if baseline == xBackfillAccountFingerprint([]xBackfillAccountSnapshot{
		{ID: "account-a", AppMode: "workspace_app"},
		{ID: "account-b", AppMode: "workspace_app"},
	}) {
		t.Fatal("changing an app mode did not change the fingerprint")
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
	beforeSend       func()
}

func (f *fakeXInboxReplyAdapter) SendInboxReply(context.Context, string, string, string) (*platform.PostResult, error) {
	if f.beforeSend != nil {
		f.beforeSend()
	}
	f.replyCalls++
	return &platform.PostResult{ExternalID: "tweet-2", URL: "https://x.com/i/status/tweet-2"}, nil
}

func TestInboxXReservationStateIsPersistedBeforeUpstreamWrite(t *testing.T) {
	persisted := false
	adapter := &fakeXInboxReplyAdapter{beforeSend: func() {
		if !persisted {
			t.Fatal("upstream write started before durable reservation state")
		}
	}}
	credits := &fakeXInboxCredits{event: xcredits.UsageEvent{
		ID:             "usage-1",
		Status:         xcredits.UsageStatusProvisional,
		OperationKey:   "post.reply_summoned",
		CatalogVersion: xcredits.CatalogVersion,
		WeightedUnits:  10,
	}}
	account := db.SocialAccount{
		ID: "account-1", Platform: "twitter",
		XAppMode: pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
	}
	item := db.InboxItem{
		ID: "item-1", Source: "x_reply", ExternalID: "tweet-1",
		Metadata: []byte(`{"reply_eligible":true}`),
	}
	_, err := sendXInboxReplyWithReservation(
		context.Background(), adapter, credits, "workspace-1", account, item,
		"token", "thanks", "key",
		func(result xInboxSendResult) error {
			if result.UsageEventID != "usage-1" || result.XCreditsCounted != 10 {
				t.Fatalf("reserved result = %+v", result)
			}
			persisted = true
			return nil
		},
	)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
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
	event                  xcredits.UsageEvent
	events                 []xcredits.UsageEvent
	reserve                xcredits.ReserveRequest
	reserveCalls           int
	reversedID             string
	reserveErr             error
	reverseErr             error
	snapshot               xcredits.Snapshot
	exposure               xcredits.ExposureReservation
	exposureErr            error
	exposureFinalizedUnits int64
	exposureReleased       bool
	exposureReconciliation bool
	exposureReconcileCalls int
	exposureReleaseErr     error
	exposureReconcileErr   error
	exposureReconcileErrs  []error
	exposureStarted        bool
	exposureStartErrs      []error
	exposureFinalizeCalls  int
	exposureFinalizeErr    error
	exposurePendingUnits   int64
	exposurePendingErrs    []error
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

func (f *fakeXInboxCredits) Reverse(_ context.Context, id string) error {
	f.reversedID = id
	return f.reverseErr
}

func (f *fakeXInboxCredits) ReverseByIdempotencyKey(context.Context, string, string) error {
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

func (f *fakeXInboxCredits) ReserveExposure(
	_ context.Context,
	req xcredits.ExposureReservationRequest,
) (xcredits.ExposureReservation, error) {
	if f.exposureErr != nil {
		return xcredits.ExposureReservation{}, f.exposureErr
	}
	if f.exposure.ReservedResources == 0 {
		return xcredits.ExposureReservation{
			ID:                 "exposure-1",
			RequestedResources: req.RequestedResources,
			ReservedResources:  req.RequestedResources,
			ReservedUnits:      int64(req.RequestedResources) * req.UnitsPerResource,
			Bypassed:           req.AppMode != string(xinbox.AppModeUniPostManaged),
		}, nil
	}
	return f.exposure, nil
}

func (f *fakeXInboxCredits) FinalizeExposure(_ context.Context, _ string, units int64) error {
	f.exposureFinalizeCalls++
	f.exposureFinalizedUnits = units
	return f.exposureFinalizeErr
}
func (f *fakeXInboxCredits) ReleaseExposure(context.Context, string) error {
	f.exposureReleased = true
	return f.exposureReleaseErr
}
func (f *fakeXInboxCredits) MarkExposureNeedsReconciliation(context.Context, string, string) error {
	f.exposureReconciliation = true
	f.exposureReconcileCalls++
	if len(f.exposureReconcileErrs) > 0 {
		err := f.exposureReconcileErrs[0]
		f.exposureReconcileErrs = f.exposureReconcileErrs[1:]
		return err
	}
	return f.exposureReconcileErr
}
func (f *fakeXInboxCredits) MarkExposureReleasePending(context.Context, string, string) error {
	f.exposureReconciliation = true
	return f.exposureReconcileErr
}

func (f *fakeXInboxCredits) MarkExposureReadStarted(context.Context, string) error {
	f.exposureStarted = true
	if len(f.exposureStartErrs) == 0 {
		return nil
	}
	err := f.exposureStartErrs[0]
	f.exposureStartErrs = f.exposureStartErrs[1:]
	return err
}

func (f *fakeXInboxCredits) MarkExposureFinalizePending(
	_ context.Context,
	_ string,
	actualUnits int64,
	_ string,
) error {
	f.exposurePendingUnits = actualUnits
	if len(f.exposurePendingErrs) == 0 {
		return nil
	}
	err := f.exposurePendingErrs[0]
	f.exposurePendingErrs = f.exposurePendingErrs[1:]
	return err
}

type fakeXInboxBackfillAdapter struct {
	mentionPageSizes []int
	dmPageSizes      []int
	mentionTokens    []string
	mentionPages     []platform.TwitterInboxPage
	dmTokens         []string
	dmPages          []platform.TwitterInboxPage
	mentionErr       error
	beforeMention    func()
}

func (f *fakeXInboxBackfillAdapter) FetchInboxMentions(
	_ context.Context,
	_ string,
	_ string,
	_ time.Time,
	paginationToken string,
	maxResults int,
) (platform.TwitterInboxPage, error) {
	if f.beforeMention != nil {
		f.beforeMention()
	}
	f.mentionPageSizes = append(f.mentionPageSizes, maxResults)
	f.mentionTokens = append(f.mentionTokens, paginationToken)
	if f.mentionErr != nil {
		return platform.TwitterInboxPage{}, f.mentionErr
	}
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

func TestInboxXBackfillDoesNotCallMentionsWhenFewerThanFiveResultsAreAffordable(t *testing.T) {
	monthly := int64(15)
	daily := int64(100)
	credits := &fakeXInboxCredits{snapshot: xcredits.Snapshot{
		MonthlyRemaining:  &monthly,
		InboundDailyLimit: &daily,
	}, exposureErr: xcredits.ErrMonthlyLimitExceeded}
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
		"run-boundary",
		result,
	)
	if len(adapter.mentionPageSizes) != 0 {
		t.Fatalf("mention page sizes = %v, want no paid request", adapter.mentionPageSizes)
	}
	if !result.StoppedAtBoundary || result.StopReason == "" {
		t.Fatalf("result = %+v, want cap/allowance boundary", result)
	}
}

func TestInboxXBackfillDoesNotCallMentionsForOneThroughFourRemainingItems(t *testing.T) {
	for remaining := 1; remaining < xMentionsMinimumPageSize; remaining++ {
		t.Run(fmt.Sprintf("remaining_%d", remaining), func(t *testing.T) {
			handler := &InboxHandler{xCredits: &fakeXInboxCredits{}}
			adapter := &fakeXInboxBackfillAdapter{}
			result := &xBackfillAccountResult{}
			handler.runXBackfillPages(
				context.Background(),
				"workspace-1",
				db.SocialAccount{
					ID:                "account-1",
					ExternalAccountID: "user-1",
					XAppMode:          pgtype.Text{String: string(xinbox.AppModeWorkspace), Valid: true},
					Scope:             []string{"tweet.read", "users.read"},
				},
				"token",
				adapter,
				"x_reply",
				xBackfillRequest{LookbackDays: 7, MaxItems: remaining},
				time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC),
				fmt.Sprintf("run-%d", remaining),
				result,
			)
			if len(adapter.mentionPageSizes) != 0 || !result.StoppedAtBoundary {
				t.Fatalf("adapter/result = %+v %+v", adapter, result)
			}
		})
	}
}

func TestInboxXBackfillDuplicateExposureNeverCallsX(t *testing.T) {
	credits := &fakeXInboxCredits{exposure: xcredits.ExposureReservation{
		ID:                 "existing-reservation",
		RequestedResources: 5,
		ReservedResources:  5,
		ReservedUnits:      75,
		Duplicate:          true,
	}}
	handler := &InboxHandler{xCredits: credits}
	adapter := &fakeXInboxBackfillAdapter{}
	result := &xBackfillAccountResult{}
	handler.runXBackfillPages(
		context.Background(), "workspace-1",
		db.SocialAccount{
			ID: "account-1", ExternalAccountID: "user-1",
			XAppMode: pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
			Scope:    []string{"tweet.read", "users.read"},
		},
		"token", adapter, "x_reply",
		xBackfillRequest{LookbackDays: 7, MaxItems: 5},
		time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC), "run-duplicate", result,
	)
	if len(adapter.mentionPageSizes) != 0 {
		t.Fatalf("duplicate reservation triggered paid X read: %v", adapter.mentionPageSizes)
	}
	if result.StopReason != "duplicate_exposure_reservation" {
		t.Fatalf("result = %+v", result)
	}
}

func TestInboxXBackfillPersistsReadStartedBeforePaidRead(t *testing.T) {
	credits := &fakeXInboxCredits{}
	adapter := &fakeXInboxBackfillAdapter{beforeMention: func() {
		if !credits.exposureStarted {
			t.Fatal("paid X read started before durable read-started state")
		}
	}}
	handler := &InboxHandler{xCredits: credits}
	result := &xBackfillAccountResult{}
	handler.runXBackfillPages(
		context.Background(), "workspace-1",
		db.SocialAccount{
			ID: "account-1", ExternalAccountID: "user-1",
			XAppMode: pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
			Scope:    []string{"tweet.read", "users.read"},
		},
		"token", adapter, "x_reply",
		xBackfillRequest{LookbackDays: 7, MaxItems: 5},
		time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC), "run-started", result,
	)
	if !credits.exposureStarted || len(adapter.mentionPageSizes) != 1 {
		t.Fatalf("credits/adapter = %+v %+v", credits, adapter)
	}
}

func TestInboxXBackfillPersistsExactSettlementIntentBeforeFinalize(t *testing.T) {
	credits := &fakeXInboxCredits{
		exposureFinalizeErr: errors.New("database unavailable"),
		exposurePendingErrs: []error{errors.New("transient persistence failure"), nil},
	}
	adapter := &fakeXInboxBackfillAdapter{mentionPages: []platform.TwitterInboxPage{{
		Entries: []platform.TwitterInboxEntry{{ExternalID: "tweet-2", Source: "x_reply"}},
	}}}
	ingestion := xinbox.NewIngestionService(xinbox.IngestionConfig{
		Store: fakeXInboxIngestionStore{},
		AtomicProcess: func(
			context.Context,
			xinbox.InboundAdmissionRequest,
			xinbox.InboxItem,
		) (xinbox.InboundAdmission, xinbox.InboxItem, bool, error) {
			return xinbox.InboundAdmission{Accepted: true}, xinbox.InboxItem{}, true, nil
		},
	})
	handler := &InboxHandler{xCredits: credits, xIngestion: ingestion}
	result := &xBackfillAccountResult{}
	handler.runXBackfillPages(
		context.Background(), "workspace-1",
		db.SocialAccount{
			ID: "account-1", ExternalAccountID: "user-1",
			XAppMode: pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
			Scope:    []string{"tweet.read", "users.read"},
		},
		"token", adapter, "x_reply",
		xBackfillRequest{LookbackDays: 7, MaxItems: 5},
		time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC), "run-finalize", result,
	)
	wantUnits := xcredits.OperationWeight("post.read") + xcredits.OperationWeight("user.read")
	if credits.exposurePendingUnits != wantUnits || credits.exposureFinalizeCalls != 1 {
		t.Fatalf("pending/finalize = %d/%d, want %d/1", credits.exposurePendingUnits, credits.exposureFinalizeCalls, wantUnits)
	}
	if result.StopReason != "usage_reservation_settlement_failed" {
		t.Fatalf("result = %+v", result)
	}
}

func TestInboxXBackfillLostExecutionLeaseStopsBeforeReservationAndPaidRead(t *testing.T) {
	credits := &fakeXInboxCredits{}
	handler := &InboxHandler{xCredits: credits}
	adapter := &fakeXInboxBackfillAdapter{}
	result := &xBackfillAccountResult{}
	handler.runXBackfillPagesWithLease(
		context.Background(), "workspace-1",
		db.SocialAccount{
			ID: "account-1", ExternalAccountID: "user-1",
			XAppMode: pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
			Scope:    []string{"tweet.read", "users.read"},
		},
		"token", adapter, "x_reply",
		xBackfillRequest{LookbackDays: 7, MaxItems: 5},
		time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC), "run-fenced", result,
		func(context.Context) error { return errors.New("lease lost") },
	)
	if len(adapter.mentionPageSizes) != 0 {
		t.Fatalf("lost lease triggered paid X read: %v", adapter.mentionPageSizes)
	}
	if result.StopReason != "confirmation_execution_lease_lost" {
		t.Fatalf("result = %+v", result)
	}
}

func TestInboxXBackfillLostLeaseAfterReservationReleasesBeforePaidRead(t *testing.T) {
	credits := &fakeXInboxCredits{}
	handler := &InboxHandler{xCredits: credits}
	adapter := &fakeXInboxBackfillAdapter{}
	result := &xBackfillAccountResult{}
	checks := 0
	handler.runXBackfillPagesWithLease(
		context.Background(), "workspace-1",
		db.SocialAccount{
			ID: "account-1", ExternalAccountID: "user-1",
			XAppMode: pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
			Scope:    []string{"tweet.read", "users.read"},
		},
		"token", adapter, "x_reply",
		xBackfillRequest{LookbackDays: 7, MaxItems: 5},
		time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC), "run-refenced", result,
		func(context.Context) error {
			checks++
			if checks == 2 {
				return errors.New("lease lost after reservation")
			}
			return nil
		},
	)
	if checks != 2 || len(adapter.mentionPageSizes) != 0 || !credits.exposureReleased {
		t.Fatalf("checks=%d adapter=%v credits=%+v", checks, adapter.mentionPageSizes, credits)
	}
	if result.StopReason != "confirmation_execution_lease_lost" {
		t.Fatalf("result = %+v", result)
	}
}

func TestInboxXBackfillFailedLeaseReleaseRetainsExposureForReconciliation(t *testing.T) {
	credits := &fakeXInboxCredits{exposureReleaseErr: errors.New("database unavailable")}
	handler := &InboxHandler{xCredits: credits}
	adapter := &fakeXInboxBackfillAdapter{}
	result := &xBackfillAccountResult{}
	checks := 0
	handler.runXBackfillPagesWithLease(
		context.Background(), "workspace-1",
		db.SocialAccount{
			ID: "account-1", ExternalAccountID: "user-1",
			XAppMode: pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
			Scope:    []string{"tweet.read", "users.read"},
		},
		"token", adapter, "x_reply",
		xBackfillRequest{LookbackDays: 7, MaxItems: 5},
		time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC), "run-release-reconcile", result,
		func(context.Context) error {
			checks++
			if checks == 2 {
				return errors.New("lease lost after reservation")
			}
			return nil
		},
	)
	if len(adapter.mentionPageSizes) != 0 ||
		!credits.exposureReleased ||
		!credits.exposureReconciliation {
		t.Fatalf("adapter=%v credits=%+v", adapter.mentionPageSizes, credits)
	}
	if result.StopReason != "usage_reservation_release_needs_reconciliation" {
		t.Fatalf("result = %+v", result)
	}
}

func TestInboxXBackfillDMStillRunsWhenMentionsCannotMeetFiveItemMinimum(t *testing.T) {
	handler := &InboxHandler{xCredits: &fakeXInboxCredits{}}
	adapter := &fakeXInboxBackfillAdapter{}
	result := &xBackfillAccountResult{}
	account := db.SocialAccount{
		ID: "account-1", ExternalAccountID: "user-1",
		XAppMode: pgtype.Text{String: string(xinbox.AppModeWorkspace), Valid: true},
		Scope:    []string{"tweet.read", "users.read", "dm.read"},
	}
	request := xBackfillRequest{
		LookbackDays: 7, MaxItems: 1, IncludeReplies: true, IncludeDMs: true,
	}
	handler.runXBackfillPages(
		context.Background(), "workspace-1", account, "token", adapter, "x_reply",
		request, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC), "run-mixed", result,
	)
	handler.runXBackfillPages(
		context.Background(), "workspace-1", account, "token", adapter, "x_dm",
		request, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC), "run-mixed", result,
	)
	if len(adapter.mentionPageSizes) != 0 || !reflect.DeepEqual(adapter.dmPageSizes, []int{1}) {
		t.Fatalf("mentions=%v dms=%v", adapter.mentionPageSizes, adapter.dmPageSizes)
	}
}

func TestInboxXBackfillSettlesShortPageAndClassifiesReadErrors(t *testing.T) {
	account := db.SocialAccount{
		ID:                "account-1",
		ExternalAccountID: "user-1",
		XAppMode:          pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
		Scope:             []string{"tweet.read", "users.read"},
	}
	tests := []struct {
		name        string
		adapterErr  error
		releaseErr  error
		wantRelease bool
		wantManual  bool
	}{
		{name: "short page finalizes unused exposure"},
		{
			name:        "definitive read rejection releases exposure",
			adapterErr:  &platform.TwitterInboxHTTPError{Stage: "x_inbox_read", StatusCode: 403},
			wantRelease: true,
		},
		{
			name:        "definitive read rejection release failure is retried by worker",
			adapterErr:  &platform.TwitterInboxHTTPError{Stage: "x_inbox_read", StatusCode: 403},
			releaseErr:  errors.New("database unavailable"),
			wantRelease: true,
			wantManual:  true,
		},
		{
			name: "ambiguous server response retains exposure",
			adapterErr: &platform.TwitterInboxHTTPError{
				Stage: "x_inbox_read", StatusCode: 502, Retryable: true,
			},
			wantManual: true,
		},
		{
			name:       "paid 2xx decode failure retains exposure",
			adapterErr: errors.New("x_inbox_read: decode X inbox response: invalid character"),
			wantManual: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credits := &fakeXInboxCredits{exposureReleaseErr: tt.releaseErr}
			handler := &InboxHandler{xCredits: credits}
			adapter := &fakeXInboxBackfillAdapter{mentionErr: tt.adapterErr}
			result := &xBackfillAccountResult{}
			handler.runXBackfillPages(
				context.Background(), "workspace-1", account, "token", adapter, "x_reply",
				xBackfillRequest{LookbackDays: 7, MaxItems: 5},
				time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC), "run-settle", result,
			)
			if tt.adapterErr == nil && credits.exposureFinalizedUnits != 0 {
				t.Fatalf("short page actual units = %d, want 0", credits.exposureFinalizedUnits)
			}
			if credits.exposureReleased != tt.wantRelease ||
				credits.exposureReconciliation != tt.wantManual {
				t.Fatalf("credits = %+v", credits)
			}
		})
	}
}

func TestInboxXBackfillRetriesAmbiguousReadReconciliationPersistence(t *testing.T) {
	credits := &fakeXInboxCredits{
		exposureReconcileErrs: []error{errors.New("transient database failure"), nil},
	}
	handler := &InboxHandler{xCredits: credits}
	adapter := &fakeXInboxBackfillAdapter{mentionErr: &platform.TwitterInboxHTTPError{
		Stage: "x_inbox_read", StatusCode: 502, Retryable: true,
	}}
	result := &xBackfillAccountResult{}
	handler.runXBackfillPages(
		context.Background(), "workspace-1",
		db.SocialAccount{
			ID: "account-1", ExternalAccountID: "user-1",
			XAppMode: pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
			Scope:    []string{"tweet.read", "users.read"},
		},
		"token", adapter, "x_reply",
		xBackfillRequest{LookbackDays: 7, MaxItems: 5},
		time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC), "run-ambiguous", result,
	)
	if credits.exposureReconcileCalls != 2 || !credits.exposureReconciliation {
		t.Fatalf("credits = %+v", credits)
	}
	if result.StopReason != "upstream_read_failed" {
		t.Fatalf("result = %+v", result)
	}
}

type atomicExposureCredits struct {
	*fakeXInboxCredits
	mu                 sync.Mutex
	availableResources int
}

func (f *atomicExposureCredits) ReserveExposure(
	_ context.Context,
	req xcredits.ExposureReservationRequest,
) (xcredits.ExposureReservation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	resources := req.RequestedResources
	if resources > f.availableResources {
		resources = f.availableResources
	}
	if resources < req.MinimumResources {
		return xcredits.ExposureReservation{}, xcredits.ErrMonthlyLimitExceeded
	}
	f.availableResources -= resources
	return xcredits.ExposureReservation{
		ID:                 "reservation-" + req.IdempotencyKey,
		RequestedResources: req.RequestedResources,
		ReservedResources:  resources,
		ReservedUnits:      int64(resources) * req.UnitsPerResource,
	}, nil
}

type countingXInboxBackfillAdapter struct {
	requested atomic.Int64
}

func (a *countingXInboxBackfillAdapter) FetchInboxMentions(
	context.Context, string, string, time.Time, string, int,
) (platform.TwitterInboxPage, error) {
	panic("use countedFetchInboxMentions")
}

func (a *countingXInboxBackfillAdapter) countedFetchInboxMentions(
	_ context.Context, _ string, _ string, _ time.Time, _ string, maxResults int,
) (platform.TwitterInboxPage, error) {
	a.requested.Add(int64(maxResults))
	return platform.TwitterInboxPage{}, nil
}

func (a *countingXInboxBackfillAdapter) FetchInboxDMEvents(
	context.Context, string, time.Time, string, int,
) (platform.TwitterInboxPage, error) {
	return platform.TwitterInboxPage{}, nil
}

func (a *countingXInboxBackfillAdapter) SendInboxReply(context.Context, string, string, string) (*platform.PostResult, error) {
	return nil, errors.New("not used")
}

func (a *countingXInboxBackfillAdapter) SendInboxDMToConversation(context.Context, string, string, string) (*platform.TwitterDMSendResult, error) {
	return nil, errors.New("not used")
}

func (a *countingXInboxBackfillAdapter) SendInboxDMToParticipant(context.Context, string, string, string) (*platform.TwitterDMSendResult, error) {
	return nil, errors.New("not used")
}

type countedMentionsAdapter struct {
	*countingXInboxBackfillAdapter
}

func (a countedMentionsAdapter) FetchInboxMentions(
	ctx context.Context, token, user string, start time.Time, page string, max int,
) (platform.TwitterInboxPage, error) {
	return a.countedFetchInboxMentions(ctx, token, user, start, page, max)
}

func TestInboxXConcurrentBackfillsNeverRequestBeyondAtomicExposure(t *testing.T) {
	credits := &atomicExposureCredits{
		fakeXInboxCredits:  &fakeXInboxCredits{},
		availableResources: 5,
	}
	handler := &InboxHandler{xCredits: credits}
	counter := &countingXInboxBackfillAdapter{}
	account := db.SocialAccount{
		ID:                "account-1",
		ExternalAccountID: "user-1",
		XAppMode:          pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
		Scope:             []string{"tweet.read", "users.read"},
	}
	var wait sync.WaitGroup
	for index := range 2 {
		wait.Add(1)
		go func(runID string) {
			defer wait.Done()
			result := &xBackfillAccountResult{}
			handler.runXBackfillPages(
				context.Background(), "workspace-1", account, "token",
				countedMentionsAdapter{counter}, "x_reply",
				xBackfillRequest{LookbackDays: 7, MaxItems: 5},
				time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC), runID, result,
			)
		}(fmt.Sprintf("run-%d", index))
	}
	wait.Wait()
	if got := counter.requested.Load(); got != 5 {
		t.Fatalf("total upstream requested exposure = %d, want 5", got)
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
		"run-horizon",
		result,
	)
	if !reflect.DeepEqual(adapter.dmTokens, []string{""}) {
		t.Fatalf("pagination tokens = %v, want one initial request", adapter.dmTokens)
	}
}

func TestInboxXReplyReservesAndClassifiesSummonedReply(t *testing.T) {
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
	if adapter.replyCalls != 1 || credits.reversedID != "" {
		t.Fatalf("adapter/settlement = %+v %+v", adapter, credits)
	}
	if credits.reserve.OperationKey != "post.reply_summoned" ||
		credits.reserve.IdempotencyKey != "inbox:item-1:client-key" {
		t.Fatalf("reserve = %+v", credits.reserve)
	}
	if result.XCreditsCounted != 10 || result.Operation != "post.reply_summoned" ||
		result.CatalogVersion != xcredits.CatalogVersion ||
		result.BillingMode != string(xinbox.AppModeUniPostManaged) ||
		result.UsageEventID != "usage-1" {
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
	if credits.reversedID != "usage-1" {
		t.Fatalf("settlement = %+v", credits)
	}
}

func TestInboxXConfirmedFailureRetainsOperationWhenUsageReversalFails(t *testing.T) {
	adapter := &fakeFailingXInboxReplyAdapter{
		err: errors.New("X inbox API returned HTTP 403"),
	}
	credits := &fakeXInboxCredits{
		event: xcredits.UsageEvent{
			ID: "usage-1", Status: xcredits.UsageStatusProvisional,
			OperationKey: "post.reply_summoned", CatalogVersion: xcredits.CatalogVersion,
			WeightedUnits: 10,
		},
		reverseErr: errors.New("database unavailable"),
	}
	account := db.SocialAccount{
		ID: "account-1", Platform: "twitter",
		XAppMode: pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
	}
	item := db.InboxItem{
		ID: "item-1", Source: "x_reply", ExternalID: "tweet-1",
		Metadata: []byte(`{"reply_eligible":true}`),
	}
	result, err := sendXInboxReply(
		context.Background(), adapter, credits, "workspace-1", account, item,
		"user-token", "thanks", "client-key",
	)
	if !errors.Is(err, ErrXUsageReversalPending) {
		t.Fatalf("err = %v, want ErrXUsageReversalPending", err)
	}
	if result.UsageEventID != "usage-1" || result.XCreditsCounted != 10 {
		t.Fatalf("result = %+v", result)
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
	if credits.reversedID != "" {
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
			if credits.reversedID != "" {
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

type inboxFeatureFlags bool

func (f inboxFeatureFlags) ForWorkspace(context.Context, string, string) (bool, error) {
	return bool(f), nil
}

func TestInboxXDMSGateUsesWorkspaceFeatureEvaluation(t *testing.T) {
	handler := (&InboxHandler{}).SetFeatureFlags(inboxFeatureFlags(false))
	available, err := handler.xDMsAvailable(context.Background(), "workspace-1")
	if err != nil {
		t.Fatal(err)
	}
	if available {
		t.Fatal("xDMsAvailable = true, want false")
	}

	recorder := httptest.NewRecorder()
	handler.writeXDMSUnavailable(recorder)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "FEATURE_NOT_AVAILABLE") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestInboxTenantIsolationMediaContextScopesAccountLookup(t *testing.T) {
	store := &inboxTenantIsolationDB{
		item: db.InboxItem{
			ID:              "item-1",
			SocialAccountID: "account-1",
			WorkspaceID:     "workspace-1",
			Source:          "ig_comment",
			ExternalID:      "comment-1",
		},
	}
	recorder := httptest.NewRecorder()
	request := inboxTenantIsolationRequest(http.MethodGet, "/v1/inbox/item-1/media-context", "workspace-1", "id", "item-1")

	NewInboxHandler(db.New(store), nil, nil).MediaContext(recorder, request)

	if !store.called("-- name: GetSocialAccountByIDAndWorkspace") {
		t.Fatal("MediaContext did not use the workspace-scoped social-account lookup")
	}
	if store.called("-- name: GetSocialAccount :one") {
		t.Fatal("MediaContext used the unscoped social-account lookup")
	}
}

func TestInboxTenantIsolationReplyReturnsNotFoundWhenDerivedLookupRejectsTarget(t *testing.T) {
	store := &inboxTenantIsolationDB{itemErr: pgx.ErrNoRows}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/inbox/item-1/reply", strings.NewReader(`{"text":"hello"}`))
	ctx := auth.SetWorkspaceID(request.Context(), "workspace-1")
	ctx = inboxaccess.WithContext(ctx, inboxaccess.Scope{WorkspaceID: "workspace-1", Mode: inboxaccess.ModeWorkspace})
	request = request.WithContext(ctx)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", "item-1")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))

	NewInboxHandler(db.New(store), nil, nil).Reply(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", recorder.Code, recorder.Body.String())
	}
	if !store.called("-- name: GetInboxItem") {
		t.Fatal("Reply did not use the derived-workspace Inbox lookup")
	}
	if store.called("-- name: GetSocialAccount") {
		t.Fatal("Reply continued to account or adapter work after target rejection")
	}
}

func TestInboxManagedScopePropagatesToEveryHTTPQuery(t *testing.T) {
	tests := []struct {
		name               string
		request            func(method, target, param, value string) *http.Request
		wantWorkspaceScope bool
		wantExternalUserID string
	}{
		{
			name:               "managed user",
			request:            managedInboxRequest,
			wantWorkspaceScope: false,
			wantExternalUserID: "managed-a",
		},
		{
			name:               "workspace aggregate",
			request:            workspaceInboxRequest,
			wantWorkspaceScope: true,
			wantExternalUserID: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := db.InboxItem{
				ID:               "item-a",
				SocialAccountID:  "account-a",
				WorkspaceID:      "workspace-1",
				Source:           "unsupported",
				ExternalID:       "external-a",
				ParentExternalID: pgtype.Text{String: "media-a", Valid: true},
				ThreadKey:        "thread-a",
				ThreadStatus:     "open",
			}
			outbound := db.XInboxOutboundRequest{
				ID:              "operation-a",
				WorkspaceID:     "workspace-1",
				SocialAccountID: "account-a",
				InboxItemID:     "item-a",
				Status:          "pending",
			}

			routes := []struct {
				name        string
				method      string
				target      string
				param       string
				value       string
				body        string
				handle      func(*InboxHandler, http.ResponseWriter, *http.Request)
				wantMarkers []string
			}{
				{name: "list", method: http.MethodGet, target: "/v1/inbox", handle: func(h *InboxHandler, w http.ResponseWriter, r *http.Request) { h.List(w, r) }, wantMarkers: []string{"-- name: ListInboxItemsByWorkspace"}},
				{name: "unread count", method: http.MethodGet, target: "/v1/inbox/unread-count", handle: func(h *InboxHandler, w http.ResponseWriter, r *http.Request) { h.UnreadCount(w, r) }, wantMarkers: []string{"-- name: CountUnreadByWorkspace"}},
				{name: "get", method: http.MethodGet, target: "/v1/inbox/item-a", param: "id", value: "item-a", handle: func(h *InboxHandler, w http.ResponseWriter, r *http.Request) { h.Get(w, r) }, wantMarkers: []string{"-- name: GetInboxItem"}},
				{name: "mark read", method: http.MethodPost, target: "/v1/inbox/item-a/read", param: "id", value: "item-a", handle: func(h *InboxHandler, w http.ResponseWriter, r *http.Request) { h.MarkRead(w, r) }, wantMarkers: []string{"-- name: GetInboxItem", "-- name: MarkInboxItemRead"}},
				{name: "mark all read", method: http.MethodPost, target: "/v1/inbox/mark-all-read", handle: func(h *InboxHandler, w http.ResponseWriter, r *http.Request) { h.MarkAllRead(w, r) }, wantMarkers: []string{"-- name: MarkAllInboxItemsRead"}},
				{name: "media context", method: http.MethodGet, target: "/v1/inbox/item-a/media-context", param: "id", value: "item-a", handle: func(h *InboxHandler, w http.ResponseWriter, r *http.Request) { h.MediaContext(w, r) }, wantMarkers: []string{"-- name: GetInboxItem"}},
				{name: "reply", method: http.MethodPost, target: "/v1/inbox/item-a/reply", param: "id", value: "item-a", body: `{"text":"managed-scope-test-body"}`, handle: func(h *InboxHandler, w http.ResponseWriter, r *http.Request) { h.Reply(w, r) }, wantMarkers: []string{"-- name: GetInboxItem"}},
				{name: "X outbound status", method: http.MethodGet, target: "/v1/inbox/x/outbound/operation-a", param: "requestID", value: "operation-a", handle: func(h *InboxHandler, w http.ResponseWriter, r *http.Request) { h.XOutboundStatus(w, r) }, wantMarkers: []string{"-- name: GetInboxItem"}},
				{name: "thread state", method: http.MethodPost, target: "/v1/inbox/item-a/thread-state", param: "id", value: "item-a", body: `{"thread_status":"resolved"}`, handle: func(h *InboxHandler, w http.ResponseWriter, r *http.Request) { h.UpdateThreadState(w, r) }, wantMarkers: []string{"-- name: GetInboxItem", "-- name: UpdateInboxThreadState"}},
			}

			for _, route := range routes {
				t.Run(route.name, func(t *testing.T) {
					store := &inboxTenantIsolationDB{item: item, outbound: outbound}
					handler := NewInboxHandler(db.New(store), nil, nil)
					recorder := httptest.NewRecorder()
					request := test.request(route.method, route.target, route.param, route.value)
					if route.body != "" {
						request.Body = io.NopCloser(strings.NewReader(route.body))
					}

					route.handle(handler, recorder, request)

					for _, marker := range route.wantMarkers {
						calls := store.callsFor(marker)
						if len(calls) == 0 {
							t.Fatalf("%s was not called; status=%d body=%s", marker, recorder.Code, recorder.Body.String())
						}
						for _, call := range calls {
							workspaceScope, externalUserID := inboxTenantIsolationScopeArgs(t, marker, call.args)
							if workspaceScope != test.wantWorkspaceScope || externalUserID != test.wantExternalUserID {
								t.Fatalf("%s scope = (%v, %q), want (%v, %q); args=%#v", marker, workspaceScope, externalUserID, test.wantWorkspaceScope, test.wantExternalUserID, call.args)
							}
						}
					}
				})
			}
		})
	}
}

func TestInboxManagedScopeObjectDenialsStopBeforeSensitiveWork(t *testing.T) {
	baseOutbound := db.XInboxOutboundRequest{
		ID:              "operation-b",
		WorkspaceID:     "workspace-1",
		SocialAccountID: "account-b",
		InboxItemID:     "item-b",
		Status:          "pending",
	}
	routes := []struct {
		name           string
		method         string
		target         string
		param          string
		value          string
		body           string
		routeClass     string
		handle         func(*InboxHandler, http.ResponseWriter, *http.Request)
		allowedMarkers []string
	}{
		{name: "get", method: http.MethodGet, target: "/v1/inbox/item-b", param: "id", value: "item-b", routeClass: "get", handle: func(h *InboxHandler, w http.ResponseWriter, r *http.Request) { h.Get(w, r) }, allowedMarkers: []string{"-- name: GetInboxItem"}},
		{name: "media", method: http.MethodGet, target: "/v1/inbox/item-b/media-context", param: "id", value: "item-b", routeClass: "media_context", handle: func(h *InboxHandler, w http.ResponseWriter, r *http.Request) { h.MediaContext(w, r) }, allowedMarkers: []string{"-- name: GetInboxItem"}},
		{name: "mark read", method: http.MethodPost, target: "/v1/inbox/item-b/read", param: "id", value: "item-b", routeClass: "mark_read", handle: func(h *InboxHandler, w http.ResponseWriter, r *http.Request) { h.MarkRead(w, r) }, allowedMarkers: []string{"-- name: GetInboxItem"}},
		{name: "reply", method: http.MethodPost, target: "/v1/inbox/item-b/reply", param: "id", value: "item-b", body: `{"text":"managed-scope-test-body"}`, routeClass: "reply", handle: func(h *InboxHandler, w http.ResponseWriter, r *http.Request) { h.Reply(w, r) }, allowedMarkers: []string{"-- name: GetInboxItem"}},
		{name: "reply malformed body", method: http.MethodPost, target: "/v1/inbox/item-b/reply", param: "id", value: "item-b", body: `{`, routeClass: "reply", handle: func(h *InboxHandler, w http.ResponseWriter, r *http.Request) { h.Reply(w, r) }, allowedMarkers: []string{"-- name: GetInboxItem"}},
		{name: "thread", method: http.MethodPost, target: "/v1/inbox/item-b/thread-state", param: "id", value: "item-b", body: `{"thread_status":"resolved"}`, routeClass: "thread_state", handle: func(h *InboxHandler, w http.ResponseWriter, r *http.Request) { h.UpdateThreadState(w, r) }, allowedMarkers: []string{"-- name: GetInboxItem"}},
		{name: "thread malformed body", method: http.MethodPost, target: "/v1/inbox/item-b/thread-state", param: "id", value: "item-b", body: `{`, routeClass: "thread_state", handle: func(h *InboxHandler, w http.ResponseWriter, r *http.Request) { h.UpdateThreadState(w, r) }, allowedMarkers: []string{"-- name: GetInboxItem"}},
		{name: "X outbound status", method: http.MethodGet, target: "/v1/inbox/x/outbound/operation-b", param: "requestID", value: "operation-b", routeClass: "x_outbound_status", handle: func(h *InboxHandler, w http.ResponseWriter, r *http.Request) { h.XOutboundStatus(w, r) }, allowedMarkers: []string{"-- name: GetXInboxOutboundRequestByID", "-- name: GetInboxItem"}},
	}

	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			store := &inboxTenantIsolationDB{itemErr: pgx.ErrNoRows, outbound: baseOutbound}
			adapterCalls := 0
			handler := NewInboxHandler(db.New(store), nil, nil)
			handler.xAdapterFactory = func() xInboxBackfillAdapter {
				adapterCalls++
				return &fakeXInboxBackfillAdapter{}
			}
			recorder := httptest.NewRecorder()
			request := managedInboxRequest(route.method, route.target, route.param, route.value)
			if route.body != "" {
				request.Body = io.NopCloser(strings.NewReader(route.body))
			}

			var logs bytes.Buffer
			previousLogger := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			defer slog.SetDefault(previousLogger)

			route.handle(handler, recorder, request)

			if recorder.Code != http.StatusNotFound {
				t.Fatalf("status=%d, want 404; body=%s", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), `"code":"NOT_FOUND"`) {
				t.Fatalf("unstable public denial envelope: %s", recorder.Body.String())
			}
			if adapterCalls != 0 || store.execCalls != 0 {
				t.Fatalf("sensitive work after scoped denial: adapter=%d exec=%d calls=%#v", adapterCalls, store.execCalls, store.calls)
			}
			if got, want := store.callMarkers(), route.allowedMarkers; !reflect.DeepEqual(got, want) {
				t.Fatalf("DB calls after scoped denial = %#v, want %#v", got, want)
			}

			logText := logs.String()
			if strings.Count(logText, `"msg":"inbox_scope_object_rejected"`) != 1 ||
				!strings.Contains(logText, `"workspace_id":"workspace-1"`) ||
				!strings.Contains(logText, `"route_class":"`+route.routeClass+`"`) ||
				!strings.Contains(logText, `"scope_mode":"managed_user"`) {
				t.Fatalf("sanitized denial log missing or duplicated: %s", logText)
			}
			for _, forbidden := range []string{"item-b", "operation-b", "managed-a", "managed-scope-test-body", "external_user_id", "request_id"} {
				if strings.Contains(logText, forbidden) {
					t.Fatalf("denial log contains forbidden value %q: %s", forbidden, logText)
				}
			}
		})
	}
}

func TestInboxManagedScopeMalformedPayloadInScopeRemainsBadRequest(t *testing.T) {
	routes := []struct {
		name   string
		target string
		handle func(*InboxHandler, http.ResponseWriter, *http.Request)
	}{
		{name: "reply", target: "/v1/inbox/item-a/reply", handle: func(h *InboxHandler, w http.ResponseWriter, r *http.Request) { h.Reply(w, r) }},
		{name: "thread state", target: "/v1/inbox/item-a/thread-state", handle: func(h *InboxHandler, w http.ResponseWriter, r *http.Request) { h.UpdateThreadState(w, r) }},
	}
	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			store := &inboxTenantIsolationDB{item: db.InboxItem{
				ID:              "item-a",
				SocialAccountID: "account-a",
				WorkspaceID:     "workspace-1",
				Source:          "ig_comment",
			}}
			recorder := httptest.NewRecorder()
			request := managedInboxRequest(http.MethodPost, route.target, "id", "item-a")
			request.Body = io.NopCloser(strings.NewReader(`{`))

			route.handle(NewInboxHandler(db.New(store), nil, nil), recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status=%d, want 400; body=%s", recorder.Code, recorder.Body.String())
			}
			calls := store.callsFor("-- name: GetInboxItem")
			if len(calls) != 1 {
				t.Fatalf("scoped authorization calls=%d, want 1; calls=%#v", len(calls), store.calls)
			}
			workspaceScope, externalUserID := inboxTenantIsolationScopeArgs(t, "-- name: GetInboxItem", calls[0].args)
			if workspaceScope || externalUserID != "managed-a" {
				t.Fatalf("authorization scope=(%v,%q), want (false,managed-a)", workspaceScope, externalUserID)
			}
		})
	}
}

func TestInboxManagedScopeReplyCompletedIdempotentReloadPreservesExactScope(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	encryptedToken, err := encryptor.Encrypt("x-access-token")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name               string
		request            func(method, target, param, value string) *http.Request
		wantWorkspaceScope bool
		wantExternalUserID string
	}{
		{name: "managed user", request: managedInboxRequest, wantExternalUserID: "managed-a"},
		{name: "workspace aggregate", request: workspaceInboxRequest, wantWorkspaceScope: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target := db.InboxItem{
				ID:              "item-a",
				SocialAccountID: "account-a",
				WorkspaceID:     "workspace-1",
				Source:          "x_reply",
				ExternalID:      "tweet-a",
				Metadata:        []byte(`{"reply_eligible":true}`),
			}
			replayed := db.InboxItem{
				ID:              "reply-a",
				SocialAccountID: "account-a",
				WorkspaceID:     "workspace-1",
				Source:          "x_reply",
				ExternalID:      "tweet-reply-a",
				IsOwn:           true,
			}
			store := &inboxTenantIsolationDB{
				items:    []db.InboxItem{target, replayed},
				claimErr: pgx.ErrNoRows,
				account: db.SocialAccount{
					ID:                "account-a",
					ProfileID:         "profile-a",
					Platform:          "twitter",
					AccessToken:       encryptedToken,
					ExternalAccountID: "x-account-a",
					Scope:             []string{"tweet.read", "tweet.write", "users.read"},
					Status:            "active",
					ConnectionType:    "managed",
				},
				outbound: db.XInboxOutboundRequest{
					ID:                     "operation-a",
					WorkspaceID:            "workspace-1",
					SocialAccountID:        "account-a",
					InboxItemID:            "item-a",
					IdempotencyKey:         "idempotency-a",
					PayloadHash:            xInboxReplyPayloadHash(target, "hello"),
					Status:                 "completed",
					ResponseInboxItemID:    pgtype.Text{String: "reply-a", Valid: true},
					ReconciliationDeadline: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
				},
			}
			recorder := httptest.NewRecorder()
			request := test.request(http.MethodPost, "/v1/inbox/item-a/reply", "id", "item-a")
			request.Body = io.NopCloser(strings.NewReader(`{"text":"hello"}`))
			request.Header.Set("Idempotency-Key", "idempotency-a")

			NewInboxHandler(db.New(store), encryptor, nil).Reply(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status=%d, want 200; body=%s calls=%#v", recorder.Code, recorder.Body.String(), store.calls)
			}
			calls := store.callsFor("-- name: GetInboxItem")
			if len(calls) != 2 {
				t.Fatalf("GetInboxItem calls=%d, want initial+reload; calls=%#v", len(calls), store.calls)
			}
			for index, call := range calls {
				workspaceScope, externalUserID := inboxTenantIsolationScopeArgs(t, "-- name: GetInboxItem", call.args)
				if workspaceScope != test.wantWorkspaceScope || externalUserID != test.wantExternalUserID {
					t.Fatalf("GetInboxItem[%d] scope=(%v,%q), want (%v,%q)", index, workspaceScope, externalUserID, test.wantWorkspaceScope, test.wantExternalUserID)
				}
			}
		})
	}
}

func TestInboxManagedScopeCompleteKnownOutboundBranchesPreserveExactScope(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	encryptedPayload, err := encryptor.Encrypt("reply body")
	if err != nil {
		t.Fatal(err)
	}
	managedContext := managedInboxRequest(http.MethodGet, "/v1/inbox", "", "").Context()

	t.Run("completed response reload", func(t *testing.T) {
		tx := &inboxCompleteKnownTx{
			outbound: db.XInboxOutboundRequest{
				ID:                  "operation-a",
				WorkspaceID:         "workspace-1",
				SocialAccountID:     "account-a",
				InboxItemID:         "item-a",
				Status:              "completed",
				ResponseInboxItemID: pgtype.Text{String: "reply-a", Valid: true},
			},
			item: db.InboxItem{ID: "reply-a", SocialAccountID: "account-a", WorkspaceID: "workspace-1"},
		}

		item, _, err := (&InboxHandler{encryptor: encryptor}).completeKnownXInboxOutboundWithTx(managedContext, "operation-a", tx)
		if err != nil || item.ID != "reply-a" {
			t.Fatalf("completed reload = %+v, %v", item, err)
		}
		assertInboxCompleteKnownScope(t, tx, false, "managed-a")
		if !tx.committed {
			t.Fatal("completed branch did not commit")
		}
	})

	t.Run("remote succeeded target reload", func(t *testing.T) {
		tx := &inboxCompleteKnownTx{
			outbound: db.XInboxOutboundRequest{
				ID:                   "operation-a",
				WorkspaceID:          "workspace-1",
				SocialAccountID:      "account-a",
				InboxItemID:          "item-a",
				Status:               "remote_succeeded",
				EncryptedPayload:     pgtype.Text{String: encryptedPayload, Valid: true},
				RemoteExternalID:     pgtype.Text{String: "tweet-reply-a", Valid: true},
				RemoteConversationID: pgtype.Text{String: "conversation-a", Valid: true},
			},
			itemErr: pgx.ErrNoRows,
		}

		_, _, err := (&InboxHandler{encryptor: encryptor}).completeKnownXInboxOutboundWithTx(managedContext, "operation-a", tx)
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("remote target reload error=%v, want pgx.ErrNoRows", err)
		}
		assertInboxCompleteKnownScope(t, tx, false, "managed-a")
		if tx.committed {
			t.Fatal("remote target miss committed")
		}
	})
}

func TestInboxManagedScopeCompleteKnownOutboundMissingContextFailsClosed(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	tx := &inboxCompleteKnownTx{
		outbound: db.XInboxOutboundRequest{
			ID:                  "operation-a",
			WorkspaceID:         "workspace-1",
			SocialAccountID:     "account-a",
			InboxItemID:         "item-a",
			Status:              "completed",
			ResponseInboxItemID: pgtype.Text{String: "reply-a", Valid: true},
		},
		item:               db.InboxItem{ID: "reply-a", SocialAccountID: "account-a", WorkspaceID: "workspace-1"},
		rejectMissingScope: true,
	}

	_, _, err = (&InboxHandler{encryptor: encryptor}).completeKnownXInboxOutboundWithTx(context.Background(), "operation-a", tx)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing-scope completion error=%v, want pgx.ErrNoRows", err)
	}
	assertInboxCompleteKnownScope(t, tx, false, "")
	if tx.committed {
		t.Fatal("missing-scope completion committed")
	}
}

func assertInboxCompleteKnownScope(t *testing.T, tx *inboxCompleteKnownTx, wantWorkspaceScope bool, wantExternalUserID string) {
	t.Helper()
	if len(tx.getInboxCalls) != 1 {
		t.Fatalf("GetInboxItem calls=%d, want 1", len(tx.getInboxCalls))
	}
	workspaceScope, externalUserID := inboxTenantIsolationScopeArgs(t, "-- name: GetInboxItem", tx.getInboxCalls[0])
	if workspaceScope != wantWorkspaceScope || externalUserID != wantExternalUserID {
		t.Fatalf("GetInboxItem scope=(%v,%q), want (%v,%q)", workspaceScope, externalUserID, wantWorkspaceScope, wantExternalUserID)
	}
}

func TestInboxManagedScopeMissingContextFailsClosed(t *testing.T) {
	store := &inboxTenantIsolationDB{itemErr: pgx.ErrNoRows}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/inbox/item-a", nil)
	request = request.WithContext(auth.SetWorkspaceID(request.Context(), "workspace-1"))
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", "item-a")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))

	NewInboxHandler(db.New(store), nil, nil).Get(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404; body=%s", recorder.Code, recorder.Body.String())
	}
	calls := store.callsFor("-- name: GetInboxItem")
	if len(calls) != 1 {
		t.Fatalf("GetInboxItem calls=%d, want 1", len(calls))
	}
	workspaceScope, externalUserID := inboxTenantIsolationScopeArgs(t, "-- name: GetInboxItem", calls[0].args)
	if workspaceScope || externalUserID != "" {
		t.Fatalf("missing scope widened to (%v, %q)", workspaceScope, externalUserID)
	}
}

func TestXOutboundStatusTenantIsolation(t *testing.T) {
	now := pgtype.Timestamptz{Time: time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC), Valid: true}
	baseOutbound := db.XInboxOutboundRequest{
		ID:                 "request-1",
		WorkspaceID:        "workspace-1",
		SocialAccountID:    "account-1",
		InboxItemID:        "item-1",
		IdempotencyKey:     "key-1",
		PayloadHash:        "hash-1",
		Status:             "needs_reconciliation",
		EncryptedPayload:   pgtype.Text{String: "ciphertext", Valid: true},
		CreatedAt:          now,
		UpdatedAt:          now,
		NextAttemptAt:      now,
		CompletionAttempts: 2,
	}

	tests := []struct {
		name       string
		target     db.InboxItem
		targetErr  error
		wantStatus int
	}{
		{
			name:       "missing derived target fails closed",
			targetErr:  pgx.ErrNoRows,
			wantStatus: http.StatusNotFound,
		},
		{
			name: "mismatched target account fails closed",
			target: db.InboxItem{
				ID:              "item-1",
				WorkspaceID:     "workspace-1",
				SocialAccountID: "account-2",
				Source:          "x_reply",
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "matching target returns safe status",
			target: db.InboxItem{
				ID:              "item-1",
				WorkspaceID:     "workspace-1",
				SocialAccountID: "account-1",
				Source:          "x_reply",
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &inboxTenantIsolationDB{
				outbound: baseOutbound,
				item:     test.target,
				itemErr:  test.targetErr,
			}
			recorder := httptest.NewRecorder()
			request := inboxTenantIsolationRequest(http.MethodGet, "/v1/inbox/x/outbound/request-1", "workspace-1", "requestID", "request-1")

			NewInboxHandler(db.New(store), nil, nil).XOutboundStatus(recorder, request)

			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), "ciphertext") || strings.Contains(recorder.Body.String(), "encrypted_payload") {
				t.Fatalf("response exposed protected payload: %s", recorder.Body.String())
			}
		})
	}
}

type inboxTenantIsolationQueryCall struct {
	query string
	args  []interface{}
}

type inboxTenantIsolationDB struct {
	item      db.InboxItem
	items     []db.InboxItem
	itemErr   error
	itemCalls int
	account   db.SocialAccount
	outbound  db.XInboxOutboundRequest
	claimErr  error
	calls     []inboxTenantIsolationQueryCall
	execCalls int
}

func (f *inboxTenantIsolationDB) Exec(_ context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	f.calls = append(f.calls, inboxTenantIsolationQueryCall{query: query, args: args})
	f.execCalls++
	switch {
	case strings.Contains(query, "-- name: MarkInboxItemRead"),
		strings.Contains(query, "-- name: MarkAllInboxItemsRead"),
		strings.Contains(query, "-- name: UpdateInboxThreadState"):
		return pgconn.NewCommandTag("UPDATE 1"), nil
	default:
		return pgconn.CommandTag{}, errors.New("unexpected Exec call")
	}
}

func (f *inboxTenantIsolationDB) Query(_ context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	f.calls = append(f.calls, inboxTenantIsolationQueryCall{query: query, args: args})
	if strings.Contains(query, "-- name: ListInboxItemsByWorkspace") {
		return &metaWebhookRoutingRows{}, nil
	}
	return nil, errors.New("unexpected Query call")
}

func (f *inboxTenantIsolationDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	f.calls = append(f.calls, inboxTenantIsolationQueryCall{query: query, args: args})
	switch {
	case strings.Contains(query, "-- name: ClaimXInboxOutboundRequest"):
		return metaWebhookRoutingRow{err: f.claimErr}
	case strings.Contains(query, "-- name: GetXInboxOutboundRequestByID"):
		return metaWebhookRoutingRow{values: inboxTenantIsolationOutboundValues(f.outbound)}
	case strings.Contains(query, "-- name: GetXInboxOutboundRequest"):
		return metaWebhookRoutingRow{values: inboxTenantIsolationOutboundValues(f.outbound)}
	case strings.Contains(query, "-- name: CountUnreadByWorkspace"):
		return metaWebhookRoutingRow{values: []any{int32(0)}}
	case strings.Contains(query, "-- name: GetInboxMediaCache"):
		return metaWebhookRoutingRow{values: []any{
			"https://example.invalid/media.jpg",
			"caption",
			"2026-07-20T00:00:00Z",
			"IMAGE",
			"https://example.invalid/post",
			pgtype.Timestamptz{Time: time.Now(), Valid: true},
		}}
	case strings.Contains(query, "-- name: GetInboxItem"):
		if f.itemErr != nil {
			return metaWebhookRoutingRow{err: f.itemErr}
		}
		if len(f.items) > 0 {
			if f.itemCalls >= len(f.items) {
				return metaWebhookRoutingRow{err: pgx.ErrNoRows}
			}
			item := f.items[f.itemCalls]
			f.itemCalls++
			return metaWebhookRoutingRow{values: inboxTenantIsolationItemValues(item)}
		}
		return metaWebhookRoutingRow{values: inboxTenantIsolationItemValues(f.item)}
	case strings.Contains(query, "-- name: GetSocialAccountByIDAndWorkspace"):
		if f.account.ID != "" {
			return metaWebhookRoutingRow{values: inboxTenantIsolationSocialAccountValues(f.account)}
		}
		return metaWebhookRoutingRow{err: pgx.ErrNoRows}
	case strings.Contains(query, "-- name: GetSocialAccount :one"):
		return metaWebhookRoutingRow{err: pgx.ErrNoRows}
	default:
		return metaWebhookRoutingRow{err: errors.New("unexpected QueryRow call")}
	}
}

func (f *inboxTenantIsolationDB) called(marker string) bool {
	for _, call := range f.calls {
		if strings.Contains(call.query, marker) {
			return true
		}
	}
	return false
}

func (f *inboxTenantIsolationDB) callsFor(marker string) []inboxTenantIsolationQueryCall {
	var calls []inboxTenantIsolationQueryCall
	for _, call := range f.calls {
		if strings.Contains(call.query, marker) {
			calls = append(calls, call)
		}
	}
	return calls
}

func (f *inboxTenantIsolationDB) callMarkers() []string {
	markers := make([]string, 0, len(f.calls))
	for _, call := range f.calls {
		firstLine := strings.SplitN(call.query, "\n", 2)[0]
		fields := strings.Fields(strings.TrimPrefix(firstLine, "-- name: "))
		if len(fields) == 0 {
			markers = append(markers, firstLine)
			continue
		}
		markers = append(markers, "-- name: "+fields[0])
	}
	return markers
}

func inboxTenantIsolationScopeArgs(t *testing.T, marker string, args []interface{}) (bool, string) {
	t.Helper()
	var workspaceIndex, externalUserIndex int
	switch marker {
	case "-- name: ListInboxItemsByWorkspace":
		workspaceIndex, externalUserIndex = 2, 3
	case "-- name: CountUnreadByWorkspace", "-- name: MarkAllInboxItemsRead":
		workspaceIndex, externalUserIndex = 1, 2
	case "-- name: GetInboxItem", "-- name: MarkInboxItemRead":
		workspaceIndex, externalUserIndex = 2, 3
	case "-- name: UpdateInboxThreadState":
		workspaceIndex, externalUserIndex = 6, 7
	default:
		t.Fatalf("no scope argument mapping for %s", marker)
	}
	if len(args) <= externalUserIndex {
		t.Fatalf("%s args too short: %#v", marker, args)
	}
	workspaceScope, ok := args[workspaceIndex].(bool)
	if !ok {
		t.Fatalf("%s workspace scope type = %T, want bool", marker, args[workspaceIndex])
	}
	externalUserID, ok := args[externalUserIndex].(string)
	if !ok {
		t.Fatalf("%s external user type = %T, want string", marker, args[externalUserIndex])
	}
	return workspaceScope, externalUserID
}

func managedInboxRequest(method, target, param, value string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	ctx := auth.SetWorkspaceID(request.Context(), "workspace-1")
	ctx = inboxaccess.WithContext(ctx, inboxaccess.Scope{
		WorkspaceID:    "workspace-1",
		Mode:           inboxaccess.ModeManagedUser,
		ExternalUserID: "managed-a",
	})
	request = request.WithContext(ctx)
	if param == "" {
		return request
	}
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add(param, value)
	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
}

func workspaceInboxRequest(method, target, param, value string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	ctx := auth.SetWorkspaceID(request.Context(), "workspace-1")
	ctx = inboxaccess.WithContext(ctx, inboxaccess.Scope{WorkspaceID: "workspace-1", Mode: inboxaccess.ModeWorkspace})
	request = request.WithContext(ctx)
	if param == "" {
		return request
	}
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add(param, value)
	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
}

func inboxTenantIsolationRequest(method, target, workspaceID, param, value string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	ctx := auth.SetWorkspaceID(request.Context(), workspaceID)
	ctx = inboxaccess.WithContext(ctx, inboxaccess.Scope{WorkspaceID: workspaceID, Mode: inboxaccess.ModeWorkspace})
	request = request.WithContext(ctx)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add(param, value)
	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
}

func inboxTenantIsolationItemValues(item db.InboxItem) []any {
	return []any{
		item.ID,
		item.SocialAccountID,
		item.WorkspaceID,
		item.Source,
		item.ExternalID,
		item.ParentExternalID,
		item.AuthorName,
		item.AuthorID,
		item.AuthorAvatarUrl,
		item.Body,
		item.IsRead,
		item.IsOwn,
		item.ReceivedAt,
		item.CreatedAt,
		item.Metadata,
		item.ThreadKey,
		item.ThreadStatus,
		item.AssignedTo,
		item.LinkedPostID,
	}
}

func inboxTenantIsolationOutboundValues(outbound db.XInboxOutboundRequest) []any {
	return []any{
		outbound.ID,
		outbound.WorkspaceID,
		outbound.SocialAccountID,
		outbound.InboxItemID,
		outbound.IdempotencyKey,
		outbound.PayloadHash,
		outbound.Status,
		outbound.ResponseInboxItemID,
		outbound.CreatedAt,
		outbound.UpdatedAt,
		outbound.EncryptedPayload,
		outbound.BodyHash,
		outbound.UsageEventID,
		outbound.OperationKey,
		outbound.ReservedUnits,
		outbound.RemoteExternalID,
		outbound.RemoteConversationID,
		outbound.RemoteUrl,
		outbound.SendStartedAt,
		outbound.RemoteOutcomeKnownAt,
		outbound.ReconciliationDeadline,
		outbound.CompletionAttempts,
		outbound.NextAttemptAt,
		outbound.LastError,
	}
}

func inboxTenantIsolationSocialAccountValues(account db.SocialAccount) []any {
	return []any{
		account.ID,
		account.ProfileID,
		account.Platform,
		account.AccessToken,
		account.RefreshToken,
		account.TokenExpiresAt,
		account.ExternalAccountID,
		account.AccountName,
		account.AccountAvatarUrl,
		account.ConnectedAt,
		account.DisconnectedAt,
		account.Metadata,
		account.Scope,
		account.Status,
		account.ConnectionType,
		account.ConnectSessionID,
		account.ExternalUserID,
		account.ExternalUserEmail,
		account.LastRefreshedAt,
		account.XAppMode,
	}
}

type inboxCompleteKnownTx struct {
	outbound           db.XInboxOutboundRequest
	item               db.InboxItem
	itemErr            error
	rejectMissingScope bool
	getInboxCalls      [][]interface{}
	committed          bool
}

func (f *inboxCompleteKnownTx) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("unexpected Begin")
}

func (f *inboxCompleteKnownTx) Commit(context.Context) error {
	f.committed = true
	return nil
}

func (f *inboxCompleteKnownTx) Rollback(context.Context) error { return nil }

func (f *inboxCompleteKnownTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("unexpected CopyFrom")
}

func (f *inboxCompleteKnownTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }

func (f *inboxCompleteKnownTx) LargeObjects() pgx.LargeObjects { return pgx.LargeObjects{} }

func (f *inboxCompleteKnownTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("unexpected Prepare")
}

func (f *inboxCompleteKnownTx) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected Exec")
}

func (f *inboxCompleteKnownTx) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query")
}

func (f *inboxCompleteKnownTx) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetXInboxOutboundRequestByIDForUpdate"):
		return metaWebhookRoutingRow{values: inboxTenantIsolationOutboundValues(f.outbound)}
	case strings.Contains(query, "-- name: GetInboxItem"):
		f.getInboxCalls = append(f.getInboxCalls, append([]interface{}(nil), args...))
		if f.rejectMissingScope && len(args) >= 4 && args[2] == false && args[3] == "" {
			return metaWebhookRoutingRow{err: pgx.ErrNoRows}
		}
		if f.itemErr != nil {
			return metaWebhookRoutingRow{err: f.itemErr}
		}
		return metaWebhookRoutingRow{values: inboxTenantIsolationItemValues(f.item)}
	default:
		return metaWebhookRoutingRow{err: errors.New("unexpected QueryRow")}
	}
}

func (f *inboxCompleteKnownTx) Conn() *pgx.Conn { return nil }

func TestInboxXSuccessfulWriteDefersFinalizationToDurableCompletion(t *testing.T) {
	adapter := &fakeXInboxReplyAdapter{}
	credits := &fakeXInboxCredits{
		event: xcredits.UsageEvent{
			ID:             "usage-1",
			Status:         xcredits.UsageStatusProvisional,
			OperationKey:   "post.reply_summoned",
			CatalogVersion: xcredits.CatalogVersion,
			WeightedUnits:  10,
		},
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
	if result.ExternalID != "tweet-2" || result.UsageEventID != "usage-1" {
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
