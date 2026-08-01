package socialconnectioncutover

import (
	"strings"
	"testing"
)

// Quarantine must be recoverable by the customer without operator help.
//
// Every account listing filters on `disconnected_at IS NULL`
// (ListSocialAccountsByProfile, ListSocialAccountsByWorkspace, and their
// filtered variants). Writing disconnected_at during quarantine therefore
// erases the account from the Dashboard and from GET /v1/accounts, and the
// Dashboard's Reconnect control only renders for accounts the list returned.
// Since the targeted-reconnect API requires the caller to pass the quarantined
// account_id, an invisible account is an unrecoverable one.
//
// 'reconnect_required' alone already blocks publishing:
// socialAccountDisconnectedForPublish and socialAccountUnavailableForDelivery
// both treat it as unavailable, as does the retry policy_admission gate.
func TestCutoverQuarantineKeepsAccountsVisibleAndRecoverable(t *testing.T) {
	quarantine := cutoverStatement(t, "SET status = 'reconnect_required',")

	if strings.Contains(quarantine, "disconnected_at =") {
		t.Fatalf("quarantine must not write disconnected_at, which hides the account from every listing:\n%s", quarantine)
	}
	for _, want := range []string{
		"status = 'reconnect_required'",
		"'reconnect_required_at'",
		// Already-disconnected rows are not publishable and nobody is waiting on
		// them; relabelling them would resurrect dead accounts in the UI.
		"AND sa.disconnected_at IS NULL",
		"AND sa.status <> 'disconnected'",
	} {
		if !strings.Contains(quarantine, want) {
			t.Fatalf("quarantine statement missing %q:\n%s", want, quarantine)
		}
	}
}

// access_token and refresh_token are AES-256-GCM ciphertexts encrypted with a
// fresh random nonce per call (internal/crypto.AESEncryptor.Encrypt). Two rows
// holding byte-identical credentials therefore always compare as different, so
// COUNT(DISTINCT access_token) > 1 is true for every multi-row group and would
// quarantine healthy accounts wholesale.
func TestCutoverNeverComparesEncryptedCredentialCiphertext(t *testing.T) {
	for name, sql := range map[string]string{
		"cutover":   cutoverSQL,
		"preflight": preflightConflictsSQL,
	} {
		for _, forbidden := range []string{
			"COUNT(DISTINCT access_token)",
			"COUNT(DISTINCT refresh_token)",
			"COUNT(DISTINCT COALESCE(refresh_token, ''))",
			"access_token_count",
			"refresh_token_count",
		} {
			if strings.Contains(sql, forbidden) {
				t.Fatalf("%s SQL compares encrypted credential ciphertext via %q", name, forbidden)
			}
		}
	}
}

// A row the customer already disconnected carries frozen credentials, scopes,
// and routing metadata. Letting it diverge from the live row quarantines the
// live account for a difference that says nothing about it. One-row-per-Profile
// stays unscoped: it guards a unique index, not staleness.
func TestCutoverStalenessCountersAreScopedToLiveRows(t *testing.T) {
	for name, sql := range map[string]string{
		"cutover":   cutoverSQL,
		"preflight": preflightConflictsSQL,
	} {
		if !strings.Contains(sql, "AS is_live") {
			t.Fatalf("%s SQL does not define a live-row predicate", name)
		}
		for _, counter := range []string{"app_mode_count", "token_expiry_count", "scope_count", "routing_metadata_count"} {
			definition := counterDefinition(t, name, sql, counter)
			if !strings.Contains(definition, "is_live") && !strings.Contains(definition, "status <> 'disconnected'") {
				t.Fatalf("%s SQL counter %q is not scoped to live rows:\n%s", name, counter, definition)
			}
		}
	}
}

// The preflight report is the only thing an operator sees before committing an
// irreversible cutover. If it classifies differently from the reconciliation,
// a clean report can still be followed by mass quarantine.
func TestPreflightAndCutoverClassifyIdenticalReasons(t *testing.T) {
	reasons := []string{
		"mixed_ownership", "cross_managed_user", "missing_managed_owner",
		"owner_on_byo_connection", "duplicate_profile_binding", "incompatible_app_mode",
		"incompatible_credential_state", "incompatible_scope", "incompatible_routing_metadata",
		"missing_provider_identity",
	}
	for _, reason := range reasons {
		if !strings.Contains(cutoverSQL, reason) {
			t.Fatalf("cutover SQL missing conflict reason %q", reason)
		}
		if !strings.Contains(preflightConflictsSQL, reason) {
			t.Fatalf("preflight SQL missing conflict reason %q", reason)
		}
	}
	for _, want := range []string{
		"WHEN token_expiry_count > 1",
		"AND is_live",
	} {
		if !strings.Contains(cutoverSQL, want) || !strings.Contains(preflightConflictsSQL, want) {
			t.Fatalf("preflight and cutover classification diverge on %q", want)
		}
	}
}

