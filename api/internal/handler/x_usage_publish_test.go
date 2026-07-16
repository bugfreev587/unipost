package handler

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/xcredits"
	"github.com/xiaoboyu/unipost-api/internal/xinbox"
)

type fakeXUsageService struct {
	requests []xcredits.ReserveRequest
	finals   []string
	reverses []string
	event    xcredits.UsageEvent
	err      error
}

func (f *fakeXUsageService) Reserve(_ context.Context, req xcredits.ReserveRequest) (xcredits.UsageEvent, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return xcredits.UsageEvent{}, f.err
	}
	if f.event.ID != "" || f.event.Status != "" {
		return f.event, nil
	}
	return xcredits.UsageEvent{
		ID:             "xue_1",
		Status:         xcredits.UsageStatusProvisional,
		OperationKey:   req.OperationKey,
		CatalogVersion: xcredits.CatalogVersion,
		WeightedUnits:  req.RequestedUnits,
	}, nil
}

func (f *fakeXUsageService) Finalize(_ context.Context, eventID string, _ int64) error {
	f.finals = append(f.finals, eventID)
	return nil
}

func (f *fakeXUsageService) Reverse(_ context.Context, eventID string) error {
	f.reverses = append(f.reverses, eventID)
	return nil
}

func TestXOperationForTextUsesConservativeURLWeight(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{text: "plain launch update", want: "post.create"},
		{text: "read https://unipost.dev/docs", want: "post.create_url"},
		{text: "visit www.unipost.dev", want: "post.create_url"},
		{text: "quoted https://x.com/unipost/status/1", want: "post.create_url"},
		{text: "docs are at unipost.dev/docs", want: "post.create_url"},
		{text: "short link bit.ly/launch", want: "post.create_url"},
		{text: "international domain 例子.公司/发布", want: "post.create_url"},
		{text: "punctuation (unipost.dev), works", want: "post.create_url"},
	}
	for _, tt := range tests {
		if got := xOperationForText(tt.text); got != tt.want {
			t.Fatalf("xOperationForText(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}

func TestReserveManagedXUsageBypassesBYO(t *testing.T) {
	fake := &fakeXUsageService{}
	h := &SocialPostHandler{xUsage: fake}
	account := db.SocialAccount{
		ID:             "sa_1",
		Platform:       "twitter",
		ConnectionType: "byo",
		XAppMode:       pgtype.Text{String: string(xinbox.AppModeWorkspace), Valid: true},
	}

	event, err := h.reserveManagedXUsage(context.Background(), "ws_1", "job_1:1:main", account, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if event.ID != "" {
		t.Fatalf("event = %+v, want empty bypass event", event)
	}
	if len(fake.requests) != 0 {
		t.Fatalf("reserve requests = %d, want 0", len(fake.requests))
	}
}

func TestReserveManagedXUsageLegacyUnknownPreservesPublishingWithoutCredits(t *testing.T) {
	fake := &fakeXUsageService{}
	h := NewSocialPostHandler(nil, nil, nil, nil, nil, nil, nil).SetXUsageService(fake)
	event, err := h.reserveManagedXUsage(context.Background(), "ws_1", "job_legacy:main", db.SocialAccount{
		ID:             "sa_legacy",
		Platform:       "twitter",
		ConnectionType: "byo",
		XAppMode:       pgtype.Text{String: string(xinbox.AppModeLegacyUnknown), Valid: true},
	}, "legacy publish")
	if err != nil {
		t.Fatalf("reserveManagedXUsage: %v", err)
	}
	if event.ID != "" || len(fake.requests) != 0 {
		t.Fatalf("event=%+v reserve requests=%d, want publishing bypass without credits", event, len(fake.requests))
	}
}

func TestReserveManagedXUsageNullAppModeUsesLegacyBypass(t *testing.T) {
	fake := &fakeXUsageService{}
	h := NewSocialPostHandler(nil, nil, nil, nil, nil, nil, nil).SetXUsageService(fake)
	event, err := h.reserveManagedXUsage(context.Background(), "ws_1", "job_1:null", db.SocialAccount{
		Platform: "twitter",
	}, "hello")
	if err != nil {
		t.Fatalf("reserveManagedXUsage: %v", err)
	}
	if event.ID != "" || len(fake.requests) != 0 {
		t.Fatalf("event=%+v reserve requests=%d, want legacy bypass", event, len(fake.requests))
	}
}

func TestReserveManagedXUsageRejectsInvalidPersistedAppMode(t *testing.T) {
	fake := &fakeXUsageService{}
	h := NewSocialPostHandler(nil, nil, nil, nil, nil, nil, nil).SetXUsageService(fake)
	_, err := h.reserveManagedXUsage(context.Background(), "ws_1", "job_1:invalid", db.SocialAccount{
		Platform: "twitter",
		XAppMode: pgtype.Text{String: "garbage", Valid: true},
	}, "hello")
	if err == nil {
		t.Fatal("invalid persisted app mode error = nil, want validation error")
	}
}

func TestReserveManagedXUsageUsesCatalogWeight(t *testing.T) {
	fake := &fakeXUsageService{}
	h := &SocialPostHandler{xUsage: fake}
	account := db.SocialAccount{
		ID:             "sa_1",
		Platform:       "twitter",
		ConnectionType: "managed",
		XAppMode:       pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
	}

	event, err := h.reserveManagedXUsage(context.Background(), "ws_1", "job_1:1:main", account, "https://unipost.dev")
	if err != nil {
		t.Fatal(err)
	}
	if event.ID == "" || len(fake.requests) != 1 {
		t.Fatalf("event=%+v requests=%d", event, len(fake.requests))
	}
	req := fake.requests[0]
	if req.OperationKey != "post.create_url" || req.RequestedUnits != 200 {
		t.Fatalf("request = %+v", req)
	}
	if req.IdempotencyKey != "job_1:1:main" {
		t.Fatalf("idempotency key = %q", req.IdempotencyKey)
	}
}

func TestReserveManagedXOperationUsesFirstCommentWeight(t *testing.T) {
	fake := &fakeXUsageService{}
	h := &SocialPostHandler{xUsage: fake}
	account := db.SocialAccount{
		ID:             "sa_1",
		Platform:       "twitter",
		ConnectionType: "managed",
		XAppMode:       pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
	}

	_, err := h.reserveManagedXOperation(
		context.Background(),
		"ws_1",
		"job_1:1:first-comment",
		account,
		"post.create",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.requests) != 1 {
		t.Fatalf("reserve requests = %d, want 1", len(fake.requests))
	}
	req := fake.requests[0]
	if req.OperationKey != "post.create" || req.RequestedUnits != 15 {
		t.Fatalf("request = %+v", req)
	}
	if req.IdempotencyKey != "job_1:1:first-comment" {
		t.Fatalf("idempotency key = %q", req.IdempotencyKey)
	}
}

func TestReserveManagedXOperationStopsDuplicateUnknownOutcome(t *testing.T) {
	fake := &fakeXUsageService{event: xcredits.UsageEvent{
		ID:             "xue_existing",
		Status:         xcredits.UsageStatusProvisional,
		OperationKey:   "post.create",
		CatalogVersion: xcredits.CatalogVersion,
		WeightedUnits:  15,
		Duplicate:      true,
	}}
	h := &SocialPostHandler{xUsage: fake}
	account := db.SocialAccount{
		ID:             "sa_1",
		Platform:       "twitter",
		ConnectionType: "managed",
		XAppMode:       pgtype.Text{String: string(xinbox.AppModeUniPostManaged), Valid: true},
	}

	_, err := h.reserveManagedXOperation(context.Background(), "ws_1", "result_1:main", account, "post.create")
	if !errors.Is(err, ErrXWriteOutcomePending) {
		t.Fatalf("error = %v, want ErrXWriteOutcomePending", err)
	}
}

func TestSettleManagedXUsageFinalizesSuccessAndReversesConfirmedFailure(t *testing.T) {
	fake := &fakeXUsageService{}
	event := xcredits.UsageEvent{ID: "xue_1", WeightedUnits: 15}

	if err := settleManagedXUsage(context.Background(), fake, event, nil); err != nil {
		t.Fatal(err)
	}
	if len(fake.finals) != 1 || len(fake.reverses) != 0 {
		t.Fatalf("success finals=%v reverses=%v", fake.finals, fake.reverses)
	}

	fake.finals = nil
	if err := settleManagedXUsage(context.Background(), fake, event, errors.New("tweet failed (400): invalid")); err != nil {
		t.Fatal(err)
	}
	if len(fake.finals) != 0 || len(fake.reverses) != 1 {
		t.Fatalf("failure finals=%v reverses=%v", fake.finals, fake.reverses)
	}
}

func TestSettleManagedXUsageKeepsUnknownWriteProvisional(t *testing.T) {
	for _, transportErr := range []error{
		errors.New("create_tweet timeout after 20s"),
		errors.New("create_tweet: unexpected EOF"),
		errors.New("create_tweet: connection reset by peer"),
		errors.New("create_tweet_reply: stream error: stream ID 7; INTERNAL_ERROR"),
	} {
		fake := &fakeXUsageService{}
		event := xcredits.UsageEvent{ID: "xue_1", WeightedUnits: 15}
		err := settleManagedXUsage(context.Background(), fake, event, transportErr)
		if !errors.Is(err, ErrXWriteOutcomePending) {
			t.Fatalf("transport error %q settled as %v, want ErrXWriteOutcomePending", transportErr, err)
		}
		if len(fake.finals) != 0 || len(fake.reverses) != 0 {
			t.Fatalf("transport error %q finals=%v reverses=%v", transportErr, fake.finals, fake.reverses)
		}
	}
}

func TestPublishGateOrdersDailyCapBeforeXUsage(t *testing.T) {
	source, err := os.ReadFile("social_posts.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	daily := strings.Index(text, "dailyTracker.Allow(acc.ID, acc.Platform)")
	usage := strings.Index(text, "h.reserveManagedXUsage(")
	if daily < 0 || usage < 0 {
		t.Fatalf("daily gate index=%d usage index=%d", daily, usage)
	}
	if daily >= usage {
		t.Fatalf("daily safety gate must execute before X usage reservation")
	}
}

func TestQueuedXUsageKeyUsesStableResultID(t *testing.T) {
	source, err := os.ReadFile("social_post_queue.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, "xUsageKeyForResult(res.ID)") {
		t.Fatal("queued publishing must derive the X usage key from the stable social post result ID")
	}
	if strings.Contains(text, `fmt.Sprintf("%s:%d", job.ID, job.Attempts)`) {
		t.Fatal("queued publishing must not derive the X usage key from mutable job attempts")
	}
}

func TestPostResultResponseIncludesXUsageContract(t *testing.T) {
	row := db.SocialPostResult{
		ID:                    "result_1",
		SocialAccountID:       "sa_1",
		Status:                "published",
		XCreditsCounted:       15,
		XCreditOperation:      pgtype.Text{String: "post.create", Valid: true},
		XCreditCatalogVersion: pgtype.Text{String: xcredits.CatalogVersion, Valid: true},
		XCreditBillingMode:    pgtype.Text{String: "unipost_managed_app", Valid: true},
	}
	got := postResultResponseFromDBResult(row, accountSummary{Platform: "twitter"})
	if got.XCreditsCounted != 15 ||
		got.XCreditOperation == nil || *got.XCreditOperation != "post.create" ||
		got.XCreditCatalog == nil || *got.XCreditCatalog != xcredits.CatalogVersion ||
		got.XCreditBillingMode == nil || *got.XCreditBillingMode != "unipost_managed_app" {
		t.Fatalf("response = %+v", got)
	}
}
