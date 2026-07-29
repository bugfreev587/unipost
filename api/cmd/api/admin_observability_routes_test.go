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
		"go postFailureDebugWriter.Start(workerCtx)",
		"SetPostFailureDebugSink(postFailureDebugWriter)",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("main.go missing background diagnostic wiring %q", required)
		}
	}
}