// cutoverStatement returns the single SQL statement containing marker.
func cutoverStatement(t *testing.T, marker string) string {
	t.Helper()
	index := strings.Index(cutoverSQL, marker)
	if index < 0 {
		t.Fatalf("cutover SQL has no statement containing %q", marker)
	}
	start := max(strings.LastIndex(cutoverSQL[:index], "\n\n"), 0)
	end := strings.Index(cutoverSQL[index:], ";")
	if end < 0 {
		t.Fatalf("statement containing %q is unterminated", marker)
	}
	return cutoverSQL[start : index+end+1]
}

// counterDefinition returns the aggregate expression bound to name.
func counterDefinition(t *testing.T, sqlName, sql, name string) string {
	t.Helper()
	index := strings.Index(sql, " AS "+name)
	if index < 0 {
		t.Fatalf("%s SQL does not define %q", sqlName, name)
	}
	start := strings.LastIndex(sql[:index], "COUNT(")
	if start < 0 {
		t.Fatalf("%s SQL definition of %q is not an aggregate", sqlName, name)
	}
	return sql[start : index+len(" AS "+name)]
}

// Live-scoped ownership classification is only safe because the attach step
// refuses to bind a row whose owner differs from the promoted connection.
// Without that equality, a group containing one live row and one
// already-disconnected row belonging to a different managed end user would
// promote on the live row's ownership and then bind the other user's
// historical row to those credentials.
func TestCutoverAttachRequiresOwnershipEquality(t *testing.T) {
	attach := cutoverStatement(t, "SET connection_id = sc.id")
	for _, want := range []string{
		"sa.connection_type = sc.connection_type",
		"sa.external_user_id IS NOT DISTINCT FROM sc.external_user_id",
	} {
		if !strings.Contains(attach, want) {
			t.Fatalf("attach statement missing ownership guard %q:\n%s", want, attach)
		}
	}
}

// Ownership counters describe the accounts that are connected today. Letting an
// account the customer disconnected long ago outvote the live account sharing
// its provider identity quarantined healthy accounts: in production 18 of 22
// conflict groups were a single live row outvoted by dead rows.
func TestCutoverOwnershipCountersAreScopedToLiveRows(t *testing.T) {
	for name, sql := range map[string]string{
		"cutover":   cutoverSQL,
		"preflight": preflightConflictsSQL,
	} {
		for _, counter := range []string{
			"connection_type_count", "managed_owner_count",
			"missing_managed_owner_count", "byo_owner_count",
		} {
			definition := counterDefinition(t, name, sql, counter)
			if !strings.Contains(definition, "is_live") {
				t.Fatalf("%s SQL counter %q is not scoped to live rows:\n%s", name, counter, definition)
			}
		}
		for _, want := range []string{
			"BOOL_OR(connection_type = 'managed' AND is_live) AS has_managed",
			"BOOL_OR(connection_type = 'byo' AND is_live) AS has_byo",
		} {
			if !strings.Contains(sql, want) {
				t.Fatalf("%s SQL ownership flag not scoped to live rows: %q", name, want)
			}
		}
	}
}

// One row per (profile, connection) is a unique-index boundary, not staleness.
// It must keep counting every row, or a profile holding both a live and a
// historical row for one identity would attach both and abort the cutover.
func TestCutoverDuplicateProfileBindingStillCountsEveryRow(t *testing.T) {
	for name, sql := range map[string]string{
		"cutover":   cutoverSQL,
		"preflight": preflightConflictsSQL,
	} {
		for _, want := range []string{
			"COUNT(*) AS account_count",
			"COUNT(DISTINCT profile_id) AS profile_count",
		} {
			if !strings.Contains(sql, want) {
				t.Fatalf("%s SQL must count every row for %q", name, want)
			}
		}
	}
}
