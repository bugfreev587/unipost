package socialconnectioncutover

import (
	"context"
	"strings"
	"testing"
)

func TestReconcilerRequiresDatabaseAndExactCutoverIdentity(t *testing.T) {
	for name, reconciler := range map[string]*Reconciler{
		"database":    {CommitSHA: "0123456789abcdef0123456789abcdef01234567", EnvironmentID: "environment"},
		"commit":      {EnvironmentID: "environment"},
		"environment": {CommitSHA: "0123456789abcdef0123456789abcdef01234567"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := reconciler.Run(context.Background(), Report{}); err == nil {
				t.Fatal("invalid reconciler configuration succeeded")
			}
		})
	}
}

func TestCutoverSQLIsLockedFailClosedAndConservesDurableState(t *testing.T) {
	for _, want := range []string{
		"LOCK TABLE social_accounts",
		"cutover_backend_pid = pg_backend_pid()",
		"cross_managed_user",
		"incompatible_credential_state",
		"nonterminal sibling physical targets cannot be collapsed safely",
		"migration_conflict",
		"inbox_item_supersessions",
		"physical_daily_publish_reservations",
		"cutover postcondition failed",
		"phase = 'cutover'",
	} {
		if !strings.Contains(cutoverSQL, want) {
			t.Fatalf("cutover SQL missing %q", want)
		}
	}
}

func TestCutoverOnlyMutatesUnclassifiedLegacyBindings(t *testing.T) {
	for _, want := range []string{
		"BOOL_OR(connection_id IS NULL) AS has_legacy_candidate",
		"WHERE provider_identity IS NULL\n  AND connection_id IS NULL",
		"WHERE reason IS NOT NULL\n  AND has_legacy_candidate",
		"WHERE sa.id = ANY(conflict.source_account_ids)\n  AND sa.connection_id IS NULL",
		"ON CONFLICT (workspace_id, platform, provider_identity)",
		"WHERE p.id = sa.profile_id\n  AND sa.connection_id IS NULL",
		"NOT EXISTS (\n    SELECT 1\n    FROM social_connection_migration_conflicts conflict",
	} {
		if !strings.Contains(cutoverSQL, want) {
			t.Fatalf("cutover SQL must preserve Expand-era classified bindings; missing %q", want)
		}
	}
}
