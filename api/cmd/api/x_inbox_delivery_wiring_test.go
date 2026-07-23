package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/featureflags"
	"github.com/xiaoboyu/unipost-api/internal/worker"
	"github.com/xiaoboyu/unipost-api/internal/xcredits"
	"github.com/xiaoboyu/unipost-api/internal/xinbox"
)

func TestXInboxDeliveryWorkerWiringUsesDevSafeEnvironmentContracts(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		"worker.NewPostgresXInboxDeliveryWorker(",
		"databaseURL,",
		`os.Getenv("TWITTER_BEARER_TOKEN")`,
		`strings.TrimSpace(os.Getenv("TWITTER_CONSUMER_SECRET")) != ""`,
		`os.Getenv("X_INBOX_WEBHOOK_ROUTE_SECRET")`,
		`managedXWebhookRouteKey`,
		`os.Getenv("X_INBOX_WEBHOOK_URL")`,
		`.SetEventHandler(xIngestionService.IngestStreamEvent)`,
		`r.Get("/v1/webhooks/twitter/{webhook_route_key}", xWebhookHandler.CRC)`,
		`r.Post("/v1/webhooks/twitter/{webhook_route_key}", xWebhookHandler.Handle)`,
		"go xInboxDeliveryWorker.Start(workerCtx)",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("main.go missing %q", required)
		}
	}
	if strings.Contains(text, `xinbox.WebhookRouteKey(
		os.Getenv("TWITTER_CONSUMER_SECRET")`) {
		t.Fatal("managed webhook route key must not derive from rotatable TWITTER_CONSUMER_SECRET")
	}
}

func TestXInboxDeliveryWiringUsesStrictDMCanaryAndWorkspaceFeatureEvaluator(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)

	if got := strings.Count(text, `os.Getenv("X_INBOX_DM_CANARY_SOCIAL_ACCOUNT_IDS")`); got != 1 {
		t.Fatalf("X_INBOX_DM_CANARY_SOCIAL_ACCOUNT_IDS reads = %d, want 1", got)
	}
	if got := strings.Count(text, "worker.ParseXInboxDMCanary("); got != 1 {
		t.Fatalf("ParseXInboxDMCanary calls = %d, want 1", got)
	}
	if got := strings.Count(text, "xDMsAvailable := xDMAvailability(featureFlagEvaluator)"); got != 1 {
		t.Fatalf("shared X DM availability callback declarations = %d, want 1", got)
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), "main.go", source, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}
	sharedConsumers := 0
	postgresConfigLiterals := 0
	postgresCanaryBindings := 0
	ast.Inspect(parsed, func(node ast.Node) bool {
		literal, ok := node.(*ast.CompositeLit)
		if ok {
			selector, selectorOK := literal.Type.(*ast.SelectorExpr)
			packageName, packageOK := (*ast.Ident)(nil), false
			if selectorOK {
				packageName, packageOK = selector.X.(*ast.Ident)
			}
			if packageOK && packageName.Name == "worker" && selector.Sel.Name == "PostgresXInboxDeliveryConfig" {
				postgresConfigLiterals++
				for _, element := range literal.Elts {
					field, fieldOK := element.(*ast.KeyValueExpr)
					if !fieldOK {
						continue
					}
					key, keyOK := field.Key.(*ast.Ident)
					value, valueOK := field.Value.(*ast.Ident)
					if keyOK && valueOK && key.Name == "DMCanaryAccountIDs" &&
						value.Name == "xInboxDMCanaryAccountIDs" {
						postgresCanaryBindings++
					}
				}
			}
		}

		field, ok := node.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		key, keyOK := field.Key.(*ast.Ident)
		value, valueOK := field.Value.(*ast.Ident)
		if keyOK && valueOK && key.Name == "DMsAvailable" && value.Name == "xDMsAvailable" {
			sharedConsumers++
		}
		return true
	})
	if got := sharedConsumers; got != 2 {
		t.Fatalf("shared X DM availability callback consumers = %d, want 2 (ingestion and delivery)", got)
	}
	if postgresConfigLiterals != 1 {
		t.Fatalf("PostgresXInboxDeliveryConfig literals = %d, want 1", postgresConfigLiterals)
	}
	if postgresCanaryBindings != 1 {
		t.Fatalf("Postgres DM canary bindings = %d, want 1", postgresCanaryBindings)
	}
	if !strings.Contains(text, "xInboxDMCanaryAccountIDs := parseXInboxDMCanary(") {
		t.Fatal("main.go does not parse the X DM canary once at startup")
	}
}

