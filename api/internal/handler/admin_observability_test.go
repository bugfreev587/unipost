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

func TestAdminPostFailureDebugDetailIsBoundedPrefersNewStoreAndIsAudited(t *testing.T) {
	query := strings.ToLower(adminPostFailureDebugSQL)
	for _, required := range []string{
		"post_failure_debug_details",
		"left(nullif(spr.debug_curl, ''), 8192)",
		"octet_length(nullif(spr.debug_curl, ''))",
		"workspace_id",
		"source_kind",
	} {
		if !strings.Contains(query, required) {
			t.Fatalf("debug detail query missing %q: %s", required, query)
		}
	}
	boundedValue := strings.Index(query, "left(nullif(detail.debug_text, ''), 8192)")
	legacyValue := strings.Index(query, "left(nullif(spr.debug_curl, ''), 8192)")
	if boundedValue < 0 || legacyValue < 0 || boundedValue > legacyValue {
		t.Fatal("debug detail must prefer the bounded observability store before legacy fallback")
	}

	body, err := os.ReadFile("admin.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	start := strings.Index(source, "func (h *AdminHandler) GetPostFailureDebug")
	end := strings.Index(source[start:], "func (h *AdminHandler) ListPosts")
	if start < 0 || end < 0 {
		t.Fatal("GetPostFailureDebug source boundaries not found")
	}
	handlerSource := source[start : start+end]
	for _, required := range []string{
		"audit.Log(",
		"audit.ActionPostFailureDebugViewed",
		"auth.GetUserID(r.Context())",
		"OriginalBytes",
		"StoredBytes",
		"Truncated",
	} {
		if !strings.Contains(handlerSource, required) {
			t.Fatalf("debug detail handler missing %q", required)
		}
	}
	if strings.Contains(handlerSource, "Metadata: out.DebugCurl") || strings.Contains(handlerSource, "After: out") {
		t.Fatal("debug detail audit must never copy diagnostic text into audit_log")
	}
}
