package worker

import (
	"context"
	"errors"
	"fmt"
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

	ruleID            string
	subscriptionID    string
	webhookID         string
	ruleErr           error
	subscriptionErr   error
	deleteRuleErrors  map[string]error
	deleteRuleStarted chan string
	deleteRuleRelease chan struct{}

	ruleTokens         []string
	subscriptionTokens []string
	deletedRules       []string
	deletedRuleTokens  []string
	deletedSubs        []string
	deletedSubTokens   []string
	operations         []string
	webhookURLs        []string
}

func (f *fakeXInboxDeliveryAPI) EnsureFilteredStreamRule(
	_ context.Context,
	token, accountID, handle string,
) (xinbox.StreamRule, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ruleTokens = append(f.ruleTokens, token)
	f.operations = append(f.operations, "ensure-rule:"+accountID)
	if f.ruleErr != nil {
		return xinbox.StreamRule{}, f.ruleErr
	}
	return xinbox.StreamRule{ID: f.ruleID, Tag: xinbox.FilteredStreamRuleTag(accountID), Value: xinbox.FilteredStreamRuleValue(handle)}, nil
}

func (f *fakeXInboxDeliveryAPI) DeleteFilteredStreamRule(_ context.Context, token string, ruleID string) error {
	if f.deleteRuleStarted != nil {
		select {
		case f.deleteRuleStarted <- ruleID:
		default:
		}
	}
	if f.deleteRuleRelease != nil {
		<-f.deleteRuleRelease
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedRules = append(f.deletedRules, ruleID)
	f.deletedRuleTokens = append(f.deletedRuleTokens, token)
	f.operations = append(f.operations, "delete-rule:"+ruleID)
	if err := f.deleteRuleErrors[ruleID]; err != nil {
		return err
	}
	return nil
}

func (f *fakeXInboxDeliveryAPI) EnsureWebhook(_ context.Context, _ string, configuredURL string) (xinbox.Webhook, error) {
	f.webhookURLs = append(f.webhookURLs, configuredURL)
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
	mu          sync.Mutex
	accounts    []XInboxDeliveryAccount
	cleanups    []XInboxCleanupIntent
	states      []XInboxDeliveryState
	listErr     error
	listCalls   int
	listStarted chan struct{}
	listRelease chan struct{}
	claimLimits []int
}

func (f *fakeXInboxDeliveryStore) ListAccounts(context.Context) ([]XInboxDeliveryAccount, error) {
	f.mu.Lock()
	f.listCalls++
	accounts := append([]XInboxDeliveryAccount(nil), f.accounts...)
	err := f.listErr
	started := f.listStarted
	release := f.listRelease
	f.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if release != nil {
		<-release
	}
	return accounts, err
}

func (f *fakeXInboxDeliveryStore) SaveState(_ context.Context, state XInboxDeliveryState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.states = append(f.states, state)
	for i := range f.accounts {
		if f.accounts[i].SocialAccountID == state.SocialAccountID {
			f.accounts[i].FilteredStreamRuleID = state.FilteredStreamRuleID
			f.accounts[i].ActivityDMSubscriptionID = state.ActivityDMSubscriptionID
		}
	}
	return nil
}

func (f *fakeXInboxDeliveryStore) ClaimCleanupIntents(
	_ context.Context,
	owner string,
	now time.Time,
	leaseUntil time.Time,
	limit int,
) ([]XInboxCleanupIntent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claimLimits = append(f.claimLimits, limit)
	var claimed []XInboxCleanupIntent
	for i := range f.cleanups {
		if len(claimed) >= limit ||
			f.cleanups[i].NextAttemptAt.After(now) ||
			(!f.cleanups[i].LeaseUntil.IsZero() && f.cleanups[i].LeaseUntil.After(now)) {
			continue
		}
		f.cleanups[i].LeaseOwner = owner
		f.cleanups[i].LeaseUntil = leaseUntil
		f.cleanups[i].Attempts++
		claimed = append(claimed, f.cleanups[i])
	}
	return claimed, nil
}

func TestXInboxDeliveryCancelsRemovedStreamBeforeBudgetedCleanup(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	leader := &sharedTestLeader{}
	runner := &managedStreamRunner{
		starts: make(chan XInboxAppStream, 2),
		stops:  make(chan string, 2),
	}
	account := activeManagedXInboxAccount()
	account.FilteredStreamRuleID = "active-rule"
	account.Scopes = []string{"tweet.read", "tweet.write", "users.read"}
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
	api := &fakeXInboxDeliveryAPI{
		deleteRuleStarted: make(chan string, 1),
		deleteRuleRelease: make(chan struct{}),
	}
	first := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store:                           store,
		API:                             api,
		Cipher:                          fakeXInboxCipher{},
		Leader:                          leader,
		Stream:                          runner,
		ManagedAppBearer:                "managed-token",
		ManagedConsumerSecretConfigured: true,
		EventHandler:                    func(context.Context, string, xinbox.StreamEvent) error { return nil },
		CleanupOwner:                    "worker-one",
		Now:                             func() time.Time { return now },
	})
	second := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store:                           store,
		API:                             &fakeXInboxDeliveryAPI{},
		Cipher:                          fakeXInboxCipher{},
		Leader:                          leader,
		ManagedAppBearer:                "managed-token",
		ManagedConsumerSecretConfigured: true,
		CleanupOwner:                    "worker-two",
		Now:                             func() time.Time { return now },
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	first.reconcileAndStartStreams(ctx)
	<-runner.starts
	store.mu.Lock()
	store.accounts = nil
	for i := 0; i < 100; i++ {
		store.cleanups = append(store.cleanups, XInboxCleanupIntent{
			ID:                   fmt.Sprintf("cleanup-%03d", i),
			SocialAccountID:      fmt.Sprintf("deleted-%03d", i),
			AppMode:              xinbox.AppModeUniPostManaged,
			FilteredStreamRuleID: fmt.Sprintf("cleanup-rule-%03d", i),
			NextAttemptAt:        now,
		})
	}
	store.mu.Unlock()

	firstDone := make(chan struct{})
	go func() {
		first.reconcileAndStartStreams(ctx)
		close(firstDone)
	}()

	select {
	case stopped := <-runner.stops:
		if stopped != safeAppIdentity("managed-route-key") {
			t.Fatalf("stopped = %q", stopped)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("removed stream cancellation was delayed by cleanup backlog")
	}
	<-api.deleteRuleStarted

	secondDone := make(chan error, 1)
	go func() { secondDone <- second.ReconcileOnce(ctx) }()
	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("other replica reconciliation was blocked by cleanup processing")
	}

	store.mu.Lock()
	if len(store.claimLimits) == 0 || store.claimLimits[0] > 10 {
		t.Fatalf("cleanup claim limits = %v, want first batch <= 10", store.claimLimits)
	}
	store.mu.Unlock()
	close(api.deleteRuleRelease)
	<-firstDone
}

