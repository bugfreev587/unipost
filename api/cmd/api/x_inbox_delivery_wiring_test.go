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
}
