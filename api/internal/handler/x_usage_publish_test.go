package handler

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/xcredits"
)

type fakeXUsageService struct {
	requests []xcredits.ReserveRequest
	finals   []string
	reverses []string
}

func (f *fakeXUsageService) Reserve(_ context.Context, req xcredits.ReserveRequest) (xcredits.UsageEvent, error) {
	f.requests = append(f.requests, req)
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

func TestReserveManagedXUsageUsesCatalogWeight(t *testing.T) {
	fake := &fakeXUsageService{}
	h := &SocialPostHandler{xUsage: fake}
	account := db.SocialAccount{
		ID:             "sa_1",
		Platform:       "twitter",
		ConnectionType: "managed",
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