func (f *fakeXInboxDeliveryStore) ReleaseCleanupIntent(
	_ context.Context,
	intent XInboxCleanupIntent,
	nextAttemptAt time.Time,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.cleanups {
		if f.cleanups[i].ID == intent.ID && f.cleanups[i].LeaseOwner == intent.LeaseOwner {
			intent.LeaseOwner = ""
			intent.LeaseUntil = time.Time{}
			intent.NextAttemptAt = nextAttemptAt
			f.cleanups[i] = intent
			return nil
		}
	}
	return errors.New("cleanup lease lost")
}

func (f *fakeXInboxDeliveryStore) CompleteCleanupIntent(_ context.Context, id, owner string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.cleanups {
		if f.cleanups[i].ID == id && f.cleanups[i].LeaseOwner == owner {
			f.cleanups = append(f.cleanups[:i], f.cleanups[i+1:]...)
			return nil
		}
	}
	return errors.New("cleanup lease lost")
}

type fakeXInboxUsageReader struct {
	snapshot xcredits.Snapshot
}

func (f fakeXInboxUsageReader) Snapshot(context.Context, string, time.Time) (xcredits.Snapshot, error) {
	return f.snapshot, nil
}

func activeManagedXInboxAccount() XInboxDeliveryAccount {
	return XInboxDeliveryAccount{
		SocialAccountID:          "account-1",
		WorkspaceID:              "workspace-1",
		Handle:                   "UniPostDev",
		ExternalUserID:           "2244994945",
		WebhookRouteKey:          "managed-route-key",
		AccessTokenEncrypted:     "encrypted-user-token",
		AppMode:                  xinbox.AppModeUniPostManaged,
		ConsumerSecretConfigured: true,
		Scopes:                   xinbox.RequiredInboxScopes(),
		AccountActive:            true,
		PlanAllowsInbox:          true,
	}
}

