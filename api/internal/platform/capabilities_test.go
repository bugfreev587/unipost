package platform

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestCapabilitiesAllPlatformsPresent guards against accidentally
// dropping a platform when refactoring the static map. Every adapter
// in registry.go MUST have a matching capability entry — otherwise the
// validate API would silently treat that platform as "anything goes".
func TestCapabilitiesAllPlatformsPresent(t *testing.T) {
	expected := []string{
		"twitter", "instagram", "tiktok", "youtube",
		"threads", "linkedin", "bluesky",
	}
	for _, p := range expected {
		if _, ok := Capabilities[p]; !ok {
			t.Errorf("missing capability entry for platform %q", p)
		}
	}
}

// TestCapabilitiesShape sanity-checks that every entry has plausible
// non-zero values for the fields a client must rely on. We don't
// hard-code the limits here (that'd just duplicate the source) — we
// only assert "this isn't an empty struct."
func TestCapabilitiesShape(t *testing.T) {
	for name, cap := range Capabilities {
		if cap.DisplayName == "" {
			t.Errorf("%s: DisplayName is empty", name)
		}
		if cap.Text.MaxLength <= 0 {
			t.Errorf("%s: Text.MaxLength must be > 0", name)
		}
		// requires_media platforms must accept SOMETHING (image OR video)
		if cap.Media.RequiresMedia && cap.Media.Images.MaxCount == 0 && cap.Media.Videos.MaxCount == 0 {
			t.Errorf("%s: requires_media but accepts neither images nor videos", name)
		}
	}
}

// TestCapabilitiesJSONShape locks down the JSON tag names so a
// rename in the Go struct doesn't silently break the public API.
func TestCapabilitiesJSONShape(t *testing.T) {
	cap := Capabilities["twitter"]
	out, err := json.Marshal(cap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{
		`"display_name"`,
		`"text"`,
		`"max_length"`,
		`"media"`,
		`"requires_media"`,
		`"images"`,
		`"max_count"`,
		`"videos"`,
		`"thread"`,
		`"scheduling"`,
		`"first_comment"`,
	} {
		if !strings.Contains(string(out), key) {
			t.Errorf("expected JSON to contain %s; got %s", key, string(out))
		}
	}
}

// TestCapabilitiesSchemaVersion locks the version string so a refactor
// can't silently de-bump it. The docs page hardcodes this value in an
// example response, so a drift would lie to readers.
func TestCapabilitiesSchemaVersion(t *testing.T) {
	if CapabilitiesSchemaVersion != "1.4" {
		t.Errorf("CapabilitiesSchemaVersion must be 1.4 in Sprint 4, got %s", CapabilitiesSchemaVersion)
	}
}

// TestBlueskySupportsThreads — Sprint 3 PR8 flipped this. Lock it.
func TestBlueskySupportsThreads(t *testing.T) {
	if !Capabilities["bluesky"].Text.SupportsThreads {
		t.Error("bluesky.text.supports_threads must be true after Sprint 3 PR8")
	}
}

// TestCapabilityFor exercises the case-insensitivity contract — the
// callers (handler + validate) lower-case the platform name before
// lookup, so the map keys must be lowercase.
func TestCapabilityFor(t *testing.T) {
	for name := range Capabilities {
		if name != strings.ToLower(name) {
			t.Errorf("capability key %q must be lowercase", name)
		}
	}
	if _, ok := CapabilityFor("twitter"); !ok {
		t.Error("CapabilityFor(twitter) should succeed")
	}
	if _, ok := CapabilityFor("nonexistent"); ok {
		t.Error("CapabilityFor(nonexistent) should fail")
	}
}
