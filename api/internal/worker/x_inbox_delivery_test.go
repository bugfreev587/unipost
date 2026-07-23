package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"net/http"
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

	ruleID                string
	subscriptionID        string
	webhookID             string
	ruleErr               error
	webhookErr            error
	subscriptionErr       error
	listSubscriptionErr   error
	createSubscriptionErr error
	deleteRuleErrors      map[string]error
	deleteSubErrors       map[string]error
	deleteSubResults      map[string][]error
	deleteRuleStarted     chan string
	deleteRuleRelease     chan struct{}

	ruleTokens               []string
	webhookTokens            []string
	subscriptionTokens       []string
	subscriptionAccounts     []string
	subscriptionUserIDs      []string
	subscriptionWebhookIDs   []string
	deletedRules             []string
	deletedRuleTokens        []string
	deletedSubs              []string
	deletedSubTokens         []string
	operations               []string
	webhookURLs              []string
	activitySubscriptions    []xinbox.ActivitySubscription
	listSubscriptionCalls    int
	listIdentityByToken      map[string]string
	listSubscriptionIdentity []string
	createSubscriptionCalls  int
	beforeCreateSubscription func()
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

func (f *fakeXInboxDeliveryAPI) EnsureWebhook(_ context.Context, token string, configuredURL string) (xinbox.Webhook, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.webhookTokens = append(f.webhookTokens, token)
	f.webhookURLs = append(f.webhookURLs, configuredURL)
	f.operations = append(f.operations, "ensure-webhook")
	if f.webhookErr != nil {
		return xinbox.Webhook{}, f.webhookErr
	}
	return xinbox.Webhook{ID: f.webhookID, URL: configuredURL, Valid: true}, nil
}

func (f *fakeXInboxDeliveryAPI) ListActivitySubscriptions(
	_ context.Context,
	appToken string,
) ([]xinbox.ActivitySubscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listSubscriptionCalls++
	identity := f.listIdentityByToken[appToken]
	if identity == "" {
		identity = "x-app:test"
	}
	f.listSubscriptionIdentity = append(f.listSubscriptionIdentity, identity)
	f.operations = append(f.operations, "list-subscriptions:"+identity)
	if f.listSubscriptionErr != nil {
		f.subscriptionTokens = append(f.subscriptionTokens, appToken)
		return nil, f.listSubscriptionErr
	}
	var providerErr *xinbox.ProviderHTTPError
	if errors.As(f.subscriptionErr, &providerErr) && providerErr.Method == http.MethodGet {
		f.subscriptionTokens = append(f.subscriptionTokens, appToken)
		return nil, f.subscriptionErr
	}
	if errors.As(f.subscriptionErr, &providerErr) && providerErr.Method == http.MethodDelete && len(f.activitySubscriptions) == 0 {
		return []xinbox.ActivitySubscription{{
			ID: "2001", EventType: "dm.received",
			Filter: xinbox.ActivityFilter{UserID: "old-provider-user"},
			Tag:    xinbox.DMSubscriptionTag("account-1"), WebhookID: "1001",
		}}, nil
	}
	return append([]xinbox.ActivitySubscription(nil), f.activitySubscriptions...), nil
}

func (f *fakeXInboxDeliveryAPI) CreateDMSubscription(
	_ context.Context,
	appToken string,
	accountID, userID, webhookID string,
) (xinbox.ActivitySubscription, error) {
	if f.beforeCreateSubscription != nil {
		f.beforeCreateSubscription()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createSubscriptionCalls++
	f.subscriptionTokens = append(f.subscriptionTokens, appToken)
	f.subscriptionAccounts = append(f.subscriptionAccounts, accountID)
	f.subscriptionUserIDs = append(f.subscriptionUserIDs, userID)
	f.subscriptionWebhookIDs = append(f.subscriptionWebhookIDs, webhookID)
	f.operations = append(f.operations, "ensure-subscription:"+accountID)
	if f.createSubscriptionErr != nil {
		return xinbox.ActivitySubscription{}, f.createSubscriptionErr
	}
	var providerErr *xinbox.ProviderHTTPError
	if f.subscriptionErr != nil && (!errors.As(f.subscriptionErr, &providerErr) || providerErr.Method == http.MethodPost) {
		return xinbox.ActivitySubscription{}, f.subscriptionErr
	}
	subscription := xinbox.ActivitySubscription{
		ID:        f.subscriptionID,
		EventType: "dm.received",
		Filter:    xinbox.ActivityFilter{UserID: userID},
		Tag:       xinbox.DMSubscriptionTag(accountID),
		WebhookID: webhookID,
	}
	f.activitySubscriptions = append(f.activitySubscriptions, subscription)
	return subscription, nil
}

func (f *fakeXInboxDeliveryAPI) DeleteActivitySubscription(_ context.Context, appToken string, subscriptionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedSubs = append(f.deletedSubs, subscriptionID)
	f.deletedSubTokens = append(f.deletedSubTokens, appToken)
	f.operations = append(f.operations, "delete-subscription:"+subscriptionID)
	if results := f.deleteSubResults[subscriptionID]; len(results) > 0 {
		result := results[0]
		f.deleteSubResults[subscriptionID] = results[1:]
		if result != nil &&
			!xinbox.IsProviderHTTPStatus(result, http.StatusNotFound) &&
			!xinbox.IsProviderHTTPStatus(result, http.StatusGone) {
			return result
		}
		for i := range f.activitySubscriptions {
			if f.activitySubscriptions[i].ID == subscriptionID {
				f.activitySubscriptions = append(f.activitySubscriptions[:i], f.activitySubscriptions[i+1:]...)
				break
			}
		}
		return nil
	}
	if err := f.deleteSubErrors[subscriptionID]; err != nil {
		return err
	}
	var providerErr *xinbox.ProviderHTTPError
	if errors.As(f.subscriptionErr, &providerErr) && providerErr.Method == http.MethodDelete {
		return f.subscriptionErr
	}
	for i := range f.activitySubscriptions {
		if f.activitySubscriptions[i].ID == subscriptionID {
			f.activitySubscriptions = append(f.activitySubscriptions[:i], f.activitySubscriptions[i+1:]...)
			break
		}
	}
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
	saveCalls   int
	saveErrors  map[int]error
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
	f.saveCalls++
	if err := f.saveErrors[f.saveCalls]; err != nil {
		return err
	}
	f.states = append(f.states, state)
	for i := range f.accounts {
		if f.accounts[i].SocialAccountID == state.SocialAccountID {
			f.accounts[i].FilteredStreamRuleID = state.FilteredStreamRuleID
			f.accounts[i].ActivityDMSubscriptionID = state.ActivityDMSubscriptionID
			f.accounts[i].ActivityWebhookRouteKey = state.ActivityWebhookRouteKey
			f.accounts[i].DMSubscriptionForbiddenFingerprint = state.DMSubscriptionForbiddenFingerprint
		}
	}
	return nil
}

func TestFakeXInboxDeliveryStorePersistsForbiddenFingerprintIndependentlyFromLastError(t *testing.T) {
	for _, test := range []struct {
		name        string
		fingerprint string
		lastError   string
	}{
		{name: "non-empty latch with empty error", fingerprint: "credentials-v1"},
		{name: "empty latch with human-readable error", lastError: "X rejected DM subscription provisioning"},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{{SocialAccountID: "account-1"}}}
			state := XInboxDeliveryState{
				SocialAccountID:                    "account-1",
				DMSubscriptionForbiddenFingerprint: test.fingerprint,
				LastError:                          test.lastError,
			}
			if err := store.SaveState(context.Background(), state); err != nil {
				t.Fatal(err)
			}

			accounts, err := store.ListAccounts(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(accounts) != 1 {
				t.Fatalf("account count = %d, want 1", len(accounts))
			}
			if got := accounts[0].DMSubscriptionForbiddenFingerprint; got != test.fingerprint {
				t.Fatalf("forbidden fingerprint = %q, want %q", got, test.fingerprint)
			}
			if got := store.states[0].LastError; got != test.lastError {
				t.Fatalf("last error = %q, want %q", got, test.lastError)
			}
		})
	}
}

func TestPostgresXInboxDeliveryConfigMappingDrivesObservableDMGates(t *testing.T) {
	t.Run("canary account receives exact callback contract and error", func(t *testing.T) {
		account := activeManagedXInboxAccount()
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
		api := &fakeXInboxDeliveryAPI{ruleID: "rule-1"}
		canary := map[string]struct{}{account.SocialAccountID: {}}
		sentinel := errors.New("feature evaluator unavailable")
		ctx := context.WithValue(context.Background(), xInboxDeliveryContextKey{}, &struct{}{})
		var callbackCtx context.Context
		var callbackWorkspaceID string
		callbackCalls := 0
		config := xInboxDeliveryConfigFromPostgres(
			PostgresXInboxDeliveryConfig{
				Usage:                           &fakeXInboxUsageReader{},
				ManagedAppBearer:                "managed-app-token",
				ManagedConsumerSecretConfigured: true,
				WebhookURL:                      "https://dev-api.unipost.dev/v1/webhooks/twitter",
				DMsAvailable: func(gotCtx context.Context, workspaceID string) (bool, error) {
					callbackCalls++
					callbackCtx = gotCtx
					callbackWorkspaceID = workspaceID
					return false, sentinel
				},
				DMCanaryAccountIDs: canary,
			},
			store,
			api,
			fakeXInboxCipher{},
			nil,
			nil,
		)
		subject := NewXInboxDeliveryWorker(config)
		delete(canary, account.SocialAccountID)

		err := subject.ReconcileOnce(ctx)

		if !errors.Is(err, sentinel) {
			t.Fatalf("ReconcileOnce() error = %v, want sentinel evaluator error", err)
		}
		if callbackCalls != 1 || callbackCtx != ctx || callbackWorkspaceID != account.WorkspaceID {
			t.Fatalf("callback calls = %d, context identity preserved = %v, workspace = %q; want 1, true, %q",
				callbackCalls, callbackCtx == ctx, callbackWorkspaceID, account.WorkspaceID)
		}
		if len(api.webhookURLs) != 0 || len(api.subscriptionAccounts) != 0 {
			t.Fatalf("DM provider calls = webhooks:%v subscriptions:%v, want none", api.webhookURLs, api.subscriptionAccounts)
		}
	})

	t.Run("non-canary account never evaluates or provisions DMs", func(t *testing.T) {
		account := activeManagedXInboxAccount()
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
		api := &fakeXInboxDeliveryAPI{ruleID: "rule-1"}
		callbackCalls := 0
		config := xInboxDeliveryConfigFromPostgres(
			PostgresXInboxDeliveryConfig{
				Usage:                           &fakeXInboxUsageReader{},
				ManagedAppBearer:                "managed-app-token",
				ManagedConsumerSecretConfigured: true,
				WebhookURL:                      "https://dev-api.unipost.dev/v1/webhooks/twitter",
				DMsAvailable: func(context.Context, string) (bool, error) {
					callbackCalls++
					return true, nil
				},
				DMCanaryAccountIDs: map[string]struct{}{},
			},
			store,
			api,
			fakeXInboxCipher{},
			nil,
			nil,
		)

		if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err != nil {
			t.Fatalf("ReconcileOnce() error = %v", err)
		}
		if callbackCalls != 0 || len(api.webhookURLs) != 0 || len(api.subscriptionAccounts) != 0 {
			t.Fatalf("non-canary DM activity = callback:%d webhooks:%v subscriptions:%v, want none",
				callbackCalls, api.webhookURLs, api.subscriptionAccounts)
		}
	})
}

type xInboxDeliveryContextKey struct{}

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
		Usage:                           &fakeXInboxUsageReader{},
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
	err      error
	calls    int
}

func (f *fakeXInboxUsageReader) Snapshot(context.Context, string, time.Time) (xcredits.Snapshot, error) {
	f.calls++
	return f.snapshot, f.err
}

func activeManagedXInboxAccount() XInboxDeliveryAccount {
	return XInboxDeliveryAccount{
		SocialAccountID:          "account-1",
		WorkspaceID:              "workspace-1",
		Handle:                   "UniPostDev",
		ExternalAccountID:        "2244994945",
		WebhookRouteKey:          "managed-route-key",
		AppMode:                  xinbox.AppModeUniPostManaged,
		ConsumerSecretConfigured: true,
		ActivityWebhookRouteKey:  "managed-route-key",
		Scopes:                   xinbox.RequiredInboxScopes(),
		AccountActive:            true,
		PlanAllowsInbox:          true,
	}
}