func TestXInboxDeliveryReconcilePersistsRuleAndPrivateDMSubscription(t *testing.T) {
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{activeManagedXInboxAccount()}}
	api := &fakeXInboxDeliveryAPI{ruleID: "rule-1", webhookID: "webhook-1", subscriptionID: "subscription-1"}
	worker := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store:                           store,
		API:                             api,
		Cipher:                          fakeXInboxCipher{values: map[string]string{"encrypted-user-token": "user-oauth-token"}},
		Usage:                           fakeXInboxUsageReader{},
		ManagedAppBearer:                "managed-app-token",
		ManagedConsumerSecretConfigured: true,
		WebhookURL:                      "https://dev-api.unipost.dev/v1/webhooks/twitter",
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
	if want := []string{"https://dev-api.unipost.dev/v1/webhooks/twitter/managed-route-key"}; !reflect.DeepEqual(api.webhookURLs, want) {
		t.Fatalf("webhook URLs = %v, want app-specific URL %v", api.webhookURLs, want)
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
		Store:                           store,
		API:                             api,
		Cipher:                          fakeXInboxCipher{values: map[string]string{"encrypted-user-token": "user-oauth-token"}},
		Usage:                           fakeXInboxUsageReader{},
		ManagedAppBearer:                "managed-app-token",
		ManagedConsumerSecretConfigured: true,
		WebhookURL:                      "https://dev-api.unipost.dev/v1/webhooks/twitter",
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

func TestXInboxDeliveryAfterConsumerSecretRemovalCleansAndDoesNotRecreate(t *testing.T) {
	account := activeManagedXInboxAccount()
	account.AppMode = xinbox.AppModeWorkspace
	account.WebhookRouteKey = "workspace-route-key"
	account.AppBearerTokenEncrypted = "workspace-encrypted-bearer"
	account.ConsumerSecretConfigured = false
	store := &fakeXInboxDeliveryStore{
		accounts: []XInboxDeliveryAccount{account},
		cleanups: []XInboxCleanupIntent{{
			ID:                       "consumer-secret-removal-cleanup",
			SocialAccountID:          account.SocialAccountID,
			AppMode:                  xinbox.AppModeWorkspace,
			AppBearerTokenEncrypted:  "workspace-encrypted-bearer",
			FilteredStreamRuleID:     "existing-rule",
			ActivityDMSubscriptionID: "existing-subscription",
		}},
	}
	api := &fakeXInboxDeliveryAPI{ruleID: "must-not-create", subscriptionID: "must-not-create"}
	worker := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store: store,
		API:   api,
		Cipher: fakeXInboxCipher{values: map[string]string{
			"workspace-encrypted-bearer": "workspace-bearer",
		}},
		WebhookURL: "https://dev-api.unipost.dev/v1/webhooks/twitter",
	})

	if err := worker.ReconcileOnce(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "consumer_secret") {
		t.Fatalf("reconcile error = %v, want missing consumer_secret", err)
	}
	got := store.states[len(store.states)-1]
	if got.FilteredStreamRuleID != "" || got.ActivityDMSubscriptionID != "" {
		t.Fatalf("state = %+v, want existing resources cleaned", got)
	}
	if got.DeliveryStatus != xinbox.DeliveryStatusError ||
		!strings.Contains(got.LastError, "consumer_secret") {
		t.Fatalf("state = %+v, want consistent missing credential error", got)
	}
	if len(api.ruleTokens) != 0 || len(api.subscriptionTokens) != 0 {
		t.Fatalf(
			"creation calls = rules:%v subscriptions:%v, want none",
			api.ruleTokens,
			api.subscriptionTokens,
		)
	}
	if want := []string{"existing-rule"}; !reflect.DeepEqual(api.deletedRules, want) {
		t.Fatalf("deleted rules = %v, want %v", api.deletedRules, want)
	}
	if want := []string{"existing-subscription"}; !reflect.DeepEqual(api.deletedSubs, want) {
		t.Fatalf("deleted subscriptions = %v, want %v", api.deletedSubs, want)
	}
}

