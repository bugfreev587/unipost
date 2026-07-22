package db

import (
	"os"
	"strings"
	"testing"
)

func TestInstagramInboxQuarantineScriptIsDryRunFirstAndRecoverable(t *testing.T) {
	content, err := os.ReadFile("../../ops/instagram_inbox_quarantine.sql")
	if err != nil {
		t.Fatalf("read quarantine SQL: %v", err)
	}
	source := strings.ToLower(string(content))
	compact := strings.Join(strings.Fields(source), " ")

	for _, required := range []string{
		`\set on_error_stop on`,
		`:{?incident_key}`,
		`\set apply false`,
		`\if :apply`,
		`:{?expected_count}`,
		`:{?expected_digest}`,
		`:{?recovery_ready}`,
		`begin isolation level repeatable read;`,
		`pg_advisory_xact_lock`,
		`create temp table instagram_inbox_quarantine_candidates`,
		`sa.platform = 'instagram'`,
		`i.source in ('ig_comment', 'ig_dm')`,
		`count(distinct sa.external_account_id) > 1`,
		`for update of i`,
		`to_jsonb(i) as original_row`,
		`md5(coalesce(string_agg(id, ',' order by id), ''))`,
		`insert into inbox_item_quarantine`,
		`on conflict (incident_key, original_inbox_item_id) do nothing`,
		`delete from inbox_items`,
		`candidate_count_matches`,
		`candidate_digest_matches`,
		`preserved_count_matches`,
		`deleted_count_matches`,
		`remaining_count_matches`,
		`rollback;`,
		`commit;`,
	} {
		if !strings.Contains(compact, strings.Join(strings.Fields(required), " ")) {
			t.Errorf("quarantine SQL missing %q", required)
		}
	}

	insertAt := strings.Index(compact, "insert into inbox_item_quarantine")
	deleteAt := strings.Index(compact, "delete from inbox_items")
	if insertAt < 0 || deleteAt < 0 || insertAt > deleteAt {
		t.Fatal("quarantine SQL must preserve evidence before deleting live rows")
	}
	if strings.Count(compact, "begin isolation level repeatable read;") != 1 {
		t.Fatalf("repeatable-read BEGIN count = %d, want 1", strings.Count(compact, "begin isolation level repeatable read;"))
	}
	for _, forbidden := range []string{
		"select body",
		"select author_name",
		"select author_id",
		"select original_row",
		"returning original_row",
		`\echo 'body`,
		`\echo 'author`,
	} {
		if strings.Contains(compact, forbidden) {
			t.Errorf("quarantine SQL may expose message content via %q", forbidden)
		}
	}
}

func TestInstagramInboxQuarantineRunbookRequiresApprovalAndIndependentRecoveryEvidence(t *testing.T) {
	content, err := os.ReadFile("../../../docs/instagram-inbox-quarantine-runbook.md")
	if err != nil {
		t.Fatalf("read quarantine runbook: %v", err)
	}
	source := strings.ToLower(string(content))

	for _, required := range []string{
		"exact routing deployment",
		"mapping coverage",
		"dry run",
		"candidate count",
		"candidate digest",
		"snapshot",
		"pitr",
		"explicit production approval",
		"no message bodies",
		"independent upstream ownership evidence",
		"must not blanket-restore",
		"staging",
		"production",
	} {
		if !strings.Contains(source, required) {
			t.Errorf("quarantine runbook missing %q", required)
		}
	}
}
