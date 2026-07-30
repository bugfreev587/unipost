package main

import (
	"os"
	"strings"
	"testing"
)

func TestRequestEventPartitionWiring(t *testing.T) {
	body, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	for _, required := range []string{
		`requesteventpartitions.NewPostgresStore(pool)`,
		`requesteventpartitions.NewWorker(partitionStore)`,
		`go requestEventPartitionWorker.Start(workerCtx)`,
		`Get("/v1/admin/observability/request-event-partitions"`,
		`auth.RequireSuperAdmin`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("request-event partition wiring missing %q", required)
		}
	}
}