func TestXInboxDeliveryManagedMissingConsumerSecretStaysDisabled(t *testing.T) {
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{activeManagedXInboxAccount()}}
	api := &fakeXInboxDeliveryAPI{ruleID: "must-not-create", subscriptionID: "must-not-create"}
	worker := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store:                           store,
		API:                             api,
		Cipher:                          fakeXInboxCipher{},
		ManagedAppBearer:                "managed-app-token",
		ManagedConsumerSecretConfigured: false,
		WebhookURL:                      "https://dev-api.unipost.dev/v1/webhooks/twitter",
	})

	if err := worker.ReconcileOnce(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "TWITTER_CONSUMER_SECRET") {
		t.Fatalf("reconcile error = %v, want missing managed consumer secret", err)
	}
	got := store.states[len(store.states)-1]
	if got.FilteredStreamRuleID != "" || got.ActivityDMSubscriptionID != "" ||
		got.DeliveryStatus != xinbox.DeliveryStatusError ||
		!strings.Contains(got.LastError, "TWITTER_CONSUMER_SECRET") {
		t.Fatalf("state = %+v, want disabled missing credential state", got)
	}
	if len(api.ruleTokens) != 0 || len(api.subscriptionTokens) != 0 {
		t.Fatalf(
			"creation calls = rules:%v subscriptions:%v, want none",
			api.ruleTokens,
			api.subscriptionTokens,
		)
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

func TestXInboxCredentialReplacementReconcilesNewResourcesBeforeCleaningOldApp(t *testing.T) {
	account := activeManagedXInboxAccount()
	account.AppMode = xinbox.AppModeWorkspace
	account.AppBearerTokenEncrypted = "new-encrypted-app-token"
	account.Scopes = []string{"tweet.read", "tweet.write", "users.read"}
	store := &fakeXInboxDeliveryStore{
		accounts: []XInboxDeliveryAccount{account},
		cleanups: []XInboxCleanupIntent{{
			ID:                      "replacement-cleanup",
			SocialAccountID:         account.SocialAccountID,
			AppMode:                 xinbox.AppModeWorkspace,
			AppBearerTokenEncrypted: "old-encrypted-app-token",
			FilteredStreamRuleID:    "old-exact-rule",
		}},
	}
	api := &fakeXInboxDeliveryAPI{ruleID: "new-rule"}
	worker := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store: store,
		API:   api,
		Cipher: fakeXInboxCipher{values: map[string]string{
			"new-encrypted-app-token": "new-workspace-app-token",
			"old-encrypted-app-token": "old-workspace-app-token",
		}},
	})

	if err := worker.ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.states) == 0 || store.states[len(store.states)-1].FilteredStreamRuleID != "new-rule" {
		t.Fatalf("states = %+v, want a new resource persisted", store.states)
	}
	if want := []string{
		"ensure-rule:" + account.SocialAccountID,
		"delete-rule:old-exact-rule",
	}; !reflect.DeepEqual(api.operations, want) {
		t.Fatalf("operations = %v, want new resource before old cleanup %v", api.operations, want)
	}
	if want := []string{"new-workspace-app-token"}; !reflect.DeepEqual(api.ruleTokens, want) {
		t.Fatalf("new resource tokens = %v, want %v", api.ruleTokens, want)
	}
	if want := []string{"old-workspace-app-token"}; !reflect.DeepEqual(api.deletedRuleTokens, want) {
		t.Fatalf("old cleanup tokens = %v, want %v", api.deletedRuleTokens, want)
	}
	if len(store.cleanups) != 0 {
		t.Fatalf("cleanup intents = %+v, want completed", store.cleanups)
	}
}

