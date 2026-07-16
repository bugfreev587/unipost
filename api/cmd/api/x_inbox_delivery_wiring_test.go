package main

import (
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
