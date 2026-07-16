package worker

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/xcredits"
	"github.com/xiaoboyu/unipost-api/internal/xinbox"
)

type fakeXInboxCipher struct {
	values map[string]string
}

func (f fakeXInboxCipher) Decrypt(value string) (string, error) {
	decrypted, ok := f.values[value]
	if !ok {
		return "", errors.New("ciphertext not found")
	}
	return decrypted, nil
}

type fakeXInboxDeliveryAPI struct {
	mu sync.Mutex

	ruleID          string
	subscriptionID  string
	webhookID       string
	ruleErr         error
	subscriptionErr error

	ruleTokens         []string
	subscriptionTokens []string
	deletedRules       []string
	deletedSubs        []string
	deletedSubTokens   []string
}

func (f *fakeXInboxDeliveryAPI) EnsureFilteredStreamRule(
	_ context.Context,
	token, accountID, handle string,
) (xinbox.StreamRule, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ruleTokens = append(f.ruleTokens, token)
	if f.ruleErr != nil {
		return xinbox.StreamRule{}, f.ruleErr
	}
	return xinbox.StreamRule{ID: f.ruleID, Tag: xinbox.FilteredStreamRuleTag(accountID), Value: xinbox.FilteredStreamRuleValue(handle)}, nil
}

func (f *fakeXInboxDeliveryAPI) DeleteFilteredStreamRule(_ context.Context, _ string, ruleID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedRules = append(f.deletedRules, ruleID)
	return nil
}

func (f *fakeXInboxDeliveryAPI) EnsureWebhook(_ context.Context, _ string, configuredURL string) (xinbox.Webhook, error) {
	return xinbox.Webhook{ID: f.webhookID, URL: configuredURL, Valid: true}, nil
}

func (f *fakeXInboxDeliveryAPI) EnsureDMSubscription(
	_ context.Context,
	userToken, _ string,
	accountID, userID, webhookID string,
) (xinbox.ActivitySubscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subscriptionTokens = append(f.subscriptionTokens, userToken)
	if f.subscriptionErr != nil {
		return xinbox.ActivitySubscription{}, f.subscriptionErr
	}
	return xinbox.ActivitySubscription{
		ID:        f.subscriptionID,
		EventType: "dm.received",
		Filter:    xinbox.ActivityFilter{UserID: userID},
		Tag:       xinbox.DMSubscriptionTag(accountID),
		WebhookID: webhookID,
	}, nil
}

func (f *fakeXInboxDeliveryAPI) DeleteActivitySubscription(_ context.Context, appToken string, subscriptionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedSubs = append(f.deletedSubs, subscriptionID)
	f.deletedSubTokens = append(f.deletedSubTokens, appToken)
	return nil
}

type fakeXInboxDeliveryStore struct {
	accounts []XInboxDeliveryAccount
	cleanups []XInboxCleanupIntent
	states   []XInboxDeliveryState
}

func (f *fakeXInboxDeliveryStore) ListAccounts(context.Context) ([]XInboxDeliveryAccount, error) {
	return append([]XInboxDeliveryAccount(nil), f.accounts...), nil
}

func (f *fakeXInboxDeliveryStore) SaveState(_ context.Context, state XInboxDeliveryState) error {
	f.states = append(f.states, state)
	for i := range f.accounts {
		if f.accounts[i].SocialAccountID == state.SocialAccountID {
			f.accounts[i].FilteredStreamRuleID = state.FilteredStreamRuleID
			f.accounts[i].ActivityDMSubscriptionID = state.ActivityDMSubscriptionID
		}
	}
	return nil
}

func (f *fakeXInboxDeliveryStore) ListCleanupIntents(context.Context) ([]XInboxCleanupIntent, error) {
	return append([]XInboxCleanupIntent(nil), f.cleanups...), nil
}

func (f *fakeXInboxDeliveryStore) SaveCleanupIntent(_ context.Context, intent XInboxCleanupIntent) error {
	if intent.FilteredStreamRuleID == "" && intent.ActivityDMSubscriptionID == "" {
		return errors.New("cleanup intent cannot persist without an upstream resource id")
	}
	for i := range f.cleanups {
		if f.cleanups[i].ID == intent.ID {
			f.cleanups[i] = intent
		}
	}
	return nil
}

func (f *fakeXInboxDeliveryStore) DeleteCleanupIntent(_ context.Context, id string) error {
	for i := range f.cleanups {
		if f.cleanups[i].ID == id {
			f.cleanups = append(f.cleanups[:i], f.cleanups[i+1:]...)
			return nil
		}
	}
	return nil
}

type fakeXInboxUsageReader struct {
	snapshot xcredits.Snapshot
}

func (f fakeXInboxUsageReader) Snapshot(context.Context, string, time.Time) (xcredits.Snapshot, error) {
	return f.snapshot, nil
}