func fakeDMActivitySubscription(id string, account XInboxDeliveryAccount, webhookID string) xinbox.ActivitySubscription {
	return xinbox.ActivitySubscription{
		ID: id, EventType: "dm.received",
		Filter: xinbox.ActivityFilter{UserID: account.ExternalAccountID},
		Tag:    xinbox.DMSubscriptionTag(account.SocialAccountID), WebhookID: webhookID,
	}
}

func enabledXInboxDeliveryConfig(
	store XInboxDeliveryStore,
	api XInboxDeliveryAPI,
) XInboxDeliveryConfig {
	return XInboxDeliveryConfig{
		Store:                           store,
		API:                             api,
		Cipher:                          fakeXInboxCipher{},
		Usage:                           &fakeXInboxUsageReader{},
		ManagedAppBearer:                "managed-app-token",
		ManagedConsumerSecretConfigured: true,
		WebhookURL:                      "https://dev-api.unipost.dev/v1/webhooks/twitter",
		DMsAvailable:                    func(context.Context, string) (bool, error) { return true, nil },
		DMCanaryAccountIDs:              map[string]struct{}{"account-1": {}},
	}
}

func TestXInboxDeliveryActivitySubscriptionCatalogIsCycleScopedPerApp(t *testing.T) {
	dmOnly := []string{"dm.read", "dm.write", "users.read"}

	t.Run("same app global off accounts list once and clean exact tags", func(t *testing.T) {
		first := activeManagedXInboxAccount()
		first.Scopes = dmOnly
		first.ActivityDMSubscriptionID = "2001"
		second := first
		second.SocialAccountID = "account-2"
		second.WorkspaceID = "workspace-2"
		second.ExternalAccountID = "2244994946"
		second.ActivityDMSubscriptionID = "2002"
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{first, second}}
		identity := safeAppIdentity(first.WebhookRouteKey)
		api := &fakeXInboxDeliveryAPI{
			listIdentityByToken: map[string]string{"managed-app-token": identity},
			activitySubscriptions: []xinbox.ActivitySubscription{
				fakeDMActivitySubscription("2001", first, "1001"),
				fakeDMActivitySubscription("2002", second, "1001"),
			},
		}
		config := enabledXInboxDeliveryConfig(store, api)
		config.DMsAvailable = func(context.Context, string) (bool, error) { return false, nil }
		config.DMCanaryAccountIDs = map[string]struct{}{"account-1": {}, "account-2": {}}

		if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if api.listSubscriptionCalls != 1 || !reflect.DeepEqual(api.listSubscriptionIdentity, []string{identity}) {
			t.Fatalf("list calls=%d identities=%v, want one safe app identity %q", api.listSubscriptionCalls, api.listSubscriptionIdentity, identity)
		}
		if want := []string{"2001", "2002"}; !reflect.DeepEqual(api.deletedSubs, want) {
			t.Fatalf("deleted subscriptions=%v, want %v", api.deletedSubs, want)
		}
		wantOperations := []string{
			"list-subscriptions:" + identity,
			"delete-subscription:2001",
			"delete-subscription:2002",
		}
		if !reflect.DeepEqual(api.operations, wantOperations) {
			t.Fatalf("operations=%v, want list before mutation %v", api.operations, wantOperations)
		}
	})

	t.Run("different workspace apps each list once", func(t *testing.T) {
		first := activeManagedXInboxAccount()
		first.AppMode = xinbox.AppModeWorkspace
		first.AppBearerTokenEncrypted = "encrypted-a"
		first.WebhookRouteKey = "workspace-route-a"
		first.ActivityWebhookRouteKey = first.WebhookRouteKey
		first.Scopes = dmOnly
		first.ActivityDMSubscriptionID = "2101"
		second := first
		second.SocialAccountID = "account-2"
		second.WorkspaceID = "workspace-2"
		second.ExternalAccountID = "2244994946"
		second.AppBearerTokenEncrypted = "encrypted-b"
		second.WebhookRouteKey = "workspace-route-b"
		second.ActivityWebhookRouteKey = second.WebhookRouteKey
		second.ActivityDMSubscriptionID = "2102"
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{first, second}}
		identityA := safeAppIdentity(first.WebhookRouteKey)
		identityB := safeAppIdentity(second.WebhookRouteKey)
		api := &fakeXInboxDeliveryAPI{
			listIdentityByToken: map[string]string{"workspace-token-a": identityA, "workspace-token-b": identityB},
			activitySubscriptions: []xinbox.ActivitySubscription{
				fakeDMActivitySubscription("2101", first, "1101"),
				fakeDMActivitySubscription("2102", second, "1102"),
			},
		}
		config := enabledXInboxDeliveryConfig(store, api)
		config.Cipher = fakeXInboxCipher{values: map[string]string{
			"encrypted-a": "workspace-token-a",
			"encrypted-b": "workspace-token-b",
		}}
		config.DMsAvailable = func(context.Context, string) (bool, error) { return false, nil }
		config.DMCanaryAccountIDs = map[string]struct{}{"account-1": {}, "account-2": {}}

		if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if api.listSubscriptionCalls != 2 || !reflect.DeepEqual(api.listSubscriptionIdentity, []string{identityA, identityB}) {
			t.Fatalf("list calls=%d identities=%v, want each app once %v", api.listSubscriptionCalls, api.listSubscriptionIdentity, []string{identityA, identityB})
		}
	})

	t.Run("list failure is cached and preserves all same app state", func(t *testing.T) {
		first := activeManagedXInboxAccount()
		first.Scopes = dmOnly
		first.ActivityDMSubscriptionID = "2201"
		first.ActivityWebhookRouteKey = "managed-route-key"
		second := first
		second.SocialAccountID = "account-2"
		second.WorkspaceID = "workspace-2"
		second.ExternalAccountID = "2244994946"
		second.ActivityDMSubscriptionID = "2202"
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{first, second}}
		identity := safeAppIdentity(first.WebhookRouteKey)
		api := &fakeXInboxDeliveryAPI{
			listSubscriptionErr: errors.New("safe discovery failure"),
			listIdentityByToken: map[string]string{"managed-app-token": identity},
		}
		config := enabledXInboxDeliveryConfig(store, api)
		config.DMsAvailable = func(context.Context, string) (bool, error) { return false, nil }
		config.DMCanaryAccountIDs = map[string]struct{}{"account-1": {}, "account-2": {}}

		err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background())
		if err == nil || !strings.Contains(err.Error(), "safe discovery failure") {
			t.Fatalf("error=%v, want cached discovery failure", err)
		}
		if api.listSubscriptionCalls != 1 || len(api.deletedSubs) != 0 || api.createSubscriptionCalls != 0 {
			t.Fatalf("list=%d deletes=%v creates=%d, want one list and zero mutation", api.listSubscriptionCalls, api.deletedSubs, api.createSubscriptionCalls)
		}
		if got := []string{store.accounts[0].ActivityDMSubscriptionID, store.accounts[1].ActivityDMSubscriptionID}; !reflect.DeepEqual(got, []string{"2201", "2202"}) {
			t.Fatalf("stored IDs=%v, want preserved", got)
		}
		if want := []string{"list-subscriptions:" + identity}; !reflect.DeepEqual(api.operations, want) {
			t.Fatalf("operations=%v, want %v", api.operations, want)
		}
	})

	t.Run("catalog reflects create for duplicate account row", func(t *testing.T) {
		account := activeManagedXInboxAccount()
		account.Scopes = dmOnly
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account, account}}
		identity := safeAppIdentity(account.WebhookRouteKey)
		api := &fakeXInboxDeliveryAPI{
			webhookID:             "1301",
			subscriptionID:        "2301",
			listIdentityByToken:   map[string]string{"managed-app-token": identity},
			activitySubscriptions: []xinbox.ActivitySubscription{},
		}
		config := enabledXInboxDeliveryConfig(store, api)

		if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if api.listSubscriptionCalls != 1 || api.createSubscriptionCalls != 1 {
			t.Fatalf("list=%d creates=%d, want one each with updated catalog reuse", api.listSubscriptionCalls, api.createSubscriptionCalls)
		}
		listIndex, createIndex := -1, -1
		for i, operation := range api.operations {
			if strings.HasPrefix(operation, "list-subscriptions:") && listIndex < 0 {
				listIndex = i
			}
			if operation == "ensure-subscription:account-1" && createIndex < 0 {
				createIndex = i
			}
		}
		if listIndex < 0 || createIndex < 0 || listIndex > createIndex {
			t.Fatalf("operations=%v, want list before create", api.operations)
		}
	})

	t.Run("catalog reflects delete for duplicate account row", func(t *testing.T) {
		account := activeManagedXInboxAccount()
		account.Scopes = dmOnly
		account.ActivityDMSubscriptionID = "2401"
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account, account}}
		identity := safeAppIdentity(account.WebhookRouteKey)
		api := &fakeXInboxDeliveryAPI{
			listIdentityByToken: map[string]string{"managed-app-token": identity},
			activitySubscriptions: []xinbox.ActivitySubscription{
				fakeDMActivitySubscription("2401", account, "1401"),
			},
		}
		config := enabledXInboxDeliveryConfig(store, api)
		config.DMsAvailable = func(context.Context, string) (bool, error) { return false, nil }

		if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if api.listSubscriptionCalls != 1 || !reflect.DeepEqual(api.deletedSubs, []string{"2401"}) {
			t.Fatalf("list=%d deletes=%v, want one list and one delete", api.listSubscriptionCalls, api.deletedSubs)
		}
		if want := []string{"list-subscriptions:" + identity, "delete-subscription:2401"}; !reflect.DeepEqual(api.operations, want) {
			t.Fatalf("operations=%v, want catalog-backed idempotence %v", api.operations, want)
		}
	})
}

func TestXInboxDeliveryDMDesiredRequiresFullConjunctionAndKeepsCommentsIndependent(t *testing.T) {
	tests := []struct {
		name          string
		mutateAccount func(*XInboxDeliveryAccount)
		mutateConfig  func(*XInboxDeliveryConfig)
		wantComment   bool
		wantUsageCall bool
		wantErr       bool
	}{
		{name: "active happy path", wantComment: true, wantUsageCall: true},
		{name: "inactive account", mutateAccount: func(a *XInboxDeliveryAccount) { a.AccountActive = false }},
		{name: "plan disallows inbox", mutateAccount: func(a *XInboxDeliveryAccount) { a.PlanAllowsInbox = false }},
		{name: "missing dm.read", mutateAccount: func(a *XInboxDeliveryAccount) {
			a.Scopes = []string{"tweet.read", "tweet.write", "dm.write", "users.read"}
		}, wantComment: true, wantUsageCall: true},
		{name: "missing dm.write", mutateAccount: func(a *XInboxDeliveryAccount) {
			a.Scopes = []string{"tweet.read", "tweet.write", "dm.read", "users.read"}
		}, wantComment: true, wantUsageCall: true},
		{name: "missing users.read for DM", mutateAccount: func(a *XInboxDeliveryAccount) {
			a.Scopes = []string{"tweet.read", "tweet.write", "dm.read", "dm.write"}
		}},
		{name: "workspace flag off", mutateConfig: func(c *XInboxDeliveryConfig) {
			c.DMsAvailable = func(context.Context, string) (bool, error) { return false, nil }
		}, wantComment: true, wantUsageCall: true},
		{name: "account outside strict canary", mutateConfig: func(c *XInboxDeliveryConfig) {
			c.DMCanaryAccountIDs = map[string]struct{}{}
		}, wantComment: true, wantUsageCall: true},
		{name: "managed app bearer absent", mutateConfig: func(c *XInboxDeliveryConfig) {
			c.ManagedAppBearer = ""
		}, wantErr: true},
		{name: "consumer secret absent", mutateConfig: func(c *XInboxDeliveryConfig) {
			c.ManagedConsumerSecretConfigured = false
		}, wantComment: true, wantUsageCall: true, wantErr: true},
		{name: "webhook URL absent", mutateConfig: func(c *XInboxDeliveryConfig) {
			c.WebhookURL = ""
		}, wantComment: true, wantUsageCall: true, wantErr: true},
		{name: "spend safety paused", mutateConfig: func(c *XInboxDeliveryConfig) {
			c.Usage = &fakeXInboxUsageReader{snapshot: xcredits.Snapshot{PausePaidSources: true}}
		}, wantUsageCall: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			account := activeManagedXInboxAccount()
			if test.mutateAccount != nil {
				test.mutateAccount(&account)
			}
			store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
			api := &fakeXInboxDeliveryAPI{ruleID: "rule-1", webhookID: "webhook-1", subscriptionID: "subscription-1"}
			config := enabledXInboxDeliveryConfig(store, api)
			if test.mutateConfig != nil {
				test.mutateConfig(&config)
			}
			usage, _ := config.Usage.(*fakeXInboxUsageReader)

			err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background())
			if test.wantErr && err == nil {
				t.Fatal("expected reconciliation error")
			}
			if !test.wantErr && err != nil {
				t.Fatal(err)
			}
			if got := len(api.ruleTokens) == 1; got != test.wantComment {
				t.Fatalf("comments created = %v, want %v; operations=%v", got, test.wantComment, api.operations)
			}
			wantDM := test.name == "active happy path"
			if got := len(api.subscriptionTokens) == 1; got != wantDM {
				t.Fatalf("DM created = %v, want %v; operations=%v", got, wantDM, api.operations)
			}
			if usage != nil && (usage.calls > 0) != test.wantUsageCall {
				t.Fatalf("usage called = %v, want %v", usage.calls > 0, test.wantUsageCall)
			}
		})
	}
}

