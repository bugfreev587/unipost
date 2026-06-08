package worker

import (
	"errors"
	"testing"
)

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
