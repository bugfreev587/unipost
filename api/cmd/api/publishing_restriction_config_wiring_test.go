package main

import (
	"os"
	"strings"
	"testing"
)

func TestPublishingRestrictionCampaignConfigWarningsCoverAllManualPrerequisites(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, key := range []string{
		"PUBLISHING_RESTRICTION_CAMPAIGN_PREVIEW_SECRET",
		"LOOPS_TIKTOK_FREE_RESTRICTION_NOTICE_TRANSACTIONAL_ID",
		"LOOPS_TIKTOK_FREE_RECOVERY_NOTICE_TRANSACTIONAL_ID",
	} {
		if !strings.Contains(text, key) {
			t.Fatalf("missing startup configuration warning for %s", key)
		}
	}
	if !strings.Contains(text, "warnMissingPublishingRestrictionCampaignConfig()") ||
		!strings.Contains(text, "required configuration is missing") {
		t.Fatal("publishing restriction campaign configuration warnings are not wired at startup")
	}
}