func TestXDMAvailabilityUsesExactEvaluatorContract(t *testing.T) {
	sentinel := errors.New("feature evaluator unavailable")
	for _, test := range []struct {
		name  string
		value bool
		err   error
	}{
		{name: "enabled", value: true},
		{name: "disabled", value: false},
		{name: "error", err: sentinel},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), xDMContextKey{}, &struct{}{})
			evaluator := &fakeXDMFeatureEvaluator{value: test.value, err: test.err}

			got, err := xDMAvailability(evaluator)(ctx, "workspace-1")

			if got != test.value {
				t.Fatalf("xDMAvailability() = %v, want %v", got, test.value)
			}
			if err != test.err {
				t.Fatalf("xDMAvailability() error = %v, want exact %v", err, test.err)
			}
			if evaluator.ctx != ctx {
				t.Fatal("xDMAvailability did not preserve context identity")
			}
			if evaluator.workspaceID != "workspace-1" {
				t.Fatalf("workspaceID = %q, want workspace-1", evaluator.workspaceID)
			}
			if evaluator.key != featureflags.XDMSV1 {
				t.Fatalf("feature key = %q, want %q", evaluator.key, featureflags.XDMSV1)
			}
		})
	}
}

type xDMContextKey struct{}

type fakeXDMFeatureEvaluator struct {
	ctx         context.Context
	workspaceID string
	key         string
	value       bool
	err         error
}

type integrationXDMFeatureStore struct {
	globalEnabled bool
	globalErr     error
	owner         string
	ownerErr      error
	globalKeys    []string
	ownerSpaces   []string
}

func (s *integrationXDMFeatureStore) List(context.Context) ([]featureflags.Flag, error) {
	return nil, nil
}

func (s *integrationXDMFeatureStore) Set(context.Context, string, bool, string) (featureflags.Flag, error) {
	return featureflags.Flag{}, nil
}

func (s *integrationXDMFeatureStore) GlobalEnabled(_ context.Context, key string) (bool, error) {
	s.globalKeys = append(s.globalKeys, key)
	return s.globalEnabled, s.globalErr
}

func (s *integrationXDMFeatureStore) WorkspaceOwner(_ context.Context, workspaceID string) (string, error) {
	s.ownerSpaces = append(s.ownerSpaces, workspaceID)
	return s.owner, s.ownerErr
}

type integrationXDMSuperAdmins map[string]bool

func (s integrationXDMSuperAdmins) IsSuperAdmin(_ context.Context, userID string) bool {
	return s[userID]
}

type integrationXDMIngestStore struct {
	account xinbox.InboxAccount
	inserts int
}

func (s *integrationXDMIngestStore) AccountForApp(context.Context, string, string) (xinbox.InboxAccount, error) {
	return s.account, nil
}

func (s *integrationXDMIngestStore) InsertInboxItem(_ context.Context, item xinbox.InboxItem) (xinbox.InboxItem, bool, error) {
	s.inserts++
	return item, true, nil
}

type integrationXDMDeliveryStore struct {
	account worker.XInboxDeliveryAccount
	states  []worker.XInboxDeliveryState
}

func (s *integrationXDMDeliveryStore) ListAccounts(context.Context) ([]worker.XInboxDeliveryAccount, error) {
	return []worker.XInboxDeliveryAccount{s.account}, nil
}

func (s *integrationXDMDeliveryStore) SaveState(_ context.Context, state worker.XInboxDeliveryState) error {
	s.states = append(s.states, state)
	s.account.FilteredStreamRuleID = state.FilteredStreamRuleID
	s.account.ActivityDMSubscriptionID = state.ActivityDMSubscriptionID
	s.account.ActivityWebhookRouteKey = state.ActivityWebhookRouteKey
	return nil
}

func (*integrationXDMDeliveryStore) ClaimCleanupIntents(context.Context, string, time.Time, time.Time, int) ([]worker.XInboxCleanupIntent, error) {
	return nil, nil
}

func (*integrationXDMDeliveryStore) ReleaseCleanupIntent(context.Context, worker.XInboxCleanupIntent, time.Time) error {
	return nil
}

func (*integrationXDMDeliveryStore) CompleteCleanupIntent(context.Context, string, string) error {
	return nil
}

type integrationXDMDeliveryAPI struct {
	ensureRules         int
	ensureWebhooks      int
	listSubscriptions   int
	createSubscriptions int
	deleteRules         int
	deleteSubscriptions int
}

func (a *integrationXDMDeliveryAPI) EnsureFilteredStreamRule(context.Context, string, string, string) (xinbox.StreamRule, error) {
	a.ensureRules++
	return xinbox.StreamRule{ID: "101"}, nil
}

func (a *integrationXDMDeliveryAPI) DeleteFilteredStreamRule(context.Context, string, string) error {
	a.deleteRules++
	return nil
}

func (a *integrationXDMDeliveryAPI) EnsureWebhook(_ context.Context, _ string, configuredURL string) (xinbox.Webhook, error) {
	a.ensureWebhooks++
	return xinbox.Webhook{ID: "201", URL: configuredURL, Valid: true}, nil
}