func TestXInboxDeliveryManagedMissingSpendSafetyFailsClosed(t *testing.T) {
	t.Run("no existing resources", func(t *testing.T) {
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{activeManagedXInboxAccount()}}
		api := &fakeXInboxDeliveryAPI{ruleID: "must-not-create", webhookID: "must-not-create", subscriptionID: "must-not-create"}
		config := enabledXInboxDeliveryConfig(store, api)
		config.Usage = nil

		err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background())
		if err == nil || !strings.Contains(err.Error(), "spend safety") {
			t.Fatalf("reconcile error = %v, want missing spend safety dependency", err)
		}
		if len(api.operations) != 0 {
			t.Fatalf("provider operations = %v, want none", api.operations)
		}
	})

	t.Run("existing resources and stream", func(t *testing.T) {
		account := activeManagedXInboxAccount()
		account.FilteredStreamRuleID = "rule-existing"
		account.ActivityDMSubscriptionID = "subscription-existing"
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
		api := &fakeXInboxDeliveryAPI{}
		leader := &sharedTestLeader{}
		runner := &managedStreamRunner{starts: make(chan XInboxAppStream, 1), stops: make(chan string, 1)}
		config := enabledXInboxDeliveryConfig(store, api)
		config.Usage = nil
		config.Leader = leader
		config.Stream = runner
		config.EventHandler = func(context.Context, string, xinbox.StreamEvent) error { return nil }
		worker := NewXInboxDeliveryWorker(config)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		worker.syncDesiredStreams(ctx, []XInboxAppStream{{
			Identity:        safeAppIdentity(account.WebhookRouteKey),
			WebhookRouteKey: account.WebhookRouteKey,
			BearerToken:     "managed-app-token",
		}})
		<-runner.starts

		_, _, complete, err := worker.reconcileDesiredCycle(ctx)
		if err == nil || !strings.Contains(err.Error(), "spend safety") {
			t.Fatalf("reconcile error = %v, want missing spend safety dependency", err)
		}
		if complete {
			t.Fatal("desired account set marked complete without spend safety")
		}
		if len(api.operations) != 0 {
			t.Fatalf("provider operations = %v, want existing IDs preserved", api.operations)
		}
		select {
		case stopped := <-runner.stops:
			t.Fatalf("incomplete cycle stopped existing stream %q", stopped)
		case <-time.After(20 * time.Millisecond):
		}
	})
}

func TestXInboxDeliveryManagedSpendSafetyErrorPreservesResourcesAndRunningStream(t *testing.T) {
	account := activeManagedXInboxAccount()
	account.FilteredStreamRuleID = "rule-existing"
	account.ActivityDMSubscriptionID = "subscription-existing"
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
	api := &fakeXInboxDeliveryAPI{}
	leader := &sharedTestLeader{}
	runner := &managedStreamRunner{starts: make(chan XInboxAppStream, 2), stops: make(chan string, 2)}
	config := enabledXInboxDeliveryConfig(store, api)
	config.Usage = &fakeXInboxUsageReader{err: errors.New("usage snapshot unavailable")}
	config.Leader = leader
	config.Stream = runner
	config.EventHandler = func(context.Context, string, xinbox.StreamEvent) error { return nil }
	worker := NewXInboxDeliveryWorker(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	worker.syncDesiredStreams(ctx, []XInboxAppStream{{
		Identity:        safeAppIdentity(account.WebhookRouteKey),
		WebhookRouteKey: account.WebhookRouteKey,
		BearerToken:     "managed-app-token",
	}})
	<-runner.starts

	_, _, complete, err := worker.reconcileDesiredCycle(ctx)
	if err == nil || !strings.Contains(err.Error(), "usage snapshot unavailable") {
		t.Fatalf("reconcile error = %v, want spend snapshot error", err)
	}
	if complete {
		t.Fatal("desired account set marked complete after spend snapshot error")
	}
	if len(api.operations) != 0 {
		t.Fatalf("provider operations = %v, want zero create/delete calls", api.operations)
	}
	got := store.states[len(store.states)-1]
	if got.FilteredStreamRuleID != "rule-existing" || got.ActivityDMSubscriptionID != "subscription-existing" {
		t.Fatalf("state = %+v, want existing provider IDs preserved", got)
	}

	worker.reconcileAndStartStreams(ctx)
	select {
	case stopped := <-runner.stops:
		t.Fatalf("incomplete spend cycle stopped running stream %q", stopped)
	case restarted := <-runner.starts:
		t.Fatalf("incomplete spend cycle replaced running stream %+v", restarted)
	case <-time.After(20 * time.Millisecond):
	}
	if len(api.operations) != 0 {
		t.Fatalf("provider operations after stream sync = %v, want none", api.operations)
	}
}

func TestXInboxDeliveryWorkspaceSpendSafety(t *testing.T) {
	workspaceAccount := func() XInboxDeliveryAccount {
		account := activeManagedXInboxAccount()
		account.AppMode = xinbox.AppModeWorkspace
		account.WebhookRouteKey = "workspace-route-key"
		account.AppBearerTokenEncrypted = "encrypted-workspace-token"
		account.FilteredStreamRuleID = "workspace-rule-existing"
		account.ActivityDMSubscriptionID = "workspace-subscription-existing"
		return account
	}
	newConfig := func(store XInboxDeliveryStore, api XInboxDeliveryAPI) XInboxDeliveryConfig {
		config := enabledXInboxDeliveryConfig(store, api)
		config.Cipher = fakeXInboxCipher{values: map[string]string{
			"encrypted-workspace-token": "workspace-token",
		}}
		return config
	}

	for _, test := range []struct {
		name          string
		usage         XInboxUsageReader
		wantErr       string
		wantStatus    string
		wantDeletes   bool
		wantUsageCall bool
	}{
		{name: "missing reader", wantErr: "spend safety"},
		{name: "snapshot error", usage: &fakeXInboxUsageReader{err: errors.New("workspace usage unavailable")}, wantErr: "workspace usage unavailable", wantUsageCall: true},
		{name: "paused", usage: &fakeXInboxUsageReader{snapshot: xcredits.Snapshot{PausePaidSources: true, InboundPauseReason: xcredits.PauseReasonMonthlyAllowance}}, wantStatus: xinbox.DeliveryStatusPausedAllowance, wantDeletes: true, wantUsageCall: true},
		{name: "allowed", usage: &fakeXInboxUsageReader{}, wantStatus: xinbox.DeliveryStatusActive, wantUsageCall: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			account := workspaceAccount()
			store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
			api := &fakeXInboxDeliveryAPI{
				webhookID: "workspace-webhook", subscriptionID: account.ActivityDMSubscriptionID,
				activitySubscriptions: []xinbox.ActivitySubscription{
					fakeDMActivitySubscription(account.ActivityDMSubscriptionID, account, "workspace-webhook"),
				},
			}
			config := newConfig(store, api)
			config.Usage = test.usage

			err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background())
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("ReconcileOnce() error = %v, want %q", err, test.wantErr)
				}
				if len(api.operations) != 0 {
					t.Fatalf("provider operations = %v, want none", api.operations)
				}
				got := store.states[len(store.states)-1]
				if got.FilteredStreamRuleID != account.FilteredStreamRuleID ||
					got.ActivityDMSubscriptionID != account.ActivityDMSubscriptionID {
					t.Fatalf("state = %+v, want existing provider IDs preserved", got)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				got := store.states[len(store.states)-1]
				if got.DeliveryStatus != test.wantStatus {
					t.Fatalf("delivery status = %q, want %q", got.DeliveryStatus, test.wantStatus)
				}
				if test.wantDeletes {
					if want := []string{account.FilteredStreamRuleID}; !reflect.DeepEqual(api.deletedRules, want) {
						t.Fatalf("deleted rules = %v, want %v", api.deletedRules, want)
					}
					if want := []string{account.ActivityDMSubscriptionID}; !reflect.DeepEqual(api.deletedSubs, want) {
						t.Fatalf("deleted subscriptions = %v, want %v", api.deletedSubs, want)
					}
					if got.FilteredStreamRuleID != "" || got.ActivityDMSubscriptionID != "" {
						t.Fatalf("state = %+v, want paused resources cleared", got)
					}
				}
			}
			if usage, ok := test.usage.(*fakeXInboxUsageReader); ok && (usage.calls > 0) != test.wantUsageCall {
				t.Fatalf("usage called = %v, want %v", usage.calls > 0, test.wantUsageCall)
			}
		})
	}
}

func TestXInboxDeliveryNilDMsAvailableFailsClosedOnlyForOtherwiseEligibleAccount(t *testing.T) {
	t.Run("eligible account", func(t *testing.T) {
		account := activeManagedXInboxAccount()
		account.FilteredStreamRuleID = "rule-existing"
		account.ActivityDMSubscriptionID = "subscription-existing"
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
		api := &fakeXInboxDeliveryAPI{activitySubscriptions: []xinbox.ActivitySubscription{
			fakeDMActivitySubscription(account.ActivityDMSubscriptionID, account, "existing-webhook"),
		}}
		config := enabledXInboxDeliveryConfig(store, api)
		config.DMsAvailable = nil

		err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background())
		if err == nil || !strings.Contains(err.Error(), "DM availability evaluator is not configured") {
			t.Fatalf("ReconcileOnce() error = %v, want missing DM evaluator", err)
		}
		if want := []string{"list-subscriptions:x-app:test", "delete-subscription:subscription-existing"}; !reflect.DeepEqual(api.operations, want) {
			t.Fatalf("provider operations = %v, want exact DM cleanup only %v", api.operations, want)
		}
		got := store.states[len(store.states)-1]
		if got.FilteredStreamRuleID != "rule-existing" || got.ActivityDMSubscriptionID != "" {
			t.Fatalf("state = %+v, want comments preserved and DM subscription cleared", got)
		}
	})

	for _, test := range []struct {
		name   string
		mutate func(*XInboxDeliveryAccount, *XInboxDeliveryConfig)
	}{
		{name: "missing DM scope", mutate: func(account *XInboxDeliveryAccount, _ *XInboxDeliveryConfig) {
			account.Scopes = []string{"tweet.read", "tweet.write", "users.read"}
		}},
		{name: "outside canary", mutate: func(_ *XInboxDeliveryAccount, config *XInboxDeliveryConfig) {
			config.DMCanaryAccountIDs = map[string]struct{}{}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			account := activeManagedXInboxAccount()
			account.FilteredStreamRuleID = "rule-existing"
			store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
			api := &fakeXInboxDeliveryAPI{}
			config := enabledXInboxDeliveryConfig(store, api)
			config.DMsAvailable = nil
			test.mutate(&store.accounts[0], &config)

			if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err != nil {
				t.Fatalf("ReconcileOnce() error = %v, want nil evaluator irrelevant", err)
			}
		})
	}
}

func TestXInboxDeliveryReconcilePersistsCommentsRuleAndLegacyDMSubscription(t *testing.T) {
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{activeManagedXInboxAccount()}}
	api := &fakeXInboxDeliveryAPI{ruleID: "rule-1", webhookID: "webhook-1", subscriptionID: "subscription-1"}
	worker := NewXInboxDeliveryWorker(enabledXInboxDeliveryConfig(store, api))

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
	if want := []string{"ensure-rule:account-1", "ensure-webhook", "list-subscriptions:x-app:test", "ensure-subscription:account-1"}; !reflect.DeepEqual(api.operations, want) {
		t.Fatalf("operations = %v, want %v", api.operations, want)
	}
	if want := []string{"managed-app-token"}; !reflect.DeepEqual(api.subscriptionTokens, want) {
		t.Fatalf("subscription tokens = %v, want app bearer", api.subscriptionTokens)
	}
	if want := []string{"account-1"}; !reflect.DeepEqual(api.subscriptionAccounts, want) {
		t.Fatalf("subscription account IDs = %v, want stable tag account", api.subscriptionAccounts)
	}
	if want := []string{"2244994945"}; !reflect.DeepEqual(api.subscriptionUserIDs, want) {
		t.Fatalf("subscription user IDs = %v, want exact provider user", api.subscriptionUserIDs)
	}
	if want := []string{"webhook-1"}; !reflect.DeepEqual(api.subscriptionWebhookIDs, want) {
		t.Fatalf("subscription webhook IDs = %v, want current webhook", api.subscriptionWebhookIDs)
	}
}

