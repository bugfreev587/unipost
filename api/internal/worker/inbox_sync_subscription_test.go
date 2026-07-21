package worker

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestEnsureInstagramWebhookSubscriptionUsesStoredWebhookUserIDAndCachesSuccess(t *testing.T) {
	subscriber := &fakeInboxInstagramWebhookSubscriber{webhookUserID: "resolved_ig_1"}
	worker := &InboxSyncWorker{
		instagramWebhookSubscriber: subscriber,
		igWebhookSubscriptions:     make(map[string]bool),
	}
	account := db.ListAllInboxAccountsRow{
		ID:                     "sa_1",
		Platform:               "instagram",
		ExternalAccountID:      "ig_1",
		InstagramWebhookUserID: "stored_webhook_1",
	}

	worker.ensureInstagramWebhookSubscription(context.Background(), account, "token")
	worker.ensureInstagramWebhookSubscription(context.Background(), account, "token")

	if subscriber.fetchCalls != 0 {
		t.Fatalf("resolver calls = %d, want 0", subscriber.fetchCalls)
	}
	if subscriber.subscribeCalls != 1 {
		t.Fatalf("subscriber calls = %d, want 1", subscriber.subscribeCalls)
	}
	if subscriber.subscribedAccountID != "stored_webhook_1" {
		t.Fatalf("subscribed account id = %q, want stored_webhook_1", subscriber.subscribedAccountID)
	}
}

func TestEnsureInstagramWebhookSubscriptionBackfillsBeforeSubscribe(t *testing.T) {
	database := &fakeInboxSubscriptionDB{rowsAffected: 1}
	subscriber := &fakeInboxInstagramWebhookSubscriber{webhookUserID: "resolved_webhook_1"}
	worker := &InboxSyncWorker{
		queries:                    db.New(database),
		instagramWebhookSubscriber: subscriber,
		igWebhookSubscriptions:     make(map[string]bool),
	}
	account := db.ListAllInboxAccountsRow{
		ID:                "sa_1",
		Platform:          "instagram",
		ExternalAccountID: "ig_1",
	}

	worker.ensureInstagramWebhookSubscription(context.Background(), account, "token_1")

	if subscriber.fetchCalls != 1 {
		t.Fatalf("resolver calls = %d, want 1", subscriber.fetchCalls)
	}
	if subscriber.subscribeCalls != 1 {
		t.Fatalf("subscriber calls = %d, want 1", subscriber.subscribeCalls)
	}
	if database.execCalls != 1 || database.accountID != "sa_1" || database.webhookUserID != "resolved_webhook_1" {
		t.Fatalf("persist calls=%d account=%q webhook_user_id=%q", database.execCalls, database.accountID, database.webhookUserID)
	}
	if subscriber.subscribedAccountID != "resolved_webhook_1" {
		t.Fatalf("subscribed account id = %q, want resolved_webhook_1", subscriber.subscribedAccountID)
	}
}

func TestEnsureInstagramWebhookSubscriptionRetriesResolverFailure(t *testing.T) {
	subscriber := &fakeInboxInstagramWebhookSubscriber{fetchErr: errors.New("meta denied")}
	worker := &InboxSyncWorker{
		instagramWebhookSubscriber: subscriber,
		igWebhookSubscriptions:     make(map[string]bool),
	}
	account := db.ListAllInboxAccountsRow{ID: "sa_1", Platform: "instagram", ExternalAccountID: "ig_1"}

	worker.ensureInstagramWebhookSubscription(context.Background(), account, "token")
	worker.ensureInstagramWebhookSubscription(context.Background(), account, "token")

	if subscriber.fetchCalls != 2 {
		t.Fatalf("resolver calls = %d, want 2", subscriber.fetchCalls)
	}
	if subscriber.subscribeCalls != 0 {
		t.Fatalf("subscriber calls = %d, want 0", subscriber.subscribeCalls)
	}
}