func activeManagedXInboxAccount() XInboxDeliveryAccount {
	return XInboxDeliveryAccount{
		SocialAccountID:      "account-1",
		WorkspaceID:          "workspace-1",
		Handle:               "UniPostDev",
		ExternalUserID:       "2244994945",
		AccessTokenEncrypted: "encrypted-user-token",
		AppMode:              xinbox.AppModeUniPostManaged,
		Scopes:               xinbox.RequiredInboxScopes(),
		AccountActive:        true,
		PlanAllowsInbox:      true,
	}
}

func TestXInboxDeliveryReconcilePersistsRuleAndPrivateDMSubscription(t *testing.T) {
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{activeManagedXInboxAccount()}}
	api := &fakeXInboxDeliveryAPI{ruleID: "rule-1", webhookID: "webhook-1", subscriptionID: "subscription-1"}
	worker := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store:            store,
		API:              api,
		Cipher:           fakeXInboxCipher{values: map[string]string{"encrypted-user-token": "user-oauth-token"}},
		Usage:            fakeXInboxUsageReader{},
		ManagedAppBearer: "managed-app-token",
		WebhookURL:       "https://dev-api.unipost.dev/v1/webhooks/twitter",
	})

	if err := worker.ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.states) == 0 {
		t.Fatal("no delivery state persisted")
	}
	got := store.states[len(store.states)-1]
	if got.FilteredStreamRuleID != "rule-1" ||
		got.ActivityDMSubscriptionID != "subscription-1" ||
		got.DeliveryStatus != xinbox.DeliveryStatusActive {
		t.Fatalf("state = %+v", got)
	}
	if want := []string{"managed-app-token"}; !reflect.DeepEqual(api.ruleTokens, want) {
		t.Fatalf("rule tokens = %v, want app bearer", api.ruleTokens)
	}
	if want := []string{"user-oauth-token"}; !reflect.DeepEqual(api.subscriptionTokens, want) {
		t.Fatalf("subscription tokens = %v, want connected user OAuth token", api.subscriptionTokens)
	}
}

func TestXInboxDeliveryPersistsRuleBeforeSubscriptionFailure(t *testing.T) {
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{activeManagedXInboxAccount()}}
	api := &fakeXInboxDeliveryAPI{
		ruleID:          "rule-1",
		webhookID:       "webhook-1",
		subscriptionErr: errors.New("subscription unavailable"),
	}
	worker := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store:            store,
		API:              api,
		Cipher:           fakeXInboxCipher{values: map[string]string{"encrypted-user-token": "user-oauth-token"}},
		Usage:            fakeXInboxUsageReader{},
		ManagedAppBearer: "managed-app-token",
		WebhookURL:       "https://dev-api.unipost.dev/v1/webhooks/twitter",
	})

	if err := worker.ReconcileOnce(context.Background()); err == nil {
		t.Fatal("expected subscription error")
	}
	foundDurableRule := false
	for _, state := range store.states {
		if state.FilteredStreamRuleID == "rule-1" {
			foundDurableRule = true
		}
	}
	if !foundDurableRule {
		t.Fatalf("states = %+v, want rule id persisted before later failure", store.states)
	}
}

func TestXInboxDeliveryCleanupIntentUsesExactStoredIDsAndIsIdempotent(t *testing.T) {
	store := &fakeXInboxDeliveryStore{cleanups: []XInboxCleanupIntent{{
		ID:                       "cleanup-1",
		SocialAccountID:          "deleted-account",
		AppMode:                  xinbox.AppModeWorkspace,
		AppBearerTokenEncrypted:  "encrypted-app-token",
		FilteredStreamRuleID:     "rule-exact",
		ActivityDMSubscriptionID: "subscription-exact",
	}}}
	api := &fakeXInboxDeliveryAPI{}
	worker := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store: store,
		API:   api,
		Cipher: fakeXInboxCipher{values: map[string]string{
			"encrypted-app-token": "workspace-app-token",
		}},
	})

	if err := worker.ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := worker.ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := []string{"rule-exact"}; !reflect.DeepEqual(api.deletedRules, want) {
		t.Fatalf("deleted rules = %v, want %v", api.deletedRules, want)
	}
	if want := []string{"subscription-exact"}; !reflect.DeepEqual(api.deletedSubs, want) {
		t.Fatalf("deleted subscriptions = %v, want %v", api.deletedSubs, want)
	}
	if want := []string{"workspace-app-token"}; !reflect.DeepEqual(api.deletedSubTokens, want) {
		t.Fatalf("subscription delete tokens = %v, want workspace app bearer %v", api.deletedSubTokens, want)
	}
	if len(store.cleanups) != 0 {
		t.Fatalf("cleanup intents = %+v, want empty", store.cleanups)
	}
}

