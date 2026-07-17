package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestProcessModeAcceptsMediaWorker(t *testing.T) {
	mode, err := normalizeProcessMode(" media-worker ")
	if err != nil {
		t.Fatalf("normalizeProcessMode returned error: %v", err)
	}
	if mode != processModeMediaWorker {
		t.Fatalf("mode = %q, want %q", mode, processModeMediaWorker)
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

func TestDBPoolMaxConnsUsesMediaWorkerSpecificEnv(t *testing.T) {
	t.Setenv("DATABASE_MAX_CONNS", "31")
	t.Setenv("MEDIA_PROCESSING_WORKER_DATABASE_MAX_CONNS", "13")

	got := dbPoolMaxConnsForMode(processModeMediaWorker, worker.PostDeliveryWorkerConfig{})
	if got != 13 {
		t.Fatalf("media worker db max conns = %d, want specific override 13", got)
	}
}

func TestProcessModeWorkerStartupRules(t *testing.T) {
	t.Setenv("POST_DELIVERY_WORKER_DISABLE_API_DELIVERY", "")
	t.Setenv("MEDIA_PROCESSING_WORKER_DISABLE_API_PROCESSING", "")
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
	if shouldStartHTTPServer(processModeMediaWorker) {
		t.Fatal("media worker mode must not start the public HTTP server")
	}
	if !shouldStartMediaProcessingWorkers(processModeMediaWorker) {
		t.Fatal("media worker mode should start media processing workers")
	}
	if !shouldStartMediaProcessingWorkers(processModeAPI) {
		t.Fatal("api mode should keep media workers as a rollout fallback by default")
	}
	if shouldStartMediaProcessingWorkers(processModePostDeliveryWorker) {
		t.Fatal("post delivery worker mode must not start media workers")
	}
}

func TestProcessModeCanDisableAPIMediaWorkersAfterDedicatedWorkerIsEnabled(t *testing.T) {
	t.Setenv("MEDIA_PROCESSING_WORKER_DISABLE_API_PROCESSING", "true")
	if shouldStartMediaProcessingWorkers(processModeAPI) {
		t.Fatal("api mode should stop media workers when the dedicated worker disable switch is set")
	}
}

func TestProcessModeCanDisableAPIDeliveryWorkersAfterDedicatedWorkerIsEnabled(t *testing.T) {
	t.Setenv("POST_DELIVERY_WORKER_DISABLE_API_DELIVERY", "true")
	if shouldStartPostDeliveryWorkers(processModeAPI) {
		t.Fatal("api mode should stop post delivery workers when the dedicated worker disable switch is set")
	}
}

func TestWorkerHealthHandlerOnlyExposesHealth(t *testing.T) {
	handler := newWorkerHealthHandler()

	healthReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	healthResp := httptest.NewRecorder()
	handler.ServeHTTP(healthResp, healthReq)
	if healthResp.Code != http.StatusOK {
		t.Fatalf("/health status = %d, want 200", healthResp.Code)
	}
	if !strings.Contains(healthResp.Body.String(), `"status":"ok"`) {
		t.Fatalf("/health body = %q, want status ok", healthResp.Body.String())
	}

	apiReq := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	apiResp := httptest.NewRecorder()
	handler.ServeHTTP(apiResp, apiReq)
	if apiResp.Code != http.StatusNotFound {
		t.Fatalf("/v1/me status = %d, want 404 from worker-only health handler", apiResp.Code)
	}
}
