package db

import (
	"strings"
	"testing"
)

func TestListIntegrationLogsSearchesExactHostedConnectMetadataIDs(t *testing.T) {
	query := strings.ToLower(listIntegrationLogs)
	for _, predicate := range []string{
		"metadata->>'connect_session_id' =",
		"metadata->>'external_user_id' =",
	} {
		if !strings.Contains(query, predicate) {
			t.Fatalf("query missing %q", predicate)
		}
	}
	for _, forbidden := range []string{
		"metadata->>'connect_session_id' ilike",
		"metadata->>'external_user_id' ilike",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("query contains substring metadata search %q", forbidden)
		}
	}
}