func (a *integrationXDMDeliveryAPI) ListActivitySubscriptions(context.Context, string) ([]xinbox.ActivitySubscription, error) {
	a.listSubscriptions++
	return nil, nil
}

func (a *integrationXDMDeliveryAPI) CreateDMSubscription(_ context.Context, _ string, accountID, userID, webhookID string) (xinbox.ActivitySubscription, error) {
	a.createSubscriptions++
	return xinbox.ActivitySubscription{
		ID: "301", EventType: "dm.received", Tag: xinbox.DMSubscriptionTag(accountID),
		Filter: xinbox.ActivityFilter{UserID: userID}, WebhookID: webhookID,
	}, nil
}

func (a *integrationXDMDeliveryAPI) DeleteActivitySubscription(context.Context, string, string) error {
	a.deleteSubscriptions++
	return nil
}

type integrationXDMCipher struct{}

func (integrationXDMCipher) Decrypt(string) (string, error) { return "", nil }

type integrationXDMUsage struct{}

func (integrationXDMUsage) Snapshot(context.Context, string, time.Time) (xcredits.Snapshot, error) {
	return xcredits.Snapshot{}, nil
}

func TestXDMRealEvaluatorWiringKeepsIngestionAndDeliveryInLockstep(t *testing.T) {
	globalFailure := errors.New("global feature store unavailable")
	ownerFailure := errors.New("workspace owner unavailable")
	const accountID = "00000000-0000-4000-8000-000000000001"
	for _, test := range []struct {
		name                  string
		globalEnabled         bool
		globalErr             error
		ownerErr              error
		superAdmin            bool
		canaryRaw             string
		wantInserts           int
		wantProvisioning      int
		wantEvaluatorCalls    int
		wantOwnerCalls        int
		wantIngestionErr      error
		wantReconciliationErr error
		runIngestion          bool
	}{
		{name: "global off super admin owner", superAdmin: true, canaryRaw: accountID, wantInserts: 1, wantProvisioning: 1, wantEvaluatorCalls: 2, wantOwnerCalls: 2, runIngestion: true},
		{name: "global off regular owner", canaryRaw: accountID, wantEvaluatorCalls: 2, wantOwnerCalls: 2, runIngestion: true},
		{name: "global lookup error", globalErr: globalFailure, canaryRaw: accountID, wantEvaluatorCalls: 2, wantIngestionErr: globalFailure, wantReconciliationErr: globalFailure, runIngestion: true},
		{name: "workspace owner lookup error", ownerErr: ownerFailure, canaryRaw: accountID, wantEvaluatorCalls: 2, wantOwnerCalls: 2, wantIngestionErr: ownerFailure, wantReconciliationErr: ownerFailure, runIngestion: true},
		{name: "invalid canary", globalEnabled: true, canaryRaw: "invalid", wantEvaluatorCalls: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			flagStore := &integrationXDMFeatureStore{
				globalEnabled: test.globalEnabled, globalErr: test.globalErr,
				owner: "owner-1", ownerErr: test.ownerErr,
			}
			evaluator := featureflags.NewEvaluator(flagStore, integrationXDMSuperAdmins{"owner-1": test.superAdmin})
			availability := xDMAvailability(evaluator)
			ingestStore := &integrationXDMIngestStore{account: xinbox.InboxAccount{
				ID: accountID, WorkspaceID: "workspace-1", ExternalUserID: "managed-user-1",
				ExternalAccountID: "provider-user-1", AppMode: xinbox.AppModeUniPostManaged,
				Scopes: []string{"dm.read", "users.read"}, PlanAllowsInbox: true,
			}}
			ingestion := xinbox.NewIngestionService(xinbox.IngestionConfig{
				Store: ingestStore, DMsAvailable: availability,
			})

			if test.runIngestion {
				err := ingestion.IngestActivityEvent(context.Background(), "route-1", xinbox.ActivityEvent{
					AccountID: accountID, ExternalUserID: "provider-user-1", ExternalID: "dm-1",
					ConversationID: "thread-1", SenderID: "sender-1", RecipientID: "provider-user-1",
				})
				if !errors.Is(err, test.wantIngestionErr) {
					t.Fatalf("ingestion error = %v, want errors.Is(%v)", err, test.wantIngestionErr)
				}
			}

			canary := parseXInboxDMCanary(test.canaryRaw)
			if test.name == "invalid canary" && len(canary) != 0 {
				t.Fatalf("invalid canary parsed as %#v, want empty", canary)
			}
			deliveryStore := &integrationXDMDeliveryStore{account: worker.XInboxDeliveryAccount{
				SocialAccountID: accountID, WorkspaceID: "workspace-1", WebhookRouteKey: "route-key",
				Handle: "unipostdev", ExternalAccountID: "provider-user-1", AppMode: xinbox.AppModeUniPostManaged,
				ConsumerSecretConfigured: true,
				Scopes:                   []string{"tweet.read", "tweet.write", "users.read", "dm.read", "dm.write"},
				AccountActive:            true, PlanAllowsInbox: true,
			}}
			deliveryAPI := &integrationXDMDeliveryAPI{}
			delivery := worker.NewXInboxDeliveryWorker(worker.XInboxDeliveryConfig{
				Store: deliveryStore, API: deliveryAPI, Cipher: integrationXDMCipher{}, Usage: integrationXDMUsage{},
				ManagedAppBearer: "not-recorded", ManagedConsumerSecretConfigured: true,
				WebhookURL: "https://api.example.test/v1/webhooks/twitter", DMsAvailable: availability,
				DMCanaryAccountIDs: canary,
			})
			err := delivery.ReconcileOnce(context.Background())
			if !errors.Is(err, test.wantReconciliationErr) {
				t.Fatalf("reconciliation error = %v, want errors.Is(%v)", err, test.wantReconciliationErr)
			}

			if ingestStore.inserts != test.wantInserts {
				t.Fatalf("ingestion inserts = %d, want %d", ingestStore.inserts, test.wantInserts)
			}
			if deliveryAPI.ensureRules != 1 {
				t.Fatalf("comment rule mutations = %d, want 1", deliveryAPI.ensureRules)
			}
			if deliveryAPI.ensureWebhooks != test.wantProvisioning || deliveryAPI.createSubscriptions != test.wantProvisioning {
				t.Fatalf("DM provisioning mutations: webhooks=%d subscriptions=%d, want %d each",
					deliveryAPI.ensureWebhooks, deliveryAPI.createSubscriptions, test.wantProvisioning)
			}
			if deliveryAPI.deleteRules != 0 || deliveryAPI.deleteSubscriptions != 0 {
				t.Fatalf("unexpected destructive mutations: rules=%d subscriptions=%d", deliveryAPI.deleteRules, deliveryAPI.deleteSubscriptions)
			}
			if len(flagStore.globalKeys) != test.wantEvaluatorCalls || len(flagStore.ownerSpaces) != test.wantOwnerCalls {
				t.Fatalf("evaluator calls: global=%v owner=%v", flagStore.globalKeys, flagStore.ownerSpaces)
			}
			for _, key := range flagStore.globalKeys {
				if key != featureflags.XDMSV1 {
					t.Fatalf("feature key = %q, want %q", key, featureflags.XDMSV1)
				}
			}
			for _, workspaceID := range flagStore.ownerSpaces {
				if workspaceID != "workspace-1" {
					t.Fatalf("workspaceID = %q, want workspace-1", workspaceID)
				}
			}
			if test.name == "invalid canary" && deliveryAPI.listSubscriptions != 1 {
				t.Fatalf("invalid canary orphan discovery calls = %d, want one safe read-only cleanup list", deliveryAPI.listSubscriptions)
			}
		})
	}
}

