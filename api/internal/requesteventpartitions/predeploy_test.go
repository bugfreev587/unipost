package requesteventpartitions

import (
	"context"
	"strings"
	"testing"
)

func TestEnsureDatabaseReadyRejectsMalformedDatabaseURL(t *testing.T) {
	err := EnsureDatabaseReady(context.Background(), "://malformed")
	if err == nil || !strings.Contains(err.Error(), "parse partition database URL") {
		t.Fatalf("error = %v", err)
	}
}
