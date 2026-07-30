package main

import (
	"os"
	"strings"
	"testing"
)

func TestMainWiresLiveLogsThroughTheGlobalObservabilityReadSelector(t *testing.T) {
	body, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	for _, want := range []string{
		"ws.NotifyLog(ctx, pool, ws.LogEnvelope(row))",
		"NewCachedReadSelector(observabilityReadSelector",
		"RefreshInterval:   time.Second",
		"EvaluationTimeout: 500 * time.Millisecond",
		"observabilityLiveLogMode.Start(workerCtx)",
		"ws.NewHandler(logsHub, queries).WithLogReadSelector(observabilityLiveLogMode)",
		"observabilityLiveLogMode.Stop()",
		"observabilityLiveLogMode.Wait(ctx)",
		"requestevents.Config{}",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("main.go missing live logs read-path wiring %q", want)
		}
	}
	for _, forbidden := range []string{"IntegrationLogEnvelope(row, observabilityReadSelector.UseV2(ctx))", "OnPersisted:"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("main.go keeps hot-path observability flag wiring %q", forbidden)
		}
	}
}
