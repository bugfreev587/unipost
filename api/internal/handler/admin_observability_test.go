package handler

import (
	"strings"
	"testing"
)

func TestAdminLogsListSelectOmitsPayloadColumns(t *testing.T) {
	query := strings.ToLower(adminLogsSelect(false))
	if strings.Contains(query, "request_payload") || strings.Contains(query, "response_payload") {
		t.Fatalf("list query selects payload columns: %s", query)
	}
}

func TestAdminLogsDetailSelectIncludesPayloadColumns(t *testing.T) {
	query := strings.ToLower(adminLogsSelect(true))
	for _, column := range []string{"l.request_payload", "l.response_payload"} {
		if !strings.Contains(query, column) {
			t.Fatalf("detail query missing %s", column)
		}
	}
}
