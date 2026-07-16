package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
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
	exposureReleaseErr     error
	exposureReconcileErr   error
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
	f.exposureFinalizedUnits = units
	return nil
}
func (f *fakeXInboxCredits) ReleaseExposure(context.Context, string) error {
	f.exposureReleased = true
	return f.exposureReleaseErr
}
func (f *fakeXInboxCredits) MarkExposureNeedsReconciliation(context.Context, string, string) error {
	f.exposureReconciliation = true
	return f.exposureReconcileErr
}
func (f *fakeXInboxCredits) MarkExposureReleasePending(context.Context, string, string) error {
	f.exposureReconciliation = true
	return f.exposureReconcileErr
}

type fakeXInboxBackfillAdapter struct {
	mentionPageSizes []int
	dmPageSizes      []int
	mentionTokens    []string
	mentionPages     []platform.TwitterInboxPage
	dmTokens         []string
	dmPages          []platform.TwitterInboxPage
	mentionErr       error
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