func (f *fakeXDMFeatureEvaluator) ForWorkspace(
	ctx context.Context,
	workspaceID string,
	key string,
) (bool, error) {
	f.ctx = ctx
	f.workspaceID = workspaceID
	f.key = key
	return f.value, f.err
}

func TestXInboxDeliveryWiringInvalidCanaryConfigLogsOnlySanitizedClass(t *testing.T) {
	var output bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(originalLogger) })

	const raw = "00000000-0000-4000-8000-000000000001,not-a-uuid"
	got := parseXInboxDMCanary(raw)
	if len(got) != 0 {
		t.Fatalf("parseXInboxDMCanary(invalid) = %v, want empty set", got)
	}

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("decode warning log: %v; output = %q", err, output.String())
	}
	if got := entry["error_class"]; got != "x_dm_canary_config_invalid" {
		t.Fatalf("error_class = %v, want x_dm_canary_config_invalid", got)
	}
	for _, forbidden := range []string{raw, "00000000-0000-4000-8000-000000000001", "not-a-uuid", "parse X DM canary"} {
		if strings.Contains(output.String(), forbidden) {
			t.Fatalf("warning log leaks forbidden value %q: %s", forbidden, output.String())
		}
	}
}

func TestXInboxRunbookSeparatesStableRouteSecretFromConsumerSecret(t *testing.T) {
	source, err := os.ReadFile("../../../docs/superpowers/plans/2026-07-16-x-inbox-comments-dms.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		"X_INBOX_WEBHOOK_ROUTE_SECRET",
		"Do not reuse",
		"rotating X's consumer secret",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("X Inbox runbook missing %q", required)
		}
	}
}
