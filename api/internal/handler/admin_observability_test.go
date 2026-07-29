package handler

import (
	"os"
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

func TestAdminPostFailuresListNeverReadsDebugCurl(t *testing.T) {
	body, err := os.ReadFile("admin.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	start := strings.Index(source, "func (h *AdminHandler) queryPostFailures")
	if start < 0 {
		t.Fatal("could not find queryPostFailures source")
	}
	end := strings.Index(source[start:], "func (h *AdminHandler) ListUserPostFailures")
	if end < 0 {
		t.Fatal("could not isolate queryPostFailures source")
	}
	query := strings.ToLower(source[start : start+end])
	if strings.Contains(query, "spr.debug_curl") {
		t.Fatalf("list query reads debug TOAST: %s", query)
	}
	if strings.Count(query, "null::text as debug_curl") != 3 {
		t.Fatalf("all three list branches must project a null debug value")
	}
}
