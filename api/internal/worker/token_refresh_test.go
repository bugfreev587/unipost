package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platformcontrols"
)

func TestTokenRefreshFullyDisabledStopsBeforeTokenOrAdapter(t *testing.T) {
	evaluator := &blockedPlatformControlEvaluator{}
	w := NewTokenRefreshWorker(nil, nil).SetPlatformOperationalControls(evaluator)
	w.refreshOne(context.Background(), db.SocialAccount{ID: "tk_1", Platform: "tiktok"})
	if evaluator.calls != 1 || evaluator.op != platformcontrols.OperationRead {
		t.Fatalf("evaluation=%+v", evaluator)
	}
}

func TestRefreshFailureShouldMarkReconnectRequired(t *testing.T) {
	if !refreshFailureShouldMarkReconnectRequired(errors.New(`refresh failed (400): {"error":{"message":"Error validating access token: Session has expired","type":"OAuthException","code":190}}`)) {
		t.Fatal("expected Meta OAuth 190 refresh failure to require reconnect")
	}

	if refreshFailureShouldMarkReconnectRequired(errors.New(`refresh failed (500): {"error":{"message":"temporarily unavailable"}}`)) {
		t.Fatal("temporary refresh failures should stay retryable")
	}

	if refreshFailureShouldMarkReconnectRequired(nil) {
		t.Fatal("nil error should not require reconnect")
	}
}
