package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestAdminPostFailureRoutesRequireSuperAdmin(t *testing.T) {
	body, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	for _, route := range []string{
		`/v1/admin/post-failures`,
		`/v1/admin/post-failures/\{id\}/debug`,
	} {
		pattern := regexp.MustCompile(`(?s)r\.With\(auth\.RequireSuperAdmin\([^\n]+Admin errors are restricted to super admins[^\n]+\)\)\.\s*Get\("` + route + `"`)
		if !pattern.MatchString(source) {
			t.Fatalf("route %s is not protected by RequireSuperAdmin", route)
		}
	}
}

func TestPublishingFailureDebugWriterIsWiredAsBackgroundObservability(t *testing.T) {
	body, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	for _, required := range []string{
		"postfailuredebug.NewPostgresStore(pool)",
		"postfailuredebug.NewWriter(",
		"go postFailureDebugWriter.Start(ctx)",
		"SetPostFailureDebugSink(postFailureDebugWriter)",
		"waitForPostFailureDebugShutdown(",
		"writer.Stop()",
		"writer.Wait(",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("main.go missing background diagnostic wiring %q", required)
		}
	}
}

func TestCanonicalRequestEventRecorderIsDarkDualWriteMiddleware(t *testing.T) {
	body, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	for _, required := range []string{
		"requestevents.NewPostgresStore(pool)",
		"requestevents.NewRecorder(",
		"go requestEventRecorder.Start(ctx)",
		"r.Use(integrationlogs.Middleware(integrationLogger))",
		"r.Use(apiMetricsRecorder.Middleware)",
		"r.Use(requestevents.Middleware(requestEventRecorder))",
		"waitForRequestEventRecorderShutdown(",
		"recorder.Stop()",
		"recorder.Wait(",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("main.go missing dark request-event wiring %q", required)
		}
	}
	if strings.Contains(source, "requestEventParityMonitor.Start") {
		t.Fatal("async insert timestamps must not drive automatic exact-parity alerts")
	}
	statsPattern := regexp.MustCompile(`(?s)r\.With\(auth\.RequireSuperAdmin\([^\n]+Request-event telemetry is restricted to super admins[^\n]+\)\)\.\s*Get\("/v1/admin/observability/request-events", requestevents\.StatsHandler\(requestEventRecorder\)\)`)
	if !statsPattern.MatchString(source) {
		t.Fatal("request-event recorder stats route is missing or not protected by RequireSuperAdmin")
	}
}

func TestObservabilityReadsV2IsWiredOnlyAsOptionalReadDependencies(t *testing.T) {
	body, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	for _, required := range []string{
		"observabilityreads.NewPostgresLogStore(pool)",
		"observabilityreads.NewReadSelector(featureFlagEvaluator, slog.Default())",
		"NewLogsHandler(queries).SetObservabilityReads(observabilityReadSelector, observabilityLogStore)",
		"SetObservabilityReads(observabilityReadSelector, observabilityLogStore)",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("main.go missing observability read wiring %q", required)
		}
	}
	for _, forbidden := range []string{
		"observabilityLogStore.Start",
		"observabilityReadSelector.ForWorkspace",
		"requestEventRecorder.SetObservabilityReads",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("read flag leaked into writer/background/business wiring %q", forbidden)
		}
	}
}
