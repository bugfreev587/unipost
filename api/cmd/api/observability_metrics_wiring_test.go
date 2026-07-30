package main

import (
	"os"
	"strings"
	"testing"
)

func TestMainWiresOneGlobalObservabilityMetricsReaderIntoWorkspaceAndAdminHandlers(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"observabilityMetricsStore := observabilityreads.NewPostgresMetricsStore(pool)",
		"handler.NewAPIMetricsHandler(queries).WithObservabilityReads(observabilityReadSelector, observabilityMetricsStore)",
		"handler.NewAdminAPIMetricsHandler(queries).WithObservabilityReads(observabilityReadSelector, observabilityMetricsStore)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("main.go missing %q", want)
		}
	}
}
