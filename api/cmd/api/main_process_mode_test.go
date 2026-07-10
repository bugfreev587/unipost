package main

import (
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/worker"
)

func TestProcessModeDefaultsToAPI(t *testing.T) {
	mode, err := normalizeProcessMode("")
	if err != nil {
		t.Fatalf("normalizeProcessMode returned error: %v", err)
	}
	if mode != processModeAPI {
		t.Fatalf("mode = %q, want %q", mode, processModeAPI)
	}
}

func TestProcessModeAcceptsPostDeliveryWorker(t *testing.T) {
	mode, err := normalizeProcessMode(" post-delivery-worker ")
	if err != nil {
		t.Fatalf("normalizeProcessMode returned error: %v", err)
	}
	if mode != processModePostDeliveryWorker {
		t.Fatalf("mode = %q, want %q", mode, processModePostDeliveryWorker)
	}
}

func TestProcessModeRejectsUnknownMode(t *testing.T) {
	if _, err := normalizeProcessMode("scheduler"); err == nil {
		t.Fatal("expected unknown process mode to fail")
	}
}

func TestDBPoolMaxConnsUsesWorkerDefaultInWorkerMode(t *testing.T) {
	t.Setenv("DATABASE_MAX_CONNS", "")
	t.Setenv("API_DATABASE_MAX_CONNS", "")
	t.Setenv("POST_DELIVERY_WORKER_DATABASE_MAX_CONNS", "")
	config := worker.PostDeliveryWorkerConfig{GlobalConcurrency: 17}

	got := dbPoolMaxConnsForMode(processModePostDeliveryWorker, config)
	want := int32(22)
	if got != want {
		t.Fatalf("worker db max conns = %d, want %d", got, want)
	}
}

func TestDBPoolMaxConnsPrefersSpecificEnvOverGeneric(t *testing.T) {
	t.Setenv("DATABASE_MAX_CONNS", "31")
	t.Setenv("POST_DELIVERY_WORKER_DATABASE_MAX_CONNS", "43")

	got := dbPoolMaxConnsForMode(processModePostDeliveryWorker, worker.PostDeliveryWorkerConfig{GlobalConcurrency: 10})
	if got != 43 {
		t.Fatalf("worker db max conns = %d, want specific override 43", got)
	}
}

func TestProcessModeWorkerStartupRules(t *testing.T) {
	t.Setenv("POST_DELIVERY_WORKER_DISABLE_API_DELIVERY", "")
	if !shouldStartHTTPServer(processModeAPI) {
		t.Fatal("api mode should start the HTTP server")
	}
	if !shouldStartPostDeliveryWorkers(processModeAPI) {
		t.Fatal("api mode should keep post delivery workers as a rollout fallback by default")
	}
	if shouldStartHTTPServer(processModePostDeliveryWorker) {
		t.Fatal("post delivery worker mode must not start the HTTP server")
	}
	if !shouldStartPostDeliveryWorkers(processModePostDeliveryWorker) {
		t.Fatal("post delivery worker mode should start post delivery workers")
	}
}

func TestProcessModeCanDisableAPIDeliveryWorkersAfterDedicatedWorkerIsEnabled(t *testing.T) {
	t.Setenv("POST_DELIVERY_WORKER_DISABLE_API_DELIVERY", "true")
	if shouldStartPostDeliveryWorkers(processModeAPI) {
		t.Fatal("api mode should stop post delivery workers when the dedicated worker disable switch is set")
	}
}