func TestXInboxDeliverySourceUsesXPlatformAccountIDForActivityFilter(t *testing.T) {
	source, err := os.ReadFile("x_inbox_delivery.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, "COALESCE(sa.external_account_id, '')") {
		t.Fatal("delivery account query must load the X platform account ID")
	}
	if strings.Contains(text, "COALESCE(sa.external_user_id, '')") {
		t.Fatal("delivery account query must not use the Hosted Connect external user ID as the X user ID")
	}
}

func TestXInboxDeliveryEvaluatorErrorFailsDMClosedCleansSubscriptionAndKeepsComments(t *testing.T) {
	account := activeManagedXInboxAccount()
	account.ActivityDMSubscriptionID = "existing-subscription"
	account.ActivityWebhookRouteKey = account.WebhookRouteKey
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
	api := &fakeXInboxDeliveryAPI{
		ruleID: "rule-1",
		activitySubscriptions: []xinbox.ActivitySubscription{
			fakeDMActivitySubscription(account.ActivityDMSubscriptionID, account, "existing-webhook"),
		},
	}
	config := enabledXInboxDeliveryConfig(store, api)
	config.DMsAvailable = func(context.Context, string) (bool, error) {
		return false, errors.New("feature evaluator unavailable")
	}

	err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background())
	if err == nil || !strings.Contains(err.Error(), "feature evaluator unavailable") {
		t.Fatalf("reconcile error = %v, want evaluator error", err)
	}
	if want := []string{"existing-subscription"}; !reflect.DeepEqual(api.deletedSubs, want) {
		t.Fatalf("deleted subscriptions = %v, want %v", api.deletedSubs, want)
	}
	if len(api.subscriptionTokens) != 0 || len(api.webhookURLs) != 0 {
		t.Fatalf("DM provisioning calls = subscriptions:%v webhooks:%v, want none", api.subscriptionTokens, api.webhookURLs)
	}
	got := store.states[len(store.states)-1]
	if got.FilteredStreamRuleID != "rule-1" || got.ActivityDMSubscriptionID != "" {
		t.Fatalf("state = %+v, want comments active and DM cleaned", got)
	}
	if got.DeliveryStatus != xinbox.DeliveryStatusError || !strings.Contains(got.LastError, "dm source") {
		t.Fatalf("state = %+v, want source-specific evaluator error", got)
	}
}

func TestXInboxDeliveryEvaluatorErrorDoesNotAffectAccountWithoutDMScope(t *testing.T) {
	account := activeManagedXInboxAccount()
	account.Scopes = []string{"tweet.read", "tweet.write", "users.read"}
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
	api := &fakeXInboxDeliveryAPI{ruleID: "rule-1"}
	config := enabledXInboxDeliveryConfig(store, api)
	config.DMsAvailable = func(context.Context, string) (bool, error) {
		return false, errors.New("feature evaluator unavailable")
	}

	if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err != nil {
		t.Fatalf("comments-only reconcile error = %v, want evaluator irrelevant", err)
	}
	got := store.states[len(store.states)-1]
	if got.FilteredStreamRuleID != "rule-1" || got.DeliveryStatus != xinbox.DeliveryStatusActive || got.LastError != "" {
		t.Fatalf("state = %+v, want comments-only active state", got)
	}
}

func TestXInboxDeliveryDMDesiredWithoutCommentsAndCommentsFailureDoesNotBlockDM(t *testing.T) {
	t.Run("DM only scopes", func(t *testing.T) {
		account := activeManagedXInboxAccount()
		account.Scopes = []string{"dm.read", "dm.write", "users.read"}
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
		api := &fakeXInboxDeliveryAPI{webhookID: "webhook-1", subscriptionID: "subscription-1"}

		if err := NewXInboxDeliveryWorker(enabledXInboxDeliveryConfig(store, api)).ReconcileOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if len(api.ruleTokens) != 0 || len(api.subscriptionTokens) != 1 {
			t.Fatalf("calls = comments:%v DM:%v, want DM only", api.ruleTokens, api.subscriptionTokens)
		}
	})

	t.Run("comments provider failure", func(t *testing.T) {
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{activeManagedXInboxAccount()}}
		api := &fakeXInboxDeliveryAPI{
			ruleErr:        errors.New("comments provider unavailable"),
			webhookID:      "webhook-1",
			subscriptionID: "subscription-1",
		}
		err := NewXInboxDeliveryWorker(enabledXInboxDeliveryConfig(store, api)).ReconcileOnce(context.Background())
		if err == nil || !strings.Contains(err.Error(), "comments source") {
			t.Fatalf("reconcile error = %v, want comments source error", err)
		}
		got := store.states[len(store.states)-1]
		if got.ActivityDMSubscriptionID != "subscription-1" {
			t.Fatalf("state = %+v, want DM persisted despite comments failure", got)
		}
	})
}

func TestXInboxDeliveryRouteReplacementClearsRecordedSubscriptionBeforeProvisioning(t *testing.T) {
	account := activeManagedXInboxAccount()
	account.ActivityDMSubscriptionID = "old-subscription"
	account.ActivityWebhookRouteKey = "old-route-key"
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
	api := &fakeXInboxDeliveryAPI{
		ruleID:         "rule-1",
		webhookID:      "new-webhook",
		subscriptionID: "new-subscription",
		activitySubscriptions: []xinbox.ActivitySubscription{{
			ID: "old-subscription", EventType: "dm.received",
			Filter: xinbox.ActivityFilter{UserID: account.ExternalAccountID},
			Tag:    xinbox.DMSubscriptionTag(account.SocialAccountID), WebhookID: "old-webhook",
		}},
	}
	worker := NewXInboxDeliveryWorker(enabledXInboxDeliveryConfig(store, api))

	if err := worker.ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := []string{"old-subscription"}; !reflect.DeepEqual(api.deletedSubs, want) {
		t.Fatalf("deleted subscriptions = %v, want route-mismatched generation %v", api.deletedSubs, want)
	}
	if want := []string{"managed-app-token"}; !reflect.DeepEqual(api.webhookTokens, want) {
		t.Fatalf("EnsureWebhook tokens = %v, want managed bearer %v", api.webhookTokens, want)
	}
	got := store.states[len(store.states)-1]
	if got.ActivityDMSubscriptionID != "new-subscription" || got.ActivityWebhookRouteKey != "managed-route-key" {
		t.Fatalf("final state = %+v, want replacement persisted", got)
	}
	foundCleared := false
	for _, state := range store.states {
		if state.ActivityDMSubscriptionID == "" && state.ActivityWebhookRouteKey == "" {
			foundCleared = true
			break
		}
	}
	if !foundCleared {
		t.Fatalf("states = %+v, want old subscription cleared before replacement", store.states)
	}
	if want := []string{
		"ensure-rule:account-1",
		"ensure-webhook",
		"list-subscriptions:x-app:test",
		"delete-subscription:old-subscription",
		"ensure-subscription:account-1",
	}; !reflect.DeepEqual(api.operations, want) {
		t.Fatalf("operations = %v, want %v", api.operations, want)
	}

	api.operations = nil
	if err := worker.ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(api.deletedSubs) != 1 {
		t.Fatalf("second cycle repeated subscription cleanup: %v", api.deletedSubs)
	}
	if want := []string{"ensure-webhook", "list-subscriptions:x-app:test"}; !reflect.DeepEqual(api.operations, want) {
		t.Fatalf("second-cycle operations = %v, want exact subscription reuse without another POST %v", api.operations, want)
	}
}