func TestXInboxCleanupProcessesMultipleAppGenerationsForOneAccount(t *testing.T) {
	store := &fakeXInboxDeliveryStore{cleanups: []XInboxCleanupIntent{
		{
			ID:                      "cleanup-generation-a",
			CleanupKey:              "key-generation-a",
			SocialAccountID:         "same-account",
			AppMode:                 xinbox.AppModeWorkspace,
			SourceAppIdentity:       "app-a",
			AppBearerTokenEncrypted: "encrypted-app-a",
			FilteredStreamRuleID:    "rule-a",
		},
		{
			ID:                       "cleanup-generation-b",
			CleanupKey:               "key-generation-b",
			SocialAccountID:          "same-account",
			AppMode:                  xinbox.AppModeWorkspace,
			SourceAppIdentity:        "app-b",
			AppBearerTokenEncrypted:  "encrypted-app-b",
			FilteredStreamRuleID:     "rule-b",
			ActivityDMSubscriptionID: "subscription-b",
		},
	}}
	api := &fakeXInboxDeliveryAPI{}
	worker := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store: store,
		API:   api,
		Cipher: fakeXInboxCipher{values: map[string]string{
			"encrypted-app-a": "app-a-token",
			"encrypted-app-b": "app-b-token",
		}},
	})

	if err := worker.ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := []string{"rule-a", "rule-b"}; !reflect.DeepEqual(api.deletedRules, want) {
		t.Fatalf("deleted rules = %v, want both generations %v", api.deletedRules, want)
	}
	if want := []string{"app-a-token", "app-b-token"}; !reflect.DeepEqual(api.deletedRuleTokens, want) {
		t.Fatalf("deleted rule tokens = %v, want generation-specific tokens %v", api.deletedRuleTokens, want)
	}
	if want := []string{"subscription-b"}; !reflect.DeepEqual(api.deletedSubs, want) {
		t.Fatalf("deleted subscriptions = %v, want %v", api.deletedSubs, want)
	}
	if len(store.cleanups) != 0 {
		t.Fatalf("cleanup intents = %+v, want both generations completed", store.cleanups)
	}
}

func TestXInboxCleanupClaimsAreMutuallyExclusiveAcrossWorkers(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := &fakeXInboxDeliveryStore{cleanups: []XInboxCleanupIntent{
		{ID: "cleanup-1", NextAttemptAt: now},
		{ID: "cleanup-2", NextAttemptAt: now},
	}}
	type result struct {
		owner   string
		intents []XInboxCleanupIntent
	}
	results := make(chan result, 2)
	for _, owner := range []string{"worker-one", "worker-two"} {
		owner := owner
		go func() {
			intents, err := store.ClaimCleanupIntents(
				context.Background(),
				owner,
				now,
				now.Add(time.Minute),
				2,
			)
			if err != nil {
				t.Error(err)
			}
			results <- result{owner: owner, intents: intents}
		}()
	}
	first := <-results
	second := <-results
	claimed := make(map[string]string)
	for _, batch := range []result{first, second} {
		for _, intent := range batch.intents {
			if previous := claimed[intent.ID]; previous != "" {
				t.Fatalf("intent %s claimed by both %s and %s", intent.ID, previous, batch.owner)
			}
			claimed[intent.ID] = batch.owner
		}
	}
	if len(claimed) != 2 {
		t.Fatalf("claimed = %v, want every due intent exactly once", claimed)
	}
}

func TestXInboxCleanupRetryBackoffIsDeterministicAndCapped(t *testing.T) {
	tests := []struct {
		attempts int
		want     time.Duration
	}{
		{attempts: 1, want: time.Minute},
		{attempts: 2, want: 2 * time.Minute},
		{attempts: 6, want: 32 * time.Minute},
		{attempts: 7, want: time.Hour},
		{attempts: 20, want: time.Hour},
	}
	for _, tt := range tests {
		if got := cleanupRetryDelay(tt.attempts); got != tt.want {
			t.Fatalf("cleanupRetryDelay(%d) = %s, want %s", tt.attempts, got, tt.want)
		}
	}
}

