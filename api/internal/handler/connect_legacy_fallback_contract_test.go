package handler

import (
	"os"
	"strings"
	"testing"
)

// Until the operator-run cutover promotes them, every account in the database
// predates connection authority. SaveVerified reports that with
// ErrLegacyFallbackRequired, and each connect path must then fall back to its
// pre-feature write path rather than failing.
//
// The legacy path reactivates the existing social_accounts row in place, which
// preserves the account ID that scheduled posts, post results, analytics, and
// Inbox rows all reference. Treating the sentinel as a hard error instead would
// break reconnect — the primary recovery flow for an expired token — for every
// pre-cutover account, for the entire duration of the Expand window.
func TestConnectPathsFallBackForPreCutoverAccounts(t *testing.T) {
	for path, want := range map[string][]string{
		// OAuth callback: the shared helper reports "not handled" so the
		// caller runs the legacy reactivate branch below it.
		"oauth.go": {
			"socialconnections.ErrLegacyFallbackRequired",
			"using legacy account path",
			"return false",
		},
		// Facebook Pages finalize: per-Page fallback, so one pre-cutover Page
		// does not fail the whole selection.
		"oauth_facebook.go": {
			"socialconnections.ErrLegacyFallbackRequired",
			"using legacy account path",
		},
		// Direct-credential connect: this endpoint always answered a duplicate
		// with 409 ACCOUNT_ALREADY_CONNECTED, so the fallback keeps that.
		"social_accounts.go": {
			"socialconnections.ErrLegacyFallbackRequired",
			"ACCOUNT_ALREADY_CONNECTED",
		},
	} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		body := string(source)
		for _, fragment := range want {
			if !strings.Contains(body, fragment) {
				t.Fatalf("%s does not handle pre-cutover accounts; missing %q", path, fragment)
			}
		}
	}
}

// A store error that is not the legacy sentinel must still surface as a real
// failure. Every connect path maps the remaining sentinels to a 4xx: answering
// an ownership or Profile-scope conflict with a 500 hides a caller-fixable
// problem behind a server error.
func TestConnectMapsConnectionStoreConflictsToClientErrors(t *testing.T) {
	source, err := os.ReadFile("social_accounts.go")
	if err != nil {
		t.Fatalf("read social_accounts.go: %v", err)
	}
	body := string(source)
	for _, want := range []string{
		"socialconnections.ErrAlreadyConnected",
		"socialconnections.ErrReconnectTargetConflict",
		"socialconnections.ErrOwnershipConflict",
		"socialconnections.ErrProfileNotInWorkspace",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("Connect does not map %s to a client error", want)
		}
	}
}

// The legacy reactivate path drops cached per-account provider state so
// refreshed credentials take effect immediately. The connection-authority path
// must do the same, or a reconnect leaves adapters holding stale state.
func TestOAuthConnectionSaveInvalidatesCachedAccountState(t *testing.T) {
	source, err := os.ReadFile("oauth.go")
	if err != nil {
		t.Fatalf("read oauth.go: %v", err)
	}
	body := string(source)
	start := strings.Index(body, "func (h *OAuthHandler) saveOAuthAccountViaConnection")
	if start < 0 {
		t.Fatal("saveOAuthAccountViaConnection not found")
	}
	end := strings.Index(body[start:], "\nfunc ")
	if end < 0 {
		end = len(body) - start
	}
	if !strings.Contains(body[start:start+end], "platform.InvalidateAccountState") {
		t.Fatal("connection-authority save does not invalidate cached account state")
	}
}