func TestXInboxDeliveryDailyAllowancePauseRemovesPaidSources(t *testing.T) {
	account := activeManagedXInboxAccount()
	account.FilteredStreamRuleID = "rule-1"
	account.ActivityDMSubscriptionID = "subscription-1"
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
	api := &fakeXInboxDeliveryAPI{}
	worker := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store:            store,
		API:              api,
		Cipher:           fakeXInboxCipher{values: map[string]string{"encrypted-user-token": "user-oauth-token"}},
		Usage:            fakeXInboxUsageReader{snapshot: xcredits.Snapshot{PausePaidSources: true, InboundPauseReason: xcredits.PauseReasonMonthlyAllowance}},
		ManagedAppBearer: "managed-app-token",
		WebhookURL:       "https://dev-api.unipost.dev/v1/webhooks/twitter",
	})

	if err := worker.ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := store.states[len(store.states)-1]
	if got.FilteredStreamRuleID != "" || got.ActivityDMSubscriptionID != "" {
		t.Fatalf("state = %+v, want upstream ids cleared after confirmed cleanup", got)
	}
	if got.DeliveryStatus != xinbox.DeliveryStatusPausedAllowance {
		t.Fatalf("delivery status = %q", got.DeliveryStatus)
	}
	if want := []string{"managed-app-token"}; !reflect.DeepEqual(api.deletedSubTokens, want) {
		t.Fatalf("subscription delete tokens = %v, want managed app bearer %v", api.deletedSubTokens, want)
	}
}

func TestXInboxDeliveryKeepsWorkspaceXAppsIsolated(t *testing.T) {
	first := activeManagedXInboxAccount()
	first.SocialAccountID = "account-1"
	first.WorkspaceID = "workspace-1"
	first.AppMode = xinbox.AppModeWorkspace
	first.AppBearerTokenEncrypted = "encrypted-app-one"
	first.Scopes = []string{"tweet.read", "tweet.write", "users.read"}
	second := first
	second.SocialAccountID = "account-2"
	second.WorkspaceID = "workspace-2"
	second.AppBearerTokenEncrypted = "encrypted-app-two"

	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{first, second}}
	api := &fakeXInboxDeliveryAPI{ruleID: "rule-created"}
	worker := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store: store,
		API:   api,
		Cipher: fakeXInboxCipher{values: map[string]string{
			"encrypted-app-one": "workspace-app-token-one",
			"encrypted-app-two": "workspace-app-token-two",
		}},
	})

	apps, err := worker.reconcile(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]string)
	for _, app := range apps {
		got[app.Identity] = app.BearerToken
	}
	want := map[string]string{
		"workspace:workspace-1": "workspace-app-token-one",
		"workspace:workspace-2": "workspace-app-token-two",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("apps = %v, want isolated workspace apps %v", got, want)
	}
}

type sharedTestLeader struct {
	held atomic.Bool
}

func (l *sharedTestLeader) TryAcquire(context.Context, string) (XInboxLeaderLease, bool, error) {
	if !l.held.CompareAndSwap(false, true) {
		return nil, false, nil
	}
	return testLeaderLease{release: func() { l.held.Store(false) }}, true, nil
}

type testLeaderLease struct {
	release func()
}

func (l testLeaderLease) Release(context.Context) error {
	l.release()
	return nil
}

type blockingStreamRunner struct {
	started chan struct{}
	release chan struct{}
	runs    atomic.Int32
}

func (r *blockingStreamRunner) Run(context.Context, string, string, func(xinbox.StreamEvent) error) error {
	r.runs.Add(1)
	select {
	case r.started <- struct{}{}:
	default:
	}
	<-r.release
	return nil
}

func TestXInboxDeliveryAdvisoryLeaderAllowsOneStreamAcrossReplicas(t *testing.T) {
	leader := &sharedTestLeader{}
	stream := &blockingStreamRunner{started: make(chan struct{}, 1), release: make(chan struct{})}
	config := XInboxDeliveryConfig{Leader: leader, Stream: stream}
	first := NewXInboxDeliveryWorker(config)
	second := NewXInboxDeliveryWorker(config)
	app := XInboxAppStream{Identity: "workspace:one", BearerToken: "app-token"}

	errs := make(chan error, 2)
	go func() { errs <- first.runAppStream(context.Background(), app) }()
	<-stream.started
	go func() { errs <- second.runAppStream(context.Background(), app) }()

	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if got := stream.runs.Load(); got != 1 {
		t.Fatalf("stream runs = %d, want one leader connection", got)
	}
	close(stream.release)
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
}

func TestXInboxDeliveryUsesPostgresSessionAdvisoryLock(t *testing.T) {
	source, err := os.ReadFile("x_inbox_delivery.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		"pg_try_advisory_lock(hashtextextended($1, 0))",
		"pg_advisory_unlock(hashtextextended($1, 0))",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("x_inbox_delivery.go missing %q", required)
		}
	}
}