func TestXInboxCleanupFailureSchedulesOldIntentAndProcessesNewerDueWork(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := &fakeXInboxDeliveryStore{cleanups: []XInboxCleanupIntent{
		{
			ID:                   "cleanup-old",
			SocialAccountID:      "account-old",
			AppMode:              xinbox.AppModeUniPostManaged,
			FilteredStreamRuleID: "rule-permanent-failure",
			NextAttemptAt:        now.Add(-time.Hour),
		},
		{
			ID:                   "cleanup-new",
			SocialAccountID:      "account-new",
			AppMode:              xinbox.AppModeUniPostManaged,
			FilteredStreamRuleID: "rule-newer",
			NextAttemptAt:        now,
		},
	}}
	api := &fakeXInboxDeliveryAPI{deleteRuleErrors: map[string]error{
		"rule-permanent-failure": errors.New("permanent upstream failure"),
	}}
	worker := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store:                           store,
		API:                             api,
		Cipher:                          fakeXInboxCipher{},
		ManagedAppBearer:                "managed-app-token",
		ManagedConsumerSecretConfigured: true,
		CleanupOwner:                    "worker-one",
		Now:                             func() time.Time { return now },
	})

	if err := worker.ReconcileOnce(context.Background()); err == nil {
		t.Fatal("expected cleanup failure to be reported")
	}
	if len(store.cleanups) != 1 || store.cleanups[0].ID != "cleanup-old" {
		t.Fatalf("cleanup intents = %+v, want only failed old intent", store.cleanups)
	}
	failed := store.cleanups[0]
	if failed.LeaseOwner != "" {
		t.Fatalf("failed lease owner = %q, want released", failed.LeaseOwner)
	}
	if want := now.Add(time.Minute); !failed.NextAttemptAt.Equal(want) {
		t.Fatalf("failed next attempt = %s, want %s", failed.NextAttemptAt, want)
	}
	if want := []string{"rule-permanent-failure", "rule-newer"}; !reflect.DeepEqual(api.deletedRules, want) {
		t.Fatalf("deleted rules = %v, want old failure not to starve newer work %v", api.deletedRules, want)
	}
}

func TestXInboxDeliveryDailyAllowancePauseRemovesPaidSources(t *testing.T) {
	account := activeManagedXInboxAccount()
	account.FilteredStreamRuleID = "rule-1"
	account.ActivityDMSubscriptionID = "subscription-1"
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
	api := &fakeXInboxDeliveryAPI{}
	worker := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store:                           store,
		API:                             api,
		Cipher:                          fakeXInboxCipher{values: map[string]string{"encrypted-user-token": "user-oauth-token"}},
		Usage:                           fakeXInboxUsageReader{snapshot: xcredits.Snapshot{PausePaidSources: true, InboundPauseReason: xcredits.PauseReasonMonthlyAllowance}},
		ManagedAppBearer:                "managed-app-token",
		ManagedConsumerSecretConfigured: true,
		WebhookURL:                      "https://dev-api.unipost.dev/v1/webhooks/twitter",
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
	first.WebhookRouteKey = "workspace-route-one"
	first.AppBearerTokenEncrypted = "encrypted-app-one"
	first.Scopes = []string{"tweet.read", "tweet.write", "users.read"}
	second := first
	second.SocialAccountID = "account-2"
	second.WorkspaceID = "workspace-2"
	second.WebhookRouteKey = "workspace-route-two"
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
		if !app.ConsumerSecretConfigured {
			t.Fatalf("app %q lost consumer-secret availability", app.Identity)
		}
		got[app.Identity] = app.BearerToken
	}
	want := map[string]string{
		safeAppIdentity("workspace-route-one"): "workspace-app-token-one",
		safeAppIdentity("workspace-route-two"): "workspace-app-token-two",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("apps = %v, want isolated workspace apps %v", got, want)
	}
}

type sharedTestLeader struct {
	mu       sync.Mutex
	held     map[string]bool
	cancels  map[string]context.CancelFunc
	releases atomic.Int32
}

