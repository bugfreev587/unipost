package db

import (
	"os"
	"strings"
	"testing"
)

func readQueryContractFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return strings.ToLower(string(body))
}

func TestSocialConnectionQueriesExposeLifecycleOperations(t *testing.T) {
	queries := readQueryContractFile(t, "queries/social_connections.sql")
	for _, name := range []string{
		"createsocialconnection",
		"getsocialconnectionforupdate",
		"findcanonicalsocialconnectionforupdate",
		"refreshsocialconnection",
		"disconnectsocialconnection",
		"createorreactivatesocialaccountbinding",
	} {
		if !strings.Contains(queries, "-- name: "+name+" ") {
			t.Errorf("social connection queries missing %s", name)
		}
	}
}

func TestSocialConnectionQueriesPreserveStableReconnectIdentity(t *testing.T) {
	queries := readQueryContractFile(t, "queries/social_connections.sql")
	for _, want := range []string{
		"provider_identity = @provider_identity",
		"for update of sc",
		"status = 'active'",
		"disconnected_at = null",
		"binding_version = social_accounts.binding_version + 1",
	} {
		if !strings.Contains(queries, want) {
			t.Errorf("connection lifecycle queries missing %q", want)
		}
	}
}

func TestSocialConnectionQueriesProvideDualReadProjection(t *testing.T) {
	queries := readQueryContractFile(t, "queries/social_accounts.sql")
	for _, want := range []string{
		"-- name: getresolvedsocialaccountbyidandworkspace :one",
		"sa.connection_id",
		"sa.binding_version",
		"sa.binding_status",
		"coalesce(sc.access_token, sa.access_token) as resolved_access_token",
		"coalesce(sc.refresh_token, sa.refresh_token) as resolved_refresh_token",
		"coalesce(sc.external_user_id, sa.external_user_id) as resolved_external_user_id",
		"left join social_connections sc on sc.id = sa.connection_id",
	} {
		if !strings.Contains(queries, want) {
			t.Errorf("resolved social account query missing %q", want)
		}
	}
}
