package main

import (
	"os"
	"strings"
	"testing"
)

func TestDeploymentBRegistersGIFConversionRoutesBeforeGenericMediaRoute(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	body := strings.ToLower(string(source))
	post := strings.Index(body, `r.post("/v1/media/gif-conversions"`)
	get := strings.Index(body, `r.get("/v1/media/gif-conversions/{id}"`)
	generic := strings.Index(body, `r.get("/v1/media/{id}"`)
	if post < 0 || get < 0 || generic < 0 {
		t.Fatalf("GIF or generic Media route missing")
	}
	if post > generic || get > generic {
		t.Fatal("GIF conversion routes must be registered before generic /v1/media/{id}")
	}
}

func TestDeploymentBUsesAtomicGIFAdmission(t *testing.T) {
	source, err := os.ReadFile("../../internal/mediaprocessing/admission.go")
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(string(source))
	for _, want := range []string{"pg_advisory_xact_lock", "countactivemediaprocessingjobsbyworkspace", "countgifconversionssince", "creategifmediaprocessingjob"} {
		if !strings.Contains(body, want) {
			t.Fatalf("atomic GIF admission missing %q", want)
		}
	}
}

func TestDeploymentBMediaWorkersCanOnlyBeClaimedBySharedCoordinator(t *testing.T) {
	source, err := os.ReadFile("../../internal/worker/media_audio_overlay.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	for _, forbidden := range []string{
		"func (w *MediaAudioOverlayWorker) Start(",
		"func (w *MediaAudioOverlayWorker) runOnce(",
		"ClaimMediaProcessingJobsByKind(context.Context",
		"PromoteDueMediaProcessingRetriesByKind(context.Context",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("audio worker retains independent queue ownership: %q", forbidden)
		}
	}
}
