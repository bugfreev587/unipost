package main

import (
	"os"
	"strings"
	"testing"
)

func TestXInboxOutboundRecoveryWorkerIsWired(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"handler.NewXInboxOutboundRecoveryService(inboxHandler)",
		"worker.NewXInboxOutboundRecoveryWorker(",
		"go xOutboundRecoveryWorker.Start(workerCtx)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("main.go missing X Inbox outbound recovery wiring %q", want)
		}
	}
}