func TestXInboxDeliveryWorkerOwnsRecoverableDMSubscriptionLifecycle(t *testing.T) {
	exact := func(id string) xinbox.ActivitySubscription {
		return xinbox.ActivitySubscription{
			ID:        id,
			EventType: "dm.received",
			Filter:    xinbox.ActivityFilter{UserID: "2244994945"},
			Tag:       xinbox.DMSubscriptionTag("account-1"),
			WebhookID: "1001",
		}
	}
	stale := func(id string) xinbox.ActivitySubscription {
		subscription := exact(id)
		subscription.Filter.UserID = "old-provider-user"
		return subscription
	}
	newAccount := func() XInboxDeliveryAccount {
		account := activeManagedXInboxAccount()
		account.FilteredStreamRuleID = "rule-existing"
		return account
	}

	t.Run("reuses exact subscription and ignores another tag", func(t *testing.T) {
		account := newAccount()
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
		api := &fakeXInboxDeliveryAPI{
			webhookID: "1001",
			activitySubscriptions: []xinbox.ActivitySubscription{
				exact("2002"),
				{ID: "9001", EventType: "dm.received", Filter: xinbox.ActivityFilter{UserID: "other-user"}, Tag: xinbox.DMSubscriptionTag("other-account"), WebhookID: "1001"},
			},
		}

		if err := NewXInboxDeliveryWorker(enabledXInboxDeliveryConfig(store, api)).ReconcileOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if api.createSubscriptionCalls != 0 || len(api.deletedSubs) != 0 {
			t.Fatalf("create calls=%d deletes=%v, want exact reuse without mutation", api.createSubscriptionCalls, api.deletedSubs)
		}
		if got := store.accounts[0].ActivityDMSubscriptionID; got != "2002" {
			t.Fatalf("persisted subscription = %q, want adopted 2002", got)
		}
	})

	t.Run("duplicates converge deterministically and cleanup is idempotent", func(t *testing.T) {
		account := newAccount()
		account.ActivityDMSubscriptionID = "2002"
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
		api := &fakeXInboxDeliveryAPI{
			webhookID: "1001",
			activitySubscriptions: []xinbox.ActivitySubscription{
				exact("2002"), exact("2001"), stale("2003"),
				{ID: "9001", EventType: "dm.received", Filter: xinbox.ActivityFilter{UserID: "other-user"}, Tag: xinbox.DMSubscriptionTag("other-account"), WebhookID: "1001"},
			},
		}
		worker := NewXInboxDeliveryWorker(enabledXInboxDeliveryConfig(store, api))

		if err := worker.ReconcileOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if want := []string{"2002", "2003"}; !reflect.DeepEqual(api.deletedSubs, want) {
			t.Fatalf("deleted subscriptions = %v, want deterministic duplicate/stale cleanup %v", api.deletedSubs, want)
		}
		if got := store.accounts[0].ActivityDMSubscriptionID; got != "2001" {
			t.Fatalf("persisted subscription = %q, want deterministic keeper 2001", got)
		}
		if err := worker.ReconcileOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if want := []string{"2002", "2003"}; !reflect.DeepEqual(api.deletedSubs, want) {
			t.Fatalf("second reconcile deletes = %v, want idempotent %v", api.deletedSubs, want)
		}
		if len(api.activitySubscriptions) != 2 || api.activitySubscriptions[1].Tag != xinbox.DMSubscriptionTag("other-account") {
			t.Fatalf("provider subscriptions = %+v, want keeper plus untouched other tag", api.activitySubscriptions)
		}
	})

	t.Run("missing recorded state is cleared before create", func(t *testing.T) {
		account := newAccount()
		account.ActivityDMSubscriptionID = "2999"
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
		api := &fakeXInboxDeliveryAPI{webhookID: "1001", subscriptionID: "2004"}
		api.beforeCreateSubscription = func() {
			if got := store.accounts[0].ActivityDMSubscriptionID; got != "" {
				t.Fatalf("recorded subscription at create = %q, want durable clear first", got)
			}
		}

		if err := NewXInboxDeliveryWorker(enabledXInboxDeliveryConfig(store, api)).ReconcileOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := store.accounts[0].ActivityDMSubscriptionID; got != "2004" {
			t.Fatalf("persisted subscription = %q, want replacement 2004", got)
		}
	})

	t.Run("delete or clear failure prevents replacement", func(t *testing.T) {
		for _, test := range []struct {
			name       string
			deleteErrs []error
			saveErrors map[int]error
		}{
			{name: "delete", deleteErrs: []error{errors.New("delete failed"), nil}},
			{name: "clear", saveErrors: map[int]error{1: errors.New("clear failed")}},
		} {
			t.Run(test.name, func(t *testing.T) {
				account := newAccount()
				account.ActivityDMSubscriptionID = "2002"
				store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}, saveErrors: test.saveErrors}
				api := &fakeXInboxDeliveryAPI{
					webhookID: "1001", subscriptionID: "2004",
					activitySubscriptions: []xinbox.ActivitySubscription{stale("2002")},
					deleteSubResults:      map[string][]error{"2002": test.deleteErrs},
				}
				worker := NewXInboxDeliveryWorker(enabledXInboxDeliveryConfig(store, api))

				err := worker.ReconcileOnce(context.Background())
				if err == nil {
					t.Fatal("expected lifecycle failure")
				}
				if api.createSubscriptionCalls != 0 {
					t.Fatalf("create calls = %d, want none after %s failure", api.createSubscriptionCalls, test.name)
				}
				if err := worker.ReconcileOnce(context.Background()); err != nil {
					t.Fatalf("recovery reconcile: %v", err)
				}
				if api.createSubscriptionCalls != 1 || store.accounts[0].ActivityDMSubscriptionID != "2004" {
					t.Fatalf("recovered create calls=%d state=%+v, want one replacement", api.createSubscriptionCalls, store.accounts[0])
				}
			})
		}
	})

	t.Run("list failure does not mutate subscriptions", func(t *testing.T) {
		account := newAccount()
		account.ActivityDMSubscriptionID = "2002"
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
		api := &fakeXInboxDeliveryAPI{webhookID: "1001", listSubscriptionErr: errors.New("list failed")}

		err := NewXInboxDeliveryWorker(enabledXInboxDeliveryConfig(store, api)).ReconcileOnce(context.Background())
		if err == nil || !strings.Contains(err.Error(), "list failed") {
			t.Fatalf("reconcile error = %v, want list failure", err)
		}
		if api.createSubscriptionCalls != 0 || len(api.deletedSubs) != 0 || store.accounts[0].ActivityDMSubscriptionID != "2002" {
			t.Fatalf("after list failure create=%d deletes=%v state=%+v, want no subscription mutation", api.createSubscriptionCalls, api.deletedSubs, store.accounts[0])
		}
	})

	t.Run("create failure leaves durable clear state", func(t *testing.T) {
		account := newAccount()
		account.ActivityDMSubscriptionID = "2999"
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
		api := &fakeXInboxDeliveryAPI{webhookID: "1001", subscriptionID: "2004", createSubscriptionErr: errors.New("create failed")}
		worker := NewXInboxDeliveryWorker(enabledXInboxDeliveryConfig(store, api))

		err := worker.ReconcileOnce(context.Background())
		if err == nil || !strings.Contains(err.Error(), "create failed") {
			t.Fatalf("reconcile error = %v, want create failure", err)
		}
		if api.createSubscriptionCalls != 1 || store.accounts[0].ActivityDMSubscriptionID != "" {
			t.Fatalf("after create failure calls=%d state=%+v, want one create and durable clear", api.createSubscriptionCalls, store.accounts[0])
		}
		api.createSubscriptionErr = nil
		if err := worker.ReconcileOnce(context.Background()); err != nil {
			t.Fatalf("recovery reconcile: %v", err)
		}
		if api.createSubscriptionCalls != 2 || store.accounts[0].ActivityDMSubscriptionID != "2004" {
			t.Fatalf("recovered create calls=%d state=%+v, want one successful retry", api.createSubscriptionCalls, store.accounts[0])
		}
	})

	t.Run("adoption persistence failure retries adoption without create", func(t *testing.T) {
		account := newAccount()
		account.ActivityWebhookRouteKey = ""
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}, saveErrors: map[int]error{1: errors.New("adopt failed")}}
		api := &fakeXInboxDeliveryAPI{webhookID: "1001", activitySubscriptions: []xinbox.ActivitySubscription{exact("2002")}}
		worker := NewXInboxDeliveryWorker(enabledXInboxDeliveryConfig(store, api))

		if err := worker.ReconcileOnce(context.Background()); err == nil || !strings.Contains(err.Error(), "adopt failed") {
			t.Fatalf("first reconcile error = %v, want adoption persistence failure", err)
		}
		if api.createSubscriptionCalls != 0 || store.accounts[0].ActivityDMSubscriptionID != "" {
			t.Fatalf("after failed adoption create=%d state=%+v", api.createSubscriptionCalls, store.accounts[0])
		}
		if err := worker.ReconcileOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if api.createSubscriptionCalls != 0 || store.accounts[0].ActivityDMSubscriptionID != "2002" {
			t.Fatalf("recovered adoption create=%d state=%+v", api.createSubscriptionCalls, store.accounts[0])
		}
	})

	t.Run("create persistence failure is recovered by stable-tag adoption", func(t *testing.T) {
		account := newAccount()
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}, saveErrors: map[int]error{2: errors.New("persist create failed")}}
		api := &fakeXInboxDeliveryAPI{webhookID: "1001", subscriptionID: "2004"}
		worker := NewXInboxDeliveryWorker(enabledXInboxDeliveryConfig(store, api))

		if err := worker.ReconcileOnce(context.Background()); err == nil || !strings.Contains(err.Error(), "persist create failed") {
			t.Fatalf("first reconcile error = %v, want persistence failure", err)
		}
		if api.createSubscriptionCalls != 1 || store.accounts[0].ActivityDMSubscriptionID != "" {
			t.Fatalf("after failed persist create calls=%d state=%+v", api.createSubscriptionCalls, store.accounts[0])
		}
		if err := worker.ReconcileOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if api.createSubscriptionCalls != 1 || store.accounts[0].ActivityDMSubscriptionID != "2004" {
			t.Fatalf("recovery create calls=%d state=%+v, want adoption without duplicate", api.createSubscriptionCalls, store.accounts[0])
		}
	})
}

func TestXInboxDeliveryDMDisabledCleansEntireStableTagFailClosed(t *testing.T) {
	providerSubscription := func(id, tag string) xinbox.ActivitySubscription {
		return xinbox.ActivitySubscription{
			ID: id, EventType: "dm.received",
			Filter: xinbox.ActivityFilter{UserID: "2244994945"},
			Tag:    tag, WebhookID: "1001",
		}
	}
	accountTag := xinbox.DMSubscriptionTag("account-1")
	otherTag := xinbox.DMSubscriptionTag("other-account")

	t.Run("flag off removes orphan and duplicate subscriptions only for exact account tag", func(t *testing.T) {
		account := activeManagedXInboxAccount()
		account.FilteredStreamRuleID = "rule-existing"
		account.ActivityDMSubscriptionID = ""
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
		api := &fakeXInboxDeliveryAPI{activitySubscriptions: []xinbox.ActivitySubscription{
			providerSubscription("2001", accountTag),
			providerSubscription("2002", accountTag),
			providerSubscription("9001", otherTag),
		}}
		config := enabledXInboxDeliveryConfig(store, api)
		config.DMsAvailable = func(context.Context, string) (bool, error) { return false, nil }

		if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if api.listSubscriptionCalls != 1 {
			t.Fatalf("list calls = %d, want complete discovery", api.listSubscriptionCalls)
		}
		if want := []string{"2001", "2002"}; !reflect.DeepEqual(api.deletedSubs, want) {
			t.Fatalf("deleted subscriptions = %v, want exact account tag cleanup %v", api.deletedSubs, want)
		}
		if api.createSubscriptionCalls != 0 || len(api.webhookURLs) != 0 {
			t.Fatalf("disabled DM provisioning create=%d webhooks=%v, want none", api.createSubscriptionCalls, api.webhookURLs)
		}
		if len(api.activitySubscriptions) != 1 || api.activitySubscriptions[0].Tag != otherTag {
			t.Fatalf("provider subscriptions = %+v, want untouched other tag", api.activitySubscriptions)
		}
		if got := store.accounts[0]; got.FilteredStreamRuleID != "rule-existing" || got.ActivityDMSubscriptionID != "" {
			t.Fatalf("state = %+v, want comments preserved and DM cleared", got)
		}
	})

	t.Run("evaluator failure cleans recorded and orphan subscriptions without disabling comments", func(t *testing.T) {
		account := activeManagedXInboxAccount()
		account.FilteredStreamRuleID = "rule-existing"
		account.ActivityDMSubscriptionID = "2001"
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
		api := &fakeXInboxDeliveryAPI{activitySubscriptions: []xinbox.ActivitySubscription{
			providerSubscription("2001", accountTag),
			providerSubscription("2002", accountTag),
		}}
		config := enabledXInboxDeliveryConfig(store, api)
		config.DMsAvailable = func(context.Context, string) (bool, error) { return false, errors.New("evaluator failed") }

		err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background())
		if err == nil || !strings.Contains(err.Error(), "evaluator failed") {
			t.Fatalf("reconcile error = %v, want evaluator failure", err)
		}
		if want := []string{"2001", "2002"}; !reflect.DeepEqual(api.deletedSubs, want) {
			t.Fatalf("deleted subscriptions = %v, want fail-closed cleanup %v", api.deletedSubs, want)
		}
		if got := store.accounts[0]; got.FilteredStreamRuleID != "rule-existing" || got.ActivityDMSubscriptionID != "" {
			t.Fatalf("state = %+v, want comments preserved and DM cleared", got)
		}
	})

	t.Run("incomplete discovery performs no mutation and preserves recorded state", func(t *testing.T) {
		account := activeManagedXInboxAccount()
		account.FilteredStreamRuleID = "rule-existing"
		account.ActivityDMSubscriptionID = "2001"
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
		api := &fakeXInboxDeliveryAPI{listSubscriptionErr: errors.New("discovery incomplete")}
		config := enabledXInboxDeliveryConfig(store, api)
		config.DMsAvailable = func(context.Context, string) (bool, error) { return false, nil }

		err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background())
		if err == nil || !strings.Contains(err.Error(), "discovery incomplete") {
			t.Fatalf("reconcile error = %v, want discovery failure", err)
		}
		if len(api.deletedSubs) != 0 || api.createSubscriptionCalls != 0 || len(api.webhookURLs) != 0 {
			t.Fatalf("provider mutations deletes=%v creates=%d webhooks=%v, want none", api.deletedSubs, api.createSubscriptionCalls, api.webhookURLs)
		}
		if got := store.accounts[0]; got.FilteredStreamRuleID != "rule-existing" || got.ActivityDMSubscriptionID != "2001" {
			t.Fatalf("state = %+v, want comments and recorded DM preserved", got)
		}
	})
}

func TestXInboxDeliveryCurrentRouteBaseURLReplacementUsesStableEnsureWithoutWebhookDeletion(t *testing.T) {
	account := activeManagedXInboxAccount()
	account.FilteredStreamRuleID = "rule-existing"
	account.ActivityDMSubscriptionID = "subscription-existing"
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
	api := &fakeXInboxDeliveryAPI{
		webhookID:      "current-webhook",
		subscriptionID: "subscription-current",
	}
	config := enabledXInboxDeliveryConfig(store, api)
	config.WebhookURL = "https://new-api.unipost.dev/v1/webhooks/twitter"

	if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := []string{"https://new-api.unipost.dev/v1/webhooks/twitter/managed-route-key"}; !reflect.DeepEqual(api.webhookURLs, want) {
		t.Fatalf("ensured webhook URLs = %v, want current app route %v", api.webhookURLs, want)
	}
	if want := []string{"current-webhook"}; !reflect.DeepEqual(api.subscriptionWebhookIDs, want) {
		t.Fatalf("subscription webhook IDs = %v, want ensured webhook %v", api.subscriptionWebhookIDs, want)
	}
	if got := store.states[len(store.states)-1]; got.ActivityDMSubscriptionID != "subscription-current" || got.ActivityWebhookRouteKey != "managed-route-key" {
		t.Fatalf("state = %+v, want stable ensured subscription and route", got)
	}
}

