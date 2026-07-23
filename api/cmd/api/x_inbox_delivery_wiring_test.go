package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"
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
	if got := strings.Count(text, "featureFlagEvaluator.ForWorkspace(ctx, workspaceID, featureflags.XDMSV1)"); got != 2 {
		t.Fatalf("XDMSV1 workspace evaluator calls = %d, want 2 (ingestion and delivery)", got)
	}
	for _, required := range []string{
		"xInboxDMCanaryAccountIDs := parseXInboxDMCanary(",
		"func(ctx context.Context, workspaceID string) (bool, error) {",
		"xInboxDMCanaryAccountIDs,",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("main.go missing DM delivery wiring %q", required)
		}
	}
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
