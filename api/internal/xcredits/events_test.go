package xcredits

import (
	"os"
	"strings"
	"testing"
)

func TestEventLedgerKeepsAccountReadOperationIDAfterReceiptCleanupWithoutContent(t *testing.T) {
	source, err := os.ReadFile("postgres.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "func (s *PostgresStore) ListEvents")
	if start < 0 {
		t.Fatal("ListEvents implementation not found")
	}
	query := text[start:]
	if !strings.Contains(query, "WHEN x.purpose IN ('account_profile', 'account_post_history') THEN x.idempotency_key") {
		t.Fatal("account-read operation_id must survive response-receipt cleanup")
	}
	for _, forbidden := range []string{"request_fingerprint", "encrypted_request", "response_json", "idempotency_key_hash"} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("public event query exposes forbidden receipt field %q", forbidden)
		}
	}
}
