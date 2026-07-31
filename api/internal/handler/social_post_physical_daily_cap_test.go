package handler

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

func TestDailyTargetsDeduplicateSiblingBindingsByPhysicalConnection(t *testing.T) {
	posts := []platform.PlatformPostInput{{AccountID: "binding-a"}, {AccountID: "binding-b"}}
	accounts := map[string]db.SocialAccount{
		"binding-a": {ID: "binding-a", Platform: "twitter", ConnectionID: pgtype.Text{String: "connection-a", Valid: true}},
		"binding-b": {ID: "binding-b", Platform: "twitter", ConnectionID: pgtype.Text{String: "connection-a", Valid: true}},
	}
	validation := map[string]platform.ValidateAccount{
		"binding-a": {Platform: "twitter"},
		"binding-b": {Platform: "twitter"},
	}

	targets := dailyTargetsFor(posts, accounts, validation)
	if len(targets) != 1 || targets[0].AccountID != "connection-a" || targets[0].Platform != "twitter" {
		t.Fatalf("daily targets = %+v, want one physical connection target", targets)
	}
	if key := physicalAccountKey(db.SocialAccount{ID: "legacy-binding"}); key != "legacy-binding" {
		t.Fatalf("legacy physical key = %q, want binding fallback", key)
	}
}