func (l *sharedTestLeader) TryAcquire(
	_ context.Context,
	key string,
	cancel context.CancelFunc,
) (XInboxLeaderLease, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held == nil {
		l.held = make(map[string]bool)
		l.cancels = make(map[string]context.CancelFunc)
	}
	if l.held[key] {
		return nil, false, nil
	}
	l.held[key] = true
	l.cancels[key] = cancel
	return testLeaderLease{release: func() {
		l.mu.Lock()
		delete(l.held, key)
		delete(l.cancels, key)
		l.releases.Add(1)
		l.mu.Unlock()
	}}, true, nil
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

type routingStreamRunner struct{}

func (routingStreamRunner) Run(
	_ context.Context,
	_ string,
	_ string,
	handler func(xinbox.StreamEvent) error,
) error {
	return handler(xinbox.StreamEvent{})
}

func TestXInboxDeliveryPassesSecretBoundRouteKeyToStreamIngestion(t *testing.T) {
	var gotRouteKey string
	worker := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Leader: &sharedTestLeader{},
		Stream: routingStreamRunner{},
	}).SetEventHandler(func(_ context.Context, routeKey string, _ xinbox.StreamEvent) error {
		gotRouteKey = routeKey
		return nil
	})
	routeKey := "secret-bound-route-key"
	if err := worker.runAppStream(context.Background(), XInboxAppStream{
		Identity:        safeAppIdentity(routeKey),
		WebhookRouteKey: routeKey,
		BearerToken:     "app-token",
	}); err != nil {
		t.Fatal(err)
	}
	if gotRouteKey != routeKey {
		t.Fatalf("ingestion route key = %q, want %q", gotRouteKey, routeKey)
	}
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

type managedStreamRunner struct {
	starts chan XInboxAppStream
	stops  chan string
}

func (r *managedStreamRunner) Run(
	ctx context.Context,
	identity string,
	bearer string,
	_ func(xinbox.StreamEvent) error,
) error {
	r.starts <- XInboxAppStream{Identity: identity, BearerToken: bearer}
	<-ctx.Done()
	r.stops <- identity
	return ctx.Err()
}

func TestXInboxDeliveryDesiredStreamsStopRemovedAndRestartChangedBearer(t *testing.T) {
	leader := &sharedTestLeader{}
	runner := &managedStreamRunner{
		starts: make(chan XInboxAppStream, 4),
		stops:  make(chan string, 4),
	}
	worker := NewXInboxDeliveryWorker(XInboxDeliveryConfig{Leader: leader, Stream: runner})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.syncDesiredStreams(ctx, []XInboxAppStream{{
		Identity:    "workspace:one",
		BearerToken: "token-one",
	}})
	if started := <-runner.starts; started.BearerToken != "token-one" {
		t.Fatalf("started = %+v", started)
	}

	worker.syncDesiredStreams(ctx, []XInboxAppStream{{
		Identity:    "workspace:one",
		BearerToken: "token-one",
	}})
	select {
	case duplicate := <-runner.starts:
		t.Fatalf("unchanged bearer restarted stream: %+v", duplicate)
	case <-time.After(20 * time.Millisecond):
	}

	worker.syncDesiredStreams(ctx, []XInboxAppStream{{
		Identity:    "workspace:one",
		BearerToken: "token-two",
	}})
	if stopped := <-runner.stops; stopped != "workspace:one" {
		t.Fatalf("stopped = %q", stopped)
	}
	if restarted := <-runner.starts; restarted.BearerToken != "token-two" {
		t.Fatalf("restarted = %+v", restarted)
	}

	worker.syncDesiredStreams(ctx, nil)
	if stopped := <-runner.stops; stopped != "workspace:one" {
		t.Fatalf("removed stream stopped = %q", stopped)
	}
	if leader.releases.Load() < 2 {
		t.Fatalf("lock releases = %d, want changed and removed streams released", leader.releases.Load())
	}
}

func TestXInboxDeliveryReconciliationLockSerializesReplicas(t *testing.T) {
	leader := &sharedTestLeader{}
	store := &fakeXInboxDeliveryStore{
		listStarted: make(chan struct{}, 1),
		listRelease: make(chan struct{}),
	}
	config := XInboxDeliveryConfig{
		Store:  store,
		API:    &fakeXInboxDeliveryAPI{},
		Cipher: fakeXInboxCipher{},
		Leader: leader,
	}
	first := NewXInboxDeliveryWorker(config)
	second := NewXInboxDeliveryWorker(config)
	firstDone := make(chan error, 1)
	go func() { firstDone <- first.ReconcileOnce(context.Background()) }()
	<-store.listStarted

	if err := second.ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	if store.listCalls != 1 {
		t.Fatalf("list calls while first replica holds lock = %d, want 1", store.listCalls)
	}
	store.mu.Unlock()
	close(store.listRelease)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
}

