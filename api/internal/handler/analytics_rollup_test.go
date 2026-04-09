package handler

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// TestParseGroupBy_Empty — empty input returns nil so the handler
// applies its default ("platform").
func TestParseGroupBy_Empty(t *testing.T) {
	if got := parseGroupBy(""); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// TestParseGroupBy_Single — one dimension parses cleanly.
func TestParseGroupBy_Single(t *testing.T) {
	got := parseGroupBy("platform")
	if len(got) != 1 || got[0] != "platform" {
		t.Errorf("got %v", got)
	}
}

// TestParseGroupBy_Multiple — comma-separated, with whitespace
// stripping and order preservation.
func TestParseGroupBy_Multiple(t *testing.T) {
	got := parseGroupBy("platform,  status , social_account_id")
	want := []string{"platform", "status", "social_account_id"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("idx %d: got %q want %q", i, got[i], want[i])
		}
	}
}

// TestParseGroupBy_Dedupe — duplicate entries collapse but preserve
// first-occurrence order.
func TestParseGroupBy_Dedupe(t *testing.T) {
	got := parseGroupBy("platform,status,platform,status")
	if len(got) != 2 {
		t.Errorf("expected 2 unique, got %v", got)
	}
}

// TestAllowedGroupBy_LocksTheAllowlist — the rollup query interpolates
// these strings directly into the SQL, so the allowlist is the only
// thing preventing SQL injection. Lock it so a future refactor can't
// silently expand the surface.
func TestAllowedGroupBy_LocksTheAllowlist(t *testing.T) {
	expected := map[string]bool{
		"platform":          true,
		"social_account_id": true,
		"external_user_id":  true,
		"status":            true,
	}
	if len(allowedGroupBy) != len(expected) {
		t.Errorf("allowedGroupBy has %d entries, want %d (added a dimension without thinking about SQL injection?)",
			len(allowedGroupBy), len(expected))
	}
	for k := range expected {
		if _, ok := allowedGroupBy[k]; !ok {
			t.Errorf("missing expected key %q", k)
		}
	}
	for k := range allowedGroupBy {
		if !expected[k] {
			t.Errorf("unexpected key %q in allowedGroupBy — must be in the test allowlist too", k)
		}
		// Defense in depth: every column expression must look like a
		// safe column reference, not contain semicolons / spaces /
		// quotes that could indicate injection.
		expr := allowedGroupBy[k]
		for _, bad := range []string{";", "'", "\"", " --", " /*"} {
			if strings.Contains(expr, bad) {
				t.Errorf("allowedGroupBy[%q] = %q contains forbidden token %q",
					k, expr, bad)
			}
		}
	}
}

// TestAllowedGranularity_Locked — same idea for the date_trunc unit.
func TestAllowedGranularity_Locked(t *testing.T) {
	expected := []string{"day", "week", "month"}
	if len(allowedGranularity) != len(expected) {
		t.Errorf("allowedGranularity changed unexpectedly")
	}
	for _, g := range expected {
		if _, ok := allowedGranularity[g]; !ok {
			t.Errorf("missing %q", g)
		}
	}
}

// TestParseRollupRange_RequiresBoth — both from + to are required.
func TestParseRollupRange_RequiresBoth(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/?from=2026-04-01T00:00:00Z", nil)
	_, _, ok := parseRollupRange(w, r)
	if ok {
		t.Error("missing 'to' should have failed")
	}
}

// TestParseRollupRange_RejectsHugeRange — guard against multi-year
// queries that would scan the entire results table.
func TestParseRollupRange_RejectsHugeRange(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET",
		"/?from=2024-01-01T00:00:00Z&to=2026-01-01T00:00:00Z", nil)
	_, _, ok := parseRollupRange(w, r)
	if ok {
		t.Error("2-year range should have been rejected")
	}
}

// TestParseRollupRange_HappyPath — a normal 30-day range parses cleanly.
func TestParseRollupRange_HappyPath(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET",
		"/?from=2026-04-01T00:00:00Z&to=2026-05-01T00:00:00Z", nil)
	from, to, ok := parseRollupRange(w, r)
	if !ok {
		t.Fatalf("expected ok, got fail (status %d)", w.Code)
	}
	if from.IsZero() || to.IsZero() || !from.Before(to) {
		t.Errorf("unexpected range: %v → %v", from, to)
	}
}
