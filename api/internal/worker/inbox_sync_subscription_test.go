package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestEnsureInstagramWebhookSubscriptionCachesSuccess(t *testing.T) {
	subscriber := &fakeInboxInstagramWebhookSubscriber{}
	worker := &InboxSyncWorker{
		instagramWebhookSubscriber: subscriber,
		igWebhookSubscriptions:     make(map[string]bool),
	}
	account := db.ListAllInboxAccountsRow{
		ID:                "sa_1",
		Platform:          "instagram",
		ExternalAccountID: "ig_1",
	}

	worker.ensureInstagramWebhookSubscription(context.Background(), account, "token")
	worker.ensureInstagramWebhookSubscription(context.Background(), account, "token")

	if subscriber.calls != 1 {
		t.Fatalf("subscriber calls = %d, want 1", subscriber.calls)
	}
}

func TestEnsureInstagramWebhookSubscriptionRetriesFailure(t *testing.T) {
	subscriber := &fakeInboxInstagramWebhookSubscriber{err: errors.New("meta denied")}
	worker := &InboxSyncWorker{
		instagramWebhookSubscriber: subscriber,
		igWebhookSubscriptions:     make(map[string]bool),
	}
	account := db.ListAllInboxAccountsRow{
		ID:                "sa_1",
		Platform:          "instagram",
		ExternalAccountID: "ig_1",
	}

	worker.ensureInstagramWebhookSubscription(context.Background(), account, "token")
	worker.ensureInstagramWebhookSubscription(context.Background(), account, "token")

	if subscriber.calls != 2 {
		t.Fatalf("subscriber calls = %d, want 2", subscriber.calls)
	}
}

func TestEnsureInstagramWebhookSubscriptionIgnoresOtherPlatforms(t *testing.T) {
	subscriber := &fakeInboxInstagramWebhookSubscriber{}
	worker := &InboxSyncWorker{
		instagramWebhookSubscriber: subscriber,
		igWebhookSubscriptions:     make(map[string]bool),
	}
	account := db.ListAllInboxAccountsRow{
		ID:                "sa_1",
		Platform:          "facebook",
		ExternalAccountID: "page_1",
	}

	worker.ensureInstagramWebhookSubscription(context.Background(), account, "token")

	if subscriber.calls != 0 {
		t.Fatalf("subscriber calls = %d, want 0", subscriber.calls)
	}
}

type fakeInboxInstagramWebhookSubscriber struct {
	calls int
	err   error
}

func (f *fakeInboxInstagramWebhookSubscriber) Subscribe(context.Context, string, string) error {
	f.calls++
	return f.err
}