func TestXInboxDeliveryIncompleteAccountListRetainsPreviousDesiredStreams(t *testing.T) {
	leader := &sharedTestLeader{}
	runner := &managedStreamRunner{
		starts: make(chan XInboxAppStream, 2),
		stops:  make(chan string, 2),
	}
	account := activeManagedXInboxAccount()
	account.FilteredStreamRuleID = "rule-existing"
	account.Scopes = []string{"tweet.read", "tweet.write", "users.read"}
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
	worker := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store:                           store,
		API:                             &fakeXInboxDeliveryAPI{},
		Cipher:                          fakeXInboxCipher{},
		Leader:                          leader,
		Stream:                          runner,
		ManagedAppBearer:                "managed-token",
		ManagedConsumerSecretConfigured: true,
		EventHandler:                    func(context.Context, string, xinbox.StreamEvent) error { return nil },
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.reconcileAndStartStreams(ctx)
	<-runner.starts
	store.mu.Lock()
	store.listErr = errors.New("database unavailable")
	store.mu.Unlock()
	worker.reconcileAndStartStreams(ctx)
	select {
	case stopped := <-runner.stops:
		t.Fatalf("incomplete account list stopped existing stream %q", stopped)
	case <-time.After(20 * time.Millisecond):
	}
	worker.stopAllStreams()
}

func TestXInboxDeliveryMissingWorkspaceCredentialCancelsDesiredStream(t *testing.T) {
	leader := &sharedTestLeader{}
	runner := &managedStreamRunner{
		starts: make(chan XInboxAppStream, 2),
		stops:  make(chan string, 2),
	}
	account := activeManagedXInboxAccount()
	account.AppMode = xinbox.AppModeWorkspace
	account.WebhookRouteKey = "workspace-route-key"
	account.AppBearerTokenEncrypted = "encrypted-workspace-token"
	account.FilteredStreamRuleID = "rule-existing"
	account.Scopes = []string{"tweet.read", "tweet.write", "users.read"}
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
	worker := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store: store,
		API:   &fakeXInboxDeliveryAPI{},
		Cipher: fakeXInboxCipher{values: map[string]string{
			"encrypted-workspace-token": "workspace-token",
		}},
		Leader:       leader,
		Stream:       runner,
		EventHandler: func(context.Context, string, xinbox.StreamEvent) error { return nil },
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.reconcileAndStartStreams(ctx)
	<-runner.starts
	store.mu.Lock()
	store.accounts[0].AppBearerTokenEncrypted = ""
	store.accounts[0].FilteredStreamRuleID = ""
	store.mu.Unlock()
	worker.reconcileAndStartStreams(ctx)
	if stopped := <-runner.stops; stopped != safeAppIdentity("workspace-route-key") {
		t.Fatalf("stopped = %q", stopped)
	}
}

func TestXInboxDeliveryUsesPostgresSessionAdvisoryLock(t *testing.T) {
	source, err := os.ReadFile("x_inbox_locks.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		"pg_try_advisory_lock(hashtextextended($1, 0))",
		"pg_advisory_unlock(hashtextextended($1, 0))",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("x_inbox_locks.go missing %q", required)
		}
	}
	if strings.Contains(text, "pool.Acquire") {
		t.Fatal("stream locks must not acquire one shared API-pool connection per app")
	}
	deliverySource, err := os.ReadFile("x_inbox_delivery.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(
		string(deliverySource),
		"Leader:                          NewPostgresStreamLockManager(databaseURL)",
	) {
		t.Fatal("Postgres worker must construct the isolated lock manager from DATABASE_URL")
	}
	if !strings.Contains(
		string(deliverySource),
		"COALESCE(NULLIF(pc.consumer_secret, ''), '') <> ''",
	) {
		t.Fatal("Postgres delivery account query must carry consumer-secret availability")
	}
}
