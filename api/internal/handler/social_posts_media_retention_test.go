package handler

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

func TestMediaIDsForRetentionFromPostMetadataDedupesAcrossPlatformPosts(t *testing.T) {
	meta, err := platform.EncodePostMetadata([]platform.PlatformPostInput{
		{AccountID: "sa_1", Caption: "one", MediaIDs: []string{"med_a", "med_b", "med_a"}},
		{AccountID: "sa_2", Caption: "two", MediaIDs: []string{"med_b", "med_c"}},
	})
	if err != nil {
		t.Fatalf("encode metadata: %v", err)
	}

	got := mediaIDsForRetention(db.SocialPost{
		ID:       "post_1",
		Caption:  pgtype.Text{String: "fallback", Valid: true},
		Metadata: meta,
	})

	want := []string{"med_a", "med_b", "med_c"}
	if len(got) != len(want) {
		t.Fatalf("ids = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids = %#v, want %#v", got, want)
		}
	}
}
