package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestObservabilityRollupWorkerIsAPIOnlyIndependentBackgroundWiring(t *testing.T) {
	body, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	pattern := regexp.MustCompile(`(?s)if processMode == processModeAPI \{\s*observabilityRollupStore := observabilityreads\.NewPostgresRollupStore\(pool\)\s*observabilityRollupWorker := observabilityreads\.NewRollupWorker\(observabilityRollupStore\)\s*go observabilityRollupWorker\.Start\(workerCtx\)\s*\}`)
	match := pattern.FindString(source)
	if match == "" {
		t.Fatal("main.go must construct and asynchronously start exactly one rollup store/worker inside API-only process wiring")
	}
	if strings.Count(source, "observabilityreads.NewPostgresRollupStore(pool)") != 1 || strings.Count(source, "observabilityreads.NewRollupWorker(observabilityRollupStore)") != 1 || strings.Count(source, "go observabilityRollupWorker.Start(workerCtx)") != 1 {
		t.Fatal("main.go must wire exactly one observability rollup worker")
	}
	for _, forbidden := range []string{
		"featureFlagEvaluator",
		"observabilityReadSelector",
		"publishing",
		"account",
		"quota",
		"health",
	} {
		if strings.Contains(match, forbidden) {
			t.Fatalf("rollup worker wiring is coupled to protected/readiness flow %q: %s", forbidden, match)
		}
	}
}