func TestEnsureInstagramWebhookSubscriptionDoesNotSubscribeWhenPersistenceFails(t *testing.T) {
	for _, test := range []struct {
		name         string
		rowsAffected int64
		execErr      error
	}{
		{name: "database error", execErr: errors.New("database unavailable")},
		{name: "account no longer active", rowsAffected: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			database := &fakeInboxSubscriptionDB{rowsAffected: test.rowsAffected, execErr: test.execErr}
			subscriber := &fakeInboxInstagramWebhookSubscriber{webhookUserID: "resolved_webhook_1"}
			worker := &InboxSyncWorker{
				queries:                    db.New(database),
				instagramWebhookSubscriber: subscriber,
				igWebhookSubscriptions:     make(map[string]bool),
			}
			account := db.ListAllInboxAccountsRow{ID: "sa_1", Platform: "instagram", ExternalAccountID: "ig_1"}

			worker.ensureInstagramWebhookSubscription(context.Background(), account, "token")

			if subscriber.subscribeCalls != 0 {
				t.Fatalf("subscriber calls = %d, want 0", subscriber.subscribeCalls)
			}
			if worker.igWebhookSubscriptions[account.ID] {
				t.Fatal("failed repair must not be cached")
			}
		})
	}
}

func TestEnsureInstagramWebhookSubscriptionRetriesSubscribeFailure(t *testing.T) {
	subscriber := &fakeInboxInstagramWebhookSubscriber{subscribeErr: errors.New("meta denied")}
	worker := &InboxSyncWorker{
		instagramWebhookSubscriber: subscriber,
		igWebhookSubscriptions:     make(map[string]bool),
	}
	account := db.ListAllInboxAccountsRow{
		ID:                     "sa_1",
		Platform:               "instagram",
		ExternalAccountID:      "ig_1",
		InstagramWebhookUserID: "stored_webhook_1",
	}

	worker.ensureInstagramWebhookSubscription(context.Background(), account, "token")
	worker.ensureInstagramWebhookSubscription(context.Background(), account, "token")

	if subscriber.subscribeCalls != 2 {
		t.Fatalf("subscriber calls = %d, want 2", subscriber.subscribeCalls)
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

	if subscriber.subscribeCalls != 0 || subscriber.fetchCalls != 0 {
		t.Fatalf("subscriber calls = %d resolver calls = %d, want 0", subscriber.subscribeCalls, subscriber.fetchCalls)
	}
}

type fakeInboxInstagramWebhookSubscriber struct {
	fetchCalls          int
	subscribeCalls      int
	webhookUserID       string
	fetchErr            error
	subscribeErr        error
	subscribedAccountID string
}

func (f *fakeInboxInstagramWebhookSubscriber) FetchWebhookUserID(context.Context, string) (string, error) {
	f.fetchCalls++
	return f.webhookUserID, f.fetchErr
}

func (f *fakeInboxInstagramWebhookSubscriber) Subscribe(_ context.Context, accountID, _ string) error {
	f.subscribeCalls++
	f.subscribedAccountID = accountID
	return f.subscribeErr
}

type fakeInboxSubscriptionDB struct {
	execCalls     int
	rowsAffected  int64
	execErr       error
	accountID     string
	webhookUserID string
}

func (f *fakeInboxSubscriptionDB) Exec(_ context.Context, _ string, args ...interface{}) (pgconn.CommandTag, error) {
	f.execCalls++
	if len(args) == 2 {
		f.webhookUserID, _ = args[0].(string)
		f.accountID, _ = args[1].(string)
	}
	if f.execErr != nil {
		return pgconn.CommandTag{}, f.execErr
	}
	return pgconn.NewCommandTag(fmt.Sprintf("UPDATE %d", f.rowsAffected)), nil
}

func (*fakeInboxSubscriptionDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query call")
}

func (*fakeInboxSubscriptionDB) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	return fakeInboxSubscriptionRow{}
}

type fakeInboxSubscriptionRow struct{}

func (fakeInboxSubscriptionRow) Scan(...interface{}) error {
	return errors.New("unexpected QueryRow call")
}
