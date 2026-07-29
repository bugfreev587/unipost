package db

import (
	"os"
	"strings"
	"testing"
)

func TestSocialConnectionLifecycleQueriesSeparateUnbindAndDisconnect(t *testing.T) {
	body, err := os.ReadFile("queries/social_connections.sql")
	if err != nil {
		t.Fatal(err)
	}
	queries := strings.ToLower(string(body))
	for _, want := range []string{
		"-- name: unbindsocialaccountbinding :one",
		"binding_status = 'unbound'",
		"binding_version = binding_version + 1",
		"-- name: disconnectallsocialaccountbindings :many",
		"where connection_id = @connection_id",
		"-- name: disconnectsocialconnection :one",
	} {
		if !strings.Contains(queries, want) {
			t.Errorf("social connection lifecycle queries missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"-- name: reactivatesiblingsocialaccountbindings :many",
		"profile_id <> @target_profile_id",
	} {
		if strings.Contains(queries, forbidden) {
			t.Errorf("reconnect must not reactivate sibling bindings via %q", forbidden)
		}
	}
}