func TestXInboxDeliveryWorkerDoesNotOwnAppWebhookDeletion(t *testing.T) {
	source, err := os.ReadFile("x_inbox_delivery.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"ListWebhooks(", "DeleteWebhook(", "cleanupStaleDMWebhooks", "EnsureDMSubscription("} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("delivery worker contains forbidden app-scoped webhook cleanup dependency %q", forbidden)
		}
	}
}

func TestXInboxDeliverySharedWebhookSurvivesAnotherAccountMigrationFailure(t *testing.T) {
	first := activeManagedXInboxAccount()
	first.FilteredStreamRuleID = "rule-account-1"
	first.ActivityDMSubscriptionID = "old-subscription-account-1"
	first.ActivityWebhookRouteKey = "old-route-key"

	second := activeManagedXInboxAccount()
	second.SocialAccountID = "account-2"
	second.WorkspaceID = "workspace-2"
	second.ExternalAccountID = "provider-user-2"
	second.FilteredStreamRuleID = "rule-account-2"
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{first, second}}
	api := &fakeXInboxDeliveryAPI{
		webhookID:      "shared-webhook",
		subscriptionID: "subscription-account-2",
		activitySubscriptions: []xinbox.ActivitySubscription{{
			ID: "old-subscription-account-1", EventType: "dm.received",
			Filter: xinbox.ActivityFilter{UserID: first.ExternalAccountID},
			Tag:    xinbox.DMSubscriptionTag(first.SocialAccountID), WebhookID: "old-webhook",
		}},
		deleteSubErrors: map[string]error{
			"old-subscription-account-1": errors.New("account migration failed"),
		},
	}
	config := enabledXInboxDeliveryConfig(store, api)
	config.DMCanaryAccountIDs = map[string]struct{}{"account-1": {}, "account-2": {}}

	err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background())
	if err == nil || !strings.Contains(err.Error(), "account migration failed") {
		t.Fatalf("reconcile error = %v, want first account migration failure", err)
	}
	if want := []string{
		"https://dev-api.unipost.dev/v1/webhooks/twitter/managed-route-key",
		"https://dev-api.unipost.dev/v1/webhooks/twitter/managed-route-key",
	}; !reflect.DeepEqual(api.webhookURLs, want) {
		t.Fatalf("shared webhook ensures = %v, want each independently reconciled account to ensure the shared webhook %v", api.webhookURLs, want)
	}
	if want := []string{"account-2"}; !reflect.DeepEqual(api.subscriptionAccounts, want) {
		t.Fatalf("subscription accounts = %v, want unaffected second account %v", api.subscriptionAccounts, want)
	}
	got := store.accounts[1]
	if got.ActivityDMSubscriptionID != "subscription-account-2" || got.ActivityWebhookRouteKey != "managed-route-key" {
		t.Fatalf("second account state = %+v, want shared-webhook subscription convergence", got)
	}
}

func TestXInboxDeliveryProviderMutationPersistenceFailuresConverge(t *testing.T) {
	t.Run("subscription route clear save failure retries idempotent delete", func(t *testing.T) {
		account := activeManagedXInboxAccount()
		account.FilteredStreamRuleID = "rule-existing"
		account.ActivityDMSubscriptionID = "old-subscription"
		account.ActivityWebhookRouteKey = "old-route-key"
		store := &fakeXInboxDeliveryStore{
			accounts:   []XInboxDeliveryAccount{account},
			saveErrors: map[int]error{1: errors.New("save cleared subscription failed")},
		}
		api := &fakeXInboxDeliveryAPI{
			webhookID:      "webhook-current",
			subscriptionID: "subscription-current",
			activitySubscriptions: []xinbox.ActivitySubscription{{
				ID: "old-subscription", EventType: "dm.received",
				Filter: xinbox.ActivityFilter{UserID: account.ExternalAccountID},
				Tag:    xinbox.DMSubscriptionTag(account.SocialAccountID), WebhookID: "old-webhook",
			}},
			deleteSubResults: map[string][]error{
				"old-subscription": {
					nil,
					&xinbox.ProviderHTTPError{Method: http.MethodDelete, Path: "/2/activity/subscriptions/old-subscription", StatusCode: http.StatusNotFound},
				},
			},
		}
		worker := NewXInboxDeliveryWorker(enabledXInboxDeliveryConfig(store, api))

		if err := worker.ReconcileOnce(context.Background()); err == nil || !strings.Contains(err.Error(), "save cleared subscription failed") {
			t.Fatalf("first reconcile error = %v, want clear-state persistence failure", err)
		}
		if got := store.accounts[0]; got.ActivityDMSubscriptionID != "old-subscription" || got.ActivityWebhookRouteKey != "old-route-key" {
			t.Fatalf("state after failed save = %+v, want old local generation retained", got)
		}
		if err := worker.ReconcileOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if want := []string{"old-subscription"}; !reflect.DeepEqual(api.deletedSubs, want) {
			t.Fatalf("delete attempts = %v, want no retry after provider list proves removal %v", api.deletedSubs, want)
		}
		if got := store.accounts[0]; got.ActivityDMSubscriptionID != "subscription-current" || got.ActivityWebhookRouteKey != "managed-route-key" {
			t.Fatalf("final state = %+v, want current subscription generation", got)
		}
	})

	t.Run("filtered stream ensure save failure reuses stable rule", func(t *testing.T) {
		account := activeManagedXInboxAccount()
		account.Scopes = []string{"tweet.read", "tweet.write", "users.read"}
		store := &fakeXInboxDeliveryStore{
			accounts:   []XInboxDeliveryAccount{account},
			saveErrors: map[int]error{1: errors.New("save rule failed")},
		}
		api := &fakeXInboxDeliveryAPI{ruleID: "stable-rule"}
		worker := NewXInboxDeliveryWorker(enabledXInboxDeliveryConfig(store, api))

		if err := worker.ReconcileOnce(context.Background()); err == nil || !strings.Contains(err.Error(), "save rule failed") {
			t.Fatalf("first reconcile error = %v, want rule persistence failure", err)
		}
		if got := store.accounts[0].FilteredStreamRuleID; got != "" {
			t.Fatalf("rule after failed save = %q, want local state unchanged", got)
		}
		if err := worker.ReconcileOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if want := []string{"managed-app-token", "managed-app-token"}; !reflect.DeepEqual(api.ruleTokens, want) {
			t.Fatalf("rule ensures = %v, want stable ensure retried %v", api.ruleTokens, want)
		}
		if len(api.deletedRules) != 0 || store.accounts[0].FilteredStreamRuleID != "stable-rule" {
			t.Fatalf("deleted rules=%v final state=%+v, want one stable semantic rule", api.deletedRules, store.accounts[0])
		}
	})

	t.Run("DM ensure save failure reuses stable subscription", func(t *testing.T) {
		account := activeManagedXInboxAccount()
		account.FilteredStreamRuleID = "rule-existing"
		account.ActivityWebhookRouteKey = ""
		store := &fakeXInboxDeliveryStore{
			accounts:   []XInboxDeliveryAccount{account},
			saveErrors: map[int]error{1: errors.New("save DM subscription failed")},
		}
		api := &fakeXInboxDeliveryAPI{webhookID: "stable-webhook", subscriptionID: "stable-subscription"}
		worker := NewXInboxDeliveryWorker(enabledXInboxDeliveryConfig(store, api))

		if err := worker.ReconcileOnce(context.Background()); err == nil || !strings.Contains(err.Error(), "save DM subscription failed") {
			t.Fatalf("first reconcile error = %v, want subscription persistence failure", err)
		}
		if got := store.accounts[0].ActivityDMSubscriptionID; got != "" {
			t.Fatalf("subscription after failed save = %q, want local state unchanged", got)
		}
		if err := worker.ReconcileOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if want := []string{"stable-webhook"}; !reflect.DeepEqual(api.subscriptionWebhookIDs, want) {
			t.Fatalf("subscription webhook IDs = %v, want one create followed by stable-tag adoption %v", api.subscriptionWebhookIDs, want)
		}
		if got := store.accounts[0]; got.ActivityDMSubscriptionID != "stable-subscription" || got.ActivityWebhookRouteKey != "managed-route-key" {
			t.Fatalf("final state = %+v, want stable DM subscription and route", got)
		}
	})

	t.Run("forbidden latch save failure retries once then suppresses", func(t *testing.T) {
		account := activeManagedXInboxAccount()
		account.FilteredStreamRuleID = "rule-existing"
		account.ActivityWebhookRouteKey = ""
		store := &fakeXInboxDeliveryStore{
			accounts:   []XInboxDeliveryAccount{account},
			saveErrors: map[int]error{1: errors.New("save forbidden latch failed")},
		}
		api := &fakeXInboxDeliveryAPI{
			webhookID:       "stable-webhook",
			subscriptionErr: dmSubscriptionCreateForbidden(),
		}
		worker := NewXInboxDeliveryWorker(enabledXInboxDeliveryConfig(store, api))

		if err := worker.ReconcileOnce(context.Background()); err == nil || !strings.Contains(err.Error(), "save forbidden latch failed") {
			t.Fatalf("first reconcile error = %v, want latch persistence failure", err)
		}
		if got := store.accounts[0].DMSubscriptionForbiddenFingerprint; got != "" {
			t.Fatalf("latch after failed save = %q, want no fake durability", got)
		}
		if err := worker.ReconcileOnce(context.Background()); err == nil || !xinbox.IsProviderHTTPStatus(err, http.StatusForbidden) {
			t.Fatalf("second reconcile error = %v, want controlled provider retry and durable 403", err)
		}
		if got := store.accounts[0].DMSubscriptionForbiddenFingerprint; got == "" {
			t.Fatal("second reconcile did not persist forbidden latch")
		}
		if err := worker.ReconcileOnce(context.Background()); err == nil || !strings.Contains(err.Error(), "latched") {
			t.Fatalf("third reconcile error = %v, want durable latch suppression", err)
		}
		if len(api.subscriptionTokens) != 2 {
			t.Fatalf("subscription attempts = %d, want initial call plus one controlled retry", len(api.subscriptionTokens))
		}
	})
}

func dmSubscriptionCreateForbidden() error {
	return &xinbox.ProviderHTTPError{
		Method:     http.MethodPost,
		Path:       "/2/activity/subscriptions",
		StatusCode: http.StatusForbidden,
		Code:       "client-forbidden",
		Title:      "Forbidden",
	}
}

func TestXInboxDeliveryDMCreate403LatchesSameFingerprintWithoutDisablingComments(t *testing.T) {
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{activeManagedXInboxAccount()}}
	api := &fakeXInboxDeliveryAPI{
		ruleID:          "rule-1",
		webhookID:       "webhook-1",
		subscriptionErr: dmSubscriptionCreateForbidden(),
	}
	config := enabledXInboxDeliveryConfig(store, api)
	worker := NewXInboxDeliveryWorker(config)

	err := worker.ReconcileOnce(context.Background())
	if err == nil || !xinbox.IsProviderHTTPStatus(err, http.StatusForbidden) {
		t.Fatalf("first reconcile error = %v, want provider 403", err)
	}
	first := store.states[len(store.states)-1]
	if first.FilteredStreamRuleID != "rule-1" || first.DMSubscriptionForbiddenFingerprint == "" {
		t.Fatalf("state = %+v, want comments active and DM latch persisted", first)
	}
	if strings.Contains(first.DMSubscriptionForbiddenFingerprint, "managed-route-key") ||
		strings.Contains(first.DMSubscriptionForbiddenFingerprint, "managed-app-token") {
		t.Fatalf("fingerprint leaked configuration or secret: %q", first.DMSubscriptionForbiddenFingerprint)
	}
	api.subscriptionErr = nil
	api.operations = nil
	if err := worker.ReconcileOnce(context.Background()); err == nil || !strings.Contains(err.Error(), "latched") {
		t.Fatalf("second reconcile error = %v, want latched summary", err)
	}
	if len(api.webhookURLs) != 1 || len(api.subscriptionTokens) != 1 {
		t.Fatalf("latched cycle retried provider provisioning: webhooks=%v subscriptions=%v", api.webhookURLs, api.subscriptionTokens)
	}
	if len(api.operations) != 0 {
		t.Fatalf("latched cycle operations = %v, want no provider calls", api.operations)
	}
	last := store.states[len(store.states)-1]
	if last.FilteredStreamRuleID != "rule-1" || last.DeliveryStatus != xinbox.DeliveryStatusError {
		t.Fatalf("latched state = %+v, want comments retained with aggregate error", last)
	}
}

