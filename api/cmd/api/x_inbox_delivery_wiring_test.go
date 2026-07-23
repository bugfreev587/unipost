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

	"github.com/xiaoboyu/unipost-api/internal/featureflags"
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
