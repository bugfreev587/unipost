package worker

import (
	"testing"
	"time"
)

func TestMediaCleanupWorkerRunsDaily(t *testing.T) {
	if mediaCleanupInterval != 24*time.Hour {
		t.Fatalf("mediaCleanupInterval = %s, want 24h", mediaCleanupInterval)
	}
}