func TestXInboxDeliveryDMForbiddenLogIsStructuredAndSecretSafe(t *testing.T) {
	const (
		bearerSentinel  = "managed-bearer-must-not-log"
		privateSentinel = "private-dm-content-must-not-log"
	)
	account := activeManagedXInboxAccount()
	account.Handle = privateSentinel
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
	api := &fakeXInboxDeliveryAPI{
		ruleID:          "rule-1",
		webhookID:       "webhook-1",
		subscriptionErr: dmSubscriptionCreateForbidden(),
	}
	config := enabledXInboxDeliveryConfig(store, api)
	config.ManagedAppBearer = bearerSentinel

	var output bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })
	err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background())
	if err == nil || !xinbox.IsProviderHTTPStatus(err, http.StatusForbidden) {
		t.Fatalf("reconcile error = %v, want provider 403", err)
	}
	fingerprint := store.states[len(store.states)-1].DMSubscriptionForbiddenFingerprint
	if fingerprint == "" {
		t.Fatal("DM forbidden fingerprint was not persisted")
	}

	foundDMRecord := false
	for _, line := range bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n")) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("decode structured log %q: %v", line, err)
		}
		if record["source"] != "dm" {
			continue
		}
		foundDMRecord = true
		if record["action"] != "reconcile" || record["provider_http_status"] != float64(http.StatusForbidden) {
			t.Fatalf("DM record attributes = %+v, want source/action/status", record)
		}
	}
	if !foundDMRecord {
		t.Fatalf("logs = %s, want structured DM source record", output.String())
	}
	logged := output.String()
	for _, forbidden := range []string{
		bearerSentinel,
		privateSentinel,
		fingerprint,
		"consumer_secret",
		"consumer-secret",
	} {
		if strings.Contains(logged, forbidden) {
			t.Fatalf("structured logs leaked forbidden value %q: %s", forbidden, logged)
		}
	}
}

func TestXInboxDeliveryDMProvisioning403LatchCoversEnsureReadWriteButNotNon403(t *testing.T) {
	for _, test := range []struct {
		name            string
		webhookErr      error
		subscriptionErr error
		wantLatch       bool
	}{
		{
			name:       "webhook list GET forbidden",
			webhookErr: &xinbox.ProviderHTTPError{Method: http.MethodGet, Path: "/2/webhooks", StatusCode: http.StatusForbidden},
			wantLatch:  true,
		},
		{
			name:       "webhook revalidation PUT forbidden",
			webhookErr: &xinbox.ProviderHTTPError{Method: http.MethodPut, Path: "/2/webhooks/{id}", StatusCode: http.StatusForbidden},
			wantLatch:  true,
		},
		{
			name:            "subscription list GET forbidden",
			subscriptionErr: &xinbox.ProviderHTTPError{Method: http.MethodGet, Path: "/2/activity/subscriptions", StatusCode: http.StatusForbidden},
			wantLatch:       true,
		},
		{
			name:            "subscription create POST forbidden",
			subscriptionErr: dmSubscriptionCreateForbidden(),
			wantLatch:       true,
		},
		{
			name: "subscription internal DELETE forbidden",
			subscriptionErr: &xinbox.ProviderHTTPError{
				Method:     http.MethodDelete,
				Path:       "/2/activity/subscriptions/{id}",
				StatusCode: http.StatusForbidden,
			},
		},
		{
			name:            "subscription non-403 error",
			subscriptionErr: errors.New("provider unavailable"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{activeManagedXInboxAccount()}}
			api := &fakeXInboxDeliveryAPI{
				ruleID:          "rule-1",
				webhookID:       "webhook-1",
				webhookErr:      test.webhookErr,
				subscriptionErr: test.subscriptionErr,
			}
			err := NewXInboxDeliveryWorker(enabledXInboxDeliveryConfig(store, api)).ReconcileOnce(context.Background())
			if err == nil {
				t.Fatal("expected DM provisioning error")
			}
			got := store.states[len(store.states)-1].DMSubscriptionForbiddenFingerprint != ""
			if got != test.wantLatch {
				t.Fatalf("latch set = %v, want %v; state=%+v", got, test.wantLatch, store.states[len(store.states)-1])
			}
		})
	}
}

func TestXInboxDeliveryDMForbiddenLatchClearsOnlyForDeliberateGates(t *testing.T) {
	for _, test := range []struct {
		name         string
		mutateConfig func(*XInboxDeliveryConfig)
	}{
		{name: "workspace flag off", mutateConfig: func(c *XInboxDeliveryConfig) {
			c.DMsAvailable = func(context.Context, string) (bool, error) { return false, nil }
		}},
		{name: "canary removal", mutateConfig: func(c *XInboxDeliveryConfig) {
			c.DMCanaryAccountIDs = map[string]struct{}{}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			account := activeManagedXInboxAccount()
			account.FilteredStreamRuleID = "rule-existing"
			account.ActivityDMSubscriptionID = "subscription-existing"
			account.DMSubscriptionForbiddenFingerprint = "latched-fingerprint"
			store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
			api := &fakeXInboxDeliveryAPI{activitySubscriptions: []xinbox.ActivitySubscription{
				fakeDMActivitySubscription(account.ActivityDMSubscriptionID, account, "existing-webhook"),
			}}
			config := enabledXInboxDeliveryConfig(store, api)
			test.mutateConfig(&config)

			if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			got := store.states[len(store.states)-1]
			if got.DMSubscriptionForbiddenFingerprint != "" || got.ActivityDMSubscriptionID != "" {
				t.Fatalf("state = %+v, want latch and subscription cleared", got)
			}
			if want := []string{"subscription-existing"}; !reflect.DeepEqual(api.deletedSubs, want) {
				t.Fatalf("deleted subscriptions = %v, want %v", api.deletedSubs, want)
			}
			if want := []string{"list-subscriptions:x-app:test", "delete-subscription:subscription-existing"}; !reflect.DeepEqual(api.operations, want) {
				t.Fatalf("gate-off provider operations = %v, want account subscription cleanup only %v", api.operations, want)
			}
		})
	}
}

func TestXInboxDeliveryDMForbiddenLatchDeliberateOffOnAllowsOneRetry(t *testing.T) {
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{activeManagedXInboxAccount()}}
	api := &fakeXInboxDeliveryAPI{ruleID: "rule-1", webhookID: "webhook-1", subscriptionID: "subscription-1", subscriptionErr: dmSubscriptionCreateForbidden()}
	config := enabledXInboxDeliveryConfig(store, api)

	if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err == nil {
		t.Fatal("first DM attempt must return 403")
	}
	off := config
	off.DMsAvailable = func(context.Context, string) (bool, error) { return false, nil }
	if err := NewXInboxDeliveryWorker(off).ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	api.subscriptionErr = nil
	if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(api.subscriptionTokens) != 2 {
		t.Fatalf("subscription attempts = %d, want initial 403 plus one off-on retry", len(api.subscriptionTokens))
	}
	if got := store.states[len(store.states)-1]; got.ActivityDMSubscriptionID != "subscription-1" || got.DMSubscriptionForbiddenFingerprint != "" {
		t.Fatalf("state = %+v, want successful retry and cleared latch", got)
	}
}

func TestXInboxDeliveryDMForbiddenFingerprintChangesOnlyForNonSecretConfiguration(t *testing.T) {
	t.Run("provider user change retries", func(t *testing.T) {
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{activeManagedXInboxAccount()}}
		api := &fakeXInboxDeliveryAPI{ruleID: "rule-1", webhookID: "webhook-1", subscriptionErr: dmSubscriptionCreateForbidden()}
		config := enabledXInboxDeliveryConfig(store, api)
		if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err == nil {
			t.Fatal("first DM attempt must return 403")
		}
		store.accounts[0].ExternalAccountID = "different-provider-user"
		if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err == nil {
			t.Fatal("changed non-secret configuration must make one new attempt")
		}
		if len(api.subscriptionTokens) != 2 {
			t.Fatalf("subscription attempts = %d, want retry after provider user change", len(api.subscriptionTokens))
		}
	})

	t.Run("webhook route and app identity change retries", func(t *testing.T) {
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{activeManagedXInboxAccount()}}
		api := &fakeXInboxDeliveryAPI{ruleID: "rule-1", webhookID: "webhook-1", subscriptionErr: dmSubscriptionCreateForbidden()}
		config := enabledXInboxDeliveryConfig(store, api)
		if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err == nil {
			t.Fatal("first DM attempt must return 403")
		}
		store.accounts[0].WebhookRouteKey = "replacement-route-key"
		if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err == nil {
			t.Fatal("changed webhook route/app identity must make one new attempt")
		}
		if len(api.subscriptionTokens) != 2 {
			t.Fatalf("subscription attempts = %d, want retry after route/app identity change", len(api.subscriptionTokens))
		}
	})

	t.Run("webhook base URL change retries", func(t *testing.T) {
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{activeManagedXInboxAccount()}}
		api := &fakeXInboxDeliveryAPI{ruleID: "rule-1", webhookID: "webhook-1", subscriptionErr: dmSubscriptionCreateForbidden()}
		config := enabledXInboxDeliveryConfig(store, api)
		if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err == nil {
			t.Fatal("first DM attempt must return 403")
		}
		config.WebhookURL = "https://replacement-api.unipost.dev/v1/webhooks/twitter"
		if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err == nil {
			t.Fatal("changed webhook URL must make one new attempt")
		}
		if len(api.subscriptionTokens) != 2 {
			t.Fatalf("subscription attempts = %d, want retry after webhook URL change", len(api.subscriptionTokens))
		}
	})

	t.Run("app mode change retries", func(t *testing.T) {
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{activeManagedXInboxAccount()}}
		api := &fakeXInboxDeliveryAPI{ruleID: "rule-1", webhookID: "webhook-1", subscriptionErr: dmSubscriptionCreateForbidden()}
		config := enabledXInboxDeliveryConfig(store, api)
		if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err == nil {
			t.Fatal("first DM attempt must return 403")
		}
		store.accounts[0].AppMode = xinbox.AppModeWorkspace
		store.accounts[0].AppBearerTokenEncrypted = "workspace-bearer-encrypted"
		config.Cipher = fakeXInboxCipher{values: map[string]string{"workspace-bearer-encrypted": "workspace-bearer"}}
		if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err == nil {
			t.Fatal("changed app mode must make one new attempt")
		}
		if len(api.subscriptionTokens) != 2 {
			t.Fatalf("subscription attempts = %d, want retry after app mode change", len(api.subscriptionTokens))
		}
	})

	t.Run("bearer and consumer secret changes do not retry", func(t *testing.T) {
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{activeManagedXInboxAccount()}}
		api := &fakeXInboxDeliveryAPI{ruleID: "rule-1", webhookID: "webhook-1", subscriptionErr: dmSubscriptionCreateForbidden()}
		config := enabledXInboxDeliveryConfig(store, api)
		if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err == nil {
			t.Fatal("first DM attempt must return 403")
		}
		config.ManagedAppBearer = "rotated-managed-app-token"
		config.ManagedConsumerSecretConfigured = false
		if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err == nil {
			t.Fatal("missing consumer secret must remain an actionable DM error")
		}
		config.ManagedConsumerSecretConfigured = true
		api.subscriptionErr = nil
		if err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background()); err == nil || !strings.Contains(err.Error(), "latched") {
			t.Fatalf("secret-only changed reconcile error = %v, want unchanged latch", err)
		}
		if len(api.subscriptionTokens) != 1 {
			t.Fatalf("subscription attempts = %d, want no secret-only retry", len(api.subscriptionTokens))
		}
	})
}

func TestXInboxDeliveryDelete403IsHardCleanupError(t *testing.T) {
	t.Run("subscription delete", func(t *testing.T) {
		account := activeManagedXInboxAccount()
		account.FilteredStreamRuleID = "rule-existing"
		account.ActivityDMSubscriptionID = "subscription-existing"
		store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
		api := &fakeXInboxDeliveryAPI{
			activitySubscriptions: []xinbox.ActivitySubscription{
				fakeDMActivitySubscription(account.ActivityDMSubscriptionID, account, "existing-webhook"),
			},
			deleteSubErrors: map[string]error{
				"subscription-existing": &xinbox.ProviderHTTPError{Method: http.MethodDelete, Path: "/2/activity/subscriptions/{id}", StatusCode: http.StatusForbidden},
			},
		}
		config := enabledXInboxDeliveryConfig(store, api)
		config.DMsAvailable = func(context.Context, string) (bool, error) { return false, nil }
		err := NewXInboxDeliveryWorker(config).ReconcileOnce(context.Background())
		if err == nil || !xinbox.IsProviderHTTPStatus(err, http.StatusForbidden) {
			t.Fatalf("reconcile error = %v, want delete 403", err)
		}
		if got := store.states[len(store.states)-1]; got.ActivityDMSubscriptionID != "subscription-existing" {
			t.Fatalf("state = %+v, want failed cleanup ID retained", got)
		}
		if got := store.states[len(store.states)-1].DMSubscriptionForbiddenFingerprint; got != "" {
			t.Fatalf("delete 403 set provisioning latch %q", got)
		}
	})
}

