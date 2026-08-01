package db

import (
	"os"
	"strings"
	"testing"
)

func TestTargetedReconnectRecoveryKeepsPublicAccountIDAndRequiresEvidence(t *testing.T) {
	body, err := os.ReadFile("queries/social_connections.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(body))
	for _, want := range []string{
		"-- name: getreconnectrequiredsocialaccountforupdate :one",
		"sa.id = @reconnect_account_id",
		"p.workspace_id = @workspace_id",
		"sa.profile_id = @profile_id",
		"sa.platform = @platform",
		"sa.connection_id is null",
		"sa.status = 'reconnect_required'",
		"conflict.resolved_at is null",
		"sa.id = any(conflict.source_account_ids)",
		"-- name: recoverreconnectrequiredsocialaccountbinding :one",
		"where sa.id = @reconnect_account_id",
		"binding_version = binding_version + 1",
		"connection_id = @connection_id",
		"returning id, profile_id",
		"-- name: resolvemigrationconflictsforrecoveredaccount :exec",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("targeted reconnect SQL missing %q", want)
		}
	}
}

func TestResolvedConnectionCredentialsRequireSameWorkspace(t *testing.T) {
	body, err := os.ReadFile("queries/social_accounts.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(body))
	if !strings.Contains(sql, "sa.connection_id is null or sc.workspace_id = @workspace_id") {
		t.Fatal("resolved credential query must reject a connection from another Workspace")
	}
}