func TestXInboxDeliveryLastErrorKeepsOnlyLatestSourceSummary(t *testing.T) {
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{activeManagedXInboxAccount()}}
	api := &fakeXInboxDeliveryAPI{
		ruleErr:         errors.New("comments provider unavailable"),
		webhookID:       "webhook-1",
		subscriptionErr: errors.New("DM provider unavailable"),
	}
	err := NewXInboxDeliveryWorker(enabledXInboxDeliveryConfig(store, api)).ReconcileOnce(context.Background())
	if err == nil || !strings.Contains(err.Error(), "comments source") || !strings.Contains(err.Error(), "dm source") {
		t.Fatalf("returned error = %v, want both source failures", err)
	}
	lastError := store.states[len(store.states)-1].LastError
	if !strings.HasPrefix(lastError, "dm source:") || strings.Contains(lastError, "comments source:") {
		t.Fatalf("last_error = %q, want only latest DM source summary", lastError)
	}
}

func TestXInboxDeliveryPersistsRuleWithoutAttemptingFailingSubscription(t *testing.T) {
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
		Usage:                           &fakeXInboxUsageReader{},
		ManagedAppBearer:                "managed-app-token",
		ManagedConsumerSecretConfigured: true,
		WebhookURL:                      "https://dev-api.unipost.dev/v1/webhooks/twitter",
	})

	if err := worker.ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	foundDurableRule := false
	for _, state := range store.states {
		if state.FilteredStreamRuleID == "rule-1" {
			foundDurableRule = true
		}
	}
	if !foundDurableRule {
		t.Fatalf("states = %+v, want comments rule persisted", store.states)
	}
	if len(api.subscriptionTokens) != 0 {
		t.Fatalf("subscription calls = %v, want none", api.subscriptionTokens)
	}
}

func TestXInboxDeliveryAfterConsumerSecretRemovalKeepsCommentsAndCleansPriorGeneration(t *testing.T) {
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
	api := &fakeXInboxDeliveryAPI{ruleID: "new-comments-rule", subscriptionID: "must-not-create"}
	worker := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store: store,
		API:   api,
		Cipher: fakeXInboxCipher{values: map[string]string{
			"workspace-encrypted-bearer": "workspace-bearer",
		}},
		Usage:      &fakeXInboxUsageReader{},
		WebhookURL: "https://dev-api.unipost.dev/v1/webhooks/twitter",
	})

	if err := worker.ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := store.states[len(store.states)-1]
	if got.FilteredStreamRuleID != "new-comments-rule" || got.ActivityDMSubscriptionID != "" {
		t.Fatalf("state = %+v, want comments active and DM disabled", got)
	}
	if got.DeliveryStatus != xinbox.DeliveryStatusActive || got.LastError != "" {
		t.Fatalf("state = %+v, want comments-only active state", got)
	}
	if want := []string{"workspace-bearer"}; !reflect.DeepEqual(api.ruleTokens, want) {
		t.Fatalf("comment rule tokens = %v, want %v", api.ruleTokens, want)
	}
	if len(api.subscriptionTokens) != 0 {
		t.Fatalf(
			"subscription creation calls = %v, want none",
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

func TestXInboxDeliveryManagedMissingConsumerSecretDisablesOnlyDM(t *testing.T) {
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{activeManagedXInboxAccount()}}
	api := &fakeXInboxDeliveryAPI{ruleID: "comments-rule", subscriptionID: "must-not-create"}
	config := enabledXInboxDeliveryConfig(store, api)
	config.ManagedConsumerSecretConfigured = false
	worker := NewXInboxDeliveryWorker(config)

	if err := worker.ReconcileOnce(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "TWITTER_CONSUMER_SECRET") {
		t.Fatalf("reconcile error = %v, want missing managed consumer secret", err)
	}
	got := store.states[len(store.states)-1]
	if got.FilteredStreamRuleID != "comments-rule" || got.ActivityDMSubscriptionID != "" ||
		got.DeliveryStatus != xinbox.DeliveryStatusError ||
		!strings.Contains(got.LastError, "TWITTER_CONSUMER_SECRET") {
		t.Fatalf("state = %+v, want comments retained with DM credential error", got)
	}
	if want := []string{"managed-app-token"}; !reflect.DeepEqual(api.ruleTokens, want) {
		t.Fatalf("comment rule tokens = %v, want %v", api.ruleTokens, want)
	}
	if len(api.subscriptionTokens) != 0 {
		t.Fatalf(
			"subscription creation calls = %v, want none",
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
		Usage: &fakeXInboxUsageReader{},
	})

	if err := worker.ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.states) == 0 || store.states[len(store.states)-1].FilteredStreamRuleID != "new-rule" {
		t.Fatalf("states = %+v, want a new resource persisted", store.states)
	}
	if want := []string{
		"ensure-rule:" + account.SocialAccountID,
		"list-subscriptions:x-app:test",
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
	api := &fakeXInboxDeliveryAPI{activitySubscriptions: []xinbox.ActivitySubscription{
		fakeDMActivitySubscription(account.ActivityDMSubscriptionID, account, "existing-webhook"),
	}}
	worker := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store:                           store,
		API:                             api,
		Cipher:                          fakeXInboxCipher{values: map[string]string{"encrypted-user-token": "user-oauth-token"}},
		Usage:                           &fakeXInboxUsageReader{snapshot: xcredits.Snapshot{PausePaidSources: true, InboundPauseReason: xcredits.PauseReasonMonthlyAllowance}},
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
		Usage: &fakeXInboxUsageReader{},
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

func TestXInboxDeliverySharedStreamServesTwoAccountsUntilLastCommentsSourceStops(t *testing.T) {
	leader := &sharedTestLeader{}
	runner := &managedStreamRunner{
		starts: make(chan XInboxAppStream, 4),
		stops:  make(chan string, 4),
	}
	first := activeManagedXInboxAccount()
	first.FilteredStreamRuleID = "rule-first"
	second := first
	second.SocialAccountID = "account-2"
	second.ExternalAccountID = "provider-user-2"
	second.FilteredStreamRuleID = "rule-second"
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{first, second}}
	api := &fakeXInboxDeliveryAPI{}
	worker := NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store:                           store,
		API:                             api,
		Cipher:                          fakeXInboxCipher{},
		Usage:                           &fakeXInboxUsageReader{},
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
	select {
	case duplicate := <-runner.starts:
		t.Fatalf("two accounts started duplicate app stream: %+v", duplicate)
	case <-time.After(20 * time.Millisecond):
	}

	store.mu.Lock()
	store.accounts[0].Scopes = []string{"dm.read"}
	store.mu.Unlock()
	worker.reconcileAndStartStreams(ctx)
	select {
	case stopped := <-runner.stops:
		t.Fatalf("disabling one account stopped shared stream %q", stopped)
	case <-time.After(20 * time.Millisecond):
	}

	store.mu.Lock()
	store.accounts[1].ActivityDMSubscriptionID = "dm-only-change"
	store.accounts[1].DMSubscriptionForbiddenFingerprint = "dm-latch-only-change"
	store.mu.Unlock()
	worker.reconcileAndStartStreams(ctx)
	select {
	case stopped := <-runner.stops:
		t.Fatalf("DM-only state change churned shared stream %q", stopped)
	case started := <-runner.starts:
		t.Fatalf("DM-only state change restarted shared stream %+v", started)
	case <-time.After(20 * time.Millisecond):
	}

	store.mu.Lock()
	store.accounts[1].Scopes = []string{"dm.read"}
	store.mu.Unlock()
	worker.reconcileAndStartStreams(ctx)
	if stopped := <-runner.stops; stopped != safeAppIdentity("managed-route-key") {
		t.Fatalf("last comments source stopped = %q", stopped)
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

func TestXInboxDeliveryLeadershipLossStopsLocalStreamAndReleasesLease(t *testing.T) {
	leader := &sharedTestLeader{}
	account := activeManagedXInboxAccount()
	account.FilteredStreamRuleID = "rule-existing"
	account.Scopes = []string{"tweet.read", "tweet.write", "users.read"}
	store := &fakeXInboxDeliveryStore{accounts: []XInboxDeliveryAccount{account}}
	firstRunner := &managedStreamRunner{
		starts: make(chan XInboxAppStream, 1),
		stops:  make(chan string, 1),
	}
	secondRunner := &managedStreamRunner{
		starts: make(chan XInboxAppStream, 1),
		stops:  make(chan string, 1),
	}
	newWorker := func(owner string, runner XInboxStreamRunner) *XInboxDeliveryWorker {
		return NewXInboxDeliveryWorker(XInboxDeliveryConfig{
			Store:                           store,
			API:                             &fakeXInboxDeliveryAPI{},
			Cipher:                          fakeXInboxCipher{},
			Usage:                           &fakeXInboxUsageReader{},
			Leader:                          leader,
			Stream:                          runner,
			ManagedAppBearer:                "managed-token",
			ManagedConsumerSecretConfigured: true,
			EventHandler:                    func(context.Context, string, xinbox.StreamEvent) error { return nil },
			CleanupOwner:                    owner,
		})
	}
	first := newWorker("worker-one", firstRunner)
	second := newWorker("worker-two", secondRunner)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		first.stopAllStreams()
		second.stopAllStreams()
	})

	first.reconcileAndStartStreams(ctx)
	select {
	case <-firstRunner.starts:
	case <-time.After(time.Second):
		t.Fatal("first worker did not start app stream")
	}

	store.mu.Lock()
	store.listStarted = make(chan struct{}, 1)
	store.listRelease = make(chan struct{})
	listStarted := store.listStarted
	listRelease := store.listRelease
	store.mu.Unlock()
	secondDone := make(chan struct{})
	go func() {
		second.reconcileAndStartStreams(ctx)
		close(secondDone)
	}()
	select {
	case <-listStarted:
	case <-time.After(time.Second):
		t.Fatal("second worker did not acquire reconciliation leadership")
	}

	first.reconcileAndStartStreams(ctx)
	select {
	case stopped := <-firstRunner.stops:
		if stopped != safeAppIdentity(account.WebhookRouteKey) {
			t.Fatalf("first worker stopped stream = %q", stopped)
		}
	case <-time.After(time.Second):
		t.Fatal("first worker retained local stream after losing reconciliation leadership")
	}

	close(listRelease)
	select {
	case started := <-secondRunner.starts:
		if started.Identity != safeAppIdentity(account.WebhookRouteKey) {
			t.Fatalf("second worker started stream = %+v", started)
		}
	case <-time.After(time.Second):
		t.Fatal("second worker could not acquire released app stream lease")
	}
	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("second reconciliation did not finish")
	}
	cancel()
	select {
	case <-secondRunner.stops:
	case <-time.After(time.Second):
		t.Fatal("second worker stream did not stop during test cleanup")
	}
}

func TestXInboxDeliveryAuthoritativeLeadershipIncompleteAccountListRetainsPreviousDesiredStreams(t *testing.T) {
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
		Usage:                           &fakeXInboxUsageReader{},
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
		Usage:        &fakeXInboxUsageReader{},
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
	deliveryAST, err := parser.ParseFile(token.NewFileSet(), "x_inbox_delivery.go", deliverySource, 0)
	if err != nil {
		t.Fatalf("parse x_inbox_delivery.go: %v", err)
	}
	var constructor *ast.FuncDecl
	for _, declaration := range deliveryAST.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Name.Name == "NewPostgresXInboxDeliveryWorker" {
			constructor = function
			break
		}
	}
	if constructor == nil {
		t.Fatal("Postgres delivery worker constructor is missing")
	}
	lockFromDatabaseURL := false
	configMapped := false
	ast.Inspect(constructor.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		function, ok := call.Fun.(*ast.Ident)
		if !ok {
			return true
		}
		switch function.Name {
		case "NewPostgresStreamLockManager":
			if len(call.Args) == 1 {
				selector, selectorOK := call.Args[0].(*ast.SelectorExpr)
				config, configOK := (*ast.Ident)(nil), false
				if selectorOK {
					config, configOK = selector.X.(*ast.Ident)
				}
				lockFromDatabaseURL = configOK && config.Name == "config" && selector.Sel.Name == "DatabaseURL"
			}
		case "xInboxDeliveryConfigFromPostgres":
			if len(call.Args) > 0 {
				config, ok := call.Args[0].(*ast.Ident)
				configMapped = ok && config.Name == "config"
			}
		}
		return true
	})
	if !lockFromDatabaseURL {
		t.Fatal("Postgres worker must construct the isolated lock manager from DATABASE_URL")
	}
	if !configMapped {
		t.Fatal("Postgres worker must map the named config into the delivery worker config")
	}
	if !strings.Contains(
		string(deliverySource),
		"COALESCE(NULLIF(pc.consumer_secret, ''), '') <> ''",
	) {
		t.Fatal("Postgres delivery account query must carry consumer-secret availability")
	}
}
